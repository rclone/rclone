// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

// Package experiment implements feature flag propagation.
package experiment

import (
	"context"
	"strings"
)

type key int

const (
	contextKey key    = iota
	drpcKey    string = "experiment"
)

// With registers the feature flag of an ongoing experiment.
func With(ctx context.Context, exp string) context.Context {
	existingValue := ctx.Value(contextKey)
	if existingValue != nil {
		if s, ok := existingValue.(string); ok {
			return context.WithValue(ctx, contextKey, s+","+exp)
		}
	}
	return context.WithValue(ctx, contextKey, exp)
}

// Get returns the registered feature flags.
func Get(ctx context.Context) []string {
	value := ctx.Value(contextKey)
	if value == nil {
		return []string{}
	}
	if s, ok := value.(string); ok {
		return strings.Split(s, ",")
	}
	return []string{}
}

// Has checks if the experiment registered to the comma separated list.
func Has(ctx context.Context, exp string) bool {
	value := ctx.Value(contextKey)
	if value == nil {
		return false
	}
	if s, ok := value.(string); ok {
		for _, e := range strings.Split(s, ",") {
			if e == exp {
				return true
			}
		}
	}
	return false
}
