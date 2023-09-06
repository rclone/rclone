//go:build go1.18
// +build go1.18

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package azidentity

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/confidential"
)

const credNameSecret = "ClientSecretCredential"

// ClientSecretCredentialOptions contains optional parameters for ClientSecretCredential.
type ClientSecretCredentialOptions struct {
	azcore.ClientOptions

	// AdditionallyAllowedTenants specifies additional tenants for which the credential may acquire tokens.
	// Add the wildcard value "*" to allow the credential to acquire tokens for any tenant in which the
	// application is registered.
	AdditionallyAllowedTenants []string
	// DisableInstanceDiscovery should be set true only by applications authenticating in disconnected clouds, or
	// private clouds such as Azure Stack. It determines whether the credential requests Azure AD instance metadata
	// from https://login.microsoft.com before authenticating. Setting this to true will skip this request, making
	// the application responsible for ensuring the configured authority is valid and trustworthy.
	DisableInstanceDiscovery bool
}

// ClientSecretCredential authenticates an application with a client secret.
type ClientSecretCredential struct {
	client confidentialClient
	s      *syncer
}

// NewClientSecretCredential constructs a ClientSecretCredential. Pass nil for options to accept defaults.
func NewClientSecretCredential(tenantID string, clientID string, clientSecret string, options *ClientSecretCredentialOptions) (*ClientSecretCredential, error) {
	if options == nil {
		options = &ClientSecretCredentialOptions{}
	}
	cred, err := confidential.NewCredFromSecret(clientSecret)
	if err != nil {
		return nil, err
	}
	c, err := getConfidentialClient(
		clientID, tenantID, cred, &options.ClientOptions, confidential.WithInstanceDiscovery(!options.DisableInstanceDiscovery),
	)
	if err != nil {
		return nil, err
	}
	csc := ClientSecretCredential{client: c}
	csc.s = newSyncer(credNameSecret, tenantID, options.AdditionallyAllowedTenants, csc.requestToken, csc.silentAuth)
	return &csc, nil
}

// GetToken requests an access token from Azure Active Directory. This method is called automatically by Azure SDK clients.
func (c *ClientSecretCredential) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return c.s.GetToken(ctx, opts)
}

func (c *ClientSecretCredential) silentAuth(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	ar, err := c.client.AcquireTokenSilent(ctx, opts.Scopes, confidential.WithTenantID(opts.TenantID))
	return azcore.AccessToken{Token: ar.AccessToken, ExpiresOn: ar.ExpiresOn.UTC()}, err
}

func (c *ClientSecretCredential) requestToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	ar, err := c.client.AcquireTokenByCredential(ctx, opts.Scopes, confidential.WithTenantID(opts.TenantID))
	return azcore.AccessToken{Token: ar.AccessToken, ExpiresOn: ar.ExpiresOn.UTC()}, err
}

var _ azcore.TokenCredential = (*ClientSecretCredential)(nil)
