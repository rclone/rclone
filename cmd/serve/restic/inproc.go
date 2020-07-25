package restic

import (
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/serve/httplib/httpflags"
	"net/http"
	"net/http/httptest"
)

func NewServer(args []string) (*server, error) {
	f := cmd.NewFsSrc(args)
	return newServer(f, &httpflags.Opt), nil
}

func (s *server) RoundTrip(r *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	s.handler(w, r)
	return w.Result(), nil
}
