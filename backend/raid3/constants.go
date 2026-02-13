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

	// streamReadChunkSize is the read chunk size for the stream splitter (64 KiB).
	// Small chunks give natural backpressure and avoid truncation with AsyncReader.
	// Used by both Put (operations.go) and Update (object.go) streaming paths.
	streamReadChunkSize = 64 * 1024

	// streamProducerBufferSize is the buffer size (8 MiB) between the source reader
	// and the stream splitter. A producer goroutine reads from the source into a
	// pipe; the splitter reads from the pipe via this buffered reader. This decouples
	// the source (e.g. Account/AsyncReader from rclone copy) from the splitter's write
	// rate to the three particle pipes, so the full stream is read even when backends
	// are slow. Matches the compress backend's approach (see backend/compress/compress.go bufferSize).
	streamProducerBufferSize = 8 * 1024 * 1024
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
