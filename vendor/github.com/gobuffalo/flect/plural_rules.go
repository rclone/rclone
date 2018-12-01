package flect

var pluralRules = []rule{}

// AddPlural adds a rule that will replace the given suffix with the replacement suffix.
func AddPlural(suffix string, repl string) {
	pluralMoot.Lock()
	defer pluralMoot.Unlock()
	pluralRules = append(pluralRules, rule{
		suffix: suffix,
		fn: func(s string) string {
			s = s[:len(s)-len(suffix)]
			return s + repl
		},
	})

	pluralRules = append(pluralRules, rule{
		suffix: repl,
		fn:     noop,
	})
}

var singleToPlural = map[string]string{
	"matrix":      "matrices",
	"vertix":      "vertices",
	"index":       "indices",
	"mouse":       "mice",
	"louse":       "lice",
	"ress":        "resses",
	"ox":          "oxen",
	"quiz":        "quizzes",
	"series":      "series",
	"octopus":     "octopi",
	"equipment":   "equipment",
	"information": "information",
	"rice":        "rice",
	"money":       "money",
	"species":     "species",
	"fish":        "fish",
	"sheep":       "sheep",
	"jeans":       "jeans",
	"police":      "police",
	"dear":        "dear",
	"goose":       "geese",
	"tooth":       "teeth",
	"foot":        "feet",
	"bus":         "buses",
	"fez":         "fezzes",
	"piano":       "pianos",
	"halo":        "halos",
	"photo":       "photos",
	"aircraft":    "aircraft",
	"alumna":      "alumnae",
	"alumnus":     "alumni",
	"analysis":    "analyses",
	"antenna":     "antennas",
	"antithesis":  "antitheses",
	"apex":        "apexes",
	"appendix":    "appendices",
	"axis":        "axes",
	"bacillus":    "bacilli",
	"bacterium":   "bacteria",
	"basis":       "bases",
	"beau":        "beaus",
	"bison":       "bison",
	"bureau":      "bureaus",
	"campus":      "campuses",
	"château":     "châteaux",
	"codex":       "codices",
	"concerto":    "concertos",
	"corpus":      "corpora",
	"crisis":      "crises",
	"curriculum":  "curriculums",
	"deer":        "deer",
	"diagnosis":   "diagnoses",
	"die":         "dice",
	"dwarf":       "dwarves",
	"ellipsis":    "ellipses",
	"erratum":     "errata",
	"faux pas":    "faux pas",
	"focus":       "foci",
	"formula":     "formulas",
	"fungus":      "fungi",
	"genus":       "genera",
	"graffito":    "graffiti",
	"grouse":      "grouse",
	"half":        "halves",
	"hoof":        "hooves",
	"hypothesis":  "hypotheses",
	"larva":       "larvae",
	"libretto":    "librettos",
	"loaf":        "loaves",
	"locus":       "loci",
	"minutia":     "minutiae",
	"moose":       "moose",
	"nebula":      "nebulae",
	"nucleus":     "nuclei",
	"oasis":       "oases",
	"offspring":   "offspring",
	"opus":        "opera",
	"parenthesis": "parentheses",
	"phenomenon":  "phenomena",
	"phylum":      "phyla",
	"prognosis":   "prognoses",
	"radius":      "radiuses",
	"referendum":  "referendums",
	"salmon":      "salmon",
	"shrimp":      "shrimp",
	"stimulus":    "stimuli",
	"stratum":     "strata",
	"swine":       "swine",
	"syllabus":    "syllabi",
	"symposium":   "symposiums",
	"synopsis":    "synopses",
	"tableau":     "tableaus",
	"thesis":      "theses",
	"thief":       "thieves",
	"trout":       "trout",
	"tuna":        "tuna",
	"vertebra":    "vertebrae",
	"vita":        "vitae",
	"vortex":      "vortices",
	"wharf":       "wharves",
	"wife":        "wives",
	"wolf":        "wolves",
	"datum":       "data",
	"testis":      "testes",
	"alias":       "aliases",
	"house":       "houses",
	"shoe":        "shoes",
	"news":        "news",
	"ovum":        "ova",
	"foo":         "foos",
}

var pluralToSingle = map[string]string{}

func init() {
	for k, v := range singleToPlural {
		pluralToSingle[v] = k
	}
}
func init() {
	AddPlural("campus", "campuses")
	AddPlural("man", "men")
	AddPlural("tz", "tzes")
	AddPlural("alias", "aliases")
	AddPlural("oasis", "oasis")
	AddPlural("wife", "wives")
	AddPlural("basis", "basis")
	AddPlural("atum", "ata")
	AddPlural("adium", "adia")
	AddPlural("actus", "acti")
	AddPlural("irus", "iri")
	AddPlural("iterion", "iteria")
	AddPlural("dium", "diums")
	AddPlural("ovum", "ova")
	AddPlural("ize", "izes")
	AddPlural("dge", "dges")
	AddPlural("focus", "foci")
	AddPlural("child", "children")
	AddPlural("oaf", "oaves")
	AddPlural("randum", "randa")
	AddPlural("base", "bases")
	AddPlural("atus", "atuses")
	AddPlural("ode", "odes")
	AddPlural("person", "people")
	AddPlural("va", "vae")
	AddPlural("leus", "li")
	AddPlural("oot", "eet")
	AddPlural("oose", "eese")
	AddPlural("box", "boxes")
	AddPlural("ium", "ia")
	AddPlural("sis", "ses")
	AddPlural("nna", "nnas")
	AddPlural("eses", "esis")
	AddPlural("stis", "stes")
	AddPlural("ex", "ices")
	AddPlural("ula", "ulae")
	AddPlural("isis", "ises")
	AddPlural("ouses", "ouse")
	AddPlural("olves", "olf")
	AddPlural("lf", "lves")
	AddPlural("rf", "rves")
	AddPlural("afe", "aves")
	AddPlural("bfe", "bves")
	AddPlural("cfe", "cves")
	AddPlural("dfe", "dves")
	AddPlural("efe", "eves")
	AddPlural("gfe", "gves")
	AddPlural("hfe", "hves")
	AddPlural("ife", "ives")
	AddPlural("jfe", "jves")
	AddPlural("kfe", "kves")
	AddPlural("lfe", "lves")
	AddPlural("mfe", "mves")
	AddPlural("nfe", "nves")
	AddPlural("ofe", "oves")
	AddPlural("pfe", "pves")
	AddPlural("qfe", "qves")
	AddPlural("rfe", "rves")
	AddPlural("sfe", "sves")
	AddPlural("tfe", "tves")
	AddPlural("ufe", "uves")
	AddPlural("vfe", "vves")
	AddPlural("wfe", "wves")
	AddPlural("xfe", "xves")
	AddPlural("yfe", "yves")
	AddPlural("zfe", "zves")
	AddPlural("hive", "hives")
	AddPlural("quy", "quies")
	AddPlural("by", "bies")
	AddPlural("cy", "cies")
	AddPlural("dy", "dies")
	AddPlural("fy", "fies")
	AddPlural("gy", "gies")
	AddPlural("hy", "hies")
	AddPlural("jy", "jies")
	AddPlural("ky", "kies")
	AddPlural("ly", "lies")
	AddPlural("my", "mies")
	AddPlural("ny", "nies")
	AddPlural("py", "pies")
	AddPlural("qy", "qies")
	AddPlural("ry", "ries")
	AddPlural("sy", "sies")
	AddPlural("ty", "ties")
	AddPlural("vy", "vies")
	AddPlural("wy", "wies")
	AddPlural("xy", "xies")
	AddPlural("zy", "zies")
	AddPlural("x", "xes")
	AddPlural("ch", "ches")
	AddPlural("ss", "sses")
	AddPlural("sh", "shes")
	AddPlural("oe", "oes")
	AddPlural("io", "ios")
	AddPlural("o", "oes")
}
