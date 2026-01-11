// Package raid3 implements a backend that splits data across three remotes using byte-level striping
package raid3

import (
	"time"
)

// Timeout configurations for backend initialization
const (
	// aggressiveInitTimeout is used when timeout_mode is "aggressive"
	aggressiveInitTimeout = 10 * time.Second

	// balancedInitTimeout is used when timeout_mode is "balanced"
	balancedInitTimeout = 60 * time.Second

	// standardInitTimeout is used when timeout_mode is "standard" or empty
	standardInitTimeout = 5 * time.Minute

	// errorIsFileRetryTimeout is used when retrying with adjusted root after ErrorIsFile
	errorIsFileRetryTimeout = 10 * time.Second

	// healthCheckTimeout is the timeout for health check operations
	healthCheckTimeout = 5 * time.Second

	// defaultShutdownTimeout is the default timeout for Shutdown operations
	defaultShutdownTimeout = 60 * time.Second
)

// Chunk size constants
const (
	// defaultChunkSize is the default chunk size for streaming operations (8 MiB)
	defaultChunkSize = 8 * 1024 * 1024

	// minChunkSize is the minimum allowed chunk size (1 KiB)
	minChunkSize = 1024

	// minReadChunkSize is the minimum read chunk size for streaming (2 MiB)
	// This is used when reading input for streaming operations
	minReadChunkSize = 2 * 1024 * 1024
)

// Upload queue and worker constants
const (
	// defaultUploadQueueSize is the buffer size for the upload queue
	defaultUploadQueueSize = 100

	// defaultUploadWorkers is the number of concurrent upload workers for heal operations
	defaultUploadWorkers = 2

	// rebuildWorkers is the number of concurrent workers for rebuild operations
	rebuildWorkers = 4
)
