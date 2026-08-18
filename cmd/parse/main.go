// Commande parse — le moteur en ligne de commande.
//
// Un mode par besoin :
//
//	parse "500 g de beurre"          lire une ligne, à l'œil
//	parse --filtre < lignes.txt      lire un corpus entier, en TSV
//	parse --compare < attendus.tsv   lire *et* juger face à ce qu'on attendait
//	parse --service                  répondre aux questions posées au pack
//	parse --jeu testdata/fr.txt      l'accord contre le jeu de référence
//	parse --infos                    décrire le pack chargé
//
// Les modes filtre et compare sont ce qui permet à la forge de mesurer ce
// moteur-ci sur les 320 049 lignes du corpus sans qu'une seule en sorte : un
// processus, un tube, et le corpus reste du côté qui le détient.
//
// Le mode service existe pour ce qui interroge le pack sans parser — la sonde
// du lexique, la revue de l'annotation. Ces questions sont des questions de
// langue : y répondre ici évite d'avoir à relire le pack en Python, c'est-à-dire
// d'entretenir une seconde implémentation des mêmes règles.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"moteur"
)

func main() {
	pack := flag.String("pack", "lang/fr.toml", "pack de langue")
	lexique := flag.String("aliments", "data/foods_fr.json",
		"lexique d'aliments ; vide pour s'en passer")
	filtre := flag.Bool("filtre", false, "lire stdin, écrire un TSV sur stdout")
	interactif := flag.Bool("interactif", false,
		"en filtre, rendre chaque ligne aussitôt lue — pour un appelant qui dialogue")
	compare := flag.Bool("compare", false,
		"comme --filtre, mais chaque ligne d'entrée porte aussi ce qui était "+
			"attendu, et la sortie porte le verdict")
	jeu := flag.String("jeu", "", "mesurer l'accord contre un jeu de référence")
	service := flag.Bool("service", false,
		"répondre aux questions qu'on pose au pack : normalise, unite, partitif, quantite")
	infos := flag.Bool("infos", false, "décrire le pack chargé, en JSON")
	flag.Parse()

	p, err := moteur.Charge(*pack)
	if err != nil {
		echoue(err)
	}

	var aliments moteur.Aliments
	if *lexique != "" {
		charge, err := moteur.ChargeAliments(*lexique, p)
		switch {
		case os.IsNotExist(err):
			fmt.Fprintf(os.Stderr, "%s absent : lecture sans lexique\n", *lexique)
		case err != nil:
			echoue(err)
		default:
			aliments = charge
		}
	}

	switch {
	case *infos:
		os.Exit(decritPack(p))
	case *service:
		os.Exit(serviceDuPack(p, *interactif))
	case *jeu != "":
		os.Exit(mesureJeu(*jeu, p, aliments))
	case *compare:
		os.Exit(filtreCompare(p, aliments, *interactif))
	case *filtre:
		os.Exit(filtreEntree(p, aliments, *interactif))
	case flag.NArg() > 0:
		os.Exit(litLignes(flag.Args(), p, aliments))
	default:
		fmt.Fprintln(os.Stderr, "rien à lire — voir --help")
		os.Exit(2)
	}
}

func echoue(err error) {
	fmt.Fprintln(os.Stderr, "erreur :", err)
	os.Exit(1)
}

// ------------------------------------------------------------------- affichage

// litLignes reprend la présentation de `./crawl parse` : le motif dit quelle
// règle a servi.
func litLignes(lignes []string, p *moteur.Pack, aliments moteur.Aliments) int {
	entete := fmt.Sprintf("%9s  %-18s%-7s%-30s%-20s%s",
		"quantité", "unité", "part.", "aliment", "note", "motif")
	fmt.Println(entete)
	fmt.Println(strings.Repeat("-", len([]rune(entete))))

	motifs := map[string]int{}
	ordre := []string{}
	for _, brut := range lignes {
		lu := moteur.Lit(brut, p, aliments)
		if _, vu := motifs[lu.Motif]; !vu {
			ordre = append(ordre, lu.Motif)
		}
		motifs[lu.Motif]++

		quantite := moteur.TexteQuantite(lu.Quantite)
		if lu.QuantiteMax != nil {
			quantite += "–" + moteur.TexteQuantite(lu.QuantiteMax)
		}
		switch {
		case lu.Indefinie:
			quantite = "indéf."
		case lu.Approximative && quantite != "":
			quantite = "~" + quantite
		}
		fmt.Printf("%9s  %-18s%-7s%-30s%-20s%s\n", quantite, lu.UniteCle(),
			lu.Partitif, tronque(lu.Aliment, 29), tronque(lu.Note, 19), lu.Motif)
		fmt.Println("           " + brut)
	}
	if len(lignes) > 1 {
		var resume []string
		for _, motif := range ordre {
			resume = append(resume, fmt.Sprintf("%s %d", motif, motifs[motif]))
		}
		fmt.Println("\nmotifs : " + strings.Join(resume, ", "))
	}
	return 0
}

func tronque(texte string, taille int) string {
	runes := []rune(texte)
	if len(runes) <= taille {
		return texte
	}
	return string(runes[:taille])
}

// --------------------------------------------------------------------- filtre

// colonnes du mode filtre, dans cet ordre. Le texte brut n'y figure pas : il
// pourrait contenir une tabulation, et l'appelant connaît l'ordre de ses
// propres lignes.
var colonnes = []string{
	"motif", "quantite", "quantite_max", "quantite_texte", "unite", "unite_texte",
	"partitif", "aliment", "note", "qualificatifs", "optionnel", "approximative",
	"indefinie",
}

// filtreEntree lit stdin ligne à ligne et rend un TSV, une ligne de sortie par
// ligne d'entrée. En mode interactif, chaque ligne part aussitôt : c'est ce qui
// permet à un appelant de dialoguer avec le moteur sans le relancer, au prix
// d'un appel système par ligne.
func filtreEntree(p *moteur.Pack, aliments moteur.Aliments, interactif bool) int {
	entree := bufio.NewScanner(os.Stdin)
	entree.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	sortie := bufio.NewWriter(os.Stdout)
	defer sortie.Flush()

	fmt.Fprintln(sortie, strings.Join(colonnes, "\t"))
	if interactif {
		sortie.Flush()
	}
	for entree.Scan() {
		lu := moteur.Lit(entree.Text(), p, aliments)
		ecrit(sortie, champsLus(lu), interactif)
	}
	if err := entree.Err(); err != nil {
		echoue(err)
	}
	return 0
}

func champsLus(lu *moteur.Ingredient) []string {
	return []string{
		lu.Motif,
		moteur.TexteQuantite(lu.Quantite),
		moteur.TexteQuantite(lu.QuantiteMax),
		lu.QuantiteTexte,
		lu.UniteCle(),
		lu.UniteTexte,
		lu.Partitif,
		lu.Aliment,
		lu.Note,
		strings.Join(lu.Qualificatifs, "|"),
		bit(lu.Optionnel),
		bit(lu.Approximative),
		bit(lu.Indefinie),
	}
}

func ecrit(sortie *bufio.Writer, champs []string, interactif bool) {
	for i, champ := range champs {
		champs[i] = strings.ReplaceAll(champ, "\t", " ")
	}
	fmt.Fprintln(sortie, strings.Join(champs, "\t"))
	if interactif {
		sortie.Flush()
	}
}

func bit(vrai bool) string {
	if vrai {
		return "1"
	}
	return "0"
}

// ------------------------------------------------------------------- comparer

// colonnesAttendues : ce que l'appelant joint à chaque ligne pour la faire
// juger. C'est ce qu'une source publie, ou ce qu'un humain a annoté.
var colonnesAttendues = []string{
	"brut", "quantite", "unite", "partitif", "aliment", "note", "bloc",
}

// colonnesVerdict : le jugement, champ par champ.
//
// Les quatre premiers sont de simples égalités, toujours calculées. C'est
// l'appelant qui décide lesquelles comptent : chez un site, un champ attendu
// vide veut dire « non publié » et ne doit rien coûter au parser ; dans le jeu
// annoté, il veut dire « cette ligne n'a pas d'unité » et se mesure. Le moteur
// n'a pas à connaître cette distinction, il répond à la question posée.
var colonnesVerdict = []string{
	"egal_quantite", "egal_unite", "egal_unite_famille", "egal_partitif",
	"egal_aliment", "meme_span", "bloc",
}

// filtreCompare lit une ligne et ce qu'on en attendait, et rend ce qu'il a lu
// et son verdict. C'est ce qui permet à la forge de mesurer sans lire le pack :
// tout ce qui demande de connaître la langue — « càs » vaut-il « cuillères à
// soupe », « une » vaut-il 1 — se décide ici, et la forge ne fait que compter.
func filtreCompare(p *moteur.Pack, aliments moteur.Aliments, interactif bool) int {
	entree := bufio.NewScanner(os.Stdin)
	entree.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	sortie := bufio.NewWriter(os.Stdout)
	defer sortie.Flush()

	ecrit(sortie, append(append([]string{}, colonnes...), colonnesVerdict...), interactif)
	for entree.Scan() {
		champs := strings.Split(entree.Text(), "\t")
		for len(champs) < len(colonnesAttendues) {
			champs = append(champs, "")
		}
		attendu := moteur.LigneRef{
			Brut: champs[0], Quantite: champs[1], Unite: champs[2],
			Partitif: champs[3], Aliment: champs[4], Note: champs[5],
		}
		bloc := champs[6]

		lu := moteur.Lit(attendu.Brut, p, aliments)
		strict := moteur.Compare(attendu, lu, p, false, false)
		famille := moteur.Compare(attendu, lu, p, false, true)

		verdict := []string{
			bit(*strict["quantite"]),
			bit(*strict["unite"]),
			bit(*famille["unite"]),
			bit(*strict["partitif"]),
			bit(*strict["aliment"]),
			bit(moteur.Span(p, attendu.Aliment, attendu.Note) ==
				moteur.Span(p, lu.Aliment, lu.Note)),
			"-",
		}
		if bloc != "" {
			verdict[len(verdict)-1] = bit(moteur.BlocRestitue(p, bloc, lu))
		}
		ecrit(sortie, append(champsLus(lu), verdict...), interactif)
	}
	if err := entree.Err(); err != nil {
		echoue(err)
	}
	return 0
}

// -------------------------------------------------------------------- service

// serviceDuPack répond aux questions qu'on pose au pack, une par ligne.
//
// Le récolteur en a besoin pour deux outils qui ne parsent pas : la sonde du
// lexique, qui demande « ce mot est-il une unité ? », et la revue, qui demande
// « ces deux écritures sont-elles le même lemme ? ». Ce sont des questions de
// langue : elles se posent ici, pas dans une seconde lecture du pack en Python.
//
//	normalise <texte>   → le texte normalisé
//	unite     <texte>   → clé · dimension · base · facteur   (vide si inconnue)
//	partitif  <texte>   → la forme du partitif en tête, ou vide
//	quantite  <texte>   → valeur · approximative             (vide si aucune)
func serviceDuPack(p *moteur.Pack, interactif bool) int {
	entree := bufio.NewScanner(os.Stdin)
	entree.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	sortie := bufio.NewWriter(os.Stdout)
	defer sortie.Flush()

	for entree.Scan() {
		verbe, argument, _ := strings.Cut(entree.Text(), "\t")
		var reponse []string
		switch verbe {
		case "normalise":
			reponse = []string{p.Normalise(argument)}
		case "unite":
			reponse = []string{"", "", "", ""}
			if lue := p.LireUnite(argument); lue != nil {
				reponse = []string{lue.Unite.Cle, lue.Unite.Dimension,
					texteFacultatif(lue.Unite.Base), moteur.TexteQuantite(&lue.Facteur)}
			}
		case "partitif":
			reponse = []string{p.Partitif(argument)}
		case "quantite":
			reponse = []string{"", ""}
			if q := p.Quantite(argument); q != nil {
				reponse = []string{moteur.TexteQuantite(&q.Valeur), bit(q.Approximative)}
			}
		default:
			fmt.Fprintf(os.Stderr, "verbe inconnu : %q\n", verbe)
			reponse = []string{""}
		}
		ecrit(sortie, reponse, interactif)
	}
	if err := entree.Err(); err != nil {
		echoue(err)
	}
	return 0
}

func texteFacultatif(valeur *float64) string {
	if valeur == nil {
		return ""
	}
	return moteur.TexteQuantite(valeur)
}

// decritPack rend de quoi afficher l'entête d'une commande, et surtout la table
// des unités : l'appelant y résout une clé sans avoir à relire le TOML, donc
// sans avoir à savoir le lire.
func decritPack(p *moteur.Pack) int {
	type uniteJSON struct {
		Cle       string   `json:"cle"`
		Dimension string   `json:"dimension"`
		Base      *float64 `json:"base"`
		Facteur   *float64 `json:"facteur"`
		Singulier string   `json:"singulier"`
		Pluriel   string   `json:"pluriel"`
		Abrev     string   `json:"abrev"`
	}
	unites := make([]uniteJSON, 0, len(p.ClesUnites))
	for _, cle := range p.ClesUnites {
		u := p.Unites[cle]
		unites = append(unites, uniteJSON{u.Cle, u.Dimension, u.Base, u.Facteur,
			u.Singulier, u.Pluriel, u.Abrev})
	}

	description := struct {
		Langue       string      `json:"langue"`
		Version      int         `json:"version"`
		Formes       int         `json:"formes"`
		Litterales   int         `json:"litterales"`
		SeuilPluriel float64     `json:"seuil_pluriel"`
		Unites       []uniteJSON `json:"unites"`
	}{p.Langue, p.Version, p.NombreDeFormes(), len(p.Litterales),
		p.SeuilPluriel, unites}

	encodeur := json.NewEncoder(os.Stdout)
	encodeur.SetIndent("", "  ")
	if err := encodeur.Encode(description); err != nil {
		echoue(err)
	}
	return 0
}

// ------------------------------------------------------------------------ jeu

func mesureJeu(chemin string, p *moteur.Pack, aliments moteur.Aliments) int {
	contenu, err := os.ReadFile(chemin)
	if err != nil {
		echoue(err)
	}
	lignes, plancher := moteur.AnalyseJeu(string(contenu))
	part, desaccords := moteur.Accord(lignes, p, func(brut string) *moteur.Ingredient {
		return moteur.Lit(brut, p, aliments)
	})
	fmt.Printf("accord %.1f %% sur %d lignes (plancher %.1f %%)\n",
		part*100, len(lignes), plancher*100)
	for _, d := range desaccords {
		fmt.Printf("\n  %s\n    attendu  %s\n    lu       %s\n", d.Brut, d.Attendu, d.Lu)
	}
	if part+1e-9 < plancher {
		return 1
	}
	return 0
}
