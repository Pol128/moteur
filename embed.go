package moteur

import _ "embed"

// Le pack de langue et le lexique d'aliments sont embarqués dans le module.
//
// Tranché le 19/08/2026 : le moteur marche seul. Un tiers qui écrit
// `go get github.com/Pol128/moteur` obtient un parser qui lit du français
// immédiatement, sans avoir à récupérer, installer et tenir à jour deux
// fichiers de données à côté. L'alternative — les données fournies par
// l'application appelante — déplaçait sur chaque appelant un travail qui n'a
// qu'une bonne réponse.
//
// Charge et ChargeAliments restent disponibles pour qui veut son propre pack :
// une autre langue, ou une variante locale du lexique.

//go:embed lang/fr.toml
var packFRTOML []byte

//go:embed data/foods_fr.json
var alimentsFRJSON []byte

// PackFR rend le pack français embarqué.
func PackFR() (*Pack, error) {
	return Lis(packFRTOML, "lang/fr.toml (embarqué)")
}

// AlimentsFR rend le lexique d'aliments embarqué, normalisé avec le pack donné.
func AlimentsFR(p *Pack) (*Lexique, error) {
	return LisAliments(alimentsFRJSON, p)
}

// FR rend le couple prêt à l'emploi — c'est le point d'entrée attendu de la
// plupart des appelants :
//
//	pack, lexique, err := moteur.FR()
//	ingredient := moteur.Lit("500 g de beurre demi-sel", pack, lexique)
func FR() (*Pack, *Lexique, error) {
	pack, err := PackFR()
	if err != nil {
		return nil, nil, err
	}
	lexique, err := AlimentsFR(pack)
	if err != nil {
		return nil, nil, err
	}
	return pack, lexique, nil
}
