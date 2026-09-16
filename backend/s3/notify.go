// Change notification for MinIO servers, built on the
// ListenBucketNotification API extension.

package s3

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	v4signer "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/rclone/rclone/fs"
)

const (
	// emptySHA256Hex is the hex encoded SHA-256 of the empty string, the
	// payload hash for a request with no body.
	emptySHA256Hex = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	// notifyPingSeconds is how often the server is asked to emit a keepalive
	// line. It must stay well inside --timeout, which the fshttp transport
	// applies as an idle deadline, or a quiet stream would be torn down.
	notifyPingSeconds = 10

	// notifyMaxLineSize bounds a single line of the stream. Proxies may
	// coalesce records, so the limit matches the one minio-go uses.
	notifyMaxLineSize = 4 * 1024 * 1024

	// notifyMinBackoff and notifyMaxBackoff bound the reconnection delay.
	notifyMinBackoff = time.Second
	notifyMaxBackoff = 30 * time.Second

	// notifyMaxErrorBody is how much of an error response body is read to
	// find the S3 error code.
	notifyMaxErrorBody = 4096
)

// notificationEvent is a single object event from the notification stream.
type notificationEvent struct {
	EventName string `json:"eventName"`
	S3        struct {
		Bucket struct {
			Name string `json:"name"`
		} `json:"bucket"`
		Object struct {
			Key string `json:"key"`
		} `json:"object"`
	} `json:"s3"`
}

// notificationInfo is one line of the notification stream. A line with no
// records is a keepalive produced by the ping parameter.
type notificationInfo struct {
	Records []notificationEvent `json:"Records"`
}

// errPermanent marks a failure which will not be fixed by reconnecting, such
// as a server without the extension or a policy which denies it. The stream
// gives up for the life of the Fs rather than retrying forever.
type errPermanent struct {
	err error
	// notice asks for NOTICE rather than INFO, for failures the user can act on
	notice bool
}

// Error returns the underlying error message.
func (e errPermanent) Error() string { return e.err.Error() }

// Unwrap returns the underlying error.
func (e errPermanent) Unwrap() error { return e.err }

// notifyRelativePath converts a bucket relative object key into a path
// relative to rootDirectory.
//
// It returns ok false if key is not under rootDirectory. The server side
// prefix filter makes that rare, but it is an optimisation rather than a
// guarantee, so the check is made regardless.
func notifyRelativePath(rootDirectory, key string) (relPath string, ok bool) {
	if rootDirectory == "" {
		return key, true
	}
	if key == rootDirectory {
		return "", true
	}
	// Match on the trailing slash so that a sibling directory sharing the
	// prefix, eg "docsets/x" under root "docs", is not mistaken for a child
	if prefix := rootDirectory + "/"; strings.HasPrefix(key, prefix) {
		return key[len(prefix):], true
	}
	return "", false
}

// notifyEntryType classifies an object key. Directory markers are keys ending
// in a slash, and reporting them as directories makes the VFS invalidate the
// directory itself as well as its parent.
func notifyEntryType(key string) fs.EntryType {
	if strings.HasSuffix(key, "/") {
		return fs.EntryDirectory
	}
	return fs.EntryObject
}

// notifyEventWanted reports whether an event name is an object create or
// remove. Anything else, such as an access or a replication event, leaves
// directory listings unchanged.
func notifyEventWanted(eventName string) bool {
	return strings.HasPrefix(eventName, "s3:ObjectCreated:") ||
		strings.HasPrefix(eventName, "s3:ObjectRemoved:")
}

// notifyEndpointUnsupported reports whether endpoint belongs to a provider
// known not to implement ListenBucketNotification.
//
// Quirk gating means this should not trigger in practice; it guards anyone
// who sets use_bucket_notifications by hand on the wrong provider.
func notifyEndpointUnsupported(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, suffix := range []string{"amazonaws.com", "googleapis.com"} {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}

// notifyURL builds the ListenBucketNotification URL for bucketName, watching
// keys under prefix. endpoint must carry a scheme, and prefix may be empty to
// watch the whole bucket.
func notifyURL(endpoint, bucketName, prefix string, forcePathStyle bool) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("failed to parse endpoint %q: %w", endpoint, err)
	}
	if u.Host == "" {
		return "", fmt.Errorf("endpoint %q has no host", endpoint)
	}
	if forcePathStyle {
		u.Path = path.Join("/", u.Path, bucketName)
	} else {
		u.Host = bucketName + "." + u.Host
		u.Path = path.Join("/", u.Path)
	}
	u.RawQuery = url.Values{
		"ping":   []string{strconv.Itoa(notifyPingSeconds)},
		"prefix": []string{prefix},
		"suffix": []string{""},
		"events": []string{"s3:ObjectCreated:*", "s3:ObjectRemoved:*"},
	}.Encode()
	return u.String(), nil
}

// notifyStatusError converts a non-200 response into an error, marking it
// permanent when neither the server nor its policy will ever allow the stream.
func notifyStatusError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, notifyMaxErrorBody))
	var parsed struct {
		Code string `xml:"Code"`
	}
	_ = xml.Unmarshal(body, &parsed)
	detail := strings.TrimSpace(string(body))
	switch {
	case resp.StatusCode == http.StatusNotImplemented || parsed.Code == "NotImplemented":
		return errPermanent{err: fmt.Errorf("server does not support bucket notifications: %s: %s", resp.Status, detail)}
	case resp.StatusCode == http.StatusForbidden || parsed.Code == "AccessDenied":
		return errPermanent{
			err:    fmt.Errorf("bucket notifications denied - the s3:ListenBucketNotification permission is required: %s: %s", resp.Status, detail),
			notice: true,
		}
	}
	return fmt.Errorf("bucket notification stream failed: %s: %s", resp.Status, detail)
}

// notifyPrefix returns the key prefix to watch for this Fs.
func (f *Fs) notifyPrefix() string {
	if f.rootDirectory == "" {
		return ""
	}
	return f.rootDirectory + "/"
}

// handleNotification maps one event onto a notifyFunc call, dropping events
// which cannot affect a directory listing under this Fs.
func (f *Fs) handleNotification(event notificationEvent, notifyFunc func(string, fs.EntryType)) {
	if !notifyEventWanted(event.EventName) {
		fs.Debugf(f, "Ignoring bucket notification event %q", event.EventName)
		return
	}
	// Keys arrive verbatim. Unlike the S3 event notifications AWS delivers to
	// Lambda and SQS, which URL-encode the key, MinIO sends it unescaped, so
	// decoding here would corrupt any key containing "+" or a literal "%".
	key := event.S3.Object.Key
	relPath, ok := notifyRelativePath(f.rootDirectory, key)
	if !ok {
		return
	}
	fs.Debugf(f, "Bucket notification %s: %q", event.EventName, key)
	notifyFunc(relPath, notifyEntryType(key))
}

// listenOnce opens one notification stream and reads it until it ends, the
// context is cancelled, or an error occurs.
//
// It returns delivered true if the server sent at least one line, including a
// keepalive, which is what marks the connection healthy enough to reset the
// backoff. An errPermanent means the caller must not reconnect.
func (f *Fs) listenOnce(ctx context.Context, notifyFunc func(string, fs.EntryType)) (delivered bool, err error) {
	if f.opt.Endpoint == "" {
		return false, errPermanent{err: errors.New("bucket notifications need an explicit endpoint")}
	}
	if notifyEndpointUnsupported(f.opt.Endpoint) {
		return false, errPermanent{err: fmt.Errorf("endpoint %q does not support bucket notifications", f.opt.Endpoint)}
	}
	streamURL, err := notifyURL(f.opt.Endpoint, f.rootBucket, f.notifyPrefix(), f.opt.ForcePathStyle)
	if err != nil {
		return false, errPermanent{err: err}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, streamURL, nil)
	if err != nil {
		return false, errPermanent{err: err}
	}
	req.Header.Set("X-Amz-Content-Sha256", emptySHA256Hex)

	// Retrieve the credentials immediately before connecting rather than
	// caching them across attempts. This is an aws.CredentialsCache, so a
	// stream which outlives its credentials picks up fresh ones when it
	// reconnects.
	creds, err := f.c.Options().Credentials.Retrieve(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to retrieve credentials for notification stream: %w", err)
	}
	err = v4signer.NewSigner().SignHTTP(ctx, creds, req, emptySHA256Hex, "s3", f.opt.Region, time.Now())
	if err != nil {
		return false, fmt.Errorf("failed to sign notification request: %w", err)
	}

	resp, err := f.srv.Do(req)
	if err != nil {
		return false, err
	}
	defer fs.CheckClose(resp.Body, &err)
	if resp.StatusCode != http.StatusOK {
		return false, notifyStatusError(resp)
	}

	// ListenBucketNotification has no resume token, so events which happened
	// while disconnected cannot be enumerated. Invalidating the root recovers
	// its listing; subdirectories cached before the gap stay cached until
	// --dir-cache-time expires.
	notifyFunc("", fs.EntryDirectory)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(nil, notifyMaxLineSize)
	for scanner.Scan() {
		delivered = true
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var info notificationInfo
		if err := json.Unmarshal(line, &info); err != nil {
			fs.Debugf(f, "Ignoring malformed bucket notification line: %v", err)
			continue
		}
		for _, event := range info.Records {
			f.handleNotification(event, notifyFunc)
		}
	}
	return delivered, scanner.Err()
}

// listenBucketNotification keeps a notification stream running until ctx is
// cancelled, reconnecting with jittered backoff.
//
// If the failure is one retrying cannot fix it sets disabled, which stops the
// subscription being started again for the life of this Fs.
func (f *Fs) listenBucketNotification(ctx context.Context, notifyFunc func(string, fs.EntryType), disabled *atomic.Bool) {
	backoff := notifyMinBackoff
	for {
		delivered, err := f.listenOnce(ctx, notifyFunc)
		if ctx.Err() != nil {
			return
		}
		var permanent errPermanent
		if errors.As(err, &permanent) {
			disabled.Store(true)
			if permanent.notice {
				fs.Logf(f, "Disabling bucket notifications: %v", err)
			} else {
				fs.Infof(f, "Disabling bucket notifications: %v", err)
			}
			return
		}
		if err != nil {
			fs.Debugf(f, "Bucket notification stream ended, reconnecting: %v", err)
		}
		// Sleep for a random duration up to the current backoff so that many
		// mounts reconnecting together do not synchronise
		wait := backoff/2 + time.Duration(rand.Int63n(int64(backoff/2)+1))
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		if delivered {
			backoff = notifyMinBackoff
		} else {
			backoff = min(backoff*2, notifyMaxBackoff)
		}
	}
}

// startListener starts a notification stream in the background and returns a
// function which stops it.
func (f *Fs) startListener(ctx context.Context, notifyFunc func(string, fs.EntryType), disabled *atomic.Bool) context.CancelFunc {
	subCtx, cancel := context.WithCancel(ctx)
	go f.listenBucketNotification(subCtx, notifyFunc, disabled)
	return cancel
}

// changeNotify calls notifyFunc for paths which have changed, driven by
// MinIO's ListenBucketNotification stream.
//
// Notifications are pushed by the server, so pollIntervalChan gates whether
// the subscription runs rather than setting a cadence: any non-zero interval
// starts it and 0 stops it. Closing the channel shuts it down for good.
func (f *Fs) changeNotify(ctx context.Context, notifyFunc func(string, fs.EntryType), pollIntervalChan <-chan time.Duration) {
	var disabled atomic.Bool
	go func() {
		var cancel context.CancelFunc
		stop := func() {
			if cancel != nil {
				cancel()
				cancel = nil
			}
		}
		defer stop()
		for {
			select {
			case pollInterval, ok := <-pollIntervalChan:
				if !ok {
					return
				}
				stop()
				if pollInterval != 0 && !disabled.Load() {
					cancel = f.startListener(ctx, notifyFunc, &disabled)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}
