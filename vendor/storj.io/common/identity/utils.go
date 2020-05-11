// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package identity

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/zeebo/errs"

	"storj.io/common/fpath"
)

// TLSFilesStatus is the status of keys
type TLSFilesStatus int

// Four possible outcomes for four files
const (
	NoCertNoKey = TLSFilesStatus(iota)
	CertNoKey
	NoCertKey
	CertKey
)

var (
	// ErrZeroBytes is returned for zero slice
	ErrZeroBytes = errs.New("byte slice was unexpectedly empty")
)

// writeChainData writes data to path ensuring permissions are appropriate for a cert
func writeChainData(path string, data []byte) error {
	err := writeFile(path, 0744, 0644, data)
	if err != nil {
		return errs.New("unable to write certificate to \"%s\": %v", path, err)
	}
	return nil
}

// writeKeyData writes data to path ensuring permissions are appropriate for a cert
func writeKeyData(path string, data []byte) error {
	err := writeFile(path, 0700, 0600, data)
	if err != nil {
		return errs.New("unable to write key to \"%s\": %v", path, err)
	}
	return nil
}

// writeFile writes to path, creating directories and files with the necessary permissions
func writeFile(path string, dirmode, filemode os.FileMode, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), dirmode); err != nil {
		return errs.Wrap(err)
	}
	if writable, err := fpath.IsWritable(filepath.Dir(path)); !writable || err != nil {
		return errs.Wrap(errs.New("%s is not a writeable directory: %s\n", path, err))
	}
	if err := ioutil.WriteFile(path, data, filemode); err != nil {
		return errs.Wrap(err)
	}

	return nil
}

func statTLSFiles(certPath, keyPath string) (status TLSFilesStatus, err error) {
	hasKey := true
	hasCert := true

	_, err = os.Stat(certPath)
	if err != nil {
		if os.IsNotExist(err) {
			hasCert = false
		} else {
			return NoCertNoKey, err
		}
	}

	_, err = os.Stat(keyPath)
	if err != nil {
		if os.IsNotExist(err) {
			hasKey = false
		} else {
			return NoCertNoKey, err
		}
	}
	switch {
	case hasCert && hasKey:
		return CertKey, nil
	case hasCert:
		return CertNoKey, nil
	case hasKey:
		return NoCertKey, nil
	}

	return NoCertNoKey, nil
}

func (t TLSFilesStatus) String() string {
	switch t {
	case CertKey:
		return "certificate and key"
	case CertNoKey:
		return "certificate"
	case NoCertKey:
		return "key"
	}
	return ""
}
