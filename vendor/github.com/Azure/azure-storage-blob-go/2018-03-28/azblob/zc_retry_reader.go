package azblob

import (
	"context"
	"io"
	"net"
	"net/http"
)

const CountToEnd = 0

// HTTPGetter is a function type that refers to a method that performs an HTTP GET operation.
type HTTPGetter func(ctx context.Context, i HTTPGetterInfo) (*http.Response, error)

// HTTPGetterInfo is passed to an HTTPGetter function passing it parameters
// that should be used to make an HTTP GET request.
type HTTPGetterInfo struct {
	// Offset specifies the start offset that should be used when
	// creating the HTTP GET request's Range header
	Offset int64

	// Count specifies the count of bytes that should be used to calculate
	// the end offset when creating the HTTP GET request's Range header
	Count int64

	// ETag specifies the resource's etag that should be used when creating
	// the HTTP GET request's If-Match header
	ETag ETag
}

// RetryReaderOptions contains properties which can help to decide when to do retry.
type RetryReaderOptions struct {
	// MaxRetryRequests specifies the maximum number of HTTP GET requests that will be made
	// while reading from a RetryReader. A value of zero means that no additional HTTP
	// GET requests will be made.
	MaxRetryRequests   int
	doInjectError      bool
	doInjectErrorRound int
}

// retryReader implements io.ReaderCloser methods.
// retryReader tries to read from response, and if there is retriable network error
// returned during reading, it will retry according to retry reader option through executing
// user defined action with provided data to get a new response, and continue the overall reading process
// through reading from the new response.
type retryReader struct {
	ctx             context.Context
	response        *http.Response
	info            HTTPGetterInfo
	countWasBounded bool
	o               RetryReaderOptions
	getter          HTTPGetter
}

// NewRetryReader creates a retry reader.
func NewRetryReader(ctx context.Context, initialResponse *http.Response,
	info HTTPGetterInfo, o RetryReaderOptions, getter HTTPGetter) io.ReadCloser {
	if getter == nil {
		panic("getter must not be nil")
	}
	if info.Count < 0 {
		panic("info.Count must be >= 0")
	}
	if o.MaxRetryRequests < 0 {
		panic("o.MaxRetryRequests must be >= 0")
	}
	return &retryReader{ctx: ctx, getter: getter, info: info, countWasBounded: info.Count != CountToEnd, response: initialResponse, o: o}
}

func (s *retryReader) Read(p []byte) (n int, err error) {
	for try := 0; ; try++ {
		//fmt.Println(try)       // Comment out for debugging.
		if s.countWasBounded && s.info.Count == CountToEnd {
			// User specified an original count and the remaining bytes are 0, return 0, EOF
			return 0, io.EOF
		}

		if s.response == nil { // We don't have a response stream to read from, try to get one.
			response, err := s.getter(s.ctx, s.info)
			if err != nil {
				return 0, err
			}
			// Successful GET; this is the network stream we'll read from.
			s.response = response
		}
		n, err := s.response.Body.Read(p) // Read from the stream

		// Injection mechanism for testing.
		if s.o.doInjectError && try == s.o.doInjectErrorRound {
			err = &net.DNSError{IsTemporary: true}
		}

		// We successfully read data or end EOF.
		if err == nil || err == io.EOF {
			s.info.Offset += int64(n) // Increments the start offset in case we need to make a new HTTP request in the future
			if s.info.Count != CountToEnd {
				s.info.Count -= int64(n) // Decrement the count in case we need to make a new HTTP request in the future
			}
			return n, err // Return the return to the caller
		}
		s.Close()        // Error, close stream
		s.response = nil // Our stream is no longer good

		// Check the retry count and error code, and decide whether to retry.
		if try >= s.o.MaxRetryRequests {
			return n, err // All retries exhausted
		}

		if netErr, ok := err.(net.Error); ok && (netErr.Timeout() || netErr.Temporary()) {
			continue
			// Loop around and try to get and read from new stream.
		}
		return n, err // Not retryable, just return
	}
}

func (s *retryReader) Close() error {
	if s.response != nil && s.response.Body != nil {
		return s.response.Body.Close()
	}
	return nil
}
