// Copyright (c) 2016, 2018, 2020, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.

package transfer

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"io"
	"os"
	"strconv"

	"github.com/oracle/oci-go-sdk/v65/common"
)

// multipartManifest provides thread-safe access to an ongoing manifest upload.
type multipartManifest struct {
	// key is UploadID, define it as map since user can upload multiple times
	// second key is part number
	parts map[string]map[int]uploadPart
}

type uploadPart struct {
	size       int64
	offset     int64
	partBody   []byte
	partNum    int
	hash       *string
	opcMD5     *string
	etag       *string
	err        error
	totalParts int
}

// splitFileToParts starts a goroutine to read a file and break down to parts and send the parts to
// uploadPart channel. It sends the error to error chanel. If done is closed, splitFileToParts
// abandones its works.
func (manifest *multipartManifest) splitFileToParts(done <-chan struct{}, partSize int64, isChecksumEnabled *bool, file *os.File, fileSize int64) <-chan uploadPart {

	parts := make(chan uploadPart)

	// Number of parts of the file
	numberOfParts := int(fileSize / partSize)

	// check for any left over bytes
	remainder := fileSize % partSize

	totalParts := numberOfParts
	if remainder != 0 {
		totalParts = numberOfParts + 1
	}
	go func() {
		// close the channel after splitFile returns
		defer func() {
			common.Debugln("closing parts channel from splitFileParts")
			close(parts)
		}()

		// All buffer sizes are the same in the normal case. Offsets depend on the index.
		// Second go routine should start at 100, for example, given our
		// buffer size of 100.
		for i := 0; i < numberOfParts; i++ {
			offset := partSize * int64(i) // offset of the file, start with 0

			buffer := make([]byte, partSize)
			_, err := file.ReadAt(buffer, offset)

			part := uploadPart{
				partNum:    i + 1,
				size:       partSize,
				offset:     offset,
				err:        err,
				partBody:   buffer,
				totalParts: totalParts,
			}
			// Once enabled multipartMD5 verification, add opcMD5 for part
			part.opcMD5 = getPartMD5Checksum(isChecksumEnabled, part)

			select {
			case parts <- part:
			case <-done:
				return
			}
		}

		// check for any left over bytes. Add the residual number of bytes as the
		// the last chunk size.
		if remainder != 0 {
			part := uploadPart{
				offset:     int64(numberOfParts) * partSize,
				partNum:    numberOfParts + 1,
				totalParts: totalParts,
			}

			part.partBody = make([]byte, remainder)
			_, err := file.ReadAt(part.partBody, part.offset)

			part.size = remainder
			part.err = err
			// Once enabled multipartMD5 verification, add opcMD5 for part
			part.opcMD5 = getPartMD5Checksum(isChecksumEnabled, part)

			select {
			case parts <- part:
			case <-done:
				return
			}
		}
	}()

	return parts
}

func (manifest multipartManifest) getMultipartMD5Checksum(isChecksumEnabled *bool, uploadID string) *string {
	if isChecksumEnabled == nil || !*isChecksumEnabled {
		return nil
	}

	parts := manifest.parts[uploadID]
	totalParts := len(parts)
	var bytesBuf bytes.Buffer
	for i := 1; i <= totalParts; i++ {
		part := parts[i]
		cipherStr, _ := base64.StdEncoding.DecodeString(*part.opcMD5)
		bytesBuf.Write(cipherStr)
	}
	multipartMD5 := base64.StdEncoding.EncodeToString(md5Encode(bytesBuf.Bytes())) + "-" + strconv.Itoa(totalParts)
	return &multipartMD5
}

func getPartMD5Checksum(isChecksumEnabled *bool, part uploadPart) *string {
	if isChecksumEnabled == nil || !*isChecksumEnabled {
		return nil
	}

	var buffer bytes.Buffer
	cipherStr := md5Encode(part.partBody)
	opcMD5 := base64.StdEncoding.EncodeToString(cipherStr)
	buffer.Write(cipherStr)
	return &opcMD5
}

func md5Encode(data []byte) []byte {
	// Each time handle 1 MiB bytes data
	chunkSize := 1024 * 1024
	dataLength := len(data)
	chunkNum := dataLength / chunkSize
	md5Ctx := md5.New()
	for i := 0; i < chunkNum; i++ {
		md5Ctx.Write(data[chunkSize*i : chunkSize*(i+1)])
	}
	if chunkSize*chunkNum < dataLength {
		md5Ctx.Write(data[chunkSize*chunkNum : dataLength])
	}
	return md5Ctx.Sum(nil)
}

// splitStreamToParts starts a goroutine to read a stream and break down to parts and send the parts to
// uploadPart channel. It sends the error to error channel. If done is closed, splitStreamToParts
// abandons its works.
func (manifest *multipartManifest) splitStreamToParts(done <-chan struct{}, partSize int64, isChecksumEnabled *bool, reader io.Reader) <-chan uploadPart {
	parts := make(chan uploadPart)

	go func() {
		defer close(parts)
		partNum := 1
		for {
			buffer := make([]byte, partSize)
			numberOfBytesRead, err := io.ReadFull(reader, buffer)

			// ignore io.ErrUnexpectedEOF here
			if err == io.EOF {
				break
			}

			// If the number of bytes read is less than the initial buffer size, reduce the buffer size to match the actual content size.
			// it's actually the handling of io.ErrUnexpectedEOF
			if int64(numberOfBytesRead) < partSize {
				buffer = buffer[:numberOfBytesRead]
			}

			part := uploadPart{
				partNum:  partNum,
				size:     int64(numberOfBytesRead),
				err:      nil,
				partBody: buffer,
			}
			// Once enabled multipartMD5 verification, add opcMD5 for part
			part.opcMD5 = getPartMD5Checksum(isChecksumEnabled, part)
			partNum++
			select {
			case parts <- part:
			case <-done:
				return
			}
		}
	}()

	return parts
}

// update the result in manifest
func (manifest *multipartManifest) updateManifest(result <-chan uploadPart, uploadID string) {
	if manifest.parts[uploadID] == nil {
		manifest.parts[uploadID] = make(map[int]uploadPart)
	}
	for r := range result {
		manifest.parts[uploadID][r.partNum] = r
	}
}
