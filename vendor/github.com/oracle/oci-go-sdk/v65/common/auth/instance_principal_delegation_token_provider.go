// Copyright (c) 2016, 2018, 2023, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.

package auth

import (
	"crypto/rsa"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
)

type instancePrincipalDelegationTokenConfigurationProvider struct {
	instancePrincipalKeyProvider instancePrincipalKeyProvider
	delegationToken              string
	region                       *common.Region
}
type instancePrincipalDelegationTokenError struct {
	err error
}

func (ipe instancePrincipalDelegationTokenError) Error() string {
	return fmt.Sprintf("%s\nInstance principals delegation token authentication can only be used on specific OCI services. Please confirm this code is running on the correct environment", ipe.err.Error())
}

// InstancePrincipalDelegationTokenConfigurationProvider returns a configuration for obo token instance principals
func InstancePrincipalDelegationTokenConfigurationProvider(delegationToken *string) (common.ConfigurationProvider, error) {
	if delegationToken == nil || len(*delegationToken) == 0 {
		return nil, instancePrincipalDelegationTokenError{err: fmt.Errorf("failed to create a delagationTokenConfigurationProvider: token is a mandatory input parameter")}
	}
	return newInstancePrincipalDelegationTokenConfigurationProvider(delegationToken, "", nil)
}

// InstancePrincipalDelegationTokenConfigurationProviderForRegion returns a configuration for obo token instance principals with a given region
func InstancePrincipalDelegationTokenConfigurationProviderForRegion(delegationToken *string, region common.Region) (common.ConfigurationProvider, error) {
	if delegationToken == nil || len(*delegationToken) == 0 {
		return nil, instancePrincipalDelegationTokenError{err: fmt.Errorf("failed to create a delagationTokenConfigurationProvider: token is a mandatory input parameter")}
	}
	return newInstancePrincipalDelegationTokenConfigurationProvider(delegationToken, region, nil)
}

func newInstancePrincipalDelegationTokenConfigurationProvider(delegationToken *string, region common.Region, modifier func(common.HTTPRequestDispatcher) (common.HTTPRequestDispatcher,
	error)) (common.ConfigurationProvider, error) {

	keyProvider, err := newInstancePrincipalKeyProvider(modifier)
	if err != nil {
		return nil, instancePrincipalDelegationTokenError{err: fmt.Errorf("failed to create a new key provider for instance principal: %s", err.Error())}
	}
	if len(region) > 0 {
		return instancePrincipalDelegationTokenConfigurationProvider{*keyProvider, *delegationToken, &region}, err
	}
	return instancePrincipalDelegationTokenConfigurationProvider{*keyProvider, *delegationToken, nil}, err
}

func (p instancePrincipalDelegationTokenConfigurationProvider) getInstancePrincipalDelegationTokenConfigurationProvider() (instancePrincipalDelegationTokenConfigurationProvider, error) {
	return p, nil
}

func (p instancePrincipalDelegationTokenConfigurationProvider) PrivateRSAKey() (*rsa.PrivateKey, error) {
	return p.instancePrincipalKeyProvider.PrivateRSAKey()
}

func (p instancePrincipalDelegationTokenConfigurationProvider) KeyID() (string, error) {
	return p.instancePrincipalKeyProvider.KeyID()
}

func (p instancePrincipalDelegationTokenConfigurationProvider) TenancyOCID() (string, error) {
	return p.instancePrincipalKeyProvider.TenancyOCID()
}

func (p instancePrincipalDelegationTokenConfigurationProvider) UserOCID() (string, error) {
	return "", nil
}

func (p instancePrincipalDelegationTokenConfigurationProvider) KeyFingerprint() (string, error) {
	return "", nil
}

func (p instancePrincipalDelegationTokenConfigurationProvider) Region() (string, error) {
	if p.region == nil {
		region := p.instancePrincipalKeyProvider.RegionForFederationClient()
		common.Debugf("Region in instance principal delegation token configuration provider is nil. Returning federation clients region: %s", region)
		return string(region), nil
	}
	return string(*p.region), nil
}

func (p instancePrincipalDelegationTokenConfigurationProvider) AuthType() (common.AuthConfig, error) {
	token := p.delegationToken
	return common.AuthConfig{common.InstancePrincipalDelegationToken, false, &token}, nil
}

func (p instancePrincipalDelegationTokenConfigurationProvider) Refreshable() bool {
	return true
}
