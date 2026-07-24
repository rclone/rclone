//go:build !plan9 && !solaris && !js

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License. See License.txt in the project root for license information.
//
// Adapted from internal/arrow/arrow_test.go in the Azure SDK for Go
// github.com/Azure/azure-sdk-for-go/sdk/storage/azblob at commit
// c6fa341ca22b (branch feature/storage/bifrost).

package arrowlist

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	arrowArray "github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/stretchr/testify/require"
)

// coreSchema returns a basic Arrow schema for testing with common blob fields.
func coreSchema(md *arrow.Metadata) *arrow.Schema {
	fields := []arrow.Field{
		{Name: "Name", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "Creation-Time", Type: &arrow.TimestampType{Unit: arrow.Microsecond}, Nullable: true},
		{Name: "Last-Modified", Type: &arrow.TimestampType{Unit: arrow.Microsecond}, Nullable: true},
		{Name: "BlobType", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "Etag", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "Content-Length", Type: arrow.PrimitiveTypes.Uint64, Nullable: true},
		{Name: "Content-Type", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "ServerEncrypted", Type: arrow.FixedWidthTypes.Boolean, Nullable: true},
		{Name: "AccessTier", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "LeaseState", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "LeaseStatus", Type: arrow.BinaryTypes.String, Nullable: true},
	}
	return arrow.NewSchema(fields, md)
}

// buildArrowStream constructs a synthetic Arrow IPC stream from a schema and a populate function.
func buildArrowStream(t *testing.T, schema *arrow.Schema, populate func(builder *arrowArray.RecordBuilder)) []byte {
	t.Helper()
	alloc := memory.DefaultAllocator

	var buf bytes.Buffer
	w := ipc.NewWriter(&buf, ipc.WithSchema(schema), ipc.WithAllocator(alloc))

	builder := arrowArray.NewRecordBuilder(alloc, schema)
	defer builder.Release()

	populate(builder)

	rec := builder.NewRecordBatch()
	defer rec.Release()

	require.NoError(t, w.Write(rec))
	require.NoError(t, w.Close())

	return buf.Bytes()
}

func makeHTTPResponse(data []byte, headers map[string]string) *http.Response {
	h := http.Header{}
	h.Set("Content-Type", ArrowContentType)
	for k, v := range headers {
		h.Set(k, v)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     h,
		Body:       io.NopCloser(bytes.NewReader(data)),
	}
}

func TestHandleFlatListResponse_BasicParsing(t *testing.T) {
	md := arrow.NewMetadata([]string{"NextMarker", "NumberOfRecords"}, []string{"marker123", "2"})
	schema := coreSchema(&md)

	data := buildArrowStream(t, schema, func(b *arrowArray.RecordBuilder) {
		b.Field(0).(*arrowArray.StringBuilder).Append("blob1.txt")
		b.Field(1).(*arrowArray.TimestampBuilder).Append(arrow.Timestamp(1700000000000000))
		b.Field(2).(*arrowArray.TimestampBuilder).Append(arrow.Timestamp(1700000000000000))
		b.Field(3).(*arrowArray.StringBuilder).Append("BlockBlob")
		b.Field(4).(*arrowArray.StringBuilder).Append("0x1234")
		b.Field(5).(*arrowArray.Uint64Builder).Append(1024)
		b.Field(6).(*arrowArray.StringBuilder).Append("application/octet-stream")
		b.Field(7).(*arrowArray.BooleanBuilder).Append(true)
		b.Field(8).(*arrowArray.StringBuilder).Append("Hot")
		b.Field(9).(*arrowArray.StringBuilder).Append("available")
		b.Field(10).(*arrowArray.StringBuilder).Append("unlocked")

		b.Field(0).(*arrowArray.StringBuilder).Append("blob2.txt")
		b.Field(1).(*arrowArray.TimestampBuilder).Append(arrow.Timestamp(1700000001000000))
		b.Field(2).(*arrowArray.TimestampBuilder).Append(arrow.Timestamp(1700000001000000))
		b.Field(3).(*arrowArray.StringBuilder).Append("BlockBlob")
		b.Field(4).(*arrowArray.StringBuilder).Append("0x5678")
		b.Field(5).(*arrowArray.Uint64Builder).Append(2048)
		b.Field(6).(*arrowArray.StringBuilder).Append("text/plain")
		b.Field(7).(*arrowArray.BooleanBuilder).Append(false)
		b.Field(8).(*arrowArray.StringBuilder).Append("Cool")
		b.Field(9).(*arrowArray.StringBuilder).Append("available")
		b.Field(10).(*arrowArray.StringBuilder).Append("unlocked")
	})

	resp := makeHTTPResponse(data, nil)
	result, err := HandleFlatListResponse(resp)
	require.NoError(t, err)

	require.NotNil(t, result.NextMarker)
	require.Equal(t, "marker123", *result.NextMarker)
	require.Len(t, result.Segment.BlobItems, 2)

	item := result.Segment.BlobItems[0]
	require.Equal(t, "blob1.txt", *item.Name)
	require.Equal(t, "BlockBlob", string(*item.Properties.BlobType))
	require.Equal(t, int64(1024), *item.Properties.ContentLength)
	require.Equal(t, "Hot", string(*item.Properties.AccessTier))
}

func TestHandleFlatListResponse_EmptyRecordBatch(t *testing.T) {
	md := arrow.NewMetadata([]string{"NextMarker", "NumberOfRecords"}, []string{"", "0"})
	schema := coreSchema(&md)

	data := buildArrowStream(t, schema, func(b *arrowArray.RecordBuilder) {
		// No rows added
	})

	resp := makeHTTPResponse(data, nil)
	result, err := HandleFlatListResponse(resp)
	require.NoError(t, err)
	require.Nil(t, result.NextMarker)
	require.Empty(t, result.Segment.BlobItems)
}

func TestHandleFlatListResponse_NullFields(t *testing.T) {
	md := arrow.NewMetadata([]string{"NextMarker"}, []string{""})
	schema := coreSchema(&md)

	data := buildArrowStream(t, schema, func(b *arrowArray.RecordBuilder) {
		b.Field(0).(*arrowArray.StringBuilder).Append("blob.txt")
		b.Field(1).(*arrowArray.TimestampBuilder).AppendNull()
		b.Field(2).(*arrowArray.TimestampBuilder).AppendNull()
		b.Field(3).(*arrowArray.StringBuilder).AppendNull()
		b.Field(4).(*arrowArray.StringBuilder).AppendNull()
		b.Field(5).(*arrowArray.Uint64Builder).AppendNull()
		b.Field(6).(*arrowArray.StringBuilder).AppendNull()
		b.Field(7).(*arrowArray.BooleanBuilder).AppendNull()
		b.Field(8).(*arrowArray.StringBuilder).AppendNull()
		b.Field(9).(*arrowArray.StringBuilder).AppendNull()
		b.Field(10).(*arrowArray.StringBuilder).AppendNull()
	})

	resp := makeHTTPResponse(data, nil)
	result, err := HandleFlatListResponse(resp)
	require.NoError(t, err)
	require.Len(t, result.Segment.BlobItems, 1)

	item := result.Segment.BlobItems[0]
	require.Equal(t, "blob.txt", *item.Name)
	require.Nil(t, item.Properties.CreationTime)
	require.Nil(t, item.Properties.BlobType)
	require.Nil(t, item.Properties.ETag)
	require.Nil(t, item.Properties.ContentLength)
	require.Nil(t, item.Properties.ContentType)
	require.Nil(t, item.Properties.ServerEncrypted)
}

func TestHandleFlatListResponse_UnknownColumnsIgnored(t *testing.T) {
	md := arrow.NewMetadata([]string{"NextMarker"}, []string{""})
	fields := []arrow.Field{
		{Name: "Name", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "FutureField", Type: arrow.BinaryTypes.String, Nullable: true},
	}
	schema := arrow.NewSchema(fields, &md)

	data := buildArrowStream(t, schema, func(b *arrowArray.RecordBuilder) {
		b.Field(0).(*arrowArray.StringBuilder).Append("blob.txt")
		b.Field(1).(*arrowArray.StringBuilder).Append("some-future-value")
	})

	resp := makeHTTPResponse(data, nil)
	result, err := HandleFlatListResponse(resp)
	require.NoError(t, err)
	require.Len(t, result.Segment.BlobItems, 1)
	require.Equal(t, "blob.txt", *result.Segment.BlobItems[0].Name)
}

func TestHandleFlatListResponse_MultiRecordBatch(t *testing.T) {
	alloc := memory.DefaultAllocator
	md := arrow.NewMetadata([]string{"NextMarker", "NumberOfRecords"}, []string{"", "3"})
	schema := coreSchema(&md)

	var buf bytes.Buffer
	w := ipc.NewWriter(&buf, ipc.WithSchema(schema), ipc.WithAllocator(alloc))

	// First batch: 2 rows
	builder := arrowArray.NewRecordBuilder(alloc, schema)
	for i := 0; i < 2; i++ {
		builder.Field(0).(*arrowArray.StringBuilder).Append("batch1_blob")
		for j := 1; j < 11; j++ {
			builder.Field(j).AppendNull()
		}
	}
	rec := builder.NewRecordBatch()
	require.NoError(t, w.Write(rec))
	rec.Release()
	builder.Release()

	// Second batch: 1 row
	builder = arrowArray.NewRecordBuilder(alloc, schema)
	builder.Field(0).(*arrowArray.StringBuilder).Append("batch2_blob")
	for j := 1; j < 11; j++ {
		builder.Field(j).AppendNull()
	}
	rec = builder.NewRecordBatch()
	require.NoError(t, w.Write(rec))
	rec.Release()
	builder.Release()

	require.NoError(t, w.Close())

	resp := makeHTTPResponse(buf.Bytes(), nil)
	result, err := HandleFlatListResponse(resp)
	require.NoError(t, err)
	require.Len(t, result.Segment.BlobItems, 3)
}

func TestHandleHierarchyListResponse_SeparatesBlobsAndPrefixes(t *testing.T) {
	md := arrow.NewMetadata([]string{"NextMarker"}, []string{""})
	fields := []arrow.Field{
		{Name: "Name", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "ResourceType", Type: arrow.BinaryTypes.String, Nullable: true},
	}
	schema := arrow.NewSchema(fields, &md)

	data := buildArrowStream(t, schema, func(b *arrowArray.RecordBuilder) {
		// A regular blob
		b.Field(0).(*arrowArray.StringBuilder).Append("folder/file.txt")
		b.Field(1).(*arrowArray.StringBuilder).AppendNull()

		// A blob prefix (virtual directory)
		b.Field(0).(*arrowArray.StringBuilder).Append("folder/subfolder/")
		b.Field(1).(*arrowArray.StringBuilder).Append("blobprefix")
	})

	resp := makeHTTPResponse(data, nil)
	result, err := HandleHierarchyListResponse(resp)
	require.NoError(t, err)

	require.Len(t, result.Segment.BlobItems, 1)
	require.Equal(t, "folder/file.txt", *result.Segment.BlobItems[0].Name)

	require.Len(t, result.Segment.BlobPrefixes, 1)
	require.Equal(t, "folder/subfolder/", *result.Segment.BlobPrefixes[0].Name)
}

func TestHandleFlatListResponse_ResponseHeaders(t *testing.T) {
	md := arrow.NewMetadata([]string{"NextMarker"}, []string{""})
	fields := []arrow.Field{
		{Name: "Name", Type: arrow.BinaryTypes.String, Nullable: false},
	}
	schema := arrow.NewSchema(fields, &md)

	data := buildArrowStream(t, schema, func(b *arrowArray.RecordBuilder) {
		b.Field(0).(*arrowArray.StringBuilder).Append("blob.txt")
	})

	resp := makeHTTPResponse(data, map[string]string{
		"x-ms-request-id":        "req-123",
		"x-ms-version":           "2026-10-06",
		"x-ms-client-request-id": "client-req-456",
		"Date":                   "Mon, 01 Jan 2024 00:00:00 GMT",
	})

	result, err := HandleFlatListResponse(resp)
	require.NoError(t, err)
	require.Equal(t, "req-123", *result.RequestID)
	require.Equal(t, "2026-10-06", *result.Version)
	require.Equal(t, "client-req-456", *result.ClientRequestID)
	require.NotNil(t, result.Date)
}

func TestHandleFlatListResponse_MapFields(t *testing.T) {
	md := arrow.NewMetadata([]string{"NextMarker"}, []string{""})
	fields := []arrow.Field{
		{Name: "Name", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "Metadata", Type: arrow.MapOf(arrow.BinaryTypes.String, arrow.BinaryTypes.String), Nullable: true},
		{Name: "Tags", Type: arrow.MapOf(arrow.BinaryTypes.String, arrow.BinaryTypes.String), Nullable: true},
	}
	schema := arrow.NewSchema(fields, &md)

	data := buildArrowStream(t, schema, func(b *arrowArray.RecordBuilder) {
		b.Field(0).(*arrowArray.StringBuilder).Append("blob.txt")

		metaBuilder := b.Field(1).(*arrowArray.MapBuilder)
		metaBuilder.Append(true)
		metaBuilder.KeyBuilder().(*arrowArray.StringBuilder).Append("key1")
		metaBuilder.ItemBuilder().(*arrowArray.StringBuilder).Append("value1")

		tagBuilder := b.Field(2).(*arrowArray.MapBuilder)
		tagBuilder.Append(true)
		tagBuilder.KeyBuilder().(*arrowArray.StringBuilder).Append("env")
		tagBuilder.ItemBuilder().(*arrowArray.StringBuilder).Append("prod")
	})

	resp := makeHTTPResponse(data, nil)
	result, err := HandleFlatListResponse(resp)
	require.NoError(t, err)
	require.Len(t, result.Segment.BlobItems, 1)

	item := result.Segment.BlobItems[0]
	require.NotNil(t, item.Metadata)
	require.Equal(t, "value1", *item.Metadata["key1"])

	require.NotNil(t, item.BlobTags)
	require.Len(t, item.BlobTags.BlobTagSet, 1)
}

func TestHandleFlatListResponse_BooleanFields(t *testing.T) {
	md := arrow.NewMetadata([]string{"NextMarker"}, []string{""})
	fields := []arrow.Field{
		{Name: "Name", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "Deleted", Type: arrow.FixedWidthTypes.Boolean, Nullable: true},
		{Name: "IsCurrentVersion", Type: arrow.FixedWidthTypes.Boolean, Nullable: true},
		{Name: "ServerEncrypted", Type: arrow.FixedWidthTypes.Boolean, Nullable: true},
	}
	schema := arrow.NewSchema(fields, &md)

	data := buildArrowStream(t, schema, func(b *arrowArray.RecordBuilder) {
		b.Field(0).(*arrowArray.StringBuilder).Append("blob.txt")
		b.Field(1).(*arrowArray.BooleanBuilder).Append(true)
		b.Field(2).(*arrowArray.BooleanBuilder).Append(false)
		b.Field(3).(*arrowArray.BooleanBuilder).Append(true)
	})

	resp := makeHTTPResponse(data, nil)
	result, err := HandleFlatListResponse(resp)
	require.NoError(t, err)

	item := result.Segment.BlobItems[0]
	require.True(t, *item.Deleted)
	require.False(t, *item.IsCurrentVersion)
	require.True(t, *item.Properties.ServerEncrypted)
}

func TestHandleFlatListResponse_VersionAndSnapshot(t *testing.T) {
	md := arrow.NewMetadata([]string{"NextMarker"}, []string{""})
	fields := []arrow.Field{
		{Name: "Name", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "VersionId", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "Snapshot", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "IsCurrentVersion", Type: arrow.FixedWidthTypes.Boolean, Nullable: true},
	}
	schema := arrow.NewSchema(fields, &md)

	data := buildArrowStream(t, schema, func(b *arrowArray.RecordBuilder) {
		b.Field(0).(*arrowArray.StringBuilder).Append("versioned-blob.txt")
		b.Field(1).(*arrowArray.StringBuilder).Append("2024-01-01T00:00:00.0000000Z")
		b.Field(2).(*arrowArray.StringBuilder).Append("snap-123")
		b.Field(3).(*arrowArray.BooleanBuilder).Append(true)
	})

	resp := makeHTTPResponse(data, nil)
	result, err := HandleFlatListResponse(resp)
	require.NoError(t, err)

	item := result.Segment.BlobItems[0]
	require.Equal(t, "2024-01-01T00:00:00.0000000Z", *item.VersionID)
	require.Equal(t, "snap-123", *item.Snapshot)
	require.True(t, *item.IsCurrentVersion)
}
