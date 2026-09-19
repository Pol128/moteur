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

// Entree est ce que le lexique connaît d'un aliment : sa forme canonique, son
// pluriel, et la famille qu'il lui donne.
//
// Le pluriel voyage avec le nom parce qu'il ne se déduit pas — « cœurs
// d'artichaut » met la marque sur le premier mot — et parce qu'un appelant qui
// affiche « 2 oignon » a perdu au change ce qu'il gagnait à normaliser.
type Entree struct {
	Nom     string
	Pluriel string
	Label   string
}

// Lexique indexe chaque forme normalisée vers l'entrée qui la porte : le nom,
// le pluriel et chacun des alias mènent à la même Entree.
type Lexique struct {
	formes     map[string]Entree
	categories map[string]int
	Entrees    int
}

// Contient satisfait Aliments.
func (l *Lexique) Contient(nom string) bool {
	if l == nil {
		return false
	}
	_, connue := l.formes[nom]
	return connue
}

// Resout satisfait Resolveur : il dit vers quelle entrée une forme normalisée
// se résout. C'est ce que `Contient` ne pouvait pas dire — il savait qu'une
// forme était connue, pas ce qu'elle désignait.
func (l *Lexique) Resout(nomNormalise string) (Entree, bool) {
	if l == nil {
		return Entree{}, false
	}
	entree, trouvee := l.formes[nomNormalise]
	return entree, trouvee
}

// ParCategorie rend le nombre d'entrées que le lexique porte dans chaque
// catégorie — le champ `label` des données.
//
// Le décompte se tient au chargement, sur les entrées, et non en parcourant
// l'index des formes : le nom, le pluriel et chaque alias y mènent à la même
// Entree, et les compter surcompterait chaque catégorie dans la proportion de
// ses alias.
//
// Une entrée sans label n'a pas de catégorie et n'en crée pas une vide.
func (l *Lexique) ParCategorie() map[string]int {
	if l == nil {
		return nil
	}
	// Une copie : le décompte est une vue, pas l'état interne.
	comptes := make(map[string]int, len(l.categories))
	for categorie, compte := range l.categories {
		comptes[categorie] = compte
	}
	return comptes
}

// Formes rend le nombre d'écritures reconnues.
func (l *Lexique) Formes() int {
	if l == nil {
		return 0
	}
	return len(l.formes)
}

// titreForme dit à quel titre une entrée revendique une forme. Deux entrées
// peuvent revendiquer la même : le lexique embarqué n'en a aucun cas, mais rien
// n'interdit à un appelant de fournir le sien.
//
// Tant que l'index ne portait qu'un booléen, la collision était sans
// conséquence — « connue » reste « connue ». Maintenant qu'il porte l'entrée,
// celle qui gagne emporte le nom canonique : le dernier écrit détournerait
// « ail » vers « ail des ours » dès qu'une entrée tardive le déclare en alias.
//
// Une forme n'est donc reprise que par un titre strictement plus fort, et à
// titre égal la première entrée lue la garde.
type titreForme int

const (
	titreNom titreForme = iota
	titrePluriel
	titreAlias
)

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
	return LisAliments(contenu, p)
}

// LisAliments fait le même travail depuis un lexique déjà en mémoire — celui
// que le module embarque, notamment.
func LisAliments(contenu []byte, p *Pack) (*Lexique, error) {
	var fichier struct {
		Items []entreeAliment `json:"items"`
	}
	if err := json.Unmarshal(contenu, &fichier); err != nil {
		return nil, err
	}
	lexique := &Lexique{
		formes:     map[string]Entree{},
		categories: map[string]int{},
		Entrees:    len(fichier.Items),
	}
	titres := map[string]titreForme{}
	for _, item := range fichier.Items {
		// Le pluriel est retenu séparément et non déduit : « cœurs
		// d'artichaut » met la marque sur le premier mot, pas sur le dernier,
		// et aucune règle simple ne le devine.
		entree := Entree{Nom: item.Nom, Pluriel: item.Pluriel, Label: item.Label}
		if item.Label != "" {
			lexique.categories[item.Label]++
		}
		poser := func(brute string, titre titreForme) {
			forme := p.Normalise(brute)
			if forme == "" {
				return
			}
			if pris, deja := titres[forme]; deja && pris <= titre {
				return
			}
			lexique.formes[forme] = entree
			titres[forme] = titre
		}
		poser(item.Nom, titreNom)
		poser(item.Pluriel, titrePluriel)
		for _, alias := range item.Alias {
			poser(alias, titreAlias)
		}
	}
	return lexique, nil
}
