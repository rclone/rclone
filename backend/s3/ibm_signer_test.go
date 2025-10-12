package s3

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
)

type MockAuthenticator struct {
	Token string
	Error error
}

func (m *MockAuthenticator) GetToken() (string, error) {
	return m.Token, m.Error
}

func TestSignHTTP(t *testing.T) {
	apiKey := "mock-api-key"
	instanceID := "mock-instance-id"
	token := "mock-iam-token"
	mockAuth := &MockAuthenticator{
		Token: token,
		Error: nil,
	}
	signer := &IbmIamSigner{
		APIKey:     apiKey,
		InstanceID: instanceID,
		Auth:       mockAuth,
	}
	req, err := http.NewRequest("GET", "https://example.com", nil)
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}
	credentials := aws.Credentials{
		AccessKeyID:     "mock-access-key",
		SecretAccessKey: "mock-secret-key",
	}
	err = signer.SignHTTP(context.TODO(), credentials, req, "payload-hash", "service", "region", time.Now())
	assert.NoError(t, err, "Expected no error")
	assert.Equal(t, "Bearer "+token, req.Header.Get("Authorization"), "Authorization header should be set correctly")
	assert.Equal(t, instanceID, req.Header.Get("ibm-service-instance-id"), "ibm-service-instance-id header should be set correctly")
}
