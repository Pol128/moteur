package moteur

// Lecture d'une ligne d'ingrédient française — portage de `crawler/parse.py`.
//
// Le cœur du projet. Mealie et Tandoor échouent ici parce que leur modèle est
// positionnel — `tokens[0]` la quantité, `tokens[1]` l'unité, le reste
// l'aliment — et que le français casse ce modèle sur deux points : la
// préposition partitive s'intercale, et les unités font quatre mots.
//
// Le retournement, c'est que cette grammaire est *plus* régulière que
// l'anglaise. « 2 cups sifted flour » n'a aucun marqueur de frontière ;
// « 2 cuillères à soupe de farine tamisée » en a un, explicite. Deux marqueurs
// en fait, un par famille de sites :
//
//	partitif           500 g **de** beurre          marmiton, ptitchef, 750g…
//	capitale interne   80 g **B**eurre              jow, cuisineaz
//
// Les deux sont dans le même moteur, essayés dans cet ordre.
//
// La règle qui fait tout le travail :
//
//	l'unité est le plus long préfixe que le pack reconnaît
//	ET qui est suivi d'un marqueur de frontière.
//
// Sans le « plus long », « 1 boîte de conserve de tomates » coupe au premier
// « de » et rend « conserve de tomates ». Sans le « reconnu par le pack »,
// « 4 pommes de terre » coupe aussi, et rend « pommes » comme unité.

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Aliments est le lexique, quand il existe : n'importe quoi qui sache dire si
// un nom normalisé lui appartient.
type Aliments interface {
	Contient(nomNormalise string) bool
}

// Ensemble est le lexique le plus simple qui soit.
type Ensemble map[string]bool

// Contient satisfait Aliments.
func (e Ensemble) Contient(nom string) bool { return e[nom] }

var (
	// Puces et tirets de liste tapés par l'auteur en tête de ligne.
	puces = regexp.MustCompile(`^\s*[-–—•*·]\s+`)
	// L'espace insécable et sa version étroite, que le `\s` de Go ne couvre pas.
	espaces        = regexp.MustCompile(`[\s\x{00A0}\x{202F}]+`)
	motifNombre    = regexp.MustCompile(`^-?\d+(?:[.,]\d+)?`)
	fractionAscii  = regexp.MustCompile(`^\s*/\s*(\d+)`)
	fractionMixte  = regexp.MustCompile(`^\s+(\d+)\s*/\s*(\d+)`)
	premierCarac   = regexp.MustCompile(`^\s*(.)`)
	motCommencant  = regexp.MustCompile(`^\p{L}+`)
	finPonctuation = " \t.,;:!?·•-–—*/"
	// Le point ne se retire pas d'une unité : « gou. », « c. à s. » et
	// « tran. » sont des abréviations, pas des fins de phrase.
	finUnite = " \t,;:!?·•-–—*/"
)

// Ingredient est une ligne lue. Motif dit quelle règle a servi — c'est ce qui
// rend le résidu analysable au lieu d'être un tas d'échecs indistincts.
type Ingredient struct {
	Brut          string
	Motif         string
	Quantite      *float64
	QuantiteMax   *float64 // « 2 à 3 gousses »
	QuantiteTexte string
	Approximative bool // « quelques », « environ »
	Indefinie     bool // « un peu de lait »
	Unite         *Unite
	UniteTexte    string
	Qualificatifs []string
	Partitif      string
	Aliment       string
	Note          string
	Optionnel     bool
}

// UniteCle rend la clé de l'unité, ou la chaîne vide s'il n'y en a pas.
func (i *Ingredient) UniteCle() string {
	if i.Unite == nil {
		return ""
	}
	return i.Unite.Cle
}

// EnBase rend la quantité convertie dans l'unité de base (g, ml, cm), si la
// conversion est possible.
func (i *Ingredient) EnBase() *float64 {
	if i.Quantite == nil || i.Unite == nil || i.Unite.Base == nil {
		return nil
	}
	valeur := *i.Quantite * *i.Unite.Base
	return &valeur
}

// --------------------------------------------------------------------- outils

// plieAccents décompose puis retire les marques : le nombre de caractères est
// préservé, ce sur quoi reposent les comparaisons de préfixe.
func plieAccents(texte string) string { return sansAccents(norm.NFKD.String(texte)) }

// compare met deux textes en regard sans changer leur longueur. Normalise, lui,
// retire les « (s) » et réduit les espaces : les décalages ne seraient plus
// exploitables pour découper la chaîne d'origine.
func (p *Pack) compare(a, b string, accents bool) bool {
	a, b = strings.ToLower(a), strings.ToLower(b)
	if len(p.Apostrophes) > 1 {
		canonique := string(p.Apostrophes[0])
		for _, apostrophe := range p.Apostrophes[1:] {
			a = strings.ReplaceAll(a, string(apostrophe), canonique)
			b = strings.ReplaceAll(b, string(apostrophe), canonique)
		}
	}
	if !accents {
		a, b = plieAccents(a), plieAccents(b)
	}
	return a == b
}

// octetsDe rend la longueur en octets des n premiers caractères de texte, ou -1
// si texte en compte moins. C'est ce qui permet de raisonner en caractères,
// comme Python, tout en découpant des octets.
func octetsDe(texte string, n int) int {
	compte := 0
	for i := range texte {
		if compte == n {
			return i
		}
		compte++
	}
	if compte == n {
		return len(texte)
	}
	return -1
}

// commencePar rend le nombre d'octets consommés si texte débute par forme, au
// sens de compare, ou -1.
func (p *Pack) commencePar(texte, forme string, accents bool) int {
	fin := octetsDe(texte, utf8.RuneCountInString(forme))
	if fin < 0 || !p.compare(texte[:fin], forme, accents) {
		return -1
	}
	return fin
}

// PartitifA rend le partitif qui commence exactement à l'octet i, le plus long
// d'abord, tel qu'écrit dans le texte (« d' », « de la »…).
//
// La frontière de mot est exigée des deux côtés, sinon « des » serait trouvé
// dans « dessert » et « de » dans « demi ».
func PartitifA(p *Pack, texte string, i int) string {
	if i > 0 && texte[i-1] != ' ' && texte[i-1] != '\t' {
		return ""
	}
	for _, prep := range p.Partitifs {
		consomme := p.commencePar(texte[i:], prep, p.PartitifsAccentues)
		if consomme < 0 {
			continue
		}
		fin := i + consomme
		// « d' » colle à son mot ; « de » exige une espace derrière.
		if strings.HasSuffix(prep, "'") || fin == len(texte) ||
			texte[fin] == ' ' || texte[fin] == '\t' {
			return texte[i:fin]
		}
	}
	return ""
}

func debutsDeMots(texte string) []int {
	var debuts []int
	dansUnMot := false
	for i, r := range texte {
		if unicode.IsSpace(r) {
			dansUnMot = false
			continue
		}
		if !dansUnMot {
			debuts = append(debuts, i)
			dansUnMot = true
		}
	}
	return debuts
}

// estCapitalise reconnaît une capitale en milieu de ligne : le marqueur de
// frontière de jow et CuisineAZ, qui concatènent depuis une base structurée
// (« 80 g Beurre »).
//
// Un mot tout en capitales n'en est pas un : « 1 CUILLERE DE SUCRE » est écrit
// au clavier bloqué, pas généré.
func estCapitalise(mot string) bool {
	premiere := true
	toutesEnCapitales := true
	aDesLettres := false
	for _, r := range mot {
		if !unicode.IsLetter(r) {
			continue
		}
		aDesLettres = true
		if premiere {
			if !unicode.IsUpper(r) {
				return false
			}
			premiere = false
		}
		if !unicode.IsUpper(r) {
			toutesEnCapitales = false
		}
	}
	return aDesLettres && !toutesEnCapitales
}

func premierMot(texte string) string {
	if i := strings.Index(texte, " "); i >= 0 {
		return texte[:i]
	}
	return texte
}

// ------------------------------------------------------------------ nettoyage

// Nettoie ramène la ligne à sa forme lisible : espaces réduits, puce de liste
// retirée.
func Nettoie(brut string) string {
	texte := strings.ReplaceAll(brut, "\u00a0", " ")
	texte = strings.TrimSpace(espaces.ReplaceAllString(texte, " "))
	texte = puces.ReplaceAllString(texte, "")
	return strings.TrimSpace(texte)
}

// preparationFinale détache ce qui suit la dernière virgule quand c'est une
// préparation — « 2 oignons, hachés finement ». C'est la règle des
// parenthèses, étendue à l'écriture que 20 lignes du corpus emploient.
//
// La virgule ne peut pas suffire à déclencher : elle sépare tout aussi bien
// deux aliments, et « Sel, poivre » perdrait son poivre. Ce qui tranche, c'est
// que le segment soit fait des seules formes déclarées par le pack, dont au
// moins une préparation : l'habillage (« finement ») n'est pas une préparation
// à lui seul.
func preparationFinale(texte string, p *Pack) (reste, note string) {
	virgule := strings.LastIndex(texte, ",")
	if virgule < 0 {
		return texte, ""
	}
	segment := strings.TrimSpace(texte[virgule+1:])
	tete := strings.TrimSpace(texte[:virgule])
	if segment == "" || tete == "" {
		return texte, ""
	}
	preparation := false
	for _, mot := range strings.Fields(segment) {
		normalise := p.Normalise(mot)
		if p.formesPreparation[normalise] {
			preparation = true
			continue
		}
		if !p.formesHabillage[normalise] {
			return texte, ""
		}
	}
	if !preparation {
		return texte, ""
	}
	return tete, segment
}

// ExtraitNotes sort les parenthèses de la ligne : ce sont des notes, pas des
// aliments. Marmiton balise d'ailleurs le complément à part (« (vieilles) »,
// « (bio) »), ce qui confirme la lecture.
//
// Les marqueurs « facultatif » et « optionnel » sont cherchés partout, y
// compris dans la note qu'on vient de sortir.
func ExtraitNotes(texte string, p *Pack) (reste, note string, optionnel bool) {
	// La marque de pluriel entre parenthèses — « 4 pavé(s) », 24 000 lignes
	// chez CuisineAZ — est une convention typographique, pas un complément.
	// Elle se retire avant le découpage : `Normalise` la replie déjà, mais il
	// passe après, et la marque est alors sortie de la ligne comme la note
	// « s ».
	if p.motifMarquesPluriel != nil {
		texte = p.motifMarquesPluriel.ReplaceAllString(texte, "")
	}

	ouvrants := map[rune]rune{}
	fermants := map[rune]bool{}
	for _, paire := range p.Delimiteurs {
		ouvre, _ := utf8.DecodeRuneInString(paire[0])
		ferme, _ := utf8.DecodeRuneInString(paire[1])
		ouvrants[ouvre] = ferme
		fermants[ferme] = true
	}

	var notes, sortie []string
	profondeur, debut := 0, 0
	for i, caractere := range texte {
		if _, estOuvrant := ouvrants[caractere]; estOuvrant {
			if profondeur == 0 {
				sortie = append(sortie, texte[debut:i])
				debut = i + utf8.RuneLen(caractere)
			}
			profondeur++
		} else if fermants[caractere] && profondeur > 0 {
			profondeur--
			if profondeur == 0 {
				notes = append(notes, strings.TrimSpace(texte[debut:i]))
				debut = i + utf8.RuneLen(caractere)
			}
		}
	}
	if profondeur > 0 { // parenthèse jamais refermée
		notes = append(notes, strings.TrimSpace(texte[debut:]))
	} else {
		sortie = append(sortie, texte[debut:])
	}

	reste = strings.TrimSpace(espaces.ReplaceAllString(strings.Join(sortie, ""), " "))
	var retenues []string
	for _, n := range notes {
		if n != "" {
			retenues = append(retenues, n)
		}
	}
	note = strings.Join(retenues, " ; ")

	plat := strings.ToLower(reste + " " + note)
	for _, marqueur := range p.Notes["optionnel"] {
		if strings.Contains(plat, strings.ToLower(marqueur)) {
			optionnel = true
			break
		}
	}
	if optionnel {
		// « farine facultatif » : le marqueur en fin de ligne appartient à la
		// note, pas au nom de l'aliment.
		for i, motif := range p.motifsOptionnel {
			nouveau := motif.ReplaceAllString(reste, "")
			if nouveau != reste {
				reste = strings.TrimSpace(nouveau)
				marqueur := p.Notes["optionnel"][i]
				if note == "" {
					note = marqueur
				} else {
					note = note + " ; " + marqueur
				}
				break
			}
		}
	}

	if sansNote, preparation := preparationFinale(reste, p); preparation != "" {
		reste = sansNote
		if note == "" {
			note = preparation
		} else {
			note = note + " ; " + preparation
		}
	}
	return reste, note, optionnel
}

// ------------------------------------------------------------------ quantités

type quantiteLue struct {
	valeur        *float64
	maximum       *float64
	texte         string
	approximative bool
	indefinie     bool
	trouvee       bool
}

// LitQuantite rend la quantité en tête, et ce qui reste après elle.
//
// Trois écritures, toutes mesurées dans le corpus : le nombre (« 500 », « 0,5 »,
// « 1/2 », « 1 1/2 », « ½ »), le mot (« une », « quelques », « demi ») et
// l'expression indéfinie (« un peu », « à volonté »).
func litQuantite(texte string, p *Pack) (quantiteLue, string) {
	var q quantiteLue

	// « environ 200 g de farine » : le marqueur précède le nombre 566 fois.
	for _, motif := range p.motifsApproxTete {
		if trouve := motif.FindStringIndex(texte); trouve != nil {
			q.approximative = true
			texte = texte[trouve[1]:]
			break
		}
	}

	for _, expression := range p.Indefinies {
		fin := p.commencePar(texte, expression, false)
		if fin < 0 {
			continue
		}
		if fin == len(texte) || !estAlphanumerique(texte[fin:]) {
			q.indefinie, q.trouvee = true, true
			q.texte = texte[:fin]
			return q, strings.Trim(texte[fin:], finPonctuation)
		}
	}

	if valeur, consomme, ok := litNombre(texte, p); ok {
		reste := texte[consomme:]
		q.valeur, q.trouvee = &valeur, true
		q.texte = strings.TrimSpace(texte[:consomme])
		if maximum, apres, trouve := litIntervalle(reste, p); trouve {
			q.maximum = &maximum
			reste = apres
			q.texte = strings.TrimSpace(texte[:len(texte)-len(reste)])
		}
		return q, strings.TrimLeftFunc(reste, unicode.IsSpace)
	}

	// « quelques feuilles de menthe », « une pincée de sel », « demi litre ».
	if mot := motCommencant.FindString(texte); mot != "" {
		if litterale, connue := p.Litterales[p.Normalise(mot)]; connue {
			valeur := litterale.Valeur
			q.valeur = &valeur
			q.approximative = litterale.Approximative
			q.texte = mot
			q.trouvee = true
			return q, strings.TrimLeftFunc(texte[len(mot):], unicode.IsSpace)
		}
	}

	return q, texte
}

func estAlphanumerique(texte string) bool {
	r, _ := utf8.DecodeRuneInString(texte)
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsNumber(r)
}

// litNombre lit un nombre en tête : entier, décimal, fraction, fraction
// typographique, et la forme mixte « 1 1/2 » relevée dans les recettes
// québécoises.
func litNombre(texte string, p *Pack) (valeur float64, consomme int, ok bool) {
	if texte != "" {
		premier, taille := utf8.DecodeRuneInString(texte)
		if fraction, connue := p.Fractions[string(premier)]; connue {
			return fraction.Valeur, taille, true
		}
	}

	trouve := motifNombre.FindString(texte)
	if trouve == "" {
		return 0, 0, false
	}
	valeur = versNombre(strings.ReplaceAll(trouve, ",", "."))
	consomme = len(trouve)

	if fraction := fractionAscii.FindStringSubmatchIndex(texte[consomme:]); fraction != nil {
		diviseur := versNombre(texte[consomme:][fraction[2]:fraction[3]])
		if diviseur != 0 {
			valeur /= diviseur
		}
		return valeur, consomme + fraction[1], true
	}

	suite := texte[consomme:]
	if mixte := fractionMixte.FindStringSubmatchIndex(suite); mixte != nil {
		diviseur := versNombre(suite[mixte[4]:mixte[5]])
		if diviseur != 0 {
			return valeur + versNombre(suite[mixte[2]:mixte[3]])/diviseur,
				consomme + mixte[1], true
		}
	}
	if typographique := premierCarac.FindStringSubmatchIndex(suite); typographique != nil {
		caractere := suite[typographique[2]:typographique[3]]
		if fraction, connue := p.Fractions[caractere]; connue {
			return valeur + fraction.Valeur, consomme + typographique[1], true
		}
	}
	return valeur, consomme, true
}

// litIntervalle lit « 2 à 3 gousses », « 2-3 tomates » : on garde les deux
// bornes.
func litIntervalle(texte string, p *Pack) (float64, string, bool) {
	for _, motif := range p.motifsIntervalle {
		trouve := motif.FindStringIndex(texte)
		if trouve == nil {
			continue
		}
		if haut, consomme, ok := litNombre(texte[trouve[1]:], p); ok {
			return haut, texte[trouve[1]+consomme:], true
		}
	}
	return 0, texte, false
}

func versNombre(texte string) float64 {
	valeur, _ := strconv.ParseFloat(texte, 64)
	return valeur
}

// ---------------------------------------------------------------------- unité

// uniteDuPrefixe rend l'unité que forme texte[:fin], qualificatifs compris.
func uniteDuPrefixe(p *Pack, texte string, fin int) *LectureUnite {
	prefixe := strings.Trim(texte[:fin], finUnite)
	if prefixe == "" {
		return nil
	}
	return p.LireUnite(prefixe)
}

// sansPartitifInitial traite « 100 g de d'emmental râpé » — coquille de la base
// de meilleurduchef. Le partitif a déjà été consommé ; s'il s'en présente un
// second, il n'est pas dans le nom de l'aliment non plus.
func sansPartitifInitial(p *Pack, aliment string) string {
	forme := PartitifA(p, aliment, 0)
	if forme == "" {
		return aliment
	}
	return strings.Trim(aliment[len(forme):], finPonctuation)
}

// composeDuLexique dit si le lexique reconnaît la ligne entière comme un nom
// d'aliment. Il n'a voix au chapitre que là où rien ne sépare les mots.
//
// Deux situations que rien ne distingue de l'extérieur, et qui n'appellent pas
// la même réponse :
//
//	bouquet garni        rien entre les deux mots
//	gousse de vanille    un partitif entre les deux
//
// Sans partitif, couper est **faux** : « bouquet garni » et « filet mignon »
// sont des noms entiers, et rendre l'aliment « garni » ne se défend d'aucune
// façon. Le lexique est le seul à pouvoir le dire, et il le dit.
//
// Avec un partitif, les deux lectures se tiennent — unité *gousse* + aliment
// *vanille*, ou aliment *gousse de vanille* d'un bloc — et c'est une
// convention, pas une erreur. Celle qui fait foi est celle de `testdata/fr.txt`,
// tranchée à la main : on coupe. Laisser le lexique s'y substituer irait contre
// la référence, mesuré : l'accord y perd 2,1 points.
func composeDuLexique(texte string, p *Pack, aliments Aliments) bool {
	for _, debut := range debutsDeMots(texte) {
		if debut > 0 && PartitifA(p, texte, debut) != "" {
			return false
		}
	}
	return aliments.Contient(p.Normalise(texte))
}

type corps struct {
	unite         *Unite
	facteur       float64
	qualificatifs []string
	uniteTexte    string
	partitif      string
	aliment       string
	motif         string
}

// litCorps lit unité, partitif et aliment : les deux marqueurs de frontière à
// l'œuvre.
//
// L'ordre compte. Le partitif est essayé d'abord parce qu'il est explicite ; la
// capitale n'intervient que là où il manque, ce qui est exactement le partage
// entre les sites en prose et ceux qui concatènent.
//
// aliments est le point d'accroche du lexique : « gousse de vanille », « noix
// de muscade », « feuille de brick » sont des aliments dont le premier mot est
// aussi une unité. Aucune grammaire ne peut les distinguer de « gousse d'ail ».
func litCorps(texte string, p *Pack, avecQuantite bool, aliments Aliments) corps {
	motifNu := "aliment_nu"
	if avecQuantite {
		motifNu = "quantite_aliment"
	}
	resultat := corps{facteur: 1.0, aliment: strings.Trim(texte, finPonctuation), motif: motifNu}
	if texte == "" {
		return resultat
	}

	if aliments != nil && composeDuLexique(texte, p, aliments) {
		resultat.motif = "aliment_compose"
		return resultat
	}

	debuts := debutsDeMots(texte)

	// -- marqueur 1 : le partitif. On retient le plus long préfixe qui soit une
	// unité, sans quoi « 1 boîte de conserve de tomates » coupe trop tôt.
	meilleurDebut := -1
	var meilleureForme string
	var meilleureUnite *LectureUnite
	for _, debut := range debuts {
		forme := PartitifA(p, texte, debut)
		if forme == "" {
			continue
		}
		if debut == 0 {
			reste := strings.Trim(texte[len(forme):], finPonctuation)
			// « 1/4 **de** litre de lait » : ce premier « de » appartient à la
			// quantité, pas à l'aliment. Le français écrit aussi bien « 1/2
			// litre de lait » que « 1/4 de litre de lait », et s'arrêter ici
			// rendait « litre de lait » comme aliment.
			//
			// On réessaie donc la règle ordinaire sur ce qui suit, et on ne la
			// retient que si elle trouve une unité : sans ce garde-fou,
			// « 1/2 de citron » perdrait son partitif pour rien.
			if reste != "" {
				apres := litCorps(reste, p, avecQuantite, aliments)
				if apres.unite != nil {
					return apres
				}
			}
			// Sans quantité : « du beurre », « des oeufs » — le partitif en
			// tient lieu. Avec : « un peu de lait », « 1/2 de citron ».
			resultat.partitif = forme
			resultat.aliment = sansPartitifInitial(p, reste)
			resultat.motif = "partitif_en_tete"
			if avecQuantite {
				resultat.motif = "quantite_partitif"
			}
			return resultat
		}
		if lue := uniteDuPrefixe(p, texte, debut); lue != nil {
			meilleurDebut, meilleureForme, meilleureUnite = debut, forme, lue
		}
	}
	if meilleurDebut >= 0 {
		resultat.unite = meilleureUnite.Unite
		resultat.facteur = meilleureUnite.Facteur
		resultat.qualificatifs = meilleureUnite.Qualificatifs
		resultat.uniteTexte = strings.TrimSpace(texte[:meilleurDebut])
		resultat.partitif = meilleureForme
		resultat.aliment = sansPartitifInitial(
			p, strings.Trim(texte[meilleurDebut+len(meilleureForme):], finPonctuation))
		resultat.motif = "quantite_unite_partitif"
		return resultat
	}

	// -- marqueur 2 : la capitale interne (jow, cuisineaz).
	// Toutes les capitales sont essayées, pas seulement la première : dans
	// « 1 Gousse Ail », ces sites capitalisent aussi l'unité. S'arrêter à la
	// première rendrait « Gousse Ail » comme aliment.
	var capitales [][2]int // rang dans debuts, offset
	for rang, debut := range debuts {
		if estCapitalise(premierMot(texte[debut:])) {
			capitales = append(capitales, [2]int{rang, debut})
		}
	}
	for _, capitale := range capitales {
		rang, debut := capitale[0], capitale[1]
		if rang == 0 {
			continue
		}
		if lue := uniteDuPrefixe(p, texte, debut); lue != nil {
			resultat.unite = lue.Unite
			resultat.facteur = lue.Facteur
			resultat.qualificatifs = lue.Qualificatifs
			resultat.uniteTexte = strings.TrimSpace(texte[:debut])
			resultat.aliment = strings.Trim(texte[debut:], finPonctuation)
			resultat.motif = "quantite_unite_capitale"
			return resultat
		}
	}
	if len(capitales) > 0 && capitales[0][0] == 0 {
		// « 1 Citron vert bio » : l'aliment suit directement la quantité.
		if avecQuantite {
			resultat.motif = "quantite_aliment_capitalise"
		} else {
			resultat.motif = "aliment_nu"
		}
		return resultat
	}

	// -- ni l'un ni l'autre : « 4 cuillères à soupe sucre », « 3 œufs ».
	// Une unité n'est retenue que s'il reste un aliment derrière elle : sans ça
	// « 3 noix » deviendrait trois unités de rien.
	for i := len(debuts) - 1; i >= 1; i-- {
		debut := debuts[i]
		if lue := uniteDuPrefixe(p, texte, debut); lue != nil {
			resultat.unite = lue.Unite
			resultat.facteur = lue.Facteur
			resultat.qualificatifs = lue.Qualificatifs
			resultat.uniteTexte = strings.TrimSpace(texte[:debut])
			resultat.aliment = strings.Trim(texte[debut:], finPonctuation)
			resultat.motif = "quantite_unite_aliment"
			return resultat
		}
	}
	return resultat
}

// ------------------------------------------------------------------ additions

// terme est un terme d'addition lu seul : sa valeur, et la clé de son unité —
// vide quand il n'en porte pas.
type terme struct {
	valeur float64
	unite  string
}

// litTerme lit un terme de tête d'une addition. Il n'accepte qu'« une quantité
// et son unité, et rien d'autre » : ce qui dépasse est un aliment, et deux
// aliments ne s'additionnent pas.
//
// litCorps ne peut pas servir ici, et c'est le point à ne pas rater : « une
// unité sans rien derrière est un aliment », donc « 250 gramme » lu seul rend
// l'aliment *gramme* et aucune unité. C'est une règle du parser, pas un
// accident — le terme se lit donc par le pack directement.
func litTerme(texte string, p *Pack) (terme, bool) {
	quantite, reste := litQuantite(texte, p)
	// Un intervalle — « 2 à 3 g + 100 g » — n'a pas de somme évidente : on
	// rend la main plutôt que de trancher.
	if !quantite.trouvee || quantite.valeur == nil || quantite.maximum != nil {
		return terme{}, false
	}
	reste = strings.TrimSpace(reste)
	if reste == "" {
		return terme{valeur: *quantite.valeur}, true
	}
	lue := p.LireUnite(reste)
	if lue == nil {
		return terme{}, false
	}
	return terme{valeur: *quantite.valeur * lue.Facteur, unite: lue.Unite.Cle}, true
}

// litAddition rend la somme des termes de tête d'une addition, et le dernier
// terme — seul porteur de l'aliment.
//
// Deux conditions, et la lecture ordinaire de la ligne entière s'il en manque
// une : tous les termes portent la même unité (ou aucun n'en porte), et aucun
// terme sauf le dernier ne porte autre chose qu'une quantité. C'est ce
// garde-fou qui empêche de sommer « 2 oeufs + 1 jaune », deux aliments
// différents dont la somme ne veut rien dire.
func litAddition(texte string, p *Pack, aliments Aliments) (tete float64, dernier string, ok bool) {
	if p.motifAddition == nil {
		return 0, "", false
	}
	termes := p.motifAddition.Split(texte, -1)
	if len(termes) < 2 {
		return 0, "", false
	}
	dernier = termes[len(termes)-1]

	// Le dernier terme est une ligne complète : son unité se lit par le chemin
	// ordinaire, celui-là même qui servira ensuite.
	quantite, reste := litQuantite(dernier, p)
	if !quantite.trouvee || quantite.valeur == nil || quantite.maximum != nil {
		return 0, "", false
	}
	lu := litCorps(reste, p, quantite.trouvee, aliments)
	unite := ""
	if lu.unite != nil {
		unite = lu.unite.Cle
	}

	for _, brut := range termes[:len(termes)-1] {
		lue, bon := litTerme(brut, p)
		if !bon || lue.unite != unite {
			return 0, "", false
		}
		tete += lue.valeur
	}
	return tete, dernier, true
}

// ----------------------------------------------------------------------- lire

// Lit lit une ligne d'ingrédient. N'échoue jamais : une ligne illisible rend un
// aliment nu, ce qui est la lecture la moins fausse possible.
func Lit(brut string, p *Pack, aliments Aliments) *Ingredient {
	texte := Nettoie(brut)
	ligne := &Ingredient{Brut: brut, Motif: "aliment_nu"}
	if texte == "" {
		return ligne
	}

	texte, note, optionnel := ExtraitNotes(texte, p)
	ligne.Note, ligne.Optionnel = note, optionnel

	// « 250 g + 200 g de coulis » : les termes de tête s'ajoutent au dernier,
	// qui seul porte l'aliment et se lit donc pour la ligne entière.
	tete, dernier, additionnee := litAddition(texte, p, aliments)
	if additionnee {
		texte = dernier
	}

	quantite, reste := litQuantite(texte, p)
	ligne.Quantite = quantite.valeur
	ligne.QuantiteMax = quantite.maximum
	ligne.QuantiteTexte = quantite.texte
	ligne.Approximative = quantite.approximative
	ligne.Indefinie = quantite.indefinie

	lu := litCorps(reste, p, quantite.trouvee, aliments)
	ligne.Unite = lu.unite
	ligne.UniteTexte = lu.uniteTexte
	ligne.Qualificatifs = lu.qualificatifs
	ligne.Partitif = lu.partitif
	ligne.Aliment = lu.aliment
	ligne.Motif = lu.motif

	// « demi litre » : le multiplicateur décollé de l'unité divise la quantité.
	if lu.facteur != 1.0 {
		if ligne.Quantite != nil {
			valeur := *ligne.Quantite * lu.facteur
			ligne.Quantite = &valeur
			if ligne.QuantiteMax != nil {
				maximum := *ligne.QuantiteMax * lu.facteur
				ligne.QuantiteMax = &maximum
			}
		} else {
			facteur := lu.facteur
			ligne.Quantite = &facteur
		}
	}

	// Après le facteur : chaque terme de tête porte déjà le sien.
	if additionnee && ligne.Quantite != nil {
		valeur := *ligne.Quantite + tete
		ligne.Quantite = &valeur
	}

	if ligne.Indefinie {
		ligne.Motif = "quantite_indefinie"
	}
	if quantite.approximative {
		ligne.Approximative = true
	}
	for _, motif := range p.motifsApproxPartout {
		if motif.MatchString(brut) {
			ligne.Approximative = true
			break
		}
	}
	return ligne
}

// ------------------------------------------------- motifs dérivés du pack

// compileMotifs précompile les expressions qui dépendent des marqueurs du pack.
// Les recompiler à chaque ligne coûterait plus cher que tout le reste du
// moteur réuni, sur un corpus qui se compte en centaines de milliers de lignes.
func (p *Pack) compileMotifs() {
	p.motifsOptionnel = nil
	for _, marqueur := range p.Notes["optionnel"] {
		p.motifsOptionnel = append(p.motifsOptionnel,
			regexp.MustCompile(`(?i)[\s,;]*`+regexp.QuoteMeta(marqueur)+`\s*$`))
	}
	p.motifsApproxTete = nil
	p.motifsApproxPartout = nil
	for _, marqueur := range p.Notes["approximation"] {
		p.motifsApproxTete = append(p.motifsApproxTete,
			regexp.MustCompile(`(?i)^`+regexp.QuoteMeta(marqueur)+`\b\s*`))
		// `\b` de Go ne connaît que l'ASCII : « à peu près » y perdrait sa
		// frontière de gauche. On l'écrit donc en toutes lettres.
		p.motifsApproxPartout = append(p.motifsApproxPartout,
			regexp.MustCompile(`(?i)(?:^|[^\p{L}\p{N}_])`+regexp.QuoteMeta(marqueur)+
				`(?:[^\p{L}\p{N}_]|$)`))
	}
	p.motifsIntervalle = nil
	for _, separateur := range p.SeparateursIntervalle {
		p.motifsIntervalle = append(p.motifsIntervalle,
			regexp.MustCompile(`(?i)^\s*`+regexp.QuoteMeta(separateur)+`\s*`))
	}

	p.motifMarquesPluriel = nil
	if choix := alternative(p.MarquesPluriel); choix != "" {
		p.motifMarquesPluriel = regexp.MustCompile(`(?i)(?:` + choix + `)`)
	}
	p.formesPreparation = map[string]bool{}
	for _, forme := range p.Notes["preparations"] {
		p.formesPreparation[p.Normalise(forme)] = true
	}
	p.formesHabillage = map[string]bool{}
	for _, forme := range p.Notes["adverbes"] {
		p.formesHabillage[p.Normalise(forme)] = true
	}

	p.motifAddition = nil
	if choix := alternative(p.SeparateursAddition); choix != "" {
		// Les séparateurs d'addition mesurés sont typographiques (« + ») : il
		// n'y a pas de frontière de mot à défendre, contrairement aux
		// séparateurs d'intervalle, qui sont des mots et s'ancrent en tête.
		p.motifAddition = regexp.MustCompile(`\s*(?:` + choix + `)\s*`)
	}
}

// alternative rend les marqueurs du pack en une alternative d'expression
// régulière, chacun échappé. Vide quand le pack n'en déclare aucun — et le
// moteur s'en passe alors, comme de toute règle qu'une langue ne donne pas.
func alternative(marqueurs []string) string {
	var choix []string
	for _, marqueur := range marqueurs {
		if marqueur != "" {
			choix = append(choix, regexp.QuoteMeta(marqueur))
		}
	}
	return strings.Join(choix, "|")
}
