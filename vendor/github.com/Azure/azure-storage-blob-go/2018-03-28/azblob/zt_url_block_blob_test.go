package azblob_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/Azure/azure-storage-blob-go/2018-03-28/azblob"
	chk "gopkg.in/check.v1" // go get gopkg.in/check.v1
)

type BlockBlobURLSuite struct{}

var _ = chk.Suite(&BlockBlobURLSuite{})

func (b *BlockBlobURLSuite) TestStageGetBlocks(c *chk.C) {
	bsu := getBSU()
	container, _ := createNewContainer(c, bsu)
	defer delContainer(c, container)

	blob := container.NewBlockBlobURL(generateBlobName())

	blockID := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%6d", 0)))

	putResp, err := blob.StageBlock(context.Background(), blockID, getReaderToRandomBytes(1024), azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(putResp.Response().StatusCode, chk.Equals, 201)
	c.Assert(putResp.ContentMD5(), chk.Not(chk.Equals), "")
	c.Assert(putResp.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(putResp.Version(), chk.Not(chk.Equals), "")
	c.Assert(putResp.Date().IsZero(), chk.Equals, false)

	blockList, err := blob.GetBlockList(context.Background(), azblob.BlockListAll, azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(blockList.Response().StatusCode, chk.Equals, 200)
	c.Assert(blockList.LastModified().IsZero(), chk.Equals, true)
	c.Assert(blockList.ETag(), chk.Equals, azblob.ETagNone)
	c.Assert(blockList.ContentType(), chk.Not(chk.Equals), "")
	c.Assert(blockList.BlobContentLength(), chk.Equals, int64(-1))
	c.Assert(blockList.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(blockList.Version(), chk.Not(chk.Equals), "")
	c.Assert(blockList.Date().IsZero(), chk.Equals, false)
	c.Assert(blockList.CommittedBlocks, chk.HasLen, 0)
	c.Assert(blockList.UncommittedBlocks, chk.HasLen, 1)

	listResp, err := blob.CommitBlockList(context.Background(), []string{blockID}, azblob.BlobHTTPHeaders{}, nil, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(listResp.Response().StatusCode, chk.Equals, 201)
	c.Assert(listResp.LastModified().IsZero(), chk.Equals, false)
	c.Assert(listResp.ETag(), chk.Not(chk.Equals), azblob.ETagNone)
	c.Assert(listResp.ContentMD5(), chk.Not(chk.Equals), "")
	c.Assert(listResp.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(listResp.Version(), chk.Not(chk.Equals), "")
	c.Assert(listResp.Date().IsZero(), chk.Equals, false)

	blockList, err = blob.GetBlockList(context.Background(), azblob.BlockListAll, azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(blockList.Response().StatusCode, chk.Equals, 200)
	c.Assert(blockList.LastModified().IsZero(), chk.Equals, false)
	c.Assert(blockList.ETag(), chk.Not(chk.Equals), azblob.ETagNone)
	c.Assert(blockList.ContentType(), chk.Not(chk.Equals), "")
	c.Assert(blockList.BlobContentLength(), chk.Equals, int64(1024))
	c.Assert(blockList.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(blockList.Version(), chk.Not(chk.Equals), "")
	c.Assert(blockList.Date().IsZero(), chk.Equals, false)
	c.Assert(blockList.CommittedBlocks, chk.HasLen, 1)
	c.Assert(blockList.UncommittedBlocks, chk.HasLen, 0)
}

func (b *BlockBlobURLSuite) TestStageBlockFromURL(c *chk.C) {
	bsu := getBSU()
	credential, err := getCredential()
	if err != nil {
		c.Skip(err.Error())
	}
	container, _ := createNewContainer(c, bsu)
	defer delContainer(c, container)

	testSize := 8 * 1024 * 1024 // 8MB
	r, sourceData := getRandomDataAndReader(testSize)
	ctx := context.Background() // Use default Background context
	srcBlob := container.NewBlockBlobURL(generateBlobName())
	destBlob := container.NewBlockBlobURL(generateBlobName())

	// Prepare source blob for copy.
	uploadSrcResp, err := srcBlob.Upload(ctx, r, azblob.BlobHTTPHeaders{}, azblob.Metadata{}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(uploadSrcResp.Response().StatusCode, chk.Equals, 201)

	// Get source blob URL with SAS for StageFromURL.
	srcBlobParts := azblob.NewBlobURLParts(srcBlob.URL())

	srcBlobParts.SAS = azblob.BlobSASSignatureValues{
		Protocol:      azblob.SASProtocolHTTPS,              // Users MUST use HTTPS (not HTTP)
		ExpiryTime:    time.Now().UTC().Add(48 * time.Hour), // 48-hours before expiration
		ContainerName: srcBlobParts.ContainerName,
		BlobName:      srcBlobParts.BlobName,
		Permissions:   azblob.BlobSASPermissions{Read: true}.String(),
	}.NewSASQueryParameters(credential)

	srcBlobURLWithSAS := srcBlobParts.URL()

	// Stage blocks from URL.
	blockID1, blockID2 := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%6d", 0))), base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%6d", 1)))
	stageResp1, err := destBlob.StageBlockFromURL(ctx, blockID1, srcBlobURLWithSAS, 0, 4*1024*1024, azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(stageResp1.Response().StatusCode, chk.Equals, 201)
	c.Assert(stageResp1.ContentMD5(), chk.Not(chk.Equals), "")
	c.Assert(stageResp1.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(stageResp1.Version(), chk.Not(chk.Equals), "")
	c.Assert(stageResp1.Date().IsZero(), chk.Equals, false)

	stageResp2, err := destBlob.StageBlockFromURL(ctx, blockID2, srcBlobURLWithSAS, 4*1024*1024, azblob.CountToEnd, azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(stageResp2.Response().StatusCode, chk.Equals, 201)
	c.Assert(stageResp2.ContentMD5(), chk.Not(chk.Equals), "")
	c.Assert(stageResp2.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(stageResp2.Version(), chk.Not(chk.Equals), "")
	c.Assert(stageResp2.Date().IsZero(), chk.Equals, false)

	// Check block list.
	blockList, err := destBlob.GetBlockList(context.Background(), azblob.BlockListAll, azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(blockList.Response().StatusCode, chk.Equals, 200)
	c.Assert(blockList.CommittedBlocks, chk.HasLen, 0)
	c.Assert(blockList.UncommittedBlocks, chk.HasLen, 2)

	// Commit block list.
	listResp, err := destBlob.CommitBlockList(context.Background(), []string{blockID1, blockID2}, azblob.BlobHTTPHeaders{}, nil, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(listResp.Response().StatusCode, chk.Equals, 201)

	// Check data integrity through downloading.
	downloadResp, err := destBlob.BlobURL.Download(ctx, 0, azblob.CountToEnd, azblob.BlobAccessConditions{}, false)
	c.Assert(err, chk.IsNil)
	destData, err := ioutil.ReadAll(downloadResp.Body(azblob.RetryReaderOptions{}))
	c.Assert(err, chk.IsNil)
	c.Assert(destData, chk.DeepEquals, sourceData)
}
