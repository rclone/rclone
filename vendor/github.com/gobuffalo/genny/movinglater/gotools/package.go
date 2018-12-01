package gotools

import (
	"path/filepath"

	"github.com/gobuffalo/genny"
	"github.com/pkg/errors"
)

func PackageName(f genny.File) (string, error) {
	pkg := filepath.Base(filepath.Dir(f.Name()))
	pf, err := ParseFile(f)
	if err == nil {
		pkg = pf.Ast.Name.String()
	}
	if len(pkg) == 0 || pkg == "." {
		return "", errors.New("could not determine package")
	}
	return pkg, nil
}
