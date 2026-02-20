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

	// healthCheckTimeout is the timeout for health check operations.
	// MinIO/S3 can be slow to respond on List("") under load; use a generous timeout
	// so we don't mark backends unavailable due to slow response.
	healthCheckTimeout = 30 * time.Second

	// defaultShutdownTimeout is the default timeout for Shutdown operations
	defaultShutdownTimeout = 60 * time.Second

	// listHelperTimeout is used for parity.List in List() helpers (cleanupOrphanedDirectory,
	// reconstructMissingDirectory) to avoid blocking forever if a backend hangs.
	listHelperTimeout = 30 * time.Second

	// listBackendTimeout is used for List() and ListR() backend calls (even, odd, parity).
	// Prevents indefinite hang when a backend (e.g. MinIO/S3) blocks on List/ListR.
	// Sync with MinIO can hang without this; local backends return quickly.
	listBackendTimeout = 2 * time.Minute

	// putOperationTimeout limits the total Put operation time.
	// Prevents indefinite hang when a backend (e.g. MinIO/S3) blocks on CreateMultipartUpload.
	putOperationTimeout = 5 * time.Minute

	// putStaggerDelay is the delay between starting each of the 3 Put goroutines.
	// Staggering reduces concurrent CreateMultipartUpload load on MinIO, avoiding intermittent hangs.
	// A short delay (300ms) spreads requests across backends without adding much latency.
	putStaggerDelay = 300 * time.Millisecond

	// sequentialOpenDelay is the delay between opening each of the 3 underlying backends in NewFs.
	// Opening all three in parallel can trigger "unexpected packet" on some SFTP servers (e.g. atmoz/sftp)
	// when multiple connections are established from the same process. Sequential open with delay avoids that.
	// Set to 0 to restore parallel open (e.g. for revert).
	sequentialOpenDelay = 200 * time.Millisecond

	// initOpenRetries is the number of attempts when opening each underlying backend in NewFs.
	// Some SFTP servers (e.g. atmoz/sftp) can fail the first connection from a process with
	// "unexpected packet"; retrying once or twice often succeeds.
	initOpenRetries = 3

	// initOpenRetryDelay is the delay between open attempts for each backend.
	initOpenRetryDelay = 1 * time.Second
)

// Block size for block-based compression (enables range reads via inventory)
const (
	// BlockSize is the uncompressed block size (128 KiB) for block-based compression.
	// Used for both snappy and zstd.
	BlockSize = 131072
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
