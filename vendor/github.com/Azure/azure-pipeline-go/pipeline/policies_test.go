package pipeline_test

import (
	"context"

	"github.com/Azure/azure-pipeline-go/pipeline"
)

// Here is the template for defining your own Factory & Policy:

// newMyPolicyFactory creates a 'My' policy factory. Make this function
// public if this should be callable from another package; everything
// else about the factory/policy should remain private to the package.
func newMyPolicyFactory( /* Desired parameters */ ) pipeline.Factory {
	return &myPolicyFactory{ /* Set desired fields */ }
}

type myPolicyFactory struct {
	// Desired fields (goroutine-safe because the factory is shared by many Policy objects)
}

// New initializes a Xxx policy object.
func (f *myPolicyFactory) New(next pipeline.Policy, po *pipeline.PolicyOptions) pipeline.Policy {
	return &myPolicy{next: next, po: po /* Set desired fields */}
}

type myPolicy struct {
	next pipeline.Policy
	po   *pipeline.PolicyOptions // Optional private field
	// Additional desired fields (mutable for use by this specific Policy object)
}

func (p *myPolicy) Do(ctx context.Context, request pipeline.Request) (response pipeline.Response, err error) {
	// TODO: Put your policy behavior code here
	// Your code should NOT mutate the ctx or request parameters
	// However, you can make a copy of the request and mutate the copy
	// You can also pass a different Context on.
	// You can optionally use po (PolicyOptions) in this func.

	// Forward the request to the next node in the pipeline:
	response, err = p.next.Do(ctx, request)

	// Process the response here. You can deserialize the body into an object.
	// If you do this, also define a struct that wraps an http.Response & your
	// deserialized struct. Have your wrapper struct implement the
	// pipeline.Response interface and then return your struct (via the interface)
	// After the pipeline completes, take response and perform a type assertion
	// to get back to the wrapper struct so you can access the deserialized object.

	return // Return the response & err
}

func newMyPolicyFactory2( /* Desired parameters */ ) pipeline.Factory {
	return pipeline.FactoryFunc(func(next pipeline.Policy, po *pipeline.PolicyOptions) pipeline.PolicyFunc {
		return func(ctx context.Context, request pipeline.Request) (response pipeline.Response, err error) {
			// TODO: Put your policy behavior code here
			// Your code should NOT mutate the ctx or request parameters
			// However, you can make a copy of the request and mutate the copy
			// You can also pass a different Context on.
			// You can optionally use po (PolicyOptions) in this func.

			// Forward the request to the next node in the pipeline:
			response, err = next.Do(ctx, request)

			// Process the response here. You can deserialize the body into an object.
			// If you do this, also define a struct that wraps an http.Response & your
			// deserialized struct. Have your wrapper struct implement the
			// pipeline.Response interface and then return your struct (via the interface)
			// After the pipeline completes, take response and perform a type assertion
			// to get back to the wrapper struct so you can access the deserialized object.

			return // Return the response & err
		}
	})
}
