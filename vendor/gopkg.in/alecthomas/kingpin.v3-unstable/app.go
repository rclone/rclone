package kingpin

import (
	"fmt"
	"io"
	"os"
	"strings"
)

var (
	errCommandNotSpecified = TError("command not specified")
)

// An Application contains the definitions of flags, arguments and commands
// for an application.
type Application struct {
	cmdMixin
	initialized bool

	Name string
	Help string

	author         string
	version        string
	output         io.Writer // Destination for usage.
	errors         io.Writer
	terminate      func(status int) // See Terminate()
	noInterspersed bool             // can flags be interspersed with args (or must they come first)
	envarSeparator string
	defaultEnvars  bool
	resolvers      []Resolver
	completion     bool
	helpFlag       *Clause
	helpCommand    *CmdClause
	defaultUsage   *UsageContext
}

// New creates a new Kingpin application instance.
func New(name, help string) *Application {
	a := &Application{
		Name:           name,
		Help:           help,
		output:         os.Stdout,
		errors:         os.Stderr,
		terminate:      os.Exit,
		envarSeparator: string(os.PathListSeparator),
		defaultUsage: &UsageContext{
			Template: DefaultUsageTemplate,
		},
	}
	a.flagGroup = newFlagGroup()
	a.argGroup = newArgGroup()
	a.cmdGroup = newCmdGroup(a)
	a.helpFlag = a.Flag("help", T("Show context-sensitive help.")).Action(func(e *ParseElement, c *ParseContext) error {
		c.Application.UsageForContext(c)
		c.Application.terminate(0)
		return nil
	})
	a.helpFlag.Bool()
	a.Flag("completion-bash", T("Output possible completions for the given args.")).Hidden().BoolVar(&a.completion)
	a.Flag("completion-script-bash", T("Generate completion script for bash.")).Hidden().PreAction(a.generateBashCompletionScript).Bool()
	a.Flag("completion-script-zsh", T("Generate completion script for ZSH.")).Hidden().PreAction(a.generateZSHCompletionScript).Bool()

	return a
}

// Struct allows applications to define flags with struct tags.
//
// Supported struct tags are: help, placeholder, default, short, long, required, hidden, env,
// enum, and arg.
//
// The name of the flag will default to the CamelCase name transformed to camel-case. This can
// be overridden with the "long" tag.
//
// All basic Go types are supported including floats, ints, strings, time.Duration,
// and slices of same.
//
// For compatibility, also supports the tags used by https://github.com/jessevdk/go-flags
func (a *Application) Struct(v interface{}) error {
	return a.fromStruct(nil, v)
}

func (a *Application) generateBashCompletionScript(e *ParseElement, c *ParseContext) error {
	usageContext := &UsageContext{
		Template: BashCompletionTemplate,
	}
	a.Writers(os.Stdout, os.Stderr)
	if err := a.UsageForContextWithTemplate(usageContext, c); err != nil {
		return err
	}
	a.terminate(0)
	return nil
}

func (a *Application) generateZSHCompletionScript(e *ParseElement, c *ParseContext) error {
	usageContext := &UsageContext{
		Template: ZshCompletionTemplate,
	}
	a.Writers(os.Stdout, os.Stderr)
	if err := a.UsageForContextWithTemplate(usageContext, c); err != nil {
		return err
	}
	a.terminate(0)
	return nil
}

// Action is an application-wide callback. It is used in two situations: first, with a nil "element"
// parameter when parsing is complete, and second whenever a command, argument or flag is
// encountered.
func (a *Application) Action(action Action) *Application {
	a.addAction(action)
	return a
}

// PreAction adds a callback action to be executed after flag values are parsed but before
// any other processing, such as help, completion, etc.
//
// It is called in two situations: first, with a nil "element" parameter, and second, whenever a command, argument or
// flag is encountered.
func (a *Application) PreAction(action Action) *Application {
	a.addPreAction(action)
	return a
}

// DefaultEnvars configures all flags (that do not already have an associated
// envar) to use a default environment variable in the form "<app>_<flag>".
//
// For example, if the application is named "foo" and a flag is named "bar-
// waz" the environment variable: "FOO_BAR_WAZ".
func (a *Application) DefaultEnvars() *Application {
	a.defaultEnvars = true
	return a
}

// EnvarSeparator sets the string that is used for separating values in environment variables.
//
// This defaults to the current OS's path list separator (typically : or ;).
func (a *Application) EnvarSeparator(sep string) *Application {
	a.envarSeparator = sep
	return a

}

// Resolver adds an ordered set of flag/argument resolvers.
//
// Resolvers provide default flag/argument values, from environment variables, configuration files, etc. Multiple
// resolvers may be added, and they are processed in order.
//
// The last Resolver to return a value always wins. Values returned from resolvers are not cumulative.
func (a *Application) Resolver(resolvers ...Resolver) *Application {
	a.resolvers = append(a.resolvers, resolvers...)
	return a
}

// Terminate specifies the termination handler. Defaults to os.Exit(status).
// If nil is passed, a no-op function will be used.
func (a *Application) Terminate(terminate func(int)) *Application {
	if terminate == nil {
		terminate = func(int) {}
	}
	a.terminate = terminate
	return a
}

// Writers specifies the writers to use for usage and errors. Defaults to os.Stderr.
func (a *Application) Writers(out, err io.Writer) *Application {
	a.output = out
	a.errors = err
	return a
}

// UsageTemplate specifies the text template to use when displaying usage
// information via --help. The default is DefaultUsageTemplate.
func (a *Application) UsageTemplate(template string) *Application {
	a.defaultUsage.Template = template
	return a
}

// UsageContext specifies the UsageContext to use when displaying usage
// information via --help.
func (a *Application) UsageContext(context *UsageContext) *Application {
	a.defaultUsage = context
	return a
}

// ParseContext parses the given command line and returns the fully populated
// ParseContext.
func (a *Application) ParseContext(args []string) (*ParseContext, error) {
	return a.parseContext(false, args)
}

func (a *Application) parseContext(ignoreDefault bool, args []string) (*ParseContext, error) {
	if err := a.init(); err != nil {
		return nil, err
	}
	context := tokenize(args, ignoreDefault, a.buildResolvers())
	context.Application = a
	err := parse(context, a)
	return context, err
}

// Build resolvers to emulate the envar and defaults behaviour that was previously hard-coded.
func (a *Application) buildResolvers() []Resolver {

	// .Default() has lowest priority...
	resolvers := []Resolver{defaultsResolver()}
	// Then custom resolvers...
	resolvers = append(resolvers, a.resolvers...)
	// Finally, envars are highest priority behind direct flag parsing.
	if a.defaultEnvars {
		resolvers = append(resolvers, PrefixedEnvarResolver(a.Name+"_", a.envarSeparator))
	}
	resolvers = append(resolvers, envarResolver(a.envarSeparator))

	return resolvers
}

// Parse parses command-line arguments. It returns the selected command and an
// error. The selected command will be a space separated subcommand, if
// subcommands have been configured.
//
// This will populate all flag and argument values, call all callbacks, and so
// on.
func (a *Application) Parse(args []string) (command string, err error) {
	context, parseErr := a.ParseContext(args)
	if context == nil {
		// Since we do not throw error immediately, there could be a case
		// where a context returns nil. Protect against that.
		return "", parseErr
	}

	if err = a.applyPreActions(context, !a.completion); err != nil {
		return "", err
	}

	if err = a.setDefaults(context); err != nil {
		return "", err
	}

	selected, setValuesErr := a.setValues(context)

	if a.completion {
		a.generateBashCompletion(context)
		a.terminate(0)
	} else {
		if parseErr != nil {
			return "", parseErr
		}

		a.maybeHelp(context)
		if !context.EOL() {
			return "", TError("unexpected argument '{{.Arg0}}'", V{"Arg0": context.Peek()})
		}

		if setValuesErr != nil {
			return "", setValuesErr
		}

		command, err = a.execute(context, selected)
		if err == errCommandNotSpecified {
			a.writeUsage(context, nil)
		}
	}
	return command, err
}

func (a *Application) writeUsage(context *ParseContext, err error) {
	if err != nil {
		a.Errorf("%s", err)
	}
	if err := a.UsageForContext(context); err != nil {
		panic(err)
	}
	a.terminate(1)
}

func (a *Application) maybeHelp(context *ParseContext) {
	for _, element := range context.Elements {
		if element.OneOf.Flag == a.helpFlag {
			// Re-parse the command-line ignoring defaults, so that help works correctly.
			context, _ = a.parseContext(true, context.rawArgs)
			a.writeUsage(context, nil)
		}
	}
}

// Version adds a --version flag for displaying the application version.
func (a *Application) Version(version string) *Application {
	a.version = version
	a.Flag("version", T("Show application version.")).
		PreAction(func(*ParseElement, *ParseContext) error {
			fmt.Fprintln(a.output, version)
			a.terminate(0)
			return nil
		}).
		Bool()
	return a
}

// Author sets the author name for usage templates.
func (a *Application) Author(author string) *Application {
	a.author = author
	return a
}

// Command adds a new top-level command.
func (a *Application) Command(name, help string) *CmdClause {
	return a.addCommand(name, help)
}

// Interspersed control if flags can be interspersed with positional arguments
//
// true (the default) means that they can, false means that all the flags must appear before the first positional arguments.
func (a *Application) Interspersed(interspersed bool) *Application {
	a.noInterspersed = !interspersed
	return a
}

func (a *Application) init() error {
	if a.initialized {
		return nil
	}
	if err := a.checkArgCommandMixing(); err != nil {
		return err
	}

	// If we have subcommands, add a help command at the top-level.
	if a.cmdGroup.have() {
		var command []string
		a.helpCommand = a.Command("help", T("Show help.")).
			PreAction(func(element *ParseElement, context *ParseContext) error {
				a.Usage(command)
				command = []string{}
				a.terminate(0)
				return nil
			})
		a.helpCommand.
			Arg("command", T("Show help on command.")).
			StringsVar(&command)
		// Make help first command.
		l := len(a.commandOrder)
		a.commandOrder = append(a.commandOrder[l-1:l], a.commandOrder[:l-1]...)
	}

	if err := a.flagGroup.init(); err != nil {
		return err
	}
	if err := a.cmdGroup.init(); err != nil {
		return err
	}
	if err := a.argGroup.init(); err != nil {
		return err
	}
	for _, cmd := range a.commands {
		if err := cmd.init(); err != nil {
			return err
		}
	}
	flagGroups := []*flagGroup{a.flagGroup}
	for _, cmd := range a.commandOrder {
		if err := checkDuplicateFlags(cmd, flagGroups); err != nil {
			return err
		}
	}
	a.initialized = true
	return nil
}

// Recursively check commands for duplicate flags.
func checkDuplicateFlags(current *CmdClause, flagGroups []*flagGroup) error {
	// Check for duplicates.
	for _, flags := range flagGroups {
		for _, flag := range current.flagOrder {
			if flag.shorthand != 0 {
				if _, ok := flags.short[string(flag.shorthand)]; ok {
					return TError("duplicate short flag -{{.Arg0}}", V{"Arg0": flag.shorthand})
				}
			}
			if _, ok := flags.long[flag.name]; ok {
				return TError("duplicate long flag --{{.Arg0}}", V{"Arg0": flag.name})
			}
		}
	}
	flagGroups = append(flagGroups, current.flagGroup)
	// Check subcommands.
	for _, subcmd := range current.commandOrder {
		if err := checkDuplicateFlags(subcmd, flagGroups); err != nil {
			return err
		}
	}
	return nil
}

func (a *Application) execute(context *ParseContext, selected []string) (string, error) {
	var err error

	if err = a.validateRequired(context); err != nil {
		return "", err
	}

	if err = a.applyActions(context); err != nil {
		return "", err
	}

	command := strings.Join(selected, " ")
	if command == "" && a.cmdGroup.have() {
		return "", errCommandNotSpecified
	}
	return command, err
}

func (a *Application) setDefaults(context *ParseContext) error {
	flagElements := context.Elements.FlagMap()
	argElements := context.Elements.ArgMap()

	// Check required flags and set defaults.
	for _, flag := range context.flags.long {
		if flagElements[flag.name] == nil {
			if err := flag.setDefault(context); err != nil {
				return err
			}
		} else {
			flag.reset()
		}
	}

	for _, arg := range context.arguments.args {
		if argElements[arg.name] == nil {
			if err := arg.setDefault(context); err != nil {
				return err
			}
		} else {
			arg.reset()
		}
	}

	return nil
}

func (a *Application) validateRequired(context *ParseContext) error {
	flagElements := context.Elements.FlagMap()
	argElements := context.Elements.ArgMap()

	// Check required flags and set defaults.
	for _, flag := range context.flags.long {
		if flagElements[flag.name] == nil {
			// Check required flags were provided.
			if flag.needsValue(context) {
				return TError("required flag --{{.Arg0}} not provided", V{"Arg0": flag.name})
			}
		}
	}

	for _, arg := range context.arguments.args {
		if argElements[arg.name] == nil {
			if arg.needsValue(context) {
				return TError("required argument '{{.Arg0}}' not provided", V{"Arg0": arg.name})
			}
		}
	}
	return nil
}

func (a *Application) setValues(context *ParseContext) (selected []string, err error) {
	// Set all arg and flag values.
	var (
		lastCmd *CmdClause
		flagSet = map[string]struct{}{}
	)
	for _, element := range context.Elements {
		switch {
		case element.OneOf.Flag != nil:
			clause := element.OneOf.Flag
			if _, ok := flagSet[clause.name]; ok {
				if v, ok := clause.value.(cumulativeValue); !ok || !v.IsCumulative() {
					return nil, TError("flag '{{.Arg0}}' cannot be repeated", V{"Arg0": clause.name})
				}
			}
			if err = clause.value.Set(*element.Value); err != nil {
				return
			}
			flagSet[clause.name] = struct{}{}

		case element.OneOf.Arg != nil:
			clause := element.OneOf.Arg
			if err = clause.value.Set(*element.Value); err != nil {
				return
			}

		case element.OneOf.Cmd != nil:
			clause := element.OneOf.Cmd
			if clause.validator != nil {
				if err = clause.validator(clause); err != nil {
					return
				}
			}
			selected = append(selected, clause.name)
			lastCmd = clause
		}
	}

	if lastCmd == nil || lastCmd.optionalSubcommands {
		return
	}
	if len(lastCmd.commands) > 0 {
		return nil, TError("must select a subcommand of '{{.Arg0}}'", V{"Arg0": lastCmd.FullCommand()})
	}

	return
}

// Errorf prints an error message to w in the format "<appname>: error: <message>".
func (a *Application) Errorf(format string, args ...interface{}) {
	fmt.Fprintf(a.errors, a.Name+T(": error: ")+format+"\n", args...)
}

// Fatalf writes a formatted error to w then terminates with exit status 1.
func (a *Application) Fatalf(format string, args ...interface{}) {
	a.Errorf(format, args...)
	a.terminate(1)
}

// FatalUsage prints an error message followed by usage information, then
// exits with a non-zero status.
func (a *Application) FatalUsage(format string, args ...interface{}) {
	a.Errorf(format, args...)
	a.Usage([]string{})
	a.terminate(1)
}

// FatalUsageContext writes a printf formatted error message to w, then usage
// information for the given ParseContext, before exiting.
func (a *Application) FatalUsageContext(context *ParseContext, format string, args ...interface{}) {
	a.Errorf(format, args...)
	if err := a.UsageForContext(context); err != nil {
		panic(err)
	}
	a.terminate(1)
}

// FatalIfError prints an error and exits if err is not nil. The error is printed
// with the given formatted string, if any.
func (a *Application) FatalIfError(err error, format string, args ...interface{}) {
	if err != nil {
		prefix := ""
		if format != "" {
			prefix = fmt.Sprintf(format, args...) + ": "
		}
		a.Errorf(prefix+"%s", err)
		a.terminate(1)
	}
}

func (a *Application) completionOptions(context *ParseContext) []string {
	args := context.rawArgs

	var (
		currArg string
		prevArg string
		target  cmdMixin
	)

	numArgs := len(args)
	if numArgs > 1 {
		args = args[1:]
		currArg = args[len(args)-1]
	}
	if numArgs > 2 {
		prevArg = args[len(args)-2]
	}

	target = a.cmdMixin
	if context.SelectedCommand != nil {
		// A subcommand was in use. We will use it as the target
		target = context.SelectedCommand.cmdMixin
	}

	if (currArg != "" && strings.HasPrefix(currArg, "--")) || strings.HasPrefix(prevArg, "--") {
		// Perform completion for A flag. The last/current argument started with "-"
		var (
			flagName  string // The name of a flag if given (could be half complete)
			flagValue string // The value assigned to a flag (if given) (could be half complete)
		)

		if strings.HasPrefix(prevArg, "--") && !strings.HasPrefix(currArg, "--") {
			// Matches: 	./myApp --flag value
			// Wont Match: 	./myApp --flag --
			flagName = prevArg[2:] // Strip the "--"
			flagValue = currArg
		} else if strings.HasPrefix(currArg, "--") {
			// Matches: 	./myApp --flag --
			// Matches:		./myApp --flag somevalue --
			// Matches: 	./myApp --
			flagName = currArg[2:] // Strip the "--"
		}

		options, flagMatched, valueMatched := target.FlagCompletion(flagName, flagValue)
		if valueMatched {
			// Value Matched. Show cmdCompletions
			return target.CmdCompletion(context)
		}

		// Add top level flags if we're not at the top level and no match was found.
		if context.SelectedCommand != nil && !flagMatched {
			topOptions, topFlagMatched, topValueMatched := a.FlagCompletion(flagName, flagValue)
			if topValueMatched {
				// Value Matched. Back to cmdCompletions
				return target.CmdCompletion(context)
			}

			if topFlagMatched {
				// Top level had a flag which matched the input. Return it's options.
				options = topOptions
			} else {
				// Add top level flags
				options = append(options, topOptions...)
			}
		}
		return options
	}

	// Perform completion for sub commands and arguments.
	return target.CmdCompletion(context)
}

func (a *Application) generateBashCompletion(context *ParseContext) {
	options := a.completionOptions(context)
	fmt.Printf("%s", strings.Join(options, "\n"))
}

func (a *Application) applyPreActions(context *ParseContext, dispatch bool) error {
	if !dispatch {
		return nil
	}
	if err := a.actionMixin.applyPreActions(nil, context); err != nil {
		return err
	}
	for _, element := range context.Elements {
		if err := a.actionMixin.applyPreActions(element, context); err != nil {
			return err
		}
		var applier actionApplier
		switch {
		case element.OneOf.Arg != nil:
			applier = element.OneOf.Arg
		case element.OneOf.Flag != nil:
			applier = element.OneOf.Flag
		case element.OneOf.Cmd != nil:
			applier = element.OneOf.Cmd
		}
		if err := applier.applyPreActions(element, context); err != nil {
			return err
		}
	}
	return nil
}

func (a *Application) applyActions(context *ParseContext) error {
	if err := a.actionMixin.applyActions(nil, context); err != nil {
		return err
	}
	// Dispatch to actions.
	for _, element := range context.Elements {
		if err := a.actionMixin.applyActions(element, context); err != nil {
			return err
		}
		var applier actionApplier
		switch {
		case element.OneOf.Arg != nil:
			applier = element.OneOf.Arg
		case element.OneOf.Flag != nil:
			applier = element.OneOf.Flag
		case element.OneOf.Cmd != nil:
			applier = element.OneOf.Cmd
		}
		if err := applier.applyActions(element, context); err != nil {
			return err
		}
	}
	return nil
}
