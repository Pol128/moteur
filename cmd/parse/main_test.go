package main

import (
	"os"
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
