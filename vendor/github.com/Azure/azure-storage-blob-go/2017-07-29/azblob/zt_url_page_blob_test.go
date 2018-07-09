package azblob_test

import (
	"context"

	"github.com/Azure/azure-storage-blob-go/2017-07-29/azblob"
	chk "gopkg.in/check.v1" // go get gopkg.in/check.v1
)

type PageBlobURLSuite struct{}

var _ = chk.Suite(&PageBlobURLSuite{})

func (b *PageBlobURLSuite) TestPutGetPages(c *chk.C) {
	bsu := getBSU()
	container, _ := createNewContainer(c, bsu)
	defer delContainer(c, container)

	blob, _ := createNewPageBlob(c, container)

	pageRange := azblob.PageRange{Start: 0, End: 1023}
	putResp, err := blob.UploadPages(context.Background(), 0, getReaderToRandomBytes(1024), azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(putResp.Response().StatusCode, chk.Equals, 201)
	c.Assert(putResp.LastModified().IsZero(), chk.Equals, false)
	c.Assert(putResp.ETag(), chk.Not(chk.Equals), azblob.ETagNone)
	c.Assert(putResp.ContentMD5(), chk.Not(chk.Equals), "")
	c.Assert(putResp.BlobSequenceNumber(), chk.Equals, int64(0))
	c.Assert(putResp.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(putResp.Version(), chk.Not(chk.Equals), "")
	c.Assert(putResp.Date().IsZero(), chk.Equals, false)

	pageList, err := blob.GetPageRanges(context.Background(), 0, 1023, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(pageList.Response().StatusCode, chk.Equals, 200)
	c.Assert(pageList.LastModified().IsZero(), chk.Equals, false)
	c.Assert(pageList.ETag(), chk.Not(chk.Equals), azblob.ETagNone)
	c.Assert(pageList.BlobContentLength(), chk.Equals, int64(512*10))
	c.Assert(pageList.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(pageList.Version(), chk.Not(chk.Equals), "")
	c.Assert(pageList.Date().IsZero(), chk.Equals, false)
	c.Assert(pageList.PageRange, chk.HasLen, 1)
	c.Assert(pageList.PageRange[0], chk.DeepEquals, pageRange)
}

func (b *PageBlobURLSuite) TestClearDiffPages(c *chk.C) {
	bsu := getBSU()
	container, _ := createNewContainer(c, bsu)
	defer delContainer(c, container)

	blob, _ := createNewPageBlob(c, container)
	_, err := blob.UploadPages(context.Background(), 0, getReaderToRandomBytes(2048), azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	snapshotResp, err := blob.CreateSnapshot(context.Background(), nil, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	_, err = blob.UploadPages(context.Background(), 2048, getReaderToRandomBytes(2048), azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	pageList, err := blob.GetPageRangesDiff(context.Background(), 0, 4096, snapshotResp.Snapshot(), azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(pageList.PageRange, chk.HasLen, 1)
	c.Assert(pageList.PageRange[0].Start, chk.Equals, int64(2048))
	c.Assert(pageList.PageRange[0].End, chk.Equals, int64(4095))

	clearResp, err := blob.ClearPages(context.Background(), 2048, 2048, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(clearResp.Response().StatusCode, chk.Equals, 201)

	pageList, err = blob.GetPageRangesDiff(context.Background(), 0, 4095, snapshotResp.Snapshot(), azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(pageList.PageRange, chk.HasLen, 0)
}

func (b *PageBlobURLSuite) TestIncrementalCopy(c *chk.C) {
	bsu := getBSU()
	container, _ := createNewContainer(c, bsu)
	defer delContainer(c, container)
	_, err := container.SetAccessPolicy(context.Background(), azblob.PublicAccessBlob, nil, azblob.ContainerAccessConditions{})
	c.Assert(err, chk.IsNil)

	srcBlob, _ := createNewPageBlob(c, container)
	_, err = srcBlob.UploadPages(context.Background(), 0, getReaderToRandomBytes(1024), azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	snapshotResp, err := srcBlob.CreateSnapshot(context.Background(), nil, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	dstBlob := container.NewPageBlobURL(generateBlobName())

	resp, err := dstBlob.StartCopyIncremental(context.Background(), srcBlob.URL(), snapshotResp.Snapshot(), azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.Response().StatusCode, chk.Equals, 202)
	c.Assert(resp.LastModified().IsZero(), chk.Equals, false)
	c.Assert(resp.ETag(), chk.Not(chk.Equals), azblob.ETagNone)
	c.Assert(resp.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(resp.Version(), chk.Not(chk.Equals), "")
	c.Assert(resp.Date().IsZero(), chk.Equals, false)
	c.Assert(resp.CopyID(), chk.Not(chk.Equals), "")
	c.Assert(resp.CopyStatus(), chk.Equals, azblob.CopyStatusPending)

	waitForIncrementalCopy(c, dstBlob, resp)
}

func (b *PageBlobURLSuite) TestResizePageBlob(c *chk.C) {
	bsu := getBSU()
	container, _ := createNewContainer(c, bsu)
	defer delContainer(c, container)

	blob, _ := createNewPageBlob(c, container)
	resp, err := blob.Resize(context.Background(), 2048, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.Response().StatusCode, chk.Equals, 200)

	resp, err = blob.Resize(context.Background(), 8192, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.Response().StatusCode, chk.Equals, 200)

	resp2, err := blob.GetProperties(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp2.ContentLength(), chk.Equals, int64(8192))
}

func (b *PageBlobURLSuite) TestPageSequenceNumbers(c *chk.C) {
	bsu := getBSU()
	container, _ := createNewContainer(c, bsu)
	blob, _ := createNewPageBlob(c, container)

	defer delContainer(c, container)

	resp, err := blob.UpdateSequenceNumber(context.Background(), azblob.SequenceNumberActionIncrement, 0, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.Response().StatusCode, chk.Equals, 200)

	resp, err = blob.UpdateSequenceNumber(context.Background(), azblob.SequenceNumberActionMax, 7, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.Response().StatusCode, chk.Equals, 200)

	resp, err = blob.UpdateSequenceNumber(context.Background(), azblob.SequenceNumberActionUpdate, 11, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.Response().StatusCode, chk.Equals, 200)
}
