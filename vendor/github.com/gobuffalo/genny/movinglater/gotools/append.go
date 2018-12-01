package gotools

import (
	"strings"

	"github.com/gobuffalo/genny"
	"github.com/pkg/errors"
)

//Append allows to append source into a go file
func Append(gf genny.File, expressions ...string) (genny.File, error) {
	pf, err := ParseFile(gf)
	if err != nil {
		return gf, errors.WithStack(err)
	}

	gf = pf.File
	pf.Lines = append(pf.Lines, expressions...)

	fileContent := strings.Join(pf.Lines, "\n")
	return genny.NewFile(gf.Name(), strings.NewReader(fileContent)), nil
}
