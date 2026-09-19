# Definition of Done — moteur

Ce qu'il faut avoir fait pour dire qu'une tâche est terminée **ici**. Ce
document **ajoute** à la définition générique partagée par les projets de la
maison — il n'en retire rien, et en cas de contradiction apparente c'est le
plus strict qui gagne.

Il se lit seul : ce dépôt est public, et un contributeur qui le découvre doit
pouvoir savoir ce qu'on attend de sa contribution sans rien chercher ailleurs.

## 1. Une seule commande de vérification

```sh
go test ./...
```

Elle échoue au premier manquement et couvre les garde-fous du module :
`TestPackFactice` (aucune règle de français écrite en Go), `TestJeuDeReference`
et `TestResolutionDuJeuDeReference` (les planchers de `testdata/fr.txt`),
`TestPlanchersParCategorie` (la distribution du référentiel). Verte avant toute
demande de fusion, sans exception : pas de test ignoré, pas de plancher baissé
pour obtenir du vert.

## 2. Le journal, à chaque changement visible du dehors

**Tout changement de comportement du parser ou de l'API exportée alimente la
section « À paraître » de [CHANGELOG.md](CHANGELOG.md), dans la même
contribution que le changement lui-même.**

« Visible du dehors » se lit largement : une règle de lecture nouvelle, un
champ ajouté à `Ingredient`, une signature qui bouge, un plancher relevé, une
donnée embarquée qui change de contenu. Ce qui ne change rien pour qui importe
le module — un test, un commentaire, un remaniement à comportement constant —
n'a rien à y écrire.

Il n'y a **pas de contrôle automatique** pour ce point, et c'est délibéré :
c'est la relecture qui le fait respecter, comme les autres points de cette
définition. Un second contrôle obligatoire en intégration continue coûterait
plus qu'il ne rapporte.

La raison est écrite dans l'histoire du dépôt : `v0.1.1` a tenu du 19/08 au
19/09/2026 pendant que `main` prenait onze commits, et deux consommateurs ont
importé un moteur vieux d'un mois sans que rien ne le signale. Une section
« À paraître » non vide alors qu'aucune version n'a suivi donne cette dette à
lire.

## 3. Publier n'est pas terminer

Couper une version est un geste **humain**, par `./publier vX.Y.Z` : ni
l'intégration continue ni aucun automate ne pose de tag. Publier engage des
consommateurs, et choisir le numéro est un jugement — `v0.2.0` plutôt que
`v0.1.2` parce que `Ingredient.Aliment` change de sens sans casser la
compilation.

Une tâche est donc terminée sans être publiée. Ce qu'elle doit laisser
derrière elle, c'est son entrée dans « À paraître », pour que celui qui
publiera sache quoi écrire.
