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
		packPartage, packErreur = Charge(filepath.Join("..", "lang", "fr.toml"))
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
