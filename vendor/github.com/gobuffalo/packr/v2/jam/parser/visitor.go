package parser

import (
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gobuffalo/genny"
	"github.com/gobuffalo/genny/movinglater/gotools"
	"github.com/pkg/errors"
)

type Visitor struct {
	File    genny.File
	Package string
	boxes   map[string]*Box
	errors  []error
}

func NewVisitor(f *File) *Visitor {
	return &Visitor{
		File:   f,
		boxes:  map[string]*Box{},
		errors: []error{},
	}
}

func (v *Visitor) Run() (Boxes, error) {
	var boxes Boxes
	pf, err := gotools.ParseFile(v.File)
	if err != nil {
		return boxes, errors.Wrap(err, v.File.Name())
	}

	v.Package = pf.Ast.Name.Name
	ast.Walk(v, pf.Ast)

	for _, vb := range v.boxes {
		boxes = append(boxes, vb)
	}

	sort.Slice(boxes, func(i, j int) bool {
		return boxes[i].Name < boxes[j].Name
	})

	if len(v.errors) > 0 {
		s := make([]string, len(v.errors))
		for i, e := range v.errors {
			s[i] = e.Error()
		}
		return boxes, errors.Wrap(errors.New(strings.Join(s, "\n")), v.File.Name())
	}
	return boxes, nil
}

func (v *Visitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return v
	}
	if err := v.eval(node); err != nil {
		v.errors = append(v.errors, err)
	}

	return v
}

func (v *Visitor) eval(node ast.Node) error {
	switch t := node.(type) {
	case *ast.CallExpr:
		return v.evalExpr(t)
	case *ast.Ident:
		return v.evalIdent(t)
	case *ast.GenDecl:
		for _, n := range t.Specs {
			if err := v.eval(n); err != nil {
				return errors.WithStack(err)
			}
		}
	case *ast.FuncDecl:
		if t.Body == nil {
			return nil
		}
		for _, b := range t.Body.List {
			if err := v.evalStmt(b); err != nil {
				return errors.WithStack(err)
			}
		}
		return nil
	case *ast.ValueSpec:
		for _, e := range t.Values {
			if err := v.evalExpr(e); err != nil {
				return errors.WithStack(err)
			}
		}
	}
	return nil
}

func (v *Visitor) evalStmt(stmt ast.Stmt) error {
	switch t := stmt.(type) {
	case *ast.ExprStmt:
		return v.evalExpr(t.X)
	case *ast.AssignStmt:
		for _, e := range t.Rhs {
			if err := v.evalArgs(e); err != nil {
				return errors.WithStack(err)
			}
		}
	}
	return nil
}

func (v *Visitor) evalExpr(expr ast.Expr) error {
	switch t := expr.(type) {
	case *ast.CallExpr:
		if t.Fun == nil {
			return nil
		}
		for _, a := range t.Args {
			switch at := a.(type) {
			case *ast.CallExpr:
				if sel, ok := t.Fun.(*ast.SelectorExpr); ok {
					return v.evalSelector(at, sel)
				}

				if err := v.evalArgs(at); err != nil {
					return errors.WithStack(err)
				}
			case *ast.CompositeLit:
				for _, e := range at.Elts {
					if err := v.evalExpr(e); err != nil {
						return errors.WithStack(err)
					}
				}
			}
		}
		if ft, ok := t.Fun.(*ast.SelectorExpr); ok {
			return v.evalSelector(t, ft)
		}
	case *ast.KeyValueExpr:
		return v.evalExpr(t.Value)
	}
	return nil
}

func (v *Visitor) evalArgs(expr ast.Expr) error {
	switch at := expr.(type) {
	case *ast.CompositeLit:
		for _, e := range at.Elts {
			if err := v.evalExpr(e); err != nil {
				return errors.WithStack(err)
			}
		}
	case *ast.CallExpr:
		if at.Fun == nil {
			return nil
		}
		switch st := at.Fun.(type) {
		case *ast.SelectorExpr:
			if err := v.evalSelector(at, st); err != nil {
				return errors.WithStack(err)
			}
		case *ast.Ident:
			return v.evalIdent(st)
		}
		for _, a := range at.Args {
			if err := v.evalArgs(a); err != nil {
				return errors.WithStack(err)
			}
		}
	}
	return nil
}

func (v *Visitor) evalSelector(expr *ast.CallExpr, sel *ast.SelectorExpr) error {
	x, ok := sel.X.(*ast.Ident)
	if !ok {
		return nil
	}
	if x.Name == "packr" {
		switch sel.Sel.Name {
		case "New":
			if len(expr.Args) != 2 {
				return errors.New("`New` requires two arguments")
			}

			zz := func(e ast.Expr) (string, error) {
				switch at := e.(type) {
				case *ast.Ident:
					switch at.Obj.Kind {
					case ast.Var:
						if as, ok := at.Obj.Decl.(*ast.AssignStmt); ok {
							return v.fromVariable(as)
						}
					case ast.Con:
						if vs, ok := at.Obj.Decl.(*ast.ValueSpec); ok {
							return v.fromConstant(vs)
						}
					}
					return "", v.evalIdent(at)
				case *ast.BasicLit:
					return at.Value, nil
				case *ast.CallExpr:
					return "", v.evalExpr(at)
				}
				return "", errors.Errorf("can't handle %T", e)
			}

			k1, err := zz(expr.Args[0])
			if err != nil {
				return errors.WithStack(err)
			}
			k2, err := zz(expr.Args[1])
			if err != nil {
				return errors.WithStack(err)
			}
			v.addBox(k1, k2)

			return nil
		case "NewBox":
			for _, e := range expr.Args {
				switch at := e.(type) {
				case *ast.Ident:
					switch at.Obj.Kind {
					case ast.Var:
						if as, ok := at.Obj.Decl.(*ast.AssignStmt); ok {
							v.addVariable("", as)
						}
					case ast.Con:
						if vs, ok := at.Obj.Decl.(*ast.ValueSpec); ok {
							v.addConstant("", vs)
						}
					}
					return v.evalIdent(at)
				case *ast.BasicLit:
					v.addBox("", at.Value)
				case *ast.CallExpr:
					return v.evalExpr(at)
				}
			}
		}
	}

	return nil
}

func (v *Visitor) evalIdent(i *ast.Ident) error {
	if i.Obj == nil {
		return nil
	}
	if s, ok := i.Obj.Decl.(*ast.AssignStmt); ok {
		return v.evalStmt(s)
	}
	return nil
}

func (v *Visitor) addBox(name string, path string) {
	if len(name) == 0 {
		name = path
	}
	name = strings.Replace(name, "\"", "", -1)
	path = strings.Replace(path, "\"", "", -1)
	abs := path
	if _, ok := v.boxes[name]; !ok {
		box := NewBox(name, path)
		box.Package = v.Package

		pd := filepath.Dir(v.File.Name())
		pwd, _ := os.Getwd()
		if !filepath.IsAbs(pd) {
			pd = filepath.Join(pwd, pd)
		}
		box.PackageDir = pd

		if !filepath.IsAbs(abs) {
			abs = filepath.Join(pd, abs)
		}
		box.AbsPath = abs
		v.boxes[name] = box
	}
}
func (v *Visitor) fromVariable(as *ast.AssignStmt) (string, error) {
	if len(as.Rhs) == 1 {
		if bs, ok := as.Rhs[0].(*ast.BasicLit); ok {
			return bs.Value, nil
		}
	}
	return "", errors.Wrap(errors.New("unable to find value from variable"), fmt.Sprint(as))
}

func (v *Visitor) addVariable(bn string, as *ast.AssignStmt) error {
	bv, err := v.fromVariable(as)
	if err != nil {
		return nil
	}
	if len(bn) == 0 {
		bn = bv
	}
	v.addBox(bn, bv)
	return nil
}

func (v *Visitor) fromConstant(vs *ast.ValueSpec) (string, error) {
	if len(vs.Values) == 1 {
		if bs, ok := vs.Values[0].(*ast.BasicLit); ok {
			return bs.Value, nil
		}
	}
	return "", errors.Wrap(errors.New("unable to find value from constant"), fmt.Sprint(vs))
}

func (v *Visitor) addConstant(bn string, vs *ast.ValueSpec) error {
	if len(vs.Values) == 1 {
		if bs, ok := vs.Values[0].(*ast.BasicLit); ok {
			bv := bs.Value
			if len(bn) == 0 {
				bn = bv
			}
			v.addBox(bn, bv)
		}
	}
	return nil
}
