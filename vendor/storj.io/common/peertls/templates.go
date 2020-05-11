// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package peertls

import (
	"crypto/x509"
	"crypto/x509/pkix"
)

// CATemplate returns x509.Certificate template for certificate authority
func CATemplate() (*x509.Certificate, error) {
	serialNumber, err := newSerialNumber()
	if err != nil {
		return nil, ErrTLSTemplate.Wrap(err)
	}

	template := &x509.Certificate{
		SerialNumber:          serialNumber,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		Subject:               pkix.Name{Organization: []string{"Storj"}},
	}

	return template, nil
}

// LeafTemplate returns x509.Certificate template for signing and encrypting
func LeafTemplate() (*x509.Certificate, error) {
	serialNumber, err := newSerialNumber()
	if err != nil {
		return nil, ErrTLSTemplate.Wrap(err)
	}

	template := &x509.Certificate{
		SerialNumber:          serialNumber,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
		Subject:               pkix.Name{Organization: []string{"Storj"}},
	}

	return template, nil
}
