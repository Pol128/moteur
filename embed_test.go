package moteur

import "testing"

// Ce test est la raison d'être de l'extraction : il n'ouvre aucun fichier. S'il
// passe, un tiers qui importe le module obtient un parser qui lit du français
// sans rien installer à côté.
func TestLeModuleLitDuFrancaisSansFichierExterne(t *testing.T) {
	pack, lexique, err := FR()
	if err != nil {
		t.Fatalf("FR() : %v", err)
	}
	if lexique.Entrees == 0 {
		t.Fatal("lexique vide")
	}

	lu := Lit("500 g de beurre demi-sel", pack, lexique)
	if lu == nil {
		t.Fatal("ligne non lue")
	}
	if lu.Quantite == nil || *lu.Quantite != 500 {
		t.Errorf("quantité = %v, attendu 500", lu.Quantite)
	}
	if lu.UniteCle() != "gramme" {
		t.Errorf("unité = %q, attendu gramme", lu.UniteCle())
	}
	if lu.Aliment == "" {
		t.Error("aliment vide")
	}
}
