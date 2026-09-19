package moteur

// Le garde-fou : le jeu de référence annoté à la main, et le plancher inscrit
// dans son entête. Portage de `tests/test_testdata.py`.
//
// C'est le test qui voyage avec le moteur : il ne demande ni corpus, ni base,
// ni réseau — seulement les trois fichiers publiables du projet. Un
// contributeur qui ajoute une langue peut prouver que son pack fonctionne avec
// exactement ce test-là.

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestFormatDuJeu(t *testing.T) {
	exemple := "# Segmentation de référence.\n" +
		"# plancher: 0.500\n" +
		"# source\tbrut\tquantité\tunité\tpartitif\taliment\tnote\n" +
		"marmiton\t500 g de beurre\t500\tg\tde\tbeurre\t\n" +
		"750g\tThym\t\t\t\tThym\t\n"

	lignes, plancher := AnalyseJeu(exemple)
	if plancher != 0.5 {
		t.Errorf("plancher %g, attendu 0.5", plancher)
	}
	if len(lignes) != 2 {
		t.Fatalf("%d lignes, attendu 2", len(lignes))
	}
	if lignes[0].Unite != "g" {
		t.Errorf("unité %q, attendu %q", lignes[0].Unite, "g")
	}
	if lignes[1].Aliment != "Thym" {
		t.Errorf("aliment %q, attendu %q", lignes[1].Aliment, "Thym")
	}
}

func TestAccordCompareParLePack(t *testing.T) {
	// « càs » et « cuillères à soupe » sont la même unité : c'est la
	// segmentation qu'on mesure, pas l'orthographe de l'annotateur.
	p := packFR(t)
	lignes := []LigneRef{{
		Source: "x", Brut: "2 càs de sucre", Quantite: "2",
		Unite: "cuillères à soupe", Partitif: "de", Aliment: "sucre",
	}}
	part, desaccords := Accord(lignes, p, func(brut string) *Ingredient {
		return Lit(brut, p, nil)
	})
	if part != 1.0 || len(desaccords) != 0 {
		t.Errorf("accord %.3f, %d désaccords", part, len(desaccords))
	}
}

// lecteur rend le parser tel qu'il tourne pour de vrai : avec le lexique s'il
// est là. Sans ça, le chiffre inscrit dans `testdata/fr.txt` décrirait un
// parser que personne n'exécute — mesuré sans lexique, livré avec.
func lecteur(t *testing.T, p *Pack) func(string) *Ingredient {
	t.Helper()
	chemin := filepath.Join("data", "foods_fr.json")
	lexique, err := ChargeAliments(chemin, p)
	if os.IsNotExist(err) {
		t.Logf("%s absent : mesure sans lexique", chemin)
		return func(brut string) *Ingredient { return Lit(brut, p, nil) }
	}
	if err != nil {
		t.Fatalf("lexique : %v", err)
	}
	return func(brut string) *Ingredient { return Lit(brut, p, lexique) }
}

func TestJeuDeReference(t *testing.T) {
	chemin := filepath.Join("testdata", "fr.txt")
	contenu, err := os.ReadFile(chemin)
	if os.IsNotExist(err) {
		t.Skipf("%s pas encore annoté", chemin)
	}
	if err != nil {
		t.Fatal(err)
	}

	p := packFR(t)
	lignes, plancher := AnalyseJeu(string(contenu))
	if len(lignes) == 0 {
		t.Fatal("aucune ligne dans le jeu de référence")
	}
	for _, ligne := range lignes {
		if ligne.Brut == "" {
			t.Fatal("ligne brute vide dans le jeu de référence")
		}
	}

	part, desaccords := Accord(lignes, p, lecteur(t, p))
	if math.Round(part*1000)/1000 < plancher {
		detail := ""
		for i, d := range desaccords {
			if i >= 20 {
				break
			}
			detail += "\n    " + d.Brut + "\n      attendu  " + d.Attendu +
				"\n      lu       " + d.Lu
		}
		t.Errorf("accord tombé à %.1f %% (plancher %.1f %%) sur %d lignes%s",
			part*100, plancher*100, len(lignes), detail)
	}
	t.Logf("accord %.1f %% sur %d lignes (plancher %.1f %%)",
		part*100, len(lignes), plancher*100)
}

// ------------------------------------------- le taux de résolution

// L'entête porte deux planchers, et ils ne se confondent pas : l'un juge la
// découpe, l'autre la couverture du référentiel. Les deux mesures ne montent
// pas ensemble — compléter le lexique ne redécoupe rien —, donc elles se
// lisent et se tiennent séparément.
func TestLEnteteDistingueLesDeuxPlanchers(t *testing.T) {
	exemple := "# Segmentation de référence.\n" +
		"# plancher: 0.850\n" +
		"# plancher-resolution: 0.679\n" +
		"marmiton\t500 g de beurre\t500\tg\tde\tbeurre\t\n"

	lignes, accord := AnalyseJeu(exemple)
	if accord != 0.850 {
		t.Errorf("plancher d'accord %g, attendu 0.850", accord)
	}
	if resolution := PlancherResolution(exemple); resolution != 0.679 {
		t.Errorf("plancher de résolution %g, attendu 0.679", resolution)
	}
	if len(lignes) != 1 {
		t.Errorf("%d lignes, attendu 1 : les deux planchers sont des commentaires", len(lignes))
	}
}

// Resolution mesure le référentiel, pas la segmentation : elle part de
// l'aliment **annoté** — ce que l'humain a segmenté — et demande au lexique
// s'il tombe sur une entrée. Partir de ce que le parser lit mélangerait les
// deux : une ligne mal découpée ferait baisser un taux qui prétend juger le
// lexique.
//
// Pluriels et alias comptent : ce sont des écritures de la même entrée.
func TestResolutionCompteLesAlimentsQuiTombentSurUneEntree(t *testing.T) {
	p := packFR(t)
	lexique, err := LisAliments([]byte(lexiqueFactice), p)
	if err != nil {
		t.Fatalf("lecture du lexique : %v", err)
	}

	lignes := []LigneRef{
		{Source: "x", Brut: "1 tomate", Aliment: "tomate"},           // le nom
		{Source: "x", Brut: "3 tomates", Aliment: "tomates"},         // le pluriel
		{Source: "x", Brut: "1 oignon brun", Aliment: "oignon brun"}, // un alias
		{Source: "x", Brut: "2 courgettes", Aliment: "courgettes"},   // inconnue
		// Sans aliment annoté, il n'y a pas d'occurrence à résoudre : la
		// ligne ne compte ni au numérateur ni au dénominateur.
		{Source: "x", Brut: "sel et poivre", Aliment: ""},
	}

	part, inconnues := Resolution(lignes, p, lexique)
	if part != 0.75 {
		t.Errorf("taux %.3f, attendu 0.750 : 3 formes sur 4 occurrences", part)
	}
	if len(inconnues) != 1 || inconnues[0] != "courgettes" {
		t.Errorf("inconnues %q, attendu [courgettes]", inconnues)
	}

	// Sans lexique, rien ne se résout — et surtout rien ne panique.
	if part, inconnues := Resolution(lignes, p, nil); part != 0 || len(inconnues) != 4 {
		t.Errorf("sans lexique : taux %.3f et %d inconnues, attendu 0 et 4", part, len(inconnues))
	}
}

// Le garde-fou du référentiel, jumeau de TestJeuDeReference : le taux mesuré
// sur le jeu annoté ne redescend pas sous le plancher inscrit dans son entête.
// Le plancher ne se choisit pas, il se relève — c'est la valeur mesurée le jour
// où le lexique a été complété.
func TestResolutionDuJeuDeReference(t *testing.T) {
	chemin := filepath.Join("testdata", "fr.txt")
	contenu, err := os.ReadFile(chemin)
	if os.IsNotExist(err) {
		t.Skipf("%s pas encore annoté", chemin)
	}
	if err != nil {
		t.Fatal(err)
	}

	p := packFR(t)
	lexique, err := AlimentsFR(p)
	if err != nil {
		t.Fatalf("lexique embarqué : %v", err)
	}
	lignes, _ := AnalyseJeu(string(contenu))
	plancher := PlancherResolution(string(contenu))
	if plancher == 0 {
		t.Fatalf("aucun « # plancher-resolution: » dans l'entête de %s", chemin)
	}

	part, inconnues := Resolution(lignes, p, lexique)
	if math.Round(part*1000)/1000 < plancher {
		detail := ""
		for i, forme := range inconnues {
			if i >= 20 {
				break
			}
			detail += "\n    " + forme
		}
		t.Errorf("résolution tombée à %.1f %% (plancher %.1f %%) sur %d lignes ; sans entrée%s",
			part*100, plancher*100, len(lignes), detail)
	}
	t.Logf("résolution %.1f %% sur %d lignes (plancher %.1f %%)",
		part*100, len(lignes), plancher*100)
}
