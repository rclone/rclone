package azblob

import (
	"context"
	"sync/atomic"

	"github.com/Azure/azure-pipeline-go/pipeline"
)

// NewTokenCredential creates a token credential for use with role-based
// access control (RBAC) access to Azure Storage resources.
func NewTokenCredential(token string) *TokenCredential {
	f := &TokenCredential{}
	f.SetToken(token)
	return f
}

// TokenCredential is a pipeline.Factory is the credential's policy factory.
type TokenCredential struct{ token atomic.Value }

// Token returns the current token value
func (f *TokenCredential) Token() string { return f.token.Load().(string) }

// SetToken changes the current token value
func (f *TokenCredential) SetToken(token string) { f.token.Store(token) }

// credentialMarker is a package-internal method that exists just to satisfy the Credential interface.
func (*TokenCredential) credentialMarker() {}

// New creates a credential policy object.
func (f *TokenCredential) New(next pipeline.Policy, po *pipeline.PolicyOptions) pipeline.Policy {
	return pipeline.PolicyFunc(func(ctx context.Context, request pipeline.Request) (pipeline.Response, error) {
		if request.URL.Scheme != "https" {
			panic("Token credentials require a URL using the https protocol scheme.")
		}
		request.Header[headerAuthorization] = []string{"Bearer " + f.Token()}
		return next.Do(ctx, request)
	})
}
