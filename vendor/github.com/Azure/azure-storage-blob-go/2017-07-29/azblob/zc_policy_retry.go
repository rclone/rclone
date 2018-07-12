package azblob

import (
	"context"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/Azure/azure-pipeline-go/pipeline"
	"io/ioutil"
	"io"
)

// RetryPolicy tells the pipeline what kind of retry policy to use. See the RetryPolicy* constants.
type RetryPolicy int32

const (
	// RetryPolicyExponential tells the pipeline to use an exponential back-off retry policy
	RetryPolicyExponential RetryPolicy = 0

	// RetryPolicyFixed tells the pipeline to use a fixed back-off retry policy
	RetryPolicyFixed RetryPolicy = 1
)

// RetryOptions configures the retry policy's behavior.
type RetryOptions struct {
	// Policy tells the pipeline what kind of retry policy to use. See the RetryPolicy* constants.\
	// A value of zero means that you accept our default policy.
	Policy RetryPolicy

	// MaxTries specifies the maximum number of attempts an operation will be tried before producing an error (0=default).
	// A value of zero means that you accept our default policy. A value of 1 means 1 try and no retries.
	MaxTries int32

	// TryTimeout indicates the maximum time allowed for any single try of an HTTP request.
	// A value of zero means that you accept our default timeout. NOTE: When transferring large amounts
	// of data, the default TryTimeout will probably not be sufficient. You should override this value
	// based on the bandwidth available to the host machine and proximity to the Storage service. A good
	// starting point may be something like (60 seconds per MB of anticipated-payload-size).
	TryTimeout time.Duration

	// RetryDelay specifies the amount of delay to use before retrying an operation (0=default).
	// When RetryPolicy is specified as RetryPolicyExponential, the delay increases exponentially
	// with each retry up to a maximum specified by MaxRetryDelay.
	// If you specify 0, then you must also specify 0 for MaxRetryDelay.
	// If you specify RetryDelay, then you must also specify MaxRetryDelay, and MaxRetryDelay should be
	// equal to or greater than RetryDelay.
	RetryDelay time.Duration

	// MaxRetryDelay specifies the maximum delay allowed before retrying an operation (0=default).
	// If you specify 0, then you must also specify 0 for RetryDelay.
	MaxRetryDelay time.Duration

	// RetryReadsFromSecondaryHost specifies whether the retry policy should retry a read operation against another host.
	// If RetryReadsFromSecondaryHost is "" (the default) then operations are not retried against another host.
	// NOTE: Before setting this field, make sure you understand the issues around reading stale & potentially-inconsistent
	// data at this webpage: https://docs.microsoft.com/en-us/azure/storage/common/storage-designing-ha-apps-with-ragrs
	RetryReadsFromSecondaryHost string	// Comment this our for non-Blob SDKs
}

func (o RetryOptions) retryReadsFromSecondaryHost() string {
	return o.RetryReadsFromSecondaryHost	// This is for the Blob SDK only
	//return "" // This is for non-blob SDKs
}

func (o RetryOptions) defaults() RetryOptions {
	if o.Policy != RetryPolicyExponential && o.Policy != RetryPolicyFixed {
		panic("RetryPolicy must be RetryPolicyExponential or RetryPolicyFixed")
	}
	if o.MaxTries < 0 {
		panic("MaxTries must be >= 0")
	}
	if o.TryTimeout < 0 || o.RetryDelay < 0 || o.MaxRetryDelay < 0 {
		panic("TryTimeout, RetryDelay, and MaxRetryDelay must all be >= 0")
	}
	if o.RetryDelay > o.MaxRetryDelay {
		panic("RetryDelay must be <= MaxRetryDelay")
	}
	if (o.RetryDelay == 0 && o.MaxRetryDelay != 0) || (o.RetryDelay != 0 && o.MaxRetryDelay == 0) {
		panic("Both RetryDelay and MaxRetryDelay must be 0 or neither can be 0")
	}

	IfDefault := func(current *time.Duration, desired time.Duration) {
		if *current == time.Duration(0) {
			*current = desired
		}
	}

	// Set defaults if unspecified
	if o.MaxTries == 0 {
		o.MaxTries = 4
	}
	switch o.Policy {
	case RetryPolicyExponential:
		IfDefault(&o.TryTimeout, 1*time.Minute)
		IfDefault(&o.RetryDelay, 4*time.Second)
		IfDefault(&o.MaxRetryDelay, 120*time.Second)

	case RetryPolicyFixed:
		IfDefault(&o.TryTimeout, 1*time.Minute)
		IfDefault(&o.RetryDelay, 30*time.Second)
		IfDefault(&o.MaxRetryDelay, 120*time.Second)
	}
	return o
}

func (o RetryOptions) calcDelay(try int32) time.Duration { // try is >=1; never 0
	pow := func(number int64, exponent int32) int64 { // pow is nested helper function
		var result int64 = 1
		for n := int32(0); n < exponent; n++ {
			result *= number
		}
		return result
	}

	delay := time.Duration(0)
	switch o.Policy {
	case RetryPolicyExponential:
		delay = time.Duration(pow(2, try-1)-1) * o.RetryDelay

	case RetryPolicyFixed:
		if try > 1 { // Any try after the 1st uses the fixed delay
			delay = o.RetryDelay
		}
	}

	// Introduce some jitter:  [0.0, 1.0) / 2 = [0.0, 0.5) + 0.8 = [0.8, 1.3)
	delay = time.Duration(delay.Seconds() * (rand.Float64()/2 + 0.8) * float64(time.Second)) // NOTE: We want math/rand; not crypto/rand
	if delay > o.MaxRetryDelay {
		delay = o.MaxRetryDelay
	}
	return delay
}

// NewRetryPolicyFactory creates a RetryPolicyFactory object configured using the specified options.
func NewRetryPolicyFactory(o RetryOptions) pipeline.Factory {
	o = o.defaults() // Force defaults to be calculated
	return pipeline.FactoryFunc(func(next pipeline.Policy, po *pipeline.PolicyOptions) pipeline.PolicyFunc {
		return func(ctx context.Context, request pipeline.Request) (response pipeline.Response, err error) {
			// Before each try, we'll select either the primary or secondary URL.
			primaryTry := int32(0) // This indicates how many tries we've attempted against the primary DC

			// We only consider retrying against a secondary if we have a read request (GET/HEAD) AND this policy has a Secondary URL it can use
			considerSecondary := (request.Method == http.MethodGet || request.Method == http.MethodHead) && o.retryReadsFromSecondaryHost() != ""

			// Exponential retry algorithm: ((2 ^ attempt) - 1) * delay * random(0.8, 1.2)
			// When to retry: connection failure or temporary/timeout. NOTE: StorageError considers HTTP 500/503 as temporary & is therefore retryable
			// If using a secondary:
			//    Even tries go against primary; odd tries go against the secondary
			//    For a primary wait ((2 ^ primaryTries - 1) * delay * random(0.8, 1.2)
			//    If secondary gets a 404, don't fail, retry but future retries are only against the primary
			//    When retrying against a secondary, ignore the retry count and wait (.1 second * random(0.8, 1.2))
			for try := int32(1); try <= o.MaxTries; try++ {
				logf("\n=====> Try=%d\n", try)

				// Determine which endpoint to try. It's primary if there is no secondary or if it is an add # attempt.
				tryingPrimary := !considerSecondary || (try%2 == 1)
				// Select the correct host and delay
				if tryingPrimary {
					primaryTry++
					delay := o.calcDelay(primaryTry)
					logf("Primary try=%d, Delay=%v\n", primaryTry, delay)
					time.Sleep(delay) // The 1st try returns 0 delay
				} else {
					delay := time.Second * time.Duration(rand.Float32()/2+0.8)
					logf("Secondary try=%d, Delay=%v\n", try-primaryTry, delay)
					time.Sleep(delay) // Delay with some jitter before trying secondary
				}

				// Clone the original request to ensure that each try starts with the original (unmutated) request.
				requestCopy := request.Copy()

				// For each try, seek to the beginning of the Body stream. We do this even for the 1st try because
				// the stream may not be at offset 0 when we first get it and we want the same behavior for the
				// 1st try as for additional tries.
				if err = requestCopy.RewindBody(); err != nil {
					panic(err)
				}
				if !tryingPrimary {
					requestCopy.Request.URL.Host = o.retryReadsFromSecondaryHost()
				}

				// Set the server-side timeout query parameter "timeout=[seconds]"
				timeout := int32(o.TryTimeout.Seconds()) // Max seconds per try
				if deadline, ok := ctx.Deadline(); ok {  // If user's ctx has a deadline, make the timeout the smaller of the two
					t := int32(deadline.Sub(time.Now()).Seconds()) // Duration from now until user's ctx reaches its deadline
					logf("MaxTryTimeout=%d secs, TimeTilDeadline=%d sec\n", timeout, t)
					if t < timeout {
						timeout = t
					}
					if timeout < 0 {
						timeout = 0 // If timeout ever goes negative, set it to zero; this happen while debugging
					}
					logf("TryTimeout adjusted to=%d sec\n", timeout)
				}
				q := requestCopy.Request.URL.Query()
				q.Set("timeout", strconv.Itoa(int(timeout+1))) // Add 1 to "round up"
				requestCopy.Request.URL.RawQuery = q.Encode()
				logf("Url=%s\n", requestCopy.Request.URL.String())

				// Set the time for this particular retry operation and then Do the operation.
				tryCtx, tryCancel := context.WithTimeout(ctx, time.Second*time.Duration(timeout))
				//requestCopy.Body = &deadlineExceededReadCloser{r: requestCopy.Request.Body}
				response, err = next.Do(tryCtx, requestCopy) // Make the request
				/*err = improveDeadlineExceeded(err)
				if err == nil {
					response.Response().Body = &deadlineExceededReadCloser{r: response.Response().Body}
				}*/
				logf("Err=%v, response=%v\n", err, response)

				action := "" // This MUST get changed within the switch code below
				switch {
				case ctx.Err() != nil:
					action = "NoRetry: Op timeout"
				case !tryingPrimary && response != nil && response.Response().StatusCode == http.StatusNotFound:
					// If attempt was against the secondary & it returned a StatusNotFound (404), then
					// the resource was not found. This may be due to replication delay. So, in this
					// case, we'll never try the secondary again for this operation.
					considerSecondary = false
					action = "Retry: Secondary URL returned 404"
				case err != nil:
					// NOTE: Protocol Responder returns non-nil if REST API returns invalid status code for the invoked operation
					if netErr, ok := err.(net.Error); ok && (netErr.Temporary() || netErr.Timeout()) {
						action = "Retry: net.Error and Temporary() or Timeout()"
					} else {
						action = "NoRetry: unrecognized error"
					}
				default:
					action = "NoRetry: successful HTTP request" // no error
				}

				logf("Action=%s\n", action)
				// fmt.Println(action + "\n") // This is where we could log the retry operation; action is why we're retrying
				if action[0] != 'R' { // Retry only if action starts with 'R'
					if err != nil {
						tryCancel() // If we're returning an error, cancel this current/last per-retry timeout context
					} else {
						// TODO: Right now, we've decided to leak the per-try Context until the user's Context is canceled.
						// Another option is that we wrap the last per-try context in a body and overwrite the Response's Body field with our wrapper.
						// So, when the user closes the Body, the our per-try context gets closed too.
						// Another option, is that the Last Policy do this wrapping for a per-retry context (not for the user's context)
						_ = tryCancel // So, for now, we don't call cancel: cancel()
					}
					break // Don't retry
				}
				if response != nil && response.Response() != nil && response.Response().Body != nil {
					// If we're going to retry and we got a previous response, then flush its body to avoid leaking its TCP connection
					body := response.Response().Body
					io.Copy(ioutil.Discard, body)
					body.Close()
				}
				// If retrying, cancel the current per-try timeout context
				tryCancel()
			}
			return response, err // Not retryable or too many retries; return the last response/error
		}
	})
}

// According to https://github.com/golang/go/wiki/CompilerOptimizations, the compiler will inline this method and hopefully optimize all calls to it away
var logf = func(format string, a ...interface{}) {}

// Use this version to see the retry method's code path (import "fmt")
//var logf = fmt.Printf

/*
type deadlineExceededReadCloser struct {
	r io.ReadCloser
}

func (r *deadlineExceededReadCloser) Read(p []byte) (int, error) {
	n, err := 0, io.EOF
	if r.r != nil {
		n, err = r.r.Read(p)
	}
	return n, improveDeadlineExceeded(err)
}
func (r *deadlineExceededReadCloser) Seek(offset int64, whence int) (int64, error) {
	// For an HTTP request, the ReadCloser MUST also implement seek
	// For an HTTP response, Seek MUST not be called (or this will panic)
	o, err := r.r.(io.Seeker).Seek(offset, whence)
	return o, improveDeadlineExceeded(err)
}
func (r *deadlineExceededReadCloser) Close() error {
	if c, ok := r.r.(io.Closer); ok {
		c.Close()
	}
	return nil
}

// timeoutError is the internal struct that implements our richer timeout error.
type deadlineExceeded struct {
	responseError
}

var _ net.Error = (*deadlineExceeded)(nil) // Ensure deadlineExceeded implements the net.Error interface at compile time

// improveDeadlineExceeded creates a timeoutError object that implements the error interface IF cause is a context.DeadlineExceeded error.
func improveDeadlineExceeded(cause error) error {
	// If cause is not DeadlineExceeded, return the same error passed in.
	if cause != context.DeadlineExceeded {
		return cause
	}
	// Else, convert DeadlineExceeded to our timeoutError which gives a richer string message
	return &deadlineExceeded{
		responseError: responseError{
			ErrorNode: pipeline.ErrorNode{}.Initialize(cause, 3),
		},
	}
}

// Error implements the error interface's Error method to return a string representation of the error.
func (e *deadlineExceeded) Error() string {
	return e.ErrorNode.Error("context deadline exceeded; when creating a pipeline, consider increasing RetryOptions' TryTimeout field")
}
*/
