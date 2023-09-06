// Copyright (c) 2016, 2018, 2023, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.

package auth

import (
	"bytes"
	"crypto/rsa"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
)

const (
	metadataBaseURL             = `http://169.254.169.254/opc/v2`
	metadataFallbackURL         = `http://169.254.169.254/opc/v1`
	regionPath                  = `/instance/region`
	leafCertificatePath         = `/identity/cert.pem`
	leafCertificateKeyPath      = `/identity/key.pem`
	intermediateCertificatePath = `/identity/intermediate.pem`

	leafCertificateKeyPassphrase         = `` // No passphrase for the private key for Compute instances
	intermediateCertificateKeyURL        = ``
	intermediateCertificateKeyPassphrase = `` // No passphrase for the private key for Compute instances
)

var (
	regionURL, leafCertificateURL, leafCertificateKeyURL, intermediateCertificateURL string
)

// instancePrincipalKeyProvider implements KeyProvider to provide a key ID and its corresponding private key
// for an instance principal by getting a security token via x509FederationClient.
//
// The region name of the endpoint for x509FederationClient is obtained from the metadata service on the compute
// instance.
type instancePrincipalKeyProvider struct {
	Region           common.Region
	FederationClient federationClient
	TenancyID        string
}

type instancePrincipalError struct {
	err error
}

func (ipe instancePrincipalError) Error() string {
	return fmt.Sprintf("%s\nInstance principals authentication can only be used on OCI compute instances. Please confirm this code is running on an OCI compute instance and you have set up the policy properly.\nSee https://docs.oracle.com/en-us/iaas/Content/Identity/Tasks/callingservicesfrominstances.htm for more info", ipe.err.Error())
}

// newInstancePrincipalKeyProvider creates and returns an instancePrincipalKeyProvider instance based on
// x509FederationClient.
//
// NOTE: There is a race condition between PrivateRSAKey() and KeyID().  These two pieces are tightly coupled; KeyID
// includes a security token obtained from Auth service by giving a public key which is paired with PrivateRSAKey.
// The x509FederationClient caches the security token in memory until it is expired.  Thus, even if a client obtains a
// KeyID that is not expired at the moment, the PrivateRSAKey that the client acquires at a next moment could be
// invalid because the KeyID could be already expired.
func newInstancePrincipalKeyProvider(modifier func(common.HTTPRequestDispatcher) (common.HTTPRequestDispatcher, error)) (provider *instancePrincipalKeyProvider, err error) {
	updateX509CertRetrieverURLParas(metadataBaseURL)
	clientModifier := newDispatcherModifier(modifier)

	client, err := clientModifier.Modify(&http.Client{})
	if err != nil {
		err = fmt.Errorf("failed to modify client: %s", err.Error())
		return nil, instancePrincipalError{err: err}
	}

	var region common.Region

	if region, err = getRegionForFederationClient(client, regionURL); err != nil {
		err = fmt.Errorf("failed to get the region name from %s: %s", regionURL, err.Error())
		common.Logf("%v\n", err)
		return nil, instancePrincipalError{err: err}
	}

	leafCertificateRetriever := newURLBasedX509CertificateRetriever(client,
		leafCertificateURL, leafCertificateKeyURL, leafCertificateKeyPassphrase)
	intermediateCertificateRetrievers := []x509CertificateRetriever{
		newURLBasedX509CertificateRetriever(
			client, intermediateCertificateURL, intermediateCertificateKeyURL,
			intermediateCertificateKeyPassphrase),
	}

	if err = leafCertificateRetriever.Refresh(); err != nil {
		err = fmt.Errorf("failed to refresh the leaf certificate: %s", err.Error())
		return nil, instancePrincipalError{err: err}
	}
	tenancyID := extractTenancyIDFromCertificate(leafCertificateRetriever.Certificate())

	federationClient, err := newX509FederationClient(region, tenancyID, leafCertificateRetriever, intermediateCertificateRetrievers, *clientModifier)

	if err != nil {
		err = fmt.Errorf("failed to create federation client: %s", err.Error())
		return nil, instancePrincipalError{err: err}
	}

	provider = &instancePrincipalKeyProvider{FederationClient: federationClient, TenancyID: tenancyID, Region: region}
	return
}

func getRegionForFederationClient(dispatcher common.HTTPRequestDispatcher, url string) (r common.Region, err error) {
	var body bytes.Buffer
	var statusCode int
	MaxRetriesFederationClient := 3
	for currTry := 0; currTry < MaxRetriesFederationClient; currTry++ {
		body, statusCode, err = httpGet(dispatcher, url)
		if err == nil && statusCode == 200 {
			return common.StringToRegion(body.String()), nil
		}
		common.Logf("Error in getting region from url: %s, Status code: %v, Error: %s", url, statusCode, err.Error())
		if statusCode == 404 && strings.Compare(url, metadataBaseURL+regionPath) == 0 {
			common.Logf("Falling back to http://169.254.169.254/opc/v1 to try again...")
			updateX509CertRetrieverURLParas(metadataFallbackURL)
			url = regionURL
		}
		time.Sleep(1 * time.Second)
	}
	return
}

func updateX509CertRetrieverURLParas(baseURL string) {
	regionURL = baseURL + regionPath
	leafCertificateURL = baseURL + leafCertificatePath
	leafCertificateKeyURL = baseURL + leafCertificateKeyPath
	intermediateCertificateURL = baseURL + intermediateCertificatePath
}

func (p *instancePrincipalKeyProvider) RegionForFederationClient() common.Region {
	return p.Region
}

func (p *instancePrincipalKeyProvider) PrivateRSAKey() (privateKey *rsa.PrivateKey, err error) {
	if privateKey, err = p.FederationClient.PrivateKey(); err != nil {
		err = fmt.Errorf("failed to get private key: %s", err.Error())
		return nil, instancePrincipalError{err: err}
	}
	return privateKey, nil
}

func (p *instancePrincipalKeyProvider) KeyID() (string, error) {
	var securityToken string
	var err error
	if securityToken, err = p.FederationClient.SecurityToken(); err != nil {
		err = fmt.Errorf("failed to get security token: %s", err.Error())
		return "", instancePrincipalError{err: err}
	}
	return fmt.Sprintf("ST$%s", securityToken), nil
}

func (p *instancePrincipalKeyProvider) TenancyOCID() (string, error) {
	return p.TenancyID, nil
}

func (p *instancePrincipalKeyProvider) Refreshable() bool {
	return true
}
