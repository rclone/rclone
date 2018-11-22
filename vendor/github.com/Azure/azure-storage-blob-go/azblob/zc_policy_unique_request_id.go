package azblob

import (
	"context"

	"github.com/Azure/azure-pipeline-go/pipeline"
)

// NewUniqueRequestIDPolicyFactory creates a UniqueRequestIDPolicyFactory object
// that sets the request's x-ms-client-request-id header if it doesn't already exist.
func NewUniqueRequestIDPolicyFactory() pipeline.Factory {
	return pipeline.FactoryFunc(func(next pipeline.Policy, po *pipeline.PolicyOptions) pipeline.PolicyFunc {
		// This is Policy's Do method:
		return func(ctx context.Context, request pipeline.Request) (pipeline.Response, error) {
			id := request.Header.Get(xMsClientRequestID)
			if id == "" { // Add a unique request ID if the caller didn't specify one already
				request.Header.Set(xMsClientRequestID, newUUID().String())
			}
			return next.Do(ctx, request)
		}
	})
}

const xMsClientRequestID = "x-ms-client-request-id"
