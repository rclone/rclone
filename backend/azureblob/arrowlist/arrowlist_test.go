//go:build !plan9 && !solaris && !js

package arrowlist

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/apache/arrow-go/v18/arrow"
	arrowArray "github.com/apache/arrow-go/v18/arrow/array"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// arrowPage builds a one row Arrow IPC listing page with the given blob name
// and NextMarker.
func arrowPage(t *testing.T, name, nextMarker string) []byte {
	md := arrow.NewMetadata([]string{"NextMarker", "NumberOfRecords"}, []string{nextMarker, "1"})
	fields := []arrow.Field{
		{Name: "Name", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "Content-Length", Type: arrow.PrimitiveTypes.Uint64, Nullable: true},
	}
	schema := arrow.NewSchema(fields, &md)
	return buildArrowStream(t, schema, func(b *arrowArray.RecordBuilder) {
		b.Field(0).(*arrowArray.StringBuilder).Append(name)
		b.Field(1).(*arrowArray.Uint64Builder).Append(42)
	})
}

func TestPagerArrow(t *testing.T) {
	var requests []*http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Clone(context.Background()))
		w.Header().Set("Content-Type", ArrowContentType)
		if r.URL.Query().Get("marker") == "" {
			_, _ = w.Write(arrowPage(t, "blob1.txt", "marker1"))
		} else {
			_, _ = w.Write(arrowPage(t, "blob2.txt", ""))
		}
	}))
	defer srv.Close()

	// SAS style query on the endpoint must be preserved in requests.
	client, err := NewClientWithNoCredential(srv.URL+"/testcontainer?sv=fakesas", nil)
	require.NoError(t, err)

	opts := &ListBlobsHierarchyOptions{
		ListBlobsHierarchyOptions: container.ListBlobsHierarchyOptions{
			Include:    container.ListBlobsInclude{Metadata: true, Tags: true},
			Prefix:     to.Ptr("dir/"),
			MaxResults: to.Ptr(int32(1000)),
			StartFrom:  to.Ptr("testcontainer/dir/a"),
		},
		EndBefore:      to.Ptr("testcontainer/dir/n"),
		UseArrowFormat: to.Ptr(true),
	}
	pager := client.NewListBlobsHierarchyPager("/", opts)

	var names []string
	for pager.More() {
		page, err := pager.NextPage(context.Background())
		require.NoError(t, err)
		assert.Equal(t, ArrowContentType, *page.ContentType)
		for _, item := range page.Segment.BlobItems {
			names = append(names, *item.Name)
			assert.Equal(t, int64(42), *item.Properties.ContentLength)
		}
	}
	assert.Equal(t, []string{"blob1.txt", "blob2.txt"}, names)

	require.Len(t, requests, 2)
	q := requests[0].URL.Query()
	assert.Equal(t, "list", q.Get("comp"))
	assert.Equal(t, "container", q.Get("restype"))
	assert.Equal(t, "/", q.Get("delimiter"))
	assert.Equal(t, "metadata,tags", q.Get("include"))
	assert.Equal(t, "dir/", q.Get("prefix"))
	assert.Equal(t, "1000", q.Get("maxresults"))
	assert.Equal(t, "testcontainer/dir/a", q.Get("startFrom"))
	assert.Equal(t, "testcontainer/dir/n", q.Get("endBefore"))
	assert.Equal(t, "fakesas", q.Get("sv"))
	assert.Empty(t, q.Get("marker"))
	assert.Equal(t, ArrowAcceptHeader, requests[0].Header.Get("Accept"))
	assert.Equal(t, serviceVersion, requests[0].Header.Get("x-ms-version"))
	assert.Equal(t, "marker1", requests[1].URL.Query().Get("marker"))

	// The caller's options must not have been mutated by paging.
	assert.Nil(t, opts.Marker)
}

const xmlListResponse = `<?xml version="1.0" encoding="utf-8"?>
<EnumerationResults ServiceEndpoint="https://example.com/" ContainerName="testcontainer">
  <Blobs>
    <Blob>
      <Name>file.txt</Name>
      <Properties>
        <Last-Modified>Mon, 01 Jan 2024 00:00:00 GMT</Last-Modified>
        <Etag>0x8D1</Etag>
        <Content-Length>7</Content-Length>
        <BlobType>BlockBlob</BlobType>
      </Properties>
    </Blob>
    <BlobPrefix>
      <Name>subdir/</Name>
    </BlobPrefix>
  </Blobs>
  <NextMarker />
</EnumerationResults>`

func TestPagerXMLFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(xmlListResponse))
	}))
	defer srv.Close()

	client, err := NewClientWithNoCredential(srv.URL+"/testcontainer", nil)
	require.NoError(t, err)

	pager := client.NewListBlobsHierarchyPager("/", &ListBlobsHierarchyOptions{
		UseArrowFormat: to.Ptr(true),
	})
	page, err := pager.NextPage(context.Background())
	require.NoError(t, err)
	assert.False(t, pager.More())
	assert.Equal(t, "application/xml", *page.ContentType)
	require.Len(t, page.Segment.BlobItems, 1)
	assert.Equal(t, "file.txt", *page.Segment.BlobItems[0].Name)
	assert.Equal(t, int64(7), *page.Segment.BlobItems[0].Properties.ContentLength)
	require.Len(t, page.Segment.BlobPrefixes, 1)
	assert.Equal(t, "subdir/", *page.Segment.BlobPrefixes[0].Name)
}

func TestPagerXMLFallbackWithEndBefore(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(xmlListResponse))
	}))
	defer srv.Close()

	client, err := NewClientWithNoCredential(srv.URL+"/testcontainer", nil)
	require.NoError(t, err)

	pager := client.NewListBlobsHierarchyPager("/", &ListBlobsHierarchyOptions{
		EndBefore:      to.Ptr("testcontainer/n"),
		UseArrowFormat: to.Ptr(true),
	})
	_, err = pager.NextPage(context.Background())
	require.ErrorIs(t, err, ErrEndBeforeXMLFallback)
}

func TestPagerSharedKeyAuth(t *testing.T) {
	var gotAuth, gotDate string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotDate = r.Header.Get("x-ms-date")
		w.Header().Set("Content-Type", ArrowContentType)
		_, _ = w.Write(arrowPage(t, "blob.txt", ""))
	}))
	defer srv.Close()

	cred, err := NewSharedKeyCredential("testaccount", base64.StdEncoding.EncodeToString([]byte("testkey")))
	require.NoError(t, err)
	client, err := NewClientWithSharedKeyCredential(srv.URL+"/testcontainer", cred, nil)
	require.NoError(t, err)

	pager := client.NewListBlobsHierarchyPager("/", &ListBlobsHierarchyOptions{
		UseArrowFormat: to.Ptr(true),
	})
	_, err = pager.NextPage(context.Background())
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(gotAuth, "SharedKey testaccount:"), "Authorization = %q", gotAuth)
	assert.NotEmpty(t, gotDate)
}

func TestPagerResponseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ms-error-code", string(bloberror.ContainerNotFound))
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client, err := NewClientWithNoCredential(srv.URL+"/testcontainer", nil)
	require.NoError(t, err)

	pager := client.NewListBlobsHierarchyPager("/", &ListBlobsHierarchyOptions{
		UseArrowFormat: to.Ptr(true),
	})
	_, err = pager.NextPage(context.Background())
	require.Error(t, err)
	assert.True(t, bloberror.HasCode(err, bloberror.ContainerNotFound), "expected ContainerNotFound, got %v", err)
}
