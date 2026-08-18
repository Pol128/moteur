package moteur

// Le lexique d'aliments — ce qu'aucune grammaire ne peut savoir.
// Portage de `crawler/aliments.py`.
//
// Le parser sait couper « 500 g de beurre » sans rien connaître du français :
// il lui suffit du pack de langue. Il bute sur une seule espèce de ligne, et
// elle est irréductible :
//
//	1 gousse d'ail          l'aliment est « ail »,     la gousse est l'unité
//	1 gousse de vanille     l'aliment est « gousse de vanille », d'un bloc
//
// Les deux lignes ont exactement la même forme. Ni la casse, ni la position, ni
// le voisinage ne les séparent — la différence est lexicale.
//
// Le lexique doit venir d'ailleurs que du corpus. Mesuré, et sans appel : un
// lexique construit depuis les aliments publiés par Jow et CuisineAZ, puis
// appliqué à Marmiton, fait *baisser* l'accord de 3,7 points — parce que
// CuisineAZ nomme ses aliments avec leur unité. Un lexique récolté sur les
// sites apprend leurs conventions, pas la langue.

import (
	"encoding/json"
	"os"
)

// SourceAliments décrit la provenance du lexique livré avec le moteur.
const (
	SourceAliments   = "https://github.com/Rouzax/MealieSync"
	LicenceAliments  = "MIT"
	CopyrightAliment = "Copyright (c) 2025 Rouzax"
)

// Lexique est un ensemble de formes normalisées, et rien de plus.
type Lexique struct {
	formes  map[string]bool
	Entrees int
}

// Contient satisfait Aliments.
func (l *Lexique) Contient(nom string) bool {
	if l == nil {
		return false
	}
	return l.formes[nom]
}

// Formes rend le nombre d'écritures reconnues.
func (l *Lexique) Formes() int {
	if l == nil {
		return 0
	}
	return len(l.formes)
}

type entreeAliment struct {
	Nom     string   `json:"name"`
	Pluriel string   `json:"pluralName"`
	Alias   []string `json:"aliases"`
	Label   string   `json:"label"`
}

// ChargeAliments lit le lexique et normalise ses formes avec le pack — sans
// quoi « Cœur d'artichaut » et « coeur d'artichaut » seraient deux entrées
// différentes et aucune ne répondrait.
func ChargeAliments(chemin string, p *Pack) (*Lexique, error) {
	contenu, err := os.ReadFile(chemin)
	if err != nil {
		return nil, err
	}
	var fichier struct {
		Items []entreeAliment `json:"items"`
	}
	if err := json.Unmarshal(contenu, &fichier); err != nil {
		return nil, err
	}
	lexique := &Lexique{formes: map[string]bool{}, Entrees: len(fichier.Items)}
	for _, item := range fichier.Items {
		// Le pluriel est retenu séparément et non déduit : « cœurs
		// d'artichaut » met la marque sur le premier mot, pas sur le dernier,
		// et aucune règle simple ne le devine.
		formes := append([]string{item.Nom, item.Pluriel}, item.Alias...)
		for _, brute := range formes {
			if forme := p.Normalise(brute); forme != "" {
				lexique.formes[forme] = true
			}
		}
	}
	return lexique, nil
}
