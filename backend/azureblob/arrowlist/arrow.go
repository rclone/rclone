//go:build !plan9 && !solaris && !js

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License. See License.txt in the project root for license information.
//
// Adapted from internal/arrow/arrow.go in the Azure SDK for Go
// github.com/Azure/azure-sdk-for-go/sdk/storage/azblob at commit
// c6fa341ca22b (branch feature/storage/bifrost), retyped onto the SDK's
// public type aliases so it compiles against the released azblob module.

package arrowlist

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/lease"
	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"github.com/apache/arrow-go/v18/arrow/memory"
)

const (
	// ArrowContentType is the Content-Type for Apache Arrow IPC stream responses.
	ArrowContentType = "application/vnd.apache.arrow.stream"

	// ArrowAcceptHeader is the Accept header value to request Arrow format.
	ArrowAcceptHeader = ArrowContentType

	// resourceTypeBlobPrefix is the ResourceType value that identifies a virtual directory prefix
	// in hierarchy listing responses. In XML, prefixes are separate <BlobPrefix> elements;
	// in Arrow, all rows share the same schema and are distinguished by this field value.
	resourceTypeBlobPrefix = "blobprefix"
)

// HandleFlatListResponse parses an Arrow IPC stream response into a ContainerClientListBlobFlatSegmentResponse.
func HandleFlatListResponse(resp *http.Response) (container.ListBlobsFlatResponse, error) {
	result := container.ListBlobsFlatResponse{}

	// Extract response headers
	extractResponseHeaders(resp, &result.ClientRequestID, &result.ContentType, &result.Date, &result.RequestID, &result.Version)

	// Parse Arrow IPC stream
	items, nextMarker, err := parseArrowStream(resp.Body)
	if err != nil {
		return result, fmt.Errorf("failed to parse Arrow IPC response: %w", err)
	}

	result.ListBlobsFlatSegmentResponse = container.ListBlobsFlatSegmentResponse{
		Segment: &container.BlobFlatListSegment{
			BlobItems: items,
		},
		NextMarker: nextMarker,
	}

	return result, nil
}

// HandleHierarchyListResponse parses an Arrow IPC stream response into a ContainerClientListBlobHierarchySegmentResponse.
func HandleHierarchyListResponse(resp *http.Response) (container.ListBlobsHierarchyResponse, error) {
	result := container.ListBlobsHierarchyResponse{}

	extractResponseHeaders(resp, &result.ClientRequestID, &result.ContentType, &result.Date, &result.RequestID, &result.Version)

	items, nextMarker, err := parseArrowStream(resp.Body)
	if err != nil {
		return result, fmt.Errorf("failed to parse Arrow IPC response: %w", err)
	}

	// Separate blob items from blob prefixes based on ResourceType field.
	var blobItems []*container.BlobItem
	var blobPrefixes []*container.BlobPrefix
	for _, item := range items {
		if item.Properties != nil && item.Properties.ResourceType != nil && *item.Properties.ResourceType == resourceTypeBlobPrefix {
			blobPrefixes = append(blobPrefixes, &container.BlobPrefix{
				Name:       item.Name,
				Properties: item.Properties,
			})
		} else {
			blobItems = append(blobItems, item)
		}
	}

	result.ListBlobsHierarchySegmentResponse = container.ListBlobsHierarchySegmentResponse{
		Segment: &container.BlobHierarchyListSegment{
			BlobItems:    blobItems,
			BlobPrefixes: blobPrefixes,
		},
		NextMarker: nextMarker,
	}

	return result, nil
}

func extractResponseHeaders(resp *http.Response, clientRequestID, contentType **string, date **time.Time, requestID, version **string) {
	if val := resp.Header.Get("x-ms-client-request-id"); val != "" {
		*clientRequestID = &val
	}
	if val := resp.Header.Get("Content-Type"); val != "" {
		*contentType = &val
	}
	if val := resp.Header.Get("Date"); val != "" {
		if t, err := time.Parse(time.RFC1123, val); err == nil {
			*date = &t
		}
	}
	if val := resp.Header.Get("x-ms-request-id"); val != "" {
		*requestID = &val
	}
	if val := resp.Header.Get("x-ms-version"); val != "" {
		*version = &val
	}
}

// parseArrowStream reads an Arrow IPC stream and converts it to BlobItems.
func parseArrowStream(body io.Reader) ([]*container.BlobItem, *string, error) {
	reader, err := ipc.NewReader(body, ipc.WithAllocator(memory.DefaultAllocator))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create Arrow IPC reader: %w", err)
	}
	defer reader.Release()

	// Extract NextMarker from schema metadata
	var nextMarker *string
	var numRecords int
	if md := reader.Schema().Metadata(); md.Len() > 0 {
		if idx := md.FindKey("NextMarker"); idx >= 0 {
			val := md.Values()[idx]
			if val != "" {
				nextMarker = &val
			}
		}
		if idx := md.FindKey("NumberOfRecords"); idx >= 0 {
			if n, err := strconv.Atoi(md.Values()[idx]); err == nil {
				numRecords = n
			}
		}
	}

	// Build column index for the schema
	schema := reader.Schema()
	colIndex := buildColumnIndex(schema)

	// Pre-allocate if we know the count
	items := make([]*container.BlobItem, 0, numRecords)

	// Iterate over record batches
	for reader.Next() {
		rec := reader.RecordBatch()
		rows := int(rec.NumRows())

		for row := 0; row < rows; row++ {
			item := &container.BlobItem{
				Properties: &container.BlobProperties{},
			}
			populateBlobItem(item, rec, row, colIndex)
			items = append(items, item)
		}
	}

	if err := reader.Err(); err != nil {
		return nil, nil, fmt.Errorf("error reading Arrow record batches: %w", err)
	}

	return items, nextMarker, nil
}

// columnIndex maps column names to their indices in the record batch for O(1) lookup.
type columnIndex map[string]int

func buildColumnIndex(schema *arrow.Schema) columnIndex {
	idx := make(columnIndex, len(schema.Fields()))
	for i, f := range schema.Fields() {
		idx[f.Name] = i
	}
	return idx
}

// populateBlobItem fills a BlobItem from a single row across all columns.
func populateBlobItem(item *container.BlobItem, rec arrow.RecordBatch, row int, colIdx columnIndex) {
	// Top-level BlobItem fields
	item.Name = getStringField(rec, row, colIdx, "Name")
	item.Deleted = getBoolField(rec, row, colIdx, "Deleted")
	item.Snapshot = getStringField(rec, row, colIdx, "Snapshot")
	item.VersionID = getStringField(rec, row, colIdx, "VersionId")
	item.IsCurrentVersion = getBoolField(rec, row, colIdx, "IsCurrentVersion")
	item.HasVersionsOnly = getBoolField(rec, row, colIdx, "HasVersionsOnly")

	// Metadata maps
	item.Metadata = getMapField(rec, row, colIdx, "Metadata")
	item.OrMetadata = getMapField(rec, row, colIdx, "OrMetadata")

	// Tags
	if tags := getMapField(rec, row, colIdx, "Tags"); tags != nil {
		var blobTags []*container.BlobTag
		for k, v := range tags {
			key := k
			blobTags = append(blobTags, &container.BlobTag{Key: &key, Value: v})
		}
		item.BlobTags = &container.BlobTags{BlobTagSet: blobTags}
	}

	// BlobProperties fields
	p := item.Properties
	p.CreationTime = getTimestampField(rec, row, colIdx, "Creation-Time")
	p.LastModified = getTimestampField(rec, row, colIdx, "Last-Modified")
	p.ETag = getETagField(rec, row, colIdx, "Etag")
	p.ContentLength = getInt64Field(rec, row, colIdx, "Content-Length")
	p.ContentType = getStringField(rec, row, colIdx, "Content-Type")
	p.ContentEncoding = getStringField(rec, row, colIdx, "Content-Encoding")
	p.ContentLanguage = getStringField(rec, row, colIdx, "Content-Language")
	p.ContentDisposition = getStringField(rec, row, colIdx, "Content-Disposition")
	p.CacheControl = getStringField(rec, row, colIdx, "Cache-Control")
	p.ContentMD5 = getBase64BytesField(rec, row, colIdx, "Content-MD5")
	p.BlobType = getEnumField[container.BlobType](rec, row, colIdx, "BlobType")
	p.AccessTier = getEnumField[container.AccessTier](rec, row, colIdx, "AccessTier")
	p.AccessTierInferred = getBoolField(rec, row, colIdx, "AccessTierInferred")
	p.AccessTierChangeTime = getTimestampField(rec, row, colIdx, "AccessTierChangeTime")
	p.LeaseState = getEnumField[lease.StateType](rec, row, colIdx, "LeaseState")
	p.LeaseStatus = getEnumField[lease.StatusType](rec, row, colIdx, "LeaseStatus")
	p.LeaseDuration = getEnumField[lease.DurationType](rec, row, colIdx, "LeaseDuration")
	p.ServerEncrypted = getBoolField(rec, row, colIdx, "ServerEncrypted")
	p.CustomerProvidedKeySHA256 = getStringField(rec, row, colIdx, "CustomerProvidedKeySha256")
	p.EncryptionScope = getStringField(rec, row, colIdx, "EncryptionScope")
	p.IncrementalCopy = getBoolField(rec, row, colIdx, "IncrementalCopy")
	p.IsSealed = getBoolField(rec, row, colIdx, "Sealed")
	p.ArchiveStatus = getEnumField[container.ArchiveStatus](rec, row, colIdx, "ArchiveStatus")
	p.RehydratePriority = getEnumField[blob.RehydratePriority](rec, row, colIdx, "RehydratePriority")
	p.CopyID = getStringField(rec, row, colIdx, "CopyId")
	p.CopyStatus = getEnumField[blob.CopyStatusType](rec, row, colIdx, "CopyStatus")
	p.CopySource = getStringField(rec, row, colIdx, "CopySource")
	p.CopyProgress = getStringField(rec, row, colIdx, "CopyProgress")
	p.CopyCompletionTime = getTimestampField(rec, row, colIdx, "CopyCompletionTime")
	p.CopyStatusDescription = getStringField(rec, row, colIdx, "CopyStatusDescription")
	p.DestinationSnapshot = getStringField(rec, row, colIdx, "CopyDestinationSnapshot")
	p.ImmutabilityPolicyExpiresOn = getTimestampField(rec, row, colIdx, "ImmutabilityPolicyUntilDate")
	p.ImmutabilityPolicyMode = getEnumField[container.ImmutabilityPolicyMode](rec, row, colIdx, "ImmutabilityPolicyMode")
	p.LegalHold = getBoolField(rec, row, colIdx, "LegalHold")
	p.DeletedTime = getTimestampField(rec, row, colIdx, "DeletedTime")
	p.RemainingRetentionDays = getInt32Field(rec, row, colIdx, "RemainingRetentionDays")
	p.LastAccessedOn = getTimestampField(rec, row, colIdx, "LastAccessTime")
	p.TagCount = getInt32Field(rec, row, colIdx, "TagCount")
	p.BlobSequenceNumber = getInt64Field(rec, row, colIdx, "x-ms-blob-sequence-number")
	p.ResourceType = getStringField(rec, row, colIdx, "ResourceType")

	// Content-CRC64 is a string field in Arrow but not directly mapped to BlobProperties.
	// The field exists in the Arrow schema but has no corresponding field in the Go SDK BlobProperties.
	// It is silently ignored for forward compatibility.
}

// Field accessor helpers that safely handle missing columns and null values.

func getStringField(rec arrow.RecordBatch, row int, colIdx columnIndex, name string) *string {
	idx, ok := colIdx[name]
	if !ok {
		return nil
	}
	col := rec.Column(idx)
	if col.IsNull(row) {
		return nil
	}
	if arr, ok := col.(*array.String); ok {
		val := arr.Value(row)
		return &val
	}
	return nil
}

func getBoolField(rec arrow.RecordBatch, row int, colIdx columnIndex, name string) *bool {
	idx, ok := colIdx[name]
	if !ok {
		return nil
	}
	col := rec.Column(idx)
	if col.IsNull(row) {
		return nil
	}
	if arr, ok := col.(*array.Boolean); ok {
		val := arr.Value(row)
		return &val
	}
	return nil
}

func getTimestampField(rec arrow.RecordBatch, row int, colIdx columnIndex, name string) *time.Time {
	idx, ok := colIdx[name]
	if !ok {
		return nil
	}
	col := rec.Column(idx)
	if col.IsNull(row) {
		return nil
	}
	if arr, ok := col.(*array.Timestamp); ok {
		val := arr.Value(row)
		// Arrow timestamps are stored as int64 with a unit; convert to time.Time
		dt := val.ToTime(arr.DataType().(*arrow.TimestampType).Unit)
		return &dt
	}
	return nil
}

func getInt64Field(rec arrow.RecordBatch, row int, colIdx columnIndex, name string) *int64 {
	idx, ok := colIdx[name]
	if !ok {
		return nil
	}
	col := rec.Column(idx)
	if col.IsNull(row) {
		return nil
	}
	if arr, ok := col.(*array.Uint64); ok {
		val := int64(arr.Value(row))
		return &val
	}
	if arr, ok := col.(*array.Int64); ok {
		val := arr.Value(row)
		return &val
	}
	return nil
}

func getInt32Field(rec arrow.RecordBatch, row int, colIdx columnIndex, name string) *int32 {
	idx, ok := colIdx[name]
	if !ok {
		return nil
	}
	col := rec.Column(idx)
	if col.IsNull(row) {
		return nil
	}
	if arr, ok := col.(*array.Uint64); ok {
		val := int32(arr.Value(row))
		return &val
	}
	if arr, ok := col.(*array.Int32); ok {
		val := arr.Value(row)
		return &val
	}
	return nil
}

func getETagField(rec arrow.RecordBatch, row int, colIdx columnIndex, name string) *azcore.ETag {
	s := getStringField(rec, row, colIdx, name)
	if s == nil {
		return nil
	}
	etag := azcore.ETag(*s)
	return &etag
}

func getBase64BytesField(rec arrow.RecordBatch, row int, colIdx columnIndex, name string) []byte {
	s := getStringField(rec, row, colIdx, name)
	if s == nil {
		return nil
	}
	decoded, err := base64.StdEncoding.DecodeString(*s)
	if err != nil {
		return nil
	}
	return decoded
}

// getEnumField returns a pointer to a string-typed enum value.
type stringEnum interface {
	~string
}

func getEnumField[T stringEnum](rec arrow.RecordBatch, row int, colIdx columnIndex, name string) *T {
	s := getStringField(rec, row, colIdx, name)
	if s == nil {
		return nil
	}
	val := T(*s)
	return &val
}

func getMapField(rec arrow.RecordBatch, row int, colIdx columnIndex, name string) map[string]*string {
	idx, ok := colIdx[name]
	if !ok {
		return nil
	}
	col := rec.Column(idx)
	if col.IsNull(row) {
		return nil
	}
	mapArr, ok := col.(*array.Map)
	if !ok {
		return nil
	}

	// Get the offsets for this row's map entries
	start := int(mapArr.Offsets()[row])
	end := int(mapArr.Offsets()[row+1])
	if start == end {
		return nil
	}

	keys := mapArr.Keys().(*array.String)
	values := mapArr.Items().(*array.String)

	result := make(map[string]*string, end-start)
	for i := start; i < end; i++ {
		k := keys.Value(i)
		if values.IsNull(i) {
			result[k] = nil
		} else {
			v := values.Value(i)
			result[k] = &v
		}
	}
	return result
}
