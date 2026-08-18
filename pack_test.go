package moteur

// Portage de `tests/test_langpack.py`.
//
// Les cas sont tirés du corpus, pas inventés : « CaS » et « c à s » existent
// tels quels chez ptitchef, « 4 pavé(s) » chez CuisineAZ, « 1 demi d'oignon »
// chez Marmiton. Le reste vérifie que le moteur ne sait rien du français — un
// pack factice suffit à le faire changer d'avis.

import "testing"

func TestNormalisation(t *testing.T) {
	p := packFR(t)
	for _, cas := range []struct{ a, b string }{
		{"CàS", "càs"},
		{"Pincée", "pincee"},
		{"d’ail", "d'ail"},
		// CuisineAZ écrit « 4 pavé(s) » : 24 000 lignes en dépendent.
		{"pavé(s)", "pavé"},
		{"cœur", "coeur"},
	} {
		if p.Normalise(cas.a) != p.Normalise(cas.b) {
			t.Errorf("normalise(%q) = %q ≠ normalise(%q) = %q",
				cas.a, p.Normalise(cas.a), cas.b, p.Normalise(cas.b))
		}
	}
}

func TestUniteMultiMots(t *testing.T) {
	// Les quatre mots qui mettent Tandoor par terre.
	p := packFR(t)
	for _, forme := range []string{
		"cuillère à soupe", "cuillères à soupe", "c. à s.", "càs",
		"CS", "CaS", "c à s", "cuil. à soupe", "c. à table",
	} {
		lue := p.LireUnite(forme)
		if lue == nil {
			t.Errorf("« %s » : non reconnue", forme)
			continue
		}
		if lue.Unite.Cle != "cuillere_a_soupe" {
			t.Errorf("« %s » : unité %s", forme, lue.Unite.Cle)
		}
	}
}

func TestQualificatifsDecolles(t *testing.T) {
	lue := packFR(t).LireUnite("grosses cuillères à soupe bombées")
	if lue == nil {
		t.Fatal("« grosses cuillères à soupe bombées » : non reconnue")
	}
	if lue.Unite.Cle != "cuillere_a_soupe" || lue.Facteur != 1.0 || len(lue.Qualificatifs) != 2 {
		t.Errorf("unité %s, facteur %g, qualificatifs %v",
			lue.Unite.Cle, lue.Facteur, lue.Qualificatifs)
	}
}

func TestMultiplicateurDemi(t *testing.T) {
	lue := packFR(t).LireUnite("demi litre")
	if lue == nil || lue.Unite.Cle != "litre" || lue.Facteur != 0.5 {
		t.Fatalf("« demi litre » : %+v", lue)
	}
}

func TestDemiSeulEstUneUnite(t *testing.T) {
	// « 1 demi d'oignon » : Marmiton publie « demi » comme unité. Une forme
	// reconnue telle quelle gagne sur le décollage — sans cette règle, « demi »
	// se réduirait à un facteur sans unité.
	lue := packFR(t).LireUnite("demi")
	if lue == nil || lue.Unite.Cle != "moitie" || lue.Facteur != 1.0 {
		t.Fatalf("« demi » : %+v", lue)
	}
}

func TestUniteInconnue(t *testing.T) {
	if lue := packFR(t).LireUnite("bouteille de bordeaux 1998"); lue != nil {
		t.Errorf("attendu aucune unité, obtenu %s", lue.Unite.Cle)
	}
}

func TestDimensionsEtConversion(t *testing.T) {
	p := packFR(t)
	if base := p.Unites["kilogramme"].Base; base == nil || *base != 1000 {
		t.Errorf("kilogramme : base %v", base)
	}
	if !p.Unites["litre"].Convertible() {
		t.Error("litre devrait être convertible")
	}
	if p.Unites["pincee"].Convertible() {
		t.Error("pincée ne devrait pas être convertible")
	}
	// Un verre n'a pas de contenance connue : rien n'est inventé.
	if p.Unites["verre"].Base != nil {
		t.Error("verre ne devrait pas avoir de base")
	}
}

func TestAccordEnNombre(t *testing.T) {
	p := packFR(t)
	cas := p.Unites["cuillere_a_soupe"]
	for _, essai := range []struct {
		valeur  float64
		attendu string
	}{
		{1, "cuillère à soupe"},
		{1.5, "cuillère à soupe"},
		{2, "cuillères à soupe"},
	} {
		if obtenu := p.Accorde(cas, &essai.valeur); obtenu != essai.attendu {
			t.Errorf("accorde(%g) = %q, attendu %q", essai.valeur, obtenu, essai.attendu)
		}
	}
}

func TestQuantiteLitteraleDuPack(t *testing.T) {
	p := packFR(t)
	for _, cas := range []struct {
		texte  string
		valeur float64
	}{{"une", 1}, {"demi", 0.5}} {
		q := p.Quantite(cas.texte)
		if q == nil || q.Valeur != cas.valeur {
			t.Errorf("quantite(%q) = %+v, attendu %g", cas.texte, q, cas.valeur)
		}
	}
}

func TestFractionTypographique(t *testing.T) {
	// NFKD décompose « ½ » en « 1⁄2 » avec une barre U+2044 : la table des
	// fractions est donc consultée avant normalisation.
	if q := packFR(t).Quantite("½"); q == nil || q.Valeur != 0.5 {
		t.Errorf("quantite(\"½\") = %+v", q)
	}
}

func TestNombresEcrits(t *testing.T) {
	p := packFR(t)
	for _, cas := range []struct {
		texte  string
		valeur float64
	}{{"1/2", 0.5}, {"1 1/2", 1.5}, {"2,5", 2.5}} {
		valeur, ok := p.Nombre(cas.texte)
		if !ok || valeur != cas.valeur {
			t.Errorf("nombre(%q) = %g, %v", cas.texte, valeur, ok)
		}
	}
	if _, ok := p.Nombre("beurre"); ok {
		t.Error("« beurre » n'est pas un nombre")
	}
}

func TestIndicativeMarqueeApproximative(t *testing.T) {
	q := packFR(t).Quantite("quelques")
	if q == nil || q.Valeur != 3 || !q.Approximative {
		t.Errorf("quantite(\"quelques\") = %+v", q)
	}
}

func TestIndefinie(t *testing.T) {
	p := packFR(t)
	if !p.EstIndefinie("un peu") || !p.EstIndefinie("À VOLONTÉ") {
		t.Error("« un peu » et « À VOLONTÉ » sont des quantités indéfinies")
	}
	if p.EstIndefinie("500") {
		t.Error("« 500 » n'est pas une quantité indéfinie")
	}
}

func TestPartitifPlusLongDAbord(t *testing.T) {
	p := packFR(t)
	for _, cas := range []struct{ texte, attendu string }{
		{"de la crème", "de la"},
		{"de beurre", "de"},
		{"d'ail", "d'"},
	} {
		if obtenu := p.Partitif(cas.texte); obtenu != cas.attendu {
			t.Errorf("partitif(%q) = %q, attendu %q", cas.texte, obtenu, cas.attendu)
		}
	}
}

func TestPasDeFauxPositifSurUnMotQuiCommenceParDe(t *testing.T) {
	// « des » ne doit pas être vu dans « dés de jambon ».
	p := packFR(t)
	for _, texte := range []string{"dés", "demi-sel", "beurre"} {
		if obtenu := p.Partitif(texte); obtenu != "" {
			t.Errorf("partitif(%q) = %q, attendu aucun", texte, obtenu)
		}
	}
}

// ------------------------------------------- le moteur ne connaît aucune langue

func packFactice(t *testing.T) *Pack {
	t.Helper()
	p, err := Construit(map[string]any{
		"pack":         map[string]any{"langue": "xx", "version": 3},
		"flexion":      map[string]any{"seuil_pluriel": 1},
		"prepositions": map[string]any{"partitifs": []any{"of"}},
		"quantites":    map[string]any{"litterales": map[string]any{"one": 1}},
		"unites": map[string]any{"blob": map[string]any{
			"dimension": "compte", "singulier": "blob", "pluriel": "blobz",
			"formes": []any{"blob", "blobz"},
		}},
	})
	if err != nil {
		t.Fatalf("pack factice : %v", err)
	}
	return p
}

func TestPackFactice(t *testing.T) {
	p := packFactice(t)
	if p.Langue != "xx" {
		t.Errorf("langue %q", p.Langue)
	}
	if obtenu := p.Partitif("of butter"); obtenu != "of" {
		t.Errorf("partitif(\"of butter\") = %q", obtenu)
	}
	if q := p.Quantite("one"); q == nil || q.Valeur != 1 {
		t.Errorf("quantite(\"one\") = %+v", q)
	}
	if lue := p.LireUnite("BLOBZ"); lue == nil || lue.Unite.Cle != "blob" {
		t.Errorf("lireUnite(\"BLOBZ\") = %+v", lue)
	}
}

func TestSeuilDePlurielVientDuPack(t *testing.T) {
	p := packFactice(t)
	une := 1.0
	if obtenu := p.Accorde(p.Unites["blob"], &une); obtenu != "blobz" {
		t.Errorf("accorde(1) = %q, attendu %q (seuil 1, pas 2)", obtenu, "blobz")
	}
}

func TestDeuxUnitesNePeuventRevendiquerLaMemeForme(t *testing.T) {
	// Sinon la lecture dépendrait de l'ordre du fichier.
	_, err := Construit(map[string]any{"unites": map[string]any{
		"a": map[string]any{"formes": []any{"cl"}},
		"b": map[string]any{"formes": []any{"CL"}},
	}})
	if err == nil {
		t.Error("deux unités revendiquant « cl » doivent être refusées")
	}
}
