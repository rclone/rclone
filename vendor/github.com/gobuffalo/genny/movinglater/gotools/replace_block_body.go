package gotools

import (
	"strings"

	"github.com/gobuffalo/genny"
	"github.com/pkg/errors"
)

// ReplaceBlockBody will replace found block with expressions passed
func ReplaceBlockBody(gf genny.File, search string, expressions ...string) (genny.File, error) {
	pf, err := ParseFile(gf)
	if err != nil {
		return gf, errors.WithStack(err)
	}
	gf = pf.File

	start, end := findBlockCoordinates(search, pf)
	if end < 0 {
		return gf, errors.Errorf("could not find desired block in %s", gf.Name())
	}

	pf.Lines = append(pf.Lines[:start], append(expressions, pf.Lines[end:]...)...)

	fileContent := strings.Join(pf.Lines, "\n")
	return genny.NewFile(gf.Name(), strings.NewReader(fileContent)), nil
}
