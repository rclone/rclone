package restic

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	promRegistry                         = prometheus.NewRegistry()
	promRegisterer prometheus.Registerer = promRegistry
	promGatherer   prometheus.Gatherer   = promRegistry
)

var metricLabelList = []string{"user", "repo", "type"}

var (
	metricBlobWriteTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "restic_serve_blob_write_total",
		Help: "Total number of blob written",
	}, metricLabelList)

	metricBlobWriteBytesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "restic_serve_blob_write_bytes_total",
		Help: "Total number of bytes written to blob",
	}, metricLabelList)

	metricBlobReadTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "restic_serve_blob_read_total",
		Help: "Total number of blob read",
	}, metricLabelList)

	metricBlobReadBytesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "restic_serve_blob_read_bytes_total",
		Help: "Total number of bytes read from blob",
	}, metricLabelList)

	metricBlobDeleteTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "restic_serve_blob_delete_total",
		Help: "Total number of blob deleted",
	}, metricLabelList)

	metricBlobDeleteBytesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "restic_serve_blob_delete_bytes_total",
		Help: "Total number of bytes of blob deleted",
	}, metricLabelList)
)

func init() {
	promRegisterer.MustRegister(metricBlobWriteTotal)
	promRegisterer.MustRegister(metricBlobWriteBytesTotal)
	promRegisterer.MustRegister(metricBlobReadTotal)
	promRegisterer.MustRegister(metricBlobReadBytesTotal)
	promRegisterer.MustRegister(metricBlobDeleteTotal)
	promRegisterer.MustRegister(metricBlobDeleteBytesTotal)
}

func promHandler() http.Handler {
	return promhttp.InstrumentMetricHandler(
		promRegisterer, promhttp.HandlerFor(promGatherer, promhttp.HandlerOpts{
			Registry: promRegisterer,
		}),
	)
}

var blobRe = regexp.MustCompile(`((.+))?(data|index|keys|locks|snapshots)/(.+)`)

func getMetricLabels(r *http.Request, remote string) prometheus.Labels {
	remote = strings.Trim(remote, "/")
	matches := blobRe.FindStringSubmatch(remote)
	if matches == nil {
		return nil
	}
	repo := "."
	if matches[1] != "" {
		repo = matches[1]
	}
	return prometheus.Labels{
		"user": getUser(r),
		"repo": repo,
		"type": matches[3],
	}
}

func getUser(r *http.Request) string {
	u, _, ok := r.BasicAuth()
	if ok {
		return u
	}
	return ""
}
