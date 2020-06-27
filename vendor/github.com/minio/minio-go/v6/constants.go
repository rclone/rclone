/*
 * MinIO Go Library for Amazon S3 Compatible Cloud Storage
 * Copyright 2015-2017 MinIO, Inc.
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

/// Multipart upload defaults.

// absMinPartSize - absolute minimum part size (5 MiB) below which
// a part in a multipart upload may not be uploaded.
const absMinPartSize = 1024 * 1024 * 5

// minPartSize - minimum part size 128MiB per object after which
// putObject behaves internally as multipart.
const minPartSize = 1024 * 1024 * 128

// maxPartsCount - maximum number of parts for a single multipart session.
const maxPartsCount = 10000

// maxPartSize - maximum part size 5GiB for a single multipart upload
// operation.
const maxPartSize = 1024 * 1024 * 1024 * 5

// maxSinglePutObjectSize - maximum size 5GiB of object per PUT
// operation.
const maxSinglePutObjectSize = 1024 * 1024 * 1024 * 5

// maxMultipartPutObjectSize - maximum size 5TiB of object for
// Multipart operation.
const maxMultipartPutObjectSize = 1024 * 1024 * 1024 * 1024 * 5

// unsignedPayload - value to be set to X-Amz-Content-Sha256 header when
// we don't want to sign the request payload
const unsignedPayload = "UNSIGNED-PAYLOAD"

// Total number of parallel workers used for multipart operation.
const totalWorkers = 4

// Signature related constants.
const (
	signV4Algorithm   = "AWS4-HMAC-SHA256"
	iso8601DateFormat = "20060102T150405Z"
)

const (
	// Storage class header.
	amzStorageClass = "X-Amz-Storage-Class"

	// Website redirect location header
	amzWebsiteRedirectLocation = "X-Amz-Website-Redirect-Location"

	// Object Tagging headers
	amzTaggingHeader          = "X-Amz-Tagging"
	amzTaggingHeaderDirective = "X-Amz-Tagging-Directive"

	// Object legal hold header
	amzLegalHoldHeader = "X-Amz-Object-Lock-Legal-Hold"

	// Object retention header
	amzLockMode         = "X-Amz-Object-Lock-Mode"
	amzLockRetainUntil  = "X-Amz-Object-Lock-Retain-Until-Date"
	amzBypassGovernance = "X-Amz-Bypass-Governance-Retention"
)
