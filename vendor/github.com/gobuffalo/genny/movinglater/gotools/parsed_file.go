package gotools

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gobuffalo/genny"
	"github.com/pkg/errors"
)

type ParsedFile struct {
	File    genny.File
	FileSet *token.FileSet
	Ast     *ast.File
	Lines   []string
}

func ParseFileMode(gf genny.File, mode parser.Mode) (ParsedFile, error) {
	name := gf.Name()
	pf := ParsedFile{
		FileSet: token.NewFileSet(),
	}

	gf, err := beforeParse(gf)
	if err != nil {
		return pf, errors.WithStack(err)
	}

	src := gf.String()
	f, err := parser.ParseFile(pf.FileSet, gf.Name(), src, mode)
	if err != nil && errors.Cause(err) != io.EOF {
		return pf, errors.WithStack(err)
	}
	pf.Ast = f

	pf.Lines = strings.Split(src, "\n")
	pf.File = genny.NewFile(name, gf)
	return pf, nil
}

func ParseFile(gf genny.File) (ParsedFile, error) {
	return ParseFileMode(gf, 0)
}

func beforeParse(gf genny.File) (genny.File, error) {
	src, err := ioutil.ReadAll(gf)
	if err != nil {
		return gf, errors.WithStack(err)
	}

	dir := os.TempDir()
	path := filepath.Join(dir, fmt.Sprintf("%d.go", time.Now().UnixNano()))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return gf, errors.WithStack(err)
	}

	tf, err := os.Create(path)
	if err != nil {
		return gf, errors.WithStack(err)
	}
	if _, err := tf.Write(src); err != nil {
		return gf, errors.WithStack(err)
	}
	if err := tf.Close(); err != nil {
		return gf, errors.WithStack(err)
	}
	return genny.NewFile(path, bytes.NewReader(src)), nil
}
