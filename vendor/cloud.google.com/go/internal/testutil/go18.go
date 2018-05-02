// Copyright 2018 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build go1.8

package testutil

import (
	"log"
	"time"

	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
)

type TestExporter struct {
	Spans []*trace.SpanData
	Stats chan *view.Data
}

func NewTestExporter() *TestExporter {
	te := &TestExporter{Stats: make(chan *view.Data)}

	view.RegisterExporter(te)
	view.SetReportingPeriod(time.Millisecond)
	if err := view.Register(ocgrpc.ClientRequestCountView); err != nil {
		log.Fatal(err)
	}

	trace.RegisterExporter(te)
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})

	return te
}

func (te *TestExporter) ExportSpan(s *trace.SpanData) {
	te.Spans = append(te.Spans, s)
}

func (te *TestExporter) ExportView(vd *view.Data) {
	if len(vd.Rows) > 0 {
		select {
		case te.Stats <- vd:
		default:
		}
	}
}

func (te *TestExporter) Unregister() {
	view.UnregisterExporter(te)
	trace.UnregisterExporter(te)
}
