package moteur

// Portage de `tests/test_parse.py`. Les cas sont les mêmes, un pour un : c'est
// ce qui permet de lire une différence de résultat comme une erreur de
// traduction, et non comme une variante d'écriture.
//
// Toutes les lignes viennent du corpus, y compris les fautives : « 100 g de
// d'emmental râpé » est une coquille de la base de meilleurduchef, « 1 Gousse
// Ail » l'écriture concaténée de CuisineAZ, « 2 c. à table » du québécois de
// ptitchef. Les huit lignes sur lesquelles Tandoor se trompe (NOTE-PROJET §4)
// ont leur test à elles.

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

var (
	packUneFois sync.Once
	packPartage *Pack
	packErreur  error
)

// packFR charge `lang/fr.toml` une seule fois pour tous les tests.
func packFR(t *testing.T) *Pack {
	t.Helper()
	packUneFois.Do(func() {
		packPartage, packErreur = Charge(filepath.Join("lang", "fr.toml"))
	})
	if packErreur != nil {
		t.Fatalf("chargement du pack : %v", packErreur)
	}
	return packPartage
}

func lit(t *testing.T, ligne string) *Ingredient {
	t.Helper()
	return Lit(ligne, packFR(t), nil)
}

func litAvec(t *testing.T, ligne string, aliments Aliments) *Ingredient {
	t.Helper()
	return Lit(ligne, packFR(t), aliments)
}

// resume rend les quatre champs mesurés sous une forme lisible en cas d'échec :
// quantité · unité · partitif · aliment.
func resume(i *Ingredient) string {
	return fmt.Sprintf("%s · %s · %s · %s",
		texteQuantite(i.Quantite), i.UniteCle(), i.Partitif, i.Aliment)
}

// texteQuantite affiche « ∅ » là où il n'y a pas de quantité, pour que l'échec
// se lise d'un coup d'œil.
func texteQuantite(valeur *float64) string {
	if valeur == nil {
		return "∅"
	}
	return TexteQuantite(valeur)
}

func verifie(t *testing.T, ligne, attendu string) *Ingredient {
	t.Helper()
	lu := lit(t, ligne)
	if obtenu := resume(lu); obtenu != attendu {
		t.Errorf("« %s »\n  attendu  %s\n  lu       %s", ligne, attendu, obtenu)
	}
	return lu
}

func verifieMotif(t *testing.T, lu *Ingredient, attendu string) {
	t.Helper()
	if lu.Motif != attendu {
		t.Errorf("« %s » : motif %s, attendu %s", lu.Brut, lu.Motif, attendu)
	}
}

func verifieNote(t *testing.T, lu *Ingredient, attendu string) {
	t.Helper()
	if lu.Note != attendu {
		t.Errorf("« %s » : note %q, attendu %q", lu.Brut, lu.Note, attendu)
	}
}

// ------------------------------- les sept motifs du français, §5 de la note

func TestMotifsDuFrancais(t *testing.T) {
	verifieMotif(t, verifie(t, "500 g de beurre", "500 · gramme · de · beurre"),
		"quantite_unite_partitif")
	verifie(t, "1 gousse d'ail", "1 · gousse · d' · ail")
	verifie(t, "2 cuillères à soupe de crème fraîche",
		"2 · cuillere_a_soupe · de · crème fraîche")
	verifieMotif(t, verifie(t, "3 œufs", "3 ·  ·  · œufs"), "quantite_aliment")
	verifieMotif(t, verifie(t, "sel", "∅ ·  ·  · sel"), "aliment_nu")
	verifieMotif(t, verifie(t, "du beurre", "∅ ·  · du · beurre"), "partitif_en_tete")

	lu := verifie(t, "500 g de pommes de terre (vieilles)",
		"500 · gramme · de · pommes de terre")
	if lu.Note != "vieilles" {
		t.Errorf("note : %q, attendu %q", lu.Note, "vieilles")
	}
}

// ------------------------ « 1/4 de litre de lait » : le partitif de la quantité

func TestUniteApresLePartitifDeLaQuantite(t *testing.T) {
	verifie(t, "1/4 de litre de lait", "0.25 · litre · de · lait")
	verifie(t, "1/4 de litre de crème entière liquide",
		"0.25 · litre · de · crème entière liquide")
	verifie(t, "3/4 de verre d'eau", "0.75 · verre · d' · eau")
	verifie(t, "1/2 litre de lait", "0.5 · litre · de · lait")

	// « 1/2 de citron » n'a pas d'unité : le correctif ne doit pas en inventer.
	verifieMotif(t, verifie(t, "1/2 de citron", "0.5 ·  · de · citron"),
		"quantite_partitif")

	// « un peu de lait » : « peu » n'est pas une unité, rien ne bouge. Le
	// partitif reste celui de la ligne, c'est la quantité qui est indéfinie.
	verifieMotif(t, verifie(t, "un peu de lait", "∅ ·  · de · lait"),
		"quantite_indefinie")
}

// ------------------------------------------------- les huit lignes de Tandoor

func TestLaOuTandoorSeTrompe(t *testing.T) {
	for _, cas := range []struct{ ligne, attendu string }{
		{"500 g de pommes de terre (vieilles)", "500 · gramme · de · pommes de terre"},
		{"1 gousse d'ail", "1 · gousse · d' · ail"},
		{"2 cuillères à soupe de crème fraîche", "2 · cuillere_a_soupe · de · crème fraîche"},
		{"une pincée de sel", "1 · pincee · de · sel"},
		{"3 brins de thym", "3 · brin · de · thym"},
		{"1 citron", "1 ·  ·  · citron"},
		{"sel", "∅ ·  ·  · sel"},
		{"20 cl de vin blanc", "20 · centilitre · de · vin blanc"},
	} {
		verifie(t, cas.ligne, cas.attendu)
	}
}

// ------------------------------------------- la grammaire concaténée (#15)

func TestGrammaireConcatenee(t *testing.T) {
	verifieMotif(t, verifie(t, "80 g Beurre", "80 · gramme ·  · Beurre"),
		"quantite_unite_capitale")
	verifie(t, "1/2 gou. Ail", "0.5 · gousse ·  · Ail")
	// « 1 Gousse Ail » : la première capitale n'est pas toujours l'aliment.
	// 235 lignes de CuisineAZ en dépendent.
	verifie(t, "1 Gousse Ail", "1 · gousse ·  · Ail")
	verifie(t, "4 pavé(s) Poisson blanc", "4 · pave ·  · Poisson blanc")
	verifieMotif(t, verifie(t, "1 Citron vert bio", "1 ·  ·  · Citron vert bio"),
		"quantite_aliment_capitalise")
	// « 10 cl vinaigre blanc » : la grammaire concaténée déborde chez les sites
	// en prose, 2 185 lignes mesurées.
	verifie(t, "10 cl vinaigre blanc", "10 · centilitre ·  · vinaigre blanc")

	lu := verifie(t, "1 càc Curry (poudre)", "1 · cuillere_a_cafe ·  · Curry")
	if lu.Note != "poudre" {
		t.Errorf("note : %q, attendu %q", lu.Note, "poudre")
	}
}

// ---------------------------- la règle centrale : le partitif ne coupe que
// ---------------------------- derrière une unité

func TestFrontiere(t *testing.T) {
	// « 4 pommes de terre » — l'erreur que ferait un parser qui coupe au
	// premier « de » venu.
	for _, cas := range []struct{ ligne, aliment string }{
		{"4 pommes de terre à chair ferme", "pommes de terre à chair ferme"},
		{"2 blancs d'œuf", "blancs d'œuf"},
		{"3 clous de girofle", "clous de girofle"},
		{"2 fruits de la passion", "fruits de la passion"},
	} {
		lu := lit(t, cas.ligne)
		if lu.Unite != nil {
			t.Errorf("« %s » : unité %s, attendu aucune", cas.ligne, lu.UniteCle())
		}
		if lu.Aliment != cas.aliment {
			t.Errorf("« %s » : aliment %q, attendu %q", cas.ligne, lu.Aliment, cas.aliment)
		}
	}

	// « 1 boîte de conserve de tomates » : couper au premier « de » rendrait
	// « conserve de tomates ».
	verifie(t, "1 boîte de conserve de tomates", "1 · boite · de · tomates")

	// « 3 noix » : sans rien derrière, c'est l'aliment, pas l'unité.
	lu := lit(t, "3 noix")
	if lu.Unite != nil || lu.Aliment != "noix" {
		t.Errorf("« 3 noix » : %s", resume(lu))
	}
}

func TestLexiqueSansPartitif(t *testing.T) {
	// Sans partitif, couper est faux et le lexique est le seul à pouvoir le
	// dire : rendre l'aliment « garni » ne se défend d'aucune façon.
	p := packFR(t)
	lexique := Ensemble{p.Normalise("bouquet garni"): true}

	if sans := lit(t, "1 bouquet garni"); sans.Aliment != "garni" {
		t.Errorf("sans lexique : aliment %q, attendu %q", sans.Aliment, "garni")
	}
	avec := litAvec(t, "1 bouquet garni", lexique)
	if avec.Unite != nil || avec.Aliment != "bouquet garni" || avec.Motif != "aliment_compose" {
		t.Errorf("avec lexique : %s, motif %s", resume(avec), avec.Motif)
	}
}

func TestLexiqueAvecPartitifNePrimePas(t *testing.T) {
	// Avec un partitif, les deux lectures se tiennent — c'est une convention,
	// pas une erreur, et celle qui fait foi est celle de `testdata/fr.txt` : on
	// coupe. Laisser le lexique s'y substituer coûte 2,1 points d'accord.
	p := packFR(t)
	lexique := Ensemble{p.Normalise("gousse de vanille"): true}

	avec := litAvec(t, "1 gousse de vanille", lexique)
	if avec.UniteCle() != "gousse" || avec.Aliment != "vanille" {
		t.Errorf("« 1 gousse de vanille » : %s", resume(avec))
	}
	// Et le cas voisin, que le lexique ne connaît pas, reste coupé.
	if voisin := litAvec(t, "1 gousse d'ail", lexique); voisin.Aliment != "ail" {
		t.Errorf("« 1 gousse d'ail » : aliment %q", voisin.Aliment)
	}
}

// ------------------------ la contenance écrite entre le contenant et l'aliment

func TestContenanceApresLeContenant(t *testing.T) {
	// « 1 boîte de 796 ml de tomates broyées » : derrière un contenant retenu
	// comme unité, une seconde mesure n'est pas l'aliment. Le schéma ne porte
	// qu'un couple quantité/unité — « 1 boîte » le tient déjà —, donc la
	// contenance part en note, seul endroit où elle survit.
	lu := verifie(t, "1 boîte de 796 ml (28 oz) de tomates broyées",
		"1 · boite · de · tomates broyées")
	verifieNote(t, lu, "796 ml ; 28 oz")
	verifieMotif(t, lu, "quantite_unite_contenance")

	lu = verifie(t, "1 morceau de 2,5 cm de gingembre frais",
		"1 · morceau · de · gingembre frais")
	verifieNote(t, lu, "2,5 cm")

	// La ligne du jeu de référence, mot pour mot (`testdata/fr.txt`, ptitchef).
	lu = verifie(t, "1 sachet de 90 g de pépites à la nougatine",
		"1 · sachet · de · pépites à la nougatine")
	verifieNote(t, lu, "90 g")

	// La variante à poids terminal : la mesure est écrite derrière l'aliment,
	// et rien n'a été retenu comme unité devant.
	lu = verifie(t, "1 gigot d'agneau de 2,5 kg", "1 ·  ·  · gigot d'agneau")
	verifieNote(t, lu, "2,5 kg")
	verifieMotif(t, lu, "aliment_mesure_terminale")
}

func TestUnPartitifSeulNeDeclencheRien(t *testing.T) {
	// Il faut une mesure complète derrière le partitif — une quantité *et* son
	// unité — sans quoi la règle mange l'aliment.
	verifieNote(t, verifie(t, "250 g de farine", "250 · gramme · de · farine"), "")
	verifieNote(t, verifie(t, "1 gousse d'ail", "1 · gousse · d' · ail"), "")
	verifieNote(t, verifie(t, "1 boîte de conserve de tomates",
		"1 · boite · de · tomates"), "")
	// Les deux lignes du jeu de référence que la quantité protège : sans elle,
	// « feuilles » et « zeste » passeraient pour des contenances.
	verifie(t, "15 gr de feuilles de basilic", "15 · gramme · de · feuilles de basilic")
	verifie(t, "1 c. à café de zeste d'orange râpé",
		"1 · cuillere_a_cafe · de · zeste d'orange râpé")
	// Et sa symétrique en fin de ligne : « huile de noix » garde son nom entier,
	// faute de quantité devant l'unité.
	verifie(t, "huile de noix", "∅ ·  ·  · huile de noix")
	// « 796 ml » sans rien derrière est un aliment, pas une contenance.
	verifie(t, "1 boîte de 796 ml", "1 · boite · de · 796 ml")
	// Et une ligne tronquée ne vide pas l'aliment : il faut un aliment derrière
	// la contenance, comme il faut un aliment devant la mesure terminale.
	verifie(t, "1 boîte de 796 ml de", "1 · boite · de · 796 ml de")
	verifie(t, ", de 2 kg", "∅ ·  ·  · de 2 kg")
	// « 4 personnes » n'est pas une mesure : il faut une unité, pas seulement
	// un nombre derrière le partitif.
	verifie(t, "1 plat de 4 personnes", "1 ·  ·  · plat de 4 personnes")

	// La distinction lexicale n'est pas touchée : avec un partitif, le lexique
	// ne prime toujours pas (cf. TestLexiqueAvecPartitifNePrimePas).
	p := packFR(t)
	lexique := Ensemble{p.Normalise("gousse de vanille"): true}
	if avec := litAvec(t, "1 gousse de vanille", lexique); avec.UniteCle() != "gousse" ||
		avec.Aliment != "vanille" {
		t.Errorf("« 1 gousse de vanille » : %s", resume(avec))
	}
}

// ------------------------------------------------------------------ quantités

func TestEcrituresDuNombre(t *testing.T) {
	for _, cas := range []struct {
		texte  string
		valeur float64
	}{
		{"1/2 citron", 0.5},
		{"0,5 l de lait", 0.5},
		{"½ chou blanc", 0.5},
		{"1 1/2 lb de poulet", 1.5},
		{"-1 Citron(s) vert(s)", -1},
	} {
		lu := lit(t, cas.texte)
		if lu.Quantite == nil || *lu.Quantite != cas.valeur {
			t.Errorf("« %s » : quantité %s, attendu %g",
				cas.texte, texteQuantite(lu.Quantite), cas.valeur)
		}
	}
}

func TestIntervalle(t *testing.T) {
	lu := lit(t, "2 à 3 gousses d'ail")
	if lu.Quantite == nil || *lu.Quantite != 2 ||
		lu.QuantiteMax == nil || *lu.QuantiteMax != 3 || lu.UniteCle() != "gousse" {
		t.Errorf("« 2 à 3 gousses d'ail » : %s, max %s",
			resume(lu), texteQuantite(lu.QuantiteMax))
	}
}

func TestQuantiteLitterale(t *testing.T) {
	lu := verifie(t, "quelques feuilles de basilic", "3 · feuille · de · basilic")
	if !lu.Approximative {
		t.Error("« quelques » doit marquer la quantité comme approximative")
	}
}

func TestQuantiteIndefinie(t *testing.T) {
	lu := lit(t, "Un peu de lait")
	if !lu.Indefinie || lu.Quantite != nil || lu.Aliment != "lait" {
		t.Errorf("« Un peu de lait » : indéfinie=%v, %s", lu.Indefinie, resume(lu))
	}
}

func TestMultiplicateur(t *testing.T) {
	verifie(t, "1 demi litre d'eau", "0.5 · litre · d' · eau")
}

func TestApproximationAvantLeNombre(t *testing.T) {
	lu := verifie(t, "environ 200 g de farine", "200 · gramme · de · farine")
	if !lu.Approximative {
		t.Error("« environ » doit marquer la quantité comme approximative")
	}
}

func TestQuantiteColleeALUnite(t *testing.T) {
	verifie(t, "1kg hachis porc boeuf", "1 · kilogramme ·  · hachis porc boeuf")
}

func TestConversionEnBase(t *testing.T) {
	for _, cas := range []struct {
		ligne   string
		attendu *float64
	}{
		{"2 kg de farine", pointeur(2000)},
		{"1 cuillère à soupe d'huile", pointeur(15)},
		{"1 pincée de sel", nil},
	} {
		obtenu := lit(t, cas.ligne).EnBase()
		switch {
		case cas.attendu == nil && obtenu != nil:
			t.Errorf("« %s » : %g en base, attendu aucune conversion", cas.ligne, *obtenu)
		case cas.attendu != nil && (obtenu == nil || *obtenu != *cas.attendu):
			t.Errorf("« %s » : %s en base, attendu %g",
				cas.ligne, texteQuantite(obtenu), *cas.attendu)
		}
	}
}

func pointeur(v float64) *float64 { return &v }

// ------------------------------------------------------- le corpus tel qu'il est

func TestBruitReel(t *testing.T) {
	// Coquille dans la base de meilleurduchef.
	verifie(t, "100 g de d'emmental râpé", "100 · gramme · de · emmental râpé")
	verifie(t, "2 c. à table de beurre (30 ml ; moi, omis)",
		"2 · cuillere_a_soupe · de · beurre")
	verifie(t, "- 200 g de farine", "200 · gramme · de · farine")
	verifie(t, "1 filet d’huile d’olive", "1 · filet · d’ · huile d’olive")

	lu := verifie(t, "2 cuillère à soupe de sirop d'agave (ou sucre)",
		"2 · cuillere_a_soupe · de · sirop d'agave")
	if lu.Note != "ou sucre" {
		t.Errorf("note : %q, attendu %q", lu.Note, "ou sucre")
	}

	facultatif := lit(t, "1 sachet de levure chimique (facultatif)")
	if !facultatif.Optionnel || facultatif.Aliment != "levure chimique" {
		t.Errorf("optionnel=%v, %s", facultatif.Optionnel, resume(facultatif))
	}

	if vide := lit(t, ""); vide.Aliment != "" {
		t.Errorf("ligne vide : aliment %q", vide.Aliment)
	}
	verifieMotif(t, lit(t, "TEMPS DE REPOS: 30 minutes"), "aliment_nu")

	// « 1 CUILLERE A SOUPE DE SUCRE » est tapé au clavier bloqué, pas généré :
	// un mot tout en capitales n'est pas un marqueur.
	verifie(t, "1 CUILLERE A SOUPE DE SUCRE", "1 · cuillere_a_soupe · DE · SUCRE")
}

// ------------------- MOTEUR-4 : « gramme(s) » et le second terme d'une addition

// CuisineAZ écrit le pluriel entre parenthèses — « 4 pavé(s) », 24 000 lignes.
// Le pack le déclare dans `marques_pluriel` et `Normalise` le replie déjà, mais
// l'extraction des notes passait avant : la marque en sortait comme la note
// « s », du bruit affiché sur la fiche.
func TestMarquePlurielNEstPasUneNote(t *testing.T) {
	lu := verifie(t, "4 pavé(s) de saumon", "4 · pave · de · saumon")
	if lu.Note != "" {
		t.Errorf("note : %q, attendu vide", lu.Note)
	}
}

// Non-régression du retrait : une parenthèse qui porte autre chose qu'une
// marque de pluriel reste une note.
func TestLesParenthesesRestentDesNotes(t *testing.T) {
	lu := verifie(t, "1 gousse d'ail (facultatif)", "1 · gousse · d' · ail")
	if lu.Note != "facultatif" {
		t.Errorf("note : %q, attendu %q", lu.Note, "facultatif")
	}
	if !lu.Optionnel {
		t.Error("optionnel attendu")
	}
}

// « 250 g + 200 g de X » : les deux termes portent la même unité et l'aliment
// n'apparaît qu'après le dernier — la somme est licite, et les 200 g du second
// terme étaient purement perdus.
//
// L'aliment garde la casse de la source, ici « Coulis de framboises » : le
// moteur ne recase rien nulle part (le jeu de référence annote « Thym »,
// « Bouillon cube(s) »), et l'égalité du projet passe par `Normalise`.
func TestSecondTermeDUneAddition(t *testing.T) {
	lu := verifie(t, "250 gramme(s) + 200 gramme(s) de Coulis de framboises",
		"450 · gramme · de · Coulis de framboises")
	if lu.Note != "" {
		t.Errorf("note : %q, attendu vide", lu.Note)
	}
	if lu.Unite == nil || lu.Unite.Abrev != "g" {
		t.Errorf("abréviation : %q, attendu %q", lu.UniteCle(), "g")
	}
}

// Le garde-fou, et la raison d'être de la condition sur l'unité : « 2 oeufs »
// et « 1 jaune » sont deux aliments, pas deux termes. Les additionner rendrait
// trois oeufs.
func TestAdditionRefuseeQuandUnTermePorteUnAliment(t *testing.T) {
	lu := verifie(t, "2 oeufs + 1 jaune (pour dorer)", "2 ·  ·  · oeufs + 1 jaune")
	if lu.Note != "pour dorer" {
		t.Errorf("note : %q, attendu %q", lu.Note, "pour dorer")
	}
}

// L'autre moitié du garde-fou, celle que la passe de sabotage a trouvée
// découverte : deux termes bien formés, mais d'unités différentes, ne
// s'additionnent pas davantage. Sans la condition d'unité, « 250 g + 2
// cuillères à soupe » rendrait 252 cuillères à soupe.
func TestAdditionRefuseeQuandLesUnitesDifferent(t *testing.T) {
	verifie(t, "250 g + 2 cuillères à soupe de crème fraîche",
		"250 · gramme ·  · + 2 cuillères à soupe de crème fraîche")
}
