package metrics

import (
	"context"
	"fmt"
	"net/http"

	libhttp "github.com/rclone/rclone/lib/http"
)

const metricsPath = "/metrics"

// Server represents the metrics server
type Server struct {
	ctx    context.Context
	server *libhttp.Server
}

var (
	serverInst *Server
)

// StartStandalone starts the metrics server in standalone mode
func StartStandalone(ctx context.Context) (*Server, error) {
	var err error
	serverInst, err = newServer(ctx)
	if err != nil {
		return nil, err
	}
	serverInst.Serve()

	return serverInst, nil
}

func newServer(ctx context.Context) (*Server, error) {
	if err := Init(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialise metrics: %w", err)
	}

	s := &Server{ctx: ctx}
	server, err := libhttp.NewServer(ctx,
		libhttp.WithConfig(opt.HTTP),
		libhttp.WithAuth(opt.Auth),
		libhttp.WithTemplate(opt.Template),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to init server: %w", err)
	}

	router := server.Router()
	router.Get(metricsPath, func(w http.ResponseWriter, r *http.Request) {
		Handler().ServeHTTP(w, r)
	})

	s.server = server
	return s, nil
}

// Serve starts the server
func (s *Server) Serve() {
	s.server.Serve()
}

// Wait waits for the server to finish
func (s *Server) Wait() {
	s.server.Wait()
}

// Shutdown shuts down the server
func (s *Server) Shutdown() error {
	return s.server.Shutdown()
}

// URLs returns the server URLs
func URLs() []string {
	return serverInst.server.URLs()
}
