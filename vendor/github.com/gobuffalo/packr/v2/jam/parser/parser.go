package parser

import (
	"os"
	"sort"
	"strings"

	"github.com/gobuffalo/packr/v2/plog"
	"github.com/pkg/errors"
)

// Parser to find boxes
type Parser struct {
	Prospects     []*File // a list of files to check for boxes
	IgnoreImports bool
}

// Run the parser and run any boxes found
func (p *Parser) Run() (Boxes, error) {
	var boxes Boxes
	for _, pros := range p.Prospects {
		plog.Debug(p, "Run", "parsing", pros.Name())
		v := NewVisitor(pros)
		pbr, err := v.Run()
		if err != nil {
			return boxes, errors.WithStack(err)
		}
		for _, b := range pbr {
			plog.Debug(p, "Run", "file", pros.Name(), "box", b.Name)
			boxes = append(boxes, b)
		}
	}

	pwd, _ := os.Getwd()
	sort.Slice(boxes, func(a, b int) bool {
		b1 := boxes[a]
		return !strings.HasPrefix(b1.AbsPath, pwd)
	})
	return boxes, nil
}

// New Parser from a list of File
func New(prospects ...*File) *Parser {
	return &Parser{
		Prospects: prospects,
	}
}
