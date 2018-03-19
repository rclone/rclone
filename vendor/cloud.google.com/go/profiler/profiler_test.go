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
	"runtime"
	"strings"
	"sync"
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
		profileTypes:  []pb.ProfileType{pb.ProfileType_CPU, pb.ProfileType_HEAP, pb.ProfileType_THREADS},
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
		ProfileType: a.profileTypes,
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
	oldConfig, oldMutexEnabled := config, mutexEnabled
	defer func() {
		config, mutexEnabled = oldConfig, oldMutexEnabled
	}()

	for _, tt := range []struct {
		config               Config
		enableMutex          bool
		wantProfileTypes     []pb.ProfileType
		wantDeploymentLabels map[string]string
		wantProfileLabels    map[string]string
	}{
		{
			config:               Config{ServiceVersion: testSvcVersion, zone: testZone},
			wantProfileTypes:     []pb.ProfileType{pb.ProfileType_CPU, pb.ProfileType_HEAP, pb.ProfileType_THREADS},
			wantDeploymentLabels: map[string]string{zoneNameLabel: testZone, versionLabel: testSvcVersion},
			wantProfileLabels:    map[string]string{},
		},
		{
			config:               Config{zone: testZone},
			wantProfileTypes:     []pb.ProfileType{pb.ProfileType_CPU, pb.ProfileType_HEAP, pb.ProfileType_THREADS},
			wantDeploymentLabels: map[string]string{zoneNameLabel: testZone},
			wantProfileLabels:    map[string]string{},
		},
		{
			config:               Config{ServiceVersion: testSvcVersion},
			wantProfileTypes:     []pb.ProfileType{pb.ProfileType_CPU, pb.ProfileType_HEAP, pb.ProfileType_THREADS},
			wantDeploymentLabels: map[string]string{versionLabel: testSvcVersion},
			wantProfileLabels:    map[string]string{},
		},
		{
			config:               Config{instance: testInstance},
			wantProfileTypes:     []pb.ProfileType{pb.ProfileType_CPU, pb.ProfileType_HEAP, pb.ProfileType_THREADS},
			wantDeploymentLabels: map[string]string{},
			wantProfileLabels:    map[string]string{instanceLabel: testInstance},
		},
		{
			config:               Config{instance: testInstance},
			enableMutex:          true,
			wantProfileTypes:     []pb.ProfileType{pb.ProfileType_CPU, pb.ProfileType_HEAP, pb.ProfileType_THREADS, pb.ProfileType_CONTENTION},
			wantDeploymentLabels: map[string]string{},
			wantProfileLabels:    map[string]string{instanceLabel: testInstance},
		},
		{
			config:               Config{NoHeapProfiling: true},
			wantProfileTypes:     []pb.ProfileType{pb.ProfileType_CPU, pb.ProfileType_THREADS},
			wantDeploymentLabels: map[string]string{},
			wantProfileLabels:    map[string]string{},
		},
		{
			config:               Config{NoHeapProfiling: true, NoGoroutineProfiling: true},
			wantProfileTypes:     []pb.ProfileType{pb.ProfileType_CPU},
			wantDeploymentLabels: map[string]string{},
			wantProfileLabels:    map[string]string{},
		},
	} {

		config = tt.config
		config.ProjectID = testProjectID
		config.Target = testTarget
		mutexEnabled = tt.enableMutex
		a := initializeAgent(nil)

		wantDeployment := &pb.Deployment{
			ProjectId: testProjectID,
			Target:    testTarget,
			Labels:    tt.wantDeploymentLabels,
		}
		if !testutil.Equal(a.deployment, wantDeployment) {
			t.Errorf("initializeAgent() got deployment: %v, want %v", a.deployment, wantDeployment)
		}
		if !testutil.Equal(a.profileLabels, tt.wantProfileLabels) {
			t.Errorf("initializeAgent() got profile labels: %v, want %v", a.profileLabels, tt.wantProfileLabels)
		}
		if !testutil.Equal(a.profileTypes, tt.wantProfileTypes) {
			t.Errorf("initializeAgent() got profile types: %v, want %v", a.profileTypes, tt.wantProfileTypes)
		}
	}
}

func TestInitializeConfig(t *testing.T) {
	oldConfig, oldService, oldVersion, oldEnvProjectID, oldGetProjectID, oldGetInstanceName, oldGetZone, oldOnGCE := config, os.Getenv("GAE_SERVICE"), os.Getenv("GAE_VERSION"), os.Getenv("GOOGLE_CLOUD_PROJECT"), getProjectID, getInstanceName, getZone, onGCE
	defer func() {
		config, getProjectID, getInstanceName, getZone, onGCE = oldConfig, oldGetProjectID, oldGetInstanceName, oldGetZone, oldOnGCE
		if err := os.Setenv("GAE_SERVICE", oldService); err != nil {
			t.Fatal(err)
		}
		if err := os.Setenv("GAE_VERSION", oldVersion); err != nil {
			t.Fatal(err)
		}
		if err := os.Setenv("GOOGLE_CLOUD_PROJECT", oldEnvProjectID); err != nil {
			t.Fatal(err)
		}
	}()
	const (
		testGAEService   = "test-gae-service"
		testGAEVersion   = "test-gae-version"
		testGCEProjectID = "test-gce-project-id"
		testEnvProjectID = "test-env-project-id"
	)
	for _, tt := range []struct {
		desc            string
		config          Config
		wantConfig      Config
		wantErrorString string
		onGAE           bool
		onGCE           bool
		envProjectID    bool
	}{
		{
			"accepts service name",
			Config{Service: testService},
			Config{Target: testService, ProjectID: testGCEProjectID, zone: testZone, instance: testInstance},
			"",
			false,
			true,
			false,
		},
		{
			"accepts target name",
			Config{Target: testTarget},
			Config{Target: testTarget, ProjectID: testGCEProjectID, zone: testZone, instance: testInstance},
			"",
			false,
			true,
			false,
		},
		{
			"env project overrides GCE project",
			Config{Service: testService},
			Config{Target: testService, ProjectID: testEnvProjectID, zone: testZone, instance: testInstance},
			"",
			false,
			true,
			true,
		},
		{
			"requires service name",
			Config{},
			Config{},
			"service name must be specified in the configuration",
			false,
			true,
			false,
		},
		{
			"accepts service name from config and service version from GAE",
			Config{Service: testService},
			Config{Target: testService, ServiceVersion: testGAEVersion, ProjectID: testGCEProjectID, zone: testZone, instance: testInstance},
			"",
			true,
			true,
			false,
		},
		{
			"accepts target name from config and service version from GAE",
			Config{Target: testTarget},
			Config{Target: testTarget, ServiceVersion: testGAEVersion, ProjectID: testGCEProjectID, zone: testZone, instance: testInstance},
			"",
			true,
			true,
			false,
		},
		{
			"reads both service name and version from GAE env vars",
			Config{},
			Config{Target: testGAEService, ServiceVersion: testGAEVersion, ProjectID: testGCEProjectID, zone: testZone, instance: testInstance},
			"",
			true,
			true,
			false,
		},
		{
			"accepts service version from config",
			Config{Service: testService, ServiceVersion: testSvcVersion},
			Config{Target: testService, ServiceVersion: testSvcVersion, ProjectID: testGCEProjectID, zone: testZone, instance: testInstance},
			"",
			false,
			true,
			false,
		},
		{
			"configured version has priority over GAE-provided version",
			Config{Service: testService, ServiceVersion: testSvcVersion},
			Config{Target: testService, ServiceVersion: testSvcVersion, ProjectID: testGCEProjectID, zone: testZone, instance: testInstance},
			"",
			true,
			true,
			false,
		},
		{
			"configured project ID has priority over metadata-provided project ID",
			Config{Service: testService, ProjectID: testProjectID},
			Config{Target: testService, ProjectID: testProjectID, zone: testZone, instance: testInstance},
			"",
			false,
			true,
			false,
		},
		{
			"configured project ID has priority over environment project ID",
			Config{Service: testService, ProjectID: testProjectID},
			Config{Target: testService, ProjectID: testProjectID},
			"",
			false,
			false,
			true,
		},
		{
			"requires project ID if not on GCE",
			Config{Service: testService},
			Config{Target: testService},
			"project ID must be specified in the configuration if running outside of GCP",
			false,
			false,
			false,
		},
	} {
		t.Logf("Running test: %s", tt.desc)
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
		envProjectID := ""
		if tt.envProjectID {
			envProjectID = testEnvProjectID
		}
		if err := os.Setenv("GOOGLE_CLOUD_PROJECT", envProjectID); err != nil {
			t.Fatal(err)
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
	count       int
	gotProfiles map[string][]byte
	done        chan bool
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
		fs.gotProfiles["CPU"] = in.Profile.ProfileBytes
	case pb.ProfileType_HEAP:
		fs.gotProfiles["HEAP"] = in.Profile.ProfileBytes
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
		log.Println("failed to write to gzip stream", err)
		return
	}
	if err := gz.Flush(); err != nil {
		log.Println("failed to flush to gzip stream", err)
		return
	}
	if err := gz.Close(); err != nil {
		log.Println("failed to close gzip stream", err)
	}
}

func validateProfile(rawData []byte, wantFunctionName string) error {
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

	for _, l := range p.Location {
		if len(l.Line) > 0 && l.Line[0].Function != nil && strings.Contains(l.Line[0].Function.Name, wantFunctionName) {
			return nil
		}
	}
	return fmt.Errorf("wanted function name %s not found in the profile", wantFunctionName)
}

func TestDeltaMutexProfile(t *testing.T) {
	oldMutexEnabled, oldMaxProcs := mutexEnabled, runtime.GOMAXPROCS(10)
	defer func() {
		mutexEnabled = oldMutexEnabled
		runtime.GOMAXPROCS(oldMaxProcs)
	}()
	if mutexEnabled = enableMutexProfiling(); !mutexEnabled {
		t.Skip("Go too old - mutex profiling not supported.")
	}

	hog(time.Second, mutexHog)
	go func() {
		hog(2*time.Second, backgroundHog)
	}()

	var prof bytes.Buffer
	if err := deltaMutexProfile(context.Background(), time.Second, &prof); err != nil {
		t.Fatalf("deltaMutexProfile() got error: %v", err)
	}
	p, err := profile.Parse(&prof)
	if err != nil {
		t.Fatalf("profile.Parse() got error: %v", err)
	}

	if s := sum(p, "mutexHog"); s != 0 {
		t.Errorf("mutexHog found in the delta mutex profile (sum=%d):\n%s", s, p)
	}
	if s := sum(p, "backgroundHog"); s <= 0 {
		t.Errorf("backgroundHog not in the delta mutex profile (sum=%d):\n%s", s, p)
	}
}

// sum returns the sum of all mutex counts from the samples whose
// stacks include the specified function name.
func sum(p *profile.Profile, fname string) int64 {
	locIDs := map[*profile.Location]bool{}
	for _, loc := range p.Location {
		for _, l := range loc.Line {
			if strings.Contains(l.Function.Name, fname) {
				locIDs[loc] = true
				break
			}
		}
	}
	var s int64
	for _, sample := range p.Sample {
		for _, loc := range sample.Location {
			if locIDs[loc] {
				s += sample.Value[0]
				break
			}
		}
	}
	return s
}

func mutexHog(mu1, mu2 *sync.Mutex, start time.Time, dt time.Duration) {
	for time.Since(start) < dt {
		mu1.Lock()
		runtime.Gosched()
		mu2.Lock()
		mu1.Unlock()
		mu2.Unlock()
	}
}

// backgroundHog is identical to mutexHog. We keep them separate
// in order to distinguish them with function names in the stack trace.
func backgroundHog(mu1, mu2 *sync.Mutex, start time.Time, dt time.Duration) {
	for time.Since(start) < dt {
		mu1.Lock()
		runtime.Gosched()
		mu2.Lock()
		mu1.Unlock()
		mu2.Unlock()
	}
}

func hog(dt time.Duration, hogger func(mu1, mu2 *sync.Mutex, start time.Time, dt time.Duration)) {
	start := time.Now()
	mu1 := new(sync.Mutex)
	mu2 := new(sync.Mutex)
	var wg sync.WaitGroup
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			hogger(mu1, mu2, start, dt)
		}()
	}
	wg.Wait()
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
	fakeServer := &fakeProfilerServer{gotProfiles: map[string][]byte{}, done: make(chan bool)}
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

	for _, pType := range []string{"CPU", "HEAP"} {
		if profile, ok := fakeServer.gotProfiles[pType]; !ok {
			t.Errorf("fakeServer.gotProfiles[%s] got no profile, want profile", pType)
		} else if err := validateProfile(profile, "profilee"); err != nil {
			t.Errorf("validateProfile(%s) got error: %v", pType, err)
		}
	}
}
