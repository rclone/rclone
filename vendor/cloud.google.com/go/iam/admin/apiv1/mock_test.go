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

package admin

import (
	emptypb "github.com/golang/protobuf/ptypes/empty"
	adminpb "google.golang.org/genproto/googleapis/iam/admin/v1"
	iampb "google.golang.org/genproto/googleapis/iam/v1"
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

type mockIamServer struct {
	// Embed for forward compatibility.
	// Tests will keep working if more methods are added
	// in the future.
	adminpb.IAMServer

	reqs []proto.Message

	// If set, all calls return this error.
	err error

	// responses to return if err == nil
	resps []proto.Message
}

func (s *mockIamServer) ListServiceAccounts(ctx context.Context, req *adminpb.ListServiceAccountsRequest) (*adminpb.ListServiceAccountsResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if xg := md["x-goog-api-client"]; len(xg) == 0 || !strings.Contains(xg[0], "gl-go/") {
		return nil, fmt.Errorf("x-goog-api-client = %v, expected gl-go key", xg)
	}
	s.reqs = append(s.reqs, req)
	if s.err != nil {
		return nil, s.err
	}
	return s.resps[0].(*adminpb.ListServiceAccountsResponse), nil
}

func (s *mockIamServer) GetServiceAccount(ctx context.Context, req *adminpb.GetServiceAccountRequest) (*adminpb.ServiceAccount, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if xg := md["x-goog-api-client"]; len(xg) == 0 || !strings.Contains(xg[0], "gl-go/") {
		return nil, fmt.Errorf("x-goog-api-client = %v, expected gl-go key", xg)
	}
	s.reqs = append(s.reqs, req)
	if s.err != nil {
		return nil, s.err
	}
	return s.resps[0].(*adminpb.ServiceAccount), nil
}

func (s *mockIamServer) CreateServiceAccount(ctx context.Context, req *adminpb.CreateServiceAccountRequest) (*adminpb.ServiceAccount, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if xg := md["x-goog-api-client"]; len(xg) == 0 || !strings.Contains(xg[0], "gl-go/") {
		return nil, fmt.Errorf("x-goog-api-client = %v, expected gl-go key", xg)
	}
	s.reqs = append(s.reqs, req)
	if s.err != nil {
		return nil, s.err
	}
	return s.resps[0].(*adminpb.ServiceAccount), nil
}

func (s *mockIamServer) UpdateServiceAccount(ctx context.Context, req *adminpb.ServiceAccount) (*adminpb.ServiceAccount, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if xg := md["x-goog-api-client"]; len(xg) == 0 || !strings.Contains(xg[0], "gl-go/") {
		return nil, fmt.Errorf("x-goog-api-client = %v, expected gl-go key", xg)
	}
	s.reqs = append(s.reqs, req)
	if s.err != nil {
		return nil, s.err
	}
	return s.resps[0].(*adminpb.ServiceAccount), nil
}

func (s *mockIamServer) DeleteServiceAccount(ctx context.Context, req *adminpb.DeleteServiceAccountRequest) (*emptypb.Empty, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if xg := md["x-goog-api-client"]; len(xg) == 0 || !strings.Contains(xg[0], "gl-go/") {
		return nil, fmt.Errorf("x-goog-api-client = %v, expected gl-go key", xg)
	}
	s.reqs = append(s.reqs, req)
	if s.err != nil {
		return nil, s.err
	}
	return s.resps[0].(*emptypb.Empty), nil
}

func (s *mockIamServer) ListServiceAccountKeys(ctx context.Context, req *adminpb.ListServiceAccountKeysRequest) (*adminpb.ListServiceAccountKeysResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if xg := md["x-goog-api-client"]; len(xg) == 0 || !strings.Contains(xg[0], "gl-go/") {
		return nil, fmt.Errorf("x-goog-api-client = %v, expected gl-go key", xg)
	}
	s.reqs = append(s.reqs, req)
	if s.err != nil {
		return nil, s.err
	}
	return s.resps[0].(*adminpb.ListServiceAccountKeysResponse), nil
}

func (s *mockIamServer) GetServiceAccountKey(ctx context.Context, req *adminpb.GetServiceAccountKeyRequest) (*adminpb.ServiceAccountKey, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if xg := md["x-goog-api-client"]; len(xg) == 0 || !strings.Contains(xg[0], "gl-go/") {
		return nil, fmt.Errorf("x-goog-api-client = %v, expected gl-go key", xg)
	}
	s.reqs = append(s.reqs, req)
	if s.err != nil {
		return nil, s.err
	}
	return s.resps[0].(*adminpb.ServiceAccountKey), nil
}

func (s *mockIamServer) CreateServiceAccountKey(ctx context.Context, req *adminpb.CreateServiceAccountKeyRequest) (*adminpb.ServiceAccountKey, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if xg := md["x-goog-api-client"]; len(xg) == 0 || !strings.Contains(xg[0], "gl-go/") {
		return nil, fmt.Errorf("x-goog-api-client = %v, expected gl-go key", xg)
	}
	s.reqs = append(s.reqs, req)
	if s.err != nil {
		return nil, s.err
	}
	return s.resps[0].(*adminpb.ServiceAccountKey), nil
}

func (s *mockIamServer) DeleteServiceAccountKey(ctx context.Context, req *adminpb.DeleteServiceAccountKeyRequest) (*emptypb.Empty, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if xg := md["x-goog-api-client"]; len(xg) == 0 || !strings.Contains(xg[0], "gl-go/") {
		return nil, fmt.Errorf("x-goog-api-client = %v, expected gl-go key", xg)
	}
	s.reqs = append(s.reqs, req)
	if s.err != nil {
		return nil, s.err
	}
	return s.resps[0].(*emptypb.Empty), nil
}

func (s *mockIamServer) SignBlob(ctx context.Context, req *adminpb.SignBlobRequest) (*adminpb.SignBlobResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if xg := md["x-goog-api-client"]; len(xg) == 0 || !strings.Contains(xg[0], "gl-go/") {
		return nil, fmt.Errorf("x-goog-api-client = %v, expected gl-go key", xg)
	}
	s.reqs = append(s.reqs, req)
	if s.err != nil {
		return nil, s.err
	}
	return s.resps[0].(*adminpb.SignBlobResponse), nil
}

func (s *mockIamServer) SignJwt(ctx context.Context, req *adminpb.SignJwtRequest) (*adminpb.SignJwtResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if xg := md["x-goog-api-client"]; len(xg) == 0 || !strings.Contains(xg[0], "gl-go/") {
		return nil, fmt.Errorf("x-goog-api-client = %v, expected gl-go key", xg)
	}
	s.reqs = append(s.reqs, req)
	if s.err != nil {
		return nil, s.err
	}
	return s.resps[0].(*adminpb.SignJwtResponse), nil
}

func (s *mockIamServer) GetIamPolicy(ctx context.Context, req *iampb.GetIamPolicyRequest) (*iampb.Policy, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if xg := md["x-goog-api-client"]; len(xg) == 0 || !strings.Contains(xg[0], "gl-go/") {
		return nil, fmt.Errorf("x-goog-api-client = %v, expected gl-go key", xg)
	}
	s.reqs = append(s.reqs, req)
	if s.err != nil {
		return nil, s.err
	}
	return s.resps[0].(*iampb.Policy), nil
}

func (s *mockIamServer) SetIamPolicy(ctx context.Context, req *iampb.SetIamPolicyRequest) (*iampb.Policy, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if xg := md["x-goog-api-client"]; len(xg) == 0 || !strings.Contains(xg[0], "gl-go/") {
		return nil, fmt.Errorf("x-goog-api-client = %v, expected gl-go key", xg)
	}
	s.reqs = append(s.reqs, req)
	if s.err != nil {
		return nil, s.err
	}
	return s.resps[0].(*iampb.Policy), nil
}

func (s *mockIamServer) TestIamPermissions(ctx context.Context, req *iampb.TestIamPermissionsRequest) (*iampb.TestIamPermissionsResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if xg := md["x-goog-api-client"]; len(xg) == 0 || !strings.Contains(xg[0], "gl-go/") {
		return nil, fmt.Errorf("x-goog-api-client = %v, expected gl-go key", xg)
	}
	s.reqs = append(s.reqs, req)
	if s.err != nil {
		return nil, s.err
	}
	return s.resps[0].(*iampb.TestIamPermissionsResponse), nil
}

func (s *mockIamServer) QueryGrantableRoles(ctx context.Context, req *adminpb.QueryGrantableRolesRequest) (*adminpb.QueryGrantableRolesResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if xg := md["x-goog-api-client"]; len(xg) == 0 || !strings.Contains(xg[0], "gl-go/") {
		return nil, fmt.Errorf("x-goog-api-client = %v, expected gl-go key", xg)
	}
	s.reqs = append(s.reqs, req)
	if s.err != nil {
		return nil, s.err
	}
	return s.resps[0].(*adminpb.QueryGrantableRolesResponse), nil
}

// clientOpt is the option tests should use to connect to the test server.
// It is initialized by TestMain.
var clientOpt option.ClientOption

var (
	mockIam mockIamServer
)

func TestMain(m *testing.M) {
	flag.Parse()

	serv := grpc.NewServer()
	adminpb.RegisterIAMServer(serv, &mockIam)

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

func TestIamListServiceAccounts(t *testing.T) {
	var nextPageToken string = ""
	var accountsElement *adminpb.ServiceAccount = &adminpb.ServiceAccount{}
	var accounts = []*adminpb.ServiceAccount{accountsElement}
	var expectedResponse = &adminpb.ListServiceAccountsResponse{
		NextPageToken: nextPageToken,
		Accounts:      accounts,
	}

	mockIam.err = nil
	mockIam.reqs = nil

	mockIam.resps = append(mockIam.resps[:0], expectedResponse)

	var formattedName string = fmt.Sprintf("projects/%s", "[PROJECT]")
	var request = &adminpb.ListServiceAccountsRequest{
		Name: formattedName,
	}

	c, err := NewIamClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.ListServiceAccounts(context.Background(), request).Next()

	if err != nil {
		t.Fatal(err)
	}

	if want, got := request, mockIam.reqs[0]; !proto.Equal(want, got) {
		t.Errorf("wrong request %q, want %q", got, want)
	}

	want := (interface{})(expectedResponse.Accounts[0])
	got := (interface{})(resp)
	var ok bool

	switch want := (want).(type) {
	case proto.Message:
		ok = proto.Equal(want, got.(proto.Message))
	default:
		ok = want == got
	}
	if !ok {
		t.Errorf("wrong response %q, want %q)", got, want)
	}
}

func TestIamListServiceAccountsError(t *testing.T) {
	errCode := codes.PermissionDenied
	mockIam.err = gstatus.Error(errCode, "test error")

	var formattedName string = fmt.Sprintf("projects/%s", "[PROJECT]")
	var request = &adminpb.ListServiceAccountsRequest{
		Name: formattedName,
	}

	c, err := NewIamClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.ListServiceAccounts(context.Background(), request).Next()

	if st, ok := gstatus.FromError(err); !ok {
		t.Errorf("got error %v, expected grpc error", err)
	} else if c := st.Code(); c != errCode {
		t.Errorf("got error code %q, want %q", c, errCode)
	}
	_ = resp
}
func TestIamGetServiceAccount(t *testing.T) {
	var name2 string = "name2-1052831874"
	var projectId string = "projectId-1969970175"
	var uniqueId string = "uniqueId-538310583"
	var email string = "email96619420"
	var displayName string = "displayName1615086568"
	var etag []byte = []byte("21")
	var oauth2ClientId string = "oauth2ClientId-1833466037"
	var expectedResponse = &adminpb.ServiceAccount{
		Name:           name2,
		ProjectId:      projectId,
		UniqueId:       uniqueId,
		Email:          email,
		DisplayName:    displayName,
		Etag:           etag,
		Oauth2ClientId: oauth2ClientId,
	}

	mockIam.err = nil
	mockIam.reqs = nil

	mockIam.resps = append(mockIam.resps[:0], expectedResponse)

	var formattedName string = fmt.Sprintf("projects/%s/serviceAccounts/%s", "[PROJECT]", "[SERVICE_ACCOUNT]")
	var request = &adminpb.GetServiceAccountRequest{
		Name: formattedName,
	}

	c, err := NewIamClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.GetServiceAccount(context.Background(), request)

	if err != nil {
		t.Fatal(err)
	}

	if want, got := request, mockIam.reqs[0]; !proto.Equal(want, got) {
		t.Errorf("wrong request %q, want %q", got, want)
	}

	if want, got := expectedResponse, resp; !proto.Equal(want, got) {
		t.Errorf("wrong response %q, want %q)", got, want)
	}
}

func TestIamGetServiceAccountError(t *testing.T) {
	errCode := codes.PermissionDenied
	mockIam.err = gstatus.Error(errCode, "test error")

	var formattedName string = fmt.Sprintf("projects/%s/serviceAccounts/%s", "[PROJECT]", "[SERVICE_ACCOUNT]")
	var request = &adminpb.GetServiceAccountRequest{
		Name: formattedName,
	}

	c, err := NewIamClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.GetServiceAccount(context.Background(), request)

	if st, ok := gstatus.FromError(err); !ok {
		t.Errorf("got error %v, expected grpc error", err)
	} else if c := st.Code(); c != errCode {
		t.Errorf("got error code %q, want %q", c, errCode)
	}
	_ = resp
}
func TestIamCreateServiceAccount(t *testing.T) {
	var name2 string = "name2-1052831874"
	var projectId string = "projectId-1969970175"
	var uniqueId string = "uniqueId-538310583"
	var email string = "email96619420"
	var displayName string = "displayName1615086568"
	var etag []byte = []byte("21")
	var oauth2ClientId string = "oauth2ClientId-1833466037"
	var expectedResponse = &adminpb.ServiceAccount{
		Name:           name2,
		ProjectId:      projectId,
		UniqueId:       uniqueId,
		Email:          email,
		DisplayName:    displayName,
		Etag:           etag,
		Oauth2ClientId: oauth2ClientId,
	}

	mockIam.err = nil
	mockIam.reqs = nil

	mockIam.resps = append(mockIam.resps[:0], expectedResponse)

	var formattedName string = fmt.Sprintf("projects/%s", "[PROJECT]")
	var accountId string = "accountId-803333011"
	var request = &adminpb.CreateServiceAccountRequest{
		Name:      formattedName,
		AccountId: accountId,
	}

	c, err := NewIamClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.CreateServiceAccount(context.Background(), request)

	if err != nil {
		t.Fatal(err)
	}

	if want, got := request, mockIam.reqs[0]; !proto.Equal(want, got) {
		t.Errorf("wrong request %q, want %q", got, want)
	}

	if want, got := expectedResponse, resp; !proto.Equal(want, got) {
		t.Errorf("wrong response %q, want %q)", got, want)
	}
}

func TestIamCreateServiceAccountError(t *testing.T) {
	errCode := codes.PermissionDenied
	mockIam.err = gstatus.Error(errCode, "test error")

	var formattedName string = fmt.Sprintf("projects/%s", "[PROJECT]")
	var accountId string = "accountId-803333011"
	var request = &adminpb.CreateServiceAccountRequest{
		Name:      formattedName,
		AccountId: accountId,
	}

	c, err := NewIamClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.CreateServiceAccount(context.Background(), request)

	if st, ok := gstatus.FromError(err); !ok {
		t.Errorf("got error %v, expected grpc error", err)
	} else if c := st.Code(); c != errCode {
		t.Errorf("got error code %q, want %q", c, errCode)
	}
	_ = resp
}
func TestIamUpdateServiceAccount(t *testing.T) {
	var name string = "name3373707"
	var projectId string = "projectId-1969970175"
	var uniqueId string = "uniqueId-538310583"
	var email string = "email96619420"
	var displayName string = "displayName1615086568"
	var etag2 []byte = []byte("-120")
	var oauth2ClientId string = "oauth2ClientId-1833466037"
	var expectedResponse = &adminpb.ServiceAccount{
		Name:           name,
		ProjectId:      projectId,
		UniqueId:       uniqueId,
		Email:          email,
		DisplayName:    displayName,
		Etag:           etag2,
		Oauth2ClientId: oauth2ClientId,
	}

	mockIam.err = nil
	mockIam.reqs = nil

	mockIam.resps = append(mockIam.resps[:0], expectedResponse)

	var etag []byte = []byte("21")
	var request = &adminpb.ServiceAccount{
		Etag: etag,
	}

	c, err := NewIamClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.UpdateServiceAccount(context.Background(), request)

	if err != nil {
		t.Fatal(err)
	}

	if want, got := request, mockIam.reqs[0]; !proto.Equal(want, got) {
		t.Errorf("wrong request %q, want %q", got, want)
	}

	if want, got := expectedResponse, resp; !proto.Equal(want, got) {
		t.Errorf("wrong response %q, want %q)", got, want)
	}
}

func TestIamUpdateServiceAccountError(t *testing.T) {
	errCode := codes.PermissionDenied
	mockIam.err = gstatus.Error(errCode, "test error")

	var etag []byte = []byte("21")
	var request = &adminpb.ServiceAccount{
		Etag: etag,
	}

	c, err := NewIamClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.UpdateServiceAccount(context.Background(), request)

	if st, ok := gstatus.FromError(err); !ok {
		t.Errorf("got error %v, expected grpc error", err)
	} else if c := st.Code(); c != errCode {
		t.Errorf("got error code %q, want %q", c, errCode)
	}
	_ = resp
}
func TestIamDeleteServiceAccount(t *testing.T) {
	var expectedResponse *emptypb.Empty = &emptypb.Empty{}

	mockIam.err = nil
	mockIam.reqs = nil

	mockIam.resps = append(mockIam.resps[:0], expectedResponse)

	var formattedName string = fmt.Sprintf("projects/%s/serviceAccounts/%s", "[PROJECT]", "[SERVICE_ACCOUNT]")
	var request = &adminpb.DeleteServiceAccountRequest{
		Name: formattedName,
	}

	c, err := NewIamClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	err = c.DeleteServiceAccount(context.Background(), request)

	if err != nil {
		t.Fatal(err)
	}

	if want, got := request, mockIam.reqs[0]; !proto.Equal(want, got) {
		t.Errorf("wrong request %q, want %q", got, want)
	}

}

func TestIamDeleteServiceAccountError(t *testing.T) {
	errCode := codes.PermissionDenied
	mockIam.err = gstatus.Error(errCode, "test error")

	var formattedName string = fmt.Sprintf("projects/%s/serviceAccounts/%s", "[PROJECT]", "[SERVICE_ACCOUNT]")
	var request = &adminpb.DeleteServiceAccountRequest{
		Name: formattedName,
	}

	c, err := NewIamClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	err = c.DeleteServiceAccount(context.Background(), request)

	if st, ok := gstatus.FromError(err); !ok {
		t.Errorf("got error %v, expected grpc error", err)
	} else if c := st.Code(); c != errCode {
		t.Errorf("got error code %q, want %q", c, errCode)
	}
}
func TestIamListServiceAccountKeys(t *testing.T) {
	var expectedResponse *adminpb.ListServiceAccountKeysResponse = &adminpb.ListServiceAccountKeysResponse{}

	mockIam.err = nil
	mockIam.reqs = nil

	mockIam.resps = append(mockIam.resps[:0], expectedResponse)

	var formattedName string = fmt.Sprintf("projects/%s/serviceAccounts/%s", "[PROJECT]", "[SERVICE_ACCOUNT]")
	var request = &adminpb.ListServiceAccountKeysRequest{
		Name: formattedName,
	}

	c, err := NewIamClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.ListServiceAccountKeys(context.Background(), request)

	if err != nil {
		t.Fatal(err)
	}

	if want, got := request, mockIam.reqs[0]; !proto.Equal(want, got) {
		t.Errorf("wrong request %q, want %q", got, want)
	}

	if want, got := expectedResponse, resp; !proto.Equal(want, got) {
		t.Errorf("wrong response %q, want %q)", got, want)
	}
}

func TestIamListServiceAccountKeysError(t *testing.T) {
	errCode := codes.PermissionDenied
	mockIam.err = gstatus.Error(errCode, "test error")

	var formattedName string = fmt.Sprintf("projects/%s/serviceAccounts/%s", "[PROJECT]", "[SERVICE_ACCOUNT]")
	var request = &adminpb.ListServiceAccountKeysRequest{
		Name: formattedName,
	}

	c, err := NewIamClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.ListServiceAccountKeys(context.Background(), request)

	if st, ok := gstatus.FromError(err); !ok {
		t.Errorf("got error %v, expected grpc error", err)
	} else if c := st.Code(); c != errCode {
		t.Errorf("got error code %q, want %q", c, errCode)
	}
	_ = resp
}
func TestIamGetServiceAccountKey(t *testing.T) {
	var name2 string = "name2-1052831874"
	var privateKeyData []byte = []byte("-58")
	var publicKeyData []byte = []byte("-96")
	var expectedResponse = &adminpb.ServiceAccountKey{
		Name:           name2,
		PrivateKeyData: privateKeyData,
		PublicKeyData:  publicKeyData,
	}

	mockIam.err = nil
	mockIam.reqs = nil

	mockIam.resps = append(mockIam.resps[:0], expectedResponse)

	var formattedName string = fmt.Sprintf("projects/%s/serviceAccounts/%s/keys/%s", "[PROJECT]", "[SERVICE_ACCOUNT]", "[KEY]")
	var request = &adminpb.GetServiceAccountKeyRequest{
		Name: formattedName,
	}

	c, err := NewIamClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.GetServiceAccountKey(context.Background(), request)

	if err != nil {
		t.Fatal(err)
	}

	if want, got := request, mockIam.reqs[0]; !proto.Equal(want, got) {
		t.Errorf("wrong request %q, want %q", got, want)
	}

	if want, got := expectedResponse, resp; !proto.Equal(want, got) {
		t.Errorf("wrong response %q, want %q)", got, want)
	}
}

func TestIamGetServiceAccountKeyError(t *testing.T) {
	errCode := codes.PermissionDenied
	mockIam.err = gstatus.Error(errCode, "test error")

	var formattedName string = fmt.Sprintf("projects/%s/serviceAccounts/%s/keys/%s", "[PROJECT]", "[SERVICE_ACCOUNT]", "[KEY]")
	var request = &adminpb.GetServiceAccountKeyRequest{
		Name: formattedName,
	}

	c, err := NewIamClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.GetServiceAccountKey(context.Background(), request)

	if st, ok := gstatus.FromError(err); !ok {
		t.Errorf("got error %v, expected grpc error", err)
	} else if c := st.Code(); c != errCode {
		t.Errorf("got error code %q, want %q", c, errCode)
	}
	_ = resp
}
func TestIamCreateServiceAccountKey(t *testing.T) {
	var name2 string = "name2-1052831874"
	var privateKeyData []byte = []byte("-58")
	var publicKeyData []byte = []byte("-96")
	var expectedResponse = &adminpb.ServiceAccountKey{
		Name:           name2,
		PrivateKeyData: privateKeyData,
		PublicKeyData:  publicKeyData,
	}

	mockIam.err = nil
	mockIam.reqs = nil

	mockIam.resps = append(mockIam.resps[:0], expectedResponse)

	var formattedName string = fmt.Sprintf("projects/%s/serviceAccounts/%s", "[PROJECT]", "[SERVICE_ACCOUNT]")
	var request = &adminpb.CreateServiceAccountKeyRequest{
		Name: formattedName,
	}

	c, err := NewIamClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.CreateServiceAccountKey(context.Background(), request)

	if err != nil {
		t.Fatal(err)
	}

	if want, got := request, mockIam.reqs[0]; !proto.Equal(want, got) {
		t.Errorf("wrong request %q, want %q", got, want)
	}

	if want, got := expectedResponse, resp; !proto.Equal(want, got) {
		t.Errorf("wrong response %q, want %q)", got, want)
	}
}

func TestIamCreateServiceAccountKeyError(t *testing.T) {
	errCode := codes.PermissionDenied
	mockIam.err = gstatus.Error(errCode, "test error")

	var formattedName string = fmt.Sprintf("projects/%s/serviceAccounts/%s", "[PROJECT]", "[SERVICE_ACCOUNT]")
	var request = &adminpb.CreateServiceAccountKeyRequest{
		Name: formattedName,
	}

	c, err := NewIamClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.CreateServiceAccountKey(context.Background(), request)

	if st, ok := gstatus.FromError(err); !ok {
		t.Errorf("got error %v, expected grpc error", err)
	} else if c := st.Code(); c != errCode {
		t.Errorf("got error code %q, want %q", c, errCode)
	}
	_ = resp
}
func TestIamDeleteServiceAccountKey(t *testing.T) {
	var expectedResponse *emptypb.Empty = &emptypb.Empty{}

	mockIam.err = nil
	mockIam.reqs = nil

	mockIam.resps = append(mockIam.resps[:0], expectedResponse)

	var formattedName string = fmt.Sprintf("projects/%s/serviceAccounts/%s/keys/%s", "[PROJECT]", "[SERVICE_ACCOUNT]", "[KEY]")
	var request = &adminpb.DeleteServiceAccountKeyRequest{
		Name: formattedName,
	}

	c, err := NewIamClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	err = c.DeleteServiceAccountKey(context.Background(), request)

	if err != nil {
		t.Fatal(err)
	}

	if want, got := request, mockIam.reqs[0]; !proto.Equal(want, got) {
		t.Errorf("wrong request %q, want %q", got, want)
	}

}

func TestIamDeleteServiceAccountKeyError(t *testing.T) {
	errCode := codes.PermissionDenied
	mockIam.err = gstatus.Error(errCode, "test error")

	var formattedName string = fmt.Sprintf("projects/%s/serviceAccounts/%s/keys/%s", "[PROJECT]", "[SERVICE_ACCOUNT]", "[KEY]")
	var request = &adminpb.DeleteServiceAccountKeyRequest{
		Name: formattedName,
	}

	c, err := NewIamClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	err = c.DeleteServiceAccountKey(context.Background(), request)

	if st, ok := gstatus.FromError(err); !ok {
		t.Errorf("got error %v, expected grpc error", err)
	} else if c := st.Code(); c != errCode {
		t.Errorf("got error code %q, want %q", c, errCode)
	}
}
func TestIamSignBlob(t *testing.T) {
	var keyId string = "keyId-1134673157"
	var signature []byte = []byte("-72")
	var expectedResponse = &adminpb.SignBlobResponse{
		KeyId:     keyId,
		Signature: signature,
	}

	mockIam.err = nil
	mockIam.reqs = nil

	mockIam.resps = append(mockIam.resps[:0], expectedResponse)

	var formattedName string = fmt.Sprintf("projects/%s/serviceAccounts/%s", "[PROJECT]", "[SERVICE_ACCOUNT]")
	var bytesToSign []byte = []byte("45")
	var request = &adminpb.SignBlobRequest{
		Name:        formattedName,
		BytesToSign: bytesToSign,
	}

	c, err := NewIamClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.SignBlob(context.Background(), request)

	if err != nil {
		t.Fatal(err)
	}

	if want, got := request, mockIam.reqs[0]; !proto.Equal(want, got) {
		t.Errorf("wrong request %q, want %q", got, want)
	}

	if want, got := expectedResponse, resp; !proto.Equal(want, got) {
		t.Errorf("wrong response %q, want %q)", got, want)
	}
}

func TestIamSignBlobError(t *testing.T) {
	errCode := codes.PermissionDenied
	mockIam.err = gstatus.Error(errCode, "test error")

	var formattedName string = fmt.Sprintf("projects/%s/serviceAccounts/%s", "[PROJECT]", "[SERVICE_ACCOUNT]")
	var bytesToSign []byte = []byte("45")
	var request = &adminpb.SignBlobRequest{
		Name:        formattedName,
		BytesToSign: bytesToSign,
	}

	c, err := NewIamClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.SignBlob(context.Background(), request)

	if st, ok := gstatus.FromError(err); !ok {
		t.Errorf("got error %v, expected grpc error", err)
	} else if c := st.Code(); c != errCode {
		t.Errorf("got error code %q, want %q", c, errCode)
	}
	_ = resp
}
func TestIamGetIamPolicy(t *testing.T) {
	var version int32 = 351608024
	var etag []byte = []byte("21")
	var expectedResponse = &iampb.Policy{
		Version: version,
		Etag:    etag,
	}

	mockIam.err = nil
	mockIam.reqs = nil

	mockIam.resps = append(mockIam.resps[:0], expectedResponse)

	var formattedResource string = fmt.Sprintf("projects/%s/serviceAccounts/%s", "[PROJECT]", "[SERVICE_ACCOUNT]")
	var request = &iampb.GetIamPolicyRequest{
		Resource: formattedResource,
	}

	c, err := NewIamClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.getIamPolicy(context.Background(), request)

	if err != nil {
		t.Fatal(err)
	}

	if want, got := request, mockIam.reqs[0]; !proto.Equal(want, got) {
		t.Errorf("wrong request %q, want %q", got, want)
	}

	if want, got := expectedResponse, resp; !proto.Equal(want, got) {
		t.Errorf("wrong response %q, want %q)", got, want)
	}
}

func TestIamGetIamPolicyError(t *testing.T) {
	errCode := codes.PermissionDenied
	mockIam.err = gstatus.Error(errCode, "test error")

	var formattedResource string = fmt.Sprintf("projects/%s/serviceAccounts/%s", "[PROJECT]", "[SERVICE_ACCOUNT]")
	var request = &iampb.GetIamPolicyRequest{
		Resource: formattedResource,
	}

	c, err := NewIamClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.getIamPolicy(context.Background(), request)

	if st, ok := gstatus.FromError(err); !ok {
		t.Errorf("got error %v, expected grpc error", err)
	} else if c := st.Code(); c != errCode {
		t.Errorf("got error code %q, want %q", c, errCode)
	}
	_ = resp
}
func TestIamSetIamPolicy(t *testing.T) {
	var version int32 = 351608024
	var etag []byte = []byte("21")
	var expectedResponse = &iampb.Policy{
		Version: version,
		Etag:    etag,
	}

	mockIam.err = nil
	mockIam.reqs = nil

	mockIam.resps = append(mockIam.resps[:0], expectedResponse)

	var formattedResource string = fmt.Sprintf("projects/%s/serviceAccounts/%s", "[PROJECT]", "[SERVICE_ACCOUNT]")
	var policy *iampb.Policy = &iampb.Policy{}
	var request = &iampb.SetIamPolicyRequest{
		Resource: formattedResource,
		Policy:   policy,
	}

	c, err := NewIamClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.setIamPolicy(context.Background(), request)

	if err != nil {
		t.Fatal(err)
	}

	if want, got := request, mockIam.reqs[0]; !proto.Equal(want, got) {
		t.Errorf("wrong request %q, want %q", got, want)
	}

	if want, got := expectedResponse, resp; !proto.Equal(want, got) {
		t.Errorf("wrong response %q, want %q)", got, want)
	}
}

func TestIamSetIamPolicyError(t *testing.T) {
	errCode := codes.PermissionDenied
	mockIam.err = gstatus.Error(errCode, "test error")

	var formattedResource string = fmt.Sprintf("projects/%s/serviceAccounts/%s", "[PROJECT]", "[SERVICE_ACCOUNT]")
	var policy *iampb.Policy = &iampb.Policy{}
	var request = &iampb.SetIamPolicyRequest{
		Resource: formattedResource,
		Policy:   policy,
	}

	c, err := NewIamClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.setIamPolicy(context.Background(), request)

	if st, ok := gstatus.FromError(err); !ok {
		t.Errorf("got error %v, expected grpc error", err)
	} else if c := st.Code(); c != errCode {
		t.Errorf("got error code %q, want %q", c, errCode)
	}
	_ = resp
}
func TestIamTestIamPermissions(t *testing.T) {
	var expectedResponse *iampb.TestIamPermissionsResponse = &iampb.TestIamPermissionsResponse{}

	mockIam.err = nil
	mockIam.reqs = nil

	mockIam.resps = append(mockIam.resps[:0], expectedResponse)

	var formattedResource string = fmt.Sprintf("projects/%s/serviceAccounts/%s", "[PROJECT]", "[SERVICE_ACCOUNT]")
	var permissions []string = nil
	var request = &iampb.TestIamPermissionsRequest{
		Resource:    formattedResource,
		Permissions: permissions,
	}

	c, err := NewIamClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.TestIamPermissions(context.Background(), request)

	if err != nil {
		t.Fatal(err)
	}

	if want, got := request, mockIam.reqs[0]; !proto.Equal(want, got) {
		t.Errorf("wrong request %q, want %q", got, want)
	}

	if want, got := expectedResponse, resp; !proto.Equal(want, got) {
		t.Errorf("wrong response %q, want %q)", got, want)
	}
}

func TestIamTestIamPermissionsError(t *testing.T) {
	errCode := codes.PermissionDenied
	mockIam.err = gstatus.Error(errCode, "test error")

	var formattedResource string = fmt.Sprintf("projects/%s/serviceAccounts/%s", "[PROJECT]", "[SERVICE_ACCOUNT]")
	var permissions []string = nil
	var request = &iampb.TestIamPermissionsRequest{
		Resource:    formattedResource,
		Permissions: permissions,
	}

	c, err := NewIamClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.TestIamPermissions(context.Background(), request)

	if st, ok := gstatus.FromError(err); !ok {
		t.Errorf("got error %v, expected grpc error", err)
	} else if c := st.Code(); c != errCode {
		t.Errorf("got error code %q, want %q", c, errCode)
	}
	_ = resp
}
func TestIamQueryGrantableRoles(t *testing.T) {
	var nextPageToken string = "nextPageToken-1530815211"
	var expectedResponse = &adminpb.QueryGrantableRolesResponse{
		NextPageToken: nextPageToken,
	}

	mockIam.err = nil
	mockIam.reqs = nil

	mockIam.resps = append(mockIam.resps[:0], expectedResponse)

	var fullResourceName string = "fullResourceName1300993644"
	var request = &adminpb.QueryGrantableRolesRequest{
		FullResourceName: fullResourceName,
	}

	c, err := NewIamClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.QueryGrantableRoles(context.Background(), request)

	if err != nil {
		t.Fatal(err)
	}

	if want, got := request, mockIam.reqs[0]; !proto.Equal(want, got) {
		t.Errorf("wrong request %q, want %q", got, want)
	}

	if want, got := expectedResponse, resp; !proto.Equal(want, got) {
		t.Errorf("wrong response %q, want %q)", got, want)
	}
}

func TestIamQueryGrantableRolesError(t *testing.T) {
	errCode := codes.PermissionDenied
	mockIam.err = gstatus.Error(errCode, "test error")

	var fullResourceName string = "fullResourceName1300993644"
	var request = &adminpb.QueryGrantableRolesRequest{
		FullResourceName: fullResourceName,
	}

	c, err := NewIamClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.QueryGrantableRoles(context.Background(), request)

	if st, ok := gstatus.FromError(err); !ok {
		t.Errorf("got error %v, expected grpc error", err)
	} else if c := st.Code(); c != errCode {
		t.Errorf("got error code %q, want %q", c, errCode)
	}
	_ = resp
}
func TestIamSignJwt(t *testing.T) {
	var keyId string = "keyId-1134673157"
	var signedJwt string = "signedJwt-979546844"
	var expectedResponse = &adminpb.SignJwtResponse{
		KeyId:     keyId,
		SignedJwt: signedJwt,
	}

	mockIam.err = nil
	mockIam.reqs = nil

	mockIam.resps = append(mockIam.resps[:0], expectedResponse)

	var name string = "name3373707"
	var payload string = "payload-786701938"
	var request = &adminpb.SignJwtRequest{
		Name:    name,
		Payload: payload,
	}

	c, err := NewIamClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.SignJwt(context.Background(), request)

	if err != nil {
		t.Fatal(err)
	}

	if want, got := request, mockIam.reqs[0]; !proto.Equal(want, got) {
		t.Errorf("wrong request %q, want %q", got, want)
	}

	if want, got := expectedResponse, resp; !proto.Equal(want, got) {
		t.Errorf("wrong response %q, want %q)", got, want)
	}
}

func TestIamSignJwtError(t *testing.T) {
	errCode := codes.PermissionDenied
	mockIam.err = gstatus.Error(errCode, "test error")

	var name string = "name3373707"
	var payload string = "payload-786701938"
	var request = &adminpb.SignJwtRequest{
		Name:    name,
		Payload: payload,
	}

	c, err := NewIamClient(context.Background(), clientOpt)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.SignJwt(context.Background(), request)

	if st, ok := gstatus.FromError(err); !ok {
		t.Errorf("got error %v, expected grpc error", err)
	} else if c := st.Code(); c != errCode {
		t.Errorf("got error code %q, want %q", c, errCode)
	}
	_ = resp
}
