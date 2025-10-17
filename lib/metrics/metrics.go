package metrics

import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/fshttp"
)

var (
	initErr     error
	promHandler http.Handler
)

func Init(ctx context.Context) error {
	if err := registerCollector(accounting.NewRcloneCollector(ctx)); err != nil {
		return err
	}
	httpMetrics := fshttp.NewMetrics("rclone")
	fshttp.DefaultMetrics = httpMetrics
	for _, collector := range httpMetrics.Collectors() {
		_ = registerCollector(collector)
	}

	promHandler = promhttp.Handler()
	return initErr
}

func registerCollector(collector prometheus.Collector) error {
	if collector == nil {
		return nil
	}
	if err := prometheus.Register(collector); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			return err
		}
	}
	return nil
}

func RegisterCollector(collector prometheus.Collector) error {
	return registerCollector(collector)
}

func MustRegisterCollector(collectors ...prometheus.Collector) {
	for _, collector := range collectors {
		if err := registerCollector(collector); err != nil {
			panic(err)
		}
	}
}

func Handler() http.Handler {
	if promHandler == nil {
		return promhttp.Handler()
	}
	return promHandler
}
