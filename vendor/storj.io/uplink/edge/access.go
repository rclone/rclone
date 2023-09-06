// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package edge

import (
	"context"
	"errors"

	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/uplink"
)

// We use uplinkError.* instead of errs.* to add a prefix "uplink" to every error.
// It is not called "edge" on purpose so that the entire library emits the same error prefix.
var uplinkError = errs.Class("uplink")

// ErrAuthDialFailed is a network or protocol error.
var ErrAuthDialFailed = errors.New("dial to auth service failed")

// ErrRegisterAccessFailed is an internal error in the auth service.
var ErrRegisterAccessFailed = errors.New("register access for edge services failed")

// Credentials give access to the multi-tenant gateway.
// These work in S3 clients.
type Credentials struct {
	// Base32
	// This is also used in the linkshare url path.
	AccessKeyID string
	// Base32
	SecretKey string
	// HTTP(S) URL to the gateway.
	Endpoint string
}

// RegisterAccessOptions contains optional parameters for RegisterAccess.
type RegisterAccessOptions struct {
	// Whether objects can be read without authentication.
	Public bool
}

// RegisterAccess gets credentials for the Storj-hosted Gateway and linkshare service.
// All files accessible under the Access are then also accessible via those services.
// If you call this function a lot, and the use case allows it,
// please limit the lifetime of the credentials
// by setting Permission.NotAfter when creating the Access.
func (config *Config) RegisterAccess(
	ctx context.Context,
	access *uplink.Access,
	options *RegisterAccessOptions,
) (*Credentials, error) {
	if config.AuthServiceAddress == "" {
		return nil, uplinkError.New("AuthServiceAddress is missing")
	}

	if options == nil {
		options = &RegisterAccessOptions{}
	}

	var conn *rpc.Conn
	var err error
	if config.InsecureUnencryptedConnection || config.InsecureSkipVerify {
		conn, err = config.createDialer().DialAddressUnencrypted(ctx, config.AuthServiceAddress)
	} else {
		conn, err = config.createDialer().DialAddressHostnameVerification(ctx, config.AuthServiceAddress)
	}

	if err != nil {
		return nil, uplinkError.New("%w: %v", ErrAuthDialFailed, err)
	}
	defer func() {
		_ = conn.Close()
	}()

	client := pb.NewDRPCEdgeAuthClient(conn)

	serializedAccess, err := access.Serialize()
	if err != nil {
		return nil, uplinkError.Wrap(err)
	}

	registerGatewayResponse, err := client.RegisterAccess(ctx, &pb.EdgeRegisterAccessRequest{
		AccessGrant: serializedAccess,
		Public:      options.Public,
	})

	if err != nil {
		return nil, uplinkError.New("%w: %v", ErrRegisterAccessFailed, err)
	}

	credentials := Credentials{
		AccessKeyID: registerGatewayResponse.AccessKeyId,
		SecretKey:   registerGatewayResponse.SecretKey,
		Endpoint:    registerGatewayResponse.Endpoint,
	}

	return &credentials, nil
}
