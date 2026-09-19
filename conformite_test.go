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
	"strings"
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

	// L'ensemble épinglé et le taux parlent du même parser : même lecteur,
	// même liste de désaccords. Le premier dit lesquelles, le second combien.
	epingles, err := ChargeDesaccords(filepath.Join("testdata", "desaccords.txt"))
	if err != nil {
		t.Fatal(err)
	}
	apparus, disparus := CompareDesaccords(epingles, BrutsDesaccords(desaccords))
	if len(apparus) > 0 || len(disparus) > 0 {
		t.Errorf("le jeu de référence a bougé — %d ligne(s) en désaccord nouveau, "+
			"%d ligne(s) qui n'y sont plus.\n  nouvelles :%s\n  corrigées :%s\n"+
			"Si le changement est voulu, régénérer :\n  %s",
			len(apparus), len(disparus), liste(apparus), liste(disparus),
			CommandeDesaccords)
	}

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

	// Sans occurrence du tout, le taux vaut 0 — pas NaN. La nuance n'est pas
	// cosmétique : NaN est incomparable, donc « part < plancher » est faux
	// quoi qu'il arrive, et le garde-fou du jeu annoté passerait au vert sur
	// un corpus vide, exactement quand il devrait crier.
	vide := []LigneRef{{Source: "x", Brut: "sel et poivre", Aliment: ""}}
	if part, inconnues := Resolution(vide, p, lexique); math.IsNaN(part) || part != 0 || inconnues != nil {
		t.Errorf("sans occurrence : taux %v et %d inconnues, attendu 0 et aucune", part, len(inconnues))
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

// ------------------------------------------- les désaccords épinglés

// Le taux dit combien de lignes sont fausses, jamais lesquelles. Une
// modification qui en corrige cinq et en casse cinq autres laisse 85,0 % et le
// test vert : c'est une non-régression du chiffre, pas des lignes. Le fichier
// épinglé ferme ce trou, et il se lit comme `fr.txt` — les lignes `#` et les
// lignes vides sont du commentaire.
func TestLectureDesDesaccordsEpingles(t *testing.T) {
	fichier := "# Désaccords épinglés.\n" +
		"# Régénérer : " + CommandeDesaccords + "\n" +
		"\n" +
		"140 g de farine type 55\n" +
		"140 g de farine type 55\n" +
		"Sel ou sel fin\n"

	lus := AnalyseDesaccords(fichier)
	attendus := []string{
		"140 g de farine type 55",
		"140 g de farine type 55",
		"Sel ou sel fin",
	}
	if len(lus) != len(attendus) {
		t.Fatalf("%d lignes lues, attendu %d : %q", len(lus), len(attendus), lus)
	}
	for i := range attendus {
		if lus[i] != attendus[i] {
			t.Errorf("ligne %d : %q, attendu %q", i, lus[i], attendus[i])
		}
	}
}

// Les deux sens, sur entrées fabriquées — ni `fr.txt`, ni l'état réel du
// parser. Une ligne qui se met à échouer doit être signalée ; une ligne qui
// cesse d'échouer aussi, sans quoi un fichier de manquements connus qu'on ne
// met jamais à jour redevient du bruit en trois mois.
func TestComparaisonDesDesaccordsDansLesDeuxSens(t *testing.T) {
	epingles := []string{"Sel ou sel fin", "Une pincée de sel"}

	// Rien n'a bougé : rien à dire, et l'ordre d'arrivée n'y change rien.
	apparus, disparus := CompareDesaccords(epingles, []string{"Une pincée de sel", "Sel ou sel fin"})
	if len(apparus) != 0 || len(disparus) != 0 {
		t.Errorf("listes égales : apparus %q, disparus %q", apparus, disparus)
	}

	// Une ligne aujourd'hui correcte se met à échouer.
	apparus, disparus = CompareDesaccords(epingles, append([]string{"3 tomates"}, epingles...))
	if len(apparus) != 1 || apparus[0] != "3 tomates" || len(disparus) != 0 {
		t.Errorf("ligne apparue : apparus %q, disparus %q", apparus, disparus)
	}

	// Une ligne épinglée cesse d'échouer : le fichier n'est plus à jour.
	apparus, disparus = CompareDesaccords(epingles, []string{"Sel ou sel fin"})
	if len(disparus) != 1 || disparus[0] != "Une pincée de sel" || len(apparus) != 0 {
		t.Errorf("ligne disparue : apparus %q, disparus %q", apparus, disparus)
	}
}

// `brut` n'est pas une clé unique : « 140 g de farine type 55 » revient trois
// fois dans `fr.txt`, et les 43 désaccords ne portent que 40 valeurs
// distinctes. La comparaison se fait donc avec multiplicité — sans quoi une
// régression faisant tomber cette ligne de 3 désaccords à 1 passerait
// inaperçue, exactement le trou qu'on vient boucher.
func TestComparaisonDesDesaccordsCompteLesDoublons(t *testing.T) {
	trois := []string{"140 g de farine", "140 g de farine", "140 g de farine"}

	apparus, disparus := CompareDesaccords(trois, trois[:1])
	if len(disparus) != 2 || len(apparus) != 0 {
		t.Errorf("3 désaccords tombés à 1 : apparus %q, disparus %q", apparus, disparus)
	}

	apparus, disparus = CompareDesaccords(trois[:1], trois)
	if len(apparus) != 2 || len(disparus) != 0 {
		t.Errorf("1 désaccord monté à 3 : apparus %q, disparus %q", apparus, disparus)
	}
}

// Un garde-fou qui disparaît avec son fichier ne garde rien : absent, il dit
// quoi lancer pour le produire plutôt que de se taire.
func TestFichierDesaccordsAbsentNommeLaRegeneration(t *testing.T) {
	_, err := ChargeDesaccords(filepath.Join(t.TempDir(), "desaccords.txt"))
	if err == nil {
		t.Fatal("fichier absent : aucune erreur")
	}
	if !strings.Contains(err.Error(), CommandeDesaccords) {
		t.Errorf("erreur %q, attendu la commande de régénération", err)
	}
}

// Ce que la régénération écrit doit être relu tel quel par le test : l'entête
// porte la commande, les lignes sont triées par ordre d'octets, et les
// doublons restent.
func TestTexteDesaccordsPorteSonEnteteEtSesDoublons(t *testing.T) {
	texte := TexteDesaccords([]Desaccord{
		{Brut: "Sel ou sel fin"},
		{Brut: "140 g de farine"},
		{Brut: "Sel ou sel fin"},
	})
	if !strings.Contains(texte, CommandeDesaccords) {
		t.Errorf("entête sans la commande de régénération :\n%s", texte)
	}
	if !strings.HasPrefix(texte, "#") {
		t.Errorf("pas d'entête en # :\n%s", texte)
	}

	relus := AnalyseDesaccords(texte)
	attendus := []string{"140 g de farine", "Sel ou sel fin", "Sel ou sel fin"}
	if len(relus) != len(attendus) {
		t.Fatalf("%d lignes relues, attendu %d : %q", len(relus), len(attendus), relus)
	}
	for i := range attendus {
		if relus[i] != attendus[i] {
			t.Errorf("ligne %d : %q, attendu %q", i, relus[i], attendus[i])
		}
	}

	// Régénérer deux fois de suite ne doit rien changer au fichier.
	if encore := TexteDesaccords([]Desaccord{{Brut: "140 g de farine"},
		{Brut: "Sel ou sel fin"}, {Brut: "Sel ou sel fin"}}); encore != texte {
		t.Error("la régénération n'est pas idempotente")
	}
}

// liste met une ligne par entrée, tronquée à vingt, pour un message d'erreur
// qui tient à l'écran.
func liste(bruts []string) string {
	if len(bruts) == 0 {
		return " aucune"
	}
	detail := ""
	for i, brut := range bruts {
		if i >= 20 {
			detail += "\n    …"
			break
		}
		detail += "\n    " + brut
	}
	return detail
}
