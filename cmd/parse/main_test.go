package main

import (
	"os"
	"testing"
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
