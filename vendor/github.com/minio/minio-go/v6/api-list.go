/*
 * MinIO Go Library for Amazon S3 Compatible Cloud Storage
 * Copyright 2015-2019 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package minio

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/minio/minio-go/v6/pkg/s3utils"
)

// ListBuckets list all buckets owned by this authenticated user.
//
// This call requires explicit authentication, no anonymous requests are
// allowed for listing buckets.
//
//   api := client.New(....)
//   for message := range api.ListBuckets() {
//       fmt.Println(message)
//   }
//
func (c Client) ListBuckets() ([]BucketInfo, error) {
	return c.ListBucketsWithContext(context.Background())
}

// ListBucketsWithContext list all buckets owned by this authenticated user,
// accepts a context for facilitate cancellation.
//
// This call requires explicit authentication, no anonymous requests are
// allowed for listing buckets.
//
//   api := client.New(....)
//   for message := range api.ListBucketsWithContext(context.Background()) {
//       fmt.Println(message)
//   }
//
func (c Client) ListBucketsWithContext(ctx context.Context) ([]BucketInfo, error) {
	// Execute GET on service.
	resp, err := c.executeMethod(ctx, "GET", requestMetadata{contentSHA256Hex: emptySHA256Hex})
	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}
	if resp != nil {
		if resp.StatusCode != http.StatusOK {
			return nil, httpRespToErrorResponse(resp, "", "")
		}
	}
	listAllMyBucketsResult := listAllMyBucketsResult{}
	err = xmlDecoder(resp.Body, &listAllMyBucketsResult)
	if err != nil {
		return nil, err
	}
	return listAllMyBucketsResult.Buckets.Bucket, nil
}

/// Bucket Read Operations.

// ListObjectsV2WithMetadata lists all objects matching the objectPrefix
// from the specified bucket. If recursion is enabled it would list
// all subdirectories and all its contents. This call adds
// UserMetadata information as well for each object.
//
// This is a MinIO extension, this will not work against other S3
// compatible object storage vendors.
//
// Your input parameters are just bucketName, objectPrefix, recursive
// and a done channel for pro-actively closing the internal go
// routine. If you enable recursive as 'true' this function will
// return back all the objects in a given bucket name and object
// prefix.
//
//   api := client.New(....)
//   // Create a done channel.
//   doneCh := make(chan struct{})
//   defer close(doneCh)
//   // Recursively list all objects in 'mytestbucket'
//   recursive := true
//   // Add metadata
//   metadata := true
//   for message := range api.ListObjectsV2WithMetadata("mytestbucket", "starthere", recursive, doneCh) {
//       fmt.Println(message)
//   }
//
func (c Client) ListObjectsV2WithMetadata(bucketName, objectPrefix string, recursive bool,
	doneCh <-chan struct{}) <-chan ObjectInfo {
	// Check whether this is snowball region, if yes ListObjectsV2 doesn't work, fallback to listObjectsV1.
	if location, ok := c.bucketLocCache.Get(bucketName); ok {
		if location == "snowball" {
			return c.ListObjects(bucketName, objectPrefix, recursive, doneCh)
		}
	}
	return c.listObjectsV2(bucketName, objectPrefix, recursive, true, doneCh)
}

func (c Client) listObjectsV2(bucketName, objectPrefix string, recursive, metadata bool, doneCh <-chan struct{}) <-chan ObjectInfo {
	// Allocate new list objects channel.
	objectStatCh := make(chan ObjectInfo, 1)
	// Default listing is delimited at "/"
	delimiter := "/"
	if recursive {
		// If recursive we do not delimit.
		delimiter = ""
	}

	// Return object owner information by default
	fetchOwner := true

	// Validate bucket name.
	if err := s3utils.CheckValidBucketName(bucketName); err != nil {
		defer close(objectStatCh)
		objectStatCh <- ObjectInfo{
			Err: err,
		}
		return objectStatCh
	}

	// Validate incoming object prefix.
	if err := s3utils.CheckValidObjectNamePrefix(objectPrefix); err != nil {
		defer close(objectStatCh)
		objectStatCh <- ObjectInfo{
			Err: err,
		}
		return objectStatCh
	}

	// Initiate list objects goroutine here.
	go func(objectStatCh chan<- ObjectInfo) {
		defer close(objectStatCh)
		// Save continuationToken for next request.
		var continuationToken string
		for {
			// Get list of objects a maximum of 1000 per request.
			result, err := c.listObjectsV2Query(bucketName, objectPrefix, continuationToken,
				fetchOwner, metadata, delimiter, 0, "")
			if err != nil {
				objectStatCh <- ObjectInfo{
					Err: err,
				}
				return
			}

			// If contents are available loop through and send over channel.
			for _, object := range result.Contents {
				object.ETag = trimEtag(object.ETag)
				select {
				// Send object content.
				case objectStatCh <- object:
				// If receives done from the caller, return here.
				case <-doneCh:
					return
				}
			}

			// Send all common prefixes if any.
			// NOTE: prefixes are only present if the request is delimited.
			for _, obj := range result.CommonPrefixes {
				select {
				// Send object prefixes.
				case objectStatCh <- ObjectInfo{Key: obj.Prefix}:
				// If receives done from the caller, return here.
				case <-doneCh:
					return
				}
			}

			// If continuation token present, save it for next request.
			if result.NextContinuationToken != "" {
				continuationToken = result.NextContinuationToken
			}

			// Listing ends result is not truncated, return right here.
			if !result.IsTruncated {
				return
			}
		}
	}(objectStatCh)
	return objectStatCh
}

// ListObjectsV2 lists all objects matching the objectPrefix from
// the specified bucket. If recursion is enabled it would list
// all subdirectories and all its contents.
//
// Your input parameters are just bucketName, objectPrefix, recursive
// and a done channel for pro-actively closing the internal go
// routine. If you enable recursive as 'true' this function will
// return back all the objects in a given bucket name and object
// prefix.
//
//   api := client.New(....)
//   // Create a done channel.
//   doneCh := make(chan struct{})
//   defer close(doneCh)
//   // Recursively list all objects in 'mytestbucket'
//   recursive := true
//   for message := range api.ListObjectsV2("mytestbucket", "starthere", recursive, doneCh) {
//       fmt.Println(message)
//   }
//
func (c Client) ListObjectsV2(bucketName, objectPrefix string, recursive bool, doneCh <-chan struct{}) <-chan ObjectInfo {
	// Check whether this is snowball region, if yes ListObjectsV2 doesn't work, fallback to listObjectsV1.
	if location, ok := c.bucketLocCache.Get(bucketName); ok {
		if location == "snowball" {
			return c.ListObjects(bucketName, objectPrefix, recursive, doneCh)
		}
	}
	return c.listObjectsV2(bucketName, objectPrefix, recursive, false, doneCh)
}

// listObjectsV2Query - (List Objects V2) - List some or all (up to 1000) of the objects in a bucket.
//
// You can use the request parameters as selection criteria to return a subset of the objects in a bucket.
// request parameters :-
// ---------
// ?continuation-token - Used to continue iterating over a set of objects
// ?delimiter - A delimiter is a character you use to group keys.
// ?prefix - Limits the response to keys that begin with the specified prefix.
// ?max-keys - Sets the maximum number of keys returned in the response body.
// ?start-after - Specifies the key to start after when listing objects in a bucket.
// ?metadata - Specifies if we want metadata for the objects as part of list operation.
func (c Client) listObjectsV2Query(bucketName, objectPrefix, continuationToken string, fetchOwner, metadata bool, delimiter string, maxkeys int, startAfter string) (ListBucketV2Result, error) {
	// Validate bucket name.
	if err := s3utils.CheckValidBucketName(bucketName); err != nil {
		return ListBucketV2Result{}, err
	}
	// Validate object prefix.
	if err := s3utils.CheckValidObjectNamePrefix(objectPrefix); err != nil {
		return ListBucketV2Result{}, err
	}
	// Get resources properly escaped and lined up before
	// using them in http request.
	urlValues := make(url.Values)

	// Always set list-type in ListObjects V2
	urlValues.Set("list-type", "2")

	if metadata {
		urlValues.Set("metadata", "true")
	}

	// Always set encoding-type in ListObjects V2
	urlValues.Set("encoding-type", "url")

	// Set object prefix, prefix value to be set to empty is okay.
	urlValues.Set("prefix", objectPrefix)

	// Set delimiter, delimiter value to be set to empty is okay.
	urlValues.Set("delimiter", delimiter)

	// Set continuation token
	if continuationToken != "" {
		urlValues.Set("continuation-token", continuationToken)
	}

	// Fetch owner when listing
	if fetchOwner {
		urlValues.Set("fetch-owner", "true")
	}

	// Set max keys.
	if maxkeys > 0 {
		urlValues.Set("max-keys", fmt.Sprintf("%d", maxkeys))
	}

	// Set start-after
	if startAfter != "" {
		urlValues.Set("start-after", startAfter)
	}

	// Execute GET on bucket to list objects.
	resp, err := c.executeMethod(context.Background(), "GET", requestMetadata{
		bucketName:       bucketName,
		queryValues:      urlValues,
		contentSHA256Hex: emptySHA256Hex,
	})
	defer closeResponse(resp)
	if err != nil {
		return ListBucketV2Result{}, err
	}
	if resp != nil {
		if resp.StatusCode != http.StatusOK {
			return ListBucketV2Result{}, httpRespToErrorResponse(resp, bucketName, "")
		}
	}

	// Decode listBuckets XML.
	listBucketResult := ListBucketV2Result{}
	if err = xmlDecoder(resp.Body, &listBucketResult); err != nil {
		return listBucketResult, err
	}

	// This is an additional verification check to make
	// sure proper responses are received.
	if listBucketResult.IsTruncated && listBucketResult.NextContinuationToken == "" {
		return listBucketResult, ErrorResponse{
			Code:    "NotImplemented",
			Message: "Truncated response should have continuation token set",
		}
	}

	for i, obj := range listBucketResult.Contents {
		listBucketResult.Contents[i].Key, err = decodeS3Name(obj.Key, listBucketResult.EncodingType)
		if err != nil {
			return listBucketResult, err
		}
	}

	for i, obj := range listBucketResult.CommonPrefixes {
		listBucketResult.CommonPrefixes[i].Prefix, err = decodeS3Name(obj.Prefix, listBucketResult.EncodingType)
		if err != nil {
			return listBucketResult, err
		}
	}

	// Success.
	return listBucketResult, nil
}

// ListObjects - (List Objects) - List some objects or all recursively.
//
// ListObjects lists all objects matching the objectPrefix from
// the specified bucket. If recursion is enabled it would list
// all subdirectories and all its contents.
//
// Your input parameters are just bucketName, objectPrefix, recursive
// and a done channel for pro-actively closing the internal go
// routine. If you enable recursive as 'true' this function will
// return back all the objects in a given bucket name and object
// prefix.
//
//   api := client.New(....)
//   // Create a done channel.
//   doneCh := make(chan struct{})
//   defer close(doneCh)
//   // Recurively list all objects in 'mytestbucket'
//   recursive := true
//   for message := range api.ListObjects("mytestbucket", "starthere", recursive, doneCh) {
//       fmt.Println(message)
//   }
//
func (c Client) ListObjects(bucketName, objectPrefix string, recursive bool, doneCh <-chan struct{}) <-chan ObjectInfo {
	// Allocate new list objects channel.
	objectStatCh := make(chan ObjectInfo, 1)
	// Default listing is delimited at "/"
	delimiter := "/"
	if recursive {
		// If recursive we do not delimit.
		delimiter = ""
	}
	// Validate bucket name.
	if err := s3utils.CheckValidBucketName(bucketName); err != nil {
		defer close(objectStatCh)
		objectStatCh <- ObjectInfo{
			Err: err,
		}
		return objectStatCh
	}
	// Validate incoming object prefix.
	if err := s3utils.CheckValidObjectNamePrefix(objectPrefix); err != nil {
		defer close(objectStatCh)
		objectStatCh <- ObjectInfo{
			Err: err,
		}
		return objectStatCh
	}

	// Initiate list objects goroutine here.
	go func(objectStatCh chan<- ObjectInfo) {
		defer close(objectStatCh)
		// Save marker for next request.
		var marker string
		for {
			// Get list of objects a maximum of 1000 per request.
			result, err := c.listObjectsQuery(bucketName, objectPrefix, marker, delimiter, 0)
			if err != nil {
				objectStatCh <- ObjectInfo{
					Err: err,
				}
				return
			}

			// If contents are available loop through and send over channel.
			for _, object := range result.Contents {
				// Save the marker.
				marker = object.Key
				select {
				// Send object content.
				case objectStatCh <- object:
				// If receives done from the caller, return here.
				case <-doneCh:
					return
				}
			}

			// Send all common prefixes if any.
			// NOTE: prefixes are only present if the request is delimited.
			for _, obj := range result.CommonPrefixes {
				select {
				// Send object prefixes.
				case objectStatCh <- ObjectInfo{Key: obj.Prefix}:
				// If receives done from the caller, return here.
				case <-doneCh:
					return
				}
			}

			// If next marker present, save it for next request.
			if result.NextMarker != "" {
				marker = result.NextMarker
			}

			// Listing ends result is not truncated, return right here.
			if !result.IsTruncated {
				return
			}
		}
	}(objectStatCh)
	return objectStatCh
}

// listObjects - (List Objects) - List some or all (up to 1000) of the objects in a bucket.
//
// You can use the request parameters as selection criteria to return a subset of the objects in a bucket.
// request parameters :-
// ---------
// ?marker - Specifies the key to start with when listing objects in a bucket.
// ?delimiter - A delimiter is a character you use to group keys.
// ?prefix - Limits the response to keys that begin with the specified prefix.
// ?max-keys - Sets the maximum number of keys returned in the response body.
func (c Client) listObjectsQuery(bucketName, objectPrefix, objectMarker, delimiter string, maxkeys int) (ListBucketResult, error) {
	// Validate bucket name.
	if err := s3utils.CheckValidBucketName(bucketName); err != nil {
		return ListBucketResult{}, err
	}
	// Validate object prefix.
	if err := s3utils.CheckValidObjectNamePrefix(objectPrefix); err != nil {
		return ListBucketResult{}, err
	}
	// Get resources properly escaped and lined up before
	// using them in http request.
	urlValues := make(url.Values)

	// Set object prefix, prefix value to be set to empty is okay.
	urlValues.Set("prefix", objectPrefix)

	// Set delimiter, delimiter value to be set to empty is okay.
	urlValues.Set("delimiter", delimiter)

	// Set object marker.
	if objectMarker != "" {
		urlValues.Set("marker", objectMarker)
	}

	// Set max keys.
	if maxkeys > 0 {
		urlValues.Set("max-keys", fmt.Sprintf("%d", maxkeys))
	}

	// Always set encoding-type
	urlValues.Set("encoding-type", "url")

	// Execute GET on bucket to list objects.
	resp, err := c.executeMethod(context.Background(), "GET", requestMetadata{
		bucketName:       bucketName,
		queryValues:      urlValues,
		contentSHA256Hex: emptySHA256Hex,
	})
	defer closeResponse(resp)
	if err != nil {
		return ListBucketResult{}, err
	}
	if resp != nil {
		if resp.StatusCode != http.StatusOK {
			return ListBucketResult{}, httpRespToErrorResponse(resp, bucketName, "")
		}
	}
	// Decode listBuckets XML.
	listBucketResult := ListBucketResult{}
	err = xmlDecoder(resp.Body, &listBucketResult)
	if err != nil {
		return listBucketResult, err
	}

	for i, obj := range listBucketResult.Contents {
		listBucketResult.Contents[i].Key, err = decodeS3Name(obj.Key, listBucketResult.EncodingType)
		if err != nil {
			return listBucketResult, err
		}
	}

	for i, obj := range listBucketResult.CommonPrefixes {
		listBucketResult.CommonPrefixes[i].Prefix, err = decodeS3Name(obj.Prefix, listBucketResult.EncodingType)
		if err != nil {
			return listBucketResult, err
		}
	}

	if listBucketResult.NextMarker != "" {
		listBucketResult.NextMarker, err = decodeS3Name(listBucketResult.NextMarker, listBucketResult.EncodingType)
		if err != nil {
			return listBucketResult, err
		}
	}

	return listBucketResult, nil
}

// ListIncompleteUploads - List incompletely uploaded multipart objects.
//
// ListIncompleteUploads lists all incompleted objects matching the
// objectPrefix from the specified bucket. If recursion is enabled
// it would list all subdirectories and all its contents.
//
// Your input parameters are just bucketName, objectPrefix, recursive
// and a done channel to pro-actively close the internal go routine.
// If you enable recursive as 'true' this function will return back all
// the multipart objects in a given bucket name.
//
//   api := client.New(....)
//   // Create a done channel.
//   doneCh := make(chan struct{})
//   defer close(doneCh)
//   // Recurively list all objects in 'mytestbucket'
//   recursive := true
//   for message := range api.ListIncompleteUploads("mytestbucket", "starthere", recursive) {
//       fmt.Println(message)
//   }
//
func (c Client) ListIncompleteUploads(bucketName, objectPrefix string, recursive bool, doneCh <-chan struct{}) <-chan ObjectMultipartInfo {
	// Turn on size aggregation of individual parts.
	isAggregateSize := true
	return c.listIncompleteUploads(bucketName, objectPrefix, recursive, isAggregateSize, doneCh)
}

// listIncompleteUploads lists all incomplete uploads.
func (c Client) listIncompleteUploads(bucketName, objectPrefix string, recursive, aggregateSize bool, doneCh <-chan struct{}) <-chan ObjectMultipartInfo {
	// Allocate channel for multipart uploads.
	objectMultipartStatCh := make(chan ObjectMultipartInfo, 1)
	// Delimiter is set to "/" by default.
	delimiter := "/"
	if recursive {
		// If recursive do not delimit.
		delimiter = ""
	}
	// Validate bucket name.
	if err := s3utils.CheckValidBucketName(bucketName); err != nil {
		defer close(objectMultipartStatCh)
		objectMultipartStatCh <- ObjectMultipartInfo{
			Err: err,
		}
		return objectMultipartStatCh
	}
	// Validate incoming object prefix.
	if err := s3utils.CheckValidObjectNamePrefix(objectPrefix); err != nil {
		defer close(objectMultipartStatCh)
		objectMultipartStatCh <- ObjectMultipartInfo{
			Err: err,
		}
		return objectMultipartStatCh
	}
	go func(objectMultipartStatCh chan<- ObjectMultipartInfo) {
		defer close(objectMultipartStatCh)
		// object and upload ID marker for future requests.
		var objectMarker string
		var uploadIDMarker string
		for {
			// list all multipart uploads.
			result, err := c.listMultipartUploadsQuery(bucketName, objectMarker, uploadIDMarker, objectPrefix, delimiter, 0)
			if err != nil {
				objectMultipartStatCh <- ObjectMultipartInfo{
					Err: err,
				}
				return
			}
			objectMarker = result.NextKeyMarker
			uploadIDMarker = result.NextUploadIDMarker

			// Send all multipart uploads.
			for _, obj := range result.Uploads {
				// Calculate total size of the uploaded parts if 'aggregateSize' is enabled.
				if aggregateSize {
					// Get total multipart size.
					obj.Size, err = c.getTotalMultipartSize(bucketName, obj.Key, obj.UploadID)
					if err != nil {
						objectMultipartStatCh <- ObjectMultipartInfo{
							Err: err,
						}
						continue
					}
				}
				select {
				// Send individual uploads here.
				case objectMultipartStatCh <- obj:
				// If done channel return here.
				case <-doneCh:
					return
				}
			}
			// Send all common prefixes if any.
			// NOTE: prefixes are only present if the request is delimited.
			for _, obj := range result.CommonPrefixes {
				select {
				// Send delimited prefixes here.
				case objectMultipartStatCh <- ObjectMultipartInfo{Key: obj.Prefix, Size: 0}:
				// If done channel return here.
				case <-doneCh:
					return
				}
			}
			// Listing ends if result not truncated, return right here.
			if !result.IsTruncated {
				return
			}
		}
	}(objectMultipartStatCh)
	// return.
	return objectMultipartStatCh
}

// listMultipartUploads - (List Multipart Uploads).
//   - Lists some or all (up to 1000) in-progress multipart uploads in a bucket.
//
// You can use the request parameters as selection criteria to return a subset of the uploads in a bucket.
// request parameters. :-
// ---------
// ?key-marker - Specifies the multipart upload after which listing should begin.
// ?upload-id-marker - Together with key-marker specifies the multipart upload after which listing should begin.
// ?delimiter - A delimiter is a character you use to group keys.
// ?prefix - Limits the response to keys that begin with the specified prefix.
// ?max-uploads - Sets the maximum number of multipart uploads returned in the response body.
func (c Client) listMultipartUploadsQuery(bucketName, keyMarker, uploadIDMarker, prefix, delimiter string, maxUploads int) (ListMultipartUploadsResult, error) {
	// Get resources properly escaped and lined up before using them in http request.
	urlValues := make(url.Values)
	// Set uploads.
	urlValues.Set("uploads", "")
	// Set object key marker.
	if keyMarker != "" {
		urlValues.Set("key-marker", keyMarker)
	}
	// Set upload id marker.
	if uploadIDMarker != "" {
		urlValues.Set("upload-id-marker", uploadIDMarker)
	}

	// Set object prefix, prefix value to be set to empty is okay.
	urlValues.Set("prefix", prefix)

	// Set delimiter, delimiter value to be set to empty is okay.
	urlValues.Set("delimiter", delimiter)

	// Always set encoding-type
	urlValues.Set("encoding-type", "url")

	// maxUploads should be 1000 or less.
	if maxUploads > 0 {
		// Set max-uploads.
		urlValues.Set("max-uploads", fmt.Sprintf("%d", maxUploads))
	}

	// Execute GET on bucketName to list multipart uploads.
	resp, err := c.executeMethod(context.Background(), "GET", requestMetadata{
		bucketName:       bucketName,
		queryValues:      urlValues,
		contentSHA256Hex: emptySHA256Hex,
	})
	defer closeResponse(resp)
	if err != nil {
		return ListMultipartUploadsResult{}, err
	}
	if resp != nil {
		if resp.StatusCode != http.StatusOK {
			return ListMultipartUploadsResult{}, httpRespToErrorResponse(resp, bucketName, "")
		}
	}
	// Decode response body.
	listMultipartUploadsResult := ListMultipartUploadsResult{}
	err = xmlDecoder(resp.Body, &listMultipartUploadsResult)
	if err != nil {
		return listMultipartUploadsResult, err
	}

	listMultipartUploadsResult.NextKeyMarker, err = decodeS3Name(listMultipartUploadsResult.NextKeyMarker, listMultipartUploadsResult.EncodingType)
	if err != nil {
		return listMultipartUploadsResult, err
	}

	listMultipartUploadsResult.NextUploadIDMarker, err = decodeS3Name(listMultipartUploadsResult.NextUploadIDMarker, listMultipartUploadsResult.EncodingType)
	if err != nil {
		return listMultipartUploadsResult, err
	}

	for i, obj := range listMultipartUploadsResult.Uploads {
		listMultipartUploadsResult.Uploads[i].Key, err = decodeS3Name(obj.Key, listMultipartUploadsResult.EncodingType)
		if err != nil {
			return listMultipartUploadsResult, err
		}
	}

	for i, obj := range listMultipartUploadsResult.CommonPrefixes {
		listMultipartUploadsResult.CommonPrefixes[i].Prefix, err = decodeS3Name(obj.Prefix, listMultipartUploadsResult.EncodingType)
		if err != nil {
			return listMultipartUploadsResult, err
		}
	}

	return listMultipartUploadsResult, nil
}

// listObjectParts list all object parts recursively.
func (c Client) listObjectParts(bucketName, objectName, uploadID string) (partsInfo map[int]ObjectPart, err error) {
	// Part number marker for the next batch of request.
	var nextPartNumberMarker int
	partsInfo = make(map[int]ObjectPart)
	for {
		// Get list of uploaded parts a maximum of 1000 per request.
		listObjPartsResult, err := c.listObjectPartsQuery(bucketName, objectName, uploadID, nextPartNumberMarker, 1000)
		if err != nil {
			return nil, err
		}
		// Append to parts info.
		for _, part := range listObjPartsResult.ObjectParts {
			// Trim off the odd double quotes from ETag in the beginning and end.
			part.ETag = trimEtag(part.ETag)
			partsInfo[part.PartNumber] = part
		}
		// Keep part number marker, for the next iteration.
		nextPartNumberMarker = listObjPartsResult.NextPartNumberMarker
		// Listing ends result is not truncated, return right here.
		if !listObjPartsResult.IsTruncated {
			break
		}
	}

	// Return all the parts.
	return partsInfo, nil
}

// findUploadIDs lists all incomplete uploads and find the uploadIDs of the matching object name.
func (c Client) findUploadIDs(bucketName, objectName string) ([]string, error) {
	var uploadIDs []string
	// Make list incomplete uploads recursive.
	isRecursive := true
	// Turn off size aggregation of individual parts, in this request.
	isAggregateSize := false
	// Create done channel to cleanup the routine.
	doneCh := make(chan struct{})
	defer close(doneCh)
	// List all incomplete uploads.
	for mpUpload := range c.listIncompleteUploads(bucketName, objectName, isRecursive, isAggregateSize, doneCh) {
		if mpUpload.Err != nil {
			return nil, mpUpload.Err
		}
		if objectName == mpUpload.Key {
			uploadIDs = append(uploadIDs, mpUpload.UploadID)
		}
	}
	// Return the latest upload id.
	return uploadIDs, nil
}

// getTotalMultipartSize - calculate total uploaded size for the a given multipart object.
func (c Client) getTotalMultipartSize(bucketName, objectName, uploadID string) (size int64, err error) {
	// Iterate over all parts and aggregate the size.
	partsInfo, err := c.listObjectParts(bucketName, objectName, uploadID)
	if err != nil {
		return 0, err
	}
	for _, partInfo := range partsInfo {
		size += partInfo.Size
	}
	return size, nil
}

// listObjectPartsQuery (List Parts query)
//     - lists some or all (up to 1000) parts that have been uploaded
//     for a specific multipart upload
//
// You can use the request parameters as selection criteria to return
// a subset of the uploads in a bucket, request parameters :-
// ---------
// ?part-number-marker - Specifies the part after which listing should
// begin.
// ?max-parts - Maximum parts to be listed per request.
func (c Client) listObjectPartsQuery(bucketName, objectName, uploadID string, partNumberMarker, maxParts int) (ListObjectPartsResult, error) {
	// Get resources properly escaped and lined up before using them in http request.
	urlValues := make(url.Values)
	// Set part number marker.
	urlValues.Set("part-number-marker", fmt.Sprintf("%d", partNumberMarker))
	// Set upload id.
	urlValues.Set("uploadId", uploadID)

	// maxParts should be 1000 or less.
	if maxParts > 0 {
		// Set max parts.
		urlValues.Set("max-parts", fmt.Sprintf("%d", maxParts))
	}

	// Execute GET on objectName to get list of parts.
	resp, err := c.executeMethod(context.Background(), "GET", requestMetadata{
		bucketName:       bucketName,
		objectName:       objectName,
		queryValues:      urlValues,
		contentSHA256Hex: emptySHA256Hex,
	})
	defer closeResponse(resp)
	if err != nil {
		return ListObjectPartsResult{}, err
	}
	if resp != nil {
		if resp.StatusCode != http.StatusOK {
			return ListObjectPartsResult{}, httpRespToErrorResponse(resp, bucketName, objectName)
		}
	}
	// Decode list object parts XML.
	listObjectPartsResult := ListObjectPartsResult{}
	err = xmlDecoder(resp.Body, &listObjectPartsResult)
	if err != nil {
		return listObjectPartsResult, err
	}
	return listObjectPartsResult, nil
}

// Decode an S3 object name according to the encoding type
func decodeS3Name(name, encodingType string) (string, error) {
	switch encodingType {
	case "url":
		return url.QueryUnescape(name)
	default:
		return name, nil
	}
}
