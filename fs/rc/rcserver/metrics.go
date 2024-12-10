// Package rcserver implements the HTTP endpoint to serve the remote control
package rcserver

import (
	"context"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fs/rc/jobs"
	libhttp "github.com/rclone/rclone/lib/http"
)

const path = "/metrics"

var promHandlerFunc http.HandlerFunc

func init() {
	rcloneCollector := accounting.NewRcloneCollector(context.Background())
	prometheus.MustRegister(rcloneCollector)

	m := fshttp.NewMetrics("rclone")
	for _, c := range m.Collectors() {
		prometheus.MustRegister(c)
	}
	fshttp.DefaultMetrics = m

	promHandlerFunc = promhttp.Handler().ServeHTTP
}

// MetricsStart the remote control server if configured
//
// If the server wasn't configured the *Server returned may be nil
func MetricsStart(ctx context.Context, opt *rc.Options) (*MetricsServer, error) {
	jobs.SetOpt(opt) // set the defaults for jobs
	if len(opt.MetricsHTTP.ListenAddr) > 0 {
		// Serve on the DefaultServeMux so can have global registrations appear
		s, err := newMetricsServer(ctx, opt)
		if err != nil {
			return nil, err
		}
		return s, s.Serve()
	}
	return nil, nil
}

// MetricsServer contains everything to run the rc server
type MetricsServer struct {
	ctx             context.Context // for global config
	server          *libhttp.Server
	promHandlerFunc http.Handler
	opt             *rc.Options
}

func newMetricsServer(ctx context.Context, opt *rc.Options) (*MetricsServer, error) {
	s := &MetricsServer{
		ctx:             ctx,
		opt:             opt,
		promHandlerFunc: promHandlerFunc,
	}

	var err error
	s.server, err = libhttp.NewServer(ctx,
		libhttp.WithConfig(opt.MetricsHTTP),
		libhttp.WithAuth(opt.MetricsAuth),
		libhttp.WithTemplate(opt.MetricsTemplate),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to init server: %w", err)
	}

	router := s.server.Router()
	router.Get(path, promHandlerFunc)
	return s, nil
}

// Serve runs the http server in the background.
//
// Use s.Close() and s.Wait() to shutdown server
func (s *MetricsServer) Serve() error {
	s.server.Serve()
	return nil
}

// Wait blocks while the server is serving requests
func (s *MetricsServer) Wait() {
	s.server.Wait()
}

// Shutdown gracefully shuts down the server
func (s *MetricsServer) Shutdown() error {
	return s.server.Shutdown()
}
