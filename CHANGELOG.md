# Journal des versions

Ce que chaque version a changé pour qui importe ce module. La section
« À paraître » porte ce qui est déjà sur `main` sans être publié : non vide
alors qu'aucun tag n'a suivi, elle donne à lire la dette — c'est tout son
objet. Un changement de comportement ou de l'API exportée y entre avec la
tâche qui le livre ([DOD.md](DOD.md)).

Les versions se coupent à la main, par `./publier vX.Y.Z` : publier engage des
consommateurs, ce n'est pas la conséquence mécanique d'une CI verte. Les
sections d'avant `v0.2.0` sont reprises des messages des tags annotés.

## À paraître

## v0.2.0 — 2026-09-19

Première version depuis que ce module est la seule implémentation du parser
(décidé le 19/09/2026) : tout ce qui suit a été écrit ici, et nulle part
ailleurs.

**Changement de comportement, à lire avant de monter.** `Ingredient.Aliment`
rend désormais la forme canonique du lexique, pas ce qui était écrit :
« 3 oignons » donne « oignon ». Le texte d'origine reste disponible dans le
nouveau champ `Ingredient.AlimentTexte`. Ça compile sans rien changer, et ça
affiche autre chose.

### Lecture
- la préparation qui suit une virgule finale part en note
- le motif inversé « Aliment : quantité »
- la contenance placée après son contenant sort de l'aliment
- « gramme(s) » et le second terme d'une addition
- le `Lexique` devient un index forme → entrée

### Référentiel
- `Lexique.Resout(forme)` rend l'entrée, avec son nom, son pluriel et sa catégorie
- `Lexique.ParCategorie()` recense les entrées des 28 catégories
- le référentiel a été complété : résolution 57,5 % → 67,9 % sur le jeu annoté

### Garde-fous
- `testdata/desaccords.txt` épingle les 44 lignes en désaccord, une par une :
  un échange de lignes ne passe plus sous un taux inchangé
- `testdata/categories_fr.txt` pose un plancher par catégorie du référentiel
- `plancher-resolution` dans l'entête du jeu annoté, jumeau du plancher d'accord
- une CI sur la forge : `go test ./...` sur push et sur PR

## v0.1.1 — 2026-08-19

La commande `parse` marche depuis n'importe où : pack et lexique embarqués par
défaut.

## v0.1.0 — 2026-08-19

Première version extraite : parser français, données embarquées, Apache-2.0.
