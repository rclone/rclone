// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"
	"errors"
	"strings"
	"time"
	_ "unsafe" // for go:linkname

	"github.com/zeebo/errs"

	"storj.io/common/encryption"
	"storj.io/common/grant"
	"storj.io/common/macaroon"
	"storj.io/common/paths"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/uplink/private/metaclient"
)

// An Access Grant contains everything to access a project and specific buckets.
// It includes a potentially-restricted API Key, a potentially-restricted set
// of encryption information, and information about the Satellite responsible
// for the project's metadata.
type Access struct {
	satelliteURL storj.NodeURL
	apiKey       *macaroon.APIKey
	encAccess    *grant.EncryptionAccess
}

// getAPIKey are exposing the state do private methods.
//
// NB: this is used with linkname in internal/expose.
// It needs to be updated when this is updated.
//
//lint:ignore U1000, used with linkname
//nolint:unused
//go:linkname access_getAPIKey
func access_getAPIKey(access *Access) *macaroon.APIKey { return access.apiKey }

// getEncAccess are exposing the state do private methods.
//
// NB: this is used with linkname in internal/expose.
// It needs to be updated when this is updated.
//
//lint:ignore U1000, used with linkname
//nolint:unused
//go:linkname access_getEncAccess
func access_getEncAccess(access *Access) *grant.EncryptionAccess { return access.encAccess }

// SharePrefix defines a prefix that will be shared.
type SharePrefix struct {
	Bucket string
	// Prefix is the prefix of the shared object keys.
	//
	// Note: that within a bucket, the hierarchical key derivation scheme is
	// delineated by forward slashes (/), so encryption information will be
	// included in the resulting access grant to decrypt any key that shares
	// the same prefix up until the last slash.
	Prefix string
}

// Permission defines what actions can be used to share.
type Permission struct {
	// AllowDownload gives permission to download the object's content. It
	// allows getting object metadata, but it does not allow listing buckets.
	AllowDownload bool
	// AllowUpload gives permission to create buckets and upload new objects.
	// It does not allow overwriting existing objects unless AllowDelete is
	// granted too.
	AllowUpload bool
	// AllowList gives permission to list buckets. It allows getting object
	// metadata, but it does not allow downloading the object's content.
	AllowList bool
	// AllowDelete gives permission to delete buckets and objects. Unless
	// either AllowDownload or AllowList is granted too, no object metadata and
	// no error info will be returned for deleted objects.
	AllowDelete bool
	// NotBefore restricts when the resulting access grant is valid for.
	// If set, the resulting access grant will not work if the Satellite
	// believes the time is before NotBefore.
	// If set, this value should always be before NotAfter.
	NotBefore time.Time
	// NotAfter restricts when the resulting access grant is valid for.
	// If set, the resulting access grant will not work if the Satellite
	// believes the time is after NotAfter.
	// If set, this value should always be after NotBefore.
	NotAfter time.Time
}

// ParseAccess parses a serialized access grant string.
//
// This should be the main way to instantiate an access grant for opening a project.
// See the note on RequestAccessWithPassphrase.
func ParseAccess(access string) (*Access, error) {
	inner, err := grant.ParseAccess(access)
	if err != nil {
		return nil, packageError.Wrap(err)
	}

	satelliteURL, err := parseNodeURL(inner.SatelliteAddress)
	if err != nil {
		return nil, packageError.Wrap(err)
	}

	return &Access{
		satelliteURL: satelliteURL,
		apiKey:       inner.APIKey,
		encAccess:    inner.EncAccess,
	}, nil
}

// SatelliteAddress returns the satellite node URL for this access grant.
func (access *Access) SatelliteAddress() string {
	return access.satelliteURL.String()
}

// Serialize serializes an access grant such that it can be used later with
// ParseAccess or other tools.
func (access *Access) Serialize() (string, error) {
	inner := grant.Access{
		SatelliteAddress: access.satelliteURL.String(),
		APIKey:           access.apiKey,
		EncAccess:        access.encAccess,
	}
	return inner.Serialize()
}

// RequestAccessWithPassphrase generates a new access grant using a passhprase.
// It must talk to the Satellite provided to get a project-based salt for
// deterministic key derivation.
//
// Note: this is a CPU-heavy function that uses a password-based key derivation function
// (Argon2). This should be a setup-only step. Most common interactions with the library
// should be using a serialized access grant through ParseAccess directly.
func RequestAccessWithPassphrase(ctx context.Context, satelliteAddress, apiKey, passphrase string) (*Access, error) {
	return (Config{}).RequestAccessWithPassphrase(ctx, satelliteAddress, apiKey, passphrase)
}

// RequestAccessWithPassphrase generates a new access grant using a passhprase.
// It must talk to the Satellite provided to get a project-based salt for
// deterministic key derivation.
//
// Note: this is a CPU-heavy function that uses a password-based key derivation function
// (Argon2). This should be a setup-only step. Most common interactions with the library
// should be using a serialized access grant through ParseAccess directly.
func (config Config) RequestAccessWithPassphrase(ctx context.Context, satelliteAddress, apiKey, passphrase string) (*Access, error) {
	return config_requestAccessWithPassphraseAndConcurrency(config, ctx, satelliteAddress, apiKey, passphrase, 8)
}

// requestAccessWithPassphraseAndConcurrency requests satellite for a new access grant using a passhprase and specific concurrency for the Argon2 key derivation.
//
// NB: this is used with linkname in internal/expose.
// It needs to be updated when this is updated.
//
//nolint:revive
//go:linkname config_requestAccessWithPassphraseAndConcurrency
func config_requestAccessWithPassphraseAndConcurrency(config Config, ctx context.Context, satelliteAddress, apiKey, passphrase string, concurrency uint8) (_ *Access, err error) {
	parsedAPIKey, err := macaroon.ParseAPIKey(apiKey)
	if err != nil {
		return nil, packageError.Wrap(err)
	}

	satelliteURL, err := parseNodeURL(satelliteAddress)
	if err != nil {
		return nil, packageError.Wrap(err)
	}

	dialer, err := config.getDialer(ctx)
	if err != nil {
		return nil, packageError.Wrap(err)
	}
	defer func() { err = errs.Combine(err, dialer.Pool.Close()) }()

	metainfo, err := metaclient.DialNodeURL(ctx, dialer, satelliteURL.String(), parsedAPIKey, config.UserAgent)
	if err != nil {
		return nil, packageError.Wrap(err)
	}
	defer func() { err = errs.Combine(err, metainfo.Close()) }()

	info, err := metainfo.GetProjectInfo(ctx)
	if err != nil {
		return nil, convertKnownErrors(err, "", "")
	}

	key, err := encryption.DeriveRootKey([]byte(passphrase), info.ProjectSalt, "", concurrency)
	if err != nil {
		return nil, packageError.Wrap(err)
	}

	encAccess := grant.NewEncryptionAccessWithDefaultKey(key)
	encAccess.SetDefaultPathCipher(storj.EncAESGCM)
	if config.disableObjectKeyEncryption {
		encAccess.SetDefaultPathCipher(storj.EncNull)
	}
	encAccess.LimitTo(parsedAPIKey)

	return &Access{
		satelliteURL: satelliteURL,
		apiKey:       parsedAPIKey,
		encAccess:    encAccess,
	}, nil
}

// parseNodeURL parses the address into a storj.NodeURL adding the node id if necessary
// for known addresses.
func parseNodeURL(address string) (storj.NodeURL, error) {
	nodeURL, err := storj.ParseNodeURL(address)
	if err != nil {
		return nodeURL, packageError.Wrap(err)
	}

	// Node id is required in satelliteNodeID for all unknown (non-storj) satellites.
	// For known satellite it will be automatically prepended.
	if nodeURL.ID.IsZero() {
		nodeID, found := rpc.KnownNodeID(nodeURL.Address)
		if !found {
			return nodeURL, packageError.New("node id is required in satelliteNodeURL")
		}
		nodeURL.ID = nodeID
	}

	return nodeURL, nil
}

// Share creates a new access grant with specific permissions.
//
// Access grants can only have their existing permissions restricted,
// and the resulting access grant will only allow for the intersection of all previous
// Share calls in the access grant construction chain.
//
// Prefixes, if provided, restrict the access grant (and internal encryption information)
// to only contain enough information to allow access to just those prefixes.
//
// To revoke an access grant see the Project.RevokeAccess method.
func (access *Access) Share(permission Permission, prefixes ...SharePrefix) (*Access, error) {
	internalPrefixes := make([]grant.SharePrefix, 0, len(prefixes))
	for _, prefix := range prefixes {
		internalPrefixes = append(internalPrefixes, grant.SharePrefix(prefix))
	}
	rv, err := access.toInternal().Restrict(grant.Permission(permission), internalPrefixes...)
	if err != nil {
		return nil, err
	}
	return accessFromInternal(rv)
}

func (access *Access) toInternal() *grant.Access {
	return &grant.Access{
		SatelliteAddress: access.satelliteURL.String(),
		APIKey:           access.apiKey,
		EncAccess:        access.encAccess,
	}
}

func accessFromInternal(access *grant.Access) (*Access, error) {
	satelliteURL, err := parseNodeURL(access.SatelliteAddress)
	if err != nil {
		return nil, packageError.Wrap(err)
	}

	return &Access{
		satelliteURL: satelliteURL,
		apiKey:       access.APIKey,
		encAccess:    access.EncAccess,
	}, nil
}

// RevokeAccess revokes the API key embedded in the provided access grant.
//
// When an access grant is revoked, it will also revoke any further-restricted
// access grants created (via the Access.Share method) from the revoked access
// grant.
//
// An access grant is authorized to revoke any further-restricted access grant
// created from it. An access grant cannot revoke itself. An unauthorized
// request will return an error.
//
// There may be a delay between a successful revocation request and actual
// revocation, depending on the satellite's access caching policies.
func (project *Project) RevokeAccess(ctx context.Context, access *Access) (err error) {
	defer mon.Task()(&ctx)(&err)

	metainfoClient, err := project.dialMetainfoClient(ctx)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, metainfoClient.Close()) }()

	err = metainfoClient.RevokeAPIKey(ctx, metaclient.RevokeAPIKeyParams{
		APIKey: access.apiKey.SerializeRaw(),
	})
	return convertKnownErrors(err, "", "")
}

// ReadOnlyPermission returns a Permission that allows reading and listing
// (if the parent access grant already allows those things).
func ReadOnlyPermission() Permission {
	return Permission{
		AllowDownload: true,
		AllowList:     true,
	}
}

// WriteOnlyPermission returns a Permission that allows writing and deleting
// (if the parent access grant already allows those things).
func WriteOnlyPermission() Permission {
	return Permission{
		AllowUpload: true,
		AllowDelete: true,
	}
}

// FullPermission returns a Permission that allows all actions that the
// parent access grant already allows.
func FullPermission() Permission {
	return Permission{
		AllowDownload: true,
		AllowUpload:   true,
		AllowList:     true,
		AllowDelete:   true,
	}
}

// OverrideEncryptionKey overrides the root encryption key for the prefix in
// bucket with encryptionKey.
// The prefix argument must end with slash, otherwise the method returns an
// error.
//
// This function is useful for overriding the encryption key in user-specific
// access grants when implementing multitenancy in a single app bucket.
// See the relevant section in the package documentation.
func (access *Access) OverrideEncryptionKey(bucket, prefix string, encryptionKey *EncryptionKey) error {
	if !strings.HasSuffix(prefix, "/") {
		return errors.New("prefix must end with slash")
	}

	// We need to remove the trailing slash. Otherwise, if we the shared
	// prefix is `/bob/`, the encrypted shared prefix results in
	// `enc("")/enc("bob")/enc("")`. This is an incorrect encrypted prefix,
	// what we really want is `enc("")/enc("bob")`.
	prefix = strings.TrimSuffix(prefix, "/")

	store := access.encAccess.Store

	unencPath := paths.NewUnencrypted(prefix)
	encPath, err := encryption.EncryptPathWithStoreCipher(bucket, unencPath, store)
	if err != nil {
		return convertKnownErrors(err, bucket, prefix)
	}

	err = store.Add(bucket, unencPath, encPath, *encryptionKey.key)
	return convertKnownErrors(err, bucket, prefix)
}
