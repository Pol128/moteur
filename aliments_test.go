package moteur

// Le lexique n'est plus un ensemble de formes : il indexe chaque forme vers
// l'entrée qui la porte, et `Lit` pose la forme canonique de cette entrée dans
// `Aliment`. Ce que ça sert, et qui ne s'obtient pas autrement : « 2 oignons »
// et « 3 oignon » doivent désigner la même entrée pour qu'une liste de courses,
// un calcul de calories ou une recherche par ingrédient tienne debout.

import "testing"

// Deux entrées suffisent à décrire le contrat : une qui porte un pluriel
// distinct, une qui porte un alias. Le lexique embarqué sert ensuite à vérifier
// que le contrat tient sur les données réelles.
const lexiqueFactice = `{"items":[
  {"name":"tomate","pluralName":"tomates","aliases":["tomate ronde"],"label":"Légumes"},
  {"name":"oignon jaune","pluralName":"oignons jaunes","aliases":["oignon brun"],"label":"Légumes"}
]}`

func TestLexiqueIndexeChaqueFormeVersSonEntree(t *testing.T) {
	p := packFR(t)
	lexique, err := LisAliments([]byte(lexiqueFactice), p)
	if err != nil {
		t.Fatalf("lecture du lexique : %v", err)
	}

	for _, cas := range []struct{ forme, nom, pluriel string }{
		{"tomate", "tomate", "tomates"},
		{"tomates", "tomate", "tomates"},
		{"tomate ronde", "tomate", "tomates"},
		{"oignon jaune", "oignon jaune", "oignons jaunes"},
		{"oignons jaunes", "oignon jaune", "oignons jaunes"},
		{"oignon brun", "oignon jaune", "oignons jaunes"},
	} {
		entree, trouve := lexique.Resout(p.Normalise(cas.forme))
		if !trouve {
			t.Errorf("« %s » ne se résout pas", cas.forme)
			continue
		}
		if entree.Nom != cas.nom || entree.Pluriel != cas.pluriel {
			t.Errorf("« %s » → %q / %q, attendu %q / %q",
				cas.forme, entree.Nom, entree.Pluriel, cas.nom, cas.pluriel)
		}
		if entree.Label != "Légumes" {
			t.Errorf("« %s » : label %q, attendu %q", cas.forme, entree.Label, "Légumes")
		}
		// L'appartenance ne bouge pas : `parse.go` ne consomme que celle-là.
		if !lexique.Contient(p.Normalise(cas.forme)) {
			t.Errorf("« %s » : Contient dit non", cas.forme)
		}
	}

	if _, trouve := lexique.Resout(p.Normalise("courgette")); trouve {
		t.Error("« courgette » se résout alors qu'elle n'est pas du lexique")
	}
	if lexique.Contient(p.Normalise("courgette")) {
		t.Error("« courgette » : Contient dit oui")
	}
	if lexique.Entrees != 2 {
		t.Errorf("%d entrées, attendu 2", lexique.Entrees)
	}
	if lexique.Formes() != 6 {
		t.Errorf("%d formes, attendu 6", lexique.Formes())
	}
}

func TestLitPoseLaFormeCanonique(t *testing.T) {
	p, lexique, err := FR()
	if err != nil {
		t.Fatalf("FR() : %v", err)
	}

	for _, cas := range []struct{ ligne, aliment, pourquoi string }{
		{"3 tomates", "tomate", "le pluriel de l'entrée « tomate »"},
		{"1 oignon brun", "oignon jaune", "un alias déclaré d'« oignon jaune »"},
		// Le lexique embarqué ne porte pas d'entrée « oignon » : une forme qui
		// ne se résout pas n'est pas réécrite. C'est la tâche de complétion du
		// référentiel qui comble ce manque, pas celle-ci.
		{"2 oignons", "oignons", "aucune entrée « oignon » au lexique"},
	} {
		lu := Lit(cas.ligne, p, lexique)
		if lu.Aliment != cas.aliment {
			t.Errorf("« %s » : aliment %q, attendu %q (%s)",
				cas.ligne, lu.Aliment, cas.aliment, cas.pourquoi)
		}
		if lu.Brut != cas.ligne {
			t.Errorf("« %s » : brut %q, la ligne d'origine ne doit jamais bouger",
				cas.ligne, lu.Brut)
		}
	}
}

// Le pluriel de l'entrée doit rester atteignable par l'appelant : c'est ce dont
// une fiche de recette a besoin pour accorder l'aliment à l'affichage. Une
// entrée résolue qui ne rendrait que son singulier ferait lire « 2 oignon ».
func TestLePlurielDeLEntreeResteAtteignable(t *testing.T) {
	p, lexique, err := FR()
	if err != nil {
		t.Fatalf("FR() : %v", err)
	}

	lu := Lit("3 tomates", p, lexique)
	entree, trouve := lexique.Resout(p.Normalise(lu.Aliment))
	if !trouve {
		t.Fatalf("l'aliment %q ne se résout pas", lu.Aliment)
	}
	if entree.Nom != "tomate" || entree.Pluriel != "tomates" {
		t.Errorf("entrée %q / %q, attendu %q / %q",
			entree.Nom, entree.Pluriel, "tomate", "tomates")
	}
}

// `Aliment` porte désormais l'entrée, pas la ligne. Ce que la ligne écrivait
// reste dans `AlimentTexte` — sans quoi la segmentation ne serait plus
// mesurable, cf. le test suivant.
func TestAlimentTexteGardeCeQueLaLigneEcrit(t *testing.T) {
	p, lexique, err := FR()
	if err != nil {
		t.Fatalf("FR() : %v", err)
	}

	lu := Lit("3 tomates", p, lexique)
	if lu.Aliment != "tomate" || lu.AlimentTexte != "tomates" {
		t.Errorf("aliment %q / texte %q, attendu %q / %q",
			lu.Aliment, lu.AlimentTexte, "tomate", "tomates")
	}

	// Sans lexique, il n'y a rien à résoudre : les deux disent la même chose.
	sans := Lit("3 tomates", p, nil)
	if sans.Aliment != "tomates" || sans.AlimentTexte != "tomates" {
		t.Errorf("sans lexique : aliment %q / texte %q, attendu %q des deux côtés",
			sans.Aliment, sans.AlimentTexte, "tomates")
	}
}

// Le jeu de référence annote ce que l'humain a segmenté — « tomates », au
// pluriel, comme la ligne l'écrit. La mesure d'accord porte sur cette
// segmentation, « pas [sur] l'orthographe de l'annotateur » (accord.go) : elle
// se fait donc sur `AlimentTexte`, et la résolution vers l'entrée lui est
// étrangère. Sans ça, le parser livré avec son lexique serait compté faux sur
// chaque ligne qu'il résout correctement.
func TestLAccordMesureLaSegmentationPasLaResolution(t *testing.T) {
	p, lexique, err := FR()
	if err != nil {
		t.Fatalf("FR() : %v", err)
	}

	attendu := LigneRef{Source: "x", Brut: "3 tomates", Quantite: "3", Aliment: "tomates"}
	lu := Lit(attendu.Brut, p, lexique)
	if juste, mesure := Compare(attendu, lu, p, false, false).Tout(); !juste || !mesure {
		t.Errorf("« %s » comptée fausse : lu %q (texte %q), annoté %q",
			attendu.Brut, lu.Aliment, lu.AlimentTexte, attendu.Aliment)
	}
}
