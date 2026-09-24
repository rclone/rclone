//go:build !plan9 && !solaris && !js

package oracleobjectstorage

import (
	"testing"

	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/stretchr/testify/assert"
)

func TestUseBYOKCopyObject(t *testing.T) {
	f := &Fs{opt: Options{
		SSECustomerAlgorithm: "AES256",
		SSECustomerKey:       "customer-key",
		SSECustomerKeySha256: "customer-key-sha256",
	}}
	req := &objectstorage.CopyObjectRequest{}

	useBYOKCopyObject(f, req)

	assert.Equal(t, &f.opt.SSECustomerAlgorithm, req.OpcSourceSseCustomerAlgorithm)
	assert.Equal(t, &f.opt.SSECustomerKey, req.OpcSourceSseCustomerKey)
	assert.Equal(t, &f.opt.SSECustomerKeySha256, req.OpcSourceSseCustomerKeySha256)
}
