package azblob_test

import (
	"context"

	"github.com/Azure/azure-storage-blob-go/2018-03-28/azblob"
	chk "gopkg.in/check.v1" // go get gopkg.in/check.v1
)

type StorageAccountSuite struct{}

var _ = chk.Suite(&StorageAccountSuite{})

/*func (s *StorageAccountSuite) TestGetSetProperties(c *chk.C) {
	sa := getStorageAccount(c)
	setProps := StorageServiceProperties{}
	resp, err := sa.SetProperties(context.Background(), setProps)
	c.Assert(err, chk.IsNil)
	c.Assert(resp.Response().StatusCode, chk.Equals, 202)
	c.Assert(resp.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(resp.Version(), chk.Not(chk.Equals), "")

	props, err := sa.GetProperties(context.Background())
	c.Assert(err, chk.IsNil)
	c.Assert(props.Response().StatusCode, chk.Equals, 200)
	c.Assert(props.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(props.Version(), chk.Not(chk.Equals), "")
	c.Assert(props.Logging, chk.NotNil)
	c.Assert(props.HourMetrics, chk.NotNil)
	c.Assert(props.MinuteMetrics, chk.NotNil)
	c.Assert(props.Cors, chk.HasLen, 0)
	c.Assert(props.DefaultServiceVersion, chk.IsNil) // TODO: this seems like a bug
}

func (s *StorageAccountSuite) TestGetStatus(c *chk.C) {
	sa := getStorageAccount(c)
	if !strings.Contains(sa.URL().Path, "-secondary") {
		c.Skip("only applicable on secondary storage accounts")
	}
	stats, err := sa.GetStats(context.Background())
	c.Assert(err, chk.IsNil)
	c.Assert(stats, chk.NotNil)
}*/

func (s *StorageAccountSuite) TestListContainers(c *chk.C) {
	sa := getBSU()
	resp, err := sa.ListContainersSegment(context.Background(), azblob.Marker{}, azblob.ListContainersSegmentOptions{Prefix: containerPrefix})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.Response().StatusCode, chk.Equals, 200)
	c.Assert(resp.RequestID(), chk.Not(chk.Equals), "")
	c.Assert(resp.Version(), chk.Not(chk.Equals), "")
	c.Assert(len(resp.ContainerItems) >= 0, chk.Equals, true)
	c.Assert(resp.ServiceEndpoint, chk.NotNil)

	container, name := createNewContainer(c, sa)
	defer delContainer(c, container)

	md := azblob.Metadata{
		"foo": "foovalue",
		"bar": "barvalue",
	}
	_, err = container.SetMetadata(context.Background(), md, azblob.ContainerAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err = sa.ListContainersSegment(context.Background(), azblob.Marker{}, azblob.ListContainersSegmentOptions{Detail: azblob.ListContainersDetail{Metadata: true}, Prefix: name})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.ContainerItems, chk.HasLen, 1)
	c.Assert(resp.ContainerItems[0].Name, chk.NotNil)
	c.Assert(resp.ContainerItems[0].Properties, chk.NotNil)
	c.Assert(resp.ContainerItems[0].Properties.LastModified, chk.NotNil)
	c.Assert(resp.ContainerItems[0].Properties.Etag, chk.NotNil)
	c.Assert(resp.ContainerItems[0].Properties.LeaseStatus, chk.Equals, azblob.LeaseStatusUnlocked)
	c.Assert(resp.ContainerItems[0].Properties.LeaseState, chk.Equals, azblob.LeaseStateAvailable)
	c.Assert(string(resp.ContainerItems[0].Properties.LeaseDuration), chk.Equals, "")
	c.Assert(string(resp.ContainerItems[0].Properties.PublicAccess), chk.Equals, string(azblob.PublicAccessNone))
	c.Assert(resp.ContainerItems[0].Metadata, chk.DeepEquals, md)
}

func (s *StorageAccountSuite) TestListContainersPaged(c *chk.C) {
	sa := getBSU()

	const numContainers = 4
	const maxResultsPerPage = 2
	const pagedContainersPrefix = "azblobspagedtest"

	containers := make([]azblob.ContainerURL, numContainers)
	for i := 0; i < numContainers; i++ {
		containers[i], _ = createNewContainerWithSuffix(c, sa, pagedContainersPrefix)
	}

	defer func() {
		for i := range containers {
			delContainer(c, containers[i])
		}
	}()

	marker := azblob.Marker{}
	iterations := numContainers / maxResultsPerPage

	for i := 0; i < iterations; i++ {
		resp, err := sa.ListContainersSegment(context.Background(), marker, azblob.ListContainersSegmentOptions{MaxResults: maxResultsPerPage, Prefix: containerPrefix + pagedContainersPrefix})
		c.Assert(err, chk.IsNil)
		c.Assert(resp.ContainerItems, chk.HasLen, maxResultsPerPage)

		hasMore := i < iterations-1
		c.Assert(resp.NextMarker.NotDone(), chk.Equals, hasMore)
		marker = resp.NextMarker
	}
}
