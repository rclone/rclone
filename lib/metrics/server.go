package metrics

import (
	"context"
	"fmt"
	"net/http"

	libhttp "github.com/rclone/rclone/lib/http"
)

const Path = "/metrics"

type Server struct {
	ctx    context.Context
	server *libhttp.Server
}

var (
	serverInst *Server
)

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
		libhttp.WithConfig(Opt.HTTP),
		libhttp.WithAuth(Opt.Auth),
		libhttp.WithTemplate(Opt.Template),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to init server: %w", err)
	}

	router := server.Router()
	router.Get(Path, func(w http.ResponseWriter, r *http.Request) {
		Handler().ServeHTTP(w, r)
	})

	s.server = server
	return s, nil
}

func (s *Server) Serve() {
	s.server.Serve()
}

func (s *Server) Wait() {
	s.server.Wait()
}

func (s *Server) Shutdown() error {
	return s.server.Shutdown()
}

func URLs() []string {
	return serverInst.server.URLs()
}
