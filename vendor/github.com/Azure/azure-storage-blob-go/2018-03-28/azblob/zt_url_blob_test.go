package azblob_test

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/Azure/azure-storage-blob-go/2018-03-28/azblob"

	"bytes"

	chk "gopkg.in/check.v1" // go get gopkg.in/check.v1
)

type BlobURLSuite struct{}

var _ = chk.Suite(&BlobURLSuite{})

func (b *BlobURLSuite) TestCreateDelete(c *chk.C) {
	bsu := getBSU()
	container, _ := createNewContainer(c, bsu)
	defer delContainer(c, container)

	blob := container.NewBlockBlobURL(generateBlobName())

	putResp, err := blob.Upload(context.Background(), bytes.NewReader(nil), azblob.BlobHTTPHeaders{}, nil, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(putResp.Response().StatusCode, chk.Equals, 201)
	c.Assert(putResp.ETag(), chk.Not(chk.Equals), azblob.ETagNone)
	c.Assert(putResp.LastModified().IsZero(), chk.Equals, false)
	c.Assert(putResp.ContentMD5(), chk.Not(chk.Equals), "")
	c.Assert(putResp.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(putResp.Version(), chk.Not(chk.Equals), "")
	c.Assert(putResp.Date().IsZero(), chk.Equals, false)

	delResp, err := blob.Delete(context.Background(), azblob.DeleteSnapshotsOptionNone, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(delResp.Response().StatusCode, chk.Equals, 202)
	c.Assert(delResp.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(delResp.Version(), chk.Not(chk.Equals), "")
	c.Assert(delResp.Date().IsZero(), chk.Equals, false)
}

func (b *BlobURLSuite) TestGetSetProperties(c *chk.C) {
	bsu := getBSU()
	container, _ := createNewContainer(c, bsu)
	defer delContainer(c, container)

	blob, _ := createNewBlockBlob(c, container)

	properties := azblob.BlobHTTPHeaders{
		ContentType:     "mytype",
		ContentLanguage: "martian",
	}
	setResp, err := blob.SetHTTPHeaders(context.Background(), properties, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(setResp.Response().StatusCode, chk.Equals, 200)
	c.Assert(setResp.ETag(), chk.Not(chk.Equals), azblob.ETagNone)
	c.Assert(setResp.LastModified().IsZero(), chk.Equals, false)
	c.Assert(setResp.BlobSequenceNumber(), chk.Equals, int64(-1))
	c.Assert(setResp.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(setResp.Version(), chk.Not(chk.Equals), "")
	c.Assert(setResp.Date().IsZero(), chk.Equals, false)

	/*getResp, err := blob.GetProperties(context.Background(), BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(getResp.Response().StatusCode, chk.Equals, 200)
	c.Assert(getResp.ContentType(), chk.Equals, properties.ContentType)
	c.Assert(getResp.ContentLanguage(), chk.Equals, properties.ContentLanguage)
	verifyDateResp(c, getResp.LastModified, false)
	c.Assert(getResp.BlobType(), chk.Not(chk.Equals), "")
	verifyDateResp(c, getResp.CopyCompletionTime, true)
	c.Assert(getResp.CopyStatusDescription(), chk.Equals, "")
	c.Assert(getResp.CopyID(), chk.Equals, "")
	c.Assert(getResp.CopyProgress(), chk.Equals, "")
	c.Assert(getResp.CopySource(), chk.Equals, "")
	c.Assert(getResp.CopyStatus().IsZero(), chk.Equals, true)
	c.Assert(getResp.IsIncrementalCopy(), chk.Equals, "")
	c.Assert(getResp.LeaseDuration().IsZero(), chk.Equals, true)
	c.Assert(getResp.LeaseState(), chk.Equals, LeaseStateAvailable)
	c.Assert(getResp.LeaseStatus(), chk.Equals, LeaseStatusUnlocked)
	c.Assert(getResp.ContentLength(), chk.Not(chk.Equals), "")
	c.Assert(getResp.ETag(), chk.Not(chk.Equals), ETagNone)
	c.Assert(getResp.ContentMD5(), chk.Equals, "")
	c.Assert(getResp.ContentEncoding(), chk.Equals, "")
	c.Assert(getResp.ContentDisposition(), chk.Equals, "")
	c.Assert(getResp.CacheControl(), chk.Equals, "")
	c.Assert(getResp.BlobSequenceNumber(), chk.Equals, "")
	c.Assert(getResp.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(getResp.Version(), chk.Not(chk.Equals), "")
	verifyDateResp(c, getResp.Date, false)
	c.Assert(getResp.AcceptRanges(), chk.Equals, "bytes")
	c.Assert(getResp.BlobCommittedBlockCount(), chk.Equals, "")
	*/
}

func (b *BlobURLSuite) TestGetSetMetadata(c *chk.C) {
	bsu := getBSU()
	container, _ := createNewContainer(c, bsu)
	defer delContainer(c, container)

	blob, _ := createNewBlockBlob(c, container)

	metadata := azblob.Metadata{
		"foo": "foovalue",
		"bar": "barvalue",
	}
	setResp, err := blob.SetMetadata(context.Background(), metadata, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(setResp.Response().StatusCode, chk.Equals, 200)
	c.Assert(setResp.LastModified().IsZero(), chk.Equals, false)
	c.Assert(setResp.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(setResp.Version(), chk.Not(chk.Equals), "")
	c.Assert(setResp.Date().IsZero(), chk.Equals, false)

	getResp, err := blob.GetProperties(context.Background(), azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(getResp.ETag(), chk.Not(chk.Equals), azblob.ETagNone)
	c.Assert(getResp.LastModified().IsZero(), chk.Equals, false)
	c.Assert(getResp.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(getResp.Version(), chk.Not(chk.Equals), "")
	c.Assert(getResp.Date().IsZero(), chk.Equals, false)
	md := getResp.NewMetadata()
	c.Assert(md, chk.DeepEquals, metadata)
}

func (b *BlobURLSuite) TestCopy(c *chk.C) {
	bsu := getBSU()
	container, _ := createNewContainer(c, bsu)
	defer delContainer(c, container)

	sourceBlob, _ := createNewBlockBlob(c, container)
	_, err := sourceBlob.Upload(context.Background(), getReaderToRandomBytes(2048), azblob.BlobHTTPHeaders{}, nil, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	destBlob, _ := createNewBlockBlob(c, container)
	copyResp, err := destBlob.StartCopyFromURL(context.Background(), sourceBlob.URL(), nil, azblob.BlobAccessConditions{}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(copyResp.Response().StatusCode, chk.Equals, 202)
	c.Assert(copyResp.ETag(), chk.Not(chk.Equals), azblob.ETagNone)
	c.Assert(copyResp.LastModified().IsZero(), chk.Equals, false)
	c.Assert(copyResp.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(copyResp.Version(), chk.Not(chk.Equals), "")
	c.Assert(copyResp.Date().IsZero(), chk.Equals, false)
	c.Assert(copyResp.CopyID(), chk.Not(chk.Equals), "")
	c.Assert(copyResp.CopyStatus(), chk.Not(chk.Equals), "")

	abortResp, err := destBlob.AbortCopyFromURL(context.Background(), copyResp.CopyID(), azblob.LeaseAccessConditions{})
	// small copy completes before we have time to abort so check for failure case
	c.Assert(err, chk.NotNil)
	c.Assert(abortResp, chk.IsNil)
	se, ok := err.(azblob.StorageError)
	c.Assert(ok, chk.Equals, true)
	c.Assert(se.Response().StatusCode, chk.Equals, http.StatusConflict)
}

func (b *BlobURLSuite) TestSnapshot(c *chk.C) {
	bsu := getBSU()
	container, _ := createNewContainer(c, bsu)
	defer delContainer(c, container)

	blob, _ := createNewBlockBlob(c, container)

	resp, err := blob.CreateSnapshot(context.Background(), nil, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.Response().StatusCode, chk.Equals, 201)
	c.Assert(resp.Snapshot() == "", chk.Equals, false)
	c.Assert(resp.ETag(), chk.Not(chk.Equals), azblob.ETagNone)
	c.Assert(resp.LastModified().IsZero(), chk.Equals, false)
	c.Assert(resp.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(resp.Version(), chk.Not(chk.Equals), "")
	c.Assert(resp.Date().IsZero(), chk.Equals, false)

	blobs, err := container.ListBlobsFlatSegment(context.Background(), azblob.Marker{}, azblob.ListBlobsSegmentOptions{Details: azblob.BlobListingDetails{Snapshots: true}})
	c.Assert(err, chk.IsNil)
	c.Assert(blobs.Segment.BlobItems, chk.HasLen, 2)

	_, err = blob.Delete(context.Background(), azblob.DeleteSnapshotsOptionOnly, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	blobs, err = container.ListBlobsFlatSegment(context.Background(), azblob.Marker{}, azblob.ListBlobsSegmentOptions{Details: azblob.BlobListingDetails{Snapshots: true}})
	c.Assert(err, chk.IsNil)
	c.Assert(blobs.Segment.BlobItems, chk.HasLen, 1)
}

// Copied from policy_unique_request_id.go
type uuid [16]byte

// The UUID reserved variants.
const (
	reservedNCS       byte = 0x80
	reservedRFC4122   byte = 0x40
	reservedMicrosoft byte = 0x20
	reservedFuture    byte = 0x00
)

func newUUID() (u uuid) {
	u = uuid{}
	// Set all bits to randomly (or pseudo-randomly) chosen values.
	_, err := rand.Read(u[:])
	if err != nil {
		panic("ran.Read failed")
	}
	u[8] = (u[8] | reservedRFC4122) & 0x7F // u.setVariant(ReservedRFC4122)

	var version byte = 4
	u[6] = (u[6] & 0xF) | (version << 4) // u.setVersion(4)
	return
}

func (u uuid) String() string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:])
}

func (b *BlobURLSuite) TestLeaseAcquireRelease(c *chk.C) {
	bsu := getBSU()
	container, _ := createNewContainer(c, bsu)
	defer delContainer(c, container)

	blob, _ := createNewBlockBlob(c, container)

	acq, err := blob.AcquireLease(context.Background(), "", 15, azblob.HTTPAccessConditions{})
	leaseID := acq.LeaseID() // FIX
	c.Assert(err, chk.IsNil)
	c.Assert(acq.StatusCode(), chk.Equals, 201)
	c.Assert(acq.Date().IsZero(), chk.Equals, false)
	c.Assert(acq.ETag(), chk.Not(chk.Equals), azblob.ETagNone)
	c.Assert(acq.LastModified().IsZero(), chk.Equals, false)
	c.Assert(acq.LeaseID(), chk.Equals, leaseID)
	c.Assert(acq.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(acq.Version(), chk.Not(chk.Equals), "")

	rel, err := blob.ReleaseLease(context.Background(), leaseID, azblob.HTTPAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(rel.StatusCode(), chk.Equals, 200)
	c.Assert(rel.Date().IsZero(), chk.Equals, false)
	c.Assert(rel.ETag(), chk.Not(chk.Equals), azblob.ETagNone)
	c.Assert(rel.LastModified().IsZero(), chk.Equals, false)
	c.Assert(rel.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(rel.Version(), chk.Not(chk.Equals), "")
}

func (b *BlobURLSuite) TestLeaseRenewChangeBreak(c *chk.C) {
	bsu := getBSU()
	container, _ := createNewContainer(c, bsu)
	defer delContainer(c, container)

	blob, _ := createNewBlockBlob(c, container)

	acq, err := blob.AcquireLease(context.Background(), newUUID().String(), 15, azblob.HTTPAccessConditions{})
	c.Assert(err, chk.IsNil)
	leaseID := acq.LeaseID()

	chg, err := blob.ChangeLease(context.Background(), leaseID, newUUID().String(), azblob.HTTPAccessConditions{})
	c.Assert(err, chk.IsNil)
	newID := chg.LeaseID()
	c.Assert(chg.StatusCode(), chk.Equals, 200)
	c.Assert(chg.Date().IsZero(), chk.Equals, false)
	c.Assert(chg.ETag(), chk.Not(chk.Equals), azblob.ETagNone)
	c.Assert(chg.LastModified().IsZero(), chk.Equals, false)
	c.Assert(chg.LeaseID(), chk.Equals, newID)
	c.Assert(chg.Version(), chk.Not(chk.Equals), "")

	renew, err := blob.RenewLease(context.Background(), newID, azblob.HTTPAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(renew.StatusCode(), chk.Equals, 200)
	c.Assert(renew.Date().IsZero(), chk.Equals, false)
	c.Assert(renew.ETag(), chk.Not(chk.Equals), azblob.ETagNone)
	c.Assert(renew.LastModified().IsZero(), chk.Equals, false)
	c.Assert(renew.LeaseID(), chk.Equals, newID)
	c.Assert(renew.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(renew.Version(), chk.Not(chk.Equals), "")

	brk, err := blob.BreakLease(context.Background(), 5, azblob.HTTPAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(brk.StatusCode(), chk.Equals, 202)
	c.Assert(brk.Date().IsZero(), chk.Equals, false)
	c.Assert(brk.ETag(), chk.Not(chk.Equals), azblob.ETagNone)
	c.Assert(brk.LastModified().IsZero(), chk.Equals, false)
	c.Assert(brk.LeaseTime(), chk.Not(chk.Equals), "")
	c.Assert(brk.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(brk.Version(), chk.Not(chk.Equals), "")

	_, err = blob.ReleaseLease(context.Background(), newID, azblob.HTTPAccessConditions{})
	c.Assert(err, chk.IsNil)
}

func (b *BlobURLSuite) TestGetBlobRange(c *chk.C) {
	bsu := getBSU()
	container, _ := createNewContainer(c, bsu)
	defer delContainer(c, container)

	blob, _ := createNewBlockBlob(c, container)
	contentR, contentD := getRandomDataAndReader(2048)
	_, err := blob.Upload(context.Background(), contentR, azblob.BlobHTTPHeaders{}, nil, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := blob.Download(context.Background(), 0, 1024, azblob.BlobAccessConditions{}, false)
	c.Assert(err, chk.IsNil)
	c.Assert(resp.ContentLength(), chk.Equals, int64(1024))

	download, err := ioutil.ReadAll(resp.Response().Body)
	c.Assert(err, chk.IsNil)
	c.Assert(download, chk.DeepEquals, contentD[:1024])

	resp, err = blob.Download(context.Background(), 1024, 0, azblob.BlobAccessConditions{}, false)

	c.Assert(err, chk.IsNil)
	c.Assert(resp.ContentLength(), chk.Equals, int64(1024))

	download, err = ioutil.ReadAll(resp.Response().Body)
	c.Assert(err, chk.IsNil)
	c.Assert(download, chk.DeepEquals, contentD[1024:])

	c.Assert(resp.AcceptRanges(), chk.Equals, "bytes")
	c.Assert(resp.BlobCommittedBlockCount(), chk.Equals, int32(-1))
	c.Assert(resp.BlobContentMD5(), chk.Not(chk.Equals), [md5.Size]byte{})
	c.Assert(resp.BlobSequenceNumber(), chk.Equals, int64(-1))
	c.Assert(resp.BlobType(), chk.Equals, azblob.BlobBlockBlob)
	c.Assert(resp.CacheControl(), chk.Equals, "")
	c.Assert(resp.ContentDisposition(), chk.Equals, "")
	c.Assert(resp.ContentEncoding(), chk.Equals, "")
	c.Assert(resp.ContentRange(), chk.Equals, "bytes 1024-2047/2048")
	c.Assert(resp.ContentType(), chk.Equals, "application/octet-stream")
	c.Assert(resp.CopyCompletionTime().IsZero(), chk.Equals, true)
	c.Assert(resp.CopyID(), chk.Equals, "")
	c.Assert(resp.CopyProgress(), chk.Equals, "")
	c.Assert(resp.CopySource(), chk.Equals, "")
	c.Assert(resp.CopyStatus(), chk.Equals, azblob.CopyStatusNone)
	c.Assert(resp.CopyStatusDescription(), chk.Equals, "")
	c.Assert(resp.Date().IsZero(), chk.Equals, false)
	c.Assert(resp.ETag(), chk.Not(chk.Equals), azblob.ETagNone)
	c.Assert(resp.LastModified().IsZero(), chk.Equals, false)
	c.Assert(resp.LeaseDuration(), chk.Equals, azblob.LeaseDurationNone)
	c.Assert(resp.LeaseState(), chk.Equals, azblob.LeaseStateAvailable)
	c.Assert(resp.LeaseStatus(), chk.Equals, azblob.LeaseStatusUnlocked)
	c.Assert(resp.NewMetadata(), chk.DeepEquals, azblob.Metadata{})
	c.Assert(resp.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(resp.Response().StatusCode, chk.Equals, 206)
	c.Assert(resp.Version(), chk.Not(chk.Equals), "")
}
