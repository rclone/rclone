package azblob

import (
	"github.com/Azure/azure-pipeline-go/pipeline"
)

// PipelineOptions is used to configure a request policy pipeline's retry policy and logging.
type PipelineOptions struct {
	// Log configures the pipeline's logging infrastructure indicating what information is logged and where.
	Log pipeline.LogOptions

	// Retry configures the built-in retry policy behavior.
	Retry RetryOptions

	// RequestLog configures the built-in request logging policy.
	RequestLog RequestLogOptions

	// Telemetry configures the built-in telemetry policy behavior.
	Telemetry TelemetryOptions
}

// NewPipeline creates a Pipeline using the specified credentials and options.
func NewPipeline(c Credential, o PipelineOptions) pipeline.Pipeline {
	if c == nil {
		panic("c can't be nil")
	}

	// Closest to API goes first; closest to the wire goes last
	f := []pipeline.Factory{
		NewTelemetryPolicyFactory(o.Telemetry),
		NewUniqueRequestIDPolicyFactory(),
		NewRetryPolicyFactory(o.Retry),
	}

	if _, ok := c.(*anonymousCredentialPolicyFactory); !ok {
		// For AnonymousCredential, we optimize out the policy factory since it doesn't do anything
		// NOTE: The credential's policy factory must appear close to the wire so it can sign any
		// changes made by other factories (like UniqueRequestIDPolicyFactory)
		f = append(f, c)
	}
	f = append(f,
		pipeline.MethodFactoryMarker(), // indicates at what stage in the pipeline the method factory is invoked
		NewRequestLogPolicyFactory(o.RequestLog))

	return pipeline.NewPipeline(f, pipeline.Options{HTTPSender: nil, Log: o.Log})
}
