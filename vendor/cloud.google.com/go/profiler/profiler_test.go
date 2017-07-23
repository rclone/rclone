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

package profiler

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/profiler/mocks"
	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	gax "github.com/googleapis/gax-go"
	"golang.org/x/net/context"
	pb "google.golang.org/genproto/googleapis/devtools/cloudprofiler/v2"
	edpb "google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	grpcmd "google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	testProjectID    = "test-project-ID"
	testInstanceName = "test-instance-name"
	testZoneName     = "test-zone-name"
	testTarget       = "test-target"
)

func createTestDeployment() *pb.Deployment {
	labels := make(map[string]string)
	labels[zoneNameLabel] = testZoneName
	labels[instanceLabel] = testInstanceName
	return &pb.Deployment{
		ProjectId: testProjectID,
		Target:    testTarget,
		Labels:    labels,
	}
}

func createTestAgent(psc pb.ProfilerServiceClient) *agent {
	c := &client{client: psc}
	a := &agent{
		client:     c,
		deployment: createTestDeployment(),
	}
	return a
}

func createTrailers(dur time.Duration) map[string]string {
	b, _ := proto.Marshal(&edpb.RetryInfo{
		RetryDelay: ptypes.DurationProto(dur),
	})
	return map[string]string{
		retryInfoMetadata: string(b),
	}
}

func TestCreateProfile(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mpc := mocks.NewMockProfilerServiceClient(ctrl)
	a := createTestAgent(mpc)
	p := &pb.Profile{Name: "test_profile"}
	wantRequest := pb.CreateProfileRequest{
		Deployment:  a.deployment,
		ProfileType: []pb.ProfileType{pb.ProfileType_CPU, pb.ProfileType_HEAP},
	}

	mpc.EXPECT().CreateProfile(ctx, gomock.Eq(&wantRequest), gomock.Any()).Times(1).Return(p, nil)

	gotP := a.createProfile(ctx)

	if !reflect.DeepEqual(gotP, p) {
		t.Errorf("CreateProfile() got wrong profile, got %v, want %v", gotP, p)
	}
}

func TestProfileAndUpload(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	errFunc := func(io.Writer) error { return errors.New("") }
	testDuration := time.Second * 5
	tests := []struct {
		profileType          pb.ProfileType
		duration             *time.Duration
		startCPUProfileFunc  func(io.Writer) error
		writeHeapProfileFunc func(io.Writer) error
		wantBytes            []byte
	}{
		{
			profileType: pb.ProfileType_CPU,
			duration:    &testDuration,
			startCPUProfileFunc: func(w io.Writer) error {
				w.Write([]byte{1})
				return nil
			},
			writeHeapProfileFunc: errFunc,
			wantBytes:            []byte{1},
		},
		{
			profileType:          pb.ProfileType_CPU,
			startCPUProfileFunc:  errFunc,
			writeHeapProfileFunc: errFunc,
		},
		{
			profileType: pb.ProfileType_CPU,
			duration:    &testDuration,
			startCPUProfileFunc: func(w io.Writer) error {
				w.Write([]byte{2})
				return nil
			},
			writeHeapProfileFunc: func(w io.Writer) error {
				w.Write([]byte{3})
				return nil
			},
			wantBytes: []byte{2},
		},
		{
			profileType:         pb.ProfileType_HEAP,
			startCPUProfileFunc: errFunc,
			writeHeapProfileFunc: func(w io.Writer) error {
				w.Write([]byte{4})
				return nil
			},
			wantBytes: []byte{4},
		},
		{
			profileType:          pb.ProfileType_HEAP,
			startCPUProfileFunc:  errFunc,
			writeHeapProfileFunc: errFunc,
		},
		{
			profileType: pb.ProfileType_HEAP,
			startCPUProfileFunc: func(w io.Writer) error {
				w.Write([]byte{5})
				return nil
			},
			writeHeapProfileFunc: func(w io.Writer) error {
				w.Write([]byte{6})
				return nil
			},
			wantBytes: []byte{6},
		},
		{
			profileType: pb.ProfileType_PROFILE_TYPE_UNSPECIFIED,
			startCPUProfileFunc: func(w io.Writer) error {
				w.Write([]byte{7})
				return nil
			},
			writeHeapProfileFunc: func(w io.Writer) error {
				w.Write([]byte{8})
				return nil
			},
		},
	}

	for _, tt := range tests {
		mpc := mocks.NewMockProfilerServiceClient(ctrl)
		a := createTestAgent(mpc)
		startCPUProfile = tt.startCPUProfileFunc
		stopCPUProfile = func() {}
		writeHeapProfile = tt.writeHeapProfileFunc
		var gotSleep *time.Duration
		sleep = func(ctx context.Context, d time.Duration) error {
			gotSleep = &d
			return nil
		}
		p := &pb.Profile{ProfileType: tt.profileType}
		if tt.duration != nil {
			p.Duration = ptypes.DurationProto(*tt.duration)
		}

		if tt.wantBytes != nil {
			wantProfile := &pb.Profile{
				ProfileType: p.ProfileType,
				Duration:    p.Duration,
			}
			wantProfile.Labels = a.deployment.Labels
			wantProfile.ProfileBytes = tt.wantBytes
			wantRequest := pb.UpdateProfileRequest{
				Profile: wantProfile,
			}
			mpc.EXPECT().UpdateProfile(ctx, gomock.Eq(&wantRequest)).Times(1)
		} else {
			mpc.EXPECT().UpdateProfile(gomock.Any(), gomock.Any()).MaxTimes(0)
		}

		a.profileAndUpload(ctx, p)

		if tt.duration == nil {
			if gotSleep != nil {
				t.Errorf("profileAndUpload(%v) slept for: %v, want no sleep", p, gotSleep)
			}
		} else {
			if gotSleep == nil {
				t.Errorf("profileAndUpload(%v) didn't sleep, want sleep for: %v", p, tt.duration)
			} else if *gotSleep != *tt.duration {
				t.Errorf("profileAndUpload(%v) slept for wrong duration, got: %v, want: %v", p, gotSleep, tt.duration)
			}
		}
	}
}

func TestRetry(t *testing.T) {
	normalDuration := time.Second * 3
	negativeDuration := time.Second * -3

	tests := []struct {
		trailers  map[string]string
		wantPause *time.Duration
	}{
		{
			createTrailers(normalDuration),
			&normalDuration,
		},
		{
			createTrailers(negativeDuration),
			nil,
		},
		{
			map[string]string{retryInfoMetadata: "wrong format"},
			nil,
		},
		{
			map[string]string{},
			nil,
		},
	}

	for _, tt := range tests {
		md := grpcmd.New(tt.trailers)
		r := &retryer{
			backoff: gax.Backoff{
				Initial:    initialBackoff,
				Max:        maxBackoff,
				Multiplier: backoffMultiplier,
			},
			md: md,
		}

		pause, shouldRetry := r.Retry(status.Error(codes.Aborted, ""))

		if !shouldRetry {
			t.Error("retryer.Retry() returned shouldRetry false, want true")
		}

		if tt.wantPause != nil {
			if pause != *tt.wantPause {
				t.Errorf("retryer.Retry() returned wrong pause, got: %v, want: %v", pause, tt.wantPause)
			}
		} else {
			if pause > initialBackoff {
				t.Errorf("retryer.Retry() returned wrong pause, got: %v, want: < %v", pause, initialBackoff)
			}
		}
	}

	md := grpcmd.New(map[string]string{})

	r := &retryer{
		backoff: gax.Backoff{
			Initial:    initialBackoff,
			Max:        maxBackoff,
			Multiplier: backoffMultiplier,
		},
		md: md,
	}
	for i := 0; i < 100; i++ {
		pause, shouldRetry := r.Retry(errors.New(""))
		if !shouldRetry {
			t.Errorf("retryer.Retry() called %v times, returned shouldRetry false, want true", i)
		}
		if pause > maxBackoff {
			t.Errorf("retryer.Retry() called %v times, returned wrong pause, got: %v, want: < %v", i, pause, maxBackoff)
		}
	}
}

func TestInitializeResources(t *testing.T) {
	d := createTestDeployment()
	ctx := context.Background()

	a, ctx := initializeResources(ctx, nil, d)

	if xg := a.client.xGoogHeader; len(xg) == 0 {
		t.Errorf("initializeResources() sets empty xGoogHeader")
	} else {
		if !strings.Contains(xg[0], "gl-go/") {
			t.Errorf("initializeResources() sets wrong xGoogHeader, got: %v, want gl-go key", xg[0])
		}
		if !strings.Contains(xg[0], "gccl/") {
			t.Errorf("initializeResources() sets wrong xGoogHeader, got: %v, want gccl key", xg[0])
		}
		if !strings.Contains(xg[0], "gax/") {
			t.Errorf("initializeResources() sets wrong xGoogHeader, got: %v, want gax key", xg[0])
		}
		if !strings.Contains(xg[0], "grpc/") {
			t.Errorf("initializeResources() sets wrong xGoogHeader, got: %v, want grpc key", xg[0])
		}
	}

	wantPH := "test-project-ID##test-target##instance|test-instance-name#zone|test-zone-name"
	if ph := a.client.profilerHeader; len(ph) == 0 {
		t.Errorf("initializeResources() sets empty profilerHeader")
	} else if ph[0] != wantPH {
		t.Errorf("initializeResources() sets wrong profilerHeader, got: %v, want: %v", ph[0], wantPH)
	}

	md, _ := grpcmd.FromOutgoingContext(ctx)

	if !reflect.DeepEqual(md[xGoogAPIMetadata], a.client.xGoogHeader) {
		t.Errorf("md[%v] = %v, want equal xGoogHeader = %v", xGoogAPIMetadata, md[xGoogAPIMetadata], a.client.xGoogHeader)
	}
	if !reflect.DeepEqual(md[deploymentKeyMetadata], a.client.profilerHeader) {
		t.Errorf("md[%v] = %v, want equal profilerHeader = %v", deploymentKeyMetadata, md[deploymentKeyMetadata], a.client.profilerHeader)
	}
}

func TestInitializeDeployment(t *testing.T) {
	getProjectID = func() (string, error) {
		return testProjectID, nil
	}
	getInstanceName = func() (string, error) {
		return testInstanceName, nil
	}
	getZone = func() (string, error) {
		return testZoneName, nil
	}

	config = &Config{Target: testTarget}
	d, err := initializeDeployment()

	if err != nil {
		t.Errorf("initializeDeployment() got error: %v, want no error", err)
	}

	want := createTestDeployment()

	if !reflect.DeepEqual(d, want) {
		t.Errorf("initializeDeployment() got wrong deployment, got: %v, want %v", d, want)
	}
}
