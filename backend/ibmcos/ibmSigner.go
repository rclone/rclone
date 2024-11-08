package ibmcos

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/aws/aws-sdk-go-v2/aws"
	v4signer "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

// IBM IAM signer structure. Requeres APIKye and Resource InstanceID
type IbmIamSigner struct {
	APIKey     string
	InstanceID string
}

// SignHTTP signs requests using IBM IAM token
func (signer *IbmIamSigner) SignHTTP(ctx context.Context, credentials aws.Credentials, req *http.Request, payloadHash string, service string, region string, signingTime time.Time, optFns ...func(*v4signer.SignerOptions)) error {
	fmt.Println("SignHTTP called") // Debug print
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
