package restic

import (
	"fmt"
	"github.com/ncw/rclone/cmd"
	"net/http"
	"net/http/httptest"
)

func NewServer(args []string) (*server, error) {
	f := cmd.NewFsSrc(args)
	return &server{f: f}, nil
}

func (s *server) RoundTrip(r *http.Request) (*http.Response, error) {
	fmt.Printf("Request: %s %s\n", r.Method, r.URL)
	w := httptest.NewRecorder()
	s.handler(w, r)
	return w.Result(), nil
}
