package fshttp

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics provide Transport HTTP level metrics.
type Metrics struct {
	StatusCode *prometheus.CounterVec
}

// NewMetrics creates a new metrics instance, the instance shall be assigned to
// DefaultMetrics before any processing takes place.
func NewMetrics(namespace string) *Metrics {
	return &Metrics{
		StatusCode: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "http",
			Name:      "status_code",
		}, []string{"host", "method", "code"}),
	}
}

// DefaultMetrics specifies metrics used for new Transports.
var DefaultMetrics = (*Metrics)(nil)

// Collectors returns all prometheus metrics as collectors for registration.
func (m *Metrics) Collectors() []prometheus.Collector {
	if m == nil {
		return nil
	}
	return []prometheus.Collector{
		m.StatusCode,
	}
}

func (m *Metrics) onResponse(req *http.Request, resp *http.Response) {
	if m == nil {
		return
	}

	var statusCode = 0
	if resp != nil {
		statusCode = resp.StatusCode
	}

	m.StatusCode.WithLabelValues(req.Host, req.Method, fmt.Sprint(statusCode)).Inc()
}
