package accounting

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
)

var namespace = "rclone_"

// RcloneCollector is a Prometheus collector for Rclone
type RcloneCollector struct {
	ctx              context.Context
	bytesTransferred *prometheus.Desc
	transferSpeed    *prometheus.Desc
	numOfErrors      *prometheus.Desc
	numOfCheckFiles  *prometheus.Desc
	transferredFiles *prometheus.Desc
	deletes          *prometheus.Desc
	deletedDirs      *prometheus.Desc
	renames          *prometheus.Desc
	fatalError       *prometheus.Desc
	retryError       *prometheus.Desc
}

// NewRcloneCollector make a new RcloneCollector
func NewRcloneCollector(ctx context.Context) *RcloneCollector {
	return &RcloneCollector{
		ctx: ctx,
		bytesTransferred: prometheus.NewDesc(namespace+"bytes_transferred_total",
			"Total transferred bytes since the start of the Rclone process",
			nil, nil,
		),
		transferSpeed: prometheus.NewDesc(namespace+"speed",
			"Average speed in bytes per second since the start of the Rclone process",
			nil, nil,
		),
		numOfErrors: prometheus.NewDesc(namespace+"errors_total",
			"Number of errors thrown",
			nil, nil,
		),
		numOfCheckFiles: prometheus.NewDesc(namespace+"checked_files_total",
			"Number of checked files",
			nil, nil,
		),
		transferredFiles: prometheus.NewDesc(namespace+"files_transferred_total",
			"Number of transferred files",
			nil, nil,
		),
		deletes: prometheus.NewDesc(namespace+"files_deleted_total",
			"Total number of files deleted",
			nil, nil,
		),
		deletedDirs: prometheus.NewDesc(namespace+"dirs_deleted_total",
			"Total number of directories deleted",
			nil, nil,
		),
		renames: prometheus.NewDesc(namespace+"files_renamed_total",
			"Total number of files renamed",
			nil, nil,
		),
		fatalError: prometheus.NewDesc(namespace+"fatal_error",
			"Whether a fatal error has occurred",
			nil, nil,
		),
		retryError: prometheus.NewDesc(namespace+"retry_error",
			"Whether there has been an error that will be retried",
			nil, nil,
		),
	}
}

// Describe is part of the Collector interface: https://godoc.org/github.com/prometheus/client_golang/prometheus#Collector
func (c *RcloneCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.bytesTransferred
	ch <- c.transferSpeed
	ch <- c.numOfErrors
	ch <- c.numOfCheckFiles
	ch <- c.transferredFiles
	ch <- c.deletes
	ch <- c.deletedDirs
	ch <- c.renames
	ch <- c.fatalError
	ch <- c.retryError
}

// Collect is part of the Collector interface: https://godoc.org/github.com/prometheus/client_golang/prometheus#Collector
func (c *RcloneCollector) Collect(ch chan<- prometheus.Metric) {
	s := groups.sum(c.ctx)
	s.mu.RLock()

	ch <- prometheus.MustNewConstMetric(c.bytesTransferred, prometheus.CounterValue, float64(s.bytes))
	ch <- prometheus.MustNewConstMetric(c.transferSpeed, prometheus.GaugeValue, s.speed())
	ch <- prometheus.MustNewConstMetric(c.numOfErrors, prometheus.CounterValue, float64(s.errors))
	ch <- prometheus.MustNewConstMetric(c.numOfCheckFiles, prometheus.CounterValue, float64(s.checks))
	ch <- prometheus.MustNewConstMetric(c.transferredFiles, prometheus.CounterValue, float64(s.transfers))
	ch <- prometheus.MustNewConstMetric(c.deletes, prometheus.CounterValue, float64(s.deletes))
	ch <- prometheus.MustNewConstMetric(c.deletedDirs, prometheus.CounterValue, float64(s.deletedDirs))
	ch <- prometheus.MustNewConstMetric(c.renames, prometheus.CounterValue, float64(s.renames))
	ch <- prometheus.MustNewConstMetric(c.fatalError, prometheus.GaugeValue, bool2Float(s.fatalError))
	ch <- prometheus.MustNewConstMetric(c.retryError, prometheus.GaugeValue, bool2Float(s.retryError))

	s.mu.RUnlock()
}

// bool2Float is a small function to convert a boolean into a float64 value that can be used for Prometheus
func bool2Float(e bool) float64 {
	if e {
		return 1
	}
	return 0
}
