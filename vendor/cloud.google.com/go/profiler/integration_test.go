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
	"archive/zip"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
	"text/template"
	"time"

	"cloud.google.com/go/storage"
	"golang.org/x/build/kubernetes"
	k8sapi "golang.org/x/build/kubernetes/api"
	"golang.org/x/build/kubernetes/gke"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	cloudbuild "google.golang.org/api/cloudbuild/v1"
	compute "google.golang.org/api/compute/v1"
	container "google.golang.org/api/container/v1"
	"google.golang.org/api/googleapi"
)

var (
	commit = flag.String("commit", "", "git commit to test")
	runID  = time.Now().Unix()
)

const (
	cloudScope        = "https://www.googleapis.com/auth/cloud-platform"
	monitorWriteScope = "https://www.googleapis.com/auth/monitoring.write"
	storageReadScope  = "https://www.googleapis.com/auth/devstorage.read_only"
	// benchFinishString should keep in sync with the finish string in busybench.
	benchFinishString = "busybench finished profiling"
)

const startupTemplate = `
#! /bin/bash

# Fail on any error.
set -eo pipefail

# Display commands being run.
set -x

# Install git
sudo apt-get update
sudo apt-get -y -q install git-all

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
git clone https://code.googlesource.com/gocloud $GOCLOUD_HOME

cd $GOCLOUD_HOME/profiler/busybench
git reset --hard {{.Commit}}
go get -v

# Run benchmark with agent
go run busybench.go --service="{{.Service}}" --mutex_profiling="{{.MutexProfiling}}"
`

const dockerfileFmt = `FROM golang
RUN git clone https://code.googlesource.com/gocloud /go/src/cloud.google.com/go \
    && cd /go/src/cloud.google.com/go/profiler/busybench && git reset --hard %s \
    && go get -v && go install -v
CMD ["busybench", "--service", "%s"]
 `

type testRunner struct {
	client           *http.Client
	startupTemplate  *template.Template
	containerService *container.Service
	computeService   *compute.Service
	storageClient    *storage.Client
}

type profileResponse struct {
	Profile     profileData   `json:"profile"`
	NumProfiles int32         `json:"numProfiles"`
	Deployments []interface{} `json:"deployments"`
}

type profileData struct {
	Samples           []int32       `json:"samples"`
	SampleMetrics     interface{}   `json:"sampleMetrics"`
	DefaultMetricType string        `json:"defaultMetricType"`
	TreeNodes         interface{}   `json:"treeNodes"`
	Functions         functionArray `json:"functions"`
	SourceFiles       interface{}   `json:"sourceFiles"`
}

type functionArray struct {
	Name       []string `json:"name"`
	Sourcefile []int32  `json:"sourceFile"`
}

func validateProfileData(rawData []byte, wantFunctionName string) error {
	var pr profileResponse
	if err := json.Unmarshal(rawData, &pr); err != nil {
		return err
	}

	if pr.NumProfiles == 0 {
		return fmt.Errorf("profile response contains zero profiles: %v", pr)
	}

	if len(pr.Deployments) == 0 {
		return fmt.Errorf("profile response contains zero deployments: %v", pr)
	}

	if len(pr.Profile.Functions.Name) == 0 {
		return fmt.Errorf("profile does not have function data")
	}

	for _, name := range pr.Profile.Functions.Name {
		if strings.Contains(name, wantFunctionName) {
			return nil
		}
	}
	return fmt.Errorf("wanted function name %s not found in profile", wantFunctionName)
}

type instanceConfig struct {
	name           string
	service        string
	goVersion      string
	mutexProfiling bool
}

func newInstanceConfigs() []instanceConfig {
	return []instanceConfig{
		{
			name:           fmt.Sprintf("profiler-test-go19-%d", runID),
			service:        fmt.Sprintf("profiler-test-go19-%d-gce", runID),
			goVersion:      "1.9",
			mutexProfiling: true,
		},
		{
			name:           fmt.Sprintf("profiler-test-go18-%d", runID),
			service:        fmt.Sprintf("profiler-test-go18-%d-gce", runID),
			goVersion:      "1.8",
			mutexProfiling: true,
		},
		{
			name:      fmt.Sprintf("profiler-test-go17-%d", runID),
			service:   fmt.Sprintf("profiler-test-go17-%d-gce", runID),
			goVersion: "1.7",
		},
		{
			name:      fmt.Sprintf("profiler-test-go16-%d", runID),
			service:   fmt.Sprintf("profiler-test-go16-%d-gce", runID),
			goVersion: "1.6",
		},
	}
}

type clusterConfig struct {
	clusterName     string
	podName         string
	imageSourceName string
	imageName       string
	service         string
}

func newClusterConfig(projectID string) clusterConfig {
	return clusterConfig{
		clusterName:     fmt.Sprintf("profiler-test-cluster-%d", runID),
		podName:         fmt.Sprintf("profiler-test-pod-%d", runID),
		imageSourceName: fmt.Sprintf("profiler-test/%d/Dockerfile.zip", runID),
		imageName:       fmt.Sprintf("%s/profiler-test-%d", projectID, runID),
		service:         fmt.Sprintf("profiler-test-%d-gke", runID),
	}
}

func renderStartupScript(template *template.Template, inst instanceConfig) (string, error) {
	var buf bytes.Buffer
	err := template.Execute(&buf,
		struct {
			Service        string
			GoVersion      string
			Commit         string
			MutexProfiling bool
		}{
			Service:        inst.service,
			GoVersion:      inst.goVersion,
			Commit:         *commit,
			MutexProfiling: inst.mutexProfiling,
		})
	if err != nil {
		return "", fmt.Errorf("failed to render startup script for %s: %v", inst.name, err)
	}

	return buf.String(), nil
}

func (tr *testRunner) startInstance(ctx context.Context, inst instanceConfig, projectID, zone string) error {
	img, err := tr.computeService.Images.GetFromFamily("debian-cloud", "debian-9").Context(ctx).Do()
	if err != nil {
		return err
	}

	startupScript, err := renderStartupScript(tr.startupTemplate, inst)
	if err != nil {
		return err
	}

	_, err = tr.computeService.Instances.Insert(projectID, zone, &compute.Instance{
		MachineType: fmt.Sprintf("zones/%s/machineTypes/n1-standard-1", zone),
		Name:        inst.name,
		Disks: []*compute.AttachedDisk{{
			AutoDelete: true, // delete the disk when the VM is deleted.
			Boot:       true,
			Type:       "PERSISTENT",
			Mode:       "READ_WRITE",
			InitializeParams: &compute.AttachedDiskInitializeParams{
				SourceImage: img.SelfLink,
				DiskType:    fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/zones/%s/diskTypes/pd-standard", projectID, zone),
			},
		}},
		NetworkInterfaces: []*compute.NetworkInterface{{
			Network: fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/global/networks/default", projectID),
			AccessConfigs: []*compute.AccessConfig{{
				Name: "External NAT",
			}},
		}},
		Metadata: &compute.Metadata{
			Items: []*compute.MetadataItems{{
				Key:   "startup-script",
				Value: googleapi.String(startupScript),
			}},
		},
		ServiceAccounts: []*compute.ServiceAccount{{
			Email: "default",
			Scopes: []string{
				monitorWriteScope,
			},
		}},
	}).Do()

	return err
}

func (tr *testRunner) pollForSerialOutput(ctx context.Context, projectID, zone, instanceName string) error {
	var output string
	defer func() {
		log.Printf("Serial port output for %s:\n%s", instanceName, output)
	}()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for profiling finishing on instance %s", instanceName)

		case <-time.After(20 * time.Second):
			resp, err := tr.computeService.Instances.GetSerialPortOutput(projectID, zone, instanceName).Context(ctx).Do()
			if err != nil {
				// Transient failure.
				log.Printf("Transient error getting serial port output from instance %s (will retry): %v", instanceName, err)
				continue
			}

			if output = resp.Contents; strings.Contains(output, benchFinishString) {
				return nil
			}
		}
	}
}

func (tr *testRunner) queryAndCheckProfile(service, startTime, endTime, profileType, projectID string) error {
	queryURL := fmt.Sprintf("https://cloudprofiler.googleapis.com/v2/projects/%s/profiles:query", projectID)
	const queryJsonFmt = `{"endTime": "%s", "profileType": "%s","startTime": "%s", "target": "%s"}`

	queryRequest := fmt.Sprintf(queryJsonFmt, endTime, profileType, startTime, service)

	resp, err := tr.client.Post(queryURL, "application/json", strings.NewReader(queryRequest))
	if err != nil {
		return fmt.Errorf("failed to query API: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	if err := validateProfileData(body, "busywork"); err != nil {
		return fmt.Errorf("failed to validate profile %v", err)
	}

	return nil
}

func (tr *testRunner) runTestOnGCE(ctx context.Context, t *testing.T, inst instanceConfig, projectID, zone string) {
	if err := tr.startInstance(ctx, inst, projectID, zone); err != nil {
		t.Fatalf("startInstance(%s) got error: %v", inst.name, err)
	}
	defer func() {
		if _, err := tr.computeService.Instances.Delete(projectID, zone, inst.name).Context(ctx).Do(); err != nil {
			t.Errorf("Instances.Delete(%s) got error: %v", inst.name, err)
		}
	}()

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Minute*25)
	defer cancel()
	if err := tr.pollForSerialOutput(timeoutCtx, projectID, zone, inst.name); err != nil {
		t.Fatalf("pollForSerialOutput(%s) got error: %v", inst.name, err)
	}

	timeNow := time.Now()
	endTime := timeNow.Format(time.RFC3339)
	startTime := timeNow.Add(-1 * time.Hour).Format(time.RFC3339)

	profileTypes := []string{"CPU", "HEAP", "THREADS"}
	if inst.mutexProfiling {
		profileTypes = append(profileTypes, "CONTENTION")
	}
	for _, pType := range profileTypes {
		if err := tr.queryAndCheckProfile(inst.service, startTime, endTime, pType, projectID); err != nil {
			t.Errorf("queryAndCheckProfile(%s, %s, %s, %s) got error: %v", inst.service, startTime, endTime, pType, err)
		}
	}
}

// createAndPublishDockerImage creates a docker image from source code in a GCS
// bucket and pushes the image to Google Container Registry.
func (tr *testRunner) createAndPublishDockerImage(ctx context.Context, projectID, sourceBucket, sourceObject, imageName string) error {
	cloudbuildService, err := cloudbuild.New(tr.client)

	build := &cloudbuild.Build{
		Source: &cloudbuild.Source{
			StorageSource: &cloudbuild.StorageSource{
				Bucket: sourceBucket,
				Object: sourceObject,
			},
		},
		Steps: []*cloudbuild.BuildStep{
			{
				Name: "gcr.io/cloud-builders/docker",
				Args: []string{"build", "-t", imageName, "."},
			},
		},
		Images: []string{imageName},
	}

	op, err := cloudbuildService.Projects.Builds.Create(projectID, build).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to create image: %v", err)
	}
	opID := op.Name

	// Wait for creating image.
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting creating image")

		case <-time.After(10 * time.Second):
			op, err := cloudbuildService.Operations.Get(opID).Context(ctx).Do()
			if err != nil {
				log.Printf("Transient error getting operation (will retry): %v", err)
				break
			}
			if op.Done == true {
				log.Printf("Published image %s to Google Container Registry.", imageName)
				return nil
			}
		}
	}
}

type imageResponse struct {
	Manifest map[string]interface{} `json:"manifest"`
	Name     string                 `json:"name"`
	Tags     []string               `json:"tags"`
}

// deleteDockerImage deletes a docker image from Google Container Registry.
func (tr *testRunner) deleteDockerImage(ctx context.Context, imageName string) []error {
	queryImageURL := fmt.Sprintf("https://gcr.io/v2/%s/tags/list", imageName)
	resp, err := tr.client.Get(queryImageURL)
	if err != nil {
		return []error{fmt.Errorf("failed to list tags: %v", err)}
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []error{err}
	}
	var ir imageResponse
	if err := json.Unmarshal(body, &ir); err != nil {
		return []error{err}
	}

	const deleteImageURLFmt = "https://gcr.io/v2/%s/manifests/%s"
	var errs []error
	for _, tag := range ir.Tags {
		if err := deleteDockerImageResource(tr.client, fmt.Sprintf(deleteImageURLFmt, imageName, tag)); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete tag %s: %v", tag, err))
		}
	}

	for manifest := range ir.Manifest {
		if err := deleteDockerImageResource(tr.client, fmt.Sprintf(deleteImageURLFmt, imageName, manifest)); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete manifest %s: %v", manifest, err))
		}
	}
	return errs
}

func deleteDockerImageResource(client *http.Client, url string) error {
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to get request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete resource: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("failed to delete resource: status code = %d", resp.StatusCode)
	}
	return nil
}

func (tr *testRunner) createCluster(ctx context.Context, client *http.Client, projectID, zone, clusterName string) error {
	request := &container.CreateClusterRequest{Cluster: &container.Cluster{
		Name:             clusterName,
		InitialNodeCount: 3,
		NodeConfig: &container.NodeConfig{
			OauthScopes: []string{
				storageReadScope,
			},
		},
	}}
	op, err := tr.containerService.Projects.Zones.Clusters.Create(projectID, zone, request).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to create cluster %s: %v", clusterName, err)
	}
	opID := op.Name

	// Wait for creating cluster.
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting creating cluster")

		case <-time.After(10 * time.Second):
			op, err := tr.containerService.Projects.Zones.Operations.Get(projectID, zone, opID).Context(ctx).Do()
			if err != nil {
				log.Printf("Transient error getting operation (will retry): %v", err)
				break
			}
			if op.Status == "DONE" {
				log.Printf("Created cluster %s.", clusterName)
				return nil
			}
			if op.Status == "ABORTING" {
				return fmt.Errorf("create cluster operation is aborted")
			}
		}
	}
}

func (tr *testRunner) deployContainer(ctx context.Context, kubernetesClient *kubernetes.Client, podName, imageName string) error {
	pod := &k8sapi.Pod{
		ObjectMeta: k8sapi.ObjectMeta{
			Name: podName,
		},
		Spec: k8sapi.PodSpec{
			Containers: []k8sapi.Container{
				{
					Name:  "profiler-test",
					Image: fmt.Sprintf("gcr.io/%s:latest", imageName),
				},
			},
		},
	}
	if _, err := kubernetesClient.RunLongLivedPod(ctx, pod); err != nil {
		return fmt.Errorf("failed to run pod %s: %v", podName, err)
	}
	return nil
}

func (tr *testRunner) pollPodLog(ctx context.Context, kubernetesClient *kubernetes.Client, podName string) error {
	var output string
	defer func() {
		log.Printf("Log for pod %s:\n%s", podName, output)
	}()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting profiling finishing on container")

		case <-time.After(20 * time.Second):
			var err error
			output, err = kubernetesClient.PodLog(ctx, podName)
			if err != nil {
				// Transient failure.
				log.Printf("Transient error getting log (will retry): %v", err)
				continue
			}
			if strings.Contains(output, benchFinishString) {
				return nil
			}
		}
	}
}

func (tr *testRunner) runTestOnGKE(ctx context.Context, t *testing.T, cfg clusterConfig, projectID, zone, bucket string) {
	if err := tr.uploadImageSource(ctx, bucket, cfg.imageSourceName, *commit, cfg.service); err != nil {
		t.Fatalf("uploadImageSource() got error: %v", err)
	}
	defer func() {
		if err := tr.storageClient.Bucket(bucket).Object(cfg.imageSourceName).Delete(ctx); err != nil {
			t.Errorf("Bucket(%s).Object(%s).Delete() got error: %v", bucket, cfg.imageSourceName, err)
		}
	}()

	createImageCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if err := tr.createAndPublishDockerImage(createImageCtx, projectID, bucket, cfg.imageSourceName, fmt.Sprintf("gcr.io/%s", cfg.imageName)); err != nil {
		t.Fatalf("createAndPublishDockerImage(%s) got error: %v", cfg.imageName, err)
	}
	defer func() {
		for _, err := range tr.deleteDockerImage(ctx, cfg.imageName) {
			t.Errorf("deleteDockerImage(%s) got error: %v", cfg.imageName, err)
		}
	}()

	createClusterCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if err := tr.createCluster(createClusterCtx, tr.client, projectID, zone, cfg.clusterName); err != nil {
		t.Fatalf("createCluster(%s) got error: %v", cfg.clusterName, err)
	}
	defer func() {
		if _, err := tr.containerService.Projects.Zones.Clusters.Delete(projectID, zone, cfg.clusterName).Context(ctx).Do(); err != nil {
			t.Errorf("Clusters.Delete(%s) got error: %v", cfg.clusterName, err)
		}
	}()

	kubernetesClient, err := gke.NewClient(ctx, cfg.clusterName, gke.OptZone(zone), gke.OptProject(projectID))
	if err != nil {
		t.Fatalf("gke.NewClient() got error: %v", err)
	}

	deployContainerCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if err := tr.deployContainer(deployContainerCtx, kubernetesClient, cfg.podName, cfg.imageName); err != nil {
		t.Fatalf("deployContainer(%s, %s) got error: %v", cfg.podName, cfg.imageName, err)
	}

	pollLogCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	if err := tr.pollPodLog(pollLogCtx, kubernetesClient, cfg.podName); err != nil {
		t.Fatalf("pollPodLog(%s) got error: %v", cfg.podName, err)
	}

	timeNow := time.Now()
	endTime := timeNow.Format(time.RFC3339)
	startTime := timeNow.Add(-1 * time.Hour).Format(time.RFC3339)
	for _, pType := range []string{"CPU", "HEAP", "THREADS"} {
		if err := tr.queryAndCheckProfile(cfg.service, startTime, endTime, pType, projectID); err != nil {
			t.Errorf("queryAndCheckProfile(%s, %s, %s, %s) got error: %v", cfg.service, startTime, endTime, pType, err)
		}
	}
}

// uploadImageSource uploads source code for building docker image to GCS.
func (tr *testRunner) uploadImageSource(ctx context.Context, bucket, objectName, commit, service string) error {
	zipBuf := new(bytes.Buffer)
	z := zip.NewWriter(zipBuf)
	f, err := z.Create("Dockerfile")
	if err != nil {
		return err
	}

	dockerfile := fmt.Sprintf(dockerfileFmt, commit, service)
	if _, err := f.Write([]byte(dockerfile)); err != nil {
		return err
	}

	if err := z.Close(); err != nil {
		return err
	}
	wc := tr.storageClient.Bucket(bucket).Object(objectName).NewWriter(ctx)
	wc.ContentType = "application/zip"
	wc.ACL = []storage.ACLRule{{storage.AllUsers, storage.RoleReader}}
	if _, err := wc.Write(zipBuf.Bytes()); err != nil {
		return err
	}
	return wc.Close()
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

	bucket := os.Getenv("GCLOUD_TESTS_GOLANG_BUCKET")
	if bucket == "" {
		t.Fatalf("Getenv(GCLOUD_TESTS_GOLANG_BUCKET) got empty string")
	}

	if *commit == "" {
		t.Fatal("commit flag is not set")
	}

	ctx := context.Background()

	client, err := google.DefaultClient(ctx, cloudScope)
	if err != nil {
		t.Fatalf("failed to get default client: %v", err)
	}

	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		t.Fatalf("storage.NewClient() error: %v", err)
	}

	computeService, err := compute.New(client)
	if err != nil {
		t.Fatalf("failed to initialize compute service: %v", err)
	}

	containerService, err := container.New(client)
	if err != nil {
		t.Fatalf("failed to create container client: %v", err)
	}

	template, err := template.New("startupScript").Parse(startupTemplate)
	if err != nil {
		t.Fatalf("failed to parse startup script template: %v", err)
	}
	tr := testRunner{
		computeService:   computeService,
		client:           client,
		startupTemplate:  template,
		containerService: containerService,
		storageClient:    storageClient,
	}

	cluster := newClusterConfig(projectID)
	t.Run(cluster.service, func(t *testing.T) {
		t.Parallel()
		tr.runTestOnGKE(ctx, t, cluster, projectID, zone, bucket)
	})

	instances := newInstanceConfigs()
	for _, instance := range instances {
		inst := instance // capture range variable
		t.Run(inst.service, func(t *testing.T) {
			t.Parallel()
			tr.runTestOnGCE(ctx, t, inst, projectID, zone)
		})
	}
}
