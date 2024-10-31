package s3

import (
	"context"
	"net/http"
	"time"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/aws/aws-sdk-go-v2/aws"
	v4signer "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

// IBM IAM signer structure. Requeres APIKey and Resource InstanceID
type IbmIamSigner struct {
	APIKey     string
	InstanceID string
}

// SignHTTP signs requests using IBM IAM token
func (signer *IbmIamSigner) SignHTTP(ctx context.Context, credentials aws.Credentials, req *http.Request, payloadHash string, service string, region string, signingTime time.Time, optFns ...func(*v4signer.SignerOptions)) error {
	authenticator := &core.IamAuthenticator{
		ApiKey: signer.APIKey,
	}
	token, err := authenticator.GetToken()
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("ibm-service-instance-id", signer.InstanceID)
	return nil
}

// This is needed since S3 SDK requires having credentials, eventhough authentication is happening via IBM IAM
type NoOpCredentialsProvider struct{}

func (n *NoOpCredentialsProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	return aws.Credentials{
		AccessKeyID:     "NoOpAccessKey",
		SecretAccessKey: "NoOpSecretKey",
		SessionToken:    "",
		Source:          "NoOpCredentialsProvider",
	}, nil
}

func (n *NoOpCredentialsProvider) IsExpired() bool {
	return false
}
