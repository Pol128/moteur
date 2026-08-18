# moteur

Lit une ligne d'ingrédient écrite en français et rend ses parties : quantité,
unité, qualificatifs, aliment, note.

```go
pack, lexique, err := moteur.FR()
lu := moteur.Lit("2 à 3 gousses d'ail dégermées (facultatif)", pack, lexique)
// *lu.Quantite    = 2        lu.Partitif  = "d'"
// *lu.QuantiteMax = 3        lu.Aliment   = "ail dégermées"
// lu.UniteCle()   = "gousse" lu.Note      = "facultatif"
//                            lu.Optionnel = true
```

## Le module se suffit à lui-même

Le pack de langue (`lang/fr.toml`) et le lexique d'aliments
(`data/foods_fr.json`) sont **embarqués** avec `go:embed`. `go get` suffit :
rien à récupérer ni à tenir à jour à côté.

Pour un pack à soi — une autre langue, une variante locale du lexique —
`Charge` et `ChargeAliments` lisent depuis des fichiers, `Lis` et `LisAliments`
depuis des octets.

## Aucune règle de français n'est écrite en Go

Unités, partitifs, fractions, seuils de pluriel, formes irrégulières : tout vient
du pack. Le code ne connaît pas la langue, il applique ce que le pack déclare.
C'est ce qui rend une deuxième langue possible sans toucher au moteur —
et ce que vérifie `TestPackFactice`, qui fait tourner le parser sur une langue
inventée.

## Mesure

```sh
go test ./...                       # 38 tests
```

`TestJeuDeReference` fait tourner le parser sur `testdata/fr.txt`, 287 lignes
annotées à la main, tirées d'un corpus réel — fautes de frappe comprises. Le
plancher d'accord est inscrit dans le fichier lui-même : il ne peut pas baisser
sans que quelqu'un le change explicitement.

## En ligne de commande

```sh
go install github.com/Pol128/moteur/cmd/parse@latest
parse "500 g de beurre demi-sel"              # depuis n'importe quel répertoire
parse --filtre < lignes.txt                   # TSV, pour mesurer un corpus
parse --jeu testdata/fr.txt                   # l'accord contre le jeu annoté
```

Le binaire lit lui aussi le pack et le lexique embarqués : rien à installer à
côté. `--pack` et `--aliments` restent là pour en imposer d'autres, et
`--aliments ""` demande explicitement de lire sans lexique.

## Licence

Apache-2.0 — voir [LICENSE](LICENSE) et [NOTICE](NOTICE). Le lexique d'aliments
embarqué vient de MealieSync (MIT) ; sa mention voyage avec les données.
