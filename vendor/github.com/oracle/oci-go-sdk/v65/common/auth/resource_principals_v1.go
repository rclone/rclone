// Copyright (c) 2016, 2018, 2023, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.

package auth

import (
	"context"
	"crypto/rsa"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// resourcePrincipalFederationClient is the client used to to talk acquire resource principals
// No auth client, leaf or intermediate retrievers. We use certificates retrieved by instance principals to sign the operations of
// resource principals
type resourcePrincipalFederationClient struct {
	tenancyID          string
	instanceID         string
	sessionKeySupplier sessionKeySupplier
	mux                sync.Mutex
	securityToken      securityToken
	path               string

	//instancePrincipalKeyProvider the instance Principal Key container
	instancePrincipalKeyProvider instancePrincipalKeyProvider

	//ResourcePrincipalTargetServiceClient client that calls the target service to acquire a resource principal token
	ResourcePrincipalTargetServiceClient common.BaseClient

	//ResourcePrincipalSessionTokenClient. The client used to communicate with identity to exchange a resource principal for
	// resource principal session token
	ResourcePrincipalSessionTokenClient common.BaseClient
}

type resourcePrincipalTokenRequest struct {
	InstanceID string `contributesTo:"path" name:"id"`
}

type resourcePrincipalTokenResponse struct {
	Body struct {
		ResourcePrincipalToken       string `json:"resourcePrincipalToken"`
		ServicePrincipalSessionToken string `json:"servicePrincipalSessionToken"`
	} `presentIn:"body"`
}

type resourcePrincipalSessionTokenRequestBody struct {
	ResourcePrincipalToken       string `json:"resourcePrincipalToken,omitempty"`
	ServicePrincipalSessionToken string `json:"servicePrincipalSessionToken,omitempty"`
	SessionPublicKey             string `json:"sessionPublicKey,omitempty"`
}
type resourcePrincipalSessionTokenRequest struct {
	Body resourcePrincipalSessionTokenRequestBody `contributesTo:"body"`
}

// acquireResourcePrincipalToken acquires the resource principal from the target service
func (c *resourcePrincipalFederationClient) acquireResourcePrincipalToken() (tokenResponse resourcePrincipalTokenResponse, err error) {
	rpServiceClient := c.ResourcePrincipalTargetServiceClient

	//Set the signer of this client to be the instance principal provider
	rpServiceClient.Signer = common.DefaultRequestSigner(&c.instancePrincipalKeyProvider)

	//Create a request with the instanceId
	request, err := common.MakeDefaultHTTPRequestWithTaggedStruct(http.MethodGet, c.path, resourcePrincipalTokenRequest{InstanceID: c.instanceID})
	if err != nil {
		return
	}

	//Call the target service
	response, err := rpServiceClient.Call(context.Background(), &request)
	if err != nil {
		return
	}

	defer common.CloseBodyIfValid(response)

	tokenResponse = resourcePrincipalTokenResponse{}
	err = common.UnmarshalResponse(response, &tokenResponse)
	return
}

// exchangeToken exchanges a resource principal token from the target service with a session token from identity
func (c *resourcePrincipalFederationClient) exchangeToken(publicKeyBase64 string, tokenResponse resourcePrincipalTokenResponse) (sessionToken string, err error) {
	rpServiceClient := c.ResourcePrincipalSessionTokenClient

	//Set the signer of this client to be the instance principal provider
	rpServiceClient.Signer = common.DefaultRequestSigner(&c.instancePrincipalKeyProvider)

	// Call identity service to get resource principal session token
	sessionTokenReq := resourcePrincipalSessionTokenRequest{
		resourcePrincipalSessionTokenRequestBody{
			ServicePrincipalSessionToken: tokenResponse.Body.ServicePrincipalSessionToken,
			ResourcePrincipalToken:       tokenResponse.Body.ResourcePrincipalToken,
			SessionPublicKey:             publicKeyBase64,
		},
	}

	sessionTokenHTTPReq, err := common.MakeDefaultHTTPRequestWithTaggedStruct(http.MethodPost,
		"", sessionTokenReq)
	if err != nil {
		return
	}

	sessionTokenHTTPRes, err := rpServiceClient.Call(context.Background(), &sessionTokenHTTPReq)
	if err != nil {
		return
	}
	defer common.CloseBodyIfValid(sessionTokenHTTPRes)

	sessionTokenRes := x509FederationResponse{}
	err = common.UnmarshalResponse(sessionTokenHTTPRes, &sessionTokenRes)
	if err != nil {
		return
	}

	sessionToken = sessionTokenRes.Token.Token
	return
}

// getSecurityToken makes the appropiate calls to acquire a resource principal security token
func (c *resourcePrincipalFederationClient) getSecurityToken() (securityToken, error) {
	var err error
	ipFederationClient := c.instancePrincipalKeyProvider.FederationClient

	common.Debugf("Refreshing instance principal token")
	//Refresh instance principal token
	if refreshable, ok := ipFederationClient.(*x509FederationClient); ok {
		err = refreshable.renewSecurityTokenIfNotValid()
		if err != nil {
			return nil, err
		}
	}

	//Acquire resource principal token from target service
	common.Debugf("Acquiring resource principal token from target service")
	tokenResponse, err := c.acquireResourcePrincipalToken()
	if err != nil {
		return nil, err
	}

	//Read the public key from the session supplier.
	pem := c.sessionKeySupplier.PublicKeyPemRaw()
	pemSanitized := sanitizeCertificateString(string(pem))

	//Exchange resource principal token for session token from identity
	common.Debugf("Exchanging resource principal token for resource principal session token")
	sessionToken, err := c.exchangeToken(pemSanitized, tokenResponse)
	if err != nil {
		return nil, err
	}

	return newPrincipalToken(sessionToken) // should be a resource principal token
}

func (c *resourcePrincipalFederationClient) renewSecurityToken() (err error) {
	if err = c.sessionKeySupplier.Refresh(); err != nil {
		return fmt.Errorf("failed to refresh session key: %s", err.Error())
	}
	common.Logf("Renewing resource principal security token at: %v\n", time.Now().Format("15:04:05.000"))
	if c.securityToken, err = c.getSecurityToken(); err != nil {
		return fmt.Errorf("failed to get security token: %s", err.Error())
	}
	common.Logf("Resource principal security token renewed at: %v\n", time.Now().Format("15:04:05.000"))

	return nil
}

// ResourcePrincipal Key provider in charge of resource principal acquiring tokens
type resourcePrincipalKeyProviderV1 struct {
	ResourcePrincipalClient resourcePrincipalFederationClient
}

func (c *resourcePrincipalFederationClient) renewSecurityTokenIfNotValid() (err error) {
	if c.securityToken == nil || !c.securityToken.Valid() {
		if err = c.renewSecurityToken(); err != nil {
			return fmt.Errorf("failed to renew resource prinicipal security token: %s", err.Error())
		}
	}
	return nil
}

func (c *resourcePrincipalFederationClient) PrivateKey() (*rsa.PrivateKey, error) {
	c.mux.Lock()
	defer c.mux.Unlock()

	if err := c.renewSecurityTokenIfNotValid(); err != nil {
		return nil, err
	}
	return c.sessionKeySupplier.PrivateKey(), nil
}

func (c *resourcePrincipalFederationClient) SecurityToken() (token string, err error) {
	c.mux.Lock()
	defer c.mux.Unlock()

	if err = c.renewSecurityTokenIfNotValid(); err != nil {
		return "", err
	}
	return c.securityToken.String(), nil
}

func (p *resourcePrincipalConfigurationProvider) PrivateRSAKey() (privateKey *rsa.PrivateKey, err error) {
	if privateKey, err = p.keyProvider.ResourcePrincipalClient.PrivateKey(); err != nil {
		err = fmt.Errorf("failed to get resource principal private key: %s", err.Error())
		return nil, err
	}
	return privateKey, nil
}

func (p *resourcePrincipalConfigurationProvider) KeyID() (string, error) {
	var securityToken string
	var err error
	if securityToken, err = p.keyProvider.ResourcePrincipalClient.SecurityToken(); err != nil {
		return "", fmt.Errorf("failed to get resource principal security token: %s", err.Error())
	}
	return fmt.Sprintf("ST$%s", securityToken), nil
}

func (p *resourcePrincipalConfigurationProvider) TenancyOCID() (string, error) {
	return p.keyProvider.ResourcePrincipalClient.instancePrincipalKeyProvider.TenancyOCID()
}

// todo what is this
func (p *resourcePrincipalConfigurationProvider) GetClaim(key string) (interface{}, error) {
	return nil, nil
}

// Resource Principals
type resourcePrincipalConfigurationProvider struct {
	keyProvider resourcePrincipalKeyProviderV1
	region      *common.Region
}

func newResourcePrincipalKeyProvider(ipKeyProvider instancePrincipalKeyProvider, rpTokenTargetServiceClient, rpSessionTokenClient common.BaseClient, instanceID, path string) (keyProvider resourcePrincipalKeyProviderV1, err error) {
	rpFedClient := resourcePrincipalFederationClient{}
	rpFedClient.tenancyID = ipKeyProvider.TenancyID
	rpFedClient.instanceID = instanceID
	rpFedClient.sessionKeySupplier = newSessionKeySupplier()
	rpFedClient.ResourcePrincipalTargetServiceClient = rpTokenTargetServiceClient
	rpFedClient.ResourcePrincipalSessionTokenClient = rpSessionTokenClient
	rpFedClient.instancePrincipalKeyProvider = ipKeyProvider
	rpFedClient.path = path
	keyProvider = resourcePrincipalKeyProviderV1{ResourcePrincipalClient: rpFedClient}
	return
}

func (p *resourcePrincipalConfigurationProvider) AuthType() (common.AuthConfig, error) {
	return common.AuthConfig{common.UnknownAuthenticationType, false, nil},
		fmt.Errorf("unsupported, keep the interface")
}

func (p resourcePrincipalConfigurationProvider) UserOCID() (string, error) {
	return "", nil
}

func (p resourcePrincipalConfigurationProvider) KeyFingerprint() (string, error) {
	return "", nil
}

func (p resourcePrincipalConfigurationProvider) Region() (string, error) {
	if p.region == nil {
		region := p.keyProvider.ResourcePrincipalClient.instancePrincipalKeyProvider.RegionForFederationClient()
		common.Debugf("Region in resource principal configuration provider is nil. Returning instance principal federation clients region: %s", region)
		return string(region), nil
	}
	return string(*p.region), nil
}

func (p resourcePrincipalConfigurationProvider) Refreshable() bool {
	return true
}

// resourcePrincipalConfigurationProviderForInstanceWithClients returns a configuration for instance principals
// resourcePrincipalTargetServiceTokenClient and resourcePrincipalSessionTokenClient are clients that at last need to have
// their base path and host properly set for their respective services. Additionally the clients can be further customized
// to provide mocking or any other customization for the requests/responses
func resourcePrincipalConfigurationProviderForInstanceWithClients(instancePrincipalProvider common.ConfigurationProvider,
	resourcePrincipalTargetServiceTokenClient, resourcePrincipalSessionTokenClient common.BaseClient, instanceID, path string) (*resourcePrincipalConfigurationProvider, error) {
	var ok bool
	var ip instancePrincipalConfigurationProvider
	if ip, ok = instancePrincipalProvider.(instancePrincipalConfigurationProvider); !ok {
		return nil, fmt.Errorf("instancePrincipalConfigurationProvider needs to be of type vald Instance Principal Configuration Provider")
	}

	keyProvider, err := newResourcePrincipalKeyProvider(ip.keyProvider, resourcePrincipalTargetServiceTokenClient, resourcePrincipalSessionTokenClient, instanceID, path)
	if err != nil {
		return nil, err
	}

	provider := &resourcePrincipalConfigurationProvider{
		region:      nil,
		keyProvider: keyProvider,
	}
	return provider, nil
}

const identityResourcePrincipalSessionTokenPath = "/v1/resourcePrincipalSessionToken"

// resourcePrincipalConfigurationProviderForInstanceWithInterceptor creates a resource principal configuration provider with
// a interceptor used to customize the call going to the resource principal token request to the target service
// for a given instance ID
func resourcePrincipalConfigurationProviderForInstanceWithInterceptor(instancePrincipalProvider common.ConfigurationProvider, resourcePrincipalTokenEndpoint, instanceID string, interceptor common.RequestInterceptor) (provider *resourcePrincipalConfigurationProvider, err error) {

	//Build the target service client
	rpTargetServiceClient, err := common.NewClientWithConfig(instancePrincipalProvider)
	if err != nil {
		return
	}

	rpTokenURL, err := url.Parse(resourcePrincipalTokenEndpoint)
	if err != nil {
		return
	}

	rpTargetServiceClient.Host = rpTokenURL.Scheme + "://" + rpTokenURL.Host
	rpTargetServiceClient.Interceptor = interceptor

	var path string
	if rpTokenURL.Path != "" {
		path = rpTokenURL.Path
	} else {
		path = identityResourcePrincipalSessionTokenPath
	}

	//Build the identity client for token service
	rpTokenSessionClient, err := common.NewClientWithConfig(instancePrincipalProvider)
	if err != nil {
		return
	}

	// Set RPST endpoint if passed in from env var, otherwise create it from region
	resourcePrincipalSessionTokenEndpoint := requireEnv(ResourcePrincipalSessionTokenEndpoint)
	if resourcePrincipalSessionTokenEndpoint != nil {
		rpSessionTokenURL, err := url.Parse(*resourcePrincipalSessionTokenEndpoint)
		if err != nil {
			return nil, err
		}

		rpTokenSessionClient.Host = rpSessionTokenURL.Scheme + "://" + rpSessionTokenURL.Host
	} else {
		regionStr, err := instancePrincipalProvider.Region()
		if err != nil {
			return nil, fmt.Errorf("missing RPST env var and cannot determine region: %v", err)
		}
		region := common.StringToRegion(regionStr)
		rpTokenSessionClient.Host = fmt.Sprintf("https://%s", region.Endpoint("auth"))
	}

	rpTokenSessionClient.BasePath = identityResourcePrincipalSessionTokenPath

	return resourcePrincipalConfigurationProviderForInstanceWithClients(instancePrincipalProvider, rpTargetServiceClient, rpTokenSessionClient, instanceID, path)
}

// ResourcePrincipalConfigurationProviderWithInterceptor creates a resource principal configuration provider with endpoints
// a interceptor used to customize the call going to the resource principal token request to the target service
// see https://godoc.org/github.com/oracle/oci-go-sdk/common#RequestInterceptor
func ResourcePrincipalConfigurationProviderWithInterceptor(instancePrincipalProvider common.ConfigurationProvider,
	resourcePrincipalTokenEndpoint, resourcePrincipalSessionTokenEndpoint string,
	interceptor common.RequestInterceptor) (common.ConfigurationProvider, error) {

	return resourcePrincipalConfigurationProviderForInstanceWithInterceptor(instancePrincipalProvider, resourcePrincipalTokenEndpoint, "", interceptor)
}

// resourcePrincipalConfigurationProviderV1 creates a resource principal configuration provider with
// endpoints for both resource principal token and resource principal token session
func resourcePrincipalConfigurationProviderV1(resourcePrincipalTokenEndpoint, resourceID string) (*resourcePrincipalConfigurationProvider, error) {

	instancePrincipalProvider, err := InstancePrincipalConfigurationProvider()
	if err != nil {
		return nil, err
	}
	return resourcePrincipalConfigurationProviderForInstanceWithInterceptor(instancePrincipalProvider, resourcePrincipalTokenEndpoint, resourceID, nil)
}
