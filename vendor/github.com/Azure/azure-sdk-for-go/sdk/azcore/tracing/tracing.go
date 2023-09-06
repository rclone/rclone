//go:build go1.18
// +build go1.18

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

// Package tracing contains the definitions needed to support distributed tracing.
package tracing

import (
	"context"
)

// ProviderOptions contains the optional values when creating a Provider.
type ProviderOptions struct {
	// for future expansion
}

// NewProvider creates a new Provider with the specified values.
//   - newTracerFn is the underlying implementation for creating Tracer instances
//   - options contains optional values; pass nil to accept the default value
func NewProvider(newTracerFn func(name, version string) Tracer, options *ProviderOptions) Provider {
	return Provider{
		newTracerFn: newTracerFn,
	}
}

// Provider is the factory that creates Tracer instances.
// It defaults to a no-op provider.
type Provider struct {
	newTracerFn func(name, version string) Tracer
}

// NewTracer creates a new Tracer for the specified name and version.
//   - name - the name of the tracer object, typically the fully qualified name of the service client
//   - version - the version of the module in which the service client resides
func (p Provider) NewTracer(name, version string) (tracer Tracer) {
	if p.newTracerFn != nil {
		tracer = p.newTracerFn(name, version)
	}
	return
}

/////////////////////////////////////////////////////////////////////////////////////////////////////////////

// TracerOptions contains the optional values when creating a Tracer.
type TracerOptions struct {
	// for future expansion
}

// NewTracer creates a Tracer with the specified values.
//   - newSpanFn is the underlying implementation for creating Span instances
//   - options contains optional values; pass nil to accept the default value
func NewTracer(newSpanFn func(ctx context.Context, spanName string, options *SpanOptions) (context.Context, Span), options *TracerOptions) Tracer {
	return Tracer{
		newSpanFn: newSpanFn,
	}
}

// Tracer is the factory that creates Span instances.
type Tracer struct {
	newSpanFn func(ctx context.Context, spanName string, options *SpanOptions) (context.Context, Span)
}

// Start creates a new span and a context.Context that contains it.
//   - ctx is the parent context for this span. If it contains a Span, the newly created span will be a child of that span, else it will be a root span
//   - spanName identifies the span within a trace, it's typically the fully qualified API name
//   - options contains optional values for the span, pass nil to accept any defaults
func (t Tracer) Start(ctx context.Context, spanName string, options *SpanOptions) (context.Context, Span) {
	if t.newSpanFn != nil {
		return t.newSpanFn(ctx, spanName, options)
	}
	return ctx, Span{}
}

// SpanOptions contains optional settings for creating a span.
type SpanOptions struct {
	// Kind indicates the kind of Span.
	Kind SpanKind

	// Attributes contains key-value pairs of attributes for the span.
	Attributes []Attribute
}

/////////////////////////////////////////////////////////////////////////////////////////////////////////////

// SpanImpl abstracts the underlying implementation for Span,
// allowing it to work with various tracing implementations.
// Any zero-values will have their default, no-op behavior.
type SpanImpl struct {
	// End contains the implementation for the Span.End method.
	End func()

	// SetAttributes contains the implementation for the Span.SetAttributes method.
	SetAttributes func(...Attribute)

	// AddEvent contains the implementation for the Span.AddEvent method.
	AddEvent func(string, ...Attribute)

	// AddError contains the implementation for the Span.AddError method.
	AddError func(err error)

	// SetStatus contains the implementation for the Span.SetStatus method.
	SetStatus func(SpanStatus, string)
}

// NewSpan creates a Span with the specified implementation.
func NewSpan(impl SpanImpl) Span {
	return Span{
		impl: impl,
	}
}

// Span is a single unit of a trace.  A trace can contain multiple spans.
// A zero-value Span provides a no-op implementation.
type Span struct {
	impl SpanImpl
}

// End terminates the span and MUST be called before the span leaves scope.
// Any further updates to the span will be ignored after End is called.
func (s Span) End() {
	if s.impl.End != nil {
		s.impl.End()
	}
}

// SetAttributes sets the specified attributes on the Span.
// Any existing attributes with the same keys will have their values overwritten.
func (s Span) SetAttributes(attrs ...Attribute) {
	if s.impl.SetAttributes != nil {
		s.impl.SetAttributes(attrs...)
	}
}

// AddEvent adds a named event with an optional set of attributes to the span.
func (s Span) AddEvent(name string, attrs ...Attribute) {
	if s.impl.AddEvent != nil {
		s.impl.AddEvent(name, attrs...)
	}
}

// AddError adds the specified error event to the span.
func (s Span) AddError(err error) {
	if s.impl.AddError != nil {
		s.impl.AddError(err)
	}
}

// SetStatus sets the status on the span along with a description.
func (s Span) SetStatus(code SpanStatus, desc string) {
	if s.impl.SetStatus != nil {
		s.impl.SetStatus(code, desc)
	}
}

/////////////////////////////////////////////////////////////////////////////////////////////////////////////

// Attribute is a key-value pair.
type Attribute struct {
	// Key is the name of the attribute.
	Key string

	// Value is the attribute's value.
	// Types that are natively supported include int64, float64, int, bool, string.
	// Any other type will be formatted per rules of fmt.Sprintf("%v").
	Value any
}
