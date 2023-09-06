// Copyright (c) 2016, 2018, 2020, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.

package transfer

import (
	"errors"

	"github.com/oracle/oci-go-sdk/v65/common"
)

// UploadFileRequest defines the input parameters for UploadFile method
type UploadFileRequest struct {
	UploadRequest

	// The path of the file to be uploaded (includs file name)
	FilePath string
}

var errorInvalidFilePath = errors.New("filePath is required")

const defaultFilePartSize = 128 * 1024 * 1024 // 128MB

func (request UploadFileRequest) validate() error {
	err := request.UploadRequest.validate()

	if err != nil {
		return err
	}

	if len(request.FilePath) == 0 {
		return errorInvalidFilePath
	}

	return nil
}

func (request *UploadFileRequest) initDefaultValues() error {
	if request.PartSize == nil {
		request.PartSize = common.Int64(defaultFilePartSize)
	}

	return request.UploadRequest.initDefaultValues()
}
