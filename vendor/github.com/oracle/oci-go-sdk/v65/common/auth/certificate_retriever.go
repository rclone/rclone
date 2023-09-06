// Copyright (c) 2016, 2018, 2023, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.

package auth

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"sync"

	"github.com/oracle/oci-go-sdk/v65/common"
)

// x509CertificateRetriever provides an X509 certificate with the RSA private key
type x509CertificateRetriever interface {
	Refresh() error
	CertificatePemRaw() []byte
	Certificate() *x509.Certificate
	PrivateKeyPemRaw() []byte
	PrivateKey() *rsa.PrivateKey
}

// urlBasedX509CertificateRetriever retrieves PEM-encoded X509 certificates from the given URLs.
type urlBasedX509CertificateRetriever struct {
	certURL           string
	privateKeyURL     string
	passphrase        string
	certificatePemRaw []byte
	certificate       *x509.Certificate
	privateKeyPemRaw  []byte
	privateKey        *rsa.PrivateKey
	mux               sync.Mutex
	dispatcher        common.HTTPRequestDispatcher
}

func newURLBasedX509CertificateRetriever(dispatcher common.HTTPRequestDispatcher, certURL, privateKeyURL, passphrase string) x509CertificateRetriever {
	return &urlBasedX509CertificateRetriever{
		certURL:       certURL,
		privateKeyURL: privateKeyURL,
		passphrase:    passphrase,
		mux:           sync.Mutex{},
		dispatcher:    dispatcher,
	}
}

// Refresh() is failure atomic, i.e., CertificatePemRaw(), Certificate(), PrivateKeyPemRaw(), and PrivateKey() would
// return their previous values if Refresh() fails.
func (r *urlBasedX509CertificateRetriever) Refresh() error {
	common.Debugln("Refreshing certificate")

	r.mux.Lock()
	defer r.mux.Unlock()

	var err error

	var certificatePemRaw []byte
	var certificate *x509.Certificate
	if certificatePemRaw, certificate, err = r.renewCertificate(r.certURL); err != nil {
		return fmt.Errorf("failed to renew certificate: %s", err.Error())
	}

	var privateKeyPemRaw []byte
	var privateKey *rsa.PrivateKey
	if r.privateKeyURL != "" {
		if privateKeyPemRaw, privateKey, err = r.renewPrivateKey(r.privateKeyURL, r.passphrase); err != nil {
			return fmt.Errorf("failed to renew private key: %s", err.Error())
		}
	}

	r.certificatePemRaw = certificatePemRaw
	r.certificate = certificate
	r.privateKeyPemRaw = privateKeyPemRaw
	r.privateKey = privateKey
	return nil
}

func (r *urlBasedX509CertificateRetriever) renewCertificate(url string) (certificatePemRaw []byte, certificate *x509.Certificate, err error) {
	var body bytes.Buffer
	if body, _, err = httpGet(r.dispatcher, url); err != nil {
		return nil, nil, fmt.Errorf("failed to get certificate from %s: %s", url, err.Error())
	}

	certificatePemRaw = body.Bytes()
	var block *pem.Block
	block, _ = pem.Decode(certificatePemRaw)
	if block == nil {
		return nil, nil, fmt.Errorf("failed to parse the new certificate, not valid pem data")
	}

	if certificate, err = x509.ParseCertificate(block.Bytes); err != nil {
		return nil, nil, fmt.Errorf("failed to parse the new certificate: %s", err.Error())
	}

	return certificatePemRaw, certificate, nil
}

func (r *urlBasedX509CertificateRetriever) renewPrivateKey(url, passphrase string) (privateKeyPemRaw []byte, privateKey *rsa.PrivateKey, err error) {
	var body bytes.Buffer
	if body, _, err = httpGet(r.dispatcher, url); err != nil {
		return nil, nil, fmt.Errorf("failed to get private key from %s: %s", url, err.Error())
	}

	privateKeyPemRaw = body.Bytes()
	if privateKey, err = common.PrivateKeyFromBytes(privateKeyPemRaw, &passphrase); err != nil {
		return nil, nil, fmt.Errorf("failed to parse the new private key: %s", err.Error())
	}

	return privateKeyPemRaw, privateKey, nil
}

func (r *urlBasedX509CertificateRetriever) CertificatePemRaw() []byte {
	r.mux.Lock()
	defer r.mux.Unlock()

	if r.certificatePemRaw == nil {
		return nil
	}

	c := make([]byte, len(r.certificatePemRaw))
	copy(c, r.certificatePemRaw)
	return c
}

func (r *urlBasedX509CertificateRetriever) Certificate() *x509.Certificate {
	r.mux.Lock()
	defer r.mux.Unlock()

	if r.certificate == nil {
		return nil
	}

	c := *r.certificate
	return &c
}

func (r *urlBasedX509CertificateRetriever) PrivateKeyPemRaw() []byte {
	r.mux.Lock()
	defer r.mux.Unlock()

	if r.privateKeyPemRaw == nil {
		return nil
	}

	c := make([]byte, len(r.privateKeyPemRaw))
	copy(c, r.privateKeyPemRaw)
	return c
}

func (r *urlBasedX509CertificateRetriever) PrivateKey() *rsa.PrivateKey {
	r.mux.Lock()
	defer r.mux.Unlock()

	//Nil Private keys are supported as part of a certificate
	if r.privateKey == nil {
		return nil
	}

	c := *r.privateKey
	return &c
}

// staticCertificateRetriever serves certificates from static data
type staticCertificateRetriever struct {
	Passphrase     []byte
	CertificatePem []byte
	PrivateKeyPem  []byte
	certificate    *x509.Certificate
	privateKey     *rsa.PrivateKey
	mux            sync.Mutex
}

// Refresh proccess the inputs into appropiate keys and certificates
func (r *staticCertificateRetriever) Refresh() error {
	r.mux.Lock()
	defer r.mux.Unlock()

	certifcate, err := r.readCertificate()
	if err != nil {
		r.certificate = nil
		return err
	}
	r.certificate = certifcate

	key, err := r.readPrivateKey()
	if err != nil {
		r.privateKey = nil
		return err
	}
	r.privateKey = key

	return nil
}

func (r *staticCertificateRetriever) Certificate() *x509.Certificate {
	r.mux.Lock()
	defer r.mux.Unlock()

	return r.certificate
}

func (r *staticCertificateRetriever) PrivateKey() *rsa.PrivateKey {
	r.mux.Lock()
	defer r.mux.Unlock()

	return r.privateKey
}

func (r *staticCertificateRetriever) CertificatePemRaw() []byte {
	r.mux.Lock()
	defer r.mux.Unlock()

	if r.CertificatePem == nil {
		return nil
	}

	c := make([]byte, len(r.CertificatePem))
	copy(c, r.CertificatePem)
	return c
}

func (r *staticCertificateRetriever) PrivateKeyPemRaw() []byte {
	r.mux.Lock()
	defer r.mux.Unlock()

	if r.PrivateKeyPem == nil {
		return nil
	}

	c := make([]byte, len(r.PrivateKeyPem))
	copy(c, r.PrivateKeyPem)
	return c
}

func (r *staticCertificateRetriever) readCertificate() (certificate *x509.Certificate, err error) {
	block, _ := pem.Decode(r.CertificatePem)
	if block == nil {
		return nil, fmt.Errorf("failed to parse the new certificate, not valid pem data")
	}

	if certificate, err = x509.ParseCertificate(block.Bytes); err != nil {
		return nil, fmt.Errorf("failed to parse the new certificate: %s", err.Error())
	}
	return certificate, nil
}

func (r *staticCertificateRetriever) readPrivateKey() (*rsa.PrivateKey, error) {
	if r.PrivateKeyPem == nil {
		return nil, nil
	}

	var pass *string
	if r.Passphrase == nil {
		pass = nil
	} else {
		ss := string(r.Passphrase)
		pass = &ss
	}
	return common.PrivateKeyFromBytes(r.PrivateKeyPem, pass)
}
