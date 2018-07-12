package azblob_test

import (
	"bytes"
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	chk "gopkg.in/check.v1"

	"github.com/Azure/azure-pipeline-go/pipeline"
	"github.com/Azure/azure-storage-blob-go/2016-05-31/azblob"
)

/*func  finalizer(c azblob.ContainerURL) {
	c.Delete(context.Background(), azblob.ContainerAccessConditions{})
}
// Put in Test Main
var defaultContainerURL *azblob.ContainerURL = createDefaultContainerURL()
func  createDefaultContainerURL() *azblob.ContainerURL {
	u, _ := url.Parse("")
	su := azblob.NewServiceURL(*u, azblob.NewPipeline())
	c := &su.NewContainerURL(generateContainerName())
	runtime.SetFinalizer(c, finalizer)
	return c
}

func getDefaultContainerURL() azblob.ContainerURL {
	// Call in all tests. No need to defer. Generally eliminates need for getBSU()
	return *c
}

func createContainer(c *chk.C, su *azblob.ServiceURL) (azblob.ContainerURL, func()){
	// Create container and
	// If service url not nil, use it. If nil, use default
	return containerURL, func(){ containerURL.Delete() }

}*/
// Hookup to the testing framework
func Test(t *testing.T) { chk.TestingT(t) }

type aztestsSuite struct{}

var _ = chk.Suite(&aztestsSuite{})

const (
	containerPrefix          = "go"
	blobPrefix               = "gotestblob"
	blockBlobDefaultData     = "GoBlockBlobData"
	validationErrorSubstring = "validation failed"
)

var ctx = context.Background()
var basicHeaders = azblob.BlobHTTPHeaders{ContentType: "my_type", ContentDisposition: "my_disposition",
	CacheControl: "control", ContentMD5: md5.Sum([]byte("")), ContentLanguage: "my_language", ContentEncoding: "my_encoding"}
var basicMetadata = azblob.Metadata{"foo": "bar"}

type testPipeline struct{}

const testPipelineMessage string = "Test factory invoked"

func (tm testPipeline) Do(ctx context.Context, methodFactory pipeline.Factory, request pipeline.Request) (pipeline.Response, error) {
	return nil, errors.New(testPipelineMessage)
}

// This function generates an entity name by concatenating the passed prefix,
// the name of the test requesting the entity name, and the minute, second, and nanoseconds of the call.
// This should make it easy to associate the entities with their test, uniquely identify
// them, and determine the order in which they were created.
// Note that this imposes a restriction on the length of test names
func generateName(prefix string) string {
	// These next lines up through the for loop are obtaining and walking up the stack
	// trace to extrat the test name, which is stored in name
	pc := make([]uintptr, 10)
	runtime.Callers(0, pc)
	f := runtime.FuncForPC(pc[0])
	name := f.Name()
	for i := 0; !strings.Contains(name, "Suite"); i++ { // The tests are all scoped to the suite, so this ensures getting the actual test name
		f = runtime.FuncForPC(pc[i])
		name = f.Name()
	}
	funcNameStart := strings.Index(name, "Test")
	name = name[funcNameStart+len("Test"):] // Just get the name of the test and not any of the garbage at the beginning
	name = strings.ToLower(name)            // Ensure it is a valid resource name
	currentTime := time.Now()
	name = fmt.Sprintf("%s%s%d%d%d", prefix, strings.ToLower(name), currentTime.Minute(), currentTime.Second(), currentTime.Nanosecond())
	return name
}

func generateContainerName() string {
	return generateName(containerPrefix)
}

func generateBlobName() string {
	return generateName(blobPrefix)
}

func getContainerURL(c *chk.C, bsu azblob.ServiceURL) (container azblob.ContainerURL, name string) {
	name = generateContainerName()
	container = bsu.NewContainerURL(name)

	return container, name
}

func getBlockBlobURL(c *chk.C, container azblob.ContainerURL) (blob azblob.BlockBlobURL, name string) {
	name = generateBlobName()
	blob = container.NewBlockBlobURL(name)

	return blob, name
}

func getAppendBlobURL(c *chk.C, container azblob.ContainerURL) (blob azblob.AppendBlobURL, name string) {
	name = generateBlobName()
	blob = container.NewAppendBlobURL(name)

	return blob, name
}

func getPageBlobURL(c *chk.C, container azblob.ContainerURL) (blob azblob.PageBlobURL, name string) {
	name = generateBlobName()
	blob = container.NewPageBlobURL(name)

	return
}

func createNewContainer(c *chk.C, bsu azblob.ServiceURL) (container azblob.ContainerURL, name string) {
	container, name = getContainerURL(c, bsu)

	cResp, err := container.Create(ctx, nil, azblob.PublicAccessBlob)
	c.Assert(err, chk.IsNil)
	c.Assert(cResp.StatusCode(), chk.Equals, 201)
	return container, name
}

func createNewContainerWithSuffix(c *chk.C, bsu azblob.ServiceURL, suffix string) (container azblob.ContainerURL, name string) {
	// The goal of adding the suffix is to be able to predetermine what order the containers will be in when listed.
	// We still need the container prefix to come first, though, to ensure only containers as a part of this test
	// are listed at all.
	name = generateName(containerPrefix + suffix)
	container = bsu.NewContainerURL(name)

	cResp, err := container.Create(ctx, nil, azblob.PublicAccessBlob)
	c.Assert(err, chk.IsNil)
	c.Assert(cResp.StatusCode(), chk.Equals, 201)
	return container, name
}

func createNewBlockBlob(c *chk.C, container azblob.ContainerURL) (blob azblob.BlockBlobURL, name string) {
	blob, name = getBlockBlobURL(c, container)

	cResp, err := blob.PutBlob(ctx, strings.NewReader(blockBlobDefaultData), azblob.BlobHTTPHeaders{},
		nil, azblob.BlobAccessConditions{})

	c.Assert(err, chk.IsNil)
	c.Assert(cResp.StatusCode(), chk.Equals, 201)

	return
}

func createNewAppendBlob(c *chk.C, container azblob.ContainerURL) (blob azblob.AppendBlobURL, name string) {
	blob, name = getAppendBlobURL(c, container)

	resp, err := blob.Create(ctx, azblob.BlobHTTPHeaders{}, nil, azblob.BlobAccessConditions{})

	c.Assert(err, chk.IsNil)
	c.Assert(resp.StatusCode(), chk.Equals, 201)
	return
}

func createNewPageBlob(c *chk.C, container azblob.ContainerURL) (blob azblob.PageBlobURL, name string) {
	blob, name = getPageBlobURL(c, container)

	resp, err := blob.Create(ctx, azblob.PageBlobPageBytes*10, 0, azblob.BlobHTTPHeaders{}, nil, azblob.BlobAccessConditions{})

	c.Assert(err, chk.IsNil)
	c.Assert(resp.StatusCode(), chk.Equals, 201)
	return
}
func createBlockBlobWithPrefix(c *chk.C, container azblob.ContainerURL, prefix string) (blob azblob.BlockBlobURL, name string) {
	name = prefix + generateName(blobPrefix)
	blob = container.NewBlockBlobURL(name)

	cResp, err := blob.PutBlob(ctx, strings.NewReader(blockBlobDefaultData), azblob.BlobHTTPHeaders{},
		nil, azblob.BlobAccessConditions{})

	c.Assert(err, chk.IsNil)
	c.Assert(cResp.StatusCode(), chk.Equals, 201)
	return
}

func deleteContainer(c *chk.C, container azblob.ContainerURL) {
	resp, err := container.Delete(ctx, azblob.ContainerAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.StatusCode(), chk.Equals, 202)
}

func getBSU() azblob.ServiceURL {
	name := os.Getenv("ACCOUNT_NAME")
	key := os.Getenv("ACCOUNT_KEY")
	if name == "" || key == "" {
		panic("ACCOUNT_NAME and ACCOUNT_KEY environment vars must be set before running tests")
	}
	u, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/", name))

	credential := azblob.NewSharedKeyCredential(name, key)
	pipeline := azblob.NewPipeline(credential, azblob.PipelineOptions{})
	return azblob.NewServiceURL(*u, pipeline)
}

func getAlternateBSU() (azblob.ServiceURL, error) {
	secondaryAccountName, secondaryAccountKey := os.Getenv("SECONDARY_ACCOUNT_NAME"), os.Getenv("SECONDARY_ACCOUNT_KEY")
	if secondaryAccountName == "" || secondaryAccountKey == "" {
		return azblob.ServiceURL{}, errors.New("SECONDARY_ACCOUNT_NAME and/or SECONDARY_ACCOUNT_KEY environment variables not specified.")
	}
	credentials := azblob.NewSharedKeyCredential(secondaryAccountName, secondaryAccountKey)
	pipeline := azblob.NewPipeline(credentials, azblob.PipelineOptions{ /*Log: pipeline.NewLogWrapper(pipeline.LogInfo, log.New(os.Stderr, "", log.LstdFlags))*/ })
	blobPrimaryURL, _ := url.Parse("https://" + secondaryAccountName + ".blob.core.windows.net/")
	return azblob.NewServiceURL(*blobPrimaryURL, pipeline), nil
}

func validateStorageError(c *chk.C, err error, code azblob.ServiceCodeType) {
	serr, _ := err.(azblob.StorageError)
	c.Assert(serr.ServiceCode(), chk.Equals, code)
}

func getRelativeTimeGMT(amount time.Duration) time.Time {
	currentTime := time.Now().In(time.FixedZone("GMT", 0))
	currentTime = currentTime.Add(amount * time.Second)
	return currentTime
}

func (s *aztestsSuite) TestAccountListContainersEmptyPrefix(c *chk.C) {
	bsu := getBSU()
	containerURL1, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL1)
	containerURL2, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL2)

	response, err := bsu.ListContainers(ctx, azblob.Marker{}, azblob.ListContainersOptions{})

	c.Assert(err, chk.IsNil)
	c.Assert(len(response.Containers) >= 2, chk.Equals, true) // The response should contain at least the two created containers. Probably many more
}

func (s *aztestsSuite) TestAccountListContainersIncludeTypeMetadata(c *chk.C) {
	bsu := getBSU()
	containerURLNoMetadata, nameNoMetadata := createNewContainerWithSuffix(c, bsu, "a")
	defer deleteContainer(c, containerURLNoMetadata)
	containerURLMetadata, nameMetadata := createNewContainerWithSuffix(c, bsu, "b")
	defer deleteContainer(c, containerURLMetadata)

	// Test on containers with and without metadata
	_, err := containerURLMetadata.SetMetadata(ctx, basicMetadata, azblob.ContainerAccessConditions{})
	c.Assert(err, chk.IsNil)

	// Also validates not specifying MaxResults
	response, err := bsu.ListContainers(ctx, azblob.Marker{},
		azblob.ListContainersOptions{Prefix: containerPrefix, Detail: azblob.ListContainersDetail{Metadata: true}})
	c.Assert(err, chk.IsNil)
	c.Assert(response.Containers[0].Name, chk.Equals, nameNoMetadata)
	c.Assert(response.Containers[0].Metadata, chk.HasLen, 0)
	c.Assert(response.Containers[1].Name, chk.Equals, nameMetadata)
	c.Assert(response.Containers[1].Metadata, chk.DeepEquals, basicMetadata)
}

func (s *aztestsSuite) TestAccountListContainersMaxResultsNegative(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)

	defer deleteContainer(c, containerURL)
	// The library should panic if MaxResults < -1
	defer func() {
		recover()
	}()

	bsu.ListContainers(ctx,
		azblob.Marker{}, *(&azblob.ListContainersOptions{Prefix: containerPrefix, MaxResults: -2}))

	c.Fail() // If the list call doesn't panic, we fail
}

func (s *aztestsSuite) TestAccountListContainersMaxResultsZero(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)

	defer deleteContainer(c, containerURL)

	// Max Results = 0 means the value will be ignored, the header not set, and the server default used
	resp, err := bsu.ListContainers(ctx,
		azblob.Marker{}, *(&azblob.ListContainersOptions{Prefix: containerPrefix, MaxResults: 0}))

	c.Assert(err, chk.IsNil)
	c.Assert(resp.Containers, chk.HasLen, 1)
}

func (s *aztestsSuite) TestAccountListContainersMaxResultsExact(c *chk.C) {
	// If this test fails, ensure there are no extra containers prefixed with go in the account. These may be left over if a test is interrupted.
	bsu := getBSU()
	containerURL1, containerName1 := createNewContainerWithSuffix(c, bsu, "a")
	defer deleteContainer(c, containerURL1)
	containerURL2, containerName2 := createNewContainerWithSuffix(c, bsu, "b")
	defer deleteContainer(c, containerURL2)

	response, err := bsu.ListContainers(ctx,
		azblob.Marker{}, *(&azblob.ListContainersOptions{Prefix: containerPrefix, MaxResults: 2}))

	c.Assert(err, chk.IsNil)
	c.Assert(response.Containers, chk.HasLen, 2)
	c.Assert(response.Containers[0].Name, chk.Equals, containerName1)
	c.Assert(response.Containers[1].Name, chk.Equals, containerName2)
}

func (s *aztestsSuite) TestAccountListContainersMaxResultsInsufficient(c *chk.C) {
	bsu := getBSU()
	containerURL1, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL1)
	containerURL2, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL2)

	response, err := bsu.ListContainers(ctx, azblob.Marker{},
		*(&azblob.ListContainersOptions{Prefix: containerPrefix, MaxResults: 1}))

	c.Assert(err, chk.IsNil)
	c.Assert(len(response.Containers), chk.Equals, 1)
}

func (s *aztestsSuite) TestAccountListContainersMaxResultsSufficient(c *chk.C) {
	bsu := getBSU()
	containerURL1, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL1)
	containerURL2, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL2)

	response, err := bsu.ListContainers(ctx, azblob.Marker{},
		*(&azblob.ListContainersOptions{Prefix: containerPrefix, MaxResults: 3}))

	c.Assert(err, chk.IsNil)
	c.Assert(len(response.Containers), chk.Equals, 2)
}

func (s *aztestsSuite) TestNewContainerURLValidName(c *chk.C) {
	bsu := getBSU()
	testURL := bsu.NewContainerURL(containerPrefix)

	correctURL := "https://" + os.Getenv("ACCOUNT_NAME") + ".blob.core.windows.net/" + containerPrefix
	temp := testURL.URL()
	c.Assert(temp.String(), chk.Equals, correctURL)
}

func (s *aztestsSuite) TestCreateRootContainerURL(c *chk.C) {
	bsu := getBSU()
	testURL := bsu.NewRootContainerURL()

	correctURL := "https://" + os.Getenv("ACCOUNT_NAME") + ".blob.core.windows.net/$root"
	temp := testURL.URL()
	c.Assert(temp.String(), chk.Equals, correctURL)
}

func (s *aztestsSuite) TestCreateBlobURL(c *chk.C) {
	bsu := getBSU()
	containerURL, containerName := getContainerURL(c, bsu)
	testURL, testName := getBlockBlobURL(c, containerURL)

	parts := azblob.NewBlobURLParts(testURL.URL())
	c.Assert(parts.BlobName, chk.Equals, testName)
	c.Assert(parts.ContainerName, chk.Equals, containerName)

	correctURL := "https://" + os.Getenv("ACCOUNT_NAME") + ".blob.core.windows.net/" + containerName + "/" + testName
	temp := testURL.URL()
	c.Assert(temp.String(), chk.Equals, correctURL)
}

func (s *aztestsSuite) TestCreateBlobURLWithSnapshotAndSAS(c *chk.C) {
	bsu := getBSU()
	containerURL, containerName := getContainerURL(c, bsu)
	blobURL, blobName := getBlockBlobURL(c, containerURL)

	currentTime := time.Now().UTC()
	credential := azblob.NewSharedKeyCredential(os.Getenv("ACCOUNT_NAME"), os.Getenv("ACCOUNT_KEY"))
	sasQueryParams := azblob.AccountSASSignatureValues{
		Protocol:      azblob.SASProtocolHTTPS,
		ExpiryTime:    currentTime.Add(48 * time.Hour),
		Permissions:   azblob.AccountSASPermissions{Read: true, List: true}.String(),
		Services:      azblob.AccountSASServices{Blob: true}.String(),
		ResourceTypes: azblob.AccountSASResourceTypes{Container: true, Object: true}.String(),
	}.NewSASQueryParameters(credential)

	parts := azblob.NewBlobURLParts(blobURL.URL())
	parts.SAS = sasQueryParams
	parts.Snapshot = currentTime
	testURL := parts.URL()

	// The snapshot format string is taken from the snapshotTimeFormat value in parsing_urls.go. The field is not public, so
	// it is copied here
	correctURL := "https://" + os.Getenv("ACCOUNT_NAME") + ".blob.core.windows.net/" + containerName + "/" + blobName +
		"?" + "snapshot=" + currentTime.Format("2006-01-02T15:04:05.0000000Z07:00") + "&" + sasQueryParams.Encode()
	c.Assert(testURL.String(), chk.Equals, correctURL)
}

func (s *aztestsSuite) TestAccountWithPipeline(c *chk.C) {
	bsu := getBSU()
	bsu = bsu.WithPipeline(testPipeline{}) // testPipeline returns an identifying message as an error
	containerURL := bsu.NewContainerURL("name")

	_, err := containerURL.Create(ctx, azblob.Metadata{}, azblob.PublicAccessBlob)

	c.Assert(err.Error(), chk.Equals, testPipelineMessage)
}

func (s *aztestsSuite) TestContainerCreateInvalidName(c *chk.C) {
	bsu := getBSU()
	containerURL := bsu.NewContainerURL("foo bar")

	_, err := containerURL.Create(ctx, azblob.Metadata{}, azblob.PublicAccessBlob)

	validateStorageError(c, err, azblob.ServiceCodeInvalidResourceName)
}

func (s *aztestsSuite) TestContainerCreateEmptyName(c *chk.C) {
	bsu := getBSU()
	containerURL := bsu.NewContainerURL("")

	_, err := containerURL.Create(ctx, azblob.Metadata{}, azblob.PublicAccessBlob)

	validateStorageError(c, err, azblob.ServiceCodeInvalidQueryParameterValue)
}

func (s *aztestsSuite) TestContainerCreateNameCollision(c *chk.C) {
	bsu := getBSU()
	containerURL, containerName := createNewContainer(c, bsu)

	defer deleteContainer(c, containerURL)

	containerURL = bsu.NewContainerURL(containerName)
	_, err := containerURL.Create(ctx, azblob.Metadata{}, azblob.PublicAccessBlob)

	validateStorageError(c, err, azblob.ServiceCodeContainerAlreadyExists)
}

func (s *aztestsSuite) TestContainerCreateInvalidMetadata(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := getContainerURL(c, bsu)

	_, err := containerURL.Create(ctx, azblob.Metadata{"1 foo": "bar"}, azblob.PublicAccessBlob)

	c.Assert(err, chk.NotNil)
	c.Assert(strings.Contains(err.Error(), validationErrorSubstring), chk.Equals, true)
}

func (s *aztestsSuite) TestContainerCreateNilMetadata(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := getContainerURL(c, bsu)

	_, err := containerURL.Create(ctx, nil, azblob.PublicAccessBlob)
	defer deleteContainer(c, containerURL)
	c.Assert(err, chk.IsNil)

	response, err := containerURL.GetPropertiesAndMetadata(ctx, azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(response.NewMetadata(), chk.HasLen, 0)
}

func (s *aztestsSuite) TestContainerCreateEmptyMetadata(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := getContainerURL(c, bsu)

	_, err := containerURL.Create(ctx, azblob.Metadata{}, azblob.PublicAccessBlob)
	defer deleteContainer(c, containerURL)
	c.Assert(err, chk.IsNil)

	response, err := containerURL.GetPropertiesAndMetadata(ctx, azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(response.NewMetadata(), chk.HasLen, 0)
}

// Note that for all tests that create blobs, deleting the container also deletes any blobs within that container, thus we
// simply delete the whole container after the test

func (s *aztestsSuite) TestContainerCreateAccessContainer(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := getContainerURL(c, bsu)

	_, err := containerURL.Create(ctx, nil, azblob.PublicAccessContainer)
	defer deleteContainer(c, containerURL)
	c.Assert(err, chk.IsNil)

	blobURL := containerURL.NewBlockBlobURL(blobPrefix)
	blobURL.PutBlob(ctx, bytes.NewReader([]byte("Content")), azblob.BlobHTTPHeaders{},
		basicMetadata, azblob.BlobAccessConditions{})

	// Anonymous enumeration should be valid with container access
	containerURL2 := azblob.NewContainerURL(containerURL.URL(), azblob.NewPipeline(azblob.NewAnonymousCredential(), azblob.PipelineOptions{}))
	response, err := containerURL2.ListBlobs(ctx, azblob.Marker{}, azblob.ListBlobsOptions{})
	c.Assert(err, chk.IsNil)
	c.Assert(response.Blobs.Blob[0].Name, chk.Equals, blobPrefix)

	// Getting blob data anonymously should still be valid with container access
	blobURL2 := containerURL2.NewBlockBlobURL(blobPrefix)
	resp, err := blobURL2.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.NewMetadata(), chk.DeepEquals, basicMetadata)
}

func (s *aztestsSuite) TestContainerCreateAccessBlob(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := getContainerURL(c, bsu)

	_, err := containerURL.Create(ctx, nil, azblob.PublicAccessBlob)
	defer deleteContainer(c, containerURL)
	c.Assert(err, chk.IsNil)

	blobURL := containerURL.NewBlockBlobURL(blobPrefix)
	blobURL.PutBlob(ctx, bytes.NewReader([]byte("Content")), azblob.BlobHTTPHeaders{},
		basicMetadata, azblob.BlobAccessConditions{})

	// Reference the same container URL but with anonymous credentials
	containerURL2 := azblob.NewContainerURL(containerURL.URL(), azblob.NewPipeline(azblob.NewAnonymousCredential(), azblob.PipelineOptions{}))
	_, err = containerURL2.ListBlobs(ctx, azblob.Marker{}, azblob.ListBlobsOptions{})
	validateStorageError(c, err, azblob.ServiceCodeResourceNotFound) // Listing blobs is not publicly accessible

	// Accessing blob specific data should be public
	blobURL2 := containerURL2.NewBlockBlobURL(blobPrefix)
	resp, err := blobURL2.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.NewMetadata(), chk.DeepEquals, basicMetadata)
}

func (s *aztestsSuite) TestContainerCreateAccessNone(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := getContainerURL(c, bsu)

	_, err := containerURL.Create(ctx, nil, azblob.PublicAccessNone)
	defer deleteContainer(c, containerURL)

	blobURL := containerURL.NewBlockBlobURL(blobPrefix)
	blobURL.PutBlob(ctx, bytes.NewReader([]byte("Content")), azblob.BlobHTTPHeaders{},
		basicMetadata, azblob.BlobAccessConditions{})

	// Reference the same container URL but with anonymous credentials
	containerURL2 := azblob.NewContainerURL(containerURL.URL(), azblob.NewPipeline(azblob.NewAnonymousCredential(), azblob.PipelineOptions{}))
	// Listing blobs is not public
	_, err = containerURL2.ListBlobs(ctx, azblob.Marker{}, azblob.ListBlobsOptions{})
	validateStorageError(c, err, azblob.ServiceCodeResourceNotFound)

	// Blob data is not public
	blobURL2 := containerURL2.NewBlockBlobURL(blobPrefix)
	_, err = blobURL2.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.NotNil)
	serr := err.(azblob.StorageError)
	c.Assert(serr.Response().StatusCode, chk.Equals, 404) // HEAD request does not return a status code
}

func validateContainerDeleted(c *chk.C, containerURL azblob.ContainerURL) {
	_, err := containerURL.GetPermissions(ctx, azblob.LeaseAccessConditions{})
	validateStorageError(c, err, azblob.ServiceCodeContainerNotFound)
}

func (s *aztestsSuite) TestContainerDelete(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)

	_, err := containerURL.Delete(ctx, azblob.ContainerAccessConditions{})
	c.Assert(err, chk.IsNil)

	validateContainerDeleted(c, containerURL)
}

func (s *aztestsSuite) TestContainerDeleteNonExistant(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := getContainerURL(c, bsu)

	_, err := containerURL.Delete(ctx, azblob.ContainerAccessConditions{})
	validateStorageError(c, err, azblob.ServiceCodeContainerNotFound)
}

func (s *aztestsSuite) TestContainerDeleteIfModifiedSinceTrue(c *chk.C) {
	currentTime := getRelativeTimeGMT(-10) // Ensure the requests occur at different times
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)

	defer deleteContainer(c, containerURL)

	_, err := containerURL.Delete(ctx,
		azblob.ContainerAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)
	validateContainerDeleted(c, containerURL)
}

func (s *aztestsSuite) TestContainerDeleteIfModifiedSinceFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)

	defer deleteContainer(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	_, err := containerURL.Delete(ctx,
		azblob.ContainerAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestContainerDeleteIfUnModifiedSinceTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)

	defer deleteContainer(c, containerURL)

	currentTime := getRelativeTimeGMT(10)
	_, err := containerURL.Delete(ctx,
		azblob.ContainerAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	validateContainerDeleted(c, containerURL)
}

func (s *aztestsSuite) TestContainerDeleteIfUnModifiedSinceFalse(c *chk.C) {
	currentTime := getRelativeTimeGMT(-10) // Ensure the requests occur at different times

	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)

	defer deleteContainer(c, containerURL)

	_, err := containerURL.Delete(ctx,
		azblob.ContainerAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestContainerAccessConditionsUnsupportedConditions(c *chk.C) {
	// This test defines that the library will not stop you from adding conditional headers
	// that will be ignored by the service
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)

	defer deleteContainer(c, containerURL)
	defer func() { // The library should panic if it sees If(None)Match ETAG access condition
		recover()
	}()
	invalidEtag := azblob.ETag("invalid")
	containerURL.SetMetadata(ctx, basicMetadata,
		azblob.ContainerAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: invalidEtag}})

	c.Fail() // If the method panics (as expected), then we shouldn't get here
	/*
		c.Assert(err, chk.IsNil)

		resp, err := containerURL.GetPropertiesAndMetadata(ctx, azblob.LeaseAccessConditions{})
		c.Assert(err, chk.IsNil)
		c.Assert(resp.NewMetadata(), chk.DeepEquals, basicMetadata)
	*/
}

func (s *aztestsSuite) TestContainerListBlobsNonexistantPrefix(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	createNewBlockBlob(c, containerURL)

	resp, err := containerURL.ListBlobs(ctx, azblob.Marker{}, azblob.ListBlobsOptions{Prefix: blobPrefix + blobPrefix})

	c.Assert(err, chk.IsNil)
	c.Assert(resp.Blobs.Blob, chk.HasLen, 0)
}

func (s *aztestsSuite) TestContainerListBlobsSpecificValidPrefix(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	_, blobName := createNewBlockBlob(c, containerURL)

	resp, err := containerURL.ListBlobs(ctx, azblob.Marker{}, azblob.ListBlobsOptions{Prefix: blobPrefix})

	c.Assert(err, chk.IsNil)
	c.Assert(resp.Blobs.Blob, chk.HasLen, 1)
	c.Assert(resp.Blobs.Blob[0].Name, chk.Equals, blobName)
}

func (s *aztestsSuite) TestContainerListBlobsValidDelimiter(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	createBlockBlobWithPrefix(c, containerURL, "a/1")
	createBlockBlobWithPrefix(c, containerURL, "a/2")
	createBlockBlobWithPrefix(c, containerURL, "b/1")
	_, blobName := createBlockBlobWithPrefix(c, containerURL, "blob")

	resp, err := containerURL.ListBlobs(ctx, azblob.Marker{}, azblob.ListBlobsOptions{Delimiter: "/"})

	c.Assert(err, chk.IsNil)
	c.Assert(len(resp.Blobs.Blob), chk.Equals, 1)
	c.Assert(len(resp.Blobs.BlobPrefix), chk.Equals, 2)
	c.Assert(resp.Blobs.BlobPrefix[0].Name, chk.Equals, "a/")
	c.Assert(resp.Blobs.BlobPrefix[1].Name, chk.Equals, "b/")
	c.Assert(resp.Blobs.Blob[0].Name, chk.Equals, blobName)
}

func (s *aztestsSuite) TestContainerListBlobsInvalidDelimiter(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	createBlockBlobWithPrefix(c, containerURL, "a/1")
	createBlockBlobWithPrefix(c, containerURL, "a/2")
	createBlockBlobWithPrefix(c, containerURL, "b/1")
	createBlockBlobWithPrefix(c, containerURL, "blob")

	resp, err := containerURL.ListBlobs(ctx, azblob.Marker{}, azblob.ListBlobsOptions{Delimiter: "^"})

	c.Assert(err, chk.IsNil)
	c.Assert(resp.Blobs.Blob, chk.HasLen, 4)
}

func (s *aztestsSuite) TestContainerListBlobsIncludeTypeMetadata(c *chk.C) {
	bsu := getBSU()
	container, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, container)
	_, blobNameNoMetadata := createBlockBlobWithPrefix(c, container, "a")
	blobMetadata, blobNameMetadata := createBlockBlobWithPrefix(c, container, "b")
	_, err := blobMetadata.SetMetadata(ctx, azblob.Metadata{"field": "value"}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := container.ListBlobs(ctx, azblob.Marker{}, azblob.ListBlobsOptions{Details: azblob.BlobListingDetails{Metadata: true}})

	c.Assert(err, chk.IsNil)
	c.Assert(resp.Blobs.Blob[0].Name, chk.Equals, blobNameNoMetadata)
	c.Assert(resp.Blobs.Blob[0].Metadata, chk.HasLen, 0)
	c.Assert(resp.Blobs.Blob[1].Name, chk.Equals, blobNameMetadata)
	c.Assert(resp.Blobs.Blob[1].Metadata["field"], chk.Equals, "value")
}

func (s *aztestsSuite) TestContainerListBlobsIncludeTypeSnapshots(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blob, blobName := createNewBlockBlob(c, containerURL)
	_, err := blob.CreateSnapshot(ctx, azblob.Metadata{}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := containerURL.ListBlobs(ctx, azblob.Marker{},
		azblob.ListBlobsOptions{Details: azblob.BlobListingDetails{Snapshots: true}})

	c.Assert(err, chk.IsNil)
	c.Assert(resp.Blobs.Blob, chk.HasLen, 2)
	c.Assert(resp.Blobs.Blob[0].Name, chk.Equals, blobName)
	c.Assert(resp.Blobs.Blob[0].Snapshot, chk.NotNil)
	c.Assert(resp.Blobs.Blob[1].Name, chk.Equals, blobName)
	c.Assert(resp.Blobs.Blob[1].Snapshot.IsZero(), chk.Equals, true)
}

func (s *aztestsSuite) TestContainerListBlobsIncludeTypeCopy(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, blobName := createNewBlockBlob(c, containerURL)
	blobCopyURL, blobCopyName := createBlockBlobWithPrefix(c, containerURL, "copy")
	_, err := blobCopyURL.StartCopy(ctx, blobURL.URL(), azblob.Metadata{}, azblob.BlobAccessConditions{}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := containerURL.ListBlobs(ctx, azblob.Marker{},
		azblob.ListBlobsOptions{Details: azblob.BlobListingDetails{Copy: true}})

	// These are sufficient to show that the blob copy was in fact included
	c.Assert(err, chk.IsNil)
	c.Assert(resp.Blobs.Blob, chk.HasLen, 2)
	c.Assert(resp.Blobs.Blob[1].Name, chk.Equals, blobName)
	c.Assert(resp.Blobs.Blob[0].Name, chk.Equals, blobCopyName)
	c.Assert(*resp.Blobs.Blob[0].Properties.ContentLength, chk.Equals, int64(len(blockBlobDefaultData)))
	temp := blobURL.URL()
	c.Assert(*resp.Blobs.Blob[0].Properties.CopySource, chk.Equals, temp.String())
	c.Assert(resp.Blobs.Blob[0].Properties.CopyStatus, chk.Equals, azblob.CopyStatusSuccess)
}

func (s *aztestsSuite) TestContainerListBlobsIncludeTypeUncommitted(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, blobName := getBlockBlobURL(c, containerURL)
	_, err := blobURL.PutBlock(ctx, "0000", strings.NewReader(blockBlobDefaultData), azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := containerURL.ListBlobs(ctx, azblob.Marker{},
		azblob.ListBlobsOptions{Details: azblob.BlobListingDetails{UncommittedBlobs: true}})

	c.Assert(err, chk.IsNil)
	c.Assert(resp.Blobs.Blob, chk.HasLen, 1)
	c.Assert(resp.Blobs.Blob[0].Name, chk.Equals, blobName)
}

func (s *aztestsSuite) TestContainerListBlobsIncludeMultiple(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, blobName := createNewBlockBlob(c, containerURL)
	_, err := blobURL.CreateSnapshot(ctx, azblob.Metadata{}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	blobURL2, blobName2 := createBlockBlobWithPrefix(c, containerURL, "copy")
	_, err = blobURL2.StartCopy(ctx, blobURL.URL(), azblob.Metadata{}, azblob.BlobAccessConditions{}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := containerURL.ListBlobs(ctx, azblob.Marker{},
		azblob.ListBlobsOptions{Details: azblob.BlobListingDetails{Snapshots: true, Copy: true}})

	c.Assert(err, chk.IsNil)
	c.Assert(resp.Blobs.Blob, chk.HasLen, 3)
	c.Assert(resp.Blobs.Blob[0].Name, chk.Equals, blobName2)
	c.Assert(resp.Blobs.Blob[1].Name, chk.Equals, blobName)
	c.Assert(resp.Blobs.Blob[1].Snapshot, chk.NotNil)
	c.Assert(resp.Blobs.Blob[2].Name, chk.Equals, blobName)
}

func (s *aztestsSuite) TestContainerListBlobsMaxResultsNegative(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)

	defer deleteContainer(c, containerURL)

	// If ListBlobs panics, as it should, this function will be called and recover from the panic, allowing the test to pass
	defer func() {
		recover()
	}()
	containerURL.ListBlobs(ctx, azblob.Marker{}, azblob.ListBlobsOptions{MaxResults: -2})

	// We will only reach this if we did not panic
	c.Fail()
}

func (s *aztestsSuite) TestContainerListBlobsMaxResultsZero(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	createNewBlockBlob(c, containerURL)

	resp, err := containerURL.ListBlobs(ctx, azblob.Marker{}, azblob.ListBlobsOptions{MaxResults: 0})

	c.Assert(err, chk.IsNil)
	c.Assert(resp.Blobs.Blob, chk.HasLen, 1)
}

func (s *aztestsSuite) TestContainerListBlobsMaxResultsInsufficient(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	_, blobName := createBlockBlobWithPrefix(c, containerURL, "a")
	createBlockBlobWithPrefix(c, containerURL, "b")

	resp, err := containerURL.ListBlobs(ctx, azblob.Marker{}, azblob.ListBlobsOptions{MaxResults: 1})

	c.Assert(err, chk.IsNil)
	c.Assert(resp.Blobs.Blob, chk.HasLen, 1)
	c.Assert(resp.Blobs.Blob[0].Name, chk.Equals, blobName)
}

func (s *aztestsSuite) TestContainerListBlobsMaxResultsExact(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	_, blobName := createBlockBlobWithPrefix(c, containerURL, "a")
	_, blobName2 := createBlockBlobWithPrefix(c, containerURL, "b")

	resp, err := containerURL.ListBlobs(ctx, azblob.Marker{}, azblob.ListBlobsOptions{MaxResults: 2})

	c.Assert(err, chk.IsNil)
	c.Assert(resp.Blobs.Blob, chk.HasLen, 2)
	c.Assert(resp.Blobs.Blob[0].Name, chk.Equals, blobName)
	c.Assert(resp.Blobs.Blob[1].Name, chk.Equals, blobName2)
}

func (s *aztestsSuite) TestContainerListBlobsMaxResultsSufficient(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	_, blobName := createBlockBlobWithPrefix(c, containerURL, "a")
	_, blobName2 := createBlockBlobWithPrefix(c, containerURL, "b")

	resp, err := containerURL.ListBlobs(ctx, azblob.Marker{}, azblob.ListBlobsOptions{MaxResults: 3})

	c.Assert(err, chk.IsNil)
	c.Assert(resp.Blobs.Blob, chk.HasLen, 2)
	c.Assert(resp.Blobs.Blob[0].Name, chk.Equals, blobName)
	c.Assert(resp.Blobs.Blob[1].Name, chk.Equals, blobName2)
}

func (s *aztestsSuite) TestContainerListBlobsNonExistentContainer(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := getContainerURL(c, bsu)

	_, err := containerURL.ListBlobs(ctx, azblob.Marker{}, azblob.ListBlobsOptions{})

	c.Assert(err, chk.NotNil)
}

func (s *aztestsSuite) TestContainerWithNewPipeline(c *chk.C) {
	bsu := getBSU()
	pipeline := testPipeline{}
	containerURL, _ := getContainerURL(c, bsu)
	containerURL = containerURL.WithPipeline(pipeline)

	_, err := containerURL.Create(ctx, azblob.Metadata{}, azblob.PublicAccessBlob)

	c.Assert(err, chk.NotNil)
	c.Assert(err.Error(), chk.Equals, testPipelineMessage)
}

func (s *aztestsSuite) TestContainerGetSetPermissionsMultiplePolicies(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)

	defer deleteContainer(c, containerURL)

	// Define the policies
	start := time.Now().UTC()
	expiry := start.Add(5 * time.Minute).UTC()
	expiry2 := start.Add(time.Minute).UTC()
	permissions := []azblob.SignedIdentifier{
		{ID: "0000",
			AccessPolicy: azblob.AccessPolicy{
				Start:      start,
				Expiry:     expiry,
				Permission: azblob.AccessPolicyPermission{Read: true, Write: true}.String(),
			},
		},
		{ID: "0001",
			AccessPolicy: azblob.AccessPolicy{
				Start:      start,
				Expiry:     expiry2,
				Permission: azblob.AccessPolicyPermission{Read: true}.String(),
			},
		},
	}

	_, err := containerURL.SetPermissions(ctx, azblob.PublicAccessNone, permissions,
		azblob.ContainerAccessConditions{})

	c.Assert(err, chk.IsNil)

	resp, err := containerURL.GetPermissions(ctx, azblob.LeaseAccessConditions{})

	c.Assert(err, chk.IsNil)
	c.Assert(resp.Value, chk.DeepEquals, permissions)
}

func (s *aztestsSuite) TestContainerGetPermissionsPublicAccessNotNone(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := getContainerURL(c, bsu)
	containerURL.Create(ctx, nil, azblob.PublicAccessBlob) // We create the container explicitly so we can be sure the access policy is not empty

	defer deleteContainer(c, containerURL)

	resp, err := containerURL.GetPermissions(ctx, azblob.LeaseAccessConditions{})

	c.Assert(err, chk.IsNil)
	c.Assert(resp.BlobPublicAccess(), chk.Equals, azblob.PublicAccessBlob)
}

func (s *aztestsSuite) TestContainerSetPermissionsPublicAccessNone(c *chk.C) {
	// Test the basic one by making an anonymous request to ensure it's actually doing it and also with GetPermissions
	// For all the others, can just use GetPermissions since we've validated that it at least registers on the server correctly
	bsu := getBSU()
	containerURL, containerName := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	_, blobName := createNewBlockBlob(c, containerURL)

	// Container is created with PublicAccessBlob, so setting it to None will actually test that it is changed through this method
	_, err := containerURL.SetPermissions(ctx, azblob.PublicAccessNone, nil, azblob.ContainerAccessConditions{})
	c.Assert(err, chk.IsNil)

	pipeline := azblob.NewPipeline(azblob.NewAnonymousCredential(), azblob.PipelineOptions{})
	bsu2 := azblob.NewServiceURL(bsu.URL(), pipeline)
	containerURL2 := bsu2.NewContainerURL(containerName)
	blobURL2 := containerURL2.NewBlockBlobURL(blobName)
	_, err = blobURL2.GetBlob(ctx, azblob.BlobRange{}, azblob.BlobAccessConditions{}, false)

	// Get permissions via the original container URL so the request succeeds
	resp, _ := containerURL.GetPermissions(ctx, azblob.LeaseAccessConditions{})

	// If we cannot access a blob's data, we will also not be able to enumerate blobs
	validateStorageError(c, err, azblob.ServiceCodeResourceNotFound)
	c.Assert(resp.BlobPublicAccess(), chk.Equals, azblob.PublicAccessNone)
}

func (s *aztestsSuite) TestContainerSetPermissionsPublicAccessBlob(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)

	defer deleteContainer(c, containerURL)

	_, err := containerURL.SetPermissions(ctx, azblob.PublicAccessBlob, nil, azblob.ContainerAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := containerURL.GetPermissions(ctx, azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.BlobPublicAccess(), chk.Equals, azblob.PublicAccessBlob)
}

func (s *aztestsSuite) TestContainerSetPermissionsPublicAccessContainer(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)

	defer deleteContainer(c, containerURL)

	_, err := containerURL.SetPermissions(ctx, azblob.PublicAccessContainer, nil, azblob.ContainerAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := containerURL.GetPermissions(ctx, azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.BlobPublicAccess(), chk.Equals, azblob.PublicAccessContainer)
}

func (s *aztestsSuite) TestContainerSetPermissionsACLSinglePolicy(c *chk.C) {
	bsu := getBSU()
	credentials := azblob.NewSharedKeyCredential(os.Getenv("ACCOUNT_NAME"), os.Getenv("ACCOUNT_KEY"))
	containerURL, containerName := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	_, blobName := createNewBlockBlob(c, containerURL)

	start := time.Now().UTC().Add(-15 * time.Second)
	expiry := start.Add(5 * time.Minute).UTC()
	permissions := []azblob.SignedIdentifier{{
		ID: "0000",
		AccessPolicy: azblob.AccessPolicy{
			Start:      start,
			Expiry:     expiry,
			Permission: azblob.AccessPolicyPermission{List: true}.String(),
		},
	}}
	_, err := containerURL.SetPermissions(ctx, azblob.PublicAccessNone, permissions, azblob.ContainerAccessConditions{})
	c.Assert(err, chk.IsNil)

	serviceSASValues := azblob.BlobSASSignatureValues{Version: "2015-04-05",
		Identifier: "0000", ContainerName: containerName}
	queryParams := serviceSASValues.NewSASQueryParameters(credentials)
	sasURL := bsu.URL()
	sasURL.RawQuery = queryParams.Encode()
	sasPipeline := azblob.NewPipeline(azblob.NewAnonymousCredential(), azblob.PipelineOptions{})
	sasBlobServiceURL := azblob.NewServiceURL(sasURL, sasPipeline)

	// Verifies that the SAS can access the resource
	sasContainer := sasBlobServiceURL.NewContainerURL(containerName)
	resp, err := sasContainer.ListBlobs(ctx, azblob.Marker{}, azblob.ListBlobsOptions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.Blobs.Blob[0].Name, chk.Equals, blobName)

	// Verifies that successful sas access is not just because it's public
	anonymousBlobService := azblob.NewServiceURL(bsu.URL(), sasPipeline)
	anonymousContainer := anonymousBlobService.NewContainerURL(containerName)
	_, err = anonymousContainer.ListBlobs(ctx, azblob.Marker{}, azblob.ListBlobsOptions{})
	validateStorageError(c, err, azblob.ServiceCodeResourceNotFound)
}

func (s *aztestsSuite) TestContainerSetPermissionsACLMoreThanFive(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)

	defer deleteContainer(c, containerURL)

	start := time.Now().UTC()
	expiry := start.Add(5 * time.Minute).UTC()
	permissions := make([]azblob.SignedIdentifier, 6, 6)
	for i := 0; i < 6; i++ {
		permissions[i] = azblob.SignedIdentifier{
			ID: "000" + strconv.Itoa(i),
			AccessPolicy: azblob.AccessPolicy{
				Start:      start,
				Expiry:     expiry,
				Permission: azblob.AccessPolicyPermission{List: true}.String(),
			},
		}
	}

	_, err := containerURL.SetPermissions(ctx, azblob.PublicAccessBlob, permissions, azblob.ContainerAccessConditions{})
	validateStorageError(c, err, azblob.ServiceCodeInvalidXMLDocument)
}

func (s *aztestsSuite) TestContainerSetPermissionsDeleteAndModifyACL(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)

	defer deleteContainer(c, containerURL)

	start := time.Now().UTC()
	expiry := start.Add(5 * time.Minute).UTC()
	permissions := make([]azblob.SignedIdentifier, 2, 2)
	for i := 0; i < 2; i++ {
		permissions[i] = azblob.SignedIdentifier{
			ID: "000" + strconv.Itoa(i),
			AccessPolicy: azblob.AccessPolicy{
				Start:      start,
				Expiry:     expiry,
				Permission: azblob.AccessPolicyPermission{List: true}.String(),
			},
		}
	}

	_, err := containerURL.SetPermissions(ctx, azblob.PublicAccessBlob, permissions, azblob.ContainerAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := containerURL.GetPermissions(ctx, azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.Value, chk.DeepEquals, permissions)

	permissions = resp.Value[:1] // Delete the first policy by removing it from the slice
	permissions[0].ID = "0004"   // Modify the remaining policy which is at index 0 in the new slice
	_, err = containerURL.SetPermissions(ctx, azblob.PublicAccessBlob, permissions, azblob.ContainerAccessConditions{})

	resp, err = containerURL.GetPermissions(ctx, azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.Value, chk.HasLen, 1)
	c.Assert(resp.Value, chk.DeepEquals, permissions)
}

func (s *aztestsSuite) TestContainerSetPermissionsDeleteAllPolicies(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)

	defer deleteContainer(c, containerURL)

	start := time.Now().UTC()
	expiry := start.Add(5 * time.Minute).UTC()
	permissions := make([]azblob.SignedIdentifier, 2, 2)
	for i := 0; i < 2; i++ {
		permissions[i] = azblob.SignedIdentifier{
			ID: "000" + strconv.Itoa(i),
			AccessPolicy: azblob.AccessPolicy{
				Start:      start,
				Expiry:     expiry,
				Permission: azblob.AccessPolicyPermission{List: true}.String(),
			},
		}
	}

	_, err := containerURL.SetPermissions(ctx, azblob.PublicAccessBlob, permissions, azblob.ContainerAccessConditions{})
	c.Assert(err, chk.IsNil)

	_, err = containerURL.SetPermissions(ctx, azblob.PublicAccessBlob, []azblob.SignedIdentifier{}, azblob.ContainerAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := containerURL.GetPermissions(ctx, azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.Value, chk.HasLen, 0)
}

func (s *aztestsSuite) TestContainerSetPermissionsInvalidPolicyTimes(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)

	defer deleteContainer(c, containerURL)

	// Swap start and expiry
	expiry := time.Now().UTC()
	start := expiry.Add(5 * time.Minute).UTC()
	permissions := make([]azblob.SignedIdentifier, 2, 2)
	for i := 0; i < 2; i++ {
		permissions[i] = azblob.SignedIdentifier{
			ID: "000" + strconv.Itoa(i),
			AccessPolicy: azblob.AccessPolicy{
				Start:      start,
				Expiry:     expiry,
				Permission: azblob.AccessPolicyPermission{List: true}.String(),
			},
		}
	}

	_, err := containerURL.SetPermissions(ctx, azblob.PublicAccessBlob, permissions, azblob.ContainerAccessConditions{})
	c.Assert(err, chk.IsNil)
}

func (s *aztestsSuite) TestContainerSetPermissionsNilPolicySlice(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)

	defer deleteContainer(c, containerURL)

	_, err := containerURL.SetPermissions(ctx, azblob.PublicAccessBlob, nil, azblob.ContainerAccessConditions{})
	c.Assert(err, chk.IsNil)
}

func (s *aztestsSuite) TestContainerSetPermissionsSignedIdentifierTooLong(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)

	defer deleteContainer(c, containerURL)

	id := ""
	for i := 0; i < 65; i++ {
		id += "a"
	}
	expiry := time.Now().UTC()
	start := expiry.Add(5 * time.Minute).UTC()
	permissions := make([]azblob.SignedIdentifier, 2, 2)
	for i := 0; i < 2; i++ {
		permissions[i] = azblob.SignedIdentifier{
			ID: id,
			AccessPolicy: azblob.AccessPolicy{
				Start:      start,
				Expiry:     expiry,
				Permission: azblob.AccessPolicyPermission{List: true}.String(),
			},
		}
	}

	_, err := containerURL.SetPermissions(ctx, azblob.PublicAccessBlob, permissions, azblob.ContainerAccessConditions{})
	validateStorageError(c, err, azblob.ServiceCodeInvalidXMLDocument)
}

func (s *aztestsSuite) TestContainerSetPermissionsIfModifiedSinceTrue(c *chk.C) {
	currentTime := getRelativeTimeGMT(-10)
	bsu := getBSU()
	container, _ := createNewContainer(c, bsu)

	defer deleteContainer(c, container)

	_, err := container.SetPermissions(ctx, azblob.PublicAccessNone, nil,
		azblob.ContainerAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	resp, err := container.GetPermissions(ctx, azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.BlobPublicAccess(), chk.Equals, azblob.PublicAccessNone)
}

func (s *aztestsSuite) TestContainerSetPermissionsIfModifiedSinceFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)

	defer deleteContainer(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	_, err := containerURL.SetPermissions(ctx, azblob.PublicAccessNone, nil,
		azblob.ContainerAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestContainerSetPermissionsIfUnModifiedSinceTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)

	defer deleteContainer(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	_, err := containerURL.SetPermissions(ctx, azblob.PublicAccessNone, nil,
		azblob.ContainerAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	resp, err := containerURL.GetPermissions(ctx, azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.BlobPublicAccess(), chk.Equals, azblob.PublicAccessNone)
}

func (s *aztestsSuite) TestContainerSetPermissionsIfUnModifiedSinceFalse(c *chk.C) {
	currentTime := getRelativeTimeGMT(-10)

	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)

	defer deleteContainer(c, containerURL)

	_, err := containerURL.SetPermissions(ctx, azblob.PublicAccessNone, nil,
		azblob.ContainerAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestContainerGetPropertiesAndMetadataNoMetadata(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)

	defer deleteContainer(c, containerURL)

	resp, err := containerURL.GetPropertiesAndMetadata(ctx, azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.NewMetadata(), chk.HasLen, 0)
}

func (s *aztestsSuite) TestContainerGetPropsAndMetaNonExistantContainer(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := getContainerURL(c, bsu)

	_, err := containerURL.GetPropertiesAndMetadata(ctx, azblob.LeaseAccessConditions{})
	validateStorageError(c, err, azblob.ServiceCodeContainerNotFound)
}

func (s *aztestsSuite) TestContainerSetMetadataEmpty(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := getContainerURL(c, bsu)
	_, err := containerURL.Create(ctx, basicMetadata, azblob.PublicAccessBlob)

	defer deleteContainer(c, containerURL)

	_, err = containerURL.SetMetadata(ctx, azblob.Metadata{}, azblob.ContainerAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := containerURL.GetPropertiesAndMetadata(ctx, azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.NewMetadata(), chk.HasLen, 0)
}

func (*aztestsSuite) TestContainerSetMetadataNil(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := getContainerURL(c, bsu)
	_, err := containerURL.Create(ctx, basicMetadata, azblob.PublicAccessBlob)

	defer deleteContainer(c, containerURL)

	_, err = containerURL.SetMetadata(ctx, nil, azblob.ContainerAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := containerURL.GetPropertiesAndMetadata(ctx, azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.NewMetadata(), chk.HasLen, 0)
}

func (*aztestsSuite) TestContainerSetMetadataInvalidField(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)

	defer deleteContainer(c, containerURL)

	_, err := containerURL.SetMetadata(ctx, azblob.Metadata{"!nval!d Field!@#%": "value"}, azblob.ContainerAccessConditions{})
	c.Assert(err, chk.NotNil)
	c.Assert(strings.Contains(err.Error(), validationErrorSubstring), chk.Equals, true)
}

func (*aztestsSuite) TestContainerSetMetadataNonExistant(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := getContainerURL(c, bsu)

	_, err := containerURL.SetMetadata(ctx, nil, azblob.ContainerAccessConditions{})
	validateStorageError(c, err, azblob.ServiceCodeContainerNotFound)
}

func (s *aztestsSuite) TestContainerSetMetadataIfModifiedSinceTrue(c *chk.C) {
	currentTime := getRelativeTimeGMT(-10)

	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)

	defer deleteContainer(c, containerURL)

	_, err := containerURL.SetMetadata(ctx, basicMetadata,
		azblob.ContainerAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	resp, err := containerURL.GetPropertiesAndMetadata(ctx, azblob.LeaseAccessConditions{})

	c.Assert(err, chk.IsNil)
	c.Assert(resp.NewMetadata(), chk.DeepEquals, basicMetadata)

}

func (s *aztestsSuite) TestContainerSetMetadataIfModifiedSinceFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)

	defer deleteContainer(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	_, err := containerURL.SetMetadata(ctx, basicMetadata,
		azblob.ContainerAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})

	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestContainerNewBlobURL(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := getContainerURL(c, bsu)

	blobURL := containerURL.NewBlobURL(blobPrefix)
	tempBlob := blobURL.URL()
	tempContainer := containerURL.URL()
	c.Assert(tempBlob.String(), chk.Equals, tempContainer.String()+"/"+blobPrefix)
	c.Assert(blobURL, chk.FitsTypeOf, azblob.BlobURL{})
}

func (s *aztestsSuite) TestContainerNewBlockBlobURL(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := getContainerURL(c, bsu)

	blobURL := containerURL.NewBlockBlobURL(blobPrefix)
	tempBlob := blobURL.URL()
	tempContainer := containerURL.URL()
	c.Assert(tempBlob.String(), chk.Equals, tempContainer.String()+"/"+blobPrefix)
	c.Assert(blobURL, chk.FitsTypeOf, azblob.BlockBlobURL{})
}

func (s *aztestsSuite) TestBlobWithNewPipeline(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := getContainerURL(c, bsu)
	blobURL := containerURL.NewBlockBlobURL(blobPrefix)

	newBlobURL := blobURL.WithPipeline(testPipeline{})
	_, err := newBlobURL.GetBlockList(ctx, azblob.BlockListAll, azblob.LeaseAccessConditions{})
	c.Assert(err, chk.NotNil)
	c.Assert(err.Error(), chk.Equals, testPipelineMessage)
}

func waitForCopy(c *chk.C, copyBlobURL azblob.BlockBlobURL, blobCopyResponse *azblob.BlobsCopyResponse) {
	status := blobCopyResponse.CopyStatus()
	// Wait for the copy to finish. If the copy takes longer than a minute, we will fail
	start := time.Now()
	for status != azblob.CopyStatusSuccess {
		GetPropertiesAndMetadataResult, _ := copyBlobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
		status = GetPropertiesAndMetadataResult.CopyStatus()
		currentTime := time.Now()
		if currentTime.Sub(start) >= time.Minute {
			c.Fail()
		}
	}
}

func (s *aztestsSuite) TestBlobStartCopyDestEmpty(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)
	copyBlobURL, _ := getBlockBlobURL(c, containerURL)

	blobCopyResponse, err := copyBlobURL.StartCopy(ctx, blobURL.URL(), nil, azblob.BlobAccessConditions{}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	waitForCopy(c, copyBlobURL, blobCopyResponse)

	resp, err := copyBlobURL.GetBlob(ctx, azblob.BlobRange{Offset: 0, Count: 20}, azblob.BlobAccessConditions{}, false)
	c.Assert(err, chk.IsNil)

	// Read the blob data to verify the copy
	data, _ := ioutil.ReadAll(resp.Response().Body)
	c.Assert(resp.ContentLength(), chk.Equals, int64(len(blockBlobDefaultData)))
	c.Assert(string(data), chk.Equals, blockBlobDefaultData)
	resp.Body().Close()
}

func (s *aztestsSuite) TestBlobStartCopyMetadata(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)
	copyBlobURL, _ := getBlockBlobURL(c, containerURL)

	resp, err := copyBlobURL.StartCopy(ctx, blobURL.URL(), basicMetadata, azblob.BlobAccessConditions{}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	waitForCopy(c, copyBlobURL, resp)

	resp2, err := copyBlobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp2.NewMetadata(), chk.DeepEquals, basicMetadata)
}

func (s *aztestsSuite) TestBlobStartCopyMetadataNil(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)
	copyBlobURL, _ := getBlockBlobURL(c, containerURL)

	// Have the destination start with metadata so we ensure the nil metadata passed later takes effect
	_, err := copyBlobURL.PutBlob(ctx, bytes.NewReader([]byte("data")), azblob.BlobHTTPHeaders{},
		basicMetadata, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := copyBlobURL.StartCopy(ctx, blobURL.URL(), nil, azblob.BlobAccessConditions{}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	waitForCopy(c, copyBlobURL, resp)

	resp2, err := copyBlobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp2.NewMetadata(), chk.HasLen, 0)
}

func (s *aztestsSuite) TestBlobStartCopyMetadataEmpty(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)
	copyBlobURL, _ := getBlockBlobURL(c, containerURL)

	// Have the destination start with metadata so we ensure the empty metadata passed later takes effect
	_, err := copyBlobURL.PutBlob(ctx, bytes.NewReader([]byte("data")), azblob.BlobHTTPHeaders{},
		basicMetadata, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := copyBlobURL.StartCopy(ctx, blobURL.URL(), azblob.Metadata{}, azblob.BlobAccessConditions{}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	waitForCopy(c, copyBlobURL, resp)

	resp2, err := copyBlobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp2.NewMetadata(), chk.HasLen, 0)
}

func (s *aztestsSuite) TestBlobStartCopyMetadataInvalidField(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)
	copyBlobURL, _ := getBlockBlobURL(c, containerURL)

	_, err := copyBlobURL.StartCopy(ctx, blobURL.URL(), azblob.Metadata{"I nvalid.": "bar"}, azblob.BlobAccessConditions{}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.NotNil)
	c.Assert(strings.Contains(err.Error(), validationErrorSubstring), chk.Equals, true)
}

func (s *aztestsSuite) TestBlobStartCopySourceNonExistant(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getBlockBlobURL(c, containerURL)
	copyBlobURL, _ := getBlockBlobURL(c, containerURL)

	_, err := copyBlobURL.StartCopy(ctx, blobURL.URL(), nil, azblob.BlobAccessConditions{}, azblob.BlobAccessConditions{})
	validateStorageError(c, err, azblob.ServiceCodeBlobNotFound)
}

func (s *aztestsSuite) TestBlobStartCopySourcePrivate(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	_, err := containerURL.SetPermissions(ctx, azblob.PublicAccessNone, nil, azblob.ContainerAccessConditions{})
	c.Assert(err, chk.IsNil)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	bsu2, err := getAlternateBSU()
	if err != nil {
		c.Skip(err.Error())
		return
	}
	copyContainerURL, _ := createNewContainer(c, bsu2)
	defer deleteContainer(c, copyContainerURL)
	copyBlobURL, _ := getBlockBlobURL(c, copyContainerURL)

	if bsu.String() == bsu2.String() {
		c.Skip("Test not valid because primary and secondary accounts are the same")
	}
	_, err = copyBlobURL.StartCopy(ctx, blobURL.URL(), nil, azblob.BlobAccessConditions{}, azblob.BlobAccessConditions{})
	validateStorageError(c, err, azblob.ServiceCodeCannotVerifyCopySource)
}

func (s *aztestsSuite) TestBlobStartCopyUsingSASSrc(c *chk.C) {
	bsu := getBSU()
	containerURL, containerName := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	_, err := containerURL.SetPermissions(ctx, azblob.PublicAccessNone, nil, azblob.ContainerAccessConditions{})
	c.Assert(err, chk.IsNil)
	blobURL, blobName := createNewBlockBlob(c, containerURL)

	// Create sas values for the source blob
	credentials := azblob.NewSharedKeyCredential(os.Getenv("ACCOUNT_NAME"), os.Getenv("ACCOUNT_KEY"))
	serviceSASValues := azblob.BlobSASSignatureValues{Version: "2015-04-05", StartTime: time.Now().Add(-1 * time.Hour).UTC(),
		ExpiryTime: time.Now().Add(time.Hour).UTC(), Permissions: azblob.BlobSASPermissions{Read: true, Write: true}.String(),
		ContainerName: containerName, BlobName: blobName}
	queryParams := serviceSASValues.NewSASQueryParameters(credentials)

	// Create URLs to the destination blob with sas parameters
	sasURL := blobURL.URL()
	sasURL.RawQuery = queryParams.Encode()

	// Create a new container for the destination
	bsu2, err := getAlternateBSU()
	if err != nil {
		c.Skip(err.Error())
		return
	}
	copyContainerURL, _ := createNewContainer(c, bsu2)
	defer deleteContainer(c, copyContainerURL)
	copyBlobURL, _ := getBlockBlobURL(c, copyContainerURL)

	resp, err := copyBlobURL.StartCopy(ctx, sasURL, nil, azblob.BlobAccessConditions{}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	waitForCopy(c, copyBlobURL, resp)

	resp2, err := copyBlobURL.GetBlob(ctx, azblob.BlobRange{Offset: 0, Count: int64(len(blockBlobDefaultData))}, azblob.BlobAccessConditions{}, false)
	c.Assert(err, chk.IsNil)

	data, err := ioutil.ReadAll(resp2.Response().Body)
	c.Assert(resp2.ContentLength(), chk.Equals, int64(len(blockBlobDefaultData)))
	c.Assert(string(data), chk.Equals, blockBlobDefaultData)
	resp2.Body().Close()
}

func (s *aztestsSuite) TestBlobStartCopyUsingSASDest(c *chk.C) {
	bsu := getBSU()
	containerURL, containerName := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	_, err := containerURL.SetPermissions(ctx, azblob.PublicAccessNone, nil, azblob.ContainerAccessConditions{})
	c.Assert(err, chk.IsNil)
	blobURL, blobName := createNewBlockBlob(c, containerURL)
	_ = blobURL

	// Generate SAS on the source
	serviceSASValues := azblob.BlobSASSignatureValues{ExpiryTime: time.Now().Add(time.Hour).UTC(),
		Permissions: azblob.BlobSASPermissions{Read: true, Write: true, Create: true}.String(), ContainerName: containerName, BlobName: blobName}
	credentials := azblob.NewSharedKeyCredential(os.Getenv("ACCOUNT_NAME"), os.Getenv("ACCOUNT_KEY"))
	queryParams := serviceSASValues.NewSASQueryParameters(credentials)

	// Create destination container
	bsu2, err := getAlternateBSU()
	if err != nil {
		c.Skip(err.Error())
		return
	}

	copyContainerURL, copyContainerName := createNewContainer(c, bsu2)
	defer deleteContainer(c, copyContainerURL)
	copyBlobURL, copyBlobName := getBlockBlobURL(c, copyContainerURL)

	// Generate Sas for the destination
	credentials = azblob.NewSharedKeyCredential(os.Getenv("SECONDARY_ACCOUNT_NAME"), os.Getenv("SECONDARY_ACCOUNT_KEY"))
	copyServiceSASvalues := azblob.BlobSASSignatureValues{StartTime: time.Now().Add(-1 * time.Hour).UTC(),
		ExpiryTime: time.Now().Add(time.Hour).UTC(), Permissions: azblob.BlobSASPermissions{Read: true, Write: true}.String(),
		ContainerName: copyContainerName, BlobName: copyBlobName}
	copyQueryParams := copyServiceSASvalues.NewSASQueryParameters(credentials)

	// Generate anonymous URL to destination with SAS
	anonURL := bsu2.URL()
	anonURL.RawQuery = copyQueryParams.Encode()
	anonPipeline := azblob.NewPipeline(azblob.NewAnonymousCredential(), azblob.PipelineOptions{})
	anonBSU := azblob.NewServiceURL(anonURL, anonPipeline)
	anonContainerURL := anonBSU.NewContainerURL(copyContainerName)
	anonBlobURL := anonContainerURL.NewBlockBlobURL(copyBlobName)

	// Apply sas to source
	srcBlobWithSasURL := blobURL.URL()
	srcBlobWithSasURL.RawQuery = queryParams.Encode()

	resp, err := anonBlobURL.StartCopy(ctx, srcBlobWithSasURL, nil, azblob.BlobAccessConditions{}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	// Allow copy to happen
	waitForCopy(c, anonBlobURL, resp)

	resp2, err := copyBlobURL.GetBlob(ctx, azblob.BlobRange{Offset: 0, Count: int64(len(blockBlobDefaultData))}, azblob.BlobAccessConditions{}, false)
	c.Assert(err, chk.IsNil)

	data, err := ioutil.ReadAll(resp2.Response().Body)
	_, err = resp2.Body().Read(data)
	c.Assert(resp2.ContentLength(), chk.Equals, int64(len(blockBlobDefaultData)))
	c.Assert(string(data), chk.Equals, blockBlobDefaultData)
	resp2.Body().Close()
}

func (s *aztestsSuite) TestBlobStartCopySourceIfModifiedSinceTrue(c *chk.C) {
	currentTime := getRelativeTimeGMT(-10)

	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	destBlobURL, _ := getBlockBlobURL(c, containerURL)
	_, err := destBlobURL.StartCopy(ctx, blobURL.URL(), basicMetadata,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}},
		azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := destBlobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.NewMetadata(), chk.DeepEquals, basicMetadata)
}

func (s *aztestsSuite) TestBlobStartCopySourceIfModifiedSinceFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	destBlobURL, _ := getBlockBlobURL(c, containerURL)
	_, err := destBlobURL.StartCopy(ctx, blobURL.URL(), nil,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}},
		azblob.BlobAccessConditions{})
	validateStorageError(c, err, azblob.ServiceCodeSourceConditionNotMet)
}

func (s *aztestsSuite) TestBlobStartCopySourceIfUnmodifiedSinceTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	destBlobURL, _ := getBlockBlobURL(c, containerURL)
	_, err := destBlobURL.StartCopy(ctx, blobURL.URL(), basicMetadata,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}},
		azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := destBlobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.NewMetadata(), chk.DeepEquals, basicMetadata)
}

func (s *aztestsSuite) TestBlobStartCopySourceIfUnmodifiedSinceFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(-10)

	destBlobURL, _ := getBlockBlobURL(c, containerURL)
	_, err := destBlobURL.StartCopy(ctx, blobURL.URL(), nil,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}},
		azblob.BlobAccessConditions{})
	validateStorageError(c, err, azblob.ServiceCodeSourceConditionNotMet)
}

func (s *aztestsSuite) TestBlobStartCopySourceIfMatchTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	etag := resp.ETag()

	destBlobURL, _ := getBlockBlobURL(c, containerURL)
	_, err = destBlobURL.StartCopy(ctx, blobURL.URL(), basicMetadata,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: etag}},
		azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp2, err := destBlobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp2.NewMetadata(), chk.DeepEquals, basicMetadata)
}

func (s *aztestsSuite) TestBlobStartCopySourceIfMatchFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	destBlobURL, _ := getBlockBlobURL(c, containerURL)
	_, err := destBlobURL.StartCopy(ctx, blobURL.URL(), basicMetadata,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: "a"}},
		azblob.BlobAccessConditions{})
	validateStorageError(c, err, azblob.ServiceCodeSourceConditionNotMet)
}

func (s *aztestsSuite) TestBlobStartCopySourceIfNoneMatchTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	destBlobURL, _ := getBlockBlobURL(c, containerURL)
	_, err := destBlobURL.StartCopy(ctx, blobURL.URL(), basicMetadata,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: "a"}},
		azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp2, err := destBlobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp2.NewMetadata(), chk.DeepEquals, basicMetadata)
}

func (s *aztestsSuite) TestBlobStartCopySourceIfNoneMatchFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	etag := resp.ETag()

	destBlobURL, _ := getBlockBlobURL(c, containerURL)
	_, err = destBlobURL.StartCopy(ctx, blobURL.URL(), nil,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: etag}},
		azblob.BlobAccessConditions{})
	validateStorageError(c, err, azblob.ServiceCodeSourceConditionNotMet)
}

func (s *aztestsSuite) TestBlobStartCopyDestIfModifiedSinceTrue(c *chk.C) {
	currentTime := getRelativeTimeGMT(-10)
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	destBlobURL, _ := createNewBlockBlob(c, containerURL) // The blob must exist to have a last-modified time
	_, err := destBlobURL.StartCopy(ctx, blobURL.URL(), basicMetadata,
		azblob.BlobAccessConditions{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	resp, err := destBlobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.NewMetadata(), chk.DeepEquals, basicMetadata)
}

func (s *aztestsSuite) TestBlobStartCopyDestIfModifiedSinceFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	destBlobURL, _ := createNewBlockBlob(c, containerURL)
	currentTime := getRelativeTimeGMT(10)

	_, err := destBlobURL.StartCopy(ctx, blobURL.URL(), nil,
		azblob.BlobAccessConditions{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeTargetConditionNotMet)
}

func (s *aztestsSuite) TestBlobStartCopyDestIfUnmodifiedSinceTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	destBlobURL, _ := createNewBlockBlob(c, containerURL)
	currentTime := getRelativeTimeGMT(10)

	_, err := destBlobURL.StartCopy(ctx, blobURL.URL(), basicMetadata,
		azblob.BlobAccessConditions{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	resp, err := destBlobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.NewMetadata(), chk.DeepEquals, basicMetadata)
}

func (s *aztestsSuite) TestBlobStartCopyDestIfUnmodifiedSinceFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(-10)
	destBlobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := destBlobURL.StartCopy(ctx, blobURL.URL(), nil,
		azblob.BlobAccessConditions{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeTargetConditionNotMet)
}

func (s *aztestsSuite) TestBlobStartCopyDestIfMatchTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	destBlobURL, _ := createNewBlockBlob(c, containerURL)
	resp, _ := destBlobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	etag := resp.ETag()

	_, err := destBlobURL.StartCopy(ctx, blobURL.URL(), basicMetadata,
		azblob.BlobAccessConditions{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: etag}})
	c.Assert(err, chk.IsNil)

	resp, err = destBlobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.NewMetadata(), chk.DeepEquals, basicMetadata)
}

func (s *aztestsSuite) TestBlobStartCopyDestIfMatchFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	destBlobURL, _ := createNewBlockBlob(c, containerURL)
	resp, _ := destBlobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	etag := resp.ETag()

	destBlobURL.SetMetadata(ctx, nil, azblob.BlobAccessConditions{}) // SetMetadata chances the blob's etag

	_, err := destBlobURL.StartCopy(ctx, blobURL.URL(), nil, azblob.BlobAccessConditions{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: etag}})
	validateStorageError(c, err, azblob.ServiceCodeTargetConditionNotMet)
}

func (s *aztestsSuite) TestBlobStartCopyDestIfNoneMatchTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	destBlobURL, _ := createNewBlockBlob(c, containerURL)
	resp, _ := destBlobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	etag := resp.ETag()

	destBlobURL.SetMetadata(ctx, nil, azblob.BlobAccessConditions{}) // SetMetadata chances the blob's etag

	_, err := destBlobURL.StartCopy(ctx, blobURL.URL(), basicMetadata, azblob.BlobAccessConditions{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: etag}})
	c.Assert(err, chk.IsNil)

	resp, err = destBlobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.NewMetadata(), chk.DeepEquals, basicMetadata)
}

func (s *aztestsSuite) TestBlobStartCopyDestIfNoneMatchFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	destBlobURL, _ := createNewBlockBlob(c, containerURL)
	resp, _ := destBlobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	etag := resp.ETag()

	_, err := destBlobURL.StartCopy(ctx, blobURL.URL(), nil, azblob.BlobAccessConditions{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: etag}})
	validateStorageError(c, err, azblob.ServiceCodeTargetConditionNotMet)
}

func (s *aztestsSuite) TestBlobAbortCopyInProgress(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getBlockBlobURL(c, containerURL)

	// Create a large blob that takes time to copy
	blobSize := 8 * 1024 * 1024
	blobData := make([]byte, blobSize, blobSize)
	for i := range blobData {
		blobData[i] = byte('a' + i%26)
	}
	_, err := blobURL.PutBlob(ctx, bytes.NewReader(blobData), azblob.BlobHTTPHeaders{}, nil, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	containerURL.SetPermissions(ctx, azblob.PublicAccessBlob, nil, azblob.ContainerAccessConditions{}) // So that we don't have to create a SAS

	// Must copy across accounts so it takes time to copy
	bsu2, err := getAlternateBSU()
	if err != nil {
		c.Skip(err.Error())
		return
	}

	copyContainerURL, _ := createNewContainer(c, bsu2)
	copyBlobURL, _ := getBlockBlobURL(c, copyContainerURL)

	defer deleteContainer(c, copyContainerURL)

	resp, err := copyBlobURL.StartCopy(ctx, blobURL.URL(), nil, azblob.BlobAccessConditions{}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.CopyStatus(), chk.Equals, azblob.CopyStatusPending)

	_, err = copyBlobURL.AbortCopy(ctx, resp.CopyID(), azblob.LeaseAccessConditions{})
	if err != nil {
		// If the error is nil, the test continues as normal.
		// If the error is not nil, we want to check if it's because the copy is finished and send a message indicating this.
		validateStorageError(c, err, azblob.ServiceCodeNoPendingCopyOperation)
		c.Error("The test failed because the copy completed because it was aborted")
	}

	resp2, _ := copyBlobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(resp2.CopyStatus(), chk.Equals, azblob.CopyStatusAborted)
}

func (s *aztestsSuite) TestBlobAbortCopyNoCopyStarted(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)

	defer deleteContainer(c, containerURL)

	copyBlobURL, _ := getBlockBlobURL(c, containerURL)
	_, err := copyBlobURL.AbortCopy(ctx, "copynotstarted", azblob.LeaseAccessConditions{})
	validateStorageError(c, err, azblob.ServiceCodeInvalidQueryParameterValue)
}

func (s *aztestsSuite) TestBlobSnapshotMetadata(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	resp, err := blobURL.CreateSnapshot(ctx, basicMetadata, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	// Since metadata is specified on the snapshot, the snapshot should have its own metadata different from the (empty) metadata on the source
	snapshotURL := blobURL.WithSnapshot(resp.Snapshot())
	resp2, err := snapshotURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp2.NewMetadata(), chk.DeepEquals, basicMetadata)
}

func (s *aztestsSuite) TestBlobSnapshotMetadataEmpty(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.SetMetadata(ctx, basicMetadata, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := blobURL.CreateSnapshot(ctx, azblob.Metadata{}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	// In this case, because no metadata was specified, it should copy the basicMetadata from the source
	snapshotURL := blobURL.WithSnapshot(resp.Snapshot())
	resp2, err := snapshotURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp2.NewMetadata(), chk.DeepEquals, basicMetadata)
}

func (s *aztestsSuite) TestBlobSnapshotMetadataNil(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.SetMetadata(ctx, basicMetadata, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := blobURL.CreateSnapshot(ctx, nil, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	snapshotURL := blobURL.WithSnapshot(resp.Snapshot())
	resp2, err := snapshotURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp2.NewMetadata(), chk.DeepEquals, basicMetadata)
}

func (s *aztestsSuite) TestBlobSnapshotMetadataInvalid(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.CreateSnapshot(ctx, azblob.Metadata{"Invalid Field!": "value"}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.NotNil)
	c.Assert(strings.Contains(err.Error(), validationErrorSubstring), chk.Equals, true)
}

func (s *aztestsSuite) TestBlobSnapshotBlobNotExist(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getBlockBlobURL(c, containerURL)

	_, err := blobURL.CreateSnapshot(ctx, nil, azblob.BlobAccessConditions{})
	validateStorageError(c, err, azblob.ServiceCodeBlobNotFound)
}

func (s *aztestsSuite) TestBlobSnapshotOfSnapshot(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	snapshotURL := blobURL.WithSnapshot(time.Now().UTC())
	// The library allows the server to handle the snapshot of snapshot error
	_, err := snapshotURL.CreateSnapshot(ctx, nil, azblob.BlobAccessConditions{})
	validateStorageError(c, err, azblob.ServiceCodeInvalidQueryParameterValue)
}

func (s *aztestsSuite) TestBlobSnapshotIfModifiedSinceTrue(c *chk.C) {
	currentTime := getRelativeTimeGMT(-10)

	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	resp, err := blobURL.CreateSnapshot(ctx, nil,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.Snapshot().IsZero(), chk.Equals, false) // i.e. The snapshot time is not zero. If the service gives us back a snapshot time, it successfully created a snapshot
}

func (s *aztestsSuite) TestBlobSnapshotIfModifiedSinceFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	_, err := blobURL.CreateSnapshot(ctx, nil,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobSnapshotIfUnmodifiedSinceTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	resp, err := blobURL.CreateSnapshot(ctx, nil,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.Snapshot().IsZero(), chk.Equals, false)
}

func (s *aztestsSuite) TestBlobSnapshotIfUnmodifiedSinceFalse(c *chk.C) {
	currentTime := getRelativeTimeGMT(-10)

	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.CreateSnapshot(ctx, nil,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobSnapshotIfMatchTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})

	resp2, err := blobURL.CreateSnapshot(ctx, nil,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: resp.ETag()}})
	c.Assert(err, chk.IsNil)
	c.Assert(resp2.Snapshot().IsZero(), chk.Equals, false)
}

func (s *aztestsSuite) TestBlobSnapshotIfMatchFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.CreateSnapshot(ctx, nil,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: "garbage"}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobSnapshotIfNoneMatchTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	resp, err := blobURL.CreateSnapshot(ctx, nil,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: "garbage"}})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.Snapshot().IsZero(), chk.Equals, false)
}

func (s *aztestsSuite) TestBlobSnapshotIfNoneMatchFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})

	_, err = blobURL.CreateSnapshot(ctx, nil,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: resp.ETag()}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobDownloadDataNonExistantBlob(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getBlockBlobURL(c, containerURL)

	_, err := blobURL.GetBlob(ctx, azblob.BlobRange{}, azblob.BlobAccessConditions{}, false)
	validateStorageError(c, err, azblob.ServiceCodeBlobNotFound)
}

func (s *aztestsSuite) TestBlobDownloadDataNegativeOffset(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	defer func() { // The library should fail if it seems numeric parameters that are guaranteed invalid
		recover()
	}()

	blobURL.GetBlob(ctx, azblob.BlobRange{Offset: -1}, azblob.BlobAccessConditions{}, false)

	c.Fail()
}

func (s *aztestsSuite) TestBlobDownloadDataOffsetOutOfRange(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.GetBlob(ctx, azblob.BlobRange{Offset: int64(len(blockBlobDefaultData))}, azblob.BlobAccessConditions{}, false)
	validateStorageError(c, err, azblob.ServiceCodeInvalidRange)
}

func (s *aztestsSuite) TestBlobDownloadDataCountNegative(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	defer func() { // The library should panic if it sees numeric parameters that are guaranteed invalid
		recover()
	}()

	blobURL.GetBlob(ctx, azblob.BlobRange{Count: -1}, azblob.BlobAccessConditions{}, false)

	c.Fail()
}

func (s *aztestsSuite) TestBlobDownloadDataCountZero(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	resp, err := blobURL.GetBlob(ctx, azblob.BlobRange{Count: 0}, azblob.BlobAccessConditions{}, false)
	c.Assert(err, chk.IsNil)

	// Specifying a count of 0 results in the value being ignored
	data, err := ioutil.ReadAll(resp.Response().Body)
	c.Assert(err, chk.IsNil)
	c.Assert(string(data), chk.Equals, blockBlobDefaultData)
}

func (s *aztestsSuite) TestBlobDownloadDataCountExact(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	resp, err := blobURL.GetBlob(ctx, azblob.BlobRange{Count: int64(len(blockBlobDefaultData))}, azblob.BlobAccessConditions{}, false)
	c.Assert(err, chk.IsNil)

	data, err := ioutil.ReadAll(resp.Response().Body)
	c.Assert(err, chk.IsNil)
	c.Assert(string(data), chk.Equals, blockBlobDefaultData)
}

func (s *aztestsSuite) TestBlobDownloadDataCountOutOfRange(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	resp, err := blobURL.GetBlob(ctx, azblob.BlobRange{Count: int64(len(blockBlobDefaultData)) * 2}, azblob.BlobAccessConditions{}, false)
	c.Assert(err, chk.IsNil)

	data, err := ioutil.ReadAll(resp.Response().Body)
	c.Assert(err, chk.IsNil)
	c.Assert(string(data), chk.Equals, blockBlobDefaultData)
}

func (s *aztestsSuite) TestBlobDownloadDataEmptyRangeStruct(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	resp, err := blobURL.GetBlob(ctx, azblob.BlobRange{}, azblob.BlobAccessConditions{}, false)
	c.Assert(err, chk.IsNil)

	data, err := ioutil.ReadAll(resp.Response().Body)
	c.Assert(err, chk.IsNil)
	c.Assert(string(data), chk.Equals, blockBlobDefaultData)
}

func (s *aztestsSuite) TestBlobDownloadDataContentMD5(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	resp, err := blobURL.GetBlob(ctx, azblob.BlobRange{Offset: 10, Count: 3}, azblob.BlobAccessConditions{}, true)
	c.Assert(err, chk.IsNil)
	mdf := md5.Sum([]byte(blockBlobDefaultData)[10:13])
	c.Assert(resp.ContentMD5(), chk.Equals, mdf)
}

func (s *aztestsSuite) TestBlobDownloadDataIfModifiedSinceTrue(c *chk.C) {
	currentTime := getRelativeTimeGMT(-10)

	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	resp, err := blobURL.GetBlob(ctx, azblob.BlobRange{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}}, false)
	c.Assert(err, chk.IsNil)
	c.Assert(resp.ContentLength(), chk.Equals, int64(len(blockBlobDefaultData)))
}

func (s *aztestsSuite) TestBlobDownloadDataIfModifiedSinceFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	_, err := blobURL.GetBlob(ctx, azblob.BlobRange{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}}, false)
	serr := err.(azblob.StorageError)
	c.Assert(serr.Response().StatusCode, chk.Equals, 304) // The server does not return the error in the body even though it is a GET
}

func (s *aztestsSuite) TestBlobDownloadDataIfUnmodifiedSinceTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	resp, err := blobURL.GetBlob(ctx, azblob.BlobRange{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}}, false)
	c.Assert(err, chk.IsNil)
	c.Assert(resp.ContentLength(), chk.Equals, int64(len(blockBlobDefaultData)))
}

func (s *aztestsSuite) TestBlobDownloadDataIfUnmodifiedSinceFalse(c *chk.C) {
	currentTime := getRelativeTimeGMT(-10)

	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.GetBlob(ctx, azblob.BlobRange{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}}, false)
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobDownloadDataIfMatchTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	etag := resp.ETag()

	resp2, err := blobURL.GetBlob(ctx, azblob.BlobRange{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: etag}}, false)
	c.Assert(err, chk.IsNil)
	c.Assert(resp2.ContentLength(), chk.Equals, int64(len(blockBlobDefaultData)))
}

func (s *aztestsSuite) TestBlobDownloadDataIfMatchFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	etag := resp.ETag()

	blobURL.SetMetadata(ctx, nil, azblob.BlobAccessConditions{})

	_, err = blobURL.GetBlob(ctx, azblob.BlobRange{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: etag}}, false)
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobDownloadDataIfNoneMatchTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	etag := resp.ETag()

	blobURL.SetMetadata(ctx, nil, azblob.BlobAccessConditions{})

	resp2, err := blobURL.GetBlob(ctx, azblob.BlobRange{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: etag}}, false)
	c.Assert(err, chk.IsNil)
	c.Assert(resp2.ContentLength(), chk.Equals, int64(len(blockBlobDefaultData)))
}

func (s *aztestsSuite) TestBlobDownloadDataIfNoneMatchFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	etag := resp.ETag()

	_, err = blobURL.GetBlob(ctx, azblob.BlobRange{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: etag}}, false)
	serr := err.(azblob.StorageError)
	c.Assert(serr.Response().StatusCode, chk.Equals, 304) // The server does not return the error in the body even though it is a GET
}

func (s *aztestsSuite) TestBlobDeleteNonExistant(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getBlockBlobURL(c, containerURL)

	_, err := blobURL.Delete(ctx, azblob.DeleteSnapshotsOptionInclude, azblob.BlobAccessConditions{})
	validateStorageError(c, err, azblob.ServiceCodeBlobNotFound)
}

func (s *aztestsSuite) TestBlobDeleteSnapshot(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	resp, err := blobURL.CreateSnapshot(ctx, nil, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	snapshotURL := blobURL.WithSnapshot(resp.Snapshot())

	_, err = snapshotURL.Delete(ctx, azblob.DeleteSnapshotsOptionNone, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	validateBlobDeleted(c, snapshotURL)
}

func (s *aztestsSuite) TestBlobDeleteSnapshotsInclude(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.CreateSnapshot(ctx, nil, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	_, err = blobURL.Delete(ctx, azblob.DeleteSnapshotsOptionInclude, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, _ := containerURL.ListBlobs(ctx, azblob.Marker{},
		azblob.ListBlobsOptions{Details: azblob.BlobListingDetails{Snapshots: true}})
	c.Assert(resp.Blobs.Blob, chk.HasLen, 0)
}

func (s *aztestsSuite) TestBlobDeleteSnapshotsOnly(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.CreateSnapshot(ctx, nil, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	_, err = blobURL.Delete(ctx, azblob.DeleteSnapshotsOptionOnly, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, _ := containerURL.ListBlobs(ctx, azblob.Marker{},
		azblob.ListBlobsOptions{Details: azblob.BlobListingDetails{Snapshots: true}})
	c.Assert(resp.Blobs.Blob, chk.HasLen, 1)
	c.Assert(resp.Blobs.Blob[0].Snapshot.IsZero(), chk.Equals, true)
}

func (s *aztestsSuite) TestBlobDeleteSnapshotsNoneWithSnapshots(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.CreateSnapshot(ctx, nil, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	_, err = blobURL.Delete(ctx, azblob.DeleteSnapshotsOptionNone, azblob.BlobAccessConditions{})
	validateStorageError(c, err, azblob.ServiceCodeSnapshotsPresent)
}

func validateBlobDeleted(c *chk.C, blobURL azblob.BlockBlobURL) {
	_, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.NotNil)
	serr := err.(azblob.StorageError) // Delete blob is a HEAD request and does not return a ServiceCode in the body
	c.Assert(serr.Response().StatusCode, chk.Equals, 404)
}

func (s *aztestsSuite) TestBlobDeleteIfModifiedSinceTrue(c *chk.C) {
	currentTime := getRelativeTimeGMT(-10)

	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.Delete(ctx, azblob.DeleteSnapshotsOptionNone,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	validateBlobDeleted(c, blobURL)
}

func (s *aztestsSuite) TestBlobDeleteIfModifiedSinceFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	_, err := blobURL.Delete(ctx, azblob.DeleteSnapshotsOptionNone,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobDeleteIfUnmodifiedSinceTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	_, err := blobURL.Delete(ctx, azblob.DeleteSnapshotsOptionNone,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	validateBlobDeleted(c, blobURL)
}

func (s *aztestsSuite) TestBlobDeleteIfUnmodifiedSinceFalse(c *chk.C) {
	currentTime := getRelativeTimeGMT(-10)

	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.Delete(ctx, azblob.DeleteSnapshotsOptionNone,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobDeleteIfMatchTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	resp, _ := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	etag := resp.ETag()

	_, err := blobURL.Delete(ctx, azblob.DeleteSnapshotsOptionNone,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: etag}})
	c.Assert(err, chk.IsNil)

	validateBlobDeleted(c, blobURL)
}

func (s *aztestsSuite) TestBlobDeleteIfMatchFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	resp, _ := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	etag := resp.ETag()
	blobURL.SetMetadata(ctx, nil, azblob.BlobAccessConditions{})

	_, err := blobURL.Delete(ctx, azblob.DeleteSnapshotsOptionNone,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: etag}})

	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobDeleteIfNoneMatchTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	resp, _ := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	etag := resp.ETag()
	blobURL.SetMetadata(ctx, nil, azblob.BlobAccessConditions{})

	_, err := blobURL.Delete(ctx, azblob.DeleteSnapshotsOptionNone,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: etag}})
	c.Assert(err, chk.IsNil)

	validateBlobDeleted(c, blobURL)
}

func (s *aztestsSuite) TestBlobDeleteIfNoneMatchFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	resp, _ := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	etag := resp.ETag()

	_, err := blobURL.Delete(ctx, azblob.DeleteSnapshotsOptionNone,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: etag}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobPutBlobNonEmptyBody(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getBlockBlobURL(c, containerURL)

	_, err := blobURL.PutBlob(ctx, strings.NewReader(blockBlobDefaultData), azblob.BlobHTTPHeaders{}, nil, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := blobURL.GetBlob(ctx, azblob.BlobRange{}, azblob.BlobAccessConditions{}, false)
	c.Assert(err, chk.IsNil)
	data, _ := ioutil.ReadAll(resp.Response().Body)
	c.Assert(string(data), chk.Equals, blockBlobDefaultData)
}

func (s *aztestsSuite) TestBlobPutBlobHTTPHeaders(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getBlockBlobURL(c, containerURL)

	_, err := blobURL.PutBlob(ctx, nil, basicHeaders, nil, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	h := resp.NewHTTPHeaders()
	c.Assert(h, chk.DeepEquals, basicHeaders)
}

func (s *aztestsSuite) TestBlobPutBlobMetadataNotEmpty(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getBlockBlobURL(c, containerURL)

	_, err := blobURL.PutBlob(ctx, nil, azblob.BlobHTTPHeaders{}, basicMetadata, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.NewMetadata(), chk.DeepEquals, basicMetadata)
}

func (s *aztestsSuite) TestBlobPutBlobMetadataEmpty(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getBlockBlobURL(c, containerURL)

	_, err := blobURL.PutBlob(ctx, nil, azblob.BlobHTTPHeaders{}, azblob.Metadata{}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.NewMetadata(), chk.HasLen, 0)
}

func (s *aztestsSuite) TestBlobPutBlobMetadataInvalid(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getBlockBlobURL(c, containerURL)

	_, err := blobURL.PutBlob(ctx, nil, azblob.BlobHTTPHeaders{}, azblob.Metadata{"In valid!": "bar"}, azblob.BlobAccessConditions{})
	c.Assert(strings.Contains(err.Error(), validationErrorSubstring), chk.Equals, true)
}

func validatePutBlob(c *chk.C, blobURL azblob.BlockBlobURL) {
	resp, err := blobURL.GetBlob(ctx, azblob.BlobRange{}, azblob.BlobAccessConditions{}, false)
	c.Assert(err, chk.IsNil)
	data, _ := ioutil.ReadAll(resp.Response().Body)
	c.Assert(data, chk.HasLen, 0)
}

func (s *aztestsSuite) TestBlobPutBlobIfModifiedSinceTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(-10)

	_, err := blobURL.PutBlob(ctx, nil, azblob.BlobHTTPHeaders{}, nil,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	validatePutBlob(c, blobURL)
}

func (s *aztestsSuite) TestBlobPutBlobIfModifiedSinceFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	_, err := blobURL.PutBlob(ctx, nil, azblob.BlobHTTPHeaders{}, nil,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobPutBlobIfUnmodifiedSinceTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	_, err := blobURL.PutBlob(ctx, nil, azblob.BlobHTTPHeaders{}, nil,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	validatePutBlob(c, blobURL)
}

func (s *aztestsSuite) TestBlobPutBlobIfUnmodifiedSinceFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(-10)

	_, err := blobURL.PutBlob(ctx, nil, azblob.BlobHTTPHeaders{}, nil,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobPutBlobIfMatchTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	_, err = blobURL.PutBlob(ctx, nil, azblob.BlobHTTPHeaders{}, nil,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: resp.ETag()}})
	c.Assert(err, chk.IsNil)

	validatePutBlob(c, blobURL)
}

func (s *aztestsSuite) TestBlobPutBlobIfMatchFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	_, err = blobURL.PutBlob(ctx, nil, azblob.BlobHTTPHeaders{}, nil,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: azblob.ETag("garbage")}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobPutBlobIfNoneMatchTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	_, err = blobURL.PutBlob(ctx, nil, azblob.BlobHTTPHeaders{}, nil,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: azblob.ETag("garbage")}})
	c.Assert(err, chk.IsNil)

	validatePutBlob(c, blobURL)
}

func (s *aztestsSuite) TestBlobPutBlobIfNoneMatchFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	_, err = blobURL.PutBlob(ctx, nil, azblob.BlobHTTPHeaders{}, nil,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: resp.ETag()}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobGetPropsAndMetadataIfModifiedSinceTrue(c *chk.C) {
	currentTime := getRelativeTimeGMT(-10)

	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.SetMetadata(ctx, basicMetadata, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.NewMetadata(), chk.DeepEquals, basicMetadata)
}

func (s *aztestsSuite) TestBlobGetPropsAndMetadataIfModifiedSinceFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.SetMetadata(ctx, basicMetadata, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	currentTime := getRelativeTimeGMT(10)

	_, err = blobURL.GetPropertiesAndMetadata(ctx,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	c.Assert(err, chk.NotNil)
	serr := err.(azblob.StorageError)
	c.Assert(serr.Response().StatusCode, chk.Equals, 304) // No service code returned for a HEAD
}

func (s *aztestsSuite) TestBlobGetPropsAndMetadataIfUnmodifiedSinceTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.SetMetadata(ctx, basicMetadata, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	currentTime := getRelativeTimeGMT(10)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.NewMetadata(), chk.DeepEquals, basicMetadata)
}

func (s *aztestsSuite) TestBlobGetPropsAndMetadataIfUnmodifiedSinceFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(-10)

	_, err := blobURL.SetMetadata(ctx, basicMetadata, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	_, err = blobURL.GetPropertiesAndMetadata(ctx,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	c.Assert(err, chk.NotNil)
	serr := err.(azblob.StorageError)
	c.Assert(serr.Response().StatusCode, chk.Equals, 412)
}

func (s *aztestsSuite) TestBlobGetPropsAndMetadataIfMatchTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	resp, err := blobURL.SetMetadata(ctx, basicMetadata, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp2, err := blobURL.GetPropertiesAndMetadata(ctx,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: resp.ETag()}})
	c.Assert(err, chk.IsNil)
	c.Assert(resp2.NewMetadata(), chk.DeepEquals, basicMetadata)
}

func (s *aztestsSuite) TestBlobGetPropsAndMetadataIfMatchFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.GetPropertiesAndMetadata(ctx,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: azblob.ETag("garbage")}})
	c.Assert(err, chk.NotNil)
	serr := err.(azblob.StorageError)
	c.Assert(serr.Response().StatusCode, chk.Equals, 412)
}

func (s *aztestsSuite) TestBlobGetPropsAndMetadataIfNoneMatchTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.SetMetadata(ctx, basicMetadata, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: azblob.ETag("garbage")}})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.NewMetadata(), chk.DeepEquals, basicMetadata)
}

func (s *aztestsSuite) TestBlobGetPropsAndMetadataIfNoneMatchFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	resp, err := blobURL.SetMetadata(ctx, nil, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	_, err = blobURL.GetPropertiesAndMetadata(ctx,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: resp.ETag()}})
	c.Assert(err, chk.NotNil)
	serr := err.(azblob.StorageError)
	c.Assert(serr.Response().StatusCode, chk.Equals, 304)
}

func (s *aztestsSuite) TestBlobSetPropertiesBasic(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.SetProperties(ctx, basicHeaders, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, _ := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	h := resp.NewHTTPHeaders()
	c.Assert(h, chk.DeepEquals, basicHeaders)
}

func (s *aztestsSuite) TestBlobSetPropertiesEmptyValue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.SetProperties(ctx, azblob.BlobHTTPHeaders{ContentType: "my_type"}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	_, err = blobURL.SetProperties(ctx, azblob.BlobHTTPHeaders{}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.ContentType(), chk.Equals, "")
}

func validatePropertiesSet(c *chk.C, blobURL azblob.BlockBlobURL, str string) {
	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.ContentDisposition(), chk.Equals, "my_disposition")
}

func (s *aztestsSuite) TestBlobSetPropertiesIfModifiedSinceTrue(c *chk.C) {
	currentTime := getRelativeTimeGMT(-10)

	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.SetProperties(ctx, azblob.BlobHTTPHeaders{ContentDisposition: "my_disposition"},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	validatePropertiesSet(c, blobURL, "my_disposition")
}

func (s *aztestsSuite) TestBlobSetPropertiesIfModifiedSinceFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	_, err := blobURL.SetProperties(ctx, azblob.BlobHTTPHeaders{ContentDisposition: "my_disposition"},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobSetPropertiesIfUnmodifiedSinceTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	_, err := blobURL.SetProperties(ctx, azblob.BlobHTTPHeaders{ContentDisposition: "my_disposition"},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	validatePropertiesSet(c, blobURL, "my_disposition")
}

func (s *aztestsSuite) TestBlobSetPropertiesIfUnmodifiedSinceFalse(c *chk.C) {
	currentTime := getRelativeTimeGMT(-10)

	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.SetProperties(ctx, azblob.BlobHTTPHeaders{ContentDisposition: "my_disposition"},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobSetPropertiesIfMatchTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	_, err = blobURL.SetProperties(ctx, azblob.BlobHTTPHeaders{ContentDisposition: "my_disposition"},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: resp.ETag()}})
	c.Assert(err, chk.IsNil)

	validatePropertiesSet(c, blobURL, "my_disposition")
}

func (s *aztestsSuite) TestBlobSetPropertiesIfMatchFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.SetProperties(ctx, azblob.BlobHTTPHeaders{ContentDisposition: "my_disposition"},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: azblob.ETag("garbage")}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobSetPropertiesIfNoneMatchTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.SetProperties(ctx, azblob.BlobHTTPHeaders{ContentDisposition: "my_disposition"},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: azblob.ETag("garbage")}})
	c.Assert(err, chk.IsNil)

	validatePropertiesSet(c, blobURL, "my_disposition")
}

func (s *aztestsSuite) TestBlobSetPropertiesIfNoneMatchFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	_, err = blobURL.SetProperties(ctx, azblob.BlobHTTPHeaders{ContentDisposition: "my_disposition"},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: resp.ETag()}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobSetMetadataNil(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.SetMetadata(ctx, azblob.Metadata{"not": "nil"}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	_, err = blobURL.SetMetadata(ctx, nil, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.NewMetadata(), chk.HasLen, 0)
}

func (s *aztestsSuite) TestBlobSetMetadataEmpty(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.SetMetadata(ctx, azblob.Metadata{"not": "nil"}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	_, err = blobURL.SetMetadata(ctx, azblob.Metadata{}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.NewMetadata(), chk.HasLen, 0)
}

func (s *aztestsSuite) TestBlobSetMetadataInvalidField(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.SetMetadata(ctx, azblob.Metadata{"Invalid field!": "value"}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.NotNil)
	c.Assert(strings.Contains(err.Error(), validationErrorSubstring), chk.Equals, true)
}

func validateMetadataSet(c *chk.C, blobURL azblob.BlockBlobURL) {
	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.NewMetadata(), chk.DeepEquals, basicMetadata)
}

func (s *aztestsSuite) TestBlobSetMetadataIfModifiedSinceTrue(c *chk.C) {
	currentTime := getRelativeTimeGMT(-10)

	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.SetMetadata(ctx, basicMetadata,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	validateMetadataSet(c, blobURL)
}

func (s *aztestsSuite) TestBlobSetMetadataIfModifiedSinceFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	_, err := blobURL.SetMetadata(ctx, basicMetadata,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobSetMetadataIfUnmodifiedSinceTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	_, err := blobURL.SetMetadata(ctx, basicMetadata,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	validateMetadataSet(c, blobURL)
}

func (s *aztestsSuite) TestBlobSetMetadataIfUnmodifiedSinceFalse(c *chk.C) {
	currentTime := getRelativeTimeGMT(-10)

	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.SetMetadata(ctx, basicMetadata,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobSetMetadataIfMatchTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	_, err = blobURL.SetMetadata(ctx, basicMetadata,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: resp.ETag()}})
	c.Assert(err, chk.IsNil)

	validateMetadataSet(c, blobURL)
}

func (s *aztestsSuite) TestBlobSetMetadataIfMatchFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.SetMetadata(ctx, basicMetadata,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: azblob.ETag("garbage")}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobSetMetadataIfNoneMatchTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	_, err := blobURL.SetMetadata(ctx, basicMetadata,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: azblob.ETag("garbage")}})
	c.Assert(err, chk.IsNil)

	validateMetadataSet(c, blobURL)
}

func (s *aztestsSuite) TestBlobSetMetadataIfNoneMatchFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewBlockBlob(c, containerURL)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	_, err = blobURL.SetMetadata(ctx, basicMetadata,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: resp.ETag()}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobGetBlockListNone(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getBlockBlobURL(c, containerURL)

	_, err := blobURL.PutBlock(ctx, "0000", strings.NewReader(blockBlobDefaultData), azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := blobURL.GetBlockList(ctx, azblob.BlockListNone, azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.CommittedBlocks, chk.HasLen, 0)
	c.Assert(resp.UncommittedBlocks, chk.HasLen, 0) // Not specifying a block list type should default to only returning committed blocks
}

func (s *aztestsSuite) TestBlobGetBlockListUncommitted(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getBlockBlobURL(c, containerURL)

	_, err := blobURL.PutBlock(ctx, "0000", strings.NewReader(blockBlobDefaultData), azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := blobURL.GetBlockList(ctx, azblob.BlockListUncommitted, azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.CommittedBlocks, chk.HasLen, 0)
	c.Assert(resp.UncommittedBlocks, chk.HasLen, 1)
}

func (s *aztestsSuite) TestBlobGetBlockListCommitted(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getBlockBlobURL(c, containerURL)

	_, err := blobURL.PutBlock(ctx, "0000", strings.NewReader(blockBlobDefaultData), azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)

	_, err = blobURL.PutBlockList(ctx, []string{"0000"}, azblob.BlobHTTPHeaders{}, nil, azblob.BlobAccessConditions{})

	resp, err := blobURL.GetBlockList(ctx, azblob.BlockListCommitted, azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.CommittedBlocks, chk.HasLen, 1)
	c.Assert(resp.UncommittedBlocks, chk.HasLen, 0)
}

func (s *aztestsSuite) TestBlobGetBlockListCommittedEmpty(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getBlockBlobURL(c, containerURL)

	_, err := blobURL.PutBlock(ctx, "0000", strings.NewReader(blockBlobDefaultData), azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := blobURL.GetBlockList(ctx, azblob.BlockListCommitted, azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.CommittedBlocks, chk.HasLen, 0)
	c.Assert(resp.UncommittedBlocks, chk.HasLen, 0)
}

func (s *aztestsSuite) TestBlobGetBlockListBothEmpty(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getBlockBlobURL(c, containerURL)

	_, err := blobURL.GetBlockList(ctx, azblob.BlockListAll, azblob.LeaseAccessConditions{})
	validateStorageError(c, err, azblob.ServiceCodeBlobNotFound)
}

func (s *aztestsSuite) TestBlobGetBlockListBothNotEmpty(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getBlockBlobURL(c, containerURL)

	// Put and commit two blocks
	_, err := blobURL.PutBlock(ctx, "0000", strings.NewReader(blockBlobDefaultData), azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	_, err = blobURL.PutBlock(ctx, "0001", strings.NewReader(blockBlobDefaultData), azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	_, err = blobURL.PutBlockList(ctx, []string{"0001", "0000"}, azblob.BlobHTTPHeaders{}, nil, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	// Put two uncommitted blocks
	_, err = blobURL.PutBlock(ctx, "0003", strings.NewReader(blockBlobDefaultData), azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	_, err = blobURL.PutBlock(ctx, "0002", strings.NewReader(blockBlobDefaultData), azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := blobURL.GetBlockList(ctx, azblob.BlockListAll, azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.CommittedBlocks[0].Name, chk.Equals, "0001")
	c.Assert(resp.CommittedBlocks[1].Name, chk.Equals, "0000")   // Committed blocks are returned in the order they are committed (in the commit list)
	c.Assert(resp.UncommittedBlocks[0].Name, chk.Equals, "0002") // Uncommitted blocks are returned in alphabetical order
	c.Assert(resp.UncommittedBlocks[1].Name, chk.Equals, "0003")
}

func (s *aztestsSuite) TestBlobGetBlockListInvalidType(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getBlockBlobURL(c, containerURL)

	_, err := blobURL.PutBlock(ctx, "0000", strings.NewReader(blockBlobDefaultData), azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)

	_, err = blobURL.GetBlockList(ctx, azblob.BlockListType("garbage"), azblob.LeaseAccessConditions{})
	validateStorageError(c, err, azblob.ServiceCodeInvalidQueryParameterValue)
}

func (s *aztestsSuite) TestBlobGetBlockListSnapshot(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getBlockBlobURL(c, containerURL)

	_, err := blobURL.PutBlock(ctx, "0000", strings.NewReader(blockBlobDefaultData), azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	_, err = blobURL.PutBlockList(ctx, []string{"0000"}, azblob.BlobHTTPHeaders{}, nil, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := blobURL.CreateSnapshot(ctx, nil, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	snapshotURL := blobURL.WithSnapshot(resp.Snapshot())

	resp2, err := snapshotURL.GetBlockList(ctx, azblob.BlockListAll, azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp2.CommittedBlocks, chk.HasLen, 1)
}

func (s *aztestsSuite) TestBlobPutBlockIDInvalidCharacters(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getBlockBlobURL(c, containerURL)

	_, err := blobURL.PutBlock(ctx, "!!", strings.NewReader(blockBlobDefaultData), azblob.LeaseAccessConditions{})
	validateStorageError(c, err, azblob.ServiceCodeInvalidQueryParameterValue)
}

func (s *aztestsSuite) TestBlobPutBlockIDInvalidLength(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getBlockBlobURL(c, containerURL)

	_, err := blobURL.PutBlock(ctx, "0000", strings.NewReader(blockBlobDefaultData), azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	_, err = blobURL.PutBlock(ctx, "00000000", strings.NewReader(blockBlobDefaultData), azblob.LeaseAccessConditions{})
	validateStorageError(c, err, azblob.ServiceCodeInvalidBlobOrBlock)
}

func (s *aztestsSuite) TestBlobPutBlockEmptyBody(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getBlockBlobURL(c, containerURL)

	_, err := blobURL.PutBlock(ctx, "0000", strings.NewReader(""), azblob.LeaseAccessConditions{})
	validateStorageError(c, err, azblob.ServiceCodeMissingContentLengthHeader) // The server does not accept getReaderToRandomBytes length = 0
}

func setupPutBlockListTest(c *chk.C) (containerURL azblob.ContainerURL, blobURL azblob.BlockBlobURL, id string) {
	bsu := getBSU()
	containerURL, _ = createNewContainer(c, bsu)
	blobURL, _ = getBlockBlobURL(c, containerURL)

	_, err := blobURL.PutBlock(ctx, "0000", strings.NewReader(blockBlobDefaultData), azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	id = "0000"
	return
}

func (s *aztestsSuite) TestBlobPutBlockListInvalidID(c *chk.C) {
	containerURL, blobURL, id := setupPutBlockListTest(c)
	defer deleteContainer(c, containerURL)

	_, err := blobURL.PutBlockList(ctx, []string{id[:2]}, azblob.BlobHTTPHeaders{}, nil, azblob.BlobAccessConditions{})
	validateStorageError(c, err, azblob.ServiceCodeInvalidBlockID)
}

func (s *aztestsSuite) TestBlobPutBlockListDuplicateBlocks(c *chk.C) {
	containerURL, blobURL, id := setupPutBlockListTest(c)
	defer deleteContainer(c, containerURL)

	_, err := blobURL.PutBlockList(ctx, []string{id, id}, azblob.BlobHTTPHeaders{}, nil, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := blobURL.GetBlockList(ctx, azblob.BlockListAll, azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.CommittedBlocks, chk.HasLen, 2)
}

func (s *aztestsSuite) TestBlobPutBlockListEmptyList(c *chk.C) {
	containerURL, blobURL, _ := setupPutBlockListTest(c)
	defer deleteContainer(c, containerURL)

	_, err := blobURL.PutBlockList(ctx, []string{}, azblob.BlobHTTPHeaders{}, azblob.Metadata{}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := blobURL.GetBlockList(ctx, azblob.BlockListAll, azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.CommittedBlocks, chk.HasLen, 0)
}

func (s *aztestsSuite) TestBlobPutBlockListMetadataEmpty(c *chk.C) {
	containerURL, blobURL, id := setupPutBlockListTest(c)
	defer deleteContainer(c, containerURL)

	_, err := blobURL.PutBlockList(ctx, []string{id}, azblob.BlobHTTPHeaders{}, azblob.Metadata{}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.NewMetadata(), chk.HasLen, 0)
}

func (s *aztestsSuite) TestBlobPutBlockListMetadataNonEmpty(c *chk.C) {
	containerURL, blobURL, id := setupPutBlockListTest(c)
	defer deleteContainer(c, containerURL)

	_, err := blobURL.PutBlockList(ctx, []string{id}, azblob.BlobHTTPHeaders{}, basicMetadata, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.NewMetadata(), chk.DeepEquals, basicMetadata)
}

func (s *aztestsSuite) TestBlobPutBlockListHTTPHeaders(c *chk.C) {
	containerURL, blobURL, id := setupPutBlockListTest(c)
	defer deleteContainer(c, containerURL)

	_, err := blobURL.PutBlockList(ctx, []string{id}, basicHeaders, nil, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, _ := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	h := resp.NewHTTPHeaders()
	c.Assert(h, chk.DeepEquals, basicHeaders)
}

func (s *aztestsSuite) TestBlobPutBlockListHTTPHeadersEmpty(c *chk.C) {
	containerURL, blobURL, id := setupPutBlockListTest(c)
	defer deleteContainer(c, containerURL)

	_, err := blobURL.PutBlockList(ctx, []string{id}, azblob.BlobHTTPHeaders{ContentDisposition: "my_disposition"}, nil, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	_, err = blobURL.PutBlockList(ctx, []string{id}, azblob.BlobHTTPHeaders{}, nil, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.ContentDisposition(), chk.Equals, "")
}

func validateBlobCommitted(c *chk.C, blobURL azblob.BlockBlobURL) {
	resp, err := blobURL.GetBlockList(ctx, azblob.BlockListAll, azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.CommittedBlocks, chk.HasLen, 1)
}

func (s *aztestsSuite) TestBlobPutBlockListIfModifiedSinceTrue(c *chk.C) {
	containerURL, blobURL, id := setupPutBlockListTest(c)
	defer deleteContainer(c, containerURL)
	_, err := blobURL.PutBlockList(ctx, []string{id}, azblob.BlobHTTPHeaders{}, nil, azblob.BlobAccessConditions{}) // The blob must actually exist to have a modifed time
	c.Assert(err, chk.IsNil)

	currentTime := getRelativeTimeGMT(-10)

	_, err = blobURL.PutBlockList(ctx, []string{id}, azblob.BlobHTTPHeaders{}, nil,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	validateBlobCommitted(c, blobURL)
}

func (s *aztestsSuite) TestBlobPutBlockListIfModifiedSinceFalse(c *chk.C) {
	containerURL, blobURL, id := setupPutBlockListTest(c)
	defer deleteContainer(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	_, err := blobURL.PutBlockList(ctx, []string{id}, azblob.BlobHTTPHeaders{}, nil,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobPutBlockListIfUnmodifiedSinceTrue(c *chk.C) {
	containerURL, blobURL, id := setupPutBlockListTest(c)
	defer deleteContainer(c, containerURL)
	_, err := blobURL.PutBlockList(ctx, []string{id}, azblob.BlobHTTPHeaders{}, nil, azblob.BlobAccessConditions{}) // The blob must actually exist to have a modifed time
	c.Assert(err, chk.IsNil)

	currentTime := getRelativeTimeGMT(10)

	_, err = blobURL.PutBlockList(ctx, []string{id}, azblob.BlobHTTPHeaders{}, nil,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	validateBlobCommitted(c, blobURL)
}

func (s *aztestsSuite) TestBlobPutBlockListIfUnmodifiedSinceFalse(c *chk.C) {
	containerURL, blobURL, id := setupPutBlockListTest(c)
	blobURL.PutBlockList(ctx, []string{id}, azblob.BlobHTTPHeaders{}, nil, azblob.BlobAccessConditions{}) // The blob must actually exist to have a modifed time
	defer deleteContainer(c, containerURL)

	currentTime := getRelativeTimeGMT(-10)

	_, err := blobURL.PutBlockList(ctx, []string{id}, azblob.BlobHTTPHeaders{}, nil,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})

	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobPutBlockListIfMatchTrue(c *chk.C) {
	containerURL, blobURL, id := setupPutBlockListTest(c)
	defer deleteContainer(c, containerURL)
	resp, err := blobURL.PutBlockList(ctx, []string{id}, azblob.BlobHTTPHeaders{}, nil, azblob.BlobAccessConditions{}) // The blob must actually exist to have a modifed time
	c.Assert(err, chk.IsNil)

	_, err = blobURL.PutBlockList(ctx, []string{id}, azblob.BlobHTTPHeaders{},nil, 
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: resp.ETag()}})
	c.Assert(err, chk.IsNil)

	validateBlobCommitted(c, blobURL)
}

func (s *aztestsSuite) TestBlobPutBlockListIfMatchFalse(c *chk.C) {
	containerURL, blobURL, id := setupPutBlockListTest(c)
	defer deleteContainer(c, containerURL)
	_, err := blobURL.PutBlockList(ctx, []string{id},  azblob.BlobHTTPHeaders{}, nil,azblob.BlobAccessConditions{}) // The blob must actually exist to have a modifed time
	c.Assert(err, chk.IsNil)

	_, err = blobURL.PutBlockList(ctx, []string{id},  azblob.BlobHTTPHeaders{},nil,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: azblob.ETag("garbage")}})

	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobPutBlockListIfNoneMatchTrue(c *chk.C) {
	containerURL, blobURL, id := setupPutBlockListTest(c)
	defer deleteContainer(c, containerURL)
	_, err := blobURL.PutBlockList(ctx, []string{id}, azblob.BlobHTTPHeaders{}, nil,azblob.BlobAccessConditions{}) // The blob must actually exist to have a modifed time
	c.Assert(err, chk.IsNil)

	_, err = blobURL.PutBlockList(ctx, []string{id}, azblob.BlobHTTPHeaders{},nil,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: azblob.ETag("garbage")}})
	c.Assert(err, chk.IsNil)

	validateBlobCommitted(c, blobURL)
}

func (s *aztestsSuite) TestBlobPutBlockListIfNoneMatchFalse(c *chk.C) {
	containerURL, blobURL, id := setupPutBlockListTest(c)
	defer deleteContainer(c, containerURL)
	resp, err := blobURL.PutBlockList(ctx, []string{id},  azblob.BlobHTTPHeaders{}, nil,azblob.BlobAccessConditions{}) // The blob must actually exist to have a modifed time
	c.Assert(err, chk.IsNil)

	_, err = blobURL.PutBlockList(ctx, []string{id}, azblob.BlobHTTPHeaders{},nil,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: resp.ETag()}})

	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobPutBlockListValidateData(c *chk.C) {
	containerURL, blobURL, id := setupPutBlockListTest(c)
	defer deleteContainer(c, containerURL)

	_, err := blobURL.PutBlockList(ctx, []string{id},  azblob.BlobHTTPHeaders{}, nil,azblob.BlobAccessConditions{})

	resp, err := blobURL.GetBlob(ctx, azblob.BlobRange{}, azblob.BlobAccessConditions{}, false)
	c.Assert(err, chk.IsNil)
	data, _ := ioutil.ReadAll(resp.Response().Body)
	c.Assert(string(data), chk.Equals, blockBlobDefaultData)
}

func (s *aztestsSuite) TestBlobPutBlockListModifyBlob(c *chk.C) {
	containerURL, blobURL, id := setupPutBlockListTest(c)
	defer deleteContainer(c, containerURL)

	_, err := blobURL.PutBlockList(ctx, []string{id},  azblob.BlobHTTPHeaders{}, nil,azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	_, err = blobURL.PutBlock(ctx, "0001", bytes.NewReader([]byte("new data")), azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	_, err = blobURL.PutBlock(ctx, "0010", bytes.NewReader([]byte("new data")), azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	_, err = blobURL.PutBlock(ctx, "0011", bytes.NewReader([]byte("new data")), azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	_, err = blobURL.PutBlock(ctx, "0100", bytes.NewReader([]byte("new data")), azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)

	_, err = blobURL.PutBlockList(ctx, []string{"0001", "0011"},  azblob.BlobHTTPHeaders{}, nil,azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := blobURL.GetBlockList(ctx, azblob.BlockListAll, azblob.LeaseAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.CommittedBlocks, chk.HasLen, 2)
	c.Assert(resp.CommittedBlocks[0].Name, chk.Equals, "0001")
	c.Assert(resp.CommittedBlocks[1].Name, chk.Equals, "0011")
	c.Assert(resp.UncommittedBlocks, chk.HasLen, 0)
}

func (s *aztestsSuite) TestBlobCreateAppendMetadataNonEmpty(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getAppendBlobURL(c, containerURL)

	_, err := blobURL.Create(ctx, azblob.BlobHTTPHeaders{}, basicMetadata,azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.NewMetadata(), chk.DeepEquals, basicMetadata)
}

func (s *aztestsSuite) TestBlobCreateAppendMetadataEmpty(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getAppendBlobURL(c, containerURL)

	_, err := blobURL.Create(ctx,  azblob.BlobHTTPHeaders{}, azblob.Metadata{},azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.NewMetadata(), chk.HasLen, 0)
}

func (s *aztestsSuite) TestBlobCreateAppendMetadataInvalid(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getAppendBlobURL(c, containerURL)

	_, err := blobURL.Create(ctx, azblob.BlobHTTPHeaders{}, azblob.Metadata{"In valid!": "bar"}, azblob.BlobAccessConditions{})
	c.Assert(strings.Contains(err.Error(), validationErrorSubstring), chk.Equals, true)
}

func (s *aztestsSuite) TestBlobCreateAppendHTTPHeaders(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getAppendBlobURL(c, containerURL)

	_, err := blobURL.Create(ctx, basicHeaders, nil, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	h := resp.NewHTTPHeaders()
	c.Assert(h, chk.DeepEquals, basicHeaders)
}

func validateAppendBlobPut(c *chk.C, blobURL azblob.AppendBlobURL) {
	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.NewMetadata(), chk.DeepEquals, basicMetadata)
}

func (s *aztestsSuite) TestBlobCreateAppendIfModifiedSinceTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewAppendBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(-10)

	_, err := blobURL.Create(ctx,  azblob.BlobHTTPHeaders{},basicMetadata,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	validateAppendBlobPut(c, blobURL)
}

func (s *aztestsSuite) TestBlobCreateAppendIfModifiedSinceFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewAppendBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	_, err := blobURL.Create(ctx,  azblob.BlobHTTPHeaders{},basicMetadata,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobCreateAppendIfUnmodifiedSinceTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewAppendBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	_, err := blobURL.Create(ctx,  azblob.BlobHTTPHeaders{},basicMetadata,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	validateAppendBlobPut(c, blobURL)
}

func (s *aztestsSuite) TestBlobCreateAppendIfUnmodifiedSinceFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewAppendBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(-10)

	_, err := blobURL.Create(ctx, azblob.BlobHTTPHeaders{},basicMetadata,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobCreateAppendIfMatchTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewAppendBlob(c, containerURL)

	resp, _ := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})

	_, err := blobURL.Create(ctx,  azblob.BlobHTTPHeaders{},basicMetadata,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: resp.ETag()}})
	c.Assert(err, chk.IsNil)

	validateAppendBlobPut(c, blobURL)
}

func (s *aztestsSuite) TestBlobCreateAppendIfMatchFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewAppendBlob(c, containerURL)

	_, err := blobURL.Create(ctx, azblob.BlobHTTPHeaders{},basicMetadata,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: azblob.ETag("garbage")}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobCreateAppendIfNoneMatchTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewAppendBlob(c, containerURL)

	_, err := blobURL.Create(ctx, azblob.BlobHTTPHeaders{},basicMetadata,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: azblob.ETag("garbage")}})
	c.Assert(err, chk.IsNil)

	validateAppendBlobPut(c, blobURL)
}

func (s *aztestsSuite) TestBlobCreateAppendIfNoneMatchFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewAppendBlob(c, containerURL)

	resp, _ := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})

	_, err := blobURL.Create(ctx,  azblob.BlobHTTPHeaders{},basicMetadata,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: resp.ETag()}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobAppendBlockNilBody(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewAppendBlob(c, containerURL)

	_, err := blobURL.AppendBlock(ctx, nil, azblob.BlobAccessConditions{})
	c.Assert(err, chk.NotNil)
	c.Assert(strings.Contains(err.Error(), validationErrorSubstring), chk.Equals, true)
}

func (s *aztestsSuite) TestBlobAppendBlockEmptyBody(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewAppendBlob(c, containerURL)

	_, err := blobURL.AppendBlock(ctx, strings.NewReader(""), azblob.BlobAccessConditions{})
	validateStorageError(c, err, azblob.ServiceCodeMissingContentLengthHeader) // The server does not accept getReaderToRandomBytes length = 0
}

func (s *aztestsSuite) TestBlobAppendBlockNonExistantBlob(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getAppendBlobURL(c, containerURL)

	_, err := blobURL.AppendBlock(ctx, strings.NewReader(blockBlobDefaultData), azblob.BlobAccessConditions{})
	validateStorageError(c, err, azblob.ServiceCodeBlobNotFound)
}

func validateBlockAppended(c *chk.C, blobURL azblob.AppendBlobURL, expectedSize int) {
	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.ContentLength(), chk.Equals, int64(expectedSize))
}

func (s *aztestsSuite) TestBlobAppendBlockIfModifiedSinceTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewAppendBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(-10)

	_, err := blobURL.AppendBlock(ctx, strings.NewReader(blockBlobDefaultData),
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	validateBlockAppended(c, blobURL, len(blockBlobDefaultData))
}

func (s *aztestsSuite) TestBlobAppendBlockIfModifiedSinceFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewAppendBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(10)
	_, err := blobURL.AppendBlock(ctx, strings.NewReader(blockBlobDefaultData),
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobAppendBlockIfUnmodifiedSinceTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewAppendBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(10)
	_, err := blobURL.AppendBlock(ctx, strings.NewReader(blockBlobDefaultData),
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	validateBlockAppended(c, blobURL, len(blockBlobDefaultData))
}

func (s *aztestsSuite) TestBlobAppendBlockIfUnmodifiedSinceFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewAppendBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(-10)
	_, err := blobURL.AppendBlock(ctx, strings.NewReader(blockBlobDefaultData),
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobAppendBlockIfMatchTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewAppendBlob(c, containerURL)

	resp, _ := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})

	_, err := blobURL.AppendBlock(ctx, strings.NewReader(blockBlobDefaultData),
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: resp.ETag()}})
	c.Assert(err, chk.IsNil)

	validateBlockAppended(c, blobURL, len(blockBlobDefaultData))
}

func (s *aztestsSuite) TestBlobAppendBlockIfMatchFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewAppendBlob(c, containerURL)

	_, err := blobURL.AppendBlock(ctx, strings.NewReader(blockBlobDefaultData),
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: azblob.ETag("garbage")}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobAppendBlockIfNoneMatchTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewAppendBlob(c, containerURL)

	_, err := blobURL.AppendBlock(ctx, strings.NewReader(blockBlobDefaultData),
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: azblob.ETag("garbage")}})
	c.Assert(err, chk.IsNil)

	validateBlockAppended(c, blobURL, len(blockBlobDefaultData))
}

func (s *aztestsSuite) TestBlobAppendBlockIfNoneMatchFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewAppendBlob(c, containerURL)

	resp, _ := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})

	_, err := blobURL.AppendBlock(ctx, strings.NewReader(blockBlobDefaultData),
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: resp.ETag()}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobAppendBlockIfAppendPositionMatchTrueNegOne(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewAppendBlob(c, containerURL)

	_, err := blobURL.AppendBlock(ctx, strings.NewReader(blockBlobDefaultData),
		azblob.BlobAccessConditions{AppendBlobAccessConditions: azblob.AppendBlobAccessConditions{IfAppendPositionEqual: -1}}) // This will cause the library to set the value of the header to 0
	c.Assert(err, chk.IsNil)

	validateBlockAppended(c, blobURL, len(blockBlobDefaultData))
}

func (s *aztestsSuite) TestBlobAppendBlockIfAppendPositionMatchZero(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewAppendBlob(c, containerURL)

	_, err := blobURL.AppendBlock(ctx, strings.NewReader(blockBlobDefaultData), azblob.BlobAccessConditions{}) // The position will not match, but the condition should be ignored
	c.Assert(err, chk.IsNil)
	_, err = blobURL.AppendBlock(ctx, strings.NewReader(blockBlobDefaultData),
		azblob.BlobAccessConditions{AppendBlobAccessConditions: azblob.AppendBlobAccessConditions{IfAppendPositionEqual: 0}})
	c.Assert(err, chk.IsNil)

	validateBlockAppended(c, blobURL, 2*len(blockBlobDefaultData))
}

func (s *aztestsSuite) TestBlobAppendBlockIfAppendPositionMatchTrueNonZero(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewAppendBlob(c, containerURL)

	_, err := blobURL.AppendBlock(ctx, strings.NewReader(blockBlobDefaultData), azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	_, err = blobURL.AppendBlock(ctx, strings.NewReader(blockBlobDefaultData),
		azblob.BlobAccessConditions{AppendBlobAccessConditions: azblob.AppendBlobAccessConditions{IfAppendPositionEqual: int32(len(blockBlobDefaultData))}})
	c.Assert(err, chk.IsNil)

	validateBlockAppended(c, blobURL, len(blockBlobDefaultData)*2)
}

func (s *aztestsSuite) TestBlobAppendBlockIfAppendPositionMatchFalseNegOne(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewAppendBlob(c, containerURL)

	_, err := blobURL.AppendBlock(ctx, strings.NewReader(blockBlobDefaultData), azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	_, err = blobURL.AppendBlock(ctx, strings.NewReader(blockBlobDefaultData),
		azblob.BlobAccessConditions{AppendBlobAccessConditions: azblob.AppendBlobAccessConditions{IfAppendPositionEqual: -1}}) // This will cause the library to set the value of the header to 0
	validateStorageError(c, err, azblob.ServiceCodeAppendPositionConditionNotMet)
}

func (s *aztestsSuite) TestBlobAppendBlockIfAppendPositionMatchFalseNonZero(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewAppendBlob(c, containerURL)

	_, err := blobURL.AppendBlock(ctx, strings.NewReader(blockBlobDefaultData),
		azblob.BlobAccessConditions{AppendBlobAccessConditions: azblob.AppendBlobAccessConditions{IfAppendPositionEqual: 12}})
	validateStorageError(c, err, azblob.ServiceCodeAppendPositionConditionNotMet)
}

func (s *aztestsSuite) TestBlobAppendBlockIfMaxSizeTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewAppendBlob(c, containerURL)

	_, err := blobURL.AppendBlock(ctx, strings.NewReader(blockBlobDefaultData),
		azblob.BlobAccessConditions{AppendBlobAccessConditions: azblob.AppendBlobAccessConditions{IfMaxSizeLessThanOrEqual: int32(len(blockBlobDefaultData) + 1)}})
	c.Assert(err, chk.IsNil)

	validateBlockAppended(c, blobURL, len(blockBlobDefaultData))
}

func (s *aztestsSuite) TestBlobAppendBlockIfMaxSizeFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewAppendBlob(c, containerURL)

	_, err := blobURL.AppendBlock(ctx, strings.NewReader(blockBlobDefaultData),
		azblob.BlobAccessConditions{AppendBlobAccessConditions: azblob.AppendBlobAccessConditions{IfMaxSizeLessThanOrEqual: int32(len(blockBlobDefaultData) - 1)}})
	validateStorageError(c, err, azblob.ServiceCodeMaxBlobSizeConditionNotMet)
}

func (s *aztestsSuite) TestBlobCreatePageSizeInvalid(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getPageBlobURL(c, containerURL)

	_, err := blobURL.Create(ctx, 1, 0,  azblob.BlobHTTPHeaders{}, nil, azblob.BlobAccessConditions{})
	validateStorageError(c, err, azblob.ServiceCodeInvalidHeaderValue)
}

func (s *aztestsSuite) TestBlobCreatePageSequenceInvalid(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getPageBlobURL(c, containerURL)

	// Negative sequenceNumber should cause a panic
	defer func() {
		recover()
	}()

	blobURL.Create(ctx, azblob.PageBlobPageBytes, -1, azblob.BlobHTTPHeaders{}, nil, azblob.BlobAccessConditions{})
	c.Fail()

}

func (s *aztestsSuite) TestBlobCreatePageMetadataNonEmpty(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getPageBlobURL(c, containerURL)

	_, err := blobURL.Create(ctx, azblob.PageBlobPageBytes, 0,  azblob.BlobHTTPHeaders{}, basicMetadata,azblob.BlobAccessConditions{})

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.NewMetadata(), chk.DeepEquals, basicMetadata)
}

func (s *aztestsSuite) TestBlobCreatePageMetadataEmpty(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getPageBlobURL(c, containerURL)

	_, err := blobURL.Create(ctx, azblob.PageBlobPageBytes, 0, azblob.BlobHTTPHeaders{}, azblob.Metadata{}, azblob.BlobAccessConditions{})

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.NewMetadata(), chk.HasLen, 0)
}

func (s *aztestsSuite) TestBlobCreatePageMetadataInvalid(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getPageBlobURL(c, containerURL)

	_, err := blobURL.Create(ctx, azblob.PageBlobPageBytes, 0, azblob.BlobHTTPHeaders{}, azblob.Metadata{"In valid1": "bar"}, azblob.BlobAccessConditions{})
	c.Assert(strings.Contains(err.Error(), validationErrorSubstring), chk.Equals, true)

}

func (s *aztestsSuite) TestBlobCreatePageHTTPHeaders(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getPageBlobURL(c, containerURL)

	_, err := blobURL.Create(ctx, azblob.PageBlobPageBytes, 0,  basicHeaders, nil, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	h := resp.NewHTTPHeaders()
	c.Assert(h, chk.DeepEquals, basicHeaders)
}

func validatePageBlobPut(c *chk.C, blobURL azblob.PageBlobURL) {
	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.NewMetadata(), chk.DeepEquals, basicMetadata)
}

func (s *aztestsSuite) TestBlobCreatePageIfModifiedSinceTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL) // Originally created without metadata

	currentTime := getRelativeTimeGMT(-10)

	_, err := blobURL.Create(ctx, azblob.PageBlobPageBytes, 0, azblob.BlobHTTPHeaders{}, basicMetadata,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	validatePageBlobPut(c, blobURL)
}

func (s *aztestsSuite) TestBlobCreatePageIfModifiedSinceFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL) // Originally created without metadata

	currentTime := getRelativeTimeGMT(10)

	_, err := blobURL.Create(ctx, azblob.PageBlobPageBytes, 0,  azblob.BlobHTTPHeaders{},basicMetadata,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobCreatePageIfUnmodifiedSinceTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL) // Originally created without metadata

	currentTime := getRelativeTimeGMT(10)

	_, err := blobURL.Create(ctx, azblob.PageBlobPageBytes, 0,  azblob.BlobHTTPHeaders{},basicMetadata,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	validatePageBlobPut(c, blobURL)
}

func (s *aztestsSuite) TestBlobCreatePageIfUnmodifiedSinceFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL) // Originally created without metadata

	currentTime := getRelativeTimeGMT(-10)

	_, err := blobURL.Create(ctx, azblob.PageBlobPageBytes, 0,  azblob.BlobHTTPHeaders{},basicMetadata,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobCreatePageIfMatchTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL) // Originally created without metadata

	resp, _ := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})

	_, err := blobURL.Create(ctx, azblob.PageBlobPageBytes, 0, azblob.BlobHTTPHeaders{},basicMetadata, 
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: resp.ETag()}})
	c.Assert(err, chk.IsNil)

	validatePageBlobPut(c, blobURL)
}

func (s *aztestsSuite) TestBlobCreatePageIfMatchFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL) // Originally created without metadata

	_, err := blobURL.Create(ctx, azblob.PageBlobPageBytes, 0,  azblob.BlobHTTPHeaders{},basicMetadata,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: azblob.ETag("garbage")}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobCreatePageIfNoneMatchTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL) // Originally created without metadata

	_, err := blobURL.Create(ctx, azblob.PageBlobPageBytes, 0, azblob.BlobHTTPHeaders{},basicMetadata,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: azblob.ETag("garbage")}})
	c.Assert(err, chk.IsNil)

	validatePageBlobPut(c, blobURL)
}

func (s *aztestsSuite) TestBlobCreatePageIfNoneMatchFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL) // Originally created without metadata

	resp, _ := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})

	_, err := blobURL.Create(ctx, azblob.PageBlobPageBytes, 0,  azblob.BlobHTTPHeaders{},basicMetadata,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: resp.ETag()}})

	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobPutPagesInvalidRange(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	defer func() { // The library should panic if the page range is invalid in any way
		recover()
	}()

	blobURL.PutPages(ctx, azblob.PageRange{Start: 0, End: int32(len(blockBlobDefaultData))}, strings.NewReader(blockBlobDefaultData),
		azblob.BlobAccessConditions{})
	c.Fail()
}

func (s *aztestsSuite) TestBlobPutPagesNilBody(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	// A page range that starts and ends at 0 should panic
	defer func() {
		recover()
	}()

	blobURL.PutPages(ctx, azblob.PageRange{Start: 0, End: 0}, nil, azblob.BlobAccessConditions{})
	c.Fail()
}

func (s *aztestsSuite) TestBlobPutPagesEmptyBody(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	// A page range that starts and ends at 0 should panic
	defer func() {
		recover()
	}()

	blobURL.PutPages(ctx, azblob.PageRange{Start: 0, End: 0}, bytes.NewReader([]byte{}), azblob.BlobAccessConditions{})
	c.Fail()
}

func (s *aztestsSuite) TestBlobPutPagesNonExistantBlob(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := getPageBlobURL(c, containerURL)

	_, err := blobURL.PutPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1}, getReaderToRandomBytes(azblob.PageBlobPageBytes), azblob.BlobAccessConditions{})
	validateStorageError(c, err, azblob.ServiceCodeBlobNotFound)
}

func validatePutPages(c *chk.C, blobURL azblob.PageBlobURL) {
	// This will only validate a single put page at 0-azblob.PageBlobPageBytes-1
	resp, err := blobURL.GetPageRanges(ctx, azblob.BlobRange{}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.PageRange[0], chk.Equals, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1})
}

func (s *aztestsSuite) TestBlobPutPagesIfModifiedSinceTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(-10)

	_, err := blobURL.PutPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1}, getReaderToRandomBytes(azblob.PageBlobPageBytes),
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	validatePutPages(c, blobURL)
}

func (s *aztestsSuite) TestBlobPutPagesIfModifiedSinceFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	_, err := blobURL.PutPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1}, getReaderToRandomBytes(azblob.PageBlobPageBytes),
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobPutPagesIfUnmodifiedSinceTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	_, err := blobURL.PutPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1}, getReaderToRandomBytes(azblob.PageBlobPageBytes),
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	validatePutPages(c, blobURL)
}

func (s *aztestsSuite) TestBlobPutPagesIfUnmodifiedSinceFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(-10)

	_, err := blobURL.PutPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1}, getReaderToRandomBytes(azblob.PageBlobPageBytes),
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobPutPagesIfMatchTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	resp, _ := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})

	_, err := blobURL.PutPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1}, getReaderToRandomBytes(azblob.PageBlobPageBytes),
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: resp.ETag()}})
	c.Assert(err, chk.IsNil)

	validatePutPages(c, blobURL)
}

func (s *aztestsSuite) TestBlobPutPagesIfMatchFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	_, err := blobURL.PutPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1}, getReaderToRandomBytes(azblob.PageBlobPageBytes),
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: azblob.ETag("garbage")}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobPutPagesIfNoneMatchTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	_, err := blobURL.PutPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1}, getReaderToRandomBytes(azblob.PageBlobPageBytes),
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: azblob.ETag("garbage")}})
	c.Assert(err, chk.IsNil)

	validatePutPages(c, blobURL)
}

func (s *aztestsSuite) TestBlobPutPagesIfNoneMatchFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	resp, _ := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})

	_, err := blobURL.PutPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1}, getReaderToRandomBytes(azblob.PageBlobPageBytes),
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: resp.ETag()}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobPutPagesIfSequenceNumberLessThanTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	_, err := blobURL.PutPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1}, getReaderToRandomBytes(azblob.PageBlobPageBytes),
		azblob.BlobAccessConditions{PageBlobAccessConditions: azblob.PageBlobAccessConditions{IfSequenceNumberLessThan: 10}})
	c.Assert(err, chk.IsNil)

	validatePutPages(c, blobURL)
}

func (s *aztestsSuite) TestBlobPutPagesIfSequenceNumberLessThanFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	blobURL.SetSequenceNumber(ctx, azblob.SequenceNumberActionUpdate, 10, azblob.BlobHTTPHeaders{}, azblob.BlobAccessConditions{})
	_, err := blobURL.PutPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1}, getReaderToRandomBytes(azblob.PageBlobPageBytes),
		azblob.BlobAccessConditions{PageBlobAccessConditions: azblob.PageBlobAccessConditions{IfSequenceNumberLessThan: 1}})
	validateStorageError(c, err, azblob.ServiceCodeSequenceNumberConditionNotMet)
}

func (s *aztestsSuite) TestBlobPutPagesIfSequenceNumberLessThanNegOne(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	_, err := blobURL.PutPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1}, getReaderToRandomBytes(azblob.PageBlobPageBytes),
		azblob.BlobAccessConditions{PageBlobAccessConditions: azblob.PageBlobAccessConditions{IfSequenceNumberLessThan: -1}}) // This will cause the library to set the value of the header to 0
	validateStorageError(c, err, azblob.ServiceCodeSequenceNumberConditionNotMet)
}

func (s *aztestsSuite) TestBlobPutPagesIfSequenceNumberLTETrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	blobURL.SetSequenceNumber(ctx, azblob.SequenceNumberActionUpdate, 1, azblob.BlobHTTPHeaders{}, azblob.BlobAccessConditions{})
	_, err := blobURL.PutPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1}, getReaderToRandomBytes(azblob.PageBlobPageBytes),
		azblob.BlobAccessConditions{PageBlobAccessConditions: azblob.PageBlobAccessConditions{IfSequenceNumberLessThanOrEqual: 1}})
	c.Assert(err, chk.IsNil)

	validatePutPages(c, blobURL)
}

func (s *aztestsSuite) TestBlobPutPagesIfSequenceNumberLTEqualFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	blobURL.SetSequenceNumber(ctx, azblob.SequenceNumberActionUpdate, 10, azblob.BlobHTTPHeaders{}, azblob.BlobAccessConditions{})
	_, err := blobURL.PutPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1}, getReaderToRandomBytes(azblob.PageBlobPageBytes),
		azblob.BlobAccessConditions{PageBlobAccessConditions: azblob.PageBlobAccessConditions{IfSequenceNumberLessThanOrEqual: 1}})
	validateStorageError(c, err, azblob.ServiceCodeSequenceNumberConditionNotMet)
}

func (s *aztestsSuite) TestBlobPutPagesIfSequenceNumberLTENegOne(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	_, err := blobURL.PutPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1}, getReaderToRandomBytes(azblob.PageBlobPageBytes),
		azblob.BlobAccessConditions{PageBlobAccessConditions: azblob.PageBlobAccessConditions{IfSequenceNumberLessThanOrEqual: -1}}) // This will cause the library to set the value of the header to 0
	c.Assert(err, chk.IsNil)

	validatePutPages(c, blobURL)
}

func (s *aztestsSuite) TestBlobPutPagesIfSequenceNumberEqualTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	blobURL.SetSequenceNumber(ctx, azblob.SequenceNumberActionUpdate, 1, azblob.BlobHTTPHeaders{}, azblob.BlobAccessConditions{})
	_, err := blobURL.PutPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1}, getReaderToRandomBytes(azblob.PageBlobPageBytes),
		azblob.BlobAccessConditions{PageBlobAccessConditions: azblob.PageBlobAccessConditions{IfSequenceNumberEqual: 1}})
	c.Assert(err, chk.IsNil)

	validatePutPages(c, blobURL)
}

func (s *aztestsSuite) TestBlobPutPagesIfSequenceNumberEqualFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	_, err := blobURL.PutPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1}, getReaderToRandomBytes(azblob.PageBlobPageBytes),
		azblob.BlobAccessConditions{PageBlobAccessConditions: azblob.PageBlobAccessConditions{IfSequenceNumberEqual: 1}})
	validateStorageError(c, err, azblob.ServiceCodeSequenceNumberConditionNotMet)
}

func (s *aztestsSuite) TestBlobPutPagesIfSequenceNumberEqualNegOne(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	_, err := blobURL.PutPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1}, getReaderToRandomBytes(azblob.PageBlobPageBytes),
		azblob.BlobAccessConditions{PageBlobAccessConditions: azblob.PageBlobAccessConditions{IfSequenceNumberEqual: -1}}) // This will cause the library to set the value of the header to 0
	c.Assert(err, chk.IsNil)

	validatePutPages(c, blobURL)
}

func setupClearPagesTest(c *chk.C) (azblob.ContainerURL, azblob.PageBlobURL) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	blobURL, _ := createNewPageBlob(c, containerURL)

	_, err := blobURL.PutPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1}, getReaderToRandomBytes(azblob.PageBlobPageBytes), azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	return containerURL, blobURL
}

func validateClearPagesTest(c *chk.C, blobURL azblob.PageBlobURL) {
	resp, err := blobURL.GetPageRanges(ctx, azblob.BlobRange{}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.PageRange, chk.HasLen, 0)
}

func (s *aztestsSuite) TestBlobClearPagesInvalidRange(c *chk.C) {
	containerURL, blobURL := setupClearPagesTest(c)
	defer deleteContainer(c, containerURL)

	// A misaligned page range will panic (End is set to n*512 instead of (n*512)-1 as is required)
	defer func() {
		recover()
	}()

	blobURL.ClearPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes}, azblob.BlobAccessConditions{})
	c.Fail()
}

func (s *aztestsSuite) TestBlobClearPagesIfModifiedSinceTrue(c *chk.C) {
	containerURL, blobURL := setupClearPagesTest(c)
	defer deleteContainer(c, containerURL)

	currentTime := getRelativeTimeGMT(-10)

	_, err := blobURL.ClearPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	validateClearPagesTest(c, blobURL)
}

func (s *aztestsSuite) TestBlobClearPagesIfModifiedSinceFalse(c *chk.C) {
	containerURL, blobURL := setupClearPagesTest(c)
	defer deleteContainer(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	_, err := blobURL.ClearPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobClearPagesIfUnmodifiedSinceTrue(c *chk.C) {
	containerURL, blobURL := setupClearPagesTest(c)
	defer deleteContainer(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	_, err := blobURL.ClearPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	validateClearPagesTest(c, blobURL)
}

func (s *aztestsSuite) TestBlobClearPagesIfUnmodifiedSinceFalse(c *chk.C) {
	containerURL, blobURL := setupClearPagesTest(c)
	defer deleteContainer(c, containerURL)

	currentTime := getRelativeTimeGMT(-10)

	_, err := blobURL.ClearPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobClearPagesIfMatchTrue(c *chk.C) {
	containerURL, blobURL := setupClearPagesTest(c)
	defer deleteContainer(c, containerURL)

	resp, _ := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})

	_, err := blobURL.ClearPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: resp.ETag()}})
	c.Assert(err, chk.IsNil)

	validateClearPagesTest(c, blobURL)
}

func (s *aztestsSuite) TestBlobClearPagesIfMatchFalse(c *chk.C) {
	containerURL, blobURL := setupClearPagesTest(c)
	defer deleteContainer(c, containerURL)

	_, err := blobURL.ClearPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: azblob.ETag("garbage")}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobClearPagesIfNoneMatchTrue(c *chk.C) {
	containerURL, blobURL := setupClearPagesTest(c)
	defer deleteContainer(c, containerURL)

	_, err := blobURL.ClearPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: azblob.ETag("garbage")}})
	c.Assert(err, chk.IsNil)

	validateClearPagesTest(c, blobURL)
}

func (s *aztestsSuite) TestBlobClearPagesIfNoneMatchFalse(c *chk.C) {
	containerURL, blobURL := setupClearPagesTest(c)
	defer deleteContainer(c, containerURL)

	resp, _ := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})

	_, err := blobURL.ClearPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: resp.ETag()}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobClearPagesIfSequenceNumberLessThanTrue(c *chk.C) {
	containerURL, blobURL := setupClearPagesTest(c)
	defer deleteContainer(c, containerURL)

	_, err := blobURL.ClearPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1},
		azblob.BlobAccessConditions{PageBlobAccessConditions: azblob.PageBlobAccessConditions{IfSequenceNumberLessThan: 10}})
	c.Assert(err, chk.IsNil)

	validateClearPagesTest(c, blobURL)
}

func (s *aztestsSuite) TestBlobClearPagesIfSequenceNumberLessThanFalse(c *chk.C) {
	containerURL, blobURL := setupClearPagesTest(c)
	defer deleteContainer(c, containerURL)

	_, err := blobURL.SetSequenceNumber(ctx, azblob.SequenceNumberActionUpdate, 10, azblob.BlobHTTPHeaders{}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	_, err = blobURL.ClearPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1},
		azblob.BlobAccessConditions{PageBlobAccessConditions: azblob.PageBlobAccessConditions{IfSequenceNumberLessThan: 1}})
	validateStorageError(c, err, azblob.ServiceCodeSequenceNumberConditionNotMet)
}

func (s *aztestsSuite) TestBlobClearPagesIfSequenceNumberLessThanNegOne(c *chk.C) {
	containerURL, blobURL := setupClearPagesTest(c)
	defer deleteContainer(c, containerURL)

	_, err := blobURL.ClearPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1},
		azblob.BlobAccessConditions{PageBlobAccessConditions: azblob.PageBlobAccessConditions{IfSequenceNumberLessThan: -1}}) // This will cause the library to set the value of the header to 0
	validateStorageError(c, err, azblob.ServiceCodeSequenceNumberConditionNotMet)
}

func (s *aztestsSuite) TestBlobClearPagesIfSequenceNumberLTETrue(c *chk.C) {
	containerURL, blobURL := setupClearPagesTest(c)
	defer deleteContainer(c, containerURL)

	_, err := blobURL.ClearPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1},
		azblob.BlobAccessConditions{PageBlobAccessConditions: azblob.PageBlobAccessConditions{IfSequenceNumberLessThanOrEqual: 10}})
	c.Assert(err, chk.IsNil)

	validateClearPagesTest(c, blobURL)
}

func (s *aztestsSuite) TestBlobClearPagesIfSequenceNumberLTEFalse(c *chk.C) {
	containerURL, blobURL := setupClearPagesTest(c)
	defer deleteContainer(c, containerURL)

	_, err := blobURL.SetSequenceNumber(ctx, azblob.SequenceNumberActionUpdate, 10, azblob.BlobHTTPHeaders{}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	_, err = blobURL.ClearPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1},
		azblob.BlobAccessConditions{PageBlobAccessConditions: azblob.PageBlobAccessConditions{IfSequenceNumberLessThanOrEqual: 1}})
	validateStorageError(c, err, azblob.ServiceCodeSequenceNumberConditionNotMet)
}

func (s *aztestsSuite) TestBlobClearPagesIfSequenceNumberLTENegOne(c *chk.C) {
	containerURL, blobURL := setupClearPagesTest(c)
	defer deleteContainer(c, containerURL)

	_, err := blobURL.ClearPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1},
		azblob.BlobAccessConditions{PageBlobAccessConditions: azblob.PageBlobAccessConditions{IfSequenceNumberLessThanOrEqual: -1}}) // This will cause the library to set the value of the header to 0
	c.Assert(err, chk.IsNil)

	validateClearPagesTest(c, blobURL)
}

func (s *aztestsSuite) TestBlobClearPagesIfSequenceNumberEqualTrue(c *chk.C) {
	containerURL, blobURL := setupClearPagesTest(c)
	defer deleteContainer(c, containerURL)

	_, err := blobURL.SetSequenceNumber(ctx, azblob.SequenceNumberActionUpdate, 10, azblob.BlobHTTPHeaders{}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	_, err = blobURL.ClearPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1},
		azblob.BlobAccessConditions{PageBlobAccessConditions: azblob.PageBlobAccessConditions{IfSequenceNumberEqual: 10}})
	c.Assert(err, chk.IsNil)

	validateClearPagesTest(c, blobURL)
}

func (s *aztestsSuite) TestBlobClearPagesIfSequenceNumberEqualFalse(c *chk.C) {
	containerURL, blobURL := setupClearPagesTest(c)
	defer deleteContainer(c, containerURL)

	_, err := blobURL.SetSequenceNumber(ctx, azblob.SequenceNumberActionUpdate, 10, azblob.BlobHTTPHeaders{}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	_, err = blobURL.ClearPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1},
		azblob.BlobAccessConditions{PageBlobAccessConditions: azblob.PageBlobAccessConditions{IfSequenceNumberEqual: 1}})
	validateStorageError(c, err, azblob.ServiceCodeSequenceNumberConditionNotMet)
}

func (s *aztestsSuite) TestBlobClearPagesIfSequenceNumberEqualNegOne(c *chk.C) {
	containerURL, blobURL := setupClearPagesTest(c)
	defer deleteContainer(c, containerURL)

	_, err := blobURL.ClearPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1},
		azblob.BlobAccessConditions{PageBlobAccessConditions: azblob.PageBlobAccessConditions{IfSequenceNumberEqual: -1}}) // This will cause the library to set the value of the header to 0
	c.Assert(err, chk.IsNil)

	validateClearPagesTest(c, blobURL)
}

func setupGetPageRangesTest(c *chk.C) (containerURL azblob.ContainerURL, blobURL azblob.PageBlobURL) {
	bsu := getBSU()
	containerURL, _ = createNewContainer(c, bsu)
	blobURL, _ = createNewPageBlob(c, containerURL)

	_, err := blobURL.PutPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1}, getReaderToRandomBytes(azblob.PageBlobPageBytes), azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	return
}

func validateBasicGetPageRanges(c *chk.C, resp *azblob.PageList, err error) {
	c.Assert(err, chk.IsNil)
	c.Assert(resp.PageRange, chk.HasLen, 1)
	c.Assert(resp.PageRange[0], chk.Equals, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1})
}

func (s *aztestsSuite) TestBlobGetPageRangesEmptyBlob(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	resp, err := blobURL.GetPageRanges(ctx, azblob.BlobRange{}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.PageRange, chk.HasLen, 0)
}

func (s *aztestsSuite) TestBlobGetPageRangesEmptyRange(c *chk.C) {
	containerURL, blobURL := setupGetPageRangesTest(c)
	defer deleteContainer(c, containerURL)

	resp, err := blobURL.GetPageRanges(ctx, azblob.BlobRange{}, azblob.BlobAccessConditions{})
	validateBasicGetPageRanges(c, resp, err)
}

func (s *aztestsSuite) TestBlobGetPageRangesInvalidRange(c *chk.C) {
	containerURL, blobURL := setupGetPageRangesTest(c)
	defer deleteContainer(c, containerURL)

	defer func() { // Invalid blob range should panic
		recover()
	}()

	blobURL.GetPageRanges(ctx, azblob.BlobRange{Offset: -2, Count: 500}, azblob.BlobAccessConditions{})
	c.Fail()
}

func (s *aztestsSuite) TestBlobGetPageRangesNonContiguousRanges(c *chk.C) {
	containerURL, blobURL := setupGetPageRangesTest(c)
	defer deleteContainer(c, containerURL)

	_, err := blobURL.PutPages(ctx, azblob.PageRange{Start: azblob.PageBlobPageBytes * 2, End: (azblob.PageBlobPageBytes * 3) - 1}, getReaderToRandomBytes(azblob.PageBlobPageBytes), azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	resp, err := blobURL.GetPageRanges(ctx, azblob.BlobRange{}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.PageRange, chk.HasLen, 2)
	c.Assert(resp.PageRange[0], chk.Equals, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1})
	c.Assert(resp.PageRange[1], chk.Equals, azblob.PageRange{Start: azblob.PageBlobPageBytes * 2, End: (azblob.PageBlobPageBytes * 3) - 1})
}
func (s *aztestsSuite) TestblobGetPageRangesNotPageAligned(c *chk.C) {
	containerURL, blobURL := setupGetPageRangesTest(c)
	defer deleteContainer(c, containerURL)

	resp, err := blobURL.GetPageRanges(ctx, azblob.BlobRange{Offset: 0, Count: 2000}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	validateBasicGetPageRanges(c, resp, err)
}

func (s *aztestsSuite) TestBlobGetPageRangesSnapshot(c *chk.C) {
	containerURL, blobURL := setupGetPageRangesTest(c)
	defer deleteContainer(c, containerURL)

	resp, _ := blobURL.CreateSnapshot(ctx, nil, azblob.BlobAccessConditions{})
	snapshotURL := blobURL.WithSnapshot(resp.Snapshot())
	resp2, err := snapshotURL.GetPageRanges(ctx, azblob.BlobRange{}, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	validateBasicGetPageRanges(c, resp2, err)
}

func (s *aztestsSuite) TestBlobGetPageRangesIfModifiedSinceTrue(c *chk.C) {
	containerURL, blobURL := setupGetPageRangesTest(c)
	defer deleteContainer(c, containerURL)

	currentTime := getRelativeTimeGMT(-10)

	resp, err := blobURL.GetPageRanges(ctx, azblob.BlobRange{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	validateBasicGetPageRanges(c, resp, err)
}

func (s *aztestsSuite) TestBlobGetPageRangesIfModifiedSinceFalse(c *chk.C) {
	containerURL, blobURL := setupGetPageRangesTest(c)
	defer deleteContainer(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	_, err := blobURL.GetPageRanges(ctx, azblob.BlobRange{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	serr := err.(azblob.StorageError)
	c.Assert(serr.Response().StatusCode, chk.Equals, 304) // Service Code not returned in the body for a HEAD
}

func (s *aztestsSuite) TestBlobGetPageRangesIfUnmodifiedSinceTrue(c *chk.C) {
	containerURL, blobURL := setupGetPageRangesTest(c)
	defer deleteContainer(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	resp, err := blobURL.GetPageRanges(ctx, azblob.BlobRange{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	validateBasicGetPageRanges(c, resp, err)
}

func (s *aztestsSuite) TestBlobGetPageRangesIfUnmodifiedSinceFalse(c *chk.C) {
	containerURL, blobURL := setupGetPageRangesTest(c)
	defer deleteContainer(c, containerURL)

	currentTime := getRelativeTimeGMT(-10)

	_, err := blobURL.GetPageRanges(ctx, azblob.BlobRange{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobGetPageRangesIfMatchTrue(c *chk.C) {
	containerURL, blobURL := setupGetPageRangesTest(c)
	defer deleteContainer(c, containerURL)

	resp, _ := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})

	resp2, err := blobURL.GetPageRanges(ctx, azblob.BlobRange{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: resp.ETag()}})
	validateBasicGetPageRanges(c, resp2, err)
}

func (s *aztestsSuite) TestBlobGetPageRangesIfMatchFalse(c *chk.C) {
	containerURL, blobURL := setupGetPageRangesTest(c)
	defer deleteContainer(c, containerURL)

	_, err := blobURL.GetPageRanges(ctx, azblob.BlobRange{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: azblob.ETag("garbage")}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobGetPageRangesIfNoneMatchTrue(c *chk.C) {
	containerURL, blobURL := setupGetPageRangesTest(c)
	defer deleteContainer(c, containerURL)

	resp, err := blobURL.GetPageRanges(ctx, azblob.BlobRange{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: azblob.ETag("garbage")}})
	validateBasicGetPageRanges(c, resp, err)
}

func (s *aztestsSuite) TestBlobGetPageRangesIfNoneMatchFalse(c *chk.C) {
	containerURL, blobURL := setupGetPageRangesTest(c)
	defer deleteContainer(c, containerURL)

	resp, _ := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})

	_, err := blobURL.GetPageRanges(ctx, azblob.BlobRange{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: resp.ETag()}})
	serr := err.(azblob.StorageError)
	c.Assert(serr.Response().StatusCode, chk.Equals, 304) // Service Code not returned in the body for a HEAD
}

func setupDiffPageRangesTest(c *chk.C) (containerURL azblob.ContainerURL, blobURL azblob.PageBlobURL, snapshot time.Time) {
	bsu := getBSU()
	containerURL, _ = createNewContainer(c, bsu)
	blobURL, _ = createNewPageBlob(c, containerURL)

	_, err := blobURL.PutPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1}, getReaderToRandomBytes(azblob.PageBlobPageBytes), azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := blobURL.CreateSnapshot(ctx, nil, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	snapshot = resp.Snapshot()

	_, err = blobURL.PutPages(ctx, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1}, getReaderToRandomBytes(azblob.PageBlobPageBytes), azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil) // This ensures there is a diff on the first page

	return
}

func validateDiffPageRanges(c *chk.C, resp *azblob.PageList, err error) {
	c.Assert(err, chk.IsNil)
	c.Assert(resp.PageRange, chk.HasLen, 1)
	c.Assert(resp.PageRange[0], chk.Equals, azblob.PageRange{Start: 0, End: azblob.PageBlobPageBytes - 1})
}

func (s *aztestsSuite) TestBlobDiffPageRangesNonExistantSnapshot(c *chk.C) {
	containerURL, blobURL, snapshot := setupDiffPageRangesTest(c)
	defer deleteContainer(c, containerURL)

	_, err := blobURL.GetPageRangesDiff(ctx, azblob.BlobRange{}, snapshot.Add(time.Minute), azblob.BlobAccessConditions{})
	validateStorageError(c, err, azblob.ServiceCodePreviousSnapshotNotFound)
}

func (s *aztestsSuite) TestBlobDiffPageRangeInvalidRange(c *chk.C) {
	containerURL, blobURL, snapshot := setupDiffPageRangesTest(c)
	defer deleteContainer(c, containerURL)

	defer func() { // Invalid page range should panic
		recover()
	}()

	blobURL.GetPageRangesDiff(ctx, azblob.BlobRange{Offset: -22, Count: 14}, snapshot, azblob.BlobAccessConditions{})
	c.Fail()
}

func (s *aztestsSuite) TestBlobDiffPageRangeIfModifiedSinceTrue(c *chk.C) {
	containerURL, blobURL, snapshot := setupDiffPageRangesTest(c)
	defer deleteContainer(c, containerURL)

	currentTime := getRelativeTimeGMT(-10)

	resp, err := blobURL.GetPageRangesDiff(ctx, azblob.BlobRange{}, snapshot,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	validateDiffPageRanges(c, resp, err)
}

func (s *aztestsSuite) TestBlobDiffPageRangeIfModifiedSinceFalse(c *chk.C) {
	containerURL, blobURL, snapshot := setupDiffPageRangesTest(c)
	defer deleteContainer(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	_, err := blobURL.GetPageRangesDiff(ctx, azblob.BlobRange{}, snapshot,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	serr := err.(azblob.StorageError)
	c.Assert(serr.Response().StatusCode, chk.Equals, 304) // Service Code not returned in the body for a HEAD
}

func (s *aztestsSuite) TestBlobDiffPageRangeIfUnmodifiedSinceTrue(c *chk.C) {
	containerURL, blobURL, snapshot := setupDiffPageRangesTest(c)
	defer deleteContainer(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	resp, err := blobURL.GetPageRangesDiff(ctx, azblob.BlobRange{}, snapshot,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	validateDiffPageRanges(c, resp, err)
}

func (s *aztestsSuite) TestBlobDiffPageRangeIfUnmodifiedSinceFalse(c *chk.C) {
	containerURL, blobURL, snapshot := setupDiffPageRangesTest(c)
	defer deleteContainer(c, containerURL)

	currentTime := getRelativeTimeGMT(-10)

	_, err := blobURL.GetPageRangesDiff(ctx, azblob.BlobRange{}, snapshot,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobDiffPageRangeIfMatchTrue(c *chk.C) {
	containerURL, blobURL, snapshot := setupDiffPageRangesTest(c)
	defer deleteContainer(c, containerURL)

	resp, _ := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})

	resp2, err := blobURL.GetPageRangesDiff(ctx, azblob.BlobRange{}, snapshot,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: resp.ETag()}})
	validateDiffPageRanges(c, resp2, err)
}

func (s *aztestsSuite) TestBlobDiffPageRangeIfMatchFalse(c *chk.C) {
	containerURL, blobURL, snapshot := setupDiffPageRangesTest(c)
	defer deleteContainer(c, containerURL)

	_, err := blobURL.GetPageRangesDiff(ctx, azblob.BlobRange{}, snapshot,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: azblob.ETag("garbage")}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobDiffPageRangeIfNoneMatchTrue(c *chk.C) {
	containerURL, blobURL, snapshot := setupDiffPageRangesTest(c)
	defer deleteContainer(c, containerURL)

	resp, err := blobURL.GetPageRangesDiff(ctx, azblob.BlobRange{}, snapshot,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: azblob.ETag("garbage")}})
	validateDiffPageRanges(c, resp, err)
}

func (s *aztestsSuite) TestBlobDiffPageRangeIfNoneMatchFalse(c *chk.C) {
	containerURL, blobURL, snapshot := setupDiffPageRangesTest(c)
	defer deleteContainer(c, containerURL)

	resp, _ := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})

	_, err := blobURL.GetPageRangesDiff(ctx, azblob.BlobRange{}, snapshot,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: resp.ETag()}})
	serr := err.(azblob.StorageError)
	c.Assert(serr.Response().StatusCode, chk.Equals, 304) // Service Code not returned in the body for a HEAD
}

func (s *aztestsSuite) TestBlobResizeZero(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	// The default blob is created with size > 0, so this should actually update
	_, err := blobURL.Resize(ctx, 0, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.ContentLength(), chk.Equals, int64(0))
}

func (s *aztestsSuite) TestBlobResizeInvalidSizeNegative(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	defer func() { // Negative size should panic
		recover()
	}()

	blobURL.Resize(ctx, -4, azblob.BlobAccessConditions{})
	c.Fail()
}

func (s *aztestsSuite) TestBlobResizeInvalidSizeMisaligned(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	defer func() { // Invalid size should panic
		recover()
	}()

	blobURL.Resize(ctx, 12, azblob.BlobAccessConditions{})
	c.Fail()
}

func validateResize(c *chk.C, blobURL azblob.PageBlobURL) {
	resp, _ := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(resp.ContentLength(), chk.Equals, int64(azblob.PageBlobPageBytes))
}

func (s *aztestsSuite) TestBlobResizeIfModifiedSinceTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(-10)

	_, err := blobURL.Resize(ctx, azblob.PageBlobPageBytes,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	validateResize(c, blobURL)
}

func (s *aztestsSuite) TestBlobResizeIfModifiedSinceFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	_, err := blobURL.Resize(ctx, azblob.PageBlobPageBytes,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobResizeIfUnmodifiedSinceTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	_, err := blobURL.Resize(ctx, azblob.PageBlobPageBytes,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	validateResize(c, blobURL)
}

func (s *aztestsSuite) TestBlobResizeIfUnmodifiedSinceFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(-10)

	_, err := blobURL.Resize(ctx, azblob.PageBlobPageBytes,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobResizeIfMatchTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	resp, _ := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})

	_, err := blobURL.Resize(ctx, azblob.PageBlobPageBytes,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: resp.ETag()}})
	c.Assert(err, chk.IsNil)

	validateResize(c, blobURL)
}

func (s *aztestsSuite) TestBlobResizeIfMatchFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	_, err := blobURL.Resize(ctx, azblob.PageBlobPageBytes,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: azblob.ETag("garbage")}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobResizeIfNoneMatchTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	_, err := blobURL.Resize(ctx, azblob.PageBlobPageBytes,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: azblob.ETag("garbage")}})
	c.Assert(err, chk.IsNil)

	validateResize(c, blobURL)
}

func (s *aztestsSuite) TestBlobResizeIfNoneMatchFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	resp, _ := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})

	_, err := blobURL.Resize(ctx, azblob.PageBlobPageBytes,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: resp.ETag()}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobSetSequenceNumberActionTypeInvalid(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	_, err := blobURL.SetSequenceNumber(ctx, azblob.SequenceNumberActionType("garbage"), 1, azblob.BlobHTTPHeaders{}, azblob.BlobAccessConditions{})
	validateStorageError(c, err, azblob.ServiceCodeInvalidHeaderValue)
}

func (s *aztestsSuite) TestBlobSetSequenceNumberSequenceNumberInvalid(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	defer func() { // Invalid sequence number should panic
		recover()
	}()

	blobURL.SetSequenceNumber(ctx, azblob.SequenceNumberActionUpdate, -1, azblob.BlobHTTPHeaders{}, azblob.BlobAccessConditions{})
}

func (s *aztestsSuite) TestBlobSetSequenceNumberHTTPHeaders(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	_, err := blobURL.SetSequenceNumber(ctx, azblob.SequenceNumberActionIncrement, 0, basicHeaders, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)

	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.NewHTTPHeaders(), chk.DeepEquals, basicHeaders)
}

func validateSequenceNumberSet(c *chk.C, blobURL azblob.PageBlobURL) {
	resp, err := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	c.Assert(resp.BlobSequenceNumber(), chk.Equals, "1")
}

func (s *aztestsSuite) TestBlobSetSequenceNumberIfModifiedSinceTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(-10)

	_, err := blobURL.SetSequenceNumber(ctx, azblob.SequenceNumberActionIncrement, 0, azblob.BlobHTTPHeaders{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	validateSequenceNumberSet(c, blobURL)
}

func (s *aztestsSuite) TestBlobSetSequenceNumberIfModifiedSinceFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	_, err := blobURL.SetSequenceNumber(ctx, azblob.SequenceNumberActionIncrement, 0, azblob.BlobHTTPHeaders{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobSetSequenceNumberIfUnmodifiedSinceTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(10)

	_, err := blobURL.SetSequenceNumber(ctx, azblob.SequenceNumberActionIncrement, 0, azblob.BlobHTTPHeaders{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	validateSequenceNumberSet(c, blobURL)
}

func (s *aztestsSuite) TestBlobSetSequenceNumberIfUnmodifiedSinceFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	currentTime := getRelativeTimeGMT(-10)

	_, err := blobURL.SetSequenceNumber(ctx, azblob.SequenceNumberActionIncrement, 0, azblob.BlobHTTPHeaders{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobSetSequenceNumberIfMatchTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	resp, _ := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})

	_, err := blobURL.SetSequenceNumber(ctx, azblob.SequenceNumberActionIncrement, 0, azblob.BlobHTTPHeaders{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: resp.ETag()}})
	c.Assert(err, chk.IsNil)

	validateSequenceNumberSet(c, blobURL)
}

func (s *aztestsSuite) TestBlobSetSequenceNumberIfMatchFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	_, err := blobURL.SetSequenceNumber(ctx, azblob.SequenceNumberActionIncrement, 0, azblob.BlobHTTPHeaders{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: azblob.ETag("garbage")}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobSetSequenceNumberIfNoneMatchTrue(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	_, err := blobURL.SetSequenceNumber(ctx, azblob.SequenceNumberActionIncrement, 0, azblob.BlobHTTPHeaders{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: azblob.ETag("garbage")}})
	c.Assert(err, chk.IsNil)

	validateSequenceNumberSet(c, blobURL)
}

func (s *aztestsSuite) TestBlobSetSequenceNumberIfNoneMatchFalse(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)

	resp, _ := blobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})

	_, err := blobURL.SetSequenceNumber(ctx, azblob.SequenceNumberActionIncrement, 0, azblob.BlobHTTPHeaders{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: resp.ETag()}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func waitForIncrementalCopy(c *chk.C, copyBlobURL azblob.PageBlobURL, blobCopyResponse *azblob.PageBlobsIncrementalCopyResponse) time.Time {
	status := blobCopyResponse.CopyStatus()
	var getPropertiesAndMetadataResult *azblob.BlobsGetPropertiesResponse
	// Wait for the copy to finish
	start := time.Now()
	for status != azblob.CopyStatusSuccess {
		getPropertiesAndMetadataResult, _ = copyBlobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
		status = getPropertiesAndMetadataResult.CopyStatus()
		currentTime := time.Now()
		if currentTime.Sub(start) >= time.Minute {
			c.Fail()
		}
	}
	return getPropertiesAndMetadataResult.DestinationSnapshot()
}

func setupStartIncrementalCopyTest(c *chk.C) (containerURL azblob.ContainerURL, blobURL azblob.PageBlobURL, copyBlobURL azblob.PageBlobURL, snapshot time.Time) {
	bsu := getBSU()
	containerURL, _ = createNewContainer(c, bsu)
	blobURL, _ = createNewPageBlob(c, containerURL)
	resp, _ := blobURL.CreateSnapshot(ctx, nil, azblob.BlobAccessConditions{})
	copyBlobURL, _ = getPageBlobURL(c, containerURL)

	// Must create the incremental copy blob so that the access conditions work on it
	resp2, err := copyBlobURL.StartIncrementalCopy(ctx, blobURL.URL(), resp.Snapshot(), azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
	waitForIncrementalCopy(c, copyBlobURL, resp2)

	resp, _ = blobURL.CreateSnapshot(ctx, nil, azblob.BlobAccessConditions{}) // Take a new snapshot so the next copy will succeed
	snapshot = resp.Snapshot()
	return
}

func validateIncrementalCopy(c *chk.C, copyBlobURL azblob.PageBlobURL, resp *azblob.PageBlobsIncrementalCopyResponse) {
	t := waitForIncrementalCopy(c, copyBlobURL, resp)

	// If we can access the snapshot without error, we are satisfied that it was created as a result of the copy
	copySnapshotURL := copyBlobURL.WithSnapshot(t)
	_, err := copySnapshotURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})
	c.Assert(err, chk.IsNil)
}

func (s *aztestsSuite) TestBlobStartIncrementalCopySnapshotNotExist(c *chk.C) {
	bsu := getBSU()
	containerURL, _ := createNewContainer(c, bsu)
	defer deleteContainer(c, containerURL)
	blobURL, _ := createNewPageBlob(c, containerURL)
	copyBlobURL, _ := getPageBlobURL(c, containerURL)

	_, err := copyBlobURL.StartIncrementalCopy(ctx, blobURL.URL(), time.Now(), azblob.BlobAccessConditions{})
	validateStorageError(c, err, azblob.ServiceCodeInvalidHeaderValue)
}

func (s *aztestsSuite) TestBlobStartIncrementalCopyIfModifiedSinceTrue(c *chk.C) {
	containerURL, blobURL, copyBlobURL, snapshot := setupStartIncrementalCopyTest(c)

	defer deleteContainer(c, containerURL)

	currentTime := getRelativeTimeGMT(-20)

	resp, err := copyBlobURL.StartIncrementalCopy(ctx, blobURL.URL(), snapshot,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	validateIncrementalCopy(c, copyBlobURL, resp)
}

func (s *aztestsSuite) TestBlobStartIncrementalCopyIfModifiedSinceFalse(c *chk.C) {
	containerURL, blobURL, copyBlobURL, snapshot := setupStartIncrementalCopyTest(c)

	defer deleteContainer(c, containerURL)

	currentTime := getRelativeTimeGMT(20)

	_, err := copyBlobURL.StartIncrementalCopy(ctx, blobURL.URL(), snapshot,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobStartIncrementalCopyIfUnmodifiedSinceTrue(c *chk.C) {
	containerURL, blobURL, copyBlobURL, snapshot := setupStartIncrementalCopyTest(c)

	defer deleteContainer(c, containerURL)

	currentTime := getRelativeTimeGMT(20)

	resp, err := copyBlobURL.StartIncrementalCopy(ctx, blobURL.URL(), snapshot,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	c.Assert(err, chk.IsNil)

	validateIncrementalCopy(c, copyBlobURL, resp)
}

func (s *aztestsSuite) TestBlobStartIncrementalCopyIfUnmodifiedSinceFalse(c *chk.C) {
	containerURL, blobURL, copyBlobURL, snapshot := setupStartIncrementalCopyTest(c)

	defer deleteContainer(c, containerURL)

	currentTime := getRelativeTimeGMT(-20)

	_, err := copyBlobURL.StartIncrementalCopy(ctx, blobURL.URL(), snapshot,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: currentTime}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}

func (s *aztestsSuite) TestBlobStartIncrementalCopyIfMatchTrue(c *chk.C) {
	containerURL, blobURL, copyBlobURL, snapshot := setupStartIncrementalCopyTest(c)

	defer deleteContainer(c, containerURL)

	resp, _ := copyBlobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})

	resp2, err := copyBlobURL.StartIncrementalCopy(ctx, blobURL.URL(), snapshot,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: resp.ETag()}})
	c.Assert(err, chk.IsNil)

	validateIncrementalCopy(c, copyBlobURL, resp2)
}

func (s *aztestsSuite) TestBlobStartIncrementalCopyIfMatchFalse(c *chk.C) {
	containerURL, blobURL, copyBlobURL, snapshot := setupStartIncrementalCopyTest(c)

	defer deleteContainer(c, containerURL)

	_, err := copyBlobURL.StartIncrementalCopy(ctx, blobURL.URL(), snapshot,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: azblob.ETag("garbage")}})
	validateStorageError(c, err, azblob.ServiceCodeTargetConditionNotMet)
}

func (s *aztestsSuite) TestBlobStartIncrementalCopyIfNoneMatchTrue(c *chk.C) {
	containerURL, blobURL, copyBlobURL, snapshot := setupStartIncrementalCopyTest(c)

	defer deleteContainer(c, containerURL)

	resp, err := copyBlobURL.StartIncrementalCopy(ctx, blobURL.URL(), snapshot,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: azblob.ETag("garbage")}})
	c.Assert(err, chk.IsNil)

	validateIncrementalCopy(c, copyBlobURL, resp)
}

func (s *aztestsSuite) TestBlobStartIncrementalCopyIfNoneMatchFalse(c *chk.C) {
	containerURL, blobURL, copyBlobURL, snapshot := setupStartIncrementalCopyTest(c)

	defer deleteContainer(c, containerURL)

	resp, _ := copyBlobURL.GetPropertiesAndMetadata(ctx, azblob.BlobAccessConditions{})

	_, err := copyBlobURL.StartIncrementalCopy(ctx, blobURL.URL(), snapshot,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: resp.ETag()}})
	validateStorageError(c, err, azblob.ServiceCodeConditionNotMet)
}
