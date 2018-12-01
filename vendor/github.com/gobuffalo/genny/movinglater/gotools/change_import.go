package gotools

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"

	"github.com/gobuffalo/genny"
	"github.com/pkg/errors"
	"golang.org/x/tools/go/ast/astutil"
)

func RewriteImports(gf genny.File, swaps map[string]string) (genny.File, error) {
	pf, err := ParseFileMode(gf, parser.ParseComments)
	if err != nil {
		return gf, errors.WithStack(err)
	}
	for key, value := range swaps {
		if !astutil.DeleteImport(pf.FileSet, pf.Ast, key) {
			continue
		}

		astutil.AddImport(pf.FileSet, pf.Ast, value)
	}
	ast.SortImports(pf.FileSet, pf.Ast)

	w := &bytes.Buffer{}
	if err = (&printer.Config{Mode: printer.TabIndent | printer.UseSpaces, Tabwidth: 8}).Fprint(w, pf.FileSet, pf.Ast); err != nil {
		return gf, errors.WithStack(err)
	}

	return genny.NewFile(gf.Name(), w), nil
}
