package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"text/template"
	"time"

	kingpin "gopkg.in/alecthomas/kingpin.v3-unstable"
)

var (
	// Locations to look for vendored linters.
	vendoredSearchPaths = [][]string{
		{"github.com", "alecthomas", "gometalinter", "_linters"},
		{"gopkg.in", "alecthomas", "gometalinter.v2", "_linters"},
	}
	defaultConfigPath = ".gometalinter.json"

	// Populated by goreleaser.
	version = "master"
	commit  = "?"
	date    = ""
)

func setupFlags(app *kingpin.Application) {
	app.Flag("config", "Load JSON configuration from file.").Envar("GOMETALINTER_CONFIG").Action(loadConfig).String()
	app.Flag("no-config", "Disable automatic loading of config file.").Bool()
	app.Flag("disable", "Disable previously enabled linters.").PlaceHolder("LINTER").Short('D').Action(disableAction).Strings()
	app.Flag("enable", "Enable previously disabled linters.").PlaceHolder("LINTER").Short('E').Action(enableAction).Strings()
	app.Flag("linter", "Define a linter.").PlaceHolder("NAME:COMMAND:PATTERN").Action(cliLinterOverrides).StringMap()
	app.Flag("message-overrides", "Override message from linter. {message} will be expanded to the original message.").PlaceHolder("LINTER:MESSAGE").StringMapVar(&config.MessageOverride)
	app.Flag("severity", "Map of linter severities.").PlaceHolder("LINTER:SEVERITY").StringMapVar(&config.Severity)
	app.Flag("disable-all", "Disable all linters.").Action(disableAllAction).Bool()
	app.Flag("enable-all", "Enable all linters.").Action(enableAllAction).Bool()
	app.Flag("format", "Output format.").PlaceHolder(config.Format).StringVar(&config.Format)
	app.Flag("vendored-linters", "Use vendored linters (recommended) (DEPRECATED - use binary packages).").BoolVar(&config.VendoredLinters)
	app.Flag("fast", "Only run fast linters.").BoolVar(&config.Fast)
	app.Flag("install", "Attempt to install all known linters (DEPRECATED - use binary packages).").Short('i').BoolVar(&config.Install)
	app.Flag("update", "Pass -u to go tool when installing (DEPRECATED - use binary packages).").Short('u').BoolVar(&config.Update)
	app.Flag("force", "Pass -f to go tool when installing (DEPRECATED - use binary packages).").Short('f').BoolVar(&config.Force)
	app.Flag("download-only", "Pass -d to go tool when installing (DEPRECATED - use binary packages).").BoolVar(&config.DownloadOnly)
	app.Flag("debug", "Display messages for failed linters, etc.").Short('d').BoolVar(&config.Debug)
	app.Flag("concurrency", "Number of concurrent linters to run.").PlaceHolder(fmt.Sprintf("%d", runtime.NumCPU())).Short('j').IntVar(&config.Concurrency)
	app.Flag("exclude", "Exclude messages matching these regular expressions.").Short('e').PlaceHolder("REGEXP").StringsVar(&config.Exclude)
	app.Flag("include", "Include messages matching these regular expressions.").Short('I').PlaceHolder("REGEXP").StringsVar(&config.Include)
	app.Flag("skip", "Skip directories with this name when expanding '...'.").Short('s').PlaceHolder("DIR...").StringsVar(&config.Skip)
	app.Flag("vendor", "Enable vendoring support (skips 'vendor' directories and sets GO15VENDOREXPERIMENT=1).").BoolVar(&config.Vendor)
	app.Flag("cyclo-over", "Report functions with cyclomatic complexity over N (using gocyclo).").PlaceHolder("10").IntVar(&config.Cyclo)
	app.Flag("line-length", "Report lines longer than N (using lll).").PlaceHolder("80").IntVar(&config.LineLength)
	app.Flag("misspell-locale", "Specify locale to use (using misspell).").PlaceHolder("").StringVar(&config.MisspellLocale)
	app.Flag("min-confidence", "Minimum confidence interval to pass to golint.").PlaceHolder(".80").FloatVar(&config.MinConfidence)
	app.Flag("min-occurrences", "Minimum occurrences to pass to goconst.").PlaceHolder("3").IntVar(&config.MinOccurrences)
	app.Flag("min-const-length", "Minimum constant length.").PlaceHolder("3").IntVar(&config.MinConstLength)
	app.Flag("dupl-threshold", "Minimum token sequence as a clone for dupl.").PlaceHolder("50").IntVar(&config.DuplThreshold)
	app.Flag("sort", fmt.Sprintf("Sort output by any of %s.", strings.Join(sortKeys, ", "))).PlaceHolder("none").EnumsVar(&config.Sort, sortKeys...)
	app.Flag("tests", "Include test files for linters that support this option.").Short('t').BoolVar(&config.Test)
	app.Flag("deadline", "Cancel linters if they have not completed within this duration.").PlaceHolder("30s").DurationVar((*time.Duration)(&config.Deadline))
	app.Flag("errors", "Only show errors.").BoolVar(&config.Errors)
	app.Flag("json", "Generate structured JSON rather than standard line-based output.").BoolVar(&config.JSON)
	app.Flag("checkstyle", "Generate checkstyle XML rather than standard line-based output.").BoolVar(&config.Checkstyle)
	app.Flag("enable-gc", "Enable GC for linters (useful on large repositories).").BoolVar(&config.EnableGC)
	app.Flag("aggregate", "Aggregate issues reported by several linters.").BoolVar(&config.Aggregate)
	app.Flag("warn-unmatched-nolint", "Warn if a nolint directive is not matched with an issue.").BoolVar(&config.WarnUnmatchedDirective)
	app.GetFlag("help").Short('h')
}

func cliLinterOverrides(app *kingpin.Application, element *kingpin.ParseElement, ctx *kingpin.ParseContext) error {
	// expected input structure - <name>:<command-spec>
	parts := strings.SplitN(*element.Value, ":", 2)
	if len(parts) < 2 {
		return fmt.Errorf("incorrectly formatted input: %s", *element.Value)
	}
	name := parts[0]
	spec := parts[1]
	conf, err := parseLinterConfigSpec(name, spec)
	if err != nil {
		return fmt.Errorf("incorrectly formatted input: %s", *element.Value)
	}
	config.Linters[name] = StringOrLinterConfig(conf)
	return nil
}

func loadDefaultConfig(app *kingpin.Application, element *kingpin.ParseElement, ctx *kingpin.ParseContext) error {
	if element != nil {
		return nil
	}

	for _, elem := range ctx.Elements {
		if f := elem.OneOf.Flag; f == app.GetFlag("config") || f == app.GetFlag("no-config") {
			return nil
		}
	}

	configFile, found, err := findDefaultConfigFile()
	if err != nil || !found {
		return err
	}

	return loadConfigFile(configFile)
}

func loadConfig(app *kingpin.Application, element *kingpin.ParseElement, ctx *kingpin.ParseContext) error {
	return loadConfigFile(*element.Value)
}

func disableAction(app *kingpin.Application, element *kingpin.ParseElement, ctx *kingpin.ParseContext) error {
	out := []string{}
	for _, linter := range config.Enable {
		if linter != *element.Value {
			out = append(out, linter)
		}
	}
	config.Enable = out
	return nil
}

func enableAction(app *kingpin.Application, element *kingpin.ParseElement, ctx *kingpin.ParseContext) error {
	config.Enable = append(config.Enable, *element.Value)
	return nil
}

func disableAllAction(app *kingpin.Application, element *kingpin.ParseElement, ctx *kingpin.ParseContext) error {
	config.Enable = []string{}
	return nil
}

func enableAllAction(app *kingpin.Application, element *kingpin.ParseElement, ctx *kingpin.ParseContext) error {
	for linter := range defaultLinters {
		config.Enable = append(config.Enable, linter)
	}
	config.EnableAll = true
	return nil
}

type debugFunction func(format string, args ...interface{})

func debug(format string, args ...interface{}) {
	if config.Debug {
		t := time.Now().UTC()
		fmt.Fprintf(os.Stderr, "DEBUG: [%s] ", t.Format(time.StampMilli))
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}

func namespacedDebug(prefix string) debugFunction {
	return func(format string, args ...interface{}) {
		debug(prefix+format, args...)
	}
}

func warning(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "WARNING: "+format+"\n", args...)
}

func formatLinters() string {
	nameToLinter := map[string]*Linter{}
	var linterNames []string
	for _, linter := range getDefaultLinters() {
		linterNames = append(linterNames, linter.Name)
		nameToLinter[linter.Name] = linter
	}
	sort.Strings(linterNames)

	w := bytes.NewBuffer(nil)
	for _, linterName := range linterNames {
		linter := nameToLinter[linterName]

		install := "(" + linter.InstallFrom + ")"
		if install == "()" {
			install = ""
		}
		fmt.Fprintf(w, "  %s: %s\n\tcommand: %s\n\tregex: %s\n\tfast: %t\n\tdefault enabled: %t\n\n",
			linter.Name, install, linter.Command, linter.Pattern, linter.IsFast, linter.defaultEnabled)
	}
	return w.String()
}

func formatSeverity() string {
	w := bytes.NewBuffer(nil)
	for name, severity := range config.Severity {
		fmt.Fprintf(w, "  %s -> %s\n", name, severity)
	}
	return w.String()
}

func main() {
	kingpin.Version(fmt.Sprintf("gometalinter version %s built from %s on %s", version, commit, date))
	pathsArg := kingpin.Arg("path", "Directories to lint. Defaults to \".\". <path>/... will recurse.").Strings()
	app := kingpin.CommandLine
	app.Action(loadDefaultConfig)
	setupFlags(app)
	app.Help = fmt.Sprintf(`Aggregate and normalise the output of a whole bunch of Go linters.

PlaceHolder linters:

%s

Severity override map (default is "warning"):

%s
`, formatLinters(), formatSeverity())
	kingpin.Parse()

	if config.Install {
		if config.VendoredLinters {
			configureEnvironmentForInstall()
		}
		installLinters()
		return
	}

	configureEnvironment()
	include, exclude := processConfig(config)

	start := time.Now()
	paths := resolvePaths(*pathsArg, config.Skip)

	linters := lintersFromConfig(config)
	err := validateLinters(linters, config)
	kingpin.FatalIfError(err, "")

	issues, errch := runLinters(linters, paths, config.Concurrency, exclude, include)
	status := 0
	if config.JSON {
		status |= outputToJSON(issues)
	} else if config.Checkstyle {
		status |= outputToCheckstyle(issues)
	} else {
		status |= outputToConsole(issues)
	}
	for err := range errch {
		warning("%s", err)
		status |= 2
	}
	elapsed := time.Since(start)
	debug("total elapsed time %s", elapsed)
	os.Exit(status)
}

// nolint: gocyclo
func processConfig(config *Config) (include *regexp.Regexp, exclude *regexp.Regexp) {
	tmpl, err := template.New("output").Parse(config.Format)
	kingpin.FatalIfError(err, "invalid format %q", config.Format)
	config.formatTemplate = tmpl

	// Ensure that gometalinter manages threads, not linters.
	os.Setenv("GOMAXPROCS", "1")
	// Force sorting by path if checkstyle mode is selected
	// !jsonFlag check is required to handle:
	// 	gometalinter --json --checkstyle --sort=severity
	if config.Checkstyle && !config.JSON {
		config.Sort = []string{"path"}
	}

	// PlaceHolder to skipping "vendor" directory if GO15VENDOREXPERIMENT=1 is enabled.
	// TODO(alec): This will probably need to be enabled by default at a later time.
	if os.Getenv("GO15VENDOREXPERIMENT") == "1" || config.Vendor {
		if err := os.Setenv("GO15VENDOREXPERIMENT", "1"); err != nil {
			warning("setenv GO15VENDOREXPERIMENT: %s", err)
		}
		config.Skip = append(config.Skip, "vendor")
		config.Vendor = true
	}
	if len(config.Exclude) > 0 {
		exclude = regexp.MustCompile(strings.Join(config.Exclude, "|"))
	}

	if len(config.Include) > 0 {
		include = regexp.MustCompile(strings.Join(config.Include, "|"))
	}

	runtime.GOMAXPROCS(config.Concurrency)
	return include, exclude
}

func outputToConsole(issues chan *Issue) int {
	status := 0
	for issue := range issues {
		if config.Errors && issue.Severity != Error {
			continue
		}
		fmt.Println(issue.String())
		status = 1
	}
	return status
}

func outputToJSON(issues chan *Issue) int {
	fmt.Println("[")
	status := 0
	for issue := range issues {
		if config.Errors && issue.Severity != Error {
			continue
		}
		if status != 0 {
			fmt.Printf(",\n")
		}
		d, err := json.Marshal(issue)
		kingpin.FatalIfError(err, "")
		fmt.Printf("  %s", d)
		status = 1
	}
	fmt.Printf("\n]\n")
	return status
}

func resolvePaths(paths, skip []string) []string {
	if len(paths) == 0 {
		return []string{"."}
	}

	skipPath := newPathFilter(skip)
	dirs := newStringSet()
	for _, path := range paths {
		if strings.HasSuffix(path, "/...") {
			root := filepath.Dir(path)
			_ = filepath.Walk(root, func(p string, i os.FileInfo, err error) error {
				if err != nil {
					warning("invalid path %q: %s", p, err)
					return err
				}

				skip := skipPath(p)
				switch {
				case i.IsDir() && skip:
					return filepath.SkipDir
				case !i.IsDir() && !skip && strings.HasSuffix(p, ".go"):
					dirs.add(filepath.Clean(filepath.Dir(p)))
				}
				return nil
			})
		} else {
			dirs.add(filepath.Clean(path))
		}
	}
	out := make([]string, 0, dirs.size())
	for _, d := range dirs.asSlice() {
		out = append(out, relativePackagePath(d))
	}
	sort.Strings(out)
	for _, d := range out {
		debug("linting path %s", d)
	}
	return out
}

func newPathFilter(skip []string) func(string) bool {
	filter := map[string]bool{}
	for _, name := range skip {
		filter[name] = true
	}

	return func(path string) bool {
		base := filepath.Base(path)
		if filter[base] || filter[path] {
			return true
		}
		return base != "." && base != ".." && strings.ContainsAny(base[0:1], "_.")
	}
}

func relativePackagePath(dir string) string {
	if filepath.IsAbs(dir) || strings.HasPrefix(dir, ".") {
		return dir
	}
	// package names must start with a ./
	return "./" + dir
}

func lintersFromConfig(config *Config) map[string]*Linter {
	out := map[string]*Linter{}
	config.Enable = replaceWithMegacheck(config.Enable, config.EnableAll)
	for _, name := range config.Enable {
		linter := getLinterByName(name, LinterConfig(config.Linters[name]))
		if config.Fast && !linter.IsFast {
			continue
		}
		out[name] = linter
	}
	for _, linter := range config.Disable {
		delete(out, linter)
	}
	return out
}

// replaceWithMegacheck checks enabled linters if they duplicate megacheck and
// returns a either a revised list removing those and adding megacheck or an
// unchanged slice. Emits a warning if linters were removed and swapped with
// megacheck.
func replaceWithMegacheck(enabled []string, enableAll bool) []string {
	var (
		staticcheck,
		gosimple,
		unused bool
		revised []string
	)
	for _, linter := range enabled {
		switch linter {
		case "staticcheck":
			staticcheck = true
		case "gosimple":
			gosimple = true
		case "unused":
			unused = true
		case "megacheck":
			// Don't add to revised slice, we'll add it later
		default:
			revised = append(revised, linter)
		}
	}
	if staticcheck && gosimple && unused {
		if !enableAll {
			warning("staticcheck, gosimple and unused are all set, using megacheck instead")
		}
		return append(revised, "megacheck")
	}
	return enabled
}

func findVendoredLinters() string {
	gopaths := getGoPathList()
	for _, home := range vendoredSearchPaths {
		for _, p := range gopaths {
			joined := append([]string{p, "src"}, home...)
			vendorRoot := filepath.Join(joined...)
			if _, err := os.Stat(vendorRoot); err == nil {
				return vendorRoot
			}
		}
	}
	return ""
}

// Go 1.8 compatible GOPATH.
func getGoPath() string {
	path := os.Getenv("GOPATH")
	if path == "" {
		user, err := user.Current()
		kingpin.FatalIfError(err, "")
		path = filepath.Join(user.HomeDir, "go")
	}
	return path
}

func getGoPathList() []string {
	return strings.Split(getGoPath(), string(os.PathListSeparator))
}

// addPath appends path to paths if path does not already exist in paths. Returns
// the new paths.
func addPath(paths []string, path string) []string {
	for _, existingpath := range paths {
		if path == existingpath {
			return paths
		}
	}
	return append(paths, path)
}

// configureEnvironment adds all `bin/` directories from $GOPATH to $PATH
func configureEnvironment() {
	paths := addGoBinsToPath(getGoPathList())
	setEnv("PATH", strings.Join(paths, string(os.PathListSeparator)))
	setEnv("GOROOT", discoverGoRoot())
	debugPrintEnv()
}

func discoverGoRoot() string {
	goroot := os.Getenv("GOROOT")
	if goroot == "" {
		output, err := exec.Command("go", "env", "GOROOT").Output()
		kingpin.FatalIfError(err, "could not find go binary")
		goroot = string(output)
	}
	return strings.TrimSpace(goroot)
}

func addGoBinsToPath(gopaths []string) []string {
	paths := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))
	for _, p := range gopaths {
		paths = addPath(paths, filepath.Join(p, "bin"))
	}
	gobin := os.Getenv("GOBIN")
	if gobin != "" {
		paths = addPath(paths, gobin)
	}
	return paths
}

// configureEnvironmentForInstall sets GOPATH and GOBIN so that vendored linters
// can be installed
func configureEnvironmentForInstall() {
	if config.Update {
		warning(`Linters are now vendored by default, --update ignored. The original
behaviour can be re-enabled with --no-vendored-linters.

To request an update for a vendored linter file an issue at:
https://github.com/alecthomas/gometalinter/issues/new
`)
	}
	gopaths := getGoPathList()
	vendorRoot := findVendoredLinters()
	if vendorRoot == "" {
		kingpin.Fatalf("could not find vendored linters in GOPATH=%q", getGoPath())
	}
	debug("found vendored linters at %s, updating environment", vendorRoot)

	gobin := os.Getenv("GOBIN")
	if gobin == "" {
		gobin = filepath.Join(gopaths[0], "bin")
	}
	setEnv("GOBIN", gobin)

	// "go install" panics when one GOPATH element is beneath another, so set
	// GOPATH to the vendor root
	setEnv("GOPATH", vendorRoot)
	debugPrintEnv()
}

func setEnv(key, value string) {
	if err := os.Setenv(key, value); err != nil {
		warning("setenv %s: %s", key, err)
	} else {
		debug("setenv %s=%q", key, value)
	}
}

func debugPrintEnv() {
	debug("Current environment:")
	debug("PATH=%q", os.Getenv("PATH"))
	debug("GOPATH=%q", os.Getenv("GOPATH"))
	debug("GOBIN=%q", os.Getenv("GOBIN"))
	debug("GOROOT=%q", os.Getenv("GOROOT"))
}
