package s3

import (
	"context"
	"net/http"
	"time"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/aws/aws-sdk-go-v2/aws"
	v4signer "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

// Authenticator defines an interface for obtaining an IAM token.
type Authenticator interface {
	GetToken() (string, error)
}

// IbmIamSigner is a structure for signing requests using IBM IAM.
// Requeres APIKey and Resource InstanceID
type IbmIamSigner struct {
	APIKey     string
	InstanceID string
	Auth       Authenticator
}

// SignHTTP signs requests using IBM IAM token.
func (signer *IbmIamSigner) SignHTTP(ctx context.Context, credentials aws.Credentials, req *http.Request, payloadHash string, service string, region string, signingTime time.Time, optFns ...func(*v4signer.SignerOptions)) error {
	var authenticator Authenticator
	if signer.Auth != nil {
		authenticator = signer.Auth
	} else {
		authenticator = &core.IamAuthenticator{ApiKey: signer.APIKey}
	}
	token, err := authenticator.GetToken()
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("ibm-service-instance-id", signer.InstanceID)
	return nil
}

// NoOpCredentialsProvider is needed since S3 SDK requires having credentials, even though authentication is happening via IBM IAM.
type NoOpCredentialsProvider struct{}

// Retrieve returns mock credentials for the NoOpCredentialsProvider.
func (n *NoOpCredentialsProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	return aws.Credentials{
		AccessKeyID:     "NoOpAccessKey",
		SecretAccessKey: "NoOpSecretKey",
		SessionToken:    "",
		Source:          "NoOpCredentialsProvider",
	}, nil
}

// IsExpired always returns false
func (n *NoOpCredentialsProvider) IsExpired() bool {
	return false
}
