// Copyright (c) 2016, 2018, 2023, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.

package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/oracle/oci-go-sdk/v65/common"
)

const (
	//ResourcePrincipalVersion2_2 is a supported version for resource principals
	ResourcePrincipalVersion2_2 = "2.2"
	//ResourcePrincipalVersionEnvVar environment var name for version
	ResourcePrincipalVersionEnvVar = "OCI_RESOURCE_PRINCIPAL_VERSION"
	//ResourcePrincipalRPSTEnvVar environment var name holding the token or a path to the token
	ResourcePrincipalRPSTEnvVar = "OCI_RESOURCE_PRINCIPAL_RPST"
	//ResourcePrincipalPrivatePEMEnvVar environment var holding a rsa private key in pem format or a path to one
	ResourcePrincipalPrivatePEMEnvVar = "OCI_RESOURCE_PRINCIPAL_PRIVATE_PEM"
	//ResourcePrincipalPrivatePEMPassphraseEnvVar environment var holding the passphrase to a key or a path to one
	ResourcePrincipalPrivatePEMPassphraseEnvVar = "OCI_RESOURCE_PRINCIPAL_PRIVATE_PEM_PASSPHRASE"
	//ResourcePrincipalRegionEnvVar environment variable holding a region
	ResourcePrincipalRegionEnvVar = "OCI_RESOURCE_PRINCIPAL_REGION"

	//ResourcePrincipalVersion1_1 is a supported version for resource principals
	ResourcePrincipalVersion1_1 = "1.1"
	//ResourcePrincipalSessionTokenEndpoint endpoint for retrieving the Resource Principal Session Token
	ResourcePrincipalSessionTokenEndpoint = "OCI_RESOURCE_PRINCIPAL_RPST_ENDPOINT"
	//ResourcePrincipalTokenEndpoint endpoint for retrieving the Resource Principal Token
	ResourcePrincipalTokenEndpoint = "OCI_RESOURCE_PRINCIPAL_RPT_ENDPOINT"
	// KubernetesServiceAccountTokenPath that contains cluster information
	KubernetesServiceAccountTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	// DefaultKubernetesServiceAccountCertPath that contains cluster information
	DefaultKubernetesServiceAccountCertPath = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	// OciKubernetesServiceAccountCertPath Environment variable for Kubernetes Service Account Cert Path
	OciKubernetesServiceAccountCertPath = "OCI_KUBERNETES_SERVICE_ACCOUNT_CERT_PATH"
	// KubernetesServiceHostEnvVar environment var holding the kubernetes host
	KubernetesServiceHostEnvVar = "KUBERNETES_SERVICE_HOST"
	// KubernetesProxymuxServicePort environment var holding the kubernetes port
	KubernetesProxymuxServicePort = "12250"
	// TenancyOCIDClaimKey is the key used to look up the resource tenancy in an RPST
	TenancyOCIDClaimKey = "res_tenant"
	// CompartmentOCIDClaimKey is the key used to look up the resource compartment in an RPST
	CompartmentOCIDClaimKey = "res_compartment"
)

// ConfigurationProviderWithClaimAccess mixes in a method to access the claims held on the underlying security token
type ConfigurationProviderWithClaimAccess interface {
	common.ConfigurationProvider
	ClaimHolder
}

// ResourcePrincipalConfigurationProvider returns a resource principal configuration provider using well known
// environment variables to look up token information. The environment variables can either paths or contain the material value
// of the keys. However in the case of the keys and tokens paths and values can not be mixed
func ResourcePrincipalConfigurationProvider() (ConfigurationProviderWithClaimAccess, error) {
	var version string
	var ok bool
	if version, ok = os.LookupEnv(ResourcePrincipalVersionEnvVar); !ok {
		err := fmt.Errorf("can not create resource principal, environment variable: %s, not present", ResourcePrincipalVersionEnvVar)
		return nil, resourcePrincipalError{err: err}
	}

	switch version {
	case ResourcePrincipalVersion2_2:
		rpst := requireEnv(ResourcePrincipalRPSTEnvVar)
		if rpst == nil {
			err := fmt.Errorf("can not create resource principal, environment variable: %s, not present", ResourcePrincipalVersionEnvVar)
			return nil, resourcePrincipalError{err: err}
		}
		private := requireEnv(ResourcePrincipalPrivatePEMEnvVar)
		if private == nil {
			err := fmt.Errorf("can not create resource principal, environment variable: %s, not present", ResourcePrincipalVersionEnvVar)
			return nil, resourcePrincipalError{err: err}
		}
		passphrase := requireEnv(ResourcePrincipalPrivatePEMPassphraseEnvVar)
		region := requireEnv(ResourcePrincipalRegionEnvVar)
		if region == nil {
			err := fmt.Errorf("can not create resource principal, environment variable: %s, not present", ResourcePrincipalRegionEnvVar)
			return nil, resourcePrincipalError{err: err}
		}
		return newResourcePrincipalKeyProvider22(
			*rpst, *private, passphrase, *region)
	case ResourcePrincipalVersion1_1:
		return newResourcePrincipalKeyProvider11(DefaultRptPathProvider{})
	default:
		err := fmt.Errorf("can not create resource principal, environment variable: %s, must be valid", ResourcePrincipalVersionEnvVar)
		return nil, resourcePrincipalError{err: err}
	}
}

// OkeWorkloadIdentityConfigurationProvider returns a resource principal configuration provider by OKE Workload Identity
func OkeWorkloadIdentityConfigurationProvider() (ConfigurationProviderWithClaimAccess, error) {
	return OkeWorkloadIdentityConfigurationProviderWithServiceAccountTokenProvider(NewDefaultServiceAccountTokenProvider())
}

// OkeWorkloadIdentityConfigurationProviderWithServiceAccountTokenProvider returns a resource principal configuration provider by OKE Workload Identity
// with service account token provider
func OkeWorkloadIdentityConfigurationProviderWithServiceAccountTokenProvider(saTokenProvider ServiceAccountTokenProvider) (ConfigurationProviderWithClaimAccess, error) {
	var version string
	var ok bool
	if version, ok = os.LookupEnv(ResourcePrincipalVersionEnvVar); !ok {
		err := fmt.Errorf("can not create resource principal, environment variable: %s, not present", ResourcePrincipalVersionEnvVar)
		return nil, resourcePrincipalError{err: err}
	}

	if version == ResourcePrincipalVersion1_1 || version == ResourcePrincipalVersion2_2 {

		saCertPath := requireEnv(OciKubernetesServiceAccountCertPath)

		if saCertPath == nil {
			tmp := DefaultKubernetesServiceAccountCertPath
			saCertPath = &tmp
		}

		kubernetesServiceAccountCertRaw, err := ioutil.ReadFile(*saCertPath)
		if err != nil {
			err = fmt.Errorf("can not create resource principal, error getting Kubernetes Service Account Token at %s", *saCertPath)
			return nil, resourcePrincipalError{err: err}
		}

		kubernetesServiceAccountCert := x509.NewCertPool()
		kubernetesServiceAccountCert.AppendCertsFromPEM(kubernetesServiceAccountCertRaw)

		region := requireEnv(ResourcePrincipalRegionEnvVar)
		if region == nil {
			err := fmt.Errorf("can not create resource principal, environment variable: %s, not present",
				ResourcePrincipalRegionEnvVar)
			return nil, resourcePrincipalError{err: err}
		}

		k8sServiceHost := requireEnv(KubernetesServiceHostEnvVar)
		if k8sServiceHost == nil {
			err := fmt.Errorf("can not create resource principal, environment variable: %s, not present",
				KubernetesServiceHostEnvVar)
			return nil, resourcePrincipalError{err: err}
		}
		proxymuxEndpoint := fmt.Sprintf("https://%s:%s/resourcePrincipalSessionTokens", *k8sServiceHost, KubernetesProxymuxServicePort)

		return newOkeWorkloadIdentityProvider(proxymuxEndpoint, saTokenProvider, kubernetesServiceAccountCert, *region)
	}

	err := fmt.Errorf("can not create resource principal, environment variable: %s, must be valid", ResourcePrincipalVersionEnvVar)
	return nil, resourcePrincipalError{err: err}
}

// ResourcePrincipalConfigurationProviderForRegion returns a resource principal configuration provider using well known
// environment variables to look up token information, for a given region. The environment variables can either paths or contain the material value
// of the keys. However, in the case of the keys and tokens paths and values can not be mixed
func ResourcePrincipalConfigurationProviderForRegion(region common.Region) (ConfigurationProviderWithClaimAccess, error) {
	var version string
	var ok bool
	if version, ok = os.LookupEnv(ResourcePrincipalVersionEnvVar); !ok {
		err := fmt.Errorf("can not create resource principal, environment variable: %s, not present", ResourcePrincipalVersionEnvVar)
		return nil, resourcePrincipalError{err: err}
	}

	switch version {
	case ResourcePrincipalVersion2_2:
		rpst := requireEnv(ResourcePrincipalRPSTEnvVar)
		if rpst == nil {
			err := fmt.Errorf("can not create resource principal, environment variable: %s, not present", ResourcePrincipalVersionEnvVar)
			return nil, resourcePrincipalError{err: err}
		}
		private := requireEnv(ResourcePrincipalPrivatePEMEnvVar)
		if private == nil {
			err := fmt.Errorf("can not create resource principal, environment variable: %s, not present", ResourcePrincipalVersionEnvVar)
			return nil, resourcePrincipalError{err: err}
		}
		passphrase := requireEnv(ResourcePrincipalPrivatePEMPassphraseEnvVar)
		region := string(region)
		if region == "" {
			err := fmt.Errorf("can not create resource principal, region cannot be empty")
			return nil, resourcePrincipalError{err: err}
		}
		return newResourcePrincipalKeyProvider22(
			*rpst, *private, passphrase, region)
	case ResourcePrincipalVersion1_1:
		return newResourcePrincipalKeyProvider11(DefaultRptPathProvider{})
	default:
		err := fmt.Errorf("can not create resource principal, environment variable: %s, must be valid", ResourcePrincipalVersionEnvVar)
		return nil, resourcePrincipalError{err: err}
	}
}

// ResourcePrincipalConfigurationProviderWithPathProvider returns a resource principal configuration provider using path provider.
func ResourcePrincipalConfigurationProviderWithPathProvider(pathProvider PathProvider) (ConfigurationProviderWithClaimAccess, error) {
	var version string
	var ok bool
	if version, ok = os.LookupEnv(ResourcePrincipalVersionEnvVar); !ok {
		err := fmt.Errorf("can not create resource principal, environment variable: %s, not present", ResourcePrincipalVersionEnvVar)
		return nil, resourcePrincipalError{err: err}
	} else if version != ResourcePrincipalVersion1_1 {
		err := fmt.Errorf("can not create resource principal, environment variable: %s, must be %s", ResourcePrincipalVersionEnvVar, ResourcePrincipalVersion1_1)
		return nil, resourcePrincipalError{err: err}
	}
	return newResourcePrincipalKeyProvider11(pathProvider)
}

func newResourcePrincipalKeyProvider11(pathProvider PathProvider) (ConfigurationProviderWithClaimAccess, error) {
	rptEndpoint := requireEnv(ResourcePrincipalTokenEndpoint)
	if rptEndpoint == nil {
		err := fmt.Errorf("can not create resource principal, environment variable: %s, not present", ResourcePrincipalTokenEndpoint)
		return nil, resourcePrincipalError{err: err}
	}
	rptPath, err := pathProvider.Path()
	if err != nil {
		err := fmt.Errorf("can not create resource principal, due to: %s ", err.Error())
		return nil, resourcePrincipalError{err: err}
	}
	resourceID, err := pathProvider.ResourceID()
	if err != nil {
		err := fmt.Errorf("can not create resource principal, due to: %s ", err.Error())
		return nil, resourcePrincipalError{err: err}
	}
	rp, err := resourcePrincipalConfigurationProviderV1(*rptEndpoint+*rptPath, *resourceID)
	if err != nil {
		err := fmt.Errorf("can not create resource principal, due to: %s ", err.Error())
		return nil, resourcePrincipalError{err: err}
	}
	return rp, nil
}

func requireEnv(key string) *string {
	if val, ok := os.LookupEnv(key); ok {
		return &val
	}
	return nil
}

// resourcePrincipalKeyProvider22 is key provider that reads from specified the specified environment variables
// the environment variables can host the material keys/passphrases or they can be paths to files that need to be read
type resourcePrincipalKeyProvider struct {
	FederationClient  federationClient
	KeyProviderRegion common.Region
}

func newResourcePrincipalKeyProvider22(sessionTokenLocation, privatePemLocation string,
	passphraseLocation *string, region string) (*resourcePrincipalKeyProvider, error) {

	//Check both the passphrase and the key are paths
	if passphraseLocation != nil && (!isPath(privatePemLocation) && isPath(*passphraseLocation) ||
		isPath(privatePemLocation) && !isPath(*passphraseLocation)) {
		err := fmt.Errorf("cant not create resource principal: both key and passphrase need to be path or none needs to be path")
		return nil, resourcePrincipalError{err: err}
	}

	var supplier sessionKeySupplier
	var err error

	//File based case
	if isPath(privatePemLocation) {
		supplier, err = newFileBasedKeySessionSupplier(privatePemLocation, passphraseLocation)
		if err != nil {
			err := fmt.Errorf("can not create resource principal, due to: %s ", err.Error())
			return nil, resourcePrincipalError{err: err}
		}
	} else {
		//else the content is in the env vars
		var passphrase []byte
		if passphraseLocation != nil {
			passphrase = []byte(*passphraseLocation)
		}
		supplier, err = newStaticKeySessionSupplier([]byte(privatePemLocation), passphrase)
		if err != nil {
			err := fmt.Errorf("can not create resource principal, due to: %s ", err.Error())
			return nil, resourcePrincipalError{err: err}
		}
	}

	var fd federationClient
	if isPath(sessionTokenLocation) {
		fd, _ = newFileBasedFederationClient(sessionTokenLocation, supplier)
	} else {
		fd, err = newStaticFederationClient(sessionTokenLocation, supplier)

		if err != nil {
			err := fmt.Errorf("can not create resource principal, due to: %s ", err.Error())
			return nil, resourcePrincipalError{err: err}
		}
	}

	rs := resourcePrincipalKeyProvider{
		FederationClient:  fd,
		KeyProviderRegion: common.StringToRegion(region),
	}

	return &rs, nil
}

func newOkeWorkloadIdentityProvider(proxymuxEndpoint string, saTokenProvider ServiceAccountTokenProvider,
	kubernetesServiceAccountCert *x509.CertPool, region string) (*resourcePrincipalKeyProvider, error) {
	var err error
	var fd federationClient
	fd, err = newX509FederationClientForOkeWorkloadIdentity(proxymuxEndpoint, saTokenProvider, kubernetesServiceAccountCert)

	if err != nil {
		err := fmt.Errorf("can not create resource principal, due to: %s ", err.Error())
		return nil, resourcePrincipalError{err: err}
	}

	rs := resourcePrincipalKeyProvider{
		FederationClient:  fd,
		KeyProviderRegion: common.StringToRegion(region),
	}

	return &rs, nil
}

func (p *resourcePrincipalKeyProvider) PrivateRSAKey() (privateKey *rsa.PrivateKey, err error) {
	if privateKey, err = p.FederationClient.PrivateKey(); err != nil {
		err = fmt.Errorf("failed to get private key: %s", err.Error())
		return nil, resourcePrincipalError{err: err}
	}
	return privateKey, nil
}

func (p *resourcePrincipalKeyProvider) KeyID() (string, error) {
	var securityToken string
	var err error
	if securityToken, err = p.FederationClient.SecurityToken(); err != nil {
		err = fmt.Errorf("failed to get security token: %s", err.Error())
		return "", resourcePrincipalError{err: err}
	}
	return fmt.Sprintf("ST$%s", securityToken), nil
}

func (p *resourcePrincipalKeyProvider) Region() (string, error) {
	return string(p.KeyProviderRegion), nil
}

var (
	// ErrNonStringClaim is returned if the token has a claim for a key, but it's not a string value
	ErrNonStringClaim = errors.New("claim does not have a string value")
)

func (p *resourcePrincipalKeyProvider) TenancyOCID() (string, error) {
	if claim, err := p.GetClaim(TenancyOCIDClaimKey); err != nil {
		return "", err
	} else if tenancy, ok := claim.(string); ok {
		return tenancy, nil
	} else {
		return "", ErrNonStringClaim
	}
}

func (p *resourcePrincipalKeyProvider) GetClaim(claim string) (interface{}, error) {
	return p.FederationClient.GetClaim(claim)
}

func (p *resourcePrincipalKeyProvider) KeyFingerprint() (string, error) {
	return "", nil
}

func (p *resourcePrincipalKeyProvider) UserOCID() (string, error) {
	return "", nil
}

func (p *resourcePrincipalKeyProvider) AuthType() (common.AuthConfig, error) {
	return common.AuthConfig{common.UnknownAuthenticationType, false, nil}, fmt.Errorf("unsupported, keep the interface")
}

func (p *resourcePrincipalKeyProvider) Refreshable() bool {
	return true
}

// By contract for the the content of a resource principal to be considered path, it needs to be
// an absolute path.
func isPath(str string) bool {
	return path.IsAbs(str)
}

type resourcePrincipalError struct {
	err error
}

func (ipe resourcePrincipalError) Error() string {
	return fmt.Sprintf("%s\nResource principals authentication can only be used in certain OCI services. Please check that the OCI service you're running this code from supports Resource principals.\nSee https://docs.oracle.com/en-us/iaas/Content/API/Concepts/sdk_authentication_methods.htm#sdk_authentication_methods_resource_principal for more info.", ipe.err.Error())
}
