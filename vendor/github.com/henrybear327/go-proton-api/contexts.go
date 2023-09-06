package proton

import "context"

type withClientKeyType struct{}

var withClientKey withClientKeyType

// WithClient marks this context as originating from the client with the given ID.
func WithClient(parent context.Context, clientID uint64) context.Context {
	return context.WithValue(parent, withClientKey, clientID)
}

// ClientIDFromContext returns true if this context was marked as originating from a client.
func ClientIDFromContext(ctx context.Context) (uint64, bool) {
	clientID, ok := ctx.Value(withClientKey).(uint64)
	if !ok {
		return 0, false
	}

	return clientID, true
}
