package kingpin

type argGroup struct {
	args []*Clause
}

func newArgGroup() *argGroup {
	return &argGroup{}
}

func (a *argGroup) have() bool {
	return len(a.args) > 0
}

// GetArg gets an argument definition.
//
// This allows existing arguments to be modified after definition but before parsing. Useful for
// modular applications.
func (a *argGroup) GetArg(name string) *Clause {
	for _, arg := range a.args {
		if arg.name == name {
			return arg
		}
	}
	return nil
}

func (a *argGroup) Arg(name, help string) *Clause {
	arg := NewClause(name, help)
	a.args = append(a.args, arg)
	return arg
}

func (a *argGroup) init() error {
	required := 0
	seen := map[string]struct{}{}
	previousArgMustBeLast := false
	for i, arg := range a.args {
		if previousArgMustBeLast {
			return TError("Args() can't be followed by another argument '{{.Arg0}}'", V{"Arg0": arg.name})
		}
		if arg.consumesRemainder() {
			previousArgMustBeLast = true
		}
		if _, ok := seen[arg.name]; ok {
			return TError("duplicate argument '{{.Arg0}}'", V{"Arg0": arg.name})
		}
		seen[arg.name] = struct{}{}
		if arg.required && required != i {
			return TError("required arguments found after non-required")
		}
		if arg.required {
			required++
		}
		if err := arg.init(); err != nil {
			return err
		}
	}
	return nil
}
