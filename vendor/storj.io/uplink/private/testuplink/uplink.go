// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package testuplink

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"storj.io/common/memory"
	"storj.io/uplink/private/eestream/scheduler"
)

type segmentSizeKey struct{}

type plainSizeKey struct{}

type listLimitKey struct{}

type concurrentSegmentUploadsConfigKey struct{}

type disableConcurrentSegmentUploadsKey struct{}

type (
	logWriterKey        struct{}
	logWriterContextKey struct{}
)

// WithMaxSegmentSize creates context with max segment size for testing purposes.
//
// Created context needs to be used with uplink.OpenProject to manipulate default
// segment size.
func WithMaxSegmentSize(ctx context.Context, segmentSize memory.Size) context.Context {
	return context.WithValue(ctx, segmentSizeKey{}, segmentSize)
}

// GetMaxSegmentSize returns max segment size from context if exists.
func GetMaxSegmentSize(ctx context.Context) (memory.Size, bool) {
	segmentSize, ok := ctx.Value(segmentSizeKey{}).(memory.Size)
	return segmentSize, ok
}

// WithoutPlainSize creates context with information that segment plain size shouldn't be sent.
// Only for testing purposes.
func WithoutPlainSize(ctx context.Context) context.Context {
	return context.WithValue(ctx, plainSizeKey{}, true)
}

// IsWithoutPlainSize returns true if information about not sending segment plain size exists in context.
// Only for testing purposes.
func IsWithoutPlainSize(ctx context.Context) bool {
	withoutPlainSize, _ := ctx.Value(plainSizeKey{}).(bool)
	return withoutPlainSize
}

// WithListLimit creates context with information about list limit that will be used with request.
// Only for testing purposes.
func WithListLimit(ctx context.Context, limit int) context.Context {
	return context.WithValue(ctx, listLimitKey{}, limit)
}

// GetListLimit returns value for list limit if exists in context.
// Only for testing purposes.
func GetListLimit(ctx context.Context) int {
	limit, _ := ctx.Value(listLimitKey{}).(int)
	return limit
}

// ConcurrentSegmentUploadsConfig is the configuration for concurrent
// segment uploads using the new upload codepath.
type ConcurrentSegmentUploadsConfig struct {
	// SchedulerOptions are the options for the scheduler used to place limits
	// on the amount of concurrent piece limits per-upload, across all
	// segments.
	SchedulerOptions scheduler.Options

	// LongTailMargin represents the maximum number of piece uploads beyond the
	// optimal threshold that will be uploaded for a given segment. Once an
	// upload has reached the optimal threshold, the remaining piece uploads
	// are cancelled.
	LongTailMargin int
}

// DefaultConcurrentSegmentUploadsConfig returns the default ConcurrentSegmentUploadsConfig.
func DefaultConcurrentSegmentUploadsConfig() ConcurrentSegmentUploadsConfig {
	return ConcurrentSegmentUploadsConfig{
		SchedulerOptions: scheduler.Options{
			MaximumConcurrent:        300,
			MaximumConcurrentHandles: 10,
		},
		LongTailMargin: 50,
	}
}

// WithConcurrentSegmentUploadsDefaultConfig creates a context that enables the
// new concurrent segment upload codepath for testing purposes using the
// default configuration.
//
// The context needs to be used with uplink.OpenProject to have effect.
func WithConcurrentSegmentUploadsDefaultConfig(ctx context.Context) context.Context {
	return WithConcurrentSegmentUploadsConfig(ctx, DefaultConcurrentSegmentUploadsConfig())
}

// WithConcurrentSegmentUploadsConfig creates a context that enables the
// new concurrent segment upload codepath for testing purposes using the
// given scheduler options.
//
// The context needs to be used with uplink.OpenProject to have effect.
func WithConcurrentSegmentUploadsConfig(ctx context.Context, config ConcurrentSegmentUploadsConfig) context.Context {
	return context.WithValue(ctx, concurrentSegmentUploadsConfigKey{}, config)
}

// DisableConcurrentSegmentUploads creates a context that disables the new
// concurrent segment upload codepath.
func DisableConcurrentSegmentUploads(ctx context.Context) context.Context {
	return context.WithValue(ctx, disableConcurrentSegmentUploadsKey{}, struct{}{})
}

// GetConcurrentSegmentUploadsConfig returns the scheduler options to
// use with the new concurrent segment upload codepath, if no scheduler
// options have been set it will return default configuration. Concurrent
// segment upload code path can be disabled with DisableConcurrentSegmentUploads.
func GetConcurrentSegmentUploadsConfig(ctx context.Context) *ConcurrentSegmentUploadsConfig {
	if value := ctx.Value(disableConcurrentSegmentUploadsKey{}); value != nil {
		return nil
	}
	if config, ok := ctx.Value(concurrentSegmentUploadsConfigKey{}).(ConcurrentSegmentUploadsConfig); ok {
		return &config
	}
	config := DefaultConcurrentSegmentUploadsConfig()
	return &config
}

// WithLogWriter creates context with information about upload log file.
func WithLogWriter(ctx context.Context, w io.Writer) context.Context {
	return context.WithValue(ctx, logWriterKey{}, w)
}

// GetLogWriter returns upload log file from context if exists.
func GetLogWriter(ctx context.Context) io.Writer {
	if w, ok := ctx.Value(logWriterKey{}).(io.Writer); ok {
		return w
	}
	return nil
}

type contextKeyList struct {
	key  string
	val  string
	next *contextKeyList
}

// WithLogWriterContext appends the key/val pair to the context that is logged with
// each Log call.
func WithLogWriterContext(ctx context.Context, kvs ...string) context.Context {
	for i := 0; i+1 < len(kvs); i += 2 {
		ctx = context.WithValue(ctx, logWriterContextKey{}, &contextKeyList{
			key:  kvs[i],
			val:  kvs[i+1],
			next: getLogWriterContext(ctx),
		})
	}
	return ctx
}

func getLogWriterContext(ctx context.Context) *contextKeyList {
	l, _ := ctx.Value(logWriterContextKey{}).(*contextKeyList)
	return l
}

var (
	logMu    sync.Mutex
	logStart = time.Now()
)

// Log writes to upload log file if exists.
func Log(ctx context.Context, args ...interface{}) {
	w := GetLogWriter(ctx)
	if w == nil {
		return
	}

	logMu.Lock()
	defer logMu.Unlock()

	now := time.Now()

	_, _ = io.WriteString(w, now.Truncate(0).Format(time.StampNano))
	_, _ = io.WriteString(w, " (")
	_, _ = fmt.Fprintf(w, "%-12s", now.Sub(logStart).String())
	_, _ = io.WriteString(w, ")")

	l, first := getLogWriterContext(ctx), true
	for ; l != nil; l, first = l.next, false {
		if first {
			_, _ = io.WriteString(w, " [")
		} else {
			_, _ = io.WriteString(w, ", ")
		}
		_, _ = io.WriteString(w, l.key)
		_, _ = io.WriteString(w, "=")
		_, _ = io.WriteString(w, l.val)
	}
	if !first {
		_, _ = io.WriteString(w, "]")
	}

	_, _ = io.WriteString(w, ": ")
	_, _ = fmt.Fprintln(w, args...)
}
