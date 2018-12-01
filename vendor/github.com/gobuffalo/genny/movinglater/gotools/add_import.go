package gotools

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"github.com/gobuffalo/genny"
	"github.com/pkg/errors"
)

// AddImport adds n number of import statements into the path provided
func AddImport(gf genny.File, imports ...string) (genny.File, error) {
	pf, err := ParseFile(gf)
	if err != nil {
		return gf, errors.WithStack(err)
	}
	gf = pf.File

	end := findLastImport(pf.Ast, pf.FileSet, pf.Lines)

	x := make([]string, len(imports), len(imports)+2)
	for _, i := range imports {
		x = append(x, fmt.Sprintf("\t\"%s\"", i))

	}
	if end < 0 {
		x = append([]string{"import ("}, x...)
		x = append(x, ")")
	}

	pf.Lines = append(pf.Lines[:end], append(x, pf.Lines[end:]...)...)

	fileContent := strings.Join(pf.Lines, "\n")
	return genny.NewFile(gf.Name(), strings.NewReader(fileContent)), nil
}

func findLastImport(f *ast.File, fset *token.FileSet, fileLines []string) int {
	var end = -1

	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.ImportSpec:
			end = fset.Position(x.End()).Line
			return true
		}
		return true
	})

	return end
}
