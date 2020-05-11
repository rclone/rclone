// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package tlsopts

import (
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io/ioutil"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/identity"
	"storj.io/common/peertls"
	"storj.io/common/peertls/extensions"
	"storj.io/common/pkcrypto"
)

var (
	mon = monkit.Package()
	// Error is error for tlsopts
	Error = errs.Class("tlsopts error")
)

// Options holds config, identity, and peer verification function data for use with tls.
type Options struct {
	Config            Config
	Ident             *identity.FullIdentity
	RevDB             extensions.RevocationDB
	PeerCAWhitelist   []*x509.Certificate
	VerificationFuncs *VerificationFuncs
	Cert              *tls.Certificate
}

// VerificationFuncs keeps track of of client and server peer certificate verification
// functions for use in tls handshakes.
type VerificationFuncs struct {
	client []peertls.PeerCertVerificationFunc
	server []peertls.PeerCertVerificationFunc
}

// ExtensionMap maps `pkix.Extension`s to their respective asn1 object ID string.
type ExtensionMap map[string]pkix.Extension

// NewOptions is a constructor for `tls options` given an identity, config, and
// revocation DB. A caller may pass a nil revocation DB if the revocation
// extension is disabled.
func NewOptions(i *identity.FullIdentity, c Config, revocationDB extensions.RevocationDB) (*Options, error) {
	opts := &Options{
		Config:            c,
		RevDB:             revocationDB,
		Ident:             i,
		VerificationFuncs: new(VerificationFuncs),
	}

	err := opts.configure()
	if err != nil {
		return nil, err
	}

	return opts, nil
}

// NewExtensionsMap builds an `ExtensionsMap` from the extensions in the passed certificate(s).
func NewExtensionsMap(chain ...*x509.Certificate) ExtensionMap {
	extensionMap := make(ExtensionMap)
	for _, cert := range chain {
		for _, ext := range cert.Extensions {
			extensionMap[ext.Id.String()] = ext
		}
	}
	return extensionMap
}

// ExtensionOptions converts options for use in extension handling.
func (opts *Options) ExtensionOptions() *extensions.Options {
	return &extensions.Options{
		PeerCAWhitelist: opts.PeerCAWhitelist,
		RevocationDB:    opts.RevDB,
		PeerIDVersions:  opts.Config.PeerIDVersions,
	}
}

// configure adds peer certificate verification functions and data structures
// required for completing TLS handshakes to the options.
func (opts *Options) configure() (err error) {
	if opts.Config.UsePeerCAWhitelist {
		whitelist := []byte(DefaultPeerCAWhitelist)
		if opts.Config.PeerCAWhitelistPath != "" {
			whitelist, err = ioutil.ReadFile(opts.Config.PeerCAWhitelistPath)
			if err != nil {
				return Error.New("unable to find whitelist file %v: %v", opts.Config.PeerCAWhitelistPath, err)
			}
		}
		opts.PeerCAWhitelist, err = pkcrypto.CertsFromPEM(whitelist)
		if err != nil {
			return Error.Wrap(err)
		}
		opts.VerificationFuncs.ClientAdd(peertls.VerifyCAWhitelist(opts.PeerCAWhitelist))
	}

	handlers := make(extensions.HandlerFactories, len(extensions.DefaultHandlers))
	copy(handlers, extensions.DefaultHandlers)

	if opts.Config.Extensions.Revocation {
		handlers.Register(
			extensions.RevocationCheckHandler,
			extensions.RevocationUpdateHandler,
		)
	}

	if opts.Config.Extensions.WhitelistSignedLeaf {
		handlers.Register(extensions.CAWhitelistSignedLeafHandler)
	}

	opts.handleExtensions(handlers)

	opts.Cert, err = peertls.TLSCert(opts.Ident.RawChain(), opts.Ident.Leaf, opts.Ident.Key)
	return err
}

// handleExtensions combines and wraps all extension handler functions into a peer
// certificate verification function. This allows extension handling via the
// `VerifyPeerCertificate` field in a `tls.Config` during a TLS handshake.
func (opts *Options) handleExtensions(handlers extensions.HandlerFactories) {
	if len(handlers) == 0 {
		return
	}

	handlerFuncMap := handlers.WithOptions(opts.ExtensionOptions())

	combinedHandlerFunc := func(_ [][]byte, parsedChains [][]*x509.Certificate) error {
		extensionMap := NewExtensionsMap(parsedChains[0]...)
		return extensionMap.HandleExtensions(handlerFuncMap, parsedChains)
	}

	opts.VerificationFuncs.Add(combinedHandlerFunc)
}

// HandleExtensions calls each `extensions.HandlerFunc` with its respective extension
// and the certificate chain where its object ID string matches the extension's.
func (extensionMap ExtensionMap) HandleExtensions(handlerFuncMap extensions.HandlerFuncMap, chain [][]*x509.Certificate) error {
	for idStr, extension := range extensionMap {
		for id, handlerFunc := range handlerFuncMap {
			if idStr == id.String() {
				err := handlerFunc(extension, chain)
				if err != nil {
					return Error.Wrap(err)
				}
			}
		}
	}
	return nil
}

// Client returns the client verification functions.
func (vf *VerificationFuncs) Client() []peertls.PeerCertVerificationFunc {
	return vf.client
}

// Server returns the server verification functions.
func (vf *VerificationFuncs) Server() []peertls.PeerCertVerificationFunc {
	return vf.server
}

// Add adds verification functions so the client and server lists.
func (vf *VerificationFuncs) Add(verificationFuncs ...peertls.PeerCertVerificationFunc) {
	vf.ClientAdd(verificationFuncs...)
	vf.ServerAdd(verificationFuncs...)
}

// ClientAdd adds verification functions so the client list.
func (vf *VerificationFuncs) ClientAdd(verificationFuncs ...peertls.PeerCertVerificationFunc) {
	verificationFuncs = removeNils(verificationFuncs)
	vf.client = append(vf.client, verificationFuncs...)
}

// ServerAdd adds verification functions so the server list.
func (vf *VerificationFuncs) ServerAdd(verificationFuncs ...peertls.PeerCertVerificationFunc) {
	verificationFuncs = removeNils(verificationFuncs)
	vf.server = append(vf.server, verificationFuncs...)
}

func removeNils(verificationFuncs []peertls.PeerCertVerificationFunc) []peertls.PeerCertVerificationFunc {
	result := verificationFuncs[:0]
	for _, f := range verificationFuncs {
		if f != nil {
			result = append(result, f)
		}
	}
	return result
}
