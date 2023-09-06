//go:build go1.18
// +build go1.18

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package azidentity

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/internal/log"
)

// DefaultAzureCredentialOptions contains optional parameters for DefaultAzureCredential.
// These options may not apply to all credentials in the chain.
type DefaultAzureCredentialOptions struct {
	azcore.ClientOptions

	// AdditionallyAllowedTenants specifies additional tenants for which the credential may acquire tokens. Add
	// the wildcard value "*" to allow the credential to acquire tokens for any tenant. This value can also be
	// set as a semicolon delimited list of tenants in the environment variable AZURE_ADDITIONALLY_ALLOWED_TENANTS.
	AdditionallyAllowedTenants []string
	// DisableInstanceDiscovery should be set true only by applications authenticating in disconnected clouds, or
	// private clouds such as Azure Stack. It determines whether the credential requests Azure AD instance metadata
	// from https://login.microsoft.com before authenticating. Setting this to true will skip this request, making
	// the application responsible for ensuring the configured authority is valid and trustworthy.
	DisableInstanceDiscovery bool
	// TenantID identifies the tenant the Azure CLI should authenticate in.
	// Defaults to the CLI's default tenant, which is typically the home tenant of the user logged in to the CLI.
	TenantID string
}

// DefaultAzureCredential is a default credential chain for applications that will deploy to Azure.
// It combines credentials suitable for deployment with credentials suitable for local development.
// It attempts to authenticate with each of these credential types, in the following order, stopping
// when one provides a token:
//
//   - [EnvironmentCredential]
//   - [WorkloadIdentityCredential], if environment variable configuration is set by the Azure workload
//     identity webhook. Use [WorkloadIdentityCredential] directly when not using the webhook or needing
//     more control over its configuration.
//   - [ManagedIdentityCredential]
//   - [AzureCLICredential]
//
// Consult the documentation for these credential types for more information on how they authenticate.
// Once a credential has successfully authenticated, DefaultAzureCredential will use that credential for
// every subsequent authentication.
type DefaultAzureCredential struct {
	chain *ChainedTokenCredential
}

// NewDefaultAzureCredential creates a DefaultAzureCredential. Pass nil for options to accept defaults.
func NewDefaultAzureCredential(options *DefaultAzureCredentialOptions) (*DefaultAzureCredential, error) {
	var creds []azcore.TokenCredential
	var errorMessages []string

	if options == nil {
		options = &DefaultAzureCredentialOptions{}
	}
	additionalTenants := options.AdditionallyAllowedTenants
	if len(additionalTenants) == 0 {
		if tenants := os.Getenv(azureAdditionallyAllowedTenants); tenants != "" {
			additionalTenants = strings.Split(tenants, ";")
		}
	}

	envCred, err := NewEnvironmentCredential(&EnvironmentCredentialOptions{
		ClientOptions:              options.ClientOptions,
		DisableInstanceDiscovery:   options.DisableInstanceDiscovery,
		additionallyAllowedTenants: additionalTenants,
	})
	if err == nil {
		creds = append(creds, envCred)
	} else {
		errorMessages = append(errorMessages, "EnvironmentCredential: "+err.Error())
		creds = append(creds, &defaultCredentialErrorReporter{credType: "EnvironmentCredential", err: err})
	}

	// workload identity requires values for AZURE_AUTHORITY_HOST, AZURE_CLIENT_ID, AZURE_FEDERATED_TOKEN_FILE, AZURE_TENANT_ID
	wic, err := NewWorkloadIdentityCredential(&WorkloadIdentityCredentialOptions{
		AdditionallyAllowedTenants: additionalTenants,
		ClientOptions:              options.ClientOptions,
		DisableInstanceDiscovery:   options.DisableInstanceDiscovery,
	})
	if err == nil {
		creds = append(creds, wic)
	} else {
		errorMessages = append(errorMessages, credNameWorkloadIdentity+": "+err.Error())
		creds = append(creds, &defaultCredentialErrorReporter{credType: credNameWorkloadIdentity, err: err})
	}
	o := &ManagedIdentityCredentialOptions{ClientOptions: options.ClientOptions}
	if ID, ok := os.LookupEnv(azureClientID); ok {
		o.ID = ClientID(ID)
	}
	miCred, err := NewManagedIdentityCredential(o)
	if err == nil {
		creds = append(creds, &timeoutWrapper{mic: miCred, timeout: time.Second})
	} else {
		errorMessages = append(errorMessages, credNameManagedIdentity+": "+err.Error())
		creds = append(creds, &defaultCredentialErrorReporter{credType: credNameManagedIdentity, err: err})
	}

	cliCred, err := NewAzureCLICredential(&AzureCLICredentialOptions{AdditionallyAllowedTenants: additionalTenants, TenantID: options.TenantID})
	if err == nil {
		creds = append(creds, cliCred)
	} else {
		errorMessages = append(errorMessages, credNameAzureCLI+": "+err.Error())
		creds = append(creds, &defaultCredentialErrorReporter{credType: credNameAzureCLI, err: err})
	}

	err = defaultAzureCredentialConstructorErrorHandler(len(creds), errorMessages)
	if err != nil {
		return nil, err
	}

	chain, err := NewChainedTokenCredential(creds, nil)
	if err != nil {
		return nil, err
	}
	chain.name = "DefaultAzureCredential"
	return &DefaultAzureCredential{chain: chain}, nil
}

// GetToken requests an access token from Azure Active Directory. This method is called automatically by Azure SDK clients.
func (c *DefaultAzureCredential) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return c.chain.GetToken(ctx, opts)
}

var _ azcore.TokenCredential = (*DefaultAzureCredential)(nil)

func defaultAzureCredentialConstructorErrorHandler(numberOfSuccessfulCredentials int, errorMessages []string) (err error) {
	errorMessage := strings.Join(errorMessages, "\n\t")

	if numberOfSuccessfulCredentials == 0 {
		return errors.New(errorMessage)
	}

	if len(errorMessages) != 0 {
		log.Writef(EventAuthentication, "NewDefaultAzureCredential failed to initialize some credentials:\n\t%s", errorMessage)
	}

	return nil
}

// defaultCredentialErrorReporter is a substitute for credentials that couldn't be constructed.
// Its GetToken method always returns a credentialUnavailableError having the same message as
// the error that prevented constructing the credential. This ensures the message is present
// in the error returned by ChainedTokenCredential.GetToken()
type defaultCredentialErrorReporter struct {
	credType string
	err      error
}

func (d *defaultCredentialErrorReporter) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	if _, ok := d.err.(*credentialUnavailableError); ok {
		return azcore.AccessToken{}, d.err
	}
	return azcore.AccessToken{}, newCredentialUnavailableError(d.credType, d.err.Error())
}

var _ azcore.TokenCredential = (*defaultCredentialErrorReporter)(nil)

// timeoutWrapper prevents a potentially very long timeout when managed identity isn't available
type timeoutWrapper struct {
	mic *ManagedIdentityCredential
	// timeout applies to all auth attempts until one doesn't time out
	timeout time.Duration
}

// GetToken wraps DefaultAzureCredential's initial managed identity auth attempt with a short timeout
// because managed identity may not be available and connecting to IMDS can take several minutes to time out.
func (w *timeoutWrapper) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	var tk azcore.AccessToken
	var err error
	// no need to synchronize around this value because it's written only within ChainedTokenCredential's critical section
	if w.timeout > 0 {
		c, cancel := context.WithTimeout(ctx, w.timeout)
		defer cancel()
		tk, err = w.mic.GetToken(c, opts)
		if isAuthFailedDueToContext(err) {
			err = newCredentialUnavailableError(credNameManagedIdentity, "managed identity timed out")
		} else {
			// some managed identity implementation is available, so don't apply the timeout to future calls
			w.timeout = 0
		}
	} else {
		tk, err = w.mic.GetToken(ctx, opts)
	}
	return tk, err
}

// unwraps nested AuthenticationFailedErrors to get the root error
func isAuthFailedDueToContext(err error) bool {
	for {
		var authFailedErr *AuthenticationFailedError
		if !errors.As(err, &authFailedErr) {
			break
		}
		err = authFailedErr.err
	}
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
