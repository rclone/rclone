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
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/internal/testutil"
	"cloud.google.com/go/profiler/mocks"
	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/google/pprof/profile"
	gax "github.com/googleapis/gax-go"
	"golang.org/x/net/context"
	gtransport "google.golang.org/api/transport/grpc"
	pb "google.golang.org/genproto/googleapis/devtools/cloudprofiler/v2"
	edpb "google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	grpcmd "google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	testProjectID       = "test-project-ID"
	testInstance        = "test-instance"
	testZone            = "test-zone"
	testTarget          = "test-target"
	testService         = "test-service"
	testSvcVersion      = "test-service-version"
	testProfileDuration = time.Second * 10
	testServerTimeout   = time.Second * 15
	wantFunctionName    = "profilee"
)

func createTestDeployment() *pb.Deployment {
	labels := map[string]string{
		zoneNameLabel: testZone,
		versionLabel:  testSvcVersion,
	}
	return &pb.Deployment{
		ProjectId: testProjectID,
		Target:    testService,
		Labels:    labels,
	}
}

func createTestAgent(psc pb.ProfilerServiceClient) *agent {
	return &agent{
		client:        psc,
		deployment:    createTestDeployment(),
		profileLabels: map[string]string{instanceLabel: testInstance},
	}
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

	if !testutil.Equal(gotP, p) {
		t.Errorf("CreateProfile() got wrong profile, got %v, want %v", gotP, p)
	}
}

func TestProfileAndUpload(t *testing.T) {
	oldStartCPUProfile, oldStopCPUProfile, oldWriteHeapProfile, oldSleep := startCPUProfile, stopCPUProfile, writeHeapProfile, sleep
	defer func() {
		startCPUProfile, stopCPUProfile, writeHeapProfile, sleep = oldStartCPUProfile, oldStopCPUProfile, oldWriteHeapProfile, oldSleep
	}()

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
				ProfileType:  p.ProfileType,
				Duration:     p.Duration,
				ProfileBytes: tt.wantBytes,
				Labels:       a.profileLabels,
			}
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

func TestWithXGoogHeader(t *testing.T) {
	ctx := withXGoogHeader(context.Background())
	md, _ := grpcmd.FromOutgoingContext(ctx)

	if xg := md[xGoogAPIMetadata]; len(xg) == 0 {
		t.Errorf("withXGoogHeader() sets empty xGoogHeader")
	} else {
		if !strings.Contains(xg[0], "gl-go/") {
			t.Errorf("withXGoogHeader() got: %v, want gl-go key", xg[0])
		}
		if !strings.Contains(xg[0], "gccl/") {
			t.Errorf("withXGoogHeader() got: %v, want gccl key", xg[0])
		}
		if !strings.Contains(xg[0], "gax/") {
			t.Errorf("withXGoogHeader() got: %v, want gax key", xg[0])
		}
		if !strings.Contains(xg[0], "grpc/") {
			t.Errorf("withXGoogHeader() got: %v, want grpc key", xg[0])
		}
	}
}

func TestInitializeAgent(t *testing.T) {
	oldConfig := config
	defer func() {
		config = oldConfig
	}()

	for _, tt := range []struct {
		config               Config
		wantDeploymentLabels map[string]string
		wantProfileLabels    map[string]string
	}{
		{
			config:               Config{ServiceVersion: testSvcVersion, zone: testZone},
			wantDeploymentLabels: map[string]string{zoneNameLabel: testZone, versionLabel: testSvcVersion},
			wantProfileLabels:    map[string]string{},
		},
		{
			config:               Config{zone: testZone},
			wantDeploymentLabels: map[string]string{zoneNameLabel: testZone},
			wantProfileLabels:    map[string]string{},
		},
		{
			config:               Config{ServiceVersion: testSvcVersion},
			wantDeploymentLabels: map[string]string{versionLabel: testSvcVersion},
			wantProfileLabels:    map[string]string{},
		},
		{
			config:               Config{instance: testInstance},
			wantDeploymentLabels: map[string]string{},
			wantProfileLabels:    map[string]string{instanceLabel: testInstance},
		},
	} {

		config = tt.config
		config.ProjectID = testProjectID
		config.Target = testTarget
		a := initializeAgent(nil)

		wantDeployment := &pb.Deployment{
			ProjectId: testProjectID,
			Target:    testTarget,
			Labels:    tt.wantDeploymentLabels,
		}
		if !testutil.Equal(a.deployment, wantDeployment) {
			t.Errorf("initializeResources() got deployment: %v, want %v", a.deployment, wantDeployment)
		}

		if !testutil.Equal(a.profileLabels, tt.wantProfileLabels) {
			t.Errorf("initializeResources() got profile labels: %v, want %v", a.profileLabels, tt.wantProfileLabels)
		}
	}
}

func TestInitializeConfig(t *testing.T) {
	oldConfig, oldService, oldVersion, oldGetProjectID, oldGetInstanceName, oldGetZone, oldOnGCE := config, os.Getenv("GAE_SERVICE"), os.Getenv("GAE_VERSION"), getProjectID, getInstanceName, getZone, onGCE
	defer func() {
		config, getProjectID, getInstanceName, getZone, onGCE = oldConfig, oldGetProjectID, oldGetInstanceName, oldGetZone, oldOnGCE
		if err := os.Setenv("GAE_SERVICE", oldService); err != nil {
			t.Fatal(err)
		}
		if err := os.Setenv("GAE_VERSION", oldVersion); err != nil {
			t.Fatal(err)
		}
	}()
	testGAEService := "test-gae-service"
	testGAEVersion := "test-gae-version"
	testGCEProjectID := "test-gce-project-id"
	for _, tt := range []struct {
		config          Config
		wantConfig      Config
		wantErrorString string
		onGAE           bool
		onGCE           bool
	}{
		{
			Config{Service: testService},
			Config{Target: testService, ProjectID: testGCEProjectID, zone: testZone, instance: testInstance},
			"",
			false,
			true,
		},
		{
			Config{Target: testTarget},
			Config{Target: testTarget, ProjectID: testGCEProjectID, zone: testZone, instance: testInstance},
			"",
			false,
			true,
		},
		{
			Config{},
			Config{},
			"service name must be specified in the configuration",
			false,
			true,
		},
		{
			Config{Service: testService},
			Config{Target: testService, ServiceVersion: testGAEVersion, ProjectID: testGCEProjectID, zone: testZone, instance: testInstance},
			"",
			true,
			true,
		},
		{
			Config{Target: testTarget},
			Config{Target: testTarget, ServiceVersion: testGAEVersion, ProjectID: testGCEProjectID, zone: testZone, instance: testInstance},
			"",
			true,
			true,
		},
		{
			Config{},
			Config{Target: testGAEService, ServiceVersion: testGAEVersion, ProjectID: testGCEProjectID, zone: testZone, instance: testInstance},
			"",
			true,
			true,
		},
		{
			Config{Service: testService, ServiceVersion: testSvcVersion},
			Config{Target: testService, ServiceVersion: testSvcVersion, ProjectID: testGCEProjectID, zone: testZone, instance: testInstance},
			"",
			false,
			true,
		},
		{
			Config{Service: testService, ServiceVersion: testSvcVersion},
			Config{Target: testService, ServiceVersion: testSvcVersion, ProjectID: testGCEProjectID, zone: testZone, instance: testInstance},
			"",
			true,
			true,
		},
		{
			Config{Service: testService, ProjectID: testProjectID},
			Config{Target: testService, ProjectID: testProjectID, zone: testZone, instance: testInstance},
			"",
			false,
			true,
		},
		{
			Config{Service: testService},
			Config{Target: testService},
			"project ID must be specified in the configuration if running outside of GCP",
			false,
			false,
		},
	} {
		envService, envVersion := "", ""
		if tt.onGAE {
			envService, envVersion = testGAEService, testGAEVersion
		}
		if err := os.Setenv("GAE_SERVICE", envService); err != nil {
			t.Fatal(err)
		}
		if err := os.Setenv("GAE_VERSION", envVersion); err != nil {
			t.Fatal(err)
		}
		if tt.onGCE {
			onGCE = func() bool { return true }
			getProjectID = func() (string, error) { return testGCEProjectID, nil }
			getZone = func() (string, error) { return testZone, nil }
			getInstanceName = func() (string, error) { return testInstance, nil }
		} else {
			onGCE = func() bool { return false }
			getProjectID = func() (string, error) { return "", fmt.Errorf("test get project id error") }
			getZone = func() (string, error) { return "", fmt.Errorf("test get zone error") }
			getInstanceName = func() (string, error) { return "", fmt.Errorf("test get instance error") }
		}

		errorString := ""
		if err := initializeConfig(tt.config); err != nil {
			errorString = err.Error()
		}

		if !strings.Contains(errorString, tt.wantErrorString) {
			t.Errorf("initializeConfig(%v) got error: %v, want contain %v", tt.config, errorString, tt.wantErrorString)
		}

		if tt.wantErrorString == "" {
			tt.wantConfig.APIAddr = apiAddress
		}
		tt.wantConfig.Service = tt.config.Service
		if config != tt.wantConfig {
			t.Errorf("initializeConfig(%v) got: %v, want %v", tt.config, config, tt.wantConfig)
		}
	}

	for _, tt := range []struct {
		wantErrorString   string
		getProjectIDError bool
		getZoneError      bool
		getInstanceError  bool
	}{
		{
			wantErrorString:   "failed to get the project ID from Compute Engine:",
			getProjectIDError: true,
		},
		{
			wantErrorString: "failed to get zone from Compute Engine:",
			getZoneError:    true,
		},
		{
			wantErrorString:  "failed to get instance from Compute Engine:",
			getInstanceError: true,
		},
	} {
		onGCE = func() bool { return true }
		if tt.getProjectIDError {
			getProjectID = func() (string, error) { return "", fmt.Errorf("test get project ID error") }
		} else {
			getProjectID = func() (string, error) { return testGCEProjectID, nil }
		}

		if tt.getZoneError {
			getZone = func() (string, error) { return "", fmt.Errorf("test get zone error") }
		} else {
			getZone = func() (string, error) { return testZone, nil }
		}

		if tt.getInstanceError {
			getInstanceName = func() (string, error) { return "", fmt.Errorf("test get instance error") }
		} else {
			getInstanceName = func() (string, error) { return testInstance, nil }
		}
		errorString := ""
		if err := initializeConfig(Config{Service: testService}); err != nil {
			errorString = err.Error()
		}

		if !strings.Contains(errorString, tt.wantErrorString) {
			t.Errorf("initializeConfig() got error: %v, want contain %v", errorString, tt.wantErrorString)
		}
	}
}

type fakeProfilerServer struct {
	pb.ProfilerServiceServer
	count          int
	gotCPUProfile  []byte
	gotHeapProfile []byte
	done           chan bool
}

func (fs *fakeProfilerServer) CreateProfile(ctx context.Context, in *pb.CreateProfileRequest) (*pb.Profile, error) {
	fs.count++
	switch fs.count {
	case 1:
		return &pb.Profile{Name: "testCPU", ProfileType: pb.ProfileType_CPU, Duration: ptypes.DurationProto(testProfileDuration)}, nil
	case 2:
		return &pb.Profile{Name: "testHeap", ProfileType: pb.ProfileType_HEAP}, nil
	default:
		select {}
	}
}

func (fs *fakeProfilerServer) UpdateProfile(ctx context.Context, in *pb.UpdateProfileRequest) (*pb.Profile, error) {
	switch in.Profile.ProfileType {
	case pb.ProfileType_CPU:
		fs.gotCPUProfile = in.Profile.ProfileBytes
	case pb.ProfileType_HEAP:
		fs.gotHeapProfile = in.Profile.ProfileBytes
		fs.done <- true
	}

	return in.Profile, nil
}

func profileeLoop(quit chan bool) {
	for {
		select {
		case <-quit:
			return
		default:
			profileeWork()
		}
	}
}

func profileeWork() {
	data := make([]byte, 1024*1024)
	rand.Read(data)

	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	if _, err := gz.Write(data); err != nil {
		log.Printf("failed to write to gzip stream", err)
		return
	}
	if err := gz.Flush(); err != nil {
		log.Printf("failed to flush to gzip stream", err)
		return
	}
	if err := gz.Close(); err != nil {
		log.Printf("failed to close gzip stream", err)
	}
}

func checkSymbolization(p *profile.Profile) error {
	for _, l := range p.Location {
		if len(l.Line) > 0 && l.Line[0].Function != nil && strings.Contains(l.Line[0].Function.Name, wantFunctionName) {
			return nil
		}
	}
	return fmt.Errorf("want function name %v not found in profile", wantFunctionName)
}

func validateProfile(rawData []byte) error {
	p, err := profile.ParseData(rawData)
	if err != nil {
		return fmt.Errorf("ParseData failed: %v", err)
	}

	if len(p.Sample) == 0 {
		return fmt.Errorf("profile contains zero samples: %v", p)
	}

	if len(p.Location) == 0 {
		return fmt.Errorf("profile contains zero locations: %v", p)
	}

	if len(p.Function) == 0 {
		return fmt.Errorf("profile contains zero functions: %v", p)
	}

	if err := checkSymbolization(p); err != nil {
		return fmt.Errorf("checkSymbolization failed: %v for %v", err, p)
	}
	return nil
}

func TestAgentWithServer(t *testing.T) {
	oldDialGRPC, oldConfig := dialGRPC, config
	defer func() {
		dialGRPC, config = oldDialGRPC, oldConfig
	}()

	srv, err := testutil.NewServer()
	if err != nil {
		t.Fatalf("testutil.NewServer(): %v", err)
	}
	fakeServer := &fakeProfilerServer{done: make(chan bool)}
	pb.RegisterProfilerServiceServer(srv.Gsrv, fakeServer)

	srv.Start()

	dialGRPC = gtransport.DialInsecure
	if err := Start(Config{
		Target:    testTarget,
		ProjectID: testProjectID,
		APIAddr:   srv.Addr,
		instance:  testInstance,
		zone:      testZone,
	}); err != nil {
		t.Fatalf("Start(): %v", err)
	}

	quitProfilee := make(chan bool)
	go profileeLoop(quitProfilee)

	select {
	case <-fakeServer.done:
	case <-time.After(testServerTimeout):
		t.Errorf("got timeout after %v, want fake server done", testServerTimeout)
	}
	quitProfilee <- true

	if err := validateProfile(fakeServer.gotCPUProfile); err != nil {
		t.Errorf("validateProfile(gotCPUProfile): %v", err)
	}
	if err := validateProfile(fakeServer.gotHeapProfile); err != nil {
		t.Errorf("validateProfile(gotHeapProfile): %v", err)
	}
}
