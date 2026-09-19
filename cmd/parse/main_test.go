package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Pol128/moteur"
)

// La régression que ce test ferme : un binaire installé par `go install` tourne
// dans un répertoire quelconque, sans lang/ ni data/ à côté. Tant que les
// options portaient des chemins relatifs par défaut, il échouait sur
// « open lang/fr.toml: no such file or directory ».
func TestParDefautToutVientDuModule(t *testing.T) {
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}

	p, err := choisitPack("")
	if err != nil {
		t.Fatalf("pack par défaut : %v", err)
	}
	aliments, err := choisitAliments(embarque, p)
	if err != nil {
		t.Fatalf("lexique par défaut : %v", err)
	}
	if aliments == nil {
		t.Fatal("lexique par défaut absent")
	}
}

// La forge appelle `--aliments ""` pour mesurer la lecture sans lexique : ce
// contrat-là ne doit pas bouger.
func TestChaineVideDemandeAucunLexique(t *testing.T) {
	p, err := choisitPack("")
	if err != nil {
		t.Fatal(err)
	}
	aliments, err := choisitAliments("", p)
	if err != nil {
		t.Fatal(err)
	}
	if aliments != nil {
		t.Error("lexique chargé alors qu'on demandait de s'en passer")
	}
}

// `Aliment` porte désormais la forme canonique de l'entrée, et `--compare`
// juge `egal_aliment` et `meme_span` sur `AlimentTexte`. Tant que le TSV
// n'émettait que `aliment`, une ligne se contredisait elle-même :
// « 3 tomates » annotée « tomates » sortait `aliment=tomate` avec
// `egal_aliment=1`, et rien dans la ligne ne rendait la valeur réellement
// comparée. Les deux voyagent donc, chacune dans sa colonne.
func TestLeTSVPorteLaCanoniqueEtLeTexteCompare(t *testing.T) {
	p, err := choisitPack("")
	if err != nil {
		t.Fatalf("pack par défaut : %v", err)
	}
	aliments, err := choisitAliments(embarque, p)
	if err != nil {
		t.Fatalf("lexique par défaut : %v", err)
	}

	champs := champsLus(moteur.Lit("3 tomates", p, aliments))
	if len(champs) != len(colonnes) {
		t.Fatalf("%d champs pour %d colonnes", len(champs), len(colonnes))
	}

	valeur := func(colonne string) string {
		for i, nom := range colonnes {
			if nom == colonne {
				return champs[i]
			}
		}
		t.Fatalf("colonne %q absente de %v", colonne, colonnes)
		return ""
	}

	if got := valeur("aliment"); got != "tomate" {
		t.Errorf("aliment = %q, attendu %q : c'est l'entrée du lexique", got, "tomate")
	}
	if got := valeur("aliment_texte"); got != "tomates" {
		t.Errorf("aliment_texte = %q, attendu %q : c'est ce que la ligne écrit, "+
			"et c'est sur lui que porte egal_aliment", got, "tomates")
	}
}

// `--desaccords` épingle dans un fichier les lignes que le parser ne lit pas
// comme l'annotateur : c'est de cette commande que part `testdata/desaccords.txt`,
// et c'est elle que le test du jeu de référence nomme quand l'ensemble a bougé.
//
// Elle ne change rien à ce que `--jeu` faisait déjà — ni l'affichage, ni le
// code de sortie. Le plancher reste seul maître du verdict.
func TestDesaccordsEcritLaListeSansChangerLeVerdict(t *testing.T) {
	p, err := choisitPack("")
	if err != nil {
		t.Fatal(err)
	}
	aliments, err := choisitAliments(embarque, p)
	if err != nil {
		t.Fatal(err)
	}

	// Deux lignes annotées faux, une juste : l'ordre du fichier est celui de
	// `fr.txt`, celui du résultat est trié, et le doublon reste.
	lignes := "x\t2 càs de sucre\t2\tcuillères à soupe\tde\tsucre\t\n" +
		"x\t500 g de beurre\t1\tg\tde\tbeurre\t\n" +
		"x\t500 g de beurre\t1\tg\tde\tbeurre\t\n"
	jeu := "# plancher: 0.000\n" + lignes

	dossier := t.TempDir()
	chemin := filepath.Join(dossier, "fr.txt")
	if err := os.WriteFile(chemin, []byte(jeu), 0o644); err != nil {
		t.Fatal(err)
	}
	sortie := filepath.Join(dossier, "desaccords.txt")

	if code := mesureJeu(chemin, p, aliments, sortie); code != 0 {
		t.Errorf("code de sortie %d, attendu 0 : le plancher est tenu", code)
	}
	contenu, err := os.ReadFile(sortie)
	if err != nil {
		t.Fatal(err)
	}
	bruts := moteur.AnalyseDesaccords(string(contenu))
	if len(bruts) != 2 || bruts[0] != "500 g de beurre" || bruts[1] != "500 g de beurre" {
		t.Errorf("désaccords écrits %q, attendu « 500 g de beurre » deux fois", bruts)
	}

	// Le drapeau ne rattrape pas un plancher manqué : le fichier est écrit,
	// et le code de sortie reste celui du verdict.
	haut := "# plancher: 1.000\n" + lignes
	if err := os.WriteFile(chemin, []byte(haut), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := mesureJeu(chemin, p, aliments, sortie); code != 1 {
		t.Errorf("code de sortie %d, attendu 1 : le plancher n'est pas tenu", code)
	}

	// Sans le drapeau, rien n'est écrit.
	absent := filepath.Join(dossier, "rien.txt")
	mesureJeu(chemin, p, aliments, "")
	if _, err := os.Stat(absent); !os.IsNotExist(err) {
		t.Errorf("fichier écrit sans --desaccords : %v", err)
	}
}
