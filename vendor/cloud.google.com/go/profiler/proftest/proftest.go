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
//
// Package proftest contains test helpers for profiler agent integration tests.
// This package is experimental.

// golang.org/x/build/kubernetes/dialer.go imports "context" package (rather
// than "golang.org/x/net/context") and that does not exist in Go 1.6 or
// earlier.
// +build go1.7

package proftest

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"golang.org/x/build/kubernetes"
	k8sapi "golang.org/x/build/kubernetes/api"
	"golang.org/x/build/kubernetes/gke"
	"golang.org/x/net/context"
	cloudbuild "google.golang.org/api/cloudbuild/v1"
	compute "google.golang.org/api/compute/v1"
	container "google.golang.org/api/container/v1"
	"google.golang.org/api/googleapi"
)

const (
	monitorWriteScope = "https://www.googleapis.com/auth/monitoring.write"
	storageReadScope  = "https://www.googleapis.com/auth/devstorage.read_only"
)

// TestRunner has common elements used for testing profiling agents on a range
// of environments.
type TestRunner struct {
	Client *http.Client
}

// GCETestRunner supports testing a profiling agent on GCE.
type GCETestRunner struct {
	TestRunner
	ComputeService *compute.Service
}

// GKETestRunner supports testing a profiling agent on GKE.
type GKETestRunner struct {
	TestRunner
	ContainerService *container.Service
	StorageClient    *storage.Client
	Dockerfile       string
}

// ProfileResponse contains the response produced when querying profile server.
type ProfileResponse struct {
	Profile     ProfileData   `json:"profile"`
	NumProfiles int32         `json:"numProfiles"`
	Deployments []interface{} `json:"deployments"`
}

// ProfileData has data of a single profile.
type ProfileData struct {
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

// InstanceConfig is configuration for starting single GCE instance for
// profiling agent test case.
type InstanceConfig struct {
	ProjectID     string
	Zone          string
	Name          string
	StartupScript string
	MachineType   string
}

// ClusterConfig is configuration for starting single GKE cluster for profiling
// agent test case.
type ClusterConfig struct {
	ProjectID       string
	Zone            string
	ClusterName     string
	PodName         string
	ImageSourceName string
	ImageName       string
	Bucket          string
	Dockerfile      string
}

// HasFunction returns nil if the function is present, or, if the function is
// not present, and error providing more details why the function is not
// present.
func (pr *ProfileResponse) HasFunction(functionName string) error {
	if pr.NumProfiles == 0 {
		return fmt.Errorf("failed to find function name %s in profile: profile response contains zero profiles: %v", functionName, pr)
	}
	if len(pr.Deployments) == 0 {
		return fmt.Errorf("failed to find function name %s in profile: profile response contains zero deployments: %v", functionName, pr)
	}
	if len(pr.Profile.Functions.Name) == 0 {
		return fmt.Errorf("failed to find function name %s in profile: profile does not have function data", functionName)
	}

	for _, name := range pr.Profile.Functions.Name {
		if strings.Contains(name, functionName) {
			return nil
		}
	}
	return fmt.Errorf("failed to find function name %s in profile", functionName)
}

// StartInstance starts a GCE Instance with name, zone, and projectId specified
// by the inst, and which runs the startup script specified in inst.
func (tr *GCETestRunner) StartInstance(ctx context.Context, inst *InstanceConfig) error {
	img, err := tr.ComputeService.Images.GetFromFamily("debian-cloud", "debian-9").Context(ctx).Do()
	if err != nil {
		return err
	}

	_, err = tr.ComputeService.Instances.Insert(inst.ProjectID, inst.Zone, &compute.Instance{
		MachineType: fmt.Sprintf("zones/%s/machineTypes/%s", inst.Zone, inst.MachineType),
		Name:        inst.Name,
		Disks: []*compute.AttachedDisk{{
			AutoDelete: true, // delete the disk when the VM is deleted.
			Boot:       true,
			Type:       "PERSISTENT",
			Mode:       "READ_WRITE",
			InitializeParams: &compute.AttachedDiskInitializeParams{
				SourceImage: img.SelfLink,
				DiskType:    fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/zones/%s/diskTypes/pd-standard", inst.ProjectID, inst.Zone),
			},
		}},
		NetworkInterfaces: []*compute.NetworkInterface{{
			Network: fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/global/networks/default", inst.ProjectID),
			AccessConfigs: []*compute.AccessConfig{{
				Name: "External NAT",
			}},
		}},
		Metadata: &compute.Metadata{
			Items: []*compute.MetadataItems{{
				Key:   "startup-script",
				Value: googleapi.String(inst.StartupScript),
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

// DeleteInstance deletes an instance with project id, name, and zone matched
// by inst.
func (tr *GCETestRunner) DeleteInstance(ctx context.Context, inst *InstanceConfig) error {
	if _, err := tr.ComputeService.Instances.Delete(inst.ProjectID, inst.Zone, inst.Name).Context(ctx).Do(); err != nil {
		return fmt.Errorf("Instances.Delete(%s) got error: %v", inst.Name, err)
	}
	return nil
}

// PollForSerialOutput polls serial port 2 of the GCE instance specified by
// inst and returns when the finishString appears in the serial output
// of the instance, or when the context times out.
func (tr *GCETestRunner) PollForSerialOutput(ctx context.Context, inst *InstanceConfig, finishString string) error {
	var output string
	defer func() {
		log.Printf("Serial port output for %s:\n%s", inst.Name, output)
	}()

	for {
		select {
		case <-ctx.Done():
		case <-time.After(20 * time.Second):
			resp, err := tr.ComputeService.Instances.GetSerialPortOutput(inst.ProjectID, inst.Zone, inst.Name).Port(2).Context(ctx).Do()
			if err != nil {
				// Transient failure.
				log.Printf("Transient error getting serial port output from instance %s (will retry): %v", inst.Name, err)
				continue
			}

			if output = resp.Contents; strings.Contains(output, finishString) {
				return nil
			}
		}
	}
}

// QueryProfiles retrieves profiles of a specific type, from a specific time
// range, associated with a particular service and project.
func (tr *TestRunner) QueryProfiles(projectID, service, startTime, endTime, profileType string) (ProfileResponse, error) {
	queryURL := fmt.Sprintf("https://cloudprofiler.googleapis.com/v2/projects/%s/profiles:query", projectID)
	const queryJSONFmt = `{"endTime": "%s", "profileType": "%s","startTime": "%s", "target": "%s"}`

	queryRequest := fmt.Sprintf(queryJSONFmt, endTime, profileType, startTime, service)

	resp, err := tr.Client.Post(queryURL, "application/json", strings.NewReader(queryRequest))
	if err != nil {
		return ProfileResponse{}, fmt.Errorf("failed to query API: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ProfileResponse{}, fmt.Errorf("failed to read response body: %v", err)
	}

	var pr ProfileResponse
	if err := json.Unmarshal(body, &pr); err != nil {
		return ProfileResponse{}, err
	}

	return pr, nil
}

// createAndPublishDockerImage creates a docker image from source code in a GCS
// bucket and pushes the image to Google Container Registry.
func (tr *GKETestRunner) createAndPublishDockerImage(ctx context.Context, projectID, sourceBucket, sourceObject, ImageName string) error {
	cloudbuildService, err := cloudbuild.New(tr.Client)

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
				Args: []string{"build", "-t", ImageName, "."},
			},
		},
		Images: []string{ImageName},
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
				log.Printf("Published image %s to Google Container Registry.", ImageName)
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
func (tr *GKETestRunner) deleteDockerImage(ctx context.Context, ImageName string) []error {
	queryImageURL := fmt.Sprintf("https://gcr.io/v2/%s/tags/list", ImageName)
	resp, err := tr.Client.Get(queryImageURL)
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
		if err := deleteDockerImageResource(tr.Client, fmt.Sprintf(deleteImageURLFmt, ImageName, tag)); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete tag %s: %v", tag, err))
		}
	}

	for manifest := range ir.Manifest {
		if err := deleteDockerImageResource(tr.Client, fmt.Sprintf(deleteImageURLFmt, ImageName, manifest)); err != nil {
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

func (tr *GKETestRunner) createCluster(ctx context.Context, client *http.Client, projectID, zone, ClusterName string) error {
	request := &container.CreateClusterRequest{Cluster: &container.Cluster{
		Name:             ClusterName,
		InitialNodeCount: 3,
		NodeConfig: &container.NodeConfig{
			OauthScopes: []string{
				storageReadScope,
			},
		},
	}}
	op, err := tr.ContainerService.Projects.Zones.Clusters.Create(projectID, zone, request).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to create cluster %s: %v", ClusterName, err)
	}
	opID := op.Name

	// Wait for creating cluster.
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting creating cluster")

		case <-time.After(10 * time.Second):
			op, err := tr.ContainerService.Projects.Zones.Operations.Get(projectID, zone, opID).Context(ctx).Do()
			if err != nil {
				log.Printf("Transient error getting operation (will retry): %v", err)
				break
			}
			if op.Status == "DONE" {
				log.Printf("Created cluster %s.", ClusterName)
				return nil
			}
			if op.Status == "ABORTING" {
				return fmt.Errorf("create cluster operation is aborted")
			}
		}
	}
}

func (tr *GKETestRunner) deployContainer(ctx context.Context, kubernetesClient *kubernetes.Client, podName, ImageName string) error {
	pod := &k8sapi.Pod{
		ObjectMeta: k8sapi.ObjectMeta{
			Name: podName,
		},
		Spec: k8sapi.PodSpec{
			Containers: []k8sapi.Container{
				{
					Name:  "profiler-test",
					Image: fmt.Sprintf("gcr.io/%s:latest", ImageName),
				},
			},
		},
	}
	if _, err := kubernetesClient.RunLongLivedPod(ctx, pod); err != nil {
		return fmt.Errorf("failed to run pod %s: %v", podName, err)
	}
	return nil
}

// PollPodLog polls the log of the kubernetes client and returns when the
// finishString appears in the log, or when the context times out.
func (tr *GKETestRunner) PollPodLog(ctx context.Context, kubernetesClient *kubernetes.Client, podName, finishString string) error {
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
			if strings.Contains(output, finishString) {
				return nil
			}
		}
	}
}

// DeleteClusterAndImage deletes cluster and images used to create cluster.
func (tr *GKETestRunner) DeleteClusterAndImage(ctx context.Context, cfg *ClusterConfig) []error {
	var errs []error
	if err := tr.StorageClient.Bucket(cfg.Bucket).Object(cfg.ImageSourceName).Delete(ctx); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete storage client: %v", err))
	}
	for _, err := range tr.deleteDockerImage(ctx, cfg.ImageName) {
		errs = append(errs, fmt.Errorf("failed to delete docker image: %v", err))
	}
	if _, err := tr.ContainerService.Projects.Zones.Clusters.Delete(cfg.ProjectID, cfg.Zone, cfg.ClusterName).Context(ctx).Do(); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete cluster %s: %v", cfg.ClusterName, err))
	}

	return errs
}

// StartAndDeployCluster creates image needed for cluster, then starts and
// deploys to cluster.
func (tr *GKETestRunner) StartAndDeployCluster(ctx context.Context, cfg *ClusterConfig) error {
	if err := tr.uploadImageSource(ctx, cfg.Bucket, cfg.ImageSourceName, cfg.Dockerfile); err != nil {
		return fmt.Errorf("failed to upload image source: %v", err)
	}

	createImageCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if err := tr.createAndPublishDockerImage(createImageCtx, cfg.ProjectID, cfg.Bucket, cfg.ImageSourceName, fmt.Sprintf("gcr.io/%s", cfg.ImageName)); err != nil {
		return fmt.Errorf("failed to create and publish docker image %s: %v", cfg.ImageName, err)
	}

	kubernetesClient, err := gke.NewClient(ctx, cfg.ClusterName, gke.OptZone(cfg.Zone), gke.OptProject(cfg.ProjectID))
	if err != nil {
		return fmt.Errorf("failed to create new GKE client: %v", err)
	}

	deployContainerCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if err := tr.deployContainer(deployContainerCtx, kubernetesClient, cfg.PodName, cfg.ImageName); err != nil {
		return fmt.Errorf("failed to deploy image %q to pod %q: %v", cfg.PodName, cfg.ImageName, err)
	}
	return nil
}

// uploadImageSource uploads source code for building docker image to GCS.
func (tr *GKETestRunner) uploadImageSource(ctx context.Context, bucket, objectName, dockerfile string) error {
	zipBuf := new(bytes.Buffer)
	z := zip.NewWriter(zipBuf)
	f, err := z.Create("Dockerfile")
	if err != nil {
		return err
	}

	if _, err := f.Write([]byte(dockerfile)); err != nil {
		return err
	}

	if err := z.Close(); err != nil {
		return err
	}
	wc := tr.StorageClient.Bucket(bucket).Object(objectName).NewWriter(ctx)
	wc.ContentType = "application/zip"
	wc.ACL = []storage.ACLRule{{storage.AllUsers, storage.RoleReader}}
	if _, err := wc.Write(zipBuf.Bytes()); err != nil {
		return err
	}
	return wc.Close()
}
