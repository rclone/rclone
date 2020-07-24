package restic

import (
	"github.com/rclone/rclone/cmd"
	"net/http"
	"net/http/httptest"
)

func NewServer(args []string) (*server, error) {
	f := cmd.NewFsSrc(args)
	return &server{f: f}, nil
}

func (s *server) RoundTrip(r *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	s.handler(w, r)
	return w.Result(), nil
}
