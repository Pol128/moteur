package moteur

// Ce que le parser lit, comparé à ce qu'un humain a décidé — portage de la
// partie de `crawler/mesure.py` qui définit « juste ».
//
// La comparaison passe par le pack, jamais par le texte : « càs » et
// « cuillères à soupe » sont la même unité, « une » et « 1 » la même quantité.
// C'est la segmentation qu'on mesure, pas l'orthographe de l'annotateur.
//
// Le reste de `mesure.py` — les bilans par source, par motif, les couples qui
// reviennent — n'est pas porté : c'est de la mesure de corpus, et le corpus
// reste dans la forge. Ce qui voyage avec le moteur, c'est de quoi vérifier
// qu'il tient le plancher du jeu de référence.

import (
	"math"
	"regexp"
	"strings"
)

// ChampsMesures sont les quatre champs sur lesquels une ligne se juge.
var ChampsMesures = []string{"quantite", "unite", "partitif", "aliment"}

// Tolerance : deux quantités sont les mêmes à un centième près. Le corpus écrit
// « 0,33 » là où le pack calcule 1/3 — exiger l'égalité stricte compterait faux
// une lecture juste.
const Tolerance = 0.011

// LigneRef est une ligne du jeu de référence annoté à la main.
type LigneRef struct {
	Source   string
	Brut     string
	Quantite string
	Unite    string
	Partitif string
	Aliment  string
	Note     string
}

func (l LigneRef) champ(nom string) string {
	switch nom {
	case "quantite":
		return l.Quantite
	case "unite":
		return l.Unite
	case "partitif":
		return l.Partitif
	default:
		return l.Aliment
	}
}

// Verdict porte le jugement champ par champ. Une valeur absente signifie « non
// mesuré » — le champ n'a pas été publié par la source.
type Verdict map[string]*bool

// Compare juge une ligne lue face à ce qui était attendu.
//
// videAbsent distingue les deux références du projet : chez un site, un champ
// vide veut dire « non publié » et ne doit rien coûter au parser ; dans
// l'annotation, il veut dire « cette ligne n'a pas d'unité » et doit être
// mesuré. C'est le seul paramètre qui les sépare, et c'est ce qui rend les deux
// taux comparables.
//
// uniteParFamille rattrape une convention de Jow, qui publie la famille et non
// l'unité écrite : « 80 g Bœuf » y sort avec l'unité « Kilogramme », sans que
// la quantité soit convertie pour autant.
func Compare(attendu LigneRef, lu *Ingredient, p *Pack, videAbsent, uniteParFamille bool) Verdict {
	verdict := Verdict{}
	for _, champ := range ChampsMesures {
		brut := strings.TrimSpace(attendu.champ(champ))
		if brut == "" && videAbsent {
			verdict[champ] = nil
			continue
		}
		juste := egal(champ, brut, lu, p, uniteParFamille)
		verdict[champ] = &juste
	}
	return verdict
}

// Tout dit si tous les champs mesurés sont justes. Une ligne dont aucun champ
// n'était mesurable ne compte pas comme juste.
func (v Verdict) Tout() (juste bool, mesure bool) {
	juste = true
	for _, champ := range ChampsMesures {
		valeur, present := v[champ]
		if !present || valeur == nil {
			continue
		}
		mesure = true
		if !*valeur {
			juste = false
		}
	}
	return juste, mesure
}

func egal(champ, brut string, lu *Ingredient, p *Pack, parFamille bool) bool {
	switch champ {
	case "quantite":
		var attendue *Quantite
		if brut != "" {
			attendue = p.Quantite(brut)
		}
		if attendue == nil {
			return lu.Quantite == nil
		}
		return lu.Quantite != nil && math.Abs(*lu.Quantite-attendue.Valeur) < Tolerance

	case "unite":
		var attendue *Unite
		if brut != "" {
			if lecture := p.LireUnite(brut); lecture != nil {
				attendue = lecture.Unite
			}
		}
		// Le rattrapage par famille est borné aux dimensions convertibles —
		// masse, volume, longueur — parce que là, le nom publié est celui de
		// l'unité de base et rien d'autre. L'étendre à « compte » ferait de
		// « Pièce » l'égal de « Gousse » et excuserait de vraies erreurs.
		if parFamille && attendue.Convertible() {
			return lu.Unite != nil && lu.Unite.Dimension == attendue.Dimension
		}
		cle := ""
		if attendue != nil {
			cle = attendue.Cle
		}
		return cle == lu.UniteCle()

	case "partitif":
		return p.Normalise(brut) == p.Normalise(lu.Partitif)

	default:
		// Sur AlimentTexte, et non sur Aliment : l'annotateur écrit ce que la
		// ligne écrit — « tomates », au pluriel —, et c'est la segmentation
		// qu'on mesure, pas l'orthographe. La résolution vers l'entrée du
		// lexique est étrangère à ce jugement ; la mesurer ici compterait
		// fausse chaque ligne que le parser normalise correctement.
		return p.Normalise(brut) == p.Normalise(lu.AlimentTexte)
	}
}

// CuisineAZ marque ses pluriels entre parenthèses : « branche(s) », « Pomme(s)
// de terre ». Le parser range ce « (s) » dans la note, faute de pouvoir deviner
// autre chose. Ce n'est pas un qualificatif, c'est une convention typographique
// : le retirer des deux côtés, sans quoi le recollage produirait un jeton « s »
// égaré et un faux désaccord.
var (
	marquePluriel = regexp.MustCompile(`(?i)\(\s*e?[sx]\s*\)`)
	ponctuation   = regexp.MustCompile(`[()\[\],;]`)
	// Le recollage de l'aliment efface aussi la virgule et le point-virgule ;
	// la restitution d'un bloc de quantité, non — il n'y en a pas là-dedans.
	crochets = regexp.MustCompile(`[()\[\]]`)
	marques  = map[string]bool{"s": true, "x": true, "es": true}
)

// Span recolle l'aliment et sa note, débarrassés de ce qui n'est que du
// découpage : les parenthèses qui isolent le qualificatif chez un site et
// l'espace qui l'en sépare chez l'autre.
//
// « Bœuf (carpaccio) » et « Bœuf » + « carpaccio » deviennent la même chaîne.
// Ce qui reste différent l'est vraiment.
func Span(p *Pack, aliment, note string) string {
	texte := marquePluriel.ReplaceAllString(aliment+" "+note, " ")
	texte = ponctuation.ReplaceAllString(texte, " ")
	var mots []string
	for _, mot := range strings.Fields(p.Normalise(texte)) {
		if !marques[mot] {
			mots = append(mots, mot)
		}
	}
	return strings.Join(mots, " ")
}

// BlocRestitue dit si notre découpe rend le bloc que la source publie, sans
// rien perdre.
//
// La comparaison est **textuelle**, et elle ne peut pas être autre chose :
// relire le bloc seul avec le parser donnerait toujours un aliment et jamais
// une unité, puisque « une unité sans rien derrière est un aliment » —
// « 4 pavé(s) » isolé se lit *pavé*, aliment. C'est une règle du parser, pas un
// accident, et elle rend le bloc illisible hors de sa ligne.
//
// Donc : ce que la ligne revendique comme quantité et comme unité, remis bout à
// bout, doit couvrir le bloc publié — ni plus, ni moins. Contrôle de frontière,
// pas d'exactitude.
func BlocRestitue(p *Pack, bloc string, lu *Ingredient) bool {
	return blocNu(p, bloc) == blocNu(p, lu.QuantiteTexte+" "+lu.UniteTexte)
}

func blocNu(p *Pack, texte string) string {
	texte = marquePluriel.ReplaceAllString(texte, " ")
	texte = crochets.ReplaceAllString(texte, " ")
	var mots []string
	for _, mot := range strings.Fields(p.Normalise(texte)) {
		if !marques[mot] {
			mots = append(mots, mot)
		}
	}
	return strings.Join(mots, " ")
}

// ------------------------------------------------------ le jeu de référence

var motifPlancher = regexp.MustCompile(`plancher:\s*([\d.]+)`)

// AnalyseJeu relit `testdata/fr.txt` : les lignes, et le plancher inscrit dans
// l'entête.
func AnalyseJeu(texte string) ([]LigneRef, float64) {
	plancher := 0.0
	var lignes []LigneRef
	for _, brut := range strings.Split(texte, "\n") {
		if strings.HasPrefix(brut, "#") {
			if trouve := motifPlancher.FindStringSubmatch(brut); trouve != nil {
				plancher = versNombre(trouve[1])
			}
			continue
		}
		if strings.TrimSpace(brut) == "" {
			continue
		}
		champs := strings.Split(brut, "\t")
		for len(champs) < 7 {
			champs = append(champs, "")
		}
		lignes = append(lignes, LigneRef{
			Source: champs[0], Brut: champs[1], Quantite: champs[2],
			Unite: champs[3], Partitif: champs[4], Aliment: champs[5], Note: champs[6],
		})
	}
	return lignes, plancher
}

// Desaccord est une ligne que le parser ne lit pas comme l'annotateur.
type Desaccord struct {
	Brut    string
	Attendu string
	Lu      string
}

// Accord mesure la part des lignes que le parser lit exactement comme le jeu de
// référence. Tout est mesuré, vides compris : un champ laissé vide y est une
// décision, pas une absence de publication.
func Accord(lignes []LigneRef, p *Pack, lit func(string) *Ingredient) (float64, []Desaccord) {
	if len(lignes) == 0 {
		return 0, nil
	}
	justes := 0
	var desaccords []Desaccord
	for _, ligne := range lignes {
		lu := lit(ligne.Brut)
		verdict := Compare(ligne, lu, p, false, false)
		if tout, _ := verdict.Tout(); tout {
			justes++
			continue
		}
		desaccords = append(desaccords, Desaccord{
			Brut:    ligne.Brut,
			Attendu: joint(ligne.Quantite, ligne.Unite, ligne.Partitif, ligne.Aliment),
			Lu:      joint(TexteQuantite(lu.Quantite), lu.UniteCle(), lu.Partitif, lu.AlimentTexte),
		})
	}
	return float64(justes) / float64(len(lignes)), desaccords
}

func joint(champs ...string) string {
	affiches := make([]string, 0, len(champs))
	for _, champ := range champs {
		if champ == "" {
			champ = "∅"
		}
		affiches = append(affiches, champ)
	}
	return strings.Join(affiches, " · ")
}

// TexteQuantite rend une quantité sous la forme « %g » de Python, ou la chaîne
// vide s'il n'y en a pas.
func TexteQuantite(valeur *float64) string {
	if valeur == nil {
		return ""
	}
	return formateNombre(*valeur)
}
