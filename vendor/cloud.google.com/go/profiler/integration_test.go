// Copyright 2017 Google Inc. All Rights Reserved.
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

// +build integration,go1.7

package profiler

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"testing"
	"text/template"
	"time"

	"cloud.google.com/go/profiler/proftest"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v1"
)

var (
	commit = flag.String("commit", "", "git commit to test")
	runID  = time.Now().Unix()
)

const (
	cloudScope        = "https://www.googleapis.com/auth/cloud-platform"
	benchFinishString = "busybench finished profiling"
)

const startupTemplate = `
#! /bin/bash

(
# Shut down the VM in 5 minutes after this script exits
# to stop accounting the VM for billing and cores quota.
trap "sleep 300 && poweroff" EXIT

# Fail on any error.
set -eo pipefail

# Display commands being run.
set -x

# Install git
apt-get update  >/dev/null
apt-get -y -q install git >/dev/null

# Install desired Go version
mkdir -p /tmp/bin
curl -sL -o /tmp/bin/gimme https://raw.githubusercontent.com/travis-ci/gimme/master/gimme
chmod +x /tmp/bin/gimme
export PATH=$PATH:/tmp/bin

eval "$(gimme {{.GoVersion}})"

# Set $GOPATH
export GOPATH="$HOME/go"

export GOCLOUD_HOME=$GOPATH/src/cloud.google.com/go
mkdir -p $GOCLOUD_HOME

# Install agent
git clone https://code.googlesource.com/gocloud $GOCLOUD_HOME >/dev/null

cd $GOCLOUD_HOME/profiler/busybench
git reset --hard {{.Commit}}
go get -v >/dev/null

# Run benchmark with agent
go run busybench.go --service="{{.Service}}" --mutex_profiling="{{.MutexProfiling}}"

# Write output to serial port 2 with timestamp.
) 2>&1 | while read line; do echo "$(date): ${line}"; done >/dev/ttyS1
`

const dockerfileFmt = `FROM golang
RUN git clone https://code.googlesource.com/gocloud /go/src/cloud.google.com/go \
    && cd /go/src/cloud.google.com/go/profiler/busybench && git reset --hard %s \
    && go get -v && go install -v
CMD ["busybench", "--service", "%s"]
 `

type goGCETestCase struct {
	proftest.InstanceConfig
	name             string
	goVersion        string
	mutexProfiling   bool
	wantProfileTypes []string
}

func (tc *goGCETestCase) initializeStartupScript(template *template.Template) error {
	var buf bytes.Buffer
	err := template.Execute(&buf,
		struct {
			Service        string
			GoVersion      string
			Commit         string
			MutexProfiling bool
		}{
			Service:        tc.name,
			GoVersion:      tc.goVersion,
			Commit:         *commit,
			MutexProfiling: tc.mutexProfiling,
		})
	if err != nil {
		return fmt.Errorf("failed to render startup script for %s: %v", tc.name, err)
	}
	tc.StartupScript = buf.String()
	return nil
}

func TestAgentIntegration(t *testing.T) {
	projectID := os.Getenv("GCLOUD_TESTS_GOLANG_PROJECT_ID")
	if projectID == "" {
		t.Fatalf("Getenv(GCLOUD_TESTS_GOLANG_PROJECT_ID) got empty string")
	}

	zone := os.Getenv("GCLOUD_TESTS_GOLANG_ZONE")
	if zone == "" {
		t.Fatalf("Getenv(GCLOUD_TESTS_GOLANG_ZONE) got empty string")
	}

	if *commit == "" {
		t.Fatal("commit flag is not set")
	}

	ctx := context.Background()

	client, err := google.DefaultClient(ctx, cloudScope)
	if err != nil {
		t.Fatalf("failed to get default client: %v", err)
	}

	computeService, err := compute.New(client)
	if err != nil {
		t.Fatalf("failed to initialize compute service: %v", err)
	}

	template, err := template.New("startupScript").Parse(startupTemplate)
	if err != nil {
		t.Fatalf("failed to parse startup script template: %v", err)
	}

	tr := proftest.TestRunner{
		Client: client,
	}

	gceTr := proftest.GCETestRunner{
		TestRunner:     tr,
		ComputeService: computeService,
	}

	testcases := []goGCETestCase{
		{
			InstanceConfig: proftest.InstanceConfig{
				ProjectID:   projectID,
				Zone:        zone,
				Name:        fmt.Sprintf("profiler-test-go110-%d", runID),
				MachineType: "n1-standard-1",
			},
			name:             fmt.Sprintf("profiler-test-go110-%d-gce", runID),
			wantProfileTypes: []string{"CPU", "HEAP", "THREADS", "CONTENTION"},
			goVersion:        "1.10",
			mutexProfiling:   true,
		},
		{
			InstanceConfig: proftest.InstanceConfig{
				ProjectID:   projectID,
				Zone:        zone,
				Name:        fmt.Sprintf("profiler-test-go19-%d", runID),
				MachineType: "n1-standard-1",
			},
			name:             fmt.Sprintf("profiler-test-go19-%d-gce", runID),
			wantProfileTypes: []string{"CPU", "HEAP", "THREADS", "CONTENTION"},
			goVersion:        "1.9",
			mutexProfiling:   true,
		},
		{
			InstanceConfig: proftest.InstanceConfig{
				ProjectID:   projectID,
				Zone:        zone,
				Name:        fmt.Sprintf("profiler-test-go18-%d", runID),
				MachineType: "n1-standard-1",
			},
			name:             fmt.Sprintf("profiler-test-go18-%d-gce", runID),
			wantProfileTypes: []string{"CPU", "HEAP", "THREADS", "CONTENTION"},
			goVersion:        "1.8",
			mutexProfiling:   true,
		},
		{
			InstanceConfig: proftest.InstanceConfig{
				ProjectID:   projectID,
				Zone:        zone,
				Name:        fmt.Sprintf("profiler-test-go17-%d", runID),
				MachineType: "n1-standard-1",
			},
			name:             fmt.Sprintf("profiler-test-go17-%d-gce", runID),
			wantProfileTypes: []string{"CPU", "HEAP", "THREADS"},
			goVersion:        "1.7",
		},
		{
			InstanceConfig: proftest.InstanceConfig{
				ProjectID:   projectID,
				Zone:        zone,
				Name:        fmt.Sprintf("profiler-test-go16-%d", runID),
				MachineType: "n1-standard-1",
			},
			name:             fmt.Sprintf("profiler-test-go16-%d-gce", runID),
			wantProfileTypes: []string{"CPU", "HEAP", "THREADS"},
			goVersion:        "1.6",
		},
	}

	for _, tc := range testcases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.initializeStartupScript(template); err != nil {
				t.Fatalf("failed to initialize startup script")
			}

			if err := gceTr.StartInstance(ctx, &tc.InstanceConfig); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if gceTr.DeleteInstance(ctx, &tc.InstanceConfig); err != nil {
					t.Fatal(err)
				}
			}()

			timeoutCtx, cancel := context.WithTimeout(ctx, time.Minute*25)
			defer cancel()
			if err := gceTr.PollForSerialOutput(timeoutCtx, &tc.InstanceConfig, benchFinishString); err != nil {
				t.Fatal(err)
			}

			timeNow := time.Now()
			endTime := timeNow.Format(time.RFC3339)
			startTime := timeNow.Add(-1 * time.Hour).Format(time.RFC3339)
			for _, pType := range tc.wantProfileTypes {
				pr, err := tr.QueryProfiles(tc.ProjectID, tc.name, startTime, endTime, pType)
				if err != nil {
					t.Errorf("QueryProfiles(%s, %s, %s, %s, %s) got error: %v", tc.ProjectID, tc.name, startTime, endTime, pType, err)
					continue
				}
				if err := pr.HasFunction("busywork"); err != nil {
					t.Error(err)
				}
			}
		})
	}
}
