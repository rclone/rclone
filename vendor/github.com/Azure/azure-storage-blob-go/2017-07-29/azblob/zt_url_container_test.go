package azblob_test

import (
	"context"
	"net/http"
	"time"

	"github.com/Azure/azure-storage-blob-go/2017-07-29/azblob"
	chk "gopkg.in/check.v1" // go get gopkg.in/check.v1
	"bytes"
)

type ContainerURLSuite struct{}

var _ = chk.Suite(&ContainerURLSuite{})

func delContainer(c *chk.C, container azblob.ContainerURL) {
	resp, err := container.Delete(context.Background(), azblob.ContainerAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.Response().StatusCode, chk.Equals, 202)
}

func (s *ContainerURLSuite) TestCreateDelete(c *chk.C) {
	containerName := generateContainerName()
	sa := getBSU()
	container := sa.NewContainerURL(containerName)

	cResp, err := container.Create(context.Background(), nil, azblob.PublicAccessNone)
	c.Assert(err, chk.IsNil)
	c.Assert(cResp.Response().StatusCode, chk.Equals, 201)
	c.Assert(cResp.Date().IsZero(), chk.Equals, false)
	c.Assert(cResp.ETag(), chk.Not(chk.Equals), azblob.ETagNone)
	c.Assert(cResp.LastModified().IsZero(), chk.Equals, false)
	c.Assert(cResp.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(cResp.Version(), chk.Not(chk.Equals), "")

	containers, err := sa.ListContainersSegment(context.Background(), azblob.Marker{}, azblob.ListContainersSegmentOptions{Prefix: containerPrefix})
	c.Assert(err, chk.IsNil)
	c.Assert(containers.Containers, chk.HasLen, 1)
	c.Assert(containers.Containers[0].Name, chk.Equals, containerName)

	dResp, err := container.Delete(context.Background(), azblob.ContainerAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(dResp.Response().StatusCode, chk.Equals, 202)
	c.Assert(dResp.Date().IsZero(), chk.Equals, false)
	c.Assert(dResp.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(dResp.Version(), chk.Not(chk.Equals), "")

	containers, err = sa.ListContainersSegment(context.Background(), azblob.Marker{}, azblob.ListContainersSegmentOptions{Prefix: containerPrefix})
	c.Assert(err, chk.IsNil)
	c.Assert(containers.Containers, chk.HasLen, 0)
}

/*func (s *ContainerURLSuite) TestGetProperties(c *chk.C) {
	container := getContainer(c)
	defer delContainer(c, container)

	props, err := container.GetProperties(context.Background(), LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(props.Response().StatusCode, chk.Equals, 200)
	c.Assert(props.BlobPublicAccess().IsZero(), chk.Equals, true)
	c.Assert(props.ETag(), chk.Not(chk.Equals), ETagNone)
	verifyDateResp(c, props.LastModified, false)
}*/

func (s *ContainerURLSuite) TestGetSetPermissions(c *chk.C) {
	bsu := getBSU()
	container, _ := createNewContainer(c, bsu)
	defer delContainer(c, container)

	now := time.Now().UTC().Truncate(10000 * time.Millisecond) // Enough resolution
	permissions := []azblob.SignedIdentifier{{
		ID: "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTI=",
		AccessPolicy: azblob.AccessPolicy{
			Start:      now,
			Expiry:     now.Add(5 * time.Minute).UTC(),
			Permission: "rw",
		},
	}}
	sResp, err := container.SetAccessPolicy(context.Background(), azblob.PublicAccessNone, permissions, azblob.ContainerAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(sResp.Response().StatusCode, chk.Equals, 200)
	c.Assert(sResp.Date().IsZero(), chk.Equals, false)
	c.Assert(sResp.ETag(), chk.Not(chk.Equals), azblob.ETagNone)
	c.Assert(sResp.LastModified().IsZero(), chk.Equals, false)
	c.Assert(sResp.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(sResp.Version(), chk.Not(chk.Equals), "")

	gResp, err := container.GetAccessPolicy(context.Background(), azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(gResp.Response().StatusCode, chk.Equals, 200)
	c.Assert(gResp.BlobPublicAccess(), chk.Equals, azblob.PublicAccessNone)
	c.Assert(gResp.Date().IsZero(), chk.Equals, false)
	c.Assert(gResp.ETag(), chk.Not(chk.Equals), azblob.ETagNone)
	c.Assert(gResp.LastModified().IsZero(), chk.Equals, false)
	c.Assert(gResp.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(gResp.Version(), chk.Not(chk.Equals), "")
	c.Assert(gResp.Value, chk.HasLen, 1)
	c.Assert(gResp.Value[0], chk.DeepEquals, permissions[0])
}

func (s *ContainerURLSuite) TestGetSetMetadata(c *chk.C) {
	bsu := getBSU()
	container, _ := createNewContainer(c, bsu)
	defer delContainer(c, container)

	// TODO: add test case ensuring that we get back case-sensitive keys
	md := azblob.Metadata{
		"foo": "FooValuE",
		"bar": "bArvaLue",
	}
	sResp, err := container.SetMetadata(context.Background(), md, azblob.ContainerAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(sResp.Response().StatusCode, chk.Equals, 200)
	c.Assert(sResp.Date().IsZero(), chk.Equals, false)
	c.Assert(sResp.ETag(), chk.Not(chk.Equals), azblob.ETagNone)
	c.Assert(sResp.LastModified().IsZero(), chk.Equals, false)
	c.Assert(sResp.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(sResp.Version(), chk.Not(chk.Equals), "")

	gResp, err := container.GetProperties(context.Background(), azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(gResp.Response().StatusCode, chk.Equals, 200)
	c.Assert(gResp.Date().IsZero(), chk.Equals, false)
	c.Assert(gResp.ETag(), chk.Not(chk.Equals), azblob.ETagNone)
	c.Assert(gResp.LastModified().IsZero(), chk.Equals, false)
	c.Assert(gResp.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(gResp.Version(), chk.Not(chk.Equals), "")
	nmd := gResp.NewMetadata()
	c.Assert(nmd, chk.DeepEquals, md)
}

func (s *ContainerURLSuite) TestListBlobs(c *chk.C) {
	bsu := getBSU()
	container, _ := createNewContainer(c, bsu)
	defer delContainer(c, container)

	blobs, err := container.ListBlobsFlatSegment(context.Background(), azblob.Marker{}, azblob.ListBlobsSegmentOptions{})
	c.Assert(err, chk.IsNil)
	c.Assert(blobs.Response().StatusCode, chk.Equals, 200)
	c.Assert(blobs.ContentType(), chk.Not(chk.Equals), "")
	c.Assert(blobs.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(blobs.Version(), chk.Not(chk.Equals), "")
	c.Assert(blobs.Date().IsZero(), chk.Equals, false)
	c.Assert(blobs.Blobs.Blob, chk.HasLen, 0)
	c.Assert(blobs.ServiceEndpoint, chk.NotNil)
	c.Assert(blobs.ContainerName, chk.NotNil)
	c.Assert(blobs.Prefix, chk.Equals, "")
	c.Assert(blobs.Marker, chk.Equals, "")
	c.Assert(blobs.MaxResults, chk.Equals, int32(0))
	c.Assert(blobs.Delimiter, chk.Equals, "")

	blob := container.NewBlockBlobURL(generateBlobName())

	_, err = blob.Upload(context.Background(), bytes.NewReader(nil), azblob.BlobHTTPHeaders{}, nil, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	blobs, err = container.ListBlobsFlatSegment(context.Background(), azblob.Marker{}, azblob.ListBlobsSegmentOptions{})
	c.Assert(err, chk.IsNil)
	c.Assert(blobs.Blobs.Blob, chk.HasLen, 1)
	c.Assert(blobs.Blobs.Blob[0].Name, chk.NotNil)
	c.Assert(blobs.Blobs.Blob[0].Snapshot =="", chk.Equals, true)
	c.Assert(blobs.Blobs.Blob[0].Metadata, chk.HasLen, 0)
	c.Assert(blobs.Blobs.Blob[0].Properties, chk.NotNil)
	c.Assert(blobs.Blobs.Blob[0].Properties.LastModified, chk.NotNil)
	c.Assert(blobs.Blobs.Blob[0].Properties.Etag, chk.NotNil)
	c.Assert(blobs.Blobs.Blob[0].Properties.ContentLength, chk.NotNil)
	c.Assert(blobs.Blobs.Blob[0].Properties.ContentType, chk.NotNil)
	c.Assert(blobs.Blobs.Blob[0].Properties.ContentEncoding, chk.NotNil)
	c.Assert(blobs.Blobs.Blob[0].Properties.ContentLanguage, chk.NotNil)
	c.Assert(blobs.Blobs.Blob[0].Properties.ContentMD5, chk.NotNil)
	c.Assert(blobs.Blobs.Blob[0].Properties.ContentDisposition, chk.NotNil)
	c.Assert(blobs.Blobs.Blob[0].Properties.CacheControl, chk.NotNil)
	c.Assert(blobs.Blobs.Blob[0].Properties.BlobSequenceNumber, chk.IsNil)
	c.Assert(blobs.Blobs.Blob[0].Properties.BlobType, chk.NotNil)
	c.Assert(blobs.Blobs.Blob[0].Properties.LeaseStatus, chk.Equals, azblob.LeaseStatusUnlocked)
	c.Assert(blobs.Blobs.Blob[0].Properties.LeaseState, chk.Equals, azblob.LeaseStateAvailable)
	c.Assert(string(blobs.Blobs.Blob[0].Properties.LeaseDuration), chk.Equals, "")
	c.Assert(blobs.Blobs.Blob[0].Properties.CopyID, chk.IsNil)
	c.Assert(string(blobs.Blobs.Blob[0].Properties.CopyStatus), chk.Equals, "")
	c.Assert(blobs.Blobs.Blob[0].Properties.CopySource, chk.IsNil)
	c.Assert(blobs.Blobs.Blob[0].Properties.CopyProgress, chk.IsNil)
	c.Assert(blobs.Blobs.Blob[0].Properties.CopyCompletionTime, chk.IsNil)
	c.Assert(blobs.Blobs.Blob[0].Properties.CopyStatusDescription, chk.IsNil)
	c.Assert(blobs.Blobs.Blob[0].Properties.ServerEncrypted, chk.NotNil)
	c.Assert(blobs.Blobs.Blob[0].Properties.IncrementalCopy, chk.IsNil)
}

func (s *ContainerURLSuite) TestLeaseAcquireRelease(c *chk.C) {
	bsu := getBSU()
	container, _ := createNewContainer(c, bsu)
	defer delContainer(c, container)

	acq, err := container.AcquireLease(context.Background(), "", 15, azblob.HTTPAccessConditions{})
	leaseID := acq.LeaseID()
	c.Assert(err, chk.IsNil)
	c.Assert(acq.StatusCode(), chk.Equals, 201)
	c.Assert(acq.Date().IsZero(), chk.Equals, false)
	c.Assert(acq.ETag(), chk.Not(chk.Equals), azblob.ETagNone)
	c.Assert(acq.LastModified().IsZero(), chk.Equals, false)
	c.Assert(acq.LeaseID(), chk.Equals, leaseID)
	c.Assert(acq.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(acq.Version(), chk.Not(chk.Equals), "")

	rel, err := container.ReleaseLease(context.Background(), leaseID, azblob.HTTPAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(rel.StatusCode(), chk.Equals, 200)
	c.Assert(rel.Date().IsZero(), chk.Equals, false)
	c.Assert(rel.ETag(), chk.Not(chk.Equals), azblob.ETagNone)
	c.Assert(rel.LastModified().IsZero(), chk.Equals, false)
	c.Assert(rel.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(rel.Version(), chk.Not(chk.Equals), "")
}

func (s *ContainerURLSuite) TestLeaseRenewChangeBreak(c *chk.C) {
	bsu := getBSU()
	container, _ := createNewContainer(c, bsu)
	defer delContainer(c, container)

	al, err := container.AcquireLease(context.Background(), "", 15, azblob.HTTPAccessConditions{})
	leaseID := al.LeaseID()
	c.Assert(err, chk.IsNil)

	newID := newUUID().String()
	cl, err := container.ChangeLease(context.Background(), leaseID, newID, azblob.HTTPAccessConditions{})
	newID = cl.LeaseID()
	c.Assert(err, chk.IsNil)
	c.Assert(cl.StatusCode(), chk.Equals, 200)
	c.Assert(cl.Date().IsZero(), chk.Equals, false)
	c.Assert(cl.ETag(), chk.Not(chk.Equals), azblob.ETagNone)
	c.Assert(cl.LastModified().IsZero(), chk.Equals, false)
	c.Assert(cl.LeaseID(), chk.Equals, newID)
	c.Assert(cl.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(cl.Version(), chk.Not(chk.Equals), "")

	rl, err := container.RenewLease(context.Background(), newID, azblob.HTTPAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(rl.StatusCode(), chk.Equals, 200)
	c.Assert(rl.Date().IsZero(), chk.Equals, false)
	c.Assert(rl.ETag(), chk.Not(chk.Equals), azblob.ETagNone)
	c.Assert(rl.LastModified().IsZero(), chk.Equals, false)
	c.Assert(rl.LeaseID(), chk.Equals, newID)
	c.Assert(rl.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(rl.Version(), chk.Not(chk.Equals), "")

	bl, err := container.BreakLease(context.Background(), 5, azblob.HTTPAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(bl.StatusCode(), chk.Equals, 202)
	c.Assert(bl.Date().IsZero(), chk.Equals, false)
	c.Assert(bl.ETag(), chk.Not(chk.Equals), azblob.ETagNone)
	c.Assert(bl.LastModified().IsZero(), chk.Equals, false)
	c.Assert(bl.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(bl.Version(), chk.Not(chk.Equals), "")

	_, err = container.ReleaseLease(context.Background(), newID, azblob.HTTPAccessConditions{})
	c.Assert(err, chk.IsNil)
}

func (s *ContainerURLSuite) TestListBlobsPaged(c *chk.C) {
	bsu := getBSU()
	container, _ := createNewContainer(c, bsu)

	const numBlobs = 4
	const maxResultsPerPage = 2

	blobs := make([]azblob.BlockBlobURL, numBlobs)
	for i := 0; i < numBlobs; i++ {
		blobs[i], _ = createNewBlockBlob(c, container)
	}

	defer delContainer(c, container)

	marker := azblob.Marker{}
	iterations := numBlobs / maxResultsPerPage

	for i := 0; i < iterations; i++ {
		resp, err := container.ListBlobsFlatSegment(context.Background(), marker, azblob.ListBlobsSegmentOptions{MaxResults: maxResultsPerPage})
		c.Assert(err, chk.IsNil)
		c.Assert(resp.Blobs.Blob, chk.HasLen, maxResultsPerPage)

		hasMore := i < iterations-1
		c.Assert(resp.NextMarker.NotDone(), chk.Equals, hasMore)
		marker = resp.NextMarker
	}
}

func (s *ContainerURLSuite) TestSetMetadataCondition(c *chk.C) {
	bsu := getBSU()
	container, _ := createNewContainer(c, bsu)
	defer delContainer(c, container)
	time.Sleep(time.Second * 3)
	currTime := time.Now().UTC()
	rResp, err := container.SetMetadata(context.Background(), azblob.Metadata{"foo": "bar"},
		azblob.ContainerAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currTime}})
	c.Assert(err, chk.NotNil)
	c.Assert(rResp, chk.IsNil)
	se, ok := err.(azblob.StorageError)
	c.Assert(ok, chk.Equals, true)
	c.Assert(se.Response().StatusCode, chk.Equals, http.StatusPreconditionFailed)
	gResp, err := container.GetProperties(context.Background(), azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	md := gResp.NewMetadata()
	c.Assert(md, chk.HasLen, 0)
}

func (s *ContainerURLSuite) TestListBlobsWithPrefix(c *chk.C) {
	bsu := getBSU()
	container, _ := createNewContainer(c, bsu)
	defer delContainer(c, container)

	prefixes := []string{
		"one/",
		"one/",
		"one/",
		"two/",
		"three/",
		"three/",
	}

	for _, prefix := range prefixes {
		createBlockBlobWithPrefix(c, container, prefix)
	}

	blobs, err := container.ListBlobsHierarchySegment(context.Background(), azblob.Marker{}, "/", azblob.ListBlobsSegmentOptions{})
	c.Assert(err, chk.IsNil)
	c.Assert(blobs.Blobs.BlobPrefix, chk.HasLen, 3)
	c.Assert(blobs.Blobs.Blob, chk.HasLen, 0)
}
