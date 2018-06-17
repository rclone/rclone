// Copyright 2017 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

//+build go1.7

package delay

import (
	"bytes"
	stdctx "context"
	"net/http"
	"net/http/httptest"
	"testing"

	netctx "golang.org/x/net/context"
	"google.golang.org/appengine/taskqueue"
)

var (
	stdCtxRuns = 0
	stdCtxFunc = Func("stdctx", func(c stdctx.Context) {
		stdCtxRuns++
	})
)

func TestStandardContext(t *testing.T) {
	// Fake out the adding of a task.
	var task *taskqueue.Task
	taskqueueAdder = func(_ netctx.Context, tk *taskqueue.Task, queue string) (*taskqueue.Task, error) {
		if queue != "" {
			t.Errorf(`Got queue %q, expected ""`, queue)
		}
		task = tk
		return tk, nil
	}

	c := newFakeContext()
	stdCtxRuns = 0 // reset state
	if err := stdCtxFunc.Call(c.ctx); err != nil {
		t.Fatal("Function.Call:", err)
	}

	// Simulate the Task Queue service.
	req, err := http.NewRequest("POST", path, bytes.NewBuffer(task.Payload))
	if err != nil {
		t.Fatalf("Failed making http.Request: %v", err)
	}
	rw := httptest.NewRecorder()
	runFunc(c.ctx, rw, req)

	if stdCtxRuns != 1 {
		t.Errorf("stdCtxRuns: got %d, want 1", stdCtxRuns)
	}
}
