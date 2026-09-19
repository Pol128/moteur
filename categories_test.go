package moteur

// Le plancher par catégorie du référentiel — le pendant, sur la distribution,
// de ce que `# plancher-resolution:` fait sur l'agrégat.
//
// `TestResolutionDuJeuDeReference` compte les aliments qui tombent sur **une**
// entrée, pas sur la **bonne** catégorie, et il est agrégé : le lexique
// pourrait perdre la totalité de ses « Poissons et fruits de mer » sans qu'un
// test bronche, tant que le total tient. Ce fichier tient l'autre bout.
//
// Ce sont des planchers, jamais des égalités : le référentiel s'enrichit au fil
// des manques, et un test qui exigerait un compte exact rougirait à chaque
// ajout. Seule une perte doit échouer.

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Le motif du fichier témoin : la régénération est une commande, pas une
// procédure écrite quelque part qu'on suivra de travers.
//
//	go test -run TestPlanchersParCategorie -maj-planchers
var majPlanchers = flag.Bool("maj-planchers", false,
	"réécrit testdata/categories_fr.txt depuis le lexique embarqué")

const cheminPlanchers = "categories_fr.txt"

// Un lexique où une entrée porte deux alias et une autre un seul : c'est ce qui
// sépare un décompte d'entrées d'un décompte de formes. `Lexique.formes` mène
// le nom, le pluriel et chaque alias vers la même `Entree` — le parcourir
// surcompterait chaque catégorie dans la proportion de ses alias.
const lexiqueParCategorie = `{"items":[
  {"name":"tomate","pluralName":"tomates","aliases":["tomate ronde","tomate grappe"],"label":"Légumes"},
  {"name":"oignon jaune","pluralName":"oignons jaunes","aliases":["oignon brun"],"label":"Légumes"},
  {"name":"basilic","pluralName":"basilics","aliases":[],"label":"Herbes fraîches"},
  {"name":"chose","pluralName":"choses","aliases":[],"label":""}
]}`

func TestParCategorieCompteLesEntreesPasLesFormes(t *testing.T) {
	p := packFR(t)
	lexique, err := LisAliments([]byte(lexiqueParCategorie), p)
	if err != nil {
		t.Fatalf("lecture du lexique : %v", err)
	}

	// Onze écritures pour quatre entrées : c'est l'écart que le décompte ne
	// doit pas reproduire.
	if lexique.Formes() != 11 {
		t.Errorf("%d formes, attendu 11", lexique.Formes())
	}

	comptes := lexique.ParCategorie()
	attendu := map[string]int{"Légumes": 2, "Herbes fraîches": 1}
	if len(comptes) != len(attendu) {
		t.Errorf("%d catégories, attendu %d : %v", len(comptes), len(attendu), comptes)
	}
	for categorie, compte := range attendu {
		if comptes[categorie] != compte {
			t.Errorf("« %s » : %d entrées, attendu %d", categorie, comptes[categorie], compte)
		}
	}
	// Une entrée sans label n'a pas de catégorie : elle n'en invente pas une
	// vide, qui se retrouverait dans le fichier de planchers.
	if _, present := comptes[""]; present {
		t.Errorf("une catégorie vide est comptée : %v", comptes)
	}

	// Le décompte est une vue, pas l'état interne : le modifier ne doit rien
	// changer au lexique.
	comptes["Légumes"] = 99
	if lexique.ParCategorie()["Légumes"] != 2 {
		t.Error("ParCategorie rend la carte interne : un appelant peut la corrompre")
	}

	var nul *Lexique
	if comptes := nul.ParCategorie(); len(comptes) != 0 {
		t.Errorf("lexique nul : %v, attendu aucune catégorie", comptes)
	}
}

func TestLesPlanchersNeRougissentQueSurUnePerte(t *testing.T) {
	planchers := map[string]int{"Légumes": 2, "Fruits": 1}

	// Le référentiel du jour tient : chaque catégorie est à son plancher.
	if sous, absentes := comparePlanchers(map[string]int{"Légumes": 2, "Fruits": 1}, planchers); len(sous) != 0 || len(absentes) != 0 {
		t.Errorf("%v sous plancher, %v sans plancher, attendu aucun des deux", sous, absentes)
	}

	// Ajouter des entrées laisse vert — y compris quand l'ajout crée une
	// catégorie que le fichier ne connaît pas encore. Son plancher vaut 0, et
	// le test la signale pour qu'une régénération la reprenne : sans ça,
	// « ajouter des entrées laisse le test vert » tomberait dès la première
	// catégorie neuve.
	sous, absentes := comparePlanchers(map[string]int{"Légumes": 7, "Fruits": 1, "Œufs": 3}, planchers)
	if len(sous) != 0 {
		t.Errorf("%v sous plancher, attendu aucune : un enrichissement ne rougit pas", sous)
	}
	if len(absentes) != 1 || absentes[0] != "Œufs" {
		t.Errorf("sans plancher : %v, attendu [Œufs]", absentes)
	}

	// Une perte sous le plancher rougit, et le dit : la catégorie, le
	// plancher, le compte trouvé.
	sous, _ = comparePlanchers(map[string]int{"Légumes": 1, "Fruits": 1}, planchers)
	if len(sous) != 1 || sous[0] != (sousPlancher{Categorie: "Légumes", Plancher: 2, Compte: 1}) {
		t.Fatalf("sous plancher : %v, attendu [{Légumes 2 1}]", sous)
	}

	// Une catégorie qui disparaît entièrement est une perte, pas un plancher
	// qu'on n'a plus à vérifier.
	sous, _ = comparePlanchers(map[string]int{"Fruits": 1}, planchers)
	if len(sous) != 1 || sous[0] != (sousPlancher{Categorie: "Légumes", Plancher: 2, Compte: 0}) {
		t.Fatalf("sous plancher : %v, attendu [{Légumes 2 0}]", sous)
	}
}

func TestFormatDesPlanchers(t *testing.T) {
	exemple := "# Planchers par catégorie.\n" +
		"# catégorie\tplancher\n" +
		"Légumes\t104\n" +
		"\n" +
		"Œufs\t4\n"

	planchers, err := lisPlanchers(exemple)
	if err != nil {
		t.Fatalf("lecture : %v", err)
	}
	if len(planchers) != 2 || planchers["Légumes"] != 104 || planchers["Œufs"] != 4 {
		t.Errorf("planchers %v, attendu {Légumes:104, Œufs:4}", planchers)
	}

	// Une ligne mal formée est une erreur, pas un plancher silencieusement
	// ignoré : un fichier à moitié lu rendrait le garde-fou muet sur ce qu'il
	// n'a pas su lire.
	for _, mauvais := range []string{"Légumes\tbeaucoup\n", "Légumes\n", "Légumes\t3\tde plus\n"} {
		if _, err := lisPlanchers(mauvais); err == nil {
			t.Errorf("%q lu sans erreur", mauvais)
		}
	}
}

// La régénération doit pouvoir tourner deux fois de suite sans rien changer :
// sinon `git diff` n'est jamais vide et le fichier bouge à chaque cycle.
func TestLaRegenerationEstIdempotente(t *testing.T) {
	// Insérées à rebours du tri attendu : ce que la carte rendra, quel que
	// soit l'ordre d'itération, ne sera pas trié par accident.
	comptes := map[string]int{"Œufs": 4, "Légumes": 104, "Herbes et épices": 115}

	texte := ecritPlanchers(comptes)
	relus, err := lisPlanchers(texte)
	if err != nil {
		t.Fatalf("relecture : %v", err)
	}
	if len(relus) != len(comptes) {
		t.Fatalf("%d planchers relus, attendu %d", len(relus), len(comptes))
	}
	for categorie, compte := range comptes {
		if relus[categorie] != compte {
			t.Errorf("« %s » : %d relu, attendu %d", categorie, relus[categorie], compte)
		}
	}
	if encore := ecritPlanchers(relus); encore != texte {
		t.Error("deux écritures du même décompte ne donnent pas le même texte")
	}

	// Et le tri est ce qui le garantit. Comparer deux écritures ne suffit
	// pas : l'itération d'une carte est aléatoire, donc deux écritures non
	// triées coïncident une fois sur deux — mesuré. L'ordre se vérifie donc
	// directement, sur le texte produit.
	var categories []string
	for _, ligne := range strings.Split(texte, "\n") {
		if strings.HasPrefix(ligne, "#") || strings.TrimSpace(ligne) == "" {
			continue
		}
		categories = append(categories, strings.SplitN(ligne, "\t", 2)[0])
	}
	if len(categories) != len(comptes) {
		t.Errorf("%d lignes de catégorie, attendu %d", len(categories), len(comptes))
	}
	if !sort.StringsAreSorted(categories) {
		t.Errorf("catégories non triées : %v", categories)
	}

	// La commande est dans l'entête : c'est là qu'on la cherche quand le test
	// rougit, pas dans le README.
	if !strings.Contains(texte, "-maj-planchers") {
		t.Errorf("l'entête ne porte pas la commande de régénération :\n%s", texte)
	}
}

// Le garde-fou lui-même, sur le lexique embarqué.
func TestPlanchersParCategorie(t *testing.T) {
	p := packFR(t)
	lexique, err := AlimentsFR(p)
	if err != nil {
		t.Fatalf("lexique embarqué : %v", err)
	}
	comptes := lexique.ParCategorie()
	chemin := filepath.Join("testdata", cheminPlanchers)

	if *majPlanchers {
		if err := os.WriteFile(chemin, []byte(ecritPlanchers(comptes)), 0o644); err != nil {
			t.Fatalf("écriture de %s : %v", chemin, err)
		}
		t.Logf("%s réécrit : %d catégories, %d entrées", chemin, len(comptes), lexique.Entrees)
		return
	}

	contenu, err := os.ReadFile(chemin)
	if err != nil {
		t.Fatal(err)
	}
	planchers, err := lisPlanchers(string(contenu))
	if err != nil {
		t.Fatalf("%s : %v", chemin, err)
	}
	if len(planchers) == 0 {
		t.Fatalf("aucun plancher dans %s", chemin)
	}

	sous, absentes := comparePlanchers(comptes, planchers)
	for _, perte := range sous {
		t.Errorf("« %s » : %d entrées au lexique, plancher %d",
			perte.Categorie, perte.Compte, perte.Plancher)
	}
	for _, categorie := range absentes {
		t.Logf("« %s » : %d entrées, aucun plancher — régénérer avec "+
			"« go test -run TestPlanchersParCategorie -maj-planchers »",
			categorie, comptes[categorie])
	}
	t.Logf("%d catégories, %d entrées ; %d planchers tenus",
		len(comptes), lexique.Entrees, len(planchers))
}

// ------------------------------------------- le fichier de planchers

// sousPlancher est une catégorie descendue sous le compte inscrit.
type sousPlancher struct {
	Categorie string
	Plancher  int
	Compte    int
}

// comparePlanchers confronte le décompte du jour aux planchers inscrits. Il
// rend les catégories passées dessous — une catégorie disparue y est à 0 — et
// celles que le lexique porte sans que le fichier les connaisse.
func comparePlanchers(comptes, planchers map[string]int) (sous []sousPlancher, absentes []string) {
	for categorie, plancher := range planchers {
		if compte := comptes[categorie]; compte < plancher {
			sous = append(sous, sousPlancher{Categorie: categorie, Plancher: plancher, Compte: compte})
		}
	}
	for categorie := range comptes {
		if _, inscrite := planchers[categorie]; !inscrite {
			absentes = append(absentes, categorie)
		}
	}
	sort.Slice(sous, func(i, j int) bool { return sous[i].Categorie < sous[j].Categorie })
	sort.Strings(absentes)
	return sous, absentes
}

// lisPlanchers relit `testdata/categories_fr.txt` : même forme que
// `testdata/fr.txt` — des commentaires `#` en entête, puis une ligne par
// catégorie, la catégorie et son plancher séparés par une tabulation.
func lisPlanchers(texte string) (map[string]int, error) {
	planchers := map[string]int{}
	for numero, brut := range strings.Split(texte, "\n") {
		if strings.HasPrefix(brut, "#") || strings.TrimSpace(brut) == "" {
			continue
		}
		champs := strings.Split(brut, "\t")
		if len(champs) != 2 {
			return nil, fmt.Errorf("ligne %d : %d champs, attendu « catégorie<TAB>plancher »",
				numero+1, len(champs))
		}
		plancher, err := strconv.Atoi(strings.TrimSpace(champs[1]))
		if err != nil {
			return nil, fmt.Errorf("ligne %d : plancher %q illisible", numero+1, champs[1])
		}
		planchers[champs[0]] = plancher
	}
	return planchers, nil
}

// ecritPlanchers rend le fichier tel que `-maj-planchers` l'écrit. Les
// catégories sont triées : le fichier doit être stable d'une régénération à
// l'autre, sinon le diff est illisible et jamais vide.
func ecritPlanchers(comptes map[string]int) string {
	categories := make([]string, 0, len(comptes))
	total := 0
	for categorie, compte := range comptes {
		categories = append(categories, categorie)
		total += compte
	}
	sort.Strings(categories)

	var texte strings.Builder
	texte.WriteString("# Planchers par catégorie du lexique embarqué (data/foods_fr.json).\n")
	texte.WriteString("#\n")
	texte.WriteString("# Une tabulation entre la catégorie et son plancher. Ce sont des planchers,\n")
	texte.WriteString("# pas des comptes exacts : le référentiel s'enrichit au fil des manques, et\n")
	texte.WriteString("# seule une perte doit faire rougir `TestPlanchersParCategorie`.\n")
	texte.WriteString("#\n")
	texte.WriteString("# Fichier produit — ne pas éditer à la main. Après un enrichissement du\n")
	texte.WriteString("# lexique, relever les planchers avec :\n")
	texte.WriteString("#\n")
	texte.WriteString("#   go test -run TestPlanchersParCategorie -maj-planchers\n")
	texte.WriteString("#\n")
	fmt.Fprintf(&texte, "# %d catégories, %d entrées au total.\n", len(categories), total)
	texte.WriteString("#\n")
	texte.WriteString("# catégorie\tplancher\n")
	for _, categorie := range categories {
		fmt.Fprintf(&texte, "%s\t%d\n", categorie, comptes[categorie])
	}
	return texte.String()
}
