package kingpin

import (
	"bufio"
	"os"
	"strings"
	"unicode/utf8"
)

type TokenType int

// Token types.
const (
	TokenShort TokenType = iota
	TokenLong
	TokenArg
	TokenError
	TokenEOL
)

func (t TokenType) String() string {
	switch t {
	case TokenShort:
		return T("short flag")
	case TokenLong:
		return T("long flag")
	case TokenArg:
		return T("argument")
	case TokenError:
		return T("error")
	case TokenEOL:
		return T("<EOL>")
	}
	return T("unknown")
}

var (
	TokenEOLMarker = Token{-1, TokenEOL, ""}
)

type Token struct {
	Index int
	Type  TokenType
	Value string
}

func (t *Token) Equal(o *Token) bool {
	return t.Index == o.Index
}

func (t *Token) IsFlag() bool {
	return t.Type == TokenShort || t.Type == TokenLong
}

func (t *Token) IsEOF() bool {
	return t.Type == TokenEOL
}

func (t *Token) String() string {
	switch t.Type {
	case TokenShort:
		return "-" + t.Value
	case TokenLong:
		return "--" + t.Value
	case TokenArg:
		return t.Value
	case TokenError:
		return T("error: ") + t.Value
	case TokenEOL:
		return T("<EOL>")
	default:
		panic("unhandled type")
	}
}

type OneOfClause struct {
	Flag *Clause
	Arg  *Clause
	Cmd  *CmdClause
}

// A ParseElement represents the parsers view of each element in the command-line argument slice.
type ParseElement struct {
	// Clause associated with this element. Exactly one of these will be present.
	OneOf OneOfClause
	// Value is corresponding value for an argument or flag. For commands this value will be nil.
	Value *string
}

// ParseElements represents each element in the command-line argument slice.
type ParseElements []*ParseElement

// FlagMap collects all parsed flags into a map keyed by long name.
func (p ParseElements) FlagMap() map[string]*ParseElement {
	// Collect flags into maps.
	flags := map[string]*ParseElement{}
	for _, element := range p {
		if element.OneOf.Flag != nil {
			flags[element.OneOf.Flag.name] = element
		}
	}
	return flags
}

// ArgMap collects all parsed positional arguments into a map keyed by long name.
func (p ParseElements) ArgMap() map[string]*ParseElement {
	flags := map[string]*ParseElement{}
	for _, element := range p {
		if element.OneOf.Arg != nil {
			flags[element.OneOf.Arg.name] = element
		}
	}
	return flags
}

// ParseContext holds the current context of the parser. When passed to
// Action() callbacks Elements will be fully populated with *FlagClause,
// *ArgClause and *CmdClause values and their corresponding arguments (if
// any).
type ParseContext struct {
	Application     *Application // May be nil in tests.
	SelectedCommand *CmdClause
	resolvers       []Resolver
	ignoreDefault   bool
	argsOnly        bool
	peek            []*Token
	argi            int // Index of current command-line arg we're processing.
	args            []string
	rawArgs         []string
	flags           *flagGroup
	arguments       *argGroup
	argumenti       int // Cursor into arguments
	// Flags, arguments and commands encountered and collected during parse.
	Elements ParseElements
}

func (p *ParseContext) CombinedFlagsAndArgs() []*Clause {
	return append(p.Args(), p.Flags()...)
}

func (p *ParseContext) Args() []*Clause {
	return p.arguments.args
}

func (p *ParseContext) Flags() []*Clause {
	return p.flags.flagOrder
}

// LastCmd returns true if the element is the last (sub)command being evaluated.
func (p *ParseContext) LastCmd(element *ParseElement) bool {
	lastCmdIndex := -1
	eIndex := -2
	for i, e := range p.Elements {
		if element == e {
			eIndex = i
		}

		if e.OneOf.Cmd != nil {
			lastCmdIndex = i
		}
	}
	return lastCmdIndex == eIndex
}

func (p *ParseContext) nextArg() *Clause {
	if p.argumenti >= len(p.arguments.args) {
		return nil
	}
	arg := p.arguments.args[p.argumenti]
	if !arg.consumesRemainder() {
		p.argumenti++
	}
	return arg
}

func (p *ParseContext) next() {
	p.argi++
	p.args = p.args[1:]
}

// HasTrailingArgs returns true if there are unparsed command-line arguments.
// This can occur if the parser can not match remaining arguments.
func (p *ParseContext) HasTrailingArgs() bool {
	return len(p.args) > 0
}

func tokenize(args []string, ignoreDefault bool, resolvers []Resolver) *ParseContext {
	return &ParseContext{
		ignoreDefault: ignoreDefault,
		args:          args,
		rawArgs:       args,
		flags:         newFlagGroup(),
		arguments:     newArgGroup(),
		resolvers:     resolvers,
	}
}

func (p *ParseContext) mergeFlags(flags *flagGroup) {
	for _, flag := range flags.flagOrder {
		if flag.shorthand != 0 {
			p.flags.short[string(flag.shorthand)] = flag
		}
		p.flags.long[flag.name] = flag
		p.flags.flagOrder = append(p.flags.flagOrder, flag)
	}
}

func (p *ParseContext) mergeArgs(args *argGroup) {
	p.arguments.args = append(p.arguments.args, args.args...)
}

func (p *ParseContext) EOL() bool {
	return p.Peek().Type == TokenEOL
}

// Next token in the parse context.
func (p *ParseContext) Next() *Token {
	if len(p.peek) > 0 {
		return p.pop()
	}

	// End of tokens.
	if len(p.args) == 0 {
		return &Token{Index: p.argi, Type: TokenEOL}
	}

	arg := p.args[0]
	p.next()

	if p.argsOnly {
		return &Token{p.argi, TokenArg, arg}
	}

	// All remaining args are passed directly.
	if arg == "--" {
		p.argsOnly = true
		return p.Next()
	}

	if strings.HasPrefix(arg, "--") {
		parts := strings.SplitN(arg[2:], "=", 2)
		token := &Token{p.argi, TokenLong, parts[0]}
		if len(parts) == 2 {
			p.Push(&Token{p.argi, TokenArg, parts[1]})
		}
		return token
	}

	if strings.HasPrefix(arg, "-") {
		if len(arg) == 1 {
			return &Token{Index: p.argi, Type: TokenShort}
		}
		rn, size := utf8.DecodeRuneInString(arg[1:])
		short := string(rn)
		flag, ok := p.flags.short[short]
		// Not a known short flag, we'll just return it anyway.
		if !ok {
		} else if isBoolFlag(flag.value) {
			// Bool short flag.
		} else {
			// Short flag with combined argument: -fARG
			token := &Token{p.argi, TokenShort, short}
			if len(arg) > 2 {
				p.Push(&Token{p.argi, TokenArg, arg[1+size:]})
			}
			return token
		}

		if len(arg) > 1+size {
			p.args = append([]string{"-" + arg[1+size:]}, p.args...)
		}
		return &Token{p.argi, TokenShort, short}
	}

	return &Token{p.argi, TokenArg, arg}
}

func (p *ParseContext) Peek() *Token {
	if len(p.peek) == 0 {
		return p.Push(p.Next())
	}
	return p.peek[len(p.peek)-1]
}

func (p *ParseContext) Push(token *Token) *Token {
	p.peek = append(p.peek, token)
	return token
}

func (p *ParseContext) pop() *Token {
	end := len(p.peek) - 1
	token := p.peek[end]
	p.peek = p.peek[0:end]
	return token
}

func (p *ParseContext) String() string {
	if p.SelectedCommand == nil {
		return ""
	}
	return p.SelectedCommand.FullCommand()
}

func (p *ParseContext) matchedFlag(flag *Clause, value string) {
	p.Elements = append(p.Elements, &ParseElement{OneOf: OneOfClause{Flag: flag}, Value: &value})
}

func (p *ParseContext) matchedArg(arg *Clause, value string) {
	p.Elements = append(p.Elements, &ParseElement{OneOf: OneOfClause{Arg: arg}, Value: &value})
}

func (p *ParseContext) matchedCmd(cmd *CmdClause) {
	p.Elements = append(p.Elements, &ParseElement{OneOf: OneOfClause{Cmd: cmd}})
	p.mergeFlags(cmd.flagGroup)
	p.mergeArgs(cmd.argGroup)
	p.SelectedCommand = cmd
}

// Expand arguments from a file. Lines starting with # will be treated as comments.
func ExpandArgsFromFile(filename string) (out []string, err error) {
	r, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	err = scanner.Err()
	return
}

func parse(context *ParseContext, app *Application) (err error) { // nolint: gocyclo
	context.mergeFlags(app.flagGroup)
	context.mergeArgs(app.argGroup)

	cmds := app.cmdGroup
	ignoreDefault := context.ignoreDefault

loop:
	for !context.EOL() {
		token := context.Peek()

		switch token.Type {
		case TokenLong, TokenShort:
			if flag, err := context.flags.parse(context); err != nil {
				if !ignoreDefault {
					if cmd := cmds.defaultSubcommand(); cmd != nil {
						cmd.completionAlts = cmds.cmdNames()
						context.matchedCmd(cmd)
						cmds = cmd.cmdGroup
						break
					}
				}
				return err
			} else if flag == app.helpFlag {
				ignoreDefault = true
			}

		case TokenArg:
			if context.arguments.have() {
				if app.noInterspersed {
					// no more flags
					context.argsOnly = true
				}
				arg := context.nextArg()
				if arg != nil {
					context.matchedArg(arg, token.String())
					context.Next()
					continue
				}
			}

			if cmds.have() {
				selectedDefault := false
				cmd, ok := cmds.commands[token.String()]
				if !ok {
					if !ignoreDefault {
						if cmd = cmds.defaultSubcommand(); cmd != nil {
							cmd.completionAlts = cmds.cmdNames()
							selectedDefault = true
						}
					}
					if cmd == nil {
						return TError("expected command but got {{.Arg0}}", V{"Arg0": token})
					}
				}
				if cmd == app.helpCommand {
					ignoreDefault = true
				}
				cmd.completionAlts = nil
				context.matchedCmd(cmd)
				cmds = cmd.cmdGroup
				if !selectedDefault {
					context.Next()
				}
				continue
			}

			break loop

		case TokenEOL:
			break loop
		}
	}

	// Move to innermost default command.
	for !ignoreDefault {
		if cmd := cmds.defaultSubcommand(); cmd != nil {
			cmd.completionAlts = cmds.cmdNames()
			context.matchedCmd(cmd)
			cmds = cmd.cmdGroup
		} else {
			break
		}
	}

	if !context.EOL() {
		return TError("unexpected '{{.Arg0}}'", V{"Arg0": context.Peek()})
	}

	// Set defaults for all remaining args.
	for arg := context.nextArg(); arg != nil && !arg.consumesRemainder(); arg = context.nextArg() {
		for _, defaultValue := range arg.defaultValues {
			if err := arg.value.Set(defaultValue); err != nil {
				return TError("invalid default value '{{.Arg0}}' for argument '{{.Arg1}}'", V{"Arg0": defaultValue, "Arg1": arg.name})
			}
		}
	}

	return
}
