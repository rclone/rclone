//go:build !plan9 && !solaris && !js

package oracleobjectstorage

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
)

const (
	sseDefaultAlgorithm = "AES256"
)

func getSha256(p []byte) []byte {
	h := sha256.New()
	h.Write(p)
	return h.Sum(nil)
}

func validateSSECustomerKeyOptions(opt *Options) error {
	if opt.SSEKMSKeyID != "" && (opt.SSECustomerKeyFile != "" || opt.SSECustomerKey != "") {
		return errors.New("oos: can't use vault sse_kms_key_id and local sse_customer_key at the same time")
	}
	if opt.SSECustomerKey != "" && opt.SSECustomerKeyFile != "" {
		return errors.New("oos: can't use sse_customer_key and sse_customer_key_file at the same time")
	}
	if opt.SSEKMSKeyID != "" {
		return nil
	}
	err := populateSSECustomerKeys(opt)
	if err != nil {
		return err
	}
	return nil
}

func populateSSECustomerKeys(opt *Options) error {
	if opt.SSECustomerKeyFile != "" {
		// Reads the base64-encoded AES key data from the specified file and computes its SHA256 checksum
		data, err := os.ReadFile(expandPath(opt.SSECustomerKeyFile))
		if err != nil {
			return fmt.Errorf("oos: error reading sse_customer_key_file: %v", err)
		}
		opt.SSECustomerKey = strings.TrimSpace(string(data))
	}
	if opt.SSECustomerKey != "" {
		decoded, err := base64.StdEncoding.DecodeString(opt.SSECustomerKey)
		if err != nil {
			return fmt.Errorf("oos: Could not decode sse_customer_key_file: %w", err)
		}
		sha256Checksum := base64.StdEncoding.EncodeToString(getSha256(decoded))
		if opt.SSECustomerKeySha256 == "" {
			opt.SSECustomerKeySha256 = sha256Checksum
		} else if opt.SSECustomerKeySha256 != sha256Checksum {
			return fmt.Errorf("the computed SHA256 checksum "+
				"(%v) of the key doesn't match the config entry sse_customer_key_sha256=(%v)",
				sha256Checksum, opt.SSECustomerKeySha256)
		}
		if opt.SSECustomerAlgorithm == "" {
			opt.SSECustomerAlgorithm = sseDefaultAlgorithm
		}
	}
	return nil
}

// https://docs.oracle.com/en-us/iaas/Content/Object/Tasks/usingyourencryptionkeys.htm
func useBYOKPutObject(fs *Fs, request *objectstorage.PutObjectRequest) {
	if fs.opt.SSEKMSKeyID != "" {
		request.OpcSseKmsKeyId = common.String(fs.opt.SSEKMSKeyID)
	}
	if fs.opt.SSECustomerAlgorithm != "" {
		request.OpcSseCustomerAlgorithm = common.String(fs.opt.SSECustomerAlgorithm)
	}
	if fs.opt.SSECustomerKey != "" {
		request.OpcSseCustomerKey = common.String(fs.opt.SSECustomerKey)
	}
	if fs.opt.SSECustomerKeySha256 != "" {
		request.OpcSseCustomerKeySha256 = common.String(fs.opt.SSECustomerKeySha256)
	}
}

func useBYOKHeadObject(fs *Fs, request *objectstorage.HeadObjectRequest) {
	if fs.opt.SSECustomerAlgorithm != "" {
		request.OpcSseCustomerAlgorithm = common.String(fs.opt.SSECustomerAlgorithm)
	}
	if fs.opt.SSECustomerKey != "" {
		request.OpcSseCustomerKey = common.String(fs.opt.SSECustomerKey)
	}
	if fs.opt.SSECustomerKeySha256 != "" {
		request.OpcSseCustomerKeySha256 = common.String(fs.opt.SSECustomerKeySha256)
	}
}

func useBYOKGetObject(fs *Fs, request *objectstorage.GetObjectRequest) {
	if fs.opt.SSECustomerAlgorithm != "" {
		request.OpcSseCustomerAlgorithm = common.String(fs.opt.SSECustomerAlgorithm)
	}
	if fs.opt.SSECustomerKey != "" {
		request.OpcSseCustomerKey = common.String(fs.opt.SSECustomerKey)
	}
	if fs.opt.SSECustomerKeySha256 != "" {
		request.OpcSseCustomerKeySha256 = common.String(fs.opt.SSECustomerKeySha256)
	}
}

func useBYOKCopyObject(fs *Fs, request *objectstorage.CopyObjectRequest) {
	if fs.opt.SSEKMSKeyID != "" {
		request.OpcSseKmsKeyId = common.String(fs.opt.SSEKMSKeyID)
	}
	if fs.opt.SSECustomerAlgorithm != "" {
		request.OpcSseCustomerAlgorithm = common.String(fs.opt.SSECustomerAlgorithm)
	}
	if fs.opt.SSECustomerKey != "" {
		request.OpcSseCustomerKey = common.String(fs.opt.SSECustomerKey)
	}
	if fs.opt.SSECustomerKeySha256 != "" {
		request.OpcSseCustomerKeySha256 = common.String(fs.opt.SSECustomerKeySha256)
	}
}
