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

// Une forme que le lexique ne connaît pas n'est pas réécrite : le parser ne
// devine pas de canonique. L'invariante se vérifie sur le lexique factice, et
// non sur un trou du lexique embarqué — un référentiel se complète, et un test
// épinglé sur ce qui lui manque casse le jour où on le comble.
func TestUneFormeInconnueNestPasReecrite(t *testing.T) {
	p := packFR(t)
	lexique, err := LisAliments([]byte(lexiqueFactice), p)
	if err != nil {
		t.Fatalf("lecture du lexique : %v", err)
	}

	lu := Lit("2 courgettes", p, lexique)
	if lu.Aliment != "courgettes" {
		t.Errorf("aliment %q, attendu %q : « courgette » n'est pas du lexique",
			lu.Aliment, "courgettes")
	}
	if lu.AlimentTexte != "courgettes" {
		t.Errorf("texte %q, attendu %q", lu.AlimentTexte, "courgettes")
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

// Deux entrées peuvent revendiquer la même forme. Tant que l'index ne portait
// qu'un booléen, la collision était sans effet : « connue » reste « connue ».
// Maintenant qu'il porte l'entrée, celle qui gagne emporte le nom canonique —
// et l'alias d'une entrée tardive détournerait « ail » vers « ail des ours ».
//
// La règle : une forme n'est reprise que par un titre strictement plus fort.
// Un nom bat un pluriel, un pluriel bat un alias, et à titre égal la première
// entrée lue garde sa forme.
const lexiqueEnCollision = `{"items":[
  {"name":"ail","pluralName":"aulx","aliases":[],"label":"Légumes"},
  {"name":"ail des ours","pluralName":"ails des ours","aliases":["ail"],"label":"Légumes"},
  {"name":"ciboule","pluralName":"ciboules","aliases":["ciboulette"],"label":"Légumes"},
  {"name":"ciboulette","pluralName":"ciboulettes","aliases":[],"label":"Herbes"}
]}`

func TestUneFormeNestReprisQueParUnTitrePlusFort(t *testing.T) {
	p := packFR(t)
	lexique, err := LisAliments([]byte(lexiqueEnCollision), p)
	if err != nil {
		t.Fatalf("lecture du lexique : %v", err)
	}

	for _, cas := range []struct{ forme, nom, pourquoi string }{
		// L'alias d'« ail des ours » arrive après, il ne reprend pas le nom.
		{"ail", "ail", "« ail » est le nom d'une entrée, pas un alias"},
		{"ail des ours", "ail des ours", "son propre nom lui reste"},
		// L'alias de « ciboule » arrive avant le nom d'« ciboulette » : c'est
		// le nom qui l'emporte, quel que soit l'ordre de lecture.
		{"ciboulette", "ciboulette", "« ciboulette » est un nom, l'alias cède"},
		{"ciboule", "ciboule", "son propre nom lui reste"},
	} {
		entree, trouve := lexique.Resout(p.Normalise(cas.forme))
		if !trouve {
			t.Errorf("« %s » ne se résout pas", cas.forme)
			continue
		}
		if entree.Nom != cas.nom {
			t.Errorf("« %s » → %q, attendu %q (%s)",
				cas.forme, entree.Nom, cas.nom, cas.pourquoi)
		}
	}

	// La collision ne perd aucune forme : l'appartenance répond comme avant.
	for _, forme := range []string{"ail", "aulx", "ail des ours", "ails des ours",
		"ciboule", "ciboules", "ciboulette", "ciboulettes"} {
		if !lexique.Contient(p.Normalise(forme)) {
			t.Errorf("« %s » : Contient dit non", forme)
		}
	}
}

// Les aliments les plus courants du corpus doivent être des entrées, pas des
// formes que le parser laisse passer faute de les connaître. Le lexique
// embarqué connaissait « oignon jaune », « oignon rouge » et « oignon vert »,
// mais pas « oignon » — l'aliment le plus fréquent du jeu annoté.
//
// Le test regarde les deux bouts, et c'est nécessaire : `Aliment` rend le
// texte de la ligne quand rien ne se résout, donc « farine » y apparaîtrait
// même sans entrée au référentiel. C'est la résolution qui est vérifiée ici,
// pas la recopie.
func TestLesAlimentsDeBaseDuCorpusSeResolvent(t *testing.T) {
	p, lexique, err := FR()
	if err != nil {
		t.Fatalf("FR() : %v", err)
	}

	for _, cas := range []struct{ ligne, aliment string }{
		{"2 oignons", "oignon"},
		{"200 g de farine", "farine"},
		{"20 g de menthe", "menthe"},
		{"1 cuillère à café de curcuma", "curcuma"},
		{"1 cuillère à café d'origan", "origan"},
	} {
		entree, trouve := lexique.Resout(p.Normalise(cas.aliment))
		if !trouve {
			t.Errorf("« %s » n'est pas une entrée du référentiel", cas.aliment)
			continue
		}
		if entree.Nom != cas.aliment {
			t.Errorf("« %s » se résout vers %q, attendu %q",
				cas.aliment, entree.Nom, cas.aliment)
		}
		if lu := Lit(cas.ligne, p, lexique); lu.Aliment != cas.aliment {
			t.Errorf("« %s » : aliment %q, attendu %q",
				cas.ligne, lu.Aliment, cas.aliment)
		}
	}
}
