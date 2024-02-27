// Package exitcode exports rclone's exit status numbers.
package exitcode

const (
	// Success is returned when rclone finished without error.
	Success = iota
	// UsageError is returned when there was a syntax or usage error in the arguments.
	UsageError
	// UncategorizedError is returned for any error not categorised otherwise.
	UncategorizedError
	// DirNotFound is returned when a source or destination directory is not found.
	DirNotFound
	// FileNotFound is returned when a source or destination file is not found.
	FileNotFound
	// RetryError is returned for temporary errors during operations which may be retried.
	RetryError
	// NoRetryError is returned for errors from operations which can't/shouldn't be retried.
	NoRetryError
	// FatalError is returned for errors one or more retries won't resolve.
	FatalError
	// TransferExceeded is returned when network I/O exceeded the quota.
	TransferExceeded
	// NoFilesTransferred everything succeeded, but no transfer was made.
	NoFilesTransferred
	// DurationExceeded is returned when transfer duration exceeded the quota.
	DurationExceeded
)
