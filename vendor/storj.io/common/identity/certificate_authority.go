// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package identity

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"

	"github.com/zeebo/errs"

	"storj.io/common/peertls"
	"storj.io/common/peertls/extensions"
	"storj.io/common/pkcrypto"
	"storj.io/common/storj"
)

const minimumLoggableDifficulty = 8

// PeerCertificateAuthority represents the CA which is used to validate peer identities.
type PeerCertificateAuthority struct {
	RestChain []*x509.Certificate
	// Cert is the x509 certificate of the CA
	Cert *x509.Certificate
	// The ID is calculated from the CA public key.
	ID storj.NodeID
}

// FullCertificateAuthority represents the CA which is used to author and validate full identities.
type FullCertificateAuthority struct {
	RestChain []*x509.Certificate
	// Cert is the x509 certificate of the CA
	Cert *x509.Certificate
	// The ID is calculated from the CA public key.
	ID storj.NodeID
	// Key is the private key of the CA
	Key crypto.PrivateKey
}

// CASetupConfig is for creating a CA.
type CASetupConfig struct {
	VersionNumber  uint   `default:"0" help:"which identity version to use (0 is latest)"`
	ParentCertPath string `help:"path to the parent authority's certificate chain"`
	ParentKeyPath  string `help:"path to the parent authority's private key"`
	CertPath       string `help:"path to the certificate chain for this identity" default:"$IDENTITYDIR/ca.cert"`
	KeyPath        string `help:"path to the private key for this identity" default:"$IDENTITYDIR/ca.key"`
	Difficulty     uint64 `help:"minimum difficulty for identity generation" default:"36"`
	Timeout        string `help:"timeout for CA generation; golang duration string (0 no timeout)" default:"5m"`
	Overwrite      bool   `help:"if true, existing CA certs AND keys will overwritten" default:"false" setup:"true"`
	Concurrency    uint   `help:"number of concurrent workers for certificate authority generation" default:"4"`
}

// NewCAOptions is used to pass parameters to `NewCA`.
type NewCAOptions struct {
	// VersionNumber is the IDVersion to use for the identity
	VersionNumber storj.IDVersionNumber
	// Difficulty is the number of trailing zero-bits the nodeID must have
	Difficulty uint16
	// Concurrency is the number of go routines used to generate a CA of sufficient difficulty
	Concurrency uint
	// ParentCert, if provided will be prepended to the certificate chain
	ParentCert *x509.Certificate
	// ParentKey ()
	ParentKey crypto.PrivateKey
	// Logger is used to log generation status updates
	Logger io.Writer
}

// PeerCAConfig is for locating a CA certificate without a private key.
type PeerCAConfig struct {
	CertPath string `help:"path to the certificate chain for this identity" default:"$IDENTITYDIR/ca.cert"`
}

// FullCAConfig is for locating a CA certificate and it's private key.
type FullCAConfig struct {
	CertPath string `help:"path to the certificate chain for this identity" default:"$IDENTITYDIR/ca.cert"`
	KeyPath  string `help:"path to the private key for this identity" default:"$IDENTITYDIR/ca.key"`
}

// NewCA creates a new full identity with the given difficulty.
func NewCA(ctx context.Context, opts NewCAOptions) (_ *FullCertificateAuthority, err error) {
	defer mon.Task()(&ctx)(&err)
	var (
		highscore = new(uint32)
		i         = new(uint32)

		mu          sync.Mutex
		selectedKey crypto.PrivateKey
		selectedID  storj.NodeID
	)

	if opts.Concurrency < 1 {
		opts.Concurrency = 1
	}

	if opts.Logger != nil {
		fmt.Fprintf(opts.Logger, "Generating key with a minimum a difficulty of %d...\n", opts.Difficulty)
	}

	version, err := storj.GetIDVersion(opts.VersionNumber)
	if err != nil {
		return nil, err
	}

	updateStatus := func() {
		if opts.Logger != nil {
			count := atomic.LoadUint32(i)
			hs := atomic.LoadUint32(highscore)
			_, _ = fmt.Fprintf(opts.Logger, "\rGenerated %d keys; best difficulty so far: %d", count, hs)
		}
	}
	err = GenerateKeys(ctx, minimumLoggableDifficulty, int(opts.Concurrency), version,
		func(k crypto.PrivateKey, id storj.NodeID) (done bool, err error) {
			if opts.Logger != nil {
				if atomic.AddUint32(i, 1)%100 == 0 {
					updateStatus()
				}
			}

			difficulty, err := id.Difficulty()
			if err != nil {
				return false, err
			}
			if difficulty >= opts.Difficulty {
				mu.Lock()
				if selectedKey == nil {
					updateStatus()
					selectedKey = k
					selectedID = id
				}
				mu.Unlock()
				if opts.Logger != nil {
					atomic.SwapUint32(highscore, uint32(difficulty))
					updateStatus()
					_, _ = fmt.Fprintf(opts.Logger, "\nFound a key with difficulty %d!\n", difficulty)
				}
				return true, nil
			}
			for {
				hs := atomic.LoadUint32(highscore)
				if uint32(difficulty) <= hs {
					return false, nil
				}
				if atomic.CompareAndSwapUint32(highscore, hs, uint32(difficulty)) {
					updateStatus()
					return false, nil
				}
			}
		})
	if err != nil {
		return nil, err
	}

	ct, err := peertls.CATemplate()
	if err != nil {
		return nil, err
	}

	if err := extensions.AddExtraExtension(ct, storj.NewVersionExt(version)); err != nil {
		return nil, err
	}

	var cert *x509.Certificate
	if opts.ParentKey == nil {
		cert, err = peertls.CreateSelfSignedCertificate(selectedKey, ct)
	} else {
		var pubKey crypto.PublicKey
		pubKey, err = pkcrypto.PublicKeyFromPrivate(selectedKey)
		if err != nil {
			return nil, err
		}
		cert, err = peertls.CreateCertificate(pubKey, opts.ParentKey, ct, opts.ParentCert)
	}
	if err != nil {
		return nil, err
	}

	ca := &FullCertificateAuthority{
		Cert: cert,
		Key:  selectedKey,
		ID:   selectedID,
	}
	if opts.ParentCert != nil {
		ca.RestChain = []*x509.Certificate{opts.ParentCert}
	}
	return ca, nil
}

// Status returns the status of the CA cert/key files for the config.
func (caS CASetupConfig) Status() (TLSFilesStatus, error) {
	return statTLSFiles(caS.CertPath, caS.KeyPath)
}

// Create generates and saves a CA using the config.
func (caS CASetupConfig) Create(ctx context.Context, logger io.Writer) (*FullCertificateAuthority, error) {
	var (
		err    error
		parent *FullCertificateAuthority
	)
	if caS.ParentCertPath != "" && caS.ParentKeyPath != "" {
		parent, err = FullCAConfig{
			CertPath: caS.ParentCertPath,
			KeyPath:  caS.ParentKeyPath,
		}.Load()
		if err != nil {
			return nil, err
		}
	}

	if parent == nil {
		parent = &FullCertificateAuthority{}
	}

	version, err := storj.GetIDVersion(storj.IDVersionNumber(caS.VersionNumber))
	if err != nil {
		return nil, err
	}

	ca, err := NewCA(ctx, NewCAOptions{
		VersionNumber: version.Number,
		Difficulty:    uint16(caS.Difficulty),
		Concurrency:   caS.Concurrency,
		ParentCert:    parent.Cert,
		ParentKey:     parent.Key,
		Logger:        logger,
	})
	if err != nil {
		return nil, err
	}
	caC := FullCAConfig{
		CertPath: caS.CertPath,
		KeyPath:  caS.KeyPath,
	}
	return ca, caC.Save(ca)
}

// FullConfig converts a `CASetupConfig` to `FullCAConfig`.
func (caS CASetupConfig) FullConfig() FullCAConfig {
	return FullCAConfig{
		CertPath: caS.CertPath,
		KeyPath:  caS.KeyPath,
	}
}

// Load loads a CA from the given configuration.
func (fc FullCAConfig) Load() (*FullCertificateAuthority, error) {
	p, err := fc.PeerConfig().Load()
	if err != nil {
		return nil, err
	}

	kb, err := os.ReadFile(fc.KeyPath)
	if err != nil {
		return nil, peertls.ErrNotExist.Wrap(err)
	}
	k, err := pkcrypto.PrivateKeyFromPEM(kb)
	if err != nil {
		return nil, err
	}

	return &FullCertificateAuthority{
		RestChain: p.RestChain,
		Cert:      p.Cert,
		Key:       k,
		ID:        p.ID,
	}, nil
}

// PeerConfig converts a full ca config to a peer ca config.
func (fc FullCAConfig) PeerConfig() PeerCAConfig {
	return PeerCAConfig{
		CertPath: fc.CertPath,
	}
}

// Save saves a CA with the given configuration.
func (fc FullCAConfig) Save(ca *FullCertificateAuthority) error {
	var (
		keyData   bytes.Buffer
		writeErrs errs.Group
	)
	if err := fc.PeerConfig().Save(ca.PeerCA()); err != nil {
		writeErrs.Add(err)
		return writeErrs.Err()
	}

	if fc.KeyPath != "" {
		if err := pkcrypto.WritePrivateKeyPEM(&keyData, ca.Key); err != nil {
			writeErrs.Add(err)
			return writeErrs.Err()
		}
		if err := writeKeyData(fc.KeyPath, keyData.Bytes()); err != nil {
			writeErrs.Add(err)
			return writeErrs.Err()
		}
	}

	return writeErrs.Err()
}

// SaveBackup saves the certificate of the config wth a timestamped filename.
func (fc FullCAConfig) SaveBackup(ca *FullCertificateAuthority) error {
	return FullCAConfig{
		CertPath: backupPath(fc.CertPath),
		KeyPath:  backupPath(fc.KeyPath),
	}.Save(ca)
}

// Load loads a CA from the given configuration.
func (pc PeerCAConfig) Load() (*PeerCertificateAuthority, error) {
	chainPEM, err := os.ReadFile(pc.CertPath)
	if err != nil {
		return nil, peertls.ErrNotExist.Wrap(err)
	}

	chain, err := pkcrypto.CertsFromPEM(chainPEM)
	if err != nil {
		return nil, errs.New("failed to load identity %#v: %v",
			pc.CertPath, err)
	}

	// NB: `CAIndex` is in the context of a complete chain (incl. leaf).
	// Here we're loading the CA chain (i.e. without leaf).
	nodeID, err := NodeIDFromCert(chain[peertls.CAIndex-1])
	if err != nil {
		return nil, err
	}

	return &PeerCertificateAuthority{
		// NB: `CAIndex` is in the context of a complete chain (incl. leaf).
		// Here we're loading the CA chain (i.e. without leaf).
		RestChain: chain[peertls.CAIndex:],
		Cert:      chain[peertls.CAIndex-1],
		ID:        nodeID,
	}, nil
}

// Save saves a peer CA (cert, no key) with the given configuration.
func (pc PeerCAConfig) Save(ca *PeerCertificateAuthority) error {
	var (
		certData  bytes.Buffer
		writeErrs errs.Group
	)

	chain := []*x509.Certificate{ca.Cert}
	chain = append(chain, ca.RestChain...)

	if pc.CertPath != "" {
		if err := peertls.WriteChain(&certData, chain...); err != nil {
			writeErrs.Add(err)
			return writeErrs.Err()
		}
		if err := writeChainData(pc.CertPath, certData.Bytes()); err != nil {
			writeErrs.Add(err)
			return writeErrs.Err()
		}
	}
	return nil
}

// SaveBackup saves the certificate of the config wth a timestamped filename.
func (pc PeerCAConfig) SaveBackup(ca *PeerCertificateAuthority) error {
	return PeerCAConfig{
		CertPath: backupPath(pc.CertPath),
	}.Save(ca)
}

// NewIdentity generates a new `FullIdentity` based on the CA. The CA
// cert is included in the identity's cert chain and the identity's leaf cert
// is signed by the CA.
func (ca *FullCertificateAuthority) NewIdentity(exts ...pkix.Extension) (*FullIdentity, error) {
	leafTemplate, err := peertls.LeafTemplate()
	if err != nil {
		return nil, err
	}
	// TODO: add test for this!
	version, err := ca.Version()
	if err != nil {
		return nil, err
	}
	leafKey, err := version.NewPrivateKey()
	if err != nil {
		return nil, err
	}

	if err := extensions.AddExtraExtension(leafTemplate, exts...); err != nil {
		return nil, err
	}

	pubKey, err := pkcrypto.PublicKeyFromPrivate(leafKey)
	if err != nil {
		return nil, err
	}

	leafCert, err := peertls.CreateCertificate(pubKey, ca.Key, leafTemplate, ca.Cert)
	if err != nil {
		return nil, err
	}

	return &FullIdentity{
		RestChain: ca.RestChain,
		CA:        ca.Cert,
		Leaf:      leafCert,
		Key:       leafKey,
		ID:        ca.ID,
	}, nil

}

// Chain returns the CA's certificate chain.
func (ca *FullCertificateAuthority) Chain() []*x509.Certificate {
	return append([]*x509.Certificate{ca.Cert}, ca.RestChain...)
}

// RawChain returns the CA's certificate chain as a 2d byte slice.
func (ca *FullCertificateAuthority) RawChain() [][]byte {
	chain := ca.Chain()
	rawChain := make([][]byte, len(chain))
	for i, cert := range chain {
		rawChain[i] = cert.Raw
	}
	return rawChain
}

// RawRestChain returns the "rest" (excluding `ca.Cert`) of the certificate chain as a 2d byte slice.
func (ca *FullCertificateAuthority) RawRestChain() [][]byte {
	var chain [][]byte
	for _, cert := range ca.RestChain {
		chain = append(chain, cert.Raw)
	}
	return chain
}

// PeerCA converts a FullCertificateAuthority to a PeerCertificateAuthority.
func (ca *FullCertificateAuthority) PeerCA() *PeerCertificateAuthority {
	return &PeerCertificateAuthority{
		Cert:      ca.Cert,
		ID:        ca.ID,
		RestChain: ca.RestChain,
	}
}

// Sign signs the passed certificate with ca certificate.
func (ca *FullCertificateAuthority) Sign(cert *x509.Certificate) (*x509.Certificate, error) {
	signedCert, err := peertls.CreateCertificate(cert.PublicKey, ca.Key, cert, ca.Cert)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return signedCert, nil
}

// Version looks up the version based on the certificate's ID version extension.
func (ca *FullCertificateAuthority) Version() (storj.IDVersion, error) {
	return storj.IDVersionFromCert(ca.Cert)
}

// AddExtension adds extensions to certificate authority certificate. Extensions
// are serialized into the certificate's raw bytes and it is re-signed by itself.
func (ca *FullCertificateAuthority) AddExtension(exts ...pkix.Extension) error {
	// TODO: how to properly handle this?
	if len(ca.RestChain) > 0 {
		return errs.New("adding extensions requires parent certificate's private key")
	}

	if err := extensions.AddExtraExtension(ca.Cert, exts...); err != nil {
		return err
	}

	updatedCert, err := peertls.CreateSelfSignedCertificate(ca.Key, ca.Cert)
	if err != nil {
		return err
	}

	ca.Cert = updatedCert
	return nil
}

// Revoke extends the certificate authority certificate with a certificate revocation extension.
func (ca *FullCertificateAuthority) Revoke() error {
	ext, err := extensions.NewRevocationExt(ca.Key, ca.Cert)
	if err != nil {
		return err
	}

	return ca.AddExtension(ext)
}
