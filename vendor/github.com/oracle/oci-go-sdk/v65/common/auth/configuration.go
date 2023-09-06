// Copyright (c) 2016, 2018, 2023, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.

package auth

import (
	"crypto/rsa"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
)

type instancePrincipalConfigurationProvider struct {
	keyProvider instancePrincipalKeyProvider
	region      *common.Region
}

// InstancePrincipalConfigurationProvider returns a configuration for instance principals
func InstancePrincipalConfigurationProvider() (common.ConfigurationProvider, error) {
	return newInstancePrincipalConfigurationProvider("", nil)
}

// InstancePrincipalConfigurationProviderForRegion returns a configuration for instance principals with a given region
func InstancePrincipalConfigurationProviderForRegion(region common.Region) (common.ConfigurationProvider, error) {
	return newInstancePrincipalConfigurationProvider(region, nil)
}

// InstancePrincipalConfigurationProviderWithCustomClient returns a configuration for instance principals using a modifier function to modify the HTTPRequestDispatcher
func InstancePrincipalConfigurationProviderWithCustomClient(modifier func(common.HTTPRequestDispatcher) (common.HTTPRequestDispatcher, error)) (common.ConfigurationProvider, error) {
	return newInstancePrincipalConfigurationProvider("", modifier)
}

// InstancePrincipalConfigurationForRegionWithCustomClient returns a configuration for instance principals with a given region using a modifier function to modify the HTTPRequestDispatcher
func InstancePrincipalConfigurationForRegionWithCustomClient(region common.Region, modifier func(common.HTTPRequestDispatcher) (common.HTTPRequestDispatcher, error)) (common.ConfigurationProvider, error) {
	return newInstancePrincipalConfigurationProvider(region, modifier)
}

func newInstancePrincipalConfigurationProvider(region common.Region, modifier func(common.HTTPRequestDispatcher) (common.HTTPRequestDispatcher, error)) (common.ConfigurationProvider, error) {
	var err error
	var keyProvider *instancePrincipalKeyProvider
	if keyProvider, err = newInstancePrincipalKeyProvider(modifier); err != nil {
		return nil, fmt.Errorf("failed to create a new key provider for instance principal: %s", err.Error())
	}
	if len(region) > 0 {
		return instancePrincipalConfigurationProvider{keyProvider: *keyProvider, region: &region}, nil
	}
	return instancePrincipalConfigurationProvider{keyProvider: *keyProvider, region: nil}, nil
}

// InstancePrincipalConfigurationWithCerts returns a configuration for instance principals with a given region and hardcoded certificates in lieu of metadata service certs
func InstancePrincipalConfigurationWithCerts(region common.Region, leafCertificate, leafPassphrase, leafPrivateKey []byte, intermediateCertificates [][]byte) (common.ConfigurationProvider, error) {
	leafCertificateRetriever := staticCertificateRetriever{Passphrase: leafPassphrase, CertificatePem: leafCertificate, PrivateKeyPem: leafPrivateKey}

	//The .Refresh() call actually reads the certificates from the inputs
	err := leafCertificateRetriever.Refresh()
	if err != nil {
		return nil, err
	}

	certificate := leafCertificateRetriever.Certificate()

	tenancyID := extractTenancyIDFromCertificate(certificate)
	fedClient, err := newX509FederationClientWithCerts(region, tenancyID, leafCertificate, leafPassphrase, leafPrivateKey, intermediateCertificates, *newDispatcherModifier(nil))
	if err != nil {
		return nil, err
	}

	provider := instancePrincipalConfigurationProvider{
		keyProvider: instancePrincipalKeyProvider{
			Region:           region,
			FederationClient: fedClient,
			TenancyID:        tenancyID,
		},
		region: &region,
	}
	return provider, nil

}

func (p instancePrincipalConfigurationProvider) PrivateRSAKey() (*rsa.PrivateKey, error) {
	return p.keyProvider.PrivateRSAKey()
}

func (p instancePrincipalConfigurationProvider) KeyID() (string, error) {
	return p.keyProvider.KeyID()
}

func (p instancePrincipalConfigurationProvider) TenancyOCID() (string, error) {
	return p.keyProvider.TenancyOCID()
}

func (p instancePrincipalConfigurationProvider) UserOCID() (string, error) {
	return "", nil
}

func (p instancePrincipalConfigurationProvider) KeyFingerprint() (string, error) {
	return "", nil
}

func (p instancePrincipalConfigurationProvider) Region() (string, error) {
	if p.region == nil {
		region := p.keyProvider.RegionForFederationClient()
		common.Debugf("Region in instance principal configuration provider is nil. Returning federation clients region: %s", region)
		return string(region), nil
	}
	return string(*p.region), nil
}

func (p instancePrincipalConfigurationProvider) AuthType() (common.AuthConfig, error) {
	return common.AuthConfig{common.InstancePrincipal, false, nil}, fmt.Errorf("unsupported, keep the interface")
}

func (p instancePrincipalConfigurationProvider) Refreshable() bool {
	return true
}
