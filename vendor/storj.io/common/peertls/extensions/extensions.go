// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package extensions

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"

	"github.com/zeebo/errs"

	"storj.io/common/peertls"
	"storj.io/common/pkcrypto"
)

const (
	// RevocationBucket is the bolt bucket to store revocation data in
	RevocationBucket = "revocations"
)

var (
	// DefaultHandlers is a slice of handlers that we use by default.
	//   - IDVersionHandler
	DefaultHandlers HandlerFactories

	// CAWhitelistSignedLeafHandler verifies that the leaf cert of the remote peer's
	// identity was signed by one of the CA certs in the whitelist.
	CAWhitelistSignedLeafHandler = NewHandlerFactory(
		&SignedCertExtID, caWhitelistSignedLeafHandler,
	)

	// NB: 2.999.X is reserved for "example" OIDs
	// (see http://oid-info.com/get/2.999)
	// 2.999.1.X -- storj general/misc. extensions
	// 2.999.2.X -- storj identity extensions

	// SignedCertExtID is the asn1 object ID for a pkix extension holding a
	// signature of the cert it's extending, signed by some CA (e.g. the root cert chain).
	// This extensionHandler allows for an additional signature per certificate.
	SignedCertExtID = ExtensionID{2, 999, 1, 1}
	// RevocationExtID is the asn1 object ID for a pkix extension containing the
	// most recent certificate revocation data
	// for the current TLS cert chain.
	RevocationExtID = ExtensionID{2, 999, 1, 2}
	// IdentityVersionExtID is the asn1 object ID for a pkix extension that
	// specifies the identity version of the certificate chain.
	IdentityVersionExtID = ExtensionID{2, 999, 2, 1}
	// IdentityPOWCounterExtID is the asn1 object ID for a pkix extension that
	// specifies how many times to hash the CA public key to calculate the node ID.
	IdentityPOWCounterExtID = ExtensionID{2, 999, 2, 2}

	// Error is used when an error occurs while processing an extension.
	Error = errs.Class("extension error")

	// ErrVerifyCASignedLeaf is used when a signed leaf extension signature wasn't produced
	// by any CA in the whitelist.
	ErrVerifyCASignedLeaf = Error.New("leaf not signed by any CA in the whitelist")
	// ErrUniqueExtensions is used when multiple extensions have the same Id
	ErrUniqueExtensions = Error.New("extensions are not unique")
)

// ExtensionID is an alias to an `asn1.ObjectIdentifier`.
type ExtensionID = asn1.ObjectIdentifier

// Config is used to bind cli flags for determining which extensions will
// be used by the server.
type Config struct {
	Revocation          bool `default:"true" help:"if true, client leaves may contain the most recent certificate revocation for the current certificate"`
	WhitelistSignedLeaf bool `default:"false" help:"if true, client leaves must contain a valid \"signed certificate extension\" (NB: verified against certs in the peer ca whitelist; i.e. if true, a whitelist must be provided)"`
}

// Options holds common options for use in handling extensions.
type Options struct {
	PeerCAWhitelist []*x509.Certificate
	RevocationDB    RevocationDB
	PeerIDVersions  string
}

// HandlerFactories is a collection of `HandlerFactory`s for convenience.
// Defines `Register` and `WithOptions` methods.
type HandlerFactories []*HandlerFactory

// HandlerFactory holds a factory for a handler function given the passed `Options`.
// For use in handling extensions with the corresponding ExtensionID.
type HandlerFactory struct {
	id      *ExtensionID
	factory HandlerFactoryFunc
}

// HandlerFactoryFunc is a factory function used to build `HandlerFunc`s given
// the passed options.
type HandlerFactoryFunc func(options *Options) HandlerFunc

// HandlerFunc takes an extension and the remote peer's certificate chains for
// use in extension handling.
type HandlerFunc func(pkix.Extension, [][]*x509.Certificate) error

// HandlerFuncMap maps an `ExtensionID` pointer to a `HandlerFunc`.
// Because an `ExtensionID` is a pointer , one can use a new pointer to the same
// asn1 object ID constant to store multiple `HandlerFunc`s for the same
// underlying extension id value.
type HandlerFuncMap map[*ExtensionID]HandlerFunc

// NewHandlerFactory builds a `HandlerFactory` pointer from an `ExtensionID` and a `HandlerFactoryFunc`.
func NewHandlerFactory(id *ExtensionID, handlerFactory HandlerFactoryFunc) *HandlerFactory {
	return &HandlerFactory{
		id:      id,
		factory: handlerFactory,
	}
}

// AddExtraExtension adds one or more extensions to a certificate for serialization.
// NB: this *does not* serialize or persist the extension into the certificates's
// raw bytes. To add a persistent extension use `FullCertificateAuthority.AddExtension`
// or `ManageableIdentity.AddExtension`.
func AddExtraExtension(cert *x509.Certificate, exts ...pkix.Extension) (err error) {
	if len(exts) == 0 {
		return nil
	}
	if !uniqueExts(append(cert.ExtraExtensions, exts...)) {
		return ErrUniqueExtensions
	}

	for _, ext := range exts {
		e := pkix.Extension{Id: ext.Id, Value: make([]byte, len(ext.Value))}
		copy(e.Value, ext.Value)
		cert.ExtraExtensions = append(cert.ExtraExtensions, e)
	}
	return nil
}

// Register adds an extension handler factory to the list.
func (factories *HandlerFactories) Register(newHandlers ...*HandlerFactory) {
	*factories = append(*factories, newHandlers...)
}

// WithOptions builds a `HandlerFuncMap` by calling each `HandlerFactory` with
// the passed `Options` pointer and using the respective `ExtensionID` pointer
// as the key.
func (factories HandlerFactories) WithOptions(opts *Options) HandlerFuncMap {
	handlerFuncMap := make(HandlerFuncMap)
	for _, factory := range factories {
		handlerFuncMap[factory.ID()] = factory.NewHandlerFunc(opts)
	}
	return handlerFuncMap
}

// ID returns the `ExtensionID` pointer stored with this factory. This factory
// will only handle extensions that have a matching id value.
func (handlerFactory *HandlerFactory) ID() *ExtensionID {
	return handlerFactory.id
}

// NewHandlerFunc returns a new `HandlerFunc` with the passed `Options`.
func (handlerFactory *HandlerFactory) NewHandlerFunc(opts *Options) HandlerFunc {
	return handlerFactory.factory(opts)
}

func uniqueExts(exts []pkix.Extension) bool {
	seen := make(map[string]struct{}, len(exts))
	for _, e := range exts {
		s := e.Id.String()
		if _, ok := seen[s]; ok {
			return false
		}
		seen[s] = struct{}{}
	}
	return true
}

func caWhitelistSignedLeafHandler(opts *Options) HandlerFunc {
	return func(ext pkix.Extension, chains [][]*x509.Certificate) error {
		if opts.PeerCAWhitelist == nil {
			return Error.New("no whitelist provided")
		}

		leaf := chains[0][peertls.LeafIndex]
		for _, ca := range opts.PeerCAWhitelist {
			err := pkcrypto.HashAndVerifySignature(ca.PublicKey, leaf.RawTBSCertificate, ext.Value)
			if err == nil {
				return nil
			}
		}
		return ErrVerifyCASignedLeaf
	}
}
