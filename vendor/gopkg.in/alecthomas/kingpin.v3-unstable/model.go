// nolint: golint
package kingpin

import (
	"fmt"
	"strconv"
	"strings"
)

// Data model for Kingpin command-line structure.

type FlagGroupModel struct {
	Flags []*ClauseModel
}

func (f *FlagGroupModel) FlagByName(name string) *ClauseModel {
	for _, flag := range f.Flags {
		if flag.Name == name {
			return flag
		}
	}
	return nil
}

func (f *FlagGroupModel) FlagSummary() string {
	out := []string{}
	count := 0
	for _, flag := range f.Flags {
		if flag.Name != "help" {
			count++
		}
		if flag.Required {
			if flag.IsBoolFlag() {
				if flag.IsNegatable() {
					out = append(out, fmt.Sprintf("--[no-]%s", flag.Name))
				} else {
					out = append(out, fmt.Sprintf("--%s", flag.Name))
				}
			} else {
				out = append(out, fmt.Sprintf("--%s=%s", flag.Name, flag.FormatPlaceHolder()))
			}
		}
	}
	if count != len(out) {
		out = append(out, T("[<flags>]"))
	}
	return strings.Join(out, " ")
}

type ClauseModel struct {
	Name        string
	Help        string
	Short       rune
	Default     []string
	PlaceHolder string
	Required    bool
	Hidden      bool
	Value       Value
	Cumulative  bool
	Envar       string
}

func (f *Clause) Model() *ClauseModel {
	_, cumulative := f.value.(cumulativeValue)
	envar := f.envar
	if f.noEnvar {
		envar = ""
	}
	// TODO: How do we make the model reflect the envar transformations in the resolvers?
	return &ClauseModel{
		Name:        f.name,
		Help:        f.help,
		Short:       f.shorthand,
		Default:     f.defaultValues,
		PlaceHolder: f.placeholder,
		Required:    f.required,
		Hidden:      f.hidden,
		Value:       f.value,
		Cumulative:  cumulative,
		Envar:       envar,
	}
}

func (c *ClauseModel) String() string {
	return c.Value.String()
}

func (c *ClauseModel) IsBoolFlag() bool {
	return isBoolFlag(c.Value)
}

func (c *ClauseModel) IsNegatable() bool {
	bf, ok := c.Value.(BoolFlag)
	return ok && bf.BoolFlagIsNegatable()
}

func (c *ClauseModel) FormatPlaceHolder() string {
	if c.PlaceHolder != "" {
		return c.PlaceHolder
	}
	if len(c.Default) > 0 {
		ellipsis := ""
		if len(c.Default) > 1 {
			ellipsis = "..."
		}
		if _, ok := c.Value.(*stringValue); ok {
			return strconv.Quote(c.Default[0]) + ellipsis
		}
		return c.Default[0] + ellipsis
	}
	return strings.ToUpper(c.Name)
}

type ArgGroupModel struct {
	Args []*ClauseModel
}

func (a *ArgGroupModel) ArgSummary() string {
	depth := 0
	out := []string{}
	for _, arg := range a.Args {
		h := "<" + arg.Name + ">"
		if arg.Cumulative {
			h += " ..."
		}
		if !arg.Required {
			h = "[" + h
			depth++
		}
		out = append(out, h)
	}
	if len(out) == 0 {
		return ""
	}
	out[len(out)-1] = out[len(out)-1] + strings.Repeat("]", depth)
	return strings.Join(out, " ")
}

type CmdGroupModel struct {
	Commands []*CmdModel
}

func (c *CmdGroupModel) FlattenedCommands() (out []*CmdModel) {
	for _, cmd := range c.Commands {
		if cmd.OptionalSubcommands {
			out = append(out, cmd)
		}
		if len(cmd.Commands) == 0 {
			out = append(out, cmd)
		}
		out = append(out, cmd.FlattenedCommands()...)
	}
	return
}

type CmdModel struct {
	Name                string
	Aliases             []string
	Help                string
	Depth               int
	Hidden              bool
	Default             bool
	OptionalSubcommands bool
	Parent              *CmdModel
	*FlagGroupModel
	*ArgGroupModel
	*CmdGroupModel
}

func (c *CmdModel) String() string {
	return c.CmdSummary()
}

func (c *CmdModel) CmdSummary() string {
	out := []string{}
	for cursor := c; cursor != nil; cursor = cursor.Parent {
		text := cursor.Name
		if cursor.Default {
			text = "*" + text
		}
		if flags := cursor.FlagSummary(); flags != "" {
			text += " " + flags
		}
		if args := cursor.ArgSummary(); args != "" {
			text += " " + args
		}
		out = append([]string{text}, out...)
	}
	return strings.Join(out, " ")
}

// FullCommand is the command path to this node, excluding positional arguments and flags.
func (c *CmdModel) FullCommand() string {
	out := []string{}
	for i := c; i != nil; i = i.Parent {
		out = append([]string{i.Name}, out...)
	}
	return strings.Join(out, " ")
}

type ApplicationModel struct {
	Name    string
	Help    string
	Version string
	Author  string
	*ArgGroupModel
	*CmdGroupModel
	*FlagGroupModel
}

func (a *ApplicationModel) AppSummary() string {
	summary := a.Name
	if flags := a.FlagSummary(); flags != "" {
		summary += " " + flags
	}
	if args := a.ArgSummary(); args != "" {
		summary += " " + args
	}
	if len(a.Commands) > 0 {
		summary += " <command>"
	}
	return summary
}

func (a *ApplicationModel) FindModelForCommand(cmd *CmdClause) *CmdModel {
	if cmd == nil {
		return nil
	}
	path := []string{}
	for c := cmd; c != nil; c = c.parent {
		path = append([]string{c.name}, path...)
	}
	var selected *CmdModel
	cursor := a.CmdGroupModel
	for _, component := range path {
		for _, cmd := range cursor.Commands {
			if cmd.Name == component {
				selected = cmd
				cursor = cmd.CmdGroupModel
				break
			}
		}
	}
	if selected == nil {
		panic("this shouldn't happen")
	}
	return selected
}

func (a *Application) Model() *ApplicationModel {
	return &ApplicationModel{
		Name:           a.Name,
		Help:           a.Help,
		Version:        a.version,
		Author:         a.author,
		FlagGroupModel: a.flagGroup.Model(),
		ArgGroupModel:  a.argGroup.Model(),
		CmdGroupModel:  a.cmdGroup.Model(nil),
	}
}

func (a *argGroup) Model() *ArgGroupModel {
	m := &ArgGroupModel{}
	for _, arg := range a.args {
		m.Args = append(m.Args, arg.Model())
	}
	return m
}

func (f *flagGroup) Model() *FlagGroupModel {
	m := &FlagGroupModel{}
	for _, fl := range f.flagOrder {
		m.Flags = append(m.Flags, fl.Model())
	}
	return m
}

func (c *cmdGroup) Model(parent *CmdModel) *CmdGroupModel {
	m := &CmdGroupModel{}
	for _, cm := range c.commandOrder {
		m.Commands = append(m.Commands, cm.Model(parent))
	}
	return m
}

func (c *CmdClause) Model(parent *CmdModel) *CmdModel {
	depth := 0
	for i := c; i != nil; i = i.parent {
		depth++
	}
	cmd := &CmdModel{
		Name:                c.name,
		Parent:              parent,
		Aliases:             c.aliases,
		Help:                c.help,
		Depth:               depth,
		Hidden:              c.hidden,
		Default:             c.isDefault,
		OptionalSubcommands: c.optionalSubcommands,
		FlagGroupModel:      c.flagGroup.Model(),
		ArgGroupModel:       c.argGroup.Model(),
	}
	cmd.CmdGroupModel = c.cmdGroup.Model(cmd)
	return cmd
}
