// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package identity

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/peertls"
	"storj.io/common/peertls/extensions"
	"storj.io/common/pkcrypto"
	"storj.io/common/rpc/rpcpeer"
	"storj.io/common/storj"
)

// PeerIdentity represents another peer on the network.
type PeerIdentity struct {
	RestChain []*x509.Certificate
	// CA represents the peer's self-signed CA.
	CA *x509.Certificate
	// Leaf represents the leaf they're currently using. The leaf should be
	// signed by the CA. The leaf is what is used for communication.
	Leaf *x509.Certificate
	// The ID taken from the CA public key.
	ID storj.NodeID
}

// FullIdentity represents you on the network. In addition to a PeerIdentity,
// a FullIdentity also has a Key, which a PeerIdentity doesn't have.
type FullIdentity struct {
	RestChain []*x509.Certificate
	// CA represents the peer's self-signed CA. The ID is taken from this cert.
	CA *x509.Certificate
	// Leaf represents the leaf they're currently using. The leaf should be
	// signed by the CA. The leaf is what is used for communication.
	Leaf *x509.Certificate
	// The ID taken from the CA public key.
	ID storj.NodeID
	// Key is the key this identity uses with the leaf for communication.
	Key crypto.PrivateKey
}

// ManageablePeerIdentity is a `PeerIdentity` and its corresponding `FullCertificateAuthority`
// in a single struct. It is used for making changes to the identity that require CA
// authorization; e.g. adding extensions.
type ManageablePeerIdentity struct {
	*PeerIdentity
	CA *FullCertificateAuthority
}

// ManageableFullIdentity is a `FullIdentity` and its corresponding `FullCertificateAuthority`
// in a single struct. It is used for making changes to the identity that require CA
// authorization and the leaf private key; e.g. revoking a leaf cert (private key changes).
type ManageableFullIdentity struct {
	*FullIdentity
	CA *FullCertificateAuthority
}

// SetupConfig allows you to run a set of Responsibilities with the given
// identity. You can also just load an Identity from disk.
type SetupConfig struct {
	CertPath  string `help:"path to the certificate chain for this identity" default:"$IDENTITYDIR/identity.cert" path:"true"`
	KeyPath   string `help:"path to the private key for this identity" default:"$IDENTITYDIR/identity.key" path:"true"`
	Overwrite bool   `help:"if true, existing identity certs AND keys will overwritten for" default:"false" setup:"true"`
	Version   string `help:"semantic version of identity storage format" default:"0"`
}

// Config allows you to run a set of Responsibilities with the given
// identity. You can also just load an Identity from disk.
type Config struct {
	CertPath string `help:"path to the certificate chain for this identity" default:"$IDENTITYDIR/identity.cert" user:"true" path:"true"`
	KeyPath  string `help:"path to the private key for this identity" default:"$IDENTITYDIR/identity.key" user:"true" path:"true"`
}

// PeerConfig allows you to interact with a peer identity (cert, no key) on disk.
type PeerConfig struct {
	CertPath string `help:"path to the certificate chain for this identity" default:"$IDENTITYDIR/identity.cert" user:"true" path:"true"`
}

// FullCertificateAuthorityFromPEM loads a FullIdentity from a certificate chain and
// private key PEM-encoded bytes.
func FullCertificateAuthorityFromPEM(chainPEM, keyPEM []byte) (*FullCertificateAuthority, error) {
	peerCA, err := PeerCertificateAuthorityFromPEM(chainPEM)
	if err != nil {
		return nil, err
	}

	// NB: there shouldn't be multiple keys in the key file but if there
	// are, this uses the first one
	key, err := pkcrypto.PrivateKeyFromPEM(keyPEM)
	if err != nil {
		return nil, err
	}

	return &FullCertificateAuthority{
		RestChain: peerCA.RestChain,
		Cert:      peerCA.Cert,
		Key:       key,
		ID:        peerCA.ID,
	}, nil
}

// PeerCertificateAuthorityFromPEM loads a FullIdentity from a certificate chain and
// private key PEM-encoded bytes.
func PeerCertificateAuthorityFromPEM(chainPEM []byte) (*PeerCertificateAuthority, error) {
	chain, err := pkcrypto.CertsFromPEM(chainPEM)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	// NB: the "leaf" cert in a CA chain is the "CA" cert in an identity chain
	nodeID, err := NodeIDFromCert(chain[peertls.LeafIndex])
	if err != nil {
		return nil, err
	}

	return &PeerCertificateAuthority{
		RestChain: chain[peertls.CAIndex:],
		Cert:      chain[peertls.LeafIndex],
		ID:        nodeID,
	}, nil
}

// FullIdentityFromPEM loads a FullIdentity from a certificate chain and
// private key PEM-encoded bytes.
func FullIdentityFromPEM(chainPEM, keyPEM []byte) (*FullIdentity, error) {
	peerIdent, err := PeerIdentityFromPEM(chainPEM)
	if err != nil {
		return nil, err
	}

	// NB: there shouldn't be multiple keys in the key file but if there
	// are, this uses the first one
	key, err := pkcrypto.PrivateKeyFromPEM(keyPEM)
	if err != nil {
		return nil, err
	}

	return &FullIdentity{
		RestChain: peerIdent.RestChain,
		CA:        peerIdent.CA,
		Leaf:      peerIdent.Leaf,
		Key:       key,
		ID:        peerIdent.ID,
	}, nil
}

// PeerIdentityFromPEM loads a PeerIdentity from a certificate chain and
// private key PEM-encoded bytes.
func PeerIdentityFromPEM(chainPEM []byte) (*PeerIdentity, error) {
	chain, err := pkcrypto.CertsFromPEM(chainPEM)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	if len(chain) < peertls.CAIndex+1 {
		return nil, pkcrypto.ErrChainLength.New("identity chain does not contain a CA certificate")
	}
	return PeerIdentityFromChain(chain)
}

// PeerIdentityFromChain loads a PeerIdentity from an identity certificate chain.
func PeerIdentityFromChain(chain []*x509.Certificate) (*PeerIdentity, error) {
	nodeID, err := NodeIDFromCert(chain[peertls.CAIndex])
	if err != nil {
		return nil, err
	}

	peer := &PeerIdentity{
		RestChain: chain[peertls.CAIndex+1:],
		CA:        chain[peertls.CAIndex],
		ID:        nodeID,
		Leaf:      chain[peertls.LeafIndex],
	}

	err = peer.Leaf.CheckSignatureFrom(peer.CA)
	if err != nil {
		return nil, Error.New("certificate chain invalid: %w", err)
	}

	return peer, nil
}

// PeerIdentityFromPeer loads a PeerIdentity from a peer connection.
func PeerIdentityFromPeer(peer *rpcpeer.Peer) (*PeerIdentity, error) {
	chain := peer.State.PeerCertificates
	if len(chain)-1 < peertls.CAIndex {
		return nil, Error.New("invalid certificate chain")
	}
	pi, err := PeerIdentityFromChain(chain)
	if err != nil {
		return nil, err
	}
	return pi, nil
}

// PeerIdentityFromContext loads a PeerIdentity from a ctx TLS credentials.
func PeerIdentityFromContext(ctx context.Context) (*PeerIdentity, error) {
	peer, err := rpcpeer.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	return PeerIdentityFromPeer(peer)
}

// NodeIDFromCertPath loads a node ID from a certificate file path.
func NodeIDFromCertPath(certPath string) (storj.NodeID, error) {
	/* #nosec G304 */ // Subsequent calls ensure that the file is a certificate
	certBytes, err := os.ReadFile(certPath)
	if err != nil {
		return storj.NodeID{}, err
	}
	return NodeIDFromPEM(certBytes)
}

// NodeIDFromPEM loads a node ID from certificate bytes.
func NodeIDFromPEM(pemBytes []byte) (storj.NodeID, error) {
	chain, err := pkcrypto.CertsFromPEM(pemBytes)
	if err != nil {
		return storj.NodeID{}, Error.New("invalid identity certificate")
	}
	if len(chain)-1 < peertls.CAIndex {
		return storj.NodeID{}, Error.New("no CA in identity certificate")
	}
	return NodeIDFromCert(chain[peertls.CAIndex])
}

// NodeIDFromCert looks for a version in an ID version extension in the passed
// cert and then calculates a versioned node ID using the certificate public key.
// NB: `cert` would typically be an identity's certificate authority certificate.
func NodeIDFromCert(cert *x509.Certificate) (id storj.NodeID, err error) {
	version, err := storj.IDVersionFromCert(cert)
	if err != nil {
		return id, Error.Wrap(err)
	}
	return NodeIDFromKey(cert.PublicKey, version)
}

// NodeIDFromKey calculates the node ID for a given public key with the passed version.
func NodeIDFromKey(k crypto.PublicKey, version storj.IDVersion) (storj.NodeID, error) {
	idBytes, err := peertls.DoubleSHA256PublicKey(k)
	if err != nil {
		return storj.NodeID{}, storj.ErrNodeID.Wrap(err)
	}
	return storj.NewVersionedID(idBytes, version), nil
}

// NewFullIdentity creates a new ID for nodes with difficulty and concurrency params.
func NewFullIdentity(ctx context.Context, opts NewCAOptions) (*FullIdentity, error) {
	ca, err := NewCA(ctx, opts)
	if err != nil {
		return nil, err
	}
	identity, err := ca.NewIdentity()
	if err != nil {
		return nil, err
	}
	return identity, err
}

// ToChains takes a number of certificate chains and returns them as a 2d slice of chains of certificates.
func ToChains(chains ...[]*x509.Certificate) [][]*x509.Certificate {
	combinedChains := make([][]*x509.Certificate, len(chains))
	copy(combinedChains, chains)
	return combinedChains
}

// NewManageablePeerIdentity returns a manageable identity given a full identity and a full certificate authority.
func NewManageablePeerIdentity(ident *PeerIdentity, ca *FullCertificateAuthority) *ManageablePeerIdentity {
	return &ManageablePeerIdentity{
		PeerIdentity: ident,
		CA:           ca,
	}
}

// NewManageableFullIdentity returns a manageable identity given a full identity and a full certificate authority.
func NewManageableFullIdentity(ident *FullIdentity, ca *FullCertificateAuthority) *ManageableFullIdentity {
	return &ManageableFullIdentity{
		FullIdentity: ident,
		CA:           ca,
	}
}

// Status returns the status of the identity cert/key files for the config.
func (is SetupConfig) Status() (TLSFilesStatus, error) {
	return statTLSFiles(is.CertPath, is.KeyPath)
}

// Create generates and saves a CA using the config.
func (is SetupConfig) Create(ca *FullCertificateAuthority) (*FullIdentity, error) {
	fi, err := ca.NewIdentity()
	if err != nil {
		return nil, err
	}
	fi.CA = ca.Cert
	ic := Config{
		CertPath: is.CertPath,
		KeyPath:  is.KeyPath,
	}
	return fi, ic.Save(fi)
}

// FullConfig converts a `SetupConfig` to `Config`.
func (is SetupConfig) FullConfig() Config {
	return Config{
		CertPath: is.CertPath,
		KeyPath:  is.KeyPath,
	}
}

// Load loads a FullIdentity from the config.
func (ic Config) Load() (*FullIdentity, error) {
	c, err := os.ReadFile(ic.CertPath)
	if err != nil {
		return nil, peertls.ErrNotExist.Wrap(err)
	}
	k, err := os.ReadFile(ic.KeyPath)
	if err != nil {
		return nil, peertls.ErrNotExist.Wrap(err)
	}
	fi, err := FullIdentityFromPEM(c, k)
	if err != nil {
		return nil, errs.New("failed to load identity %#v, %#v: %v",
			ic.CertPath, ic.KeyPath, err)
	}
	return fi, nil
}

// Save saves a FullIdentity according to the config.
func (ic Config) Save(fi *FullIdentity) error {
	var (
		certData, keyData                                              bytes.Buffer
		writeChainErr, writeChainDataErr, writeKeyErr, writeKeyDataErr error
	)

	chain := []*x509.Certificate{fi.Leaf, fi.CA}
	chain = append(chain, fi.RestChain...)

	if ic.CertPath != "" {
		writeChainErr = peertls.WriteChain(&certData, chain...)
		writeChainDataErr = writeChainData(ic.CertPath, certData.Bytes())
	}

	if ic.KeyPath != "" {
		writeKeyErr = pkcrypto.WritePrivateKeyPEM(&keyData, fi.Key)
		writeKeyDataErr = writeKeyData(ic.KeyPath, keyData.Bytes())
	}

	writeErr := errs.Combine(writeChainErr, writeKeyErr)
	if writeErr != nil {
		return writeErr
	}

	return errs.Combine(
		writeChainDataErr,
		writeKeyDataErr,
	)
}

// SaveBackup saves the certificate of the config with a timestamped filename.
func (ic Config) SaveBackup(fi *FullIdentity) error {
	return Config{
		CertPath: backupPath(ic.CertPath),
		KeyPath:  backupPath(ic.KeyPath),
	}.Save(fi)
}

// PeerConfig converts a Config to a PeerConfig.
func (ic Config) PeerConfig() *PeerConfig {
	return &PeerConfig{
		CertPath: ic.CertPath,
	}
}

// Load loads a PeerIdentity from the config.
func (ic PeerConfig) Load() (*PeerIdentity, error) {
	c, err := os.ReadFile(ic.CertPath)
	if err != nil {
		return nil, peertls.ErrNotExist.Wrap(err)
	}
	pi, err := PeerIdentityFromPEM(c)
	if err != nil {
		return nil, errs.New("failed to load identity %#v: %v", ic.CertPath, err)
	}
	return pi, nil
}

// Save saves a PeerIdentity according to the config.
func (ic PeerConfig) Save(peerIdent *PeerIdentity) error {
	chain := []*x509.Certificate{peerIdent.Leaf, peerIdent.CA}
	chain = append(chain, peerIdent.RestChain...)

	if ic.CertPath != "" {
		var certData bytes.Buffer
		err := peertls.WriteChain(&certData, chain...)
		if err != nil {
			return err
		}

		return writeChainData(ic.CertPath, certData.Bytes())
	}

	return nil
}

// SaveBackup saves the certificate of the config with a timestamped filename.
func (ic PeerConfig) SaveBackup(pi *PeerIdentity) error {
	return PeerConfig{
		CertPath: backupPath(ic.CertPath),
	}.Save(pi)
}

// Chain returns the Identity's certificate chain.
func (fi *FullIdentity) Chain() []*x509.Certificate {
	return append([]*x509.Certificate{fi.Leaf, fi.CA}, fi.RestChain...)
}

// RawChain returns all of the certificate chain as a 2d byte slice.
func (fi *FullIdentity) RawChain() [][]byte {
	chain := fi.Chain()
	rawChain := make([][]byte, len(chain))
	for i, cert := range chain {
		rawChain[i] = cert.Raw
	}
	return rawChain
}

// RawRestChain returns the rest (excluding leaf and CA) of the certificate chain as a 2d byte slice.
func (fi *FullIdentity) RawRestChain() [][]byte {
	rawChain := make([][]byte, len(fi.RestChain))
	for i, cert := range fi.RestChain {
		rawChain[i] = cert.Raw
	}
	return rawChain
}

// PeerIdentity converts a FullIdentity into a PeerIdentity.
func (fi *FullIdentity) PeerIdentity() *PeerIdentity {
	return &PeerIdentity{
		CA:        fi.CA,
		Leaf:      fi.Leaf,
		ID:        fi.ID,
		RestChain: fi.RestChain,
	}
}

// Version looks up the version based on the certificate's ID version extension.
func (fi *FullIdentity) Version() (storj.IDVersion, error) {
	return storj.IDVersionFromCert(fi.CA)
}

// AddExtension adds extensions to the leaf cert of an identity. Extensions
// are serialized into the certificate's raw bytes and is re-signed by it's
// certificate authority.
func (manageableIdent *ManageablePeerIdentity) AddExtension(ext ...pkix.Extension) error {
	if err := extensions.AddExtraExtension(manageableIdent.Leaf, ext...); err != nil {
		return err
	}

	updatedCert, err := peertls.CreateCertificate(manageableIdent.Leaf.PublicKey, manageableIdent.CA.Key, manageableIdent.Leaf, manageableIdent.CA.Cert)
	if err != nil {
		return err
	}

	manageableIdent.Leaf = updatedCert
	return nil
}

// Revoke extends the CA certificate with a certificate revocation extension.
func (manageableIdent *ManageableFullIdentity) Revoke() error {
	ext, err := extensions.NewRevocationExt(manageableIdent.CA.Key, manageableIdent.Leaf)
	if err != nil {
		return err
	}

	revokingIdent, err := manageableIdent.CA.NewIdentity(ext)
	if err != nil {
		return err
	}

	manageableIdent.Leaf = revokingIdent.Leaf

	return nil
}

func backupPath(path string) string {
	pathExt := filepath.Ext(path)
	base := strings.TrimSuffix(path, pathExt)
	return fmt.Sprintf(
		"%s.%s%s",
		base,
		strconv.Itoa(int(time.Now().Unix())),
		pathExt,
	)
}

// EncodePeerIdentity encodes the complete identity chain to bytes.
func EncodePeerIdentity(pi *PeerIdentity) []byte {
	var chain []byte
	chain = append(chain, pi.Leaf.Raw...)
	chain = append(chain, pi.CA.Raw...)
	for _, cert := range pi.RestChain {
		chain = append(chain, cert.Raw...)
	}
	return chain
}

// DecodePeerIdentity Decodes the bytes into complete identity chain.
func DecodePeerIdentity(ctx context.Context, chain []byte) (_ *PeerIdentity, err error) {
	defer mon.Task()(&ctx)(&err)

	var certs []*x509.Certificate
	for len(chain) > 0 {
		var raw asn1.RawValue
		var err error

		chain, err = asn1.Unmarshal(chain, &raw)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		cert, err := pkcrypto.CertFromDER(raw.FullBytes)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		certs = append(certs, cert)
	}
	if len(certs) < 2 {
		return nil, Error.New("not enough certificates")
	}
	return PeerIdentityFromChain(certs)
}
