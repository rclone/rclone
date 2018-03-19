// Copyright 2018 Google Inc. All Rights Reserved.
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

// +build !go1.8

package pubsub

import (
	"golang.org/x/net/context"
	"google.golang.org/api/option"
)

// OpenCensus only supports go 1.8 and higher.

func openCensusOptions() []option.ClientOption { return nil }

func withSubscriptionKey(ctx context.Context, _ string) context.Context {
	return ctx
}

type dummy struct{}

var (
	// Not supported below Go 1.8.
	PullCount dummy
	// Not supported below Go 1.8.
	AckCount dummy
	// Not supported below Go 1.8.
	NackCount dummy
	// Not supported below Go 1.8.
	ModAckCount dummy
	// Not supported below Go 1.8.
	StreamOpenCount dummy
	// Not supported below Go 1.8.
	StreamRetryCount dummy
	// Not supported below Go 1.8.
	StreamRequestCount dummy
	// Not supported below Go 1.8.
	StreamResponseCount dummy
)

func recordStat(context.Context, dummy, int64) {
}
