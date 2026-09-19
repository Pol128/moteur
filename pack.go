// Package moteur lit une ligne d'ingrédient à l'aide d'un pack de langue.
//
// Le moteur ne connaît aucune langue : ni les unités, ni les prépositions, ni
// la règle d'accord ne sont écrites ici. Tout vient du pack (`lang/fr.toml`).
// Ajouter l'espagnol, c'est écrire `lang/es.toml` — pas toucher au Go.
//
// Ce fichier est le portage de `crawler/langpack.py`. Le portage est
// volontairement littéral : à comportement identique, une divergence se lit
// comme une erreur de traduction et non comme une variante d'écriture.
package moteur

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/BurntSushi/toml"
	"golang.org/x/text/unicode/norm"
)

// NFKD ne décompose ni œ ni æ : la table les traite à part.
var ligatures = strings.NewReplacer(
	"œ", "oe", "Œ", "oe",
	"æ", "ae", "Æ", "ae",
)

// Unite est une unité du pack, avec toutes ses écritures.
type Unite struct {
	Cle       string
	Dimension string // masse · volume · longueur · compte · imprecise
	Singulier string
	Pluriel   string
	Abrev     string
	Formes    []string
	Base      *float64 // en grammes, millilitres ou centimètres
	Facteur   *float64 // douzaine = 12, moitié = 0,5
}

// Convertible dit si l'unité a une valeur de base connue. Un verre n'en a pas :
// lui en inventer une serait le genre de supposition que ce projet refuse.
func (u *Unite) Convertible() bool { return u != nil && u.Base != nil }

// Quantite est une valeur lue, et le fait qu'elle soit donnée à la louche.
type Quantite struct {
	Valeur        float64
	Approximative bool
}

// LectureUnite est ce qu'on a lu dans un texte d'unité : l'unité, et ce qui
// l'habillait.
//
// Facteur porte les multiplicateurs décollés en chemin : « demi litre » rend
// l'unité litre et 0,5. Les qualificatifs, eux, ne changent rien à la
// quantité — « grosse cuillère à soupe » reste une cuillère à soupe.
type LectureUnite struct {
	Unite         *Unite
	Qualificatifs []string
	Facteur       float64
}

// Pack est un pack de langue chargé.
type Pack struct {
	Langue                string
	Version               int
	Unites                map[string]*Unite
	ClesUnites            []string // ordre stable, pour tout ce qui énumère
	Partitifs             []string // du plus long au plus court
	PartitifsEnTete       []string
	PartitifsAccentues    bool
	Litterales            map[string]Quantite
	Fractions             map[string]Quantite
	Indefinies            []string // du plus long au plus court
	QualificatifsAvant    map[string]bool
	QualificatifsApres    map[string]bool
	Multiplicateurs       map[string]float64
	Notes                 map[string][]string
	Delimiteurs           [][2]string
	SeuilPluriel          float64
	SeparateurDecimal     string
	SeparateursIntervalle []string
	SeparateursAddition   []string
	SeparateursInverses   []string
	Apostrophes           []rune // la canonique en tête
	MarquesPluriel        []string

	formes map[string]*Unite

	// Précompilés une fois pour toutes : voir compileMotifs, dans parse.go.
	motifsOptionnel     []*regexp.Regexp
	motifsApproxTete    []*regexp.Regexp
	motifsApproxPartout []*regexp.Regexp
	motifsIntervalle    []*regexp.Regexp
	motifMarquesPluriel *regexp.Regexp
	motifAddition       *regexp.Regexp
	motifInverse        *regexp.Regexp

	// Les formes qui autorisent à détacher ce qui suit une virgule finale,
	// préparations et habillage confondus : voir preparationFinale.
	formesPreparation map[string]bool
	formesHabillage   map[string]bool
}

// NombreDeFormes rend le nombre d'écritures d'unité reconnues, toutes unités
// confondues.
func (p *Pack) NombreDeFormes() int { return len(p.formes) }

// --------------------------------------------------------------- normalisation

// Normalise rend comparables « Càs », « càs » et « CaS » — les trois existent
// dans le corpus. Sans cette étape, le pack devrait énumérer les casses, ce qui
// est sans fin.
//
// plierAccents est levé pour les prépositions, et pour elles seules : « 16 dés
// de foies gras » ne doit pas donner le partitif « des ».
func (p *Pack) Normalise(texte string) string {
	return p.normalise(texte, true)
}

func (p *Pack) normalise(texte string, plierAccents bool) string {
	if texte == "" {
		return ""
	}
	texte = strings.ToLower(strings.TrimSpace(texte))
	texte = ligatures.Replace(texte)
	if len(p.Apostrophes) > 1 {
		canonique := string(p.Apostrophes[0])
		for _, a := range p.Apostrophes[1:] {
			texte = strings.ReplaceAll(texte, string(a), canonique)
		}
	}
	for _, marque := range p.MarquesPluriel {
		texte = strings.ReplaceAll(texte, marque, "")
	}
	texte = norm.NFKD.String(texte)
	if plierAccents {
		texte = sansAccents(texte)
	} else {
		texte = norm.NFC.String(texte)
	}
	return reduitEspaces(texte)
}

// sansAccents retire les marques combinantes. La décomposition ayant déjà eu
// lieu, le nombre de caractères est préservé — ce sur quoi reposent les
// comparaisons de préfixe.
//
// Le critère est la **classe combinatoire**, pas la catégorie Mn. Les deux
// coïncident presque partout, et se séparent sur les sélecteurs de variante
// (U+FE00…U+FE0F) : rangés en Mn, mais de classe nulle. Un « Milsani®️ » du
// corpus suffit à faire diverger les deux lectures — mesuré, une occurrence sur
// 2 658 écritures distinctes.
func sansAccents(texte string) string {
	var b strings.Builder
	b.Grow(len(texte))
	for i, r := range texte {
		if norm.NFKD.PropertiesString(texte[i:]).CCC() != 0 {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// reduitEspaces fait le travail de `re.sub(r"\s+", " ", texte).strip()` de
// Python, dont le `\s` couvre tout l'espace Unicode et non le seul ASCII.
func reduitEspaces(texte string) string {
	champs := strings.FieldsFunc(texte, unicode.IsSpace)
	return strings.Join(champs, " ")
}

// ------------------------------------------------------------------- unités

// LitUniteExacte rend l'unité écrite exactement comme ça, sans qualificatif.
func (p *Pack) LitUniteExacte(texte string) *Unite {
	return p.formes[p.Normalise(texte)]
}

// LireUnite rend l'unité, débarrassée de ses adjectifs.
//
// Marmiton publie 162 « unités » dont une quarantaine ne sont qu'une unité de
// base habillée : « grosse cuillère à soupe », « petite boîte ». La
// construction est productive — on décolle plutôt que d'énumérer.
func (p *Pack) LireUnite(texte string) *LectureUnite {
	mots := strings.Fields(p.Normalise(texte))
	if len(mots) == 0 {
		return nil
	}
	var qualificatifs []string
	facteur := 1.0
	for change := true; change && len(mots) > 0; {
		change = false
		// Une forme reconnue telle quelle gagne toujours : c'est ce qui fait
		// lire « demi » seul comme l'unité moitié, et « demi litre » comme un
		// litre divisé par deux.
		if direct := p.formes[strings.Join(mots, " ")]; direct != nil {
			return &LectureUnite{Unite: direct, Qualificatifs: qualificatifs, Facteur: facteur}
		}
		for _, taille := range []int{3, 2, 1} { // « bien bombée » fait deux mots
			if len(mots) <= taille {
				continue
			}
			tete := strings.Join(mots[:taille], " ")
			if multiplicateur, ok := p.Multiplicateurs[tete]; ok {
				facteur *= multiplicateur
				mots = mots[taille:]
				change = true
				break
			}
			if p.QualificatifsAvant[tete] {
				qualificatifs = append(qualificatifs, tete)
				mots = mots[taille:]
				change = true
				break
			}
			queue := strings.Join(mots[len(mots)-taille:], " ")
			if p.QualificatifsApres[queue] {
				qualificatifs = append(qualificatifs, queue)
				mots = mots[:len(mots)-taille]
				change = true
				break
			}
		}
	}
	return nil
}

// Accorde rend « 1,5 cuillère à soupe » mais « 2 cuillères à soupe ». Le seuil
// vient du pack : l'anglais mettrait 1, le français met 2.
func (p *Pack) Accorde(unite *Unite, valeur *float64) string {
	if valeur == nil || *valeur < p.SeuilPluriel {
		return unite.Singulier
	}
	return unite.Pluriel
}

// ----------------------------------------------------------------- quantités

// Quantite lit une quantité écrite en toutes lettres, en fraction ou en
// chiffres.
func (p *Pack) Quantite(texte string) *Quantite {
	// Les fractions typographiques sont testées avant normalisation : NFKD
	// décompose « ½ » en « 1⁄2 », avec une barre de fraction U+2044 qui n'est
	// pas la barre oblique du clavier.
	if brute, ok := p.Fractions[strings.TrimSpace(texte)]; ok {
		return &brute
	}
	cle := p.Normalise(texte)
	if cle == "" {
		return nil
	}
	if connue, ok := p.Litterales[cle]; ok {
		return &connue
	}
	if nombre, ok := p.Nombre(cle); ok {
		return &Quantite{Valeur: nombre}
	}
	return nil
}

// Nombre lit « 0,5 », « 1/2 », « 1 1/2 ». Le second retour est faux si ce n'est
// pas un nombre.
func (p *Pack) Nombre(texte string) (float64, bool) {
	texte = strings.ReplaceAll(p.Normalise(texte), p.SeparateurDecimal, ".")
	if texte == "" {
		return 0, false
	}
	total := 0.0
	for _, morceau := range strings.Fields(texte) {
		if haut, bas, coupe := strings.Cut(morceau, "/"); coupe {
			numerateur, err1 := strconv.ParseFloat(haut, 64)
			denominateur, err2 := strconv.ParseFloat(bas, 64)
			if err1 != nil || err2 != nil || denominateur == 0 {
				return 0, false
			}
			total += numerateur / denominateur
			continue
		}
		valeur, err := strconv.ParseFloat(morceau, 64)
		if err != nil {
			return 0, false
		}
		total += valeur
	}
	return total, true
}

// EstIndefinie : « un peu de lait » — il en faut, on ne dit pas combien.
func (p *Pack) EstIndefinie(texte string) bool {
	cle := p.Normalise(texte)
	for _, expression := range p.Indefinies {
		if expression == cle {
			return true
		}
	}
	return false
}

// --------------------------------------------------------------- prépositions

// Partitif rend le partitif en tête de texte, le plus long d'abord.
//
// C'est le marqueur de frontière du français, celui que le modèle positionnel
// de Mealie et Tandoor range dans le nom de l'aliment.
func (p *Pack) Partitif(texte string) string {
	depart := p.normalise(texte, !p.PartitifsAccentues)
	for _, prep := range p.Partitifs {
		if depart == prep {
			return prep
		}
		if strings.HasPrefix(depart, prep) {
			reste := depart[len(prep):]
			if strings.HasSuffix(prep, "'") || strings.HasPrefix(reste, " ") {
				return prep
			}
		}
	}
	return ""
}

// ------------------------------------------------------------------ chargement

// Charge lit un pack depuis un fichier TOML.
func Charge(chemin string) (*Pack, error) {
	contenu, err := os.ReadFile(chemin)
	if err != nil {
		return nil, err
	}
	return Lis(contenu, chemin)
}

// Lis bâtit un pack depuis un TOML déjà en mémoire. origine ne sert qu'aux
// messages d'erreur : c'est ce qui permet de charger aussi bien un fichier que
// le pack embarqué dans le module.
func Lis(contenu []byte, origine string) (*Pack, error) {
	var brut map[string]any
	if err := toml.Unmarshal(contenu, &brut); err != nil {
		return nil, fmt.Errorf("%s : %w", origine, err)
	}
	pack, err := Construit(brut)
	if err != nil {
		return nil, fmt.Errorf("%s : %w", origine, err)
	}
	return pack, nil
}

// Construit bâtit un pack depuis la structure d'un TOML déjà lu. C'est le point
// d'entrée des tests : un pack factice suffit à prouver que le moteur ne
// connaît aucune langue.
func Construit(brut map[string]any) (*Pack, error) {
	ortho := table(brut, "orthographe")

	apostrophes := []rune(strings.Join(chaines(ortho, "apostrophes", []string{"'"}), ""))
	if len(apostrophes) == 0 {
		apostrophes = []rune{'\''}
	}
	canonique := apostrophes[0]
	if choisie := chaine(ortho, "apostrophe_canonique"); choisie != "" {
		canonique, _ = utf8.DecodeRuneInString(choisie)
	}
	ordonnees := []rune{canonique}
	for _, a := range apostrophes {
		if a != canonique {
			ordonnees = append(ordonnees, a)
		}
	}

	quant := table(brut, "quantites")
	pack := &Pack{
		Langue:                chaineDefaut(table(brut, "pack"), "langue", "??"),
		Version:               entier(table(brut, "pack"), "version"),
		Unites:                map[string]*Unite{},
		PartitifsAccentues:    booleen(table(brut, "prepositions"), "sensible_aux_accents"),
		Litterales:            map[string]Quantite{},
		Fractions:             map[string]Quantite{},
		QualificatifsAvant:    map[string]bool{},
		QualificatifsApres:    map[string]bool{},
		Multiplicateurs:       map[string]float64{},
		Notes:                 map[string][]string{},
		SeuilPluriel:          nombreDefaut(table(brut, "flexion"), "seuil_pluriel", 2),
		SeparateurDecimal:     chaineDefaut(quant, "separateur_decimal", ","),
		SeparateursIntervalle: chaines(quant, "separateurs_intervalle", nil),
		SeparateursAddition:   chaines(quant, "separateurs_addition", nil),
		SeparateursInverses:   chaines(quant, "separateurs_inverses", nil),
		Apostrophes:           ordonnees,
		MarquesPluriel:        chaines(ortho, "marques_pluriel", nil),
		formes:                map[string]*Unite{},
	}

	prepositions := table(brut, "prepositions")
	plier := !pack.PartitifsAccentues
	pack.Partitifs = pack.normaliseEtTrie(chaines(prepositions, "partitifs", nil), plier)
	pack.PartitifsEnTete = pack.normaliseEtTrie(chaines(prepositions, "en_tete", nil), plier)

	for mot, valeur := range table(quant, "litterales") {
		v, ok := nombre(valeur)
		if !ok {
			return nil, fmt.Errorf("quantité littérale « %s » : valeur non numérique", mot)
		}
		pack.Litterales[pack.Normalise(mot)] = Quantite{Valeur: v}
	}
	for symbole, valeur := range table(quant, "fractions") {
		v, ok := nombre(valeur)
		if !ok {
			return nil, fmt.Errorf("fraction « %s » : valeur non numérique", symbole)
		}
		pack.Fractions[symbole] = Quantite{Valeur: v}
	}
	for mot, valeur := range table(quant, "indicatives") {
		v, ok := nombre(valeur)
		if !ok {
			return nil, fmt.Errorf("quantité indicative « %s » : valeur non numérique", mot)
		}
		pack.Litterales[pack.Normalise(mot)] = Quantite{Valeur: v, Approximative: true}
	}
	pack.Indefinies = pack.normaliseEtTrie(
		chaines(table(quant, "indefinies"), "expressions", nil), true)

	qualif := table(brut, "qualificatifs")
	for _, q := range chaines(qualif, "avant", nil) {
		pack.QualificatifsAvant[pack.Normalise(q)] = true
	}
	for _, q := range chaines(qualif, "apres", nil) {
		pack.QualificatifsApres[pack.Normalise(q)] = true
	}
	for mot, valeur := range table(qualif, "multiplicateurs") {
		v, ok := nombre(valeur)
		if !ok {
			return nil, fmt.Errorf("multiplicateur « %s » : valeur non numérique", mot)
		}
		pack.Multiplicateurs[pack.Normalise(mot)] = v
	}

	notes := table(brut, "notes")
	for cle, valeur := range notes {
		if liste, ok := toutesChaines(valeur); ok {
			pack.Notes[cle] = liste
		}
	}
	if paires, ok := notes["delimiteurs"].([]any); ok {
		for _, paire := range paires {
			if couple, ok := toutesChaines(paire); ok && len(couple) == 2 {
				pack.Delimiteurs = append(pack.Delimiteurs, [2]string{couple[0], couple[1]})
			}
		}
	}

	// L'ordre du TOML est perdu par la lecture en table : on énumère les unités
	// triées, pour que deux exécutions rendent le même pack et le même message
	// d'erreur.
	unites := table(brut, "unites")
	cles := make([]string, 0, len(unites))
	for cle := range unites {
		cles = append(cles, cle)
	}
	sort.Strings(cles)

	for _, cle := range cles {
		corps := tableDe(unites[cle])
		formes := chaines(corps, "formes", nil)
		if len(formes) == 0 {
			return nil, fmt.Errorf("unité « %s » sans forme", cle)
		}
		singulier := chaineDefaut(corps, "singulier", cle)
		unite := &Unite{
			Cle:       cle,
			Dimension: chaineDefaut(corps, "dimension", "compte"),
			Singulier: singulier,
			Pluriel:   chaineDefaut(corps, "pluriel", singulier),
			Abrev:     chaineDefaut(corps, "abrev", singulier),
			Formes:    formes,
			Base:      nombreFacultatif(corps, "base"),
			Facteur:   nombreFacultatif(corps, "facteur"),
		}
		pack.Unites[cle] = unite
		pack.ClesUnites = append(pack.ClesUnites, cle)
		for _, forme := range formes {
			normalisee := pack.Normalise(forme)
			if normalisee == "" {
				return nil, fmt.Errorf("unité « %s » : forme vide", cle)
			}
			// Deux unités qui revendiquent la même écriture rendraient la
			// lecture dépendante de l'ordre du fichier : on refuse à l'entrée.
			if occupant, pris := pack.formes[normalisee]; pris && occupant.Cle != cle {
				return nil, fmt.Errorf(
					"forme « %s » revendiquée par « %s » et « %s »", forme, occupant.Cle, cle)
			}
			pack.formes[normalisee] = unite
		}
	}

	pack.compileMotifs()
	return pack, nil
}

// normaliseEtTrie range du plus long au plus court, à égalité dans l'ordre reçu
// — « de la » doit être essayé avant « de ». La longueur se compte en
// caractères, comme en Python : « à » en fait un, pas deux.
func (p *Pack) normaliseEtTrie(brutes []string, plierAccents bool) []string {
	formes := make([]string, 0, len(brutes))
	for _, brute := range brutes {
		formes = append(formes, p.normalise(brute, plierAccents))
	}
	sort.SliceStable(formes, func(i, j int) bool {
		return utf8.RuneCountInString(formes[i]) > utf8.RuneCountInString(formes[j])
	})
	return formes
}

// ------------------------------------------------------- lecture d'un TOML brut

func table(brut map[string]any, cle string) map[string]any {
	if brut == nil {
		return nil
	}
	return tableDe(brut[cle])
}

func tableDe(valeur any) map[string]any {
	if t, ok := valeur.(map[string]any); ok {
		return t
	}
	return nil
}

func chaine(brut map[string]any, cle string) string {
	if brut == nil {
		return ""
	}
	s, _ := brut[cle].(string)
	return s
}

func chaineDefaut(brut map[string]any, cle, defaut string) string {
	if s := chaine(brut, cle); s != "" {
		return s
	}
	return defaut
}

func booleen(brut map[string]any, cle string) bool {
	if brut == nil {
		return false
	}
	b, _ := brut[cle].(bool)
	return b
}

func entier(brut map[string]any, cle string) int {
	v, ok := nombre(brutValeur(brut, cle))
	if !ok {
		return 0
	}
	return int(v)
}

func brutValeur(brut map[string]any, cle string) any {
	if brut == nil {
		return nil
	}
	return brut[cle]
}

func nombre(valeur any) (float64, bool) {
	switch v := valeur.(type) {
	case int64:
		return float64(v), true
	case int:
		return float64(v), true
	case float64:
		return v, true
	}
	return 0, false
}

func nombreDefaut(brut map[string]any, cle string, defaut float64) float64 {
	if v, ok := nombre(brutValeur(brut, cle)); ok {
		return v
	}
	return defaut
}

func nombreFacultatif(brut map[string]any, cle string) *float64 {
	v, ok := nombre(brutValeur(brut, cle))
	if !ok {
		return nil
	}
	return &v
}

func chaines(brut map[string]any, cle string, defaut []string) []string {
	if liste, ok := toutesChaines(brutValeur(brut, cle)); ok {
		return liste
	}
	return defaut
}

// toutesChaines n'accepte une liste que si tous ses éléments sont des chaînes —
// c'est ce qui écarte `delimiteurs`, qui est une liste de paires, de la table
// des notes.
func toutesChaines(valeur any) ([]string, bool) {
	liste, ok := valeur.([]any)
	if !ok {
		return nil, false
	}
	sortie := make([]string, 0, len(liste))
	for _, element := range liste {
		s, ok := element.(string)
		if !ok {
			return nil, false
		}
		sortie = append(sortie, s)
	}
	return sortie, true
}

// formateNombre reproduit le « %g » de Python : six chiffres significatifs, et
// pas de zéros en trop. C'est le format sous lequel les quantités s'écrivent
// dans le jeu de référence comme dans la sortie du filtre.
func formateNombre(valeur float64) string {
	if valeur == math.Trunc(valeur) && math.Abs(valeur) < 1e6 {
		return strconv.FormatFloat(valeur, 'f', -1, 64)
	}
	return strconv.FormatFloat(valeur, 'g', 6, 64)
}
