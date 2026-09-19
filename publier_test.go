package moteur

// Les refus du script ./publier.
//
// Publier engage des consommateurs : couper une version reste un geste humain,
// lancé à la main. Ce qui se vérifie par une commande, c'est que le script
// refuse de partir quand il ne devrait pas — argument absent ou mal formé,
// version déjà taguée, arbre de travail sale, section « À paraître » vide,
// suite de tests rouge.
//
// Chaque cas monte un dépôt jetable dans t.TempDir(), y copie le vrai script
// et le lance : l'arbre de travail réel n'est jamais touché, aucun tag n'y est
// créé, le réseau n'est pas joint.
//
// C'est pourquoi le script place ses quatre contrôles bon marché avant
// `go test ./...` : un test qui atteindrait cette étape ici relancerait la
// suite depuis la suite. Le seul cas qui l'atteint le fait dans un module
// jetable d'un seul fichier.

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const journalExemple = `# Journal des versions

## À paraître

### Lecture
- une règle de lecture nouvelle

## v0.1.0 — 2026-08-19

Première version.
`

const journalSansRienEnAttente = `# Journal des versions

## À paraître

## v0.1.0 — 2026-08-19

Première version.
`

const journalSansEntete = `# Journal des versions

## v0.1.0 — 2026-08-19

Première version.
`

// etatPublie est ce qui doit rester intact quand le script refuse : aucun tag
// posé, aucun commit ajouté, le journal tel qu'il était.
type etatPublie struct {
	tags    string
	tete    string
	journal string
}

func releve(t *testing.T, dir string) etatPublie {
	t.Helper()
	journal, err := os.ReadFile(filepath.Join(dir, "CHANGELOG.md"))
	if err != nil {
		t.Fatalf("lecture du journal jetable : %v", err)
	}
	return etatPublie{
		tags:    gitJetable(t, dir, "tag", "-l"),
		tete:    gitJetable(t, dir, "rev-parse", "HEAD"),
		journal: string(journal),
	}
}

// depotJetable monte un dépôt git complet dans un répertoire temporaire : le
// vrai script y est copié, le journal écrit, le tout commité. L'arbre en sort
// propre — chaque test le salit ensuite comme il lui faut.
func depotJetable(t *testing.T, journal string) string {
	t.Helper()

	script, err := os.ReadFile("publier")
	if err != nil {
		t.Fatalf("lecture du script publier : %v", err)
	}

	dir := t.TempDir()
	ecritJetable(t, dir, "publier", string(script))
	if err := os.Chmod(filepath.Join(dir, "publier"), 0o755); err != nil {
		t.Fatal(err)
	}
	ecritJetable(t, dir, "CHANGELOG.md", journal)

	gitJetable(t, dir, "init", "-q", "-b", "main")
	gitJetable(t, dir, "config", "user.email", "jetable@example.invalid")
	gitJetable(t, dir, "config", "user.name", "Depot jetable")
	gitJetable(t, dir, "config", "commit.gpgsign", "false")
	gitJetable(t, dir, "add", "publier", "CHANGELOG.md")
	gitJetable(t, dir, "commit", "-q", "-m", "depot jetable")
	return dir
}

func ecritJetable(t *testing.T, dir, nom, contenu string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, nom), []byte(contenu), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitJetable(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	sortie, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s : %v\n%s", strings.Join(args, " "), err, sortie)
	}
	return string(sortie)
}

func lancePublier(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command("sh", append([]string{"./publier"}, args...)...)
	cmd.Dir = dir
	sortie, err := cmd.CombinedOutput()
	if err == nil {
		return string(sortie), 0
	}
	var echec *exec.ExitError
	if errors.As(err, &echec) {
		return string(sortie), echec.ExitCode()
	}
	t.Fatalf("lancement de ./publier : %v", err)
	return "", 0
}

// exigeRefus : le script s'est arrêté sur un code non nul, et il a dit pourquoi.
func exigeRefus(t *testing.T, sortie string, code int, attendu string) {
	t.Helper()
	if code == 0 {
		t.Fatalf("./publier a réussi, refus attendu\n%s", sortie)
	}
	if !strings.Contains(sortie, attendu) {
		t.Errorf("le refus ne dit pas « %s » :\n%s", attendu, sortie)
	}
}

// exigeIntact : rien n'a été publié — ni tag, ni commit, ni journal réécrit.
func exigeIntact(t *testing.T, dir string, avant etatPublie) {
	t.Helper()
	apres := releve(t, dir)
	if apres.tags != avant.tags {
		t.Errorf("tags %q, attendu %q — le script a tagué malgré son refus", apres.tags, avant.tags)
	}
	if apres.tete != avant.tete {
		t.Error("HEAD a bougé — le script a commité malgré son refus")
	}
	if apres.journal != avant.journal {
		t.Errorf("le journal a été réécrit malgré le refus :\n%s", apres.journal)
	}
}

func TestPublierSansArgumentAfficheSonUsage(t *testing.T) {
	dir := depotJetable(t, journalExemple)
	avant := releve(t, dir)

	sortie, code := lancePublier(t, dir)

	exigeRefus(t, sortie, code, "usage")
	exigeIntact(t, dir, avant)
}

func TestPublierRefuseUnArgumentMalForme(t *testing.T) {
	// Un argument d'une autre forme échoue comme un argument absent : la
	// version est le tag tel qu'il existera, pas un numéro à reformater.
	for _, argument := range []string{"0.3.0", "v0.3", "v0.3.0.1", "v0.3.0-rc1", "prochaine"} {
		t.Run(argument, func(t *testing.T) {
			dir := depotJetable(t, journalExemple)
			avant := releve(t, dir)

			sortie, code := lancePublier(t, dir, argument)

			exigeRefus(t, sortie, code, "usage")
			exigeIntact(t, dir, avant)
		})
	}
}

func TestPublierRefuseUneVersionDejaTaguee(t *testing.T) {
	dir := depotJetable(t, journalExemple)
	gitJetable(t, dir, "tag", "-a", "v0.3.0", "-m", "v0.3.0")
	avant := releve(t, dir)

	sortie, code := lancePublier(t, dir, "v0.3.0")

	exigeRefus(t, sortie, code, "existe déjà")
	exigeIntact(t, dir, avant)
}

func TestPublierRefuseUnArbreDeTravailSale(t *testing.T) {
	dir := depotJetable(t, journalExemple)
	ecritJetable(t, dir, "brouillon.txt", "en cours\n")
	avant := releve(t, dir)

	sortie, code := lancePublier(t, dir, "v0.3.0")

	exigeRefus(t, sortie, code, "arbre de travail")
	exigeIntact(t, dir, avant)
}

func TestPublierRefuseUneSectionAParaitreVide(t *testing.T) {
	dir := depotJetable(t, journalSansRienEnAttente)
	avant := releve(t, dir)

	sortie, code := lancePublier(t, dir, "v0.3.0")

	exigeRefus(t, sortie, code, "À paraître")
	exigeIntact(t, dir, avant)
}

func TestPublierRefuseUnJournalSansEnTeteAParaitre(t *testing.T) {
	// On ne devine pas : si l'en-tête a bougé, mieux vaut échouer que
	// réécrire le journal au mauvais endroit.
	dir := depotJetable(t, journalSansEntete)
	avant := releve(t, dir)

	sortie, code := lancePublier(t, dir, "v0.3.0")

	exigeRefus(t, sortie, code, "introuvable")
	exigeIntact(t, dir, avant)
}

func TestPublierRefuseUneSuiteRouge(t *testing.T) {
	// Le seul cas qui atteint `go test ./...`, et il l'atteint dans un module
	// jetable d'un seul fichier : la suite de ce dépôt-ci n'est pas relancée.
	dir := depotJetable(t, journalExemple)
	ecritJetable(t, dir, "go.mod", "module jetable.invalid\n\ngo 1.26.6\n")
	ecritJetable(t, dir, "rouge_test.go", "package jetable\n\nimport \"testing\"\n\nfunc TestRouge(t *testing.T) { t.Fatal(\"rouge\") }\n")
	gitJetable(t, dir, "add", "go.mod", "rouge_test.go")
	gitJetable(t, dir, "commit", "-q", "-m", "module jetable rouge")
	avant := releve(t, dir)

	sortie, code := lancePublier(t, dir, "v0.3.0")

	exigeRefus(t, sortie, code, "rouge")
	exigeIntact(t, dir, avant)
}

func TestLeJournalPorteSesSections(t *testing.T) {
	// Le journal ne commence pas au milieu : les trois tags existants y sont.
	// Et l'en-tête « À paraître » est exactement celui que le script cherche.
	journal, err := os.ReadFile("CHANGELOG.md")
	if err != nil {
		t.Fatalf("lecture de CHANGELOG.md : %v", err)
	}
	for _, section := range []string{
		"## À paraître",
		"## v0.2.0 — 2026-09-19",
		"## v0.1.1 — 2026-08-19",
		"## v0.1.0 — 2026-08-19",
	} {
		if !strings.Contains(string(journal), section+"\n") {
			t.Errorf("section « %s » absente du journal", section)
		}
	}
}
