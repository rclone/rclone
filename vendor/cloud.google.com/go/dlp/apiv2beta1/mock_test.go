// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// AUTO-GENERATED CODE. DO NOT EDIT.

package dlp

import (
	longrunningpb "google.golang.org/genproto/googleapis/longrunning"
	dlppb "google.golang.org/genproto/googleapis/privacy/dlp/v2beta1"
)

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"golang.org/x/net/context"
	"google.golang.org/api/option"
	status "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	gstatus "google.golang.org/grpc/status"
)

var _ = io.EOF
var _ = ptypes.MarshalAny
var _ status.Status

type mockDlpServer struct {
	// Embed for forward compatibility.
	// Tests will keep working if more methods are added
	// in the future.
	dlppb.DlpServiceServer

	reqs []proto.Message

	// If set, all calls return this error.
	err error

	// responses to return if err == nil
	resps []proto.Message
}

func (s *mockDlpServer) InspectContent(ctx context.Context, req *dlppb.InspectContentRequest) (*dlppb.InspectContentResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if xg := md["x-goog-api-client"]; len(xg) == 0 || !strings.Contains(xg[0], "gl-go/") {
		return nil, fmt.Errorf("x-goog-api-client = %v, expected gl-go key", xg)
	}
	s.reqs = append(s.reqs, req)
	if s.err != nil {
		return nil, s.err
	}
	return s.resps[0].(*dlppb.InspectContentResponse), nil
}

func (s *mockDlpServer) RedactContent(ctx context.Context, req *dlppb.RedactContentRequest) (*dlppb.RedactContentResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if xg := md["x-goog-api-client"]; len(xg) == 0 || !strings.Contains(xg[0], "gl-go/") {
		return nil, fmt.Errorf("x-goog-api-client = %v, expected gl-go key", xg)
	}
	s.reqs = append(s.reqs, req)
	if s.err != nil {
		return nil, s.err
	}
	return s.resps[0].(*dlppb.RedactContentResponse), nil
}

func (s *mockDlpServer) DeidentifyContent(ctx context.Context, req *dlppb.DeidentifyContentRequest) (*dlppb.DeidentifyContentResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if xg := md["x-goog-api-client"]; len(xg) == 0 || !strings.Contains(xg[0], "gl-go/") {
		return nil, fmt.Errorf("x-goog-api-client = %v, expected gl-go key", xg)
	}
	s.reqs = append(s.reqs, req)
	if s.err != nil {
		return nil, s.err
	}
	return s.resps[0].(*dlppb.DeidentifyContentResponse), nil
}

func (s *mockDlpServer) CreateInspectOperation(ctx context.Context, req *dlppb.CreateInspectOperationRequest) (*longrunningpb.Operation, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if xg := md["x-goog-api-client"]; len(xg) == 0 || !strings.Contains(xg[0], "gl-go/") {
		return nil, fmt.Errorf("x-goog-api-client = %v, expected gl-go key", xg)
	}
	s.reqs = append(s.reqs, req)
	if s.err != nil {
		return nil, s.err
	}
	return s.resps[0].(*longrunningpb.Operation), nil
}

func (s *mockDlpServer) AnalyzeDataSourceRisk(ctx context.Context, req *dlppb.AnalyzeDataSourceRiskRequest) (*longrunningpb.Operation, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if xg := md["x-goog-api-client"]; len(xg) == 0 || !strings.Contains(xg[0], "gl-go/") {
		return nil, fmt.Errorf("x-goog-api-client = %v, expected gl-go key", xg)
	}
	s.reqs = append(s.reqs, req)
	if s.err != nil {
		return nil, s.err
	}
	return s.resps[0].(*longrunningpb.Operation), nil
}

func (s *mockDlpServer) ListInspectFindings(ctx context.Context, req *dlppb.ListInspectFindingsRequest) (*dlppb.ListInspectFindingsResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if xg := md["x-goog-api-client"]; len(xg) == 0 || !strings.Contains(xg[0], "gl-go/") {
		return nil, fmt.Errorf("x-goog-api-client = %v, expected gl-go key", xg)
	}
	s.reqs = append(s.reqs, req)
	if s.err != nil {
		return nil, s.err
	}
	return s.resps[0].(*dlppb.ListInspectFindingsResponse), nil
}

func (s *mockDlpServer) ListInfoTypes(ctx context.Context, req *dlppb.ListInfoTypesRequest) (*dlppb.ListInfoTypesResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if xg := md["x-goog-api-client"]; len(xg) == 0 || !strings.Contains(xg[0], "gl-go/") {
		return nil, fmt.Errorf("x-goog-api-client = %v, expected gl-go key", xg)
	}
	s.reqs = append(s.reqs, req)
	if s.err != nil {
		return nil, s.err
	}
	return s.resps[0].(*dlppb.ListInfoTypesResponse), nil
}

func (s *mockDlpServer) ListRootCategories(ctx context.Context, req *dlppb.ListRootCategoriesRequest) (*dlppb.ListRootCategoriesResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if xg := md["x-goog-api-client"]; len(xg) == 0 || !strings.Contains(xg[0], "gl-go/") {
		return nil, fmt.Errorf("x-goog-api-client = %v, expected gl-go key", xg)
	}
	s.reqs = append(s.reqs, req)
	if s.err != nil {
		return nil, s.err
	}
	return s.resps[0].(*dlppb.ListRootCategoriesResponse), nil
}

// clientOpt is the option tests should use to connect to the test server.
// It is initialized by TestMain.
var clientOpt option.ClientOption

var (
	mockDlp mockDlpServer
)

func TestMain(m *testing.M) {
	flag.Parse()

	serv := grpc.NewServer()
	dlppb.RegisterDlpServiceServer(serv, &mockDlp)

	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		log.Fatal(err)
	}
	go serv.Serve(lis)

	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}
	clientOpt = option.WithGRPCConn(conn)

	os.Exit(m.Run())
}

func TestDlpServiceInspectContent(t *testing.T) {
	var expectedResponse *dlppb.InspectContentResponse = &dlppb.InspectContentResponse{}

	mockDlp.err = nil
	mockDlp.reqs = nil

	mockDlp.resps = append(mockDlp.resps[:0], expectedResponse)

	var name string = "EMAIL_ADDRESS"
	var infoTypesElement = &dlppb.InfoType{
		Name: name,
	}
	var infoTypes = []*dlppb.InfoType{infoTypesElement}
	var inspectConfig = &dlppb.InspectConfig{
		InfoTypes: infoTypes,
	}
	var type_ string = "text/plain"
	var value string = "My email is example@example.com."
	var itemsElement = &dlppb.ContentItem{
		Type: type_,
		DataItem: &dlppb.ContentItem_Value{
			Value: value,
		},
	}
	var items = []*dlppb.ContentItem{itemsElement}
	var request = &dlppb.InspectContentRequest{
		InspectConfig: inspectConfig,
		Items:         items,
	}

	c, err := NewClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.InspectContent(context.Background(), request)

	if err != nil {
		t.Fatal(err)
	}

	if want, got := request, mockDlp.reqs[0]; !proto.Equal(want, got) {
		t.Errorf("wrong request %q, want %q", got, want)
	}

	if want, got := expectedResponse, resp; !proto.Equal(want, got) {
		t.Errorf("wrong response %q, want %q)", got, want)
	}
}

func TestDlpServiceInspectContentError(t *testing.T) {
	errCode := codes.PermissionDenied
	mockDlp.err = gstatus.Error(errCode, "test error")

	var name string = "EMAIL_ADDRESS"
	var infoTypesElement = &dlppb.InfoType{
		Name: name,
	}
	var infoTypes = []*dlppb.InfoType{infoTypesElement}
	var inspectConfig = &dlppb.InspectConfig{
		InfoTypes: infoTypes,
	}
	var type_ string = "text/plain"
	var value string = "My email is example@example.com."
	var itemsElement = &dlppb.ContentItem{
		Type: type_,
		DataItem: &dlppb.ContentItem_Value{
			Value: value,
		},
	}
	var items = []*dlppb.ContentItem{itemsElement}
	var request = &dlppb.InspectContentRequest{
		InspectConfig: inspectConfig,
		Items:         items,
	}

	c, err := NewClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.InspectContent(context.Background(), request)

	if st, ok := gstatus.FromError(err); !ok {
		t.Errorf("got error %v, expected grpc error", err)
	} else if c := st.Code(); c != errCode {
		t.Errorf("got error code %q, want %q", c, errCode)
	}
	_ = resp
}
func TestDlpServiceRedactContent(t *testing.T) {
	var expectedResponse *dlppb.RedactContentResponse = &dlppb.RedactContentResponse{}

	mockDlp.err = nil
	mockDlp.reqs = nil

	mockDlp.resps = append(mockDlp.resps[:0], expectedResponse)

	var name string = "EMAIL_ADDRESS"
	var infoTypesElement = &dlppb.InfoType{
		Name: name,
	}
	var infoTypes = []*dlppb.InfoType{infoTypesElement}
	var inspectConfig = &dlppb.InspectConfig{
		InfoTypes: infoTypes,
	}
	var type_ string = "text/plain"
	var value string = "My email is example@example.com."
	var itemsElement = &dlppb.ContentItem{
		Type: type_,
		DataItem: &dlppb.ContentItem_Value{
			Value: value,
		},
	}
	var items = []*dlppb.ContentItem{itemsElement}
	var name2 string = "EMAIL_ADDRESS"
	var infoType = &dlppb.InfoType{
		Name: name2,
	}
	var replaceWith string = "REDACTED"
	var replaceConfigsElement = &dlppb.RedactContentRequest_ReplaceConfig{
		InfoType:    infoType,
		ReplaceWith: replaceWith,
	}
	var replaceConfigs = []*dlppb.RedactContentRequest_ReplaceConfig{replaceConfigsElement}
	var request = &dlppb.RedactContentRequest{
		InspectConfig:  inspectConfig,
		Items:          items,
		ReplaceConfigs: replaceConfigs,
	}

	c, err := NewClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.RedactContent(context.Background(), request)

	if err != nil {
		t.Fatal(err)
	}

	if want, got := request, mockDlp.reqs[0]; !proto.Equal(want, got) {
		t.Errorf("wrong request %q, want %q", got, want)
	}

	if want, got := expectedResponse, resp; !proto.Equal(want, got) {
		t.Errorf("wrong response %q, want %q)", got, want)
	}
}

func TestDlpServiceRedactContentError(t *testing.T) {
	errCode := codes.PermissionDenied
	mockDlp.err = gstatus.Error(errCode, "test error")

	var name string = "EMAIL_ADDRESS"
	var infoTypesElement = &dlppb.InfoType{
		Name: name,
	}
	var infoTypes = []*dlppb.InfoType{infoTypesElement}
	var inspectConfig = &dlppb.InspectConfig{
		InfoTypes: infoTypes,
	}
	var type_ string = "text/plain"
	var value string = "My email is example@example.com."
	var itemsElement = &dlppb.ContentItem{
		Type: type_,
		DataItem: &dlppb.ContentItem_Value{
			Value: value,
		},
	}
	var items = []*dlppb.ContentItem{itemsElement}
	var name2 string = "EMAIL_ADDRESS"
	var infoType = &dlppb.InfoType{
		Name: name2,
	}
	var replaceWith string = "REDACTED"
	var replaceConfigsElement = &dlppb.RedactContentRequest_ReplaceConfig{
		InfoType:    infoType,
		ReplaceWith: replaceWith,
	}
	var replaceConfigs = []*dlppb.RedactContentRequest_ReplaceConfig{replaceConfigsElement}
	var request = &dlppb.RedactContentRequest{
		InspectConfig:  inspectConfig,
		Items:          items,
		ReplaceConfigs: replaceConfigs,
	}

	c, err := NewClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.RedactContent(context.Background(), request)

	if st, ok := gstatus.FromError(err); !ok {
		t.Errorf("got error %v, expected grpc error", err)
	} else if c := st.Code(); c != errCode {
		t.Errorf("got error code %q, want %q", c, errCode)
	}
	_ = resp
}
func TestDlpServiceDeidentifyContent(t *testing.T) {
	var expectedResponse *dlppb.DeidentifyContentResponse = &dlppb.DeidentifyContentResponse{}

	mockDlp.err = nil
	mockDlp.reqs = nil

	mockDlp.resps = append(mockDlp.resps[:0], expectedResponse)

	var deidentifyConfig *dlppb.DeidentifyConfig = &dlppb.DeidentifyConfig{}
	var inspectConfig *dlppb.InspectConfig = &dlppb.InspectConfig{}
	var items []*dlppb.ContentItem = nil
	var request = &dlppb.DeidentifyContentRequest{
		DeidentifyConfig: deidentifyConfig,
		InspectConfig:    inspectConfig,
		Items:            items,
	}

	c, err := NewClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.DeidentifyContent(context.Background(), request)

	if err != nil {
		t.Fatal(err)
	}

	if want, got := request, mockDlp.reqs[0]; !proto.Equal(want, got) {
		t.Errorf("wrong request %q, want %q", got, want)
	}

	if want, got := expectedResponse, resp; !proto.Equal(want, got) {
		t.Errorf("wrong response %q, want %q)", got, want)
	}
}

func TestDlpServiceDeidentifyContentError(t *testing.T) {
	errCode := codes.PermissionDenied
	mockDlp.err = gstatus.Error(errCode, "test error")

	var deidentifyConfig *dlppb.DeidentifyConfig = &dlppb.DeidentifyConfig{}
	var inspectConfig *dlppb.InspectConfig = &dlppb.InspectConfig{}
	var items []*dlppb.ContentItem = nil
	var request = &dlppb.DeidentifyContentRequest{
		DeidentifyConfig: deidentifyConfig,
		InspectConfig:    inspectConfig,
		Items:            items,
	}

	c, err := NewClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.DeidentifyContent(context.Background(), request)

	if st, ok := gstatus.FromError(err); !ok {
		t.Errorf("got error %v, expected grpc error", err)
	} else if c := st.Code(); c != errCode {
		t.Errorf("got error code %q, want %q", c, errCode)
	}
	_ = resp
}
func TestDlpServiceAnalyzeDataSourceRisk(t *testing.T) {
	var expectedResponse *dlppb.RiskAnalysisOperationResult = &dlppb.RiskAnalysisOperationResult{}

	mockDlp.err = nil
	mockDlp.reqs = nil

	any, err := ptypes.MarshalAny(expectedResponse)
	if err != nil {
		t.Fatal(err)
	}
	mockDlp.resps = append(mockDlp.resps[:0], &longrunningpb.Operation{
		Name:   "longrunning-test",
		Done:   true,
		Result: &longrunningpb.Operation_Response{Response: any},
	})

	var privacyMetric *dlppb.PrivacyMetric = &dlppb.PrivacyMetric{}
	var sourceTable *dlppb.BigQueryTable = &dlppb.BigQueryTable{}
	var request = &dlppb.AnalyzeDataSourceRiskRequest{
		PrivacyMetric: privacyMetric,
		SourceTable:   sourceTable,
	}

	c, err := NewClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	respLRO, err := c.AnalyzeDataSourceRisk(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := respLRO.Wait(context.Background())

	if err != nil {
		t.Fatal(err)
	}

	if want, got := request, mockDlp.reqs[0]; !proto.Equal(want, got) {
		t.Errorf("wrong request %q, want %q", got, want)
	}

	if want, got := expectedResponse, resp; !proto.Equal(want, got) {
		t.Errorf("wrong response %q, want %q)", got, want)
	}
}

func TestDlpServiceAnalyzeDataSourceRiskError(t *testing.T) {
	errCode := codes.PermissionDenied
	mockDlp.err = nil
	mockDlp.resps = append(mockDlp.resps[:0], &longrunningpb.Operation{
		Name: "longrunning-test",
		Done: true,
		Result: &longrunningpb.Operation_Error{
			Error: &status.Status{
				Code:    int32(errCode),
				Message: "test error",
			},
		},
	})

	var privacyMetric *dlppb.PrivacyMetric = &dlppb.PrivacyMetric{}
	var sourceTable *dlppb.BigQueryTable = &dlppb.BigQueryTable{}
	var request = &dlppb.AnalyzeDataSourceRiskRequest{
		PrivacyMetric: privacyMetric,
		SourceTable:   sourceTable,
	}

	c, err := NewClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	respLRO, err := c.AnalyzeDataSourceRisk(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := respLRO.Wait(context.Background())

	if st, ok := gstatus.FromError(err); !ok {
		t.Errorf("got error %v, expected grpc error", err)
	} else if c := st.Code(); c != errCode {
		t.Errorf("got error code %q, want %q", c, errCode)
	}
	_ = resp
}
func TestDlpServiceCreateInspectOperation(t *testing.T) {
	var name2 string = "name2-1052831874"
	var expectedResponse = &dlppb.InspectOperationResult{
		Name: name2,
	}

	mockDlp.err = nil
	mockDlp.reqs = nil

	any, err := ptypes.MarshalAny(expectedResponse)
	if err != nil {
		t.Fatal(err)
	}
	mockDlp.resps = append(mockDlp.resps[:0], &longrunningpb.Operation{
		Name:   "longrunning-test",
		Done:   true,
		Result: &longrunningpb.Operation_Response{Response: any},
	})

	var name string = "EMAIL_ADDRESS"
	var infoTypesElement = &dlppb.InfoType{
		Name: name,
	}
	var infoTypes = []*dlppb.InfoType{infoTypesElement}
	var inspectConfig = &dlppb.InspectConfig{
		InfoTypes: infoTypes,
	}
	var url string = "gs://example_bucket/example_file.png"
	var fileSet = &dlppb.CloudStorageOptions_FileSet{
		Url: url,
	}
	var cloudStorageOptions = &dlppb.CloudStorageOptions{
		FileSet: fileSet,
	}
	var storageConfig = &dlppb.StorageConfig{
		Type: &dlppb.StorageConfig_CloudStorageOptions{
			CloudStorageOptions: cloudStorageOptions,
		},
	}
	var outputConfig *dlppb.OutputStorageConfig = &dlppb.OutputStorageConfig{}
	var request = &dlppb.CreateInspectOperationRequest{
		InspectConfig: inspectConfig,
		StorageConfig: storageConfig,
		OutputConfig:  outputConfig,
	}

	c, err := NewClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	respLRO, err := c.CreateInspectOperation(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := respLRO.Wait(context.Background())

	if err != nil {
		t.Fatal(err)
	}

	if want, got := request, mockDlp.reqs[0]; !proto.Equal(want, got) {
		t.Errorf("wrong request %q, want %q", got, want)
	}

	if want, got := expectedResponse, resp; !proto.Equal(want, got) {
		t.Errorf("wrong response %q, want %q)", got, want)
	}
}

func TestDlpServiceCreateInspectOperationError(t *testing.T) {
	errCode := codes.PermissionDenied
	mockDlp.err = nil
	mockDlp.resps = append(mockDlp.resps[:0], &longrunningpb.Operation{
		Name: "longrunning-test",
		Done: true,
		Result: &longrunningpb.Operation_Error{
			Error: &status.Status{
				Code:    int32(errCode),
				Message: "test error",
			},
		},
	})

	var name string = "EMAIL_ADDRESS"
	var infoTypesElement = &dlppb.InfoType{
		Name: name,
	}
	var infoTypes = []*dlppb.InfoType{infoTypesElement}
	var inspectConfig = &dlppb.InspectConfig{
		InfoTypes: infoTypes,
	}
	var url string = "gs://example_bucket/example_file.png"
	var fileSet = &dlppb.CloudStorageOptions_FileSet{
		Url: url,
	}
	var cloudStorageOptions = &dlppb.CloudStorageOptions{
		FileSet: fileSet,
	}
	var storageConfig = &dlppb.StorageConfig{
		Type: &dlppb.StorageConfig_CloudStorageOptions{
			CloudStorageOptions: cloudStorageOptions,
		},
	}
	var outputConfig *dlppb.OutputStorageConfig = &dlppb.OutputStorageConfig{}
	var request = &dlppb.CreateInspectOperationRequest{
		InspectConfig: inspectConfig,
		StorageConfig: storageConfig,
		OutputConfig:  outputConfig,
	}

	c, err := NewClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	respLRO, err := c.CreateInspectOperation(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := respLRO.Wait(context.Background())

	if st, ok := gstatus.FromError(err); !ok {
		t.Errorf("got error %v, expected grpc error", err)
	} else if c := st.Code(); c != errCode {
		t.Errorf("got error code %q, want %q", c, errCode)
	}
	_ = resp
}
func TestDlpServiceListInspectFindings(t *testing.T) {
	var nextPageToken string = "nextPageToken-1530815211"
	var expectedResponse = &dlppb.ListInspectFindingsResponse{
		NextPageToken: nextPageToken,
	}

	mockDlp.err = nil
	mockDlp.reqs = nil

	mockDlp.resps = append(mockDlp.resps[:0], expectedResponse)

	var formattedName string = fmt.Sprintf("inspect/results/%s", "[RESULT]")
	var request = &dlppb.ListInspectFindingsRequest{
		Name: formattedName,
	}

	c, err := NewClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.ListInspectFindings(context.Background(), request)

	if err != nil {
		t.Fatal(err)
	}

	if want, got := request, mockDlp.reqs[0]; !proto.Equal(want, got) {
		t.Errorf("wrong request %q, want %q", got, want)
	}

	if want, got := expectedResponse, resp; !proto.Equal(want, got) {
		t.Errorf("wrong response %q, want %q)", got, want)
	}
}

func TestDlpServiceListInspectFindingsError(t *testing.T) {
	errCode := codes.PermissionDenied
	mockDlp.err = gstatus.Error(errCode, "test error")

	var formattedName string = fmt.Sprintf("inspect/results/%s", "[RESULT]")
	var request = &dlppb.ListInspectFindingsRequest{
		Name: formattedName,
	}

	c, err := NewClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.ListInspectFindings(context.Background(), request)

	if st, ok := gstatus.FromError(err); !ok {
		t.Errorf("got error %v, expected grpc error", err)
	} else if c := st.Code(); c != errCode {
		t.Errorf("got error code %q, want %q", c, errCode)
	}
	_ = resp
}
func TestDlpServiceListInfoTypes(t *testing.T) {
	var expectedResponse *dlppb.ListInfoTypesResponse = &dlppb.ListInfoTypesResponse{}

	mockDlp.err = nil
	mockDlp.reqs = nil

	mockDlp.resps = append(mockDlp.resps[:0], expectedResponse)

	var category string = "PII"
	var languageCode string = "en"
	var request = &dlppb.ListInfoTypesRequest{
		Category:     category,
		LanguageCode: languageCode,
	}

	c, err := NewClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.ListInfoTypes(context.Background(), request)

	if err != nil {
		t.Fatal(err)
	}

	if want, got := request, mockDlp.reqs[0]; !proto.Equal(want, got) {
		t.Errorf("wrong request %q, want %q", got, want)
	}

	if want, got := expectedResponse, resp; !proto.Equal(want, got) {
		t.Errorf("wrong response %q, want %q)", got, want)
	}
}

func TestDlpServiceListInfoTypesError(t *testing.T) {
	errCode := codes.PermissionDenied
	mockDlp.err = gstatus.Error(errCode, "test error")

	var category string = "PII"
	var languageCode string = "en"
	var request = &dlppb.ListInfoTypesRequest{
		Category:     category,
		LanguageCode: languageCode,
	}

	c, err := NewClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.ListInfoTypes(context.Background(), request)

	if st, ok := gstatus.FromError(err); !ok {
		t.Errorf("got error %v, expected grpc error", err)
	} else if c := st.Code(); c != errCode {
		t.Errorf("got error code %q, want %q", c, errCode)
	}
	_ = resp
}
func TestDlpServiceListRootCategories(t *testing.T) {
	var expectedResponse *dlppb.ListRootCategoriesResponse = &dlppb.ListRootCategoriesResponse{}

	mockDlp.err = nil
	mockDlp.reqs = nil

	mockDlp.resps = append(mockDlp.resps[:0], expectedResponse)

	var languageCode string = "en"
	var request = &dlppb.ListRootCategoriesRequest{
		LanguageCode: languageCode,
	}

	c, err := NewClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.ListRootCategories(context.Background(), request)

	if err != nil {
		t.Fatal(err)
	}

	if want, got := request, mockDlp.reqs[0]; !proto.Equal(want, got) {
		t.Errorf("wrong request %q, want %q", got, want)
	}

	if want, got := expectedResponse, resp; !proto.Equal(want, got) {
		t.Errorf("wrong response %q, want %q)", got, want)
	}
}

func TestDlpServiceListRootCategoriesError(t *testing.T) {
	errCode := codes.PermissionDenied
	mockDlp.err = gstatus.Error(errCode, "test error")

	var languageCode string = "en"
	var request = &dlppb.ListRootCategoriesRequest{
		LanguageCode: languageCode,
	}

	c, err := NewClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.ListRootCategories(context.Background(), request)

	if st, ok := gstatus.FromError(err); !ok {
		t.Errorf("got error %v, expected grpc error", err)
	} else if c := st.Code(); c != errCode {
		t.Errorf("got error code %q, want %q", c, errCode)
	}
	_ = resp
}
