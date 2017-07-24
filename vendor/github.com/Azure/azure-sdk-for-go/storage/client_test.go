package storage

import (
	"bytes"
	"encoding/base64"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/dnaeon/go-vcr/cassette"
	"github.com/dnaeon/go-vcr/recorder"
	chk "gopkg.in/check.v1"
)

// Hook up gocheck to testing
func Test(t *testing.T) { chk.TestingT(t) }

type StorageClientSuite struct{}

var _ = chk.Suite(&StorageClientSuite{})

func setHeaders(haystack http.Header, predicate func(string) bool, value string) {
	for key := range haystack {
		if predicate(key) {
			haystack[key] = []string{value}
		}
	}
}

func deleteHeaders(haystack http.Header, predicate func(string) bool) {
	for key := range haystack {
		if predicate(key) {
			delete(haystack, key)
		}
	}
}
func getHeaders(haystack http.Header, predicate func(string) bool) string {
	for key, value := range haystack {
		if predicate(key) && len(value) > 0 {
			return value[0]
		}
	}
	return ""
}

// getBasicClient returns a test client from storage credentials in the env
func getBasicClient(c *chk.C) *Client {
	name := os.Getenv("ACCOUNT_NAME")
	if name == "" {
		name = dummyStorageAccount
	}
	key := os.Getenv("ACCOUNT_KEY")
	if key == "" {
		key = dummyMiniStorageKey
	}
	cli, err := NewBasicClient(name, key)
	c.Assert(err, chk.IsNil)

	return &cli
}

func (client *Client) appendRecorder(c *chk.C) *recorder.Recorder {
	tests := strings.Split(c.TestName(), ".")
	path := filepath.Join(recordingsFolder, tests[0], tests[1])
	rec, err := recorder.New(path)
	c.Assert(err, chk.IsNil)
	client.HTTPClient = &http.Client{
		Transport: rec,
	}
	rec.SetMatcher(func(r *http.Request, i cassette.Request) bool {
		return compareMethods(r, i) &&
			compareURLs(r, i) &&
			compareHeaders(r, i) &&
			compareBodies(r, i)
	})
	return rec
}

func (client *Client) usesDummies() bool {
	key, err := base64.StdEncoding.DecodeString(dummyMiniStorageKey)
	if err != nil {
		return false
	}
	if string(client.accountKey) == string(key) &&
		client.accountName == dummyStorageAccount {
		return true
	}
	return false
}

func compareMethods(r *http.Request, i cassette.Request) bool {
	return r.Method == i.Method
}

func compareURLs(r *http.Request, i cassette.Request) bool {
	newURL := modifyURL(r.URL)
	return newURL.String() == i.URL
}

func modifyURL(url *url.URL) *url.URL {
	// The URL host looks like this...
	// accountname.service.storageEndpointSuffix
	parts := strings.Split(url.Host, ".")
	// parts[0] corresponds to the storage account name, so it can be (almost) any string
	// parts[1] corresponds to the service name (table, blob, etc.).
	if !(parts[1] == blobServiceName ||
		parts[1] == tableServiceName ||
		parts[1] == queueServiceName ||
		parts[1] == fileServiceName) {
		return nil
	}
	// The rest of the host depends on which Azure cloud is used
	storageEndpointSuffix := strings.Join(parts[2:], ".")
	if !(storageEndpointSuffix == azure.PublicCloud.StorageEndpointSuffix ||
		storageEndpointSuffix == azure.USGovernmentCloud.StorageEndpointSuffix ||
		storageEndpointSuffix == azure.ChinaCloud.StorageEndpointSuffix ||
		storageEndpointSuffix == azure.GermanCloud.StorageEndpointSuffix) {
		return nil
	}

	host := dummyStorageAccount + "." + parts[1] + "." + azure.PublicCloud.StorageEndpointSuffix
	newURL := url
	newURL.Host = host
	return newURL
}

func compareHeaders(r *http.Request, i cassette.Request) bool {
	requestHeaders := r.Header
	cassetteHeaders := i.Headers
	// Some headers shall not be compared...

	getHeaderMatchPredicate := func(needle string) func(string) bool {
		return func(straw string) bool {
			return strings.EqualFold(needle, straw)
		}
	}

	isUserAgent := getHeaderMatchPredicate("User-Agent")
	isAuthorization := getHeaderMatchPredicate("Authorization")
	isDate := getHeaderMatchPredicate("x-ms-date")
	deleteHeaders(requestHeaders, isUserAgent)
	deleteHeaders(requestHeaders, isAuthorization)
	deleteHeaders(requestHeaders, isDate)

	deleteHeaders(cassetteHeaders, isUserAgent)
	deleteHeaders(cassetteHeaders, isAuthorization)
	deleteHeaders(cassetteHeaders, isDate)

	isCopySource := getHeaderMatchPredicate("X-Ms-Copy-Source")
	srcURLstr := getHeaders(requestHeaders, isCopySource)
	if srcURLstr != "" {
		srcURL, err := url.Parse(srcURLstr)
		if err != nil {
			return false
		}
		modifiedURL := modifyURL(srcURL)
		setHeaders(requestHeaders, isCopySource, modifiedURL.String())
	}

	// Do not compare the complete Content-Type header in table batch requests
	if isBatchOp(r.URL.String()) {
		// They all start like this, but then they have a UUID...
		ctPrefixBatch := "multipart/mixed; boundary=batch_"

		isContentType := getHeaderMatchPredicate("Content-Type")

		contentTypeRequest := getHeaders(requestHeaders, isContentType)
		contentTypeCassette := getHeaders(cassetteHeaders, isContentType)
		if !(strings.HasPrefix(contentTypeRequest, ctPrefixBatch) &&
			strings.HasPrefix(contentTypeCassette, ctPrefixBatch)) {
			return false
		}

		deleteHeaders(requestHeaders, isContentType)
		deleteHeaders(cassetteHeaders, isContentType)
	}

	return reflect.DeepEqual(requestHeaders, cassetteHeaders)
}

func compareBodies(r *http.Request, i cassette.Request) bool {
	body := bytes.Buffer{}
	if r.Body != nil {
		_, err := body.ReadFrom(r.Body)
		if err != nil {
			return false
		}
		r.Body = ioutil.NopCloser(&body)
	}
	// Comparing bodies in table batch operations is trickier, because the bodies include UUIDs
	if isBatchOp(r.URL.String()) {
		return compareBatchBodies(body.String(), i.Body)
	}
	return body.String() == i.Body
}

func compareBatchBodies(rBody, cBody string) bool {
	// UUIDs in the batch body look like this...
	// 2d7f2323-1e42-11e7-8c6c-6451064d81e8
	exp, err := regexp.Compile("[0-9a-z]{8}-[0-9a-z]{4}-[0-9a-z]{4}-[0-9a-z]{4}-[0-9a-z]{12}")
	if err != nil {
		return false
	}
	rBody = replaceStorageAccount(replaceUUIDs(rBody, exp))
	cBody = replaceUUIDs(cBody, exp)
	return rBody == cBody
}

func replaceUUIDs(body string, exp *regexp.Regexp) string {
	indexes := exp.FindAllStringIndex(body, -1)
	for _, pair := range indexes {
		body = strings.Replace(body, body[pair[0]:pair[1]], "00000000-0000-0000-0000-000000000000", -1)
	}
	return body
}

func isBatchOp(url string) bool {
	return url == "https://golangrocksonazure.table.core.windows.net/$batch"
}

//getEmulatorClient returns a test client for Azure Storeage Emulator
func getEmulatorClient(c *chk.C) Client {
	cli, err := NewBasicClient(StorageEmulatorAccountName, "")
	c.Assert(err, chk.IsNil)
	return cli
}

func (s *StorageClientSuite) TestNewEmulatorClient(c *chk.C) {
	cli, err := NewBasicClient(StorageEmulatorAccountName, "")
	c.Assert(err, chk.IsNil)
	c.Assert(cli.accountName, chk.Equals, StorageEmulatorAccountName)
	expectedKey, err := base64.StdEncoding.DecodeString(StorageEmulatorAccountKey)
	c.Assert(err, chk.IsNil)
	c.Assert(cli.accountKey, chk.DeepEquals, expectedKey)
}

func (s *StorageClientSuite) TestIsValidStorageAccount(c *chk.C) {
	type test struct {
		account  string
		expected bool
	}
	testCases := []test{
		{"name1", true},
		{"Name2", false},
		{"reallyLongName1234567891011", false},
		{"", false},
		{"concated&name", false},
		{"formatted name", false},
	}

	for _, tc := range testCases {
		c.Assert(IsValidStorageAccount(tc.account), chk.Equals, tc.expected)
	}
}

func (s *StorageClientSuite) TestMalformedKeyError(c *chk.C) {
	_, err := NewBasicClient(dummyStorageAccount, "malformed")
	c.Assert(err, chk.ErrorMatches, "azure: malformed storage account key: .*")
}

func (s *StorageClientSuite) TestGetBaseURL_Basic_Https(c *chk.C) {
	cli, err := NewBasicClient(dummyStorageAccount, dummyMiniStorageKey)
	c.Assert(err, chk.IsNil)
	c.Assert(cli.apiVersion, chk.Equals, DefaultAPIVersion)
	c.Assert(err, chk.IsNil)
	c.Assert(cli.getBaseURL("table").String(), chk.Equals, "https://golangrocksonazure.table.core.windows.net")
}

func (s *StorageClientSuite) TestGetBaseURL_Custom_NoHttps(c *chk.C) {
	apiVersion := "2015-01-01" // a non existing one
	cli, err := NewClient(dummyStorageAccount, dummyMiniStorageKey, "core.chinacloudapi.cn", apiVersion, false)
	c.Assert(err, chk.IsNil)
	c.Assert(cli.apiVersion, chk.Equals, apiVersion)
	c.Assert(cli.getBaseURL("table").String(), chk.Equals, "http://golangrocksonazure.table.core.chinacloudapi.cn")
}

func (s *StorageClientSuite) TestGetBaseURL_StorageEmulator(c *chk.C) {
	cli, err := NewBasicClient(StorageEmulatorAccountName, StorageEmulatorAccountKey)
	c.Assert(err, chk.IsNil)

	type test struct{ service, expected string }
	tests := []test{
		{blobServiceName, "http://127.0.0.1:10000"},
		{tableServiceName, "http://127.0.0.1:10002"},
		{queueServiceName, "http://127.0.0.1:10001"},
	}
	for _, i := range tests {
		baseURL := cli.getBaseURL(i.service)
		c.Assert(baseURL.String(), chk.Equals, i.expected)
	}
}

func (s *StorageClientSuite) TestGetEndpoint_None(c *chk.C) {
	cli, err := NewBasicClient(dummyStorageAccount, "YmFy")
	c.Assert(err, chk.IsNil)
	output := cli.getEndpoint(blobServiceName, "", url.Values{})
	c.Assert(output, chk.Equals, "https://golangrocksonazure.blob.core.windows.net/")
}

func (s *StorageClientSuite) TestGetEndpoint_PathOnly(c *chk.C) {
	cli, err := NewBasicClient(dummyStorageAccount, "YmFy")
	c.Assert(err, chk.IsNil)
	output := cli.getEndpoint(blobServiceName, "path", url.Values{})
	c.Assert(output, chk.Equals, "https://golangrocksonazure.blob.core.windows.net/path")
}

func (s *StorageClientSuite) TestGetEndpoint_ParamsOnly(c *chk.C) {
	cli, err := NewBasicClient(dummyStorageAccount, "YmFy")
	c.Assert(err, chk.IsNil)
	params := url.Values{}
	params.Set("a", "b")
	params.Set("c", "d")
	output := cli.getEndpoint(blobServiceName, "", params)
	c.Assert(output, chk.Equals, "https://golangrocksonazure.blob.core.windows.net/?a=b&c=d")
}

func (s *StorageClientSuite) TestGetEndpoint_Mixed(c *chk.C) {
	cli, err := NewBasicClient(dummyStorageAccount, "YmFy")
	c.Assert(err, chk.IsNil)
	params := url.Values{}
	params.Set("a", "b")
	params.Set("c", "d")
	output := cli.getEndpoint(blobServiceName, "path", params)
	c.Assert(output, chk.Equals, "https://golangrocksonazure.blob.core.windows.net/path?a=b&c=d")
}

func (s *StorageClientSuite) TestGetEndpoint_StorageEmulator(c *chk.C) {
	cli, err := NewBasicClient(StorageEmulatorAccountName, StorageEmulatorAccountKey)
	c.Assert(err, chk.IsNil)

	type test struct{ service, expected string }
	tests := []test{
		{blobServiceName, "http://127.0.0.1:10000/devstoreaccount1/"},
		{tableServiceName, "http://127.0.0.1:10002/devstoreaccount1/"},
		{queueServiceName, "http://127.0.0.1:10001/devstoreaccount1/"},
	}
	for _, i := range tests {
		endpoint := cli.getEndpoint(i.service, "", url.Values{})
		c.Assert(endpoint, chk.Equals, i.expected)
	}
}

func (s *StorageClientSuite) Test_getStandardHeaders(c *chk.C) {
	cli, err := NewBasicClient(dummyStorageAccount, "YmFy")
	c.Assert(err, chk.IsNil)

	headers := cli.getStandardHeaders()
	c.Assert(len(headers), chk.Equals, 3)
	c.Assert(headers["x-ms-version"], chk.Equals, cli.apiVersion)
	if _, ok := headers["x-ms-date"]; !ok {
		c.Fatal("Missing date header")
	}
	c.Assert(headers[userAgentHeader], chk.Equals, cli.getDefaultUserAgent())
}

func (s *StorageClientSuite) TestReturnsStorageServiceError(c *chk.C) {
	// attempt to delete nonexisting resources
	cli := getBasicClient(c)
	rec := cli.appendRecorder(c)
	defer rec.Stop()

	// XML response
	blobCli := cli.GetBlobService()
	cnt := blobCli.GetContainerReference(containerName(c))
	err := cnt.Delete(nil)
	c.Assert(err, chk.NotNil)

	v, ok := err.(AzureStorageServiceError)
	c.Check(ok, chk.Equals, true)
	c.Assert(v.StatusCode, chk.Equals, 404)
	c.Assert(v.Code, chk.Equals, "ContainerNotFound")
	c.Assert(v.RequestID, chk.Not(chk.Equals), "")
	c.Assert(v.Date, chk.Not(chk.Equals), "")
	c.Assert(v.APIVersion, chk.Not(chk.Equals), "")

	// JSON response
	tableCli := cli.GetTableService()
	table := tableCli.GetTableReference(tableName(c))
	err = table.Delete(30, nil)
	c.Assert(err, chk.NotNil)

	v, ok = err.(AzureStorageServiceError)
	c.Check(ok, chk.Equals, true)
	c.Assert(v.StatusCode, chk.Equals, 404)
	c.Assert(v.Code, chk.Equals, "ResourceNotFound")
	c.Assert(v.RequestID, chk.Not(chk.Equals), "")
	c.Assert(v.Date, chk.Not(chk.Equals), "")
	c.Assert(v.APIVersion, chk.Not(chk.Equals), "")
}

func (s *StorageClientSuite) TestReturnsStorageServiceError_withoutResponseBody(c *chk.C) {
	// HEAD on non-existing blob
	cli := getBlobClient(c)
	rec := cli.client.appendRecorder(c)
	defer rec.Stop()

	cnt := cli.GetContainerReference("non-existing-container")
	b := cnt.GetBlobReference("non-existing-blob")
	err := b.GetProperties(nil)

	c.Assert(err, chk.NotNil)
	c.Assert(err, chk.FitsTypeOf, AzureStorageServiceError{})

	v, ok := err.(AzureStorageServiceError)
	c.Check(ok, chk.Equals, true)
	c.Assert(v.StatusCode, chk.Equals, http.StatusNotFound)
	c.Assert(v.Code, chk.Equals, "404 The specified container does not exist.")
	c.Assert(v.RequestID, chk.Not(chk.Equals), "")
	c.Assert(v.Message, chk.Equals, "no response body was available for error status code")
}

func (s *StorageClientSuite) Test_createServiceClients(c *chk.C) {
	cli, err := NewBasicClient(dummyStorageAccount, "YmFy")
	c.Assert(err, chk.IsNil)

	ua := cli.getDefaultUserAgent()

	headers := cli.getStandardHeaders()
	c.Assert(headers[userAgentHeader], chk.Equals, ua)
	c.Assert(cli.userAgent, chk.Equals, ua)

	b := cli.GetBlobService()
	c.Assert(b.client.userAgent, chk.Equals, ua+" "+blobServiceName)
	c.Assert(cli.userAgent, chk.Equals, ua)

	t := cli.GetTableService()
	c.Assert(t.client.userAgent, chk.Equals, ua+" "+tableServiceName)
	c.Assert(cli.userAgent, chk.Equals, ua)

	q := cli.GetQueueService()
	c.Assert(q.client.userAgent, chk.Equals, ua+" "+queueServiceName)
	c.Assert(cli.userAgent, chk.Equals, ua)

	f := cli.GetFileService()
	c.Assert(f.client.userAgent, chk.Equals, ua+" "+fileServiceName)
	c.Assert(cli.userAgent, chk.Equals, ua)
}

func (s *StorageClientSuite) TestAddToUserAgent(c *chk.C) {
	cli, err := NewBasicClient(dummyStorageAccount, "YmFy")
	c.Assert(err, chk.IsNil)

	ua := cli.getDefaultUserAgent()

	err = cli.AddToUserAgent("rofl")
	c.Assert(err, chk.IsNil)
	c.Assert(cli.userAgent, chk.Equals, ua+" rofl")

	err = cli.AddToUserAgent("")
	c.Assert(err, chk.NotNil)
}

func (s *StorageClientSuite) Test_protectUserAgent(c *chk.C) {
	extraheaders := map[string]string{
		"1":             "one",
		"2":             "two",
		"3":             "three",
		userAgentHeader: "four",
	}

	cli, err := NewBasicClient(dummyStorageAccount, "YmFy")
	c.Assert(err, chk.IsNil)

	ua := cli.getDefaultUserAgent()

	got := cli.protectUserAgent(extraheaders)
	c.Assert(cli.userAgent, chk.Equals, ua+" four")
	c.Assert(got, chk.HasLen, 3)
	c.Assert(got, chk.DeepEquals, map[string]string{
		"1": "one",
		"2": "two",
		"3": "three",
	})
}

func (s *StorageClientSuite) Test_doRetry(c *chk.C) {
	cli := getBasicClient(c)
	rec := cli.appendRecorder(c)
	defer rec.Stop()

	// Prepare request that will fail with 404 (delete non extising table)
	uri, err := url.Parse(cli.getEndpoint(tableServiceName, "(retry)", url.Values{"timeout": {strconv.Itoa(30)}}))
	c.Assert(err, chk.IsNil)
	req := http.Request{
		Method: http.MethodDelete,
		URL:    uri,
		Header: http.Header{
			"Accept":       {"application/json;odata=nometadata"},
			"Prefer":       {"return-no-content"},
			"X-Ms-Version": {"2016-05-31"},
		},
	}

	ds, ok := cli.Sender.(*DefaultSender)
	c.Assert(ok, chk.Equals, true)
	// Modify sender so it retries quickly
	ds.RetryAttempts = 3
	ds.RetryDuration = time.Second
	// include 404 as a valid status code for retries
	ds.ValidStatusCodes = []int{http.StatusNotFound}
	cli.Sender = ds

	now := time.Now()
	resp, err := cli.Sender.Send(cli, &req)
	afterRetries := time.Since(now)
	c.Assert(err, chk.IsNil)
	c.Assert(resp.StatusCode, chk.Equals, http.StatusNotFound)

	// Was it the correct amount of retries... ?
	c.Assert(cli.Sender.(*DefaultSender).attempts, chk.Equals, cli.Sender.(*DefaultSender).RetryAttempts)
	// What about time... ?
	// Note, seconds are rounded
	sum := 0
	for i := 0; i < ds.RetryAttempts; i++ {
		sum += int(ds.RetryDuration.Seconds() * math.Pow(2, float64(i))) // same formula used in autorest.DelayForBackoff
	}
	c.Assert(int(afterRetries.Seconds()), chk.Equals, sum)
}
