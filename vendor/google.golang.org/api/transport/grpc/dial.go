// Copyright 2015 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package transport/grpc supports network connections to GRPC servers.
// This package is not intended for use by end developers. Use the
// google.golang.org/api/option package to configure API clients.
package transport

import (
	"errors"
	"fmt"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/internal"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
)

// Set at init time by dial_appengine.go. If nil, we're not on App Engine.
var appengineDialerHook func(context.Context) grpc.DialOption

// Dial returns a GRPC connection for use communicating with a Google cloud
// service, configured with the given ClientOptions.
func Dial(ctx context.Context, opts ...option.ClientOption) (*grpc.ClientConn, error) {
	var o internal.DialSettings
	for _, opt := range opts {
		opt.Apply(&o)
	}
	if o.HTTPClient != nil {
		return nil, errors.New("unsupported HTTP client specified")
	}
	if o.GRPCConn != nil {
		return o.GRPCConn, nil
	}
	if o.ServiceAccountJSONFilename != "" {
		ts, err := internal.ServiceAcctTokenSource(ctx, o.ServiceAccountJSONFilename, o.Scopes...)
		if err != nil {
			return nil, err
		}
		o.TokenSource = ts
	}
	if o.TokenSource == nil {
		var err error
		o.TokenSource, err = google.DefaultTokenSource(ctx, o.Scopes...)
		if err != nil {
			return nil, fmt.Errorf("google.DefaultTokenSource: %v", err)
		}
	}
	grpcOpts := []grpc.DialOption{
		grpc.WithPerRPCCredentials(oauth.TokenSource{o.TokenSource}),
		grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(nil, "")),
	}
	if appengineDialerHook != nil {
		// Use the Socket API on App Engine.
		grpcOpts = append(grpcOpts, appengineDialerHook(ctx))
	}
	grpcOpts = append(grpcOpts, o.GRPCDialOpts...)
	if o.UserAgent != "" {
		grpcOpts = append(grpcOpts, grpc.WithUserAgent(o.UserAgent))
	}
	return grpc.DialContext(ctx, o.Endpoint, grpcOpts...)
}

// DialInsecure returns an insecure GRPC connection for use communicating
// with fake or mock Google cloud service implementations, such as emulators.
// The connection is configured with the given ClientOptions.
func DialInsecure(ctx context.Context, opts ...option.ClientOption) (*grpc.ClientConn, error) {
	var o internal.DialSettings
	for _, opt := range opts {
		opt.Apply(&o)
	}
	if o.HTTPClient != nil {
		return nil, errors.New("unsupported HTTP client specified")
	}
	if o.GRPCConn != nil {
		return o.GRPCConn, nil
	}
	grpcOpts := []grpc.DialOption{grpc.WithInsecure()}
	grpcOpts = append(grpcOpts, o.GRPCDialOpts...)
	if o.UserAgent != "" {
		grpcOpts = append(grpcOpts, grpc.WithUserAgent(o.UserAgent))
	}
	return grpc.DialContext(ctx, o.Endpoint, grpcOpts...)
}
