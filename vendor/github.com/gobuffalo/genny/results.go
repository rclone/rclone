package genny

import (
	"net/http"
	"os/exec"

	"github.com/pkg/errors"
)

type Results struct {
	Files    []File
	Commands []*exec.Cmd
	Requests []RequestResult
}

func (r Results) Find(s string) (File, error) {
	for _, f := range r.Files {
		if s == f.Name() {
			return f, nil
		}
	}
	return nil, errors.Errorf("%s not found", s)
}

type RequestResult struct {
	Request  *http.Request
	Response *http.Response
	Client   *http.Client
	Error    error
}
