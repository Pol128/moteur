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

## La forme canonique de l'aliment

Le lexique n'est pas qu'une liste de formes connues : il indexe chacune d'elles
— le nom, le pluriel, chaque alias — vers l'entrée qui la porte. `Lit` s'en
sert pour poser dans `Aliment` la forme canonique de cette entrée, quand la
ligne s'y résout :

```go
pack, lexique, err := moteur.FR()
moteur.Lit("3 tomates", pack, lexique).Aliment      // « tomate »
moteur.Lit("1 oignon brun", pack, lexique).Aliment  // « oignon jaune » — alias déclaré
moteur.Lit("2 oignons", pack, lexique).Aliment      // « oignon » — le pluriel de l'entrée
```

C'est ce qui permet de rapprocher « 2 oignons » et « 3 oignon » : liste de
courses, variation du nombre de parts, calcul des calories, recherche de
recettes à partir de ce qu'on a — toutes supposent que les deux désignent la
même entrée.

Une forme que le lexique ne connaît pas n'est **jamais** réécrite : le parser
ne devine pas de canonique. Et ce que la ligne écrivait reste dans
`AlimentTexte`, résolution ou pas. `Brut`, lui, ne bouge jamais.

Pour accorder l'aliment à l'affichage, l'entrée se relit en entier — elle porte
le pluriel, qui ne se déduit pas :

```go
entree, connue := lexique.Resout(pack.Normalise("tomates"))
// entree.Nom = "tomate"   entree.Pluriel = "tomates"   entree.Label = "Légumes"
```

`Contient` répond comme avant, et l'interface `Aliments` que consomme le parser
ne bouge pas : un lexique à soi qui ne sait dire que l'appartenance continue de
marcher, et `Lit` laisse alors l'aliment tel que la ligne l'écrit.

## Ce module est la seule implémentation du parser

Il a existé une seconde implémentation, en Python, dans la forge privée qui
produit le corpus et le pack. Elle servait à itérer vite sur les règles, et les
deux étaient réputées tenues en phase par une mesure croisée que rien ne
lançait automatiquement.

**Décidé le 19/09/2026 : toute règle de français nouvelle s'écrit ici, et nulle
part ailleurs.** La version Python est gelée — elle ne reçoit plus de règle et
ne sert plus que d'oracle de test là-bas. Le prix est assumé : mettre au point
une règle demande un aller-retour Go et une publication du module, là où le
Python répondait en trois secondes.

Ce que ça garantit à qui importe ce module : ce qu'il lit est l'implémentation
maintenue, pas une copie qui suit de loin.

## Aucune règle de français n'est écrite en Go

Unités, partitifs, fractions, seuils de pluriel, formes irrégulières : tout vient
du pack. Le code ne connaît pas la langue, il applique ce que le pack déclare.
C'est ce qui rend une deuxième langue possible sans toucher au moteur —
et ce que vérifie `TestPackFactice`, qui fait tourner le parser sur une langue
inventée.

## D'où viennent le pack, le lexique et le jeu annoté

Les trois fichiers publiés ici — `lang/fr.toml`, `data/foods_fr.json`,
`testdata/fr.txt` — sont produits au contact d'un corpus de recettes qui, lui,
ne quitte pas la forge : c'est elle qui récolte les pages, mesure ce que le
pack couvre et annote le jeu de référence à la main. Ce dépôt en embarque un
**instantané**, republié à chaque version.

Un consommateur du module n'a rien à en savoir : `go get` suffit. Mais le
plancher d'accord vaut pour l'instantané embarqué, et se remesure quand le jeu
grossit.

## Mesure

```sh
go test ./...                       # 43 tests
```

`TestJeuDeReference` fait tourner le parser sur `testdata/fr.txt`, 287 lignes
annotées à la main, tirées d'un corpus réel — fautes de frappe comprises. Le
plancher d'accord est inscrit dans le fichier lui-même : il ne peut pas baisser
sans que quelqu'un le change explicitement.

Ce qu'il mesure est la **segmentation**, pas la résolution : la comparaison
porte sur `AlimentTexte`, ce que la ligne écrit et ce que l'annotateur a relu.
La forme canonique se vérifie ailleurs, sur les entrées du lexique.

`TestResolutionDuJeuDeReference` mesure l'autre moitié : la part des aliments
annotés qui tombent sur une entrée du lexique, pluriels et alias compris. Elle
ne juge pas la découpe mais la couverture du référentiel, elle a son propre
plancher dans le même entête — `# plancher-resolution:` —, et elle monte avec
le lexique. Le référentiel ne prétend pas à l'exhaustivité : il se complète au
fil des manques constatés, et le plancher interdit seulement de redescendre.

Ces deux mesures sont agrégées, et c'est leur angle mort : le lexique pourrait
perdre la totalité de ses « Poissons et fruits de mer » sans qu'elles bronchent,
tant que le total tient. `TestPlanchersParCategorie` tient la distribution —
un plancher par catégorie, dans `testdata/categories_fr.txt`, confronté au
décompte que `Lexique.ParCategorie` tient sur les entrées. Ajouter des entrées
laisse le test vert, et une catégorie nouvelle est signalée plutôt que refusée ;
seule une perte échoue. Après un enrichissement du lexique, les planchers se
relèvent par une commande :

```sh
go test -run TestPlanchersParCategorie -maj-planchers
```

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
