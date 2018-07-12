package azblob

import (
	"crypto/md5"
	"encoding/base64"
	"time"
)

// BlobHTTPHeaders contains read/writeable blob properties.
type BlobHTTPHeaders struct {
	ContentType        string
	ContentMD5         [md5.Size]byte
	ContentEncoding    string
	ContentLanguage    string
	ContentDisposition string
	CacheControl       string
}

func (h BlobHTTPHeaders) contentMD5Pointer() *string {
	if h.ContentMD5 == [md5.Size]byte{} {
		return nil
	}
	str := base64.StdEncoding.EncodeToString(h.ContentMD5[:])
	return &str
}

// NewHTTPHeaders returns the user-modifiable properties for this blob.
func (gr GetResponse) NewHTTPHeaders() BlobHTTPHeaders {
	return BlobHTTPHeaders{
		ContentType:        gr.ContentType(),
		ContentEncoding:    gr.ContentEncoding(),
		ContentLanguage:    gr.ContentLanguage(),
		ContentDisposition: gr.ContentDisposition(),
		CacheControl:       gr.CacheControl(),
		ContentMD5:         gr.ContentMD5(),
	}
}

// NewHTTPHeaders returns the user-modifiable properties for this blob.
func (bgpr BlobsGetPropertiesResponse) NewHTTPHeaders() BlobHTTPHeaders {
	return BlobHTTPHeaders{
		ContentType:        bgpr.ContentType(),
		ContentEncoding:    bgpr.ContentEncoding(),
		ContentLanguage:    bgpr.ContentLanguage(),
		ContentDisposition: bgpr.ContentDisposition(),
		CacheControl:       bgpr.CacheControl(),
		ContentMD5:         bgpr.ContentMD5(),
	}
}

func md5StringToMD5(md5String string) (hash [md5.Size]byte) {
	if md5String == "" {
		return
	}
	md5Slice, err := base64.StdEncoding.DecodeString(md5String)
	if err != nil {
		panic(err)
	}
	copy(hash[:], md5Slice)
	return
}

// ContentMD5 returns the value for header Content-MD5.
func (ababr AppendBlobsAppendBlockResponse) ContentMD5() [md5.Size]byte {
	return md5StringToMD5(ababr.rawResponse.Header.Get("Content-MD5"))
}

// ContentMD5 returns the value for header Content-MD5.
func (bgpr BlobsGetPropertiesResponse) ContentMD5() [md5.Size]byte {
	return md5StringToMD5(bgpr.rawResponse.Header.Get("Content-MD5"))
}

// ContentMD5 returns the value for header Content-MD5.
func (bpr BlobsPutResponse) ContentMD5() [md5.Size]byte {
	return md5StringToMD5(bpr.rawResponse.Header.Get("Content-MD5"))
}

// ContentMD5 returns the value for header Content-MD5.
func (bbpblr BlockBlobsPutBlockListResponse) ContentMD5() [md5.Size]byte {
	return md5StringToMD5(bbpblr.rawResponse.Header.Get("Content-MD5"))
}

// ContentMD5 returns the value for header Content-MD5.
func (bbpbr BlockBlobsPutBlockResponse) ContentMD5() [md5.Size]byte {
	return md5StringToMD5(bbpbr.rawResponse.Header.Get("Content-MD5"))
}

// BlobContentMD5 returns the value for header x-ms-blob-content-md5.
func (gr GetResponse) BlobContentMD5() [md5.Size]byte {
	return md5StringToMD5(gr.rawResponse.Header.Get("x-ms-blob-content-md5"))
}

// ContentMD5 returns the value for header Content-MD5.
func (gr GetResponse) ContentMD5() [md5.Size]byte {
	return md5StringToMD5(gr.rawResponse.Header.Get("Content-MD5"))
}

// ContentMD5 returns the value for header Content-MD5.
func (pbppr PageBlobsPutPageResponse) ContentMD5() [md5.Size]byte {
	return md5StringToMD5(pbppr.rawResponse.Header.Get("Content-MD5"))
}

// DestinationSnapshot returns the value for header x-ms-copy-destination-snapshot
func (bgpr BlobsGetPropertiesResponse) DestinationSnapshot() time.Time {
	if bgpr.IsIncrementalCopy() == "true" {
		t := bgpr.rawResponse.Header.Get("x-ms-copy-destination-snapshot")
		snapshot, err := time.Parse("2006-01-02T15:04:05Z", t)
		if err != nil {
			panic(err)
		}
		return snapshot
	}
	return time.Time{}
}
