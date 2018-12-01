package genny

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/gobuffalo/logger"
	"github.com/pkg/errors"
)

// WetRunner will execute commands and write files
// it is DESTRUCTIVE
func WetRunner(ctx context.Context) *Runner {
	r := DryRunner(ctx)
	l := logger.New(DefaultLogLvl)
	r.Logger = l

	r.ExecFn = wetExecFn
	r.FileFn = func(f File) (File, error) {
		return wetFileFn(r, f)
	}
	r.DeleteFn = os.RemoveAll
	r.RequestFn = wetRequestFn
	r.ChdirFn = func(path string, fn func() error) error {
		pwd, _ := os.Getwd()
		defer os.Chdir(pwd)
		os.MkdirAll(path, 0755)
		if err := os.Chdir(path); err != nil {
			return errors.WithStack(err)
		}
		return fn()
	}
	r.LookPathFn = exec.LookPath
	return r
}

func wetRequestFn(req *http.Request, c *http.Client) (*http.Response, error) {
	if c == nil {
		c = &http.Client{}
	}
	ctx, cancel := context.WithTimeout(req.Context(), 30*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	res, err := c.Do(req)
	if err != nil {
		return res, errors.WithStack(err)
	}

	if res.StatusCode >= 400 {
		return res, errors.WithStack(errors.Errorf("response returned non-success code: %d", res.StatusCode))
	}
	return res, nil
}

func wetExecFn(cmd *exec.Cmd) error {
	if cmd.Stdin == nil {
		cmd.Stdin = os.Stdin
	}
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}
	if cmd.Stderr == nil {
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}

func wetFileFn(r *Runner, f File) (File, error) {
	if d, ok := f.(Dir); ok {
		if err := os.MkdirAll(d.Name(), d.Perm); err != nil {
			return f, errors.WithStack(err)
		}
		return d, nil
	}

	name := f.Name()
	if !filepath.IsAbs(name) {
		name = filepath.Join(r.Root, name)
	}
	dir := filepath.Dir(name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return f, errors.WithStack(err)
	}
	ff, err := os.Create(name)
	if err != nil {
		return f, errors.WithStack(err)
	}
	defer ff.Close()
	bb := &bytes.Buffer{}
	mw := io.MultiWriter(bb, ff)
	if _, err := io.Copy(mw, f); err != nil {
		return f, errors.WithStack(err)
	}
	return NewFile(f.Name(), bb), nil
}
