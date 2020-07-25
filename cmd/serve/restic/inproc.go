package restic

import (
	"net/http"
	"net/http/httptest"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/serve/httplib/httpflags"
)

// NewServer returns a rclone server object which talks the restic http protocol
func NewServer(args []string) (http.RoundTripper, error) {
	f := cmd.NewFsSrc(args)
	return newServer(f, &httpflags.Opt), nil
}

func (s *server) RoundTrip(r *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	s.handler(w, r)
	return w.Result(), nil
}
