// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package expose

// RequestAccessWithPassphraseAndConcurrency exposes uplink.requestAccessWithPassphraseAndConcurrency.
//
// func RequestAccessWithPassphraseAndConcurrency(ctx context.Context, config uplink.Config, satelliteNodeURL, apiKey, passphrase string, concurrency uint8) (_ *uplink.Access, err error).
var RequestAccessWithPassphraseAndConcurrency interface{}

// EnablePathEncryptionBypass exposes uplink.enablePathEncryptionBypass.
//
// func EnablePathEncryptionBypass(access *Access) error.
var EnablePathEncryptionBypass interface{}
