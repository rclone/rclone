package storage

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	chk "gopkg.in/check.v1"
)

type BlobSASURISuite struct{}

var _ = chk.Suite(&BlobSASURISuite{})

var oldAPIVer = "2013-08-15"
var newerAPIVer = "2015-04-05"

func (s *BlobSASURISuite) TestGetBlobSASURI(c *chk.C) {
	api, err := NewClient("foo", dummyMiniStorageKey, DefaultBaseURL, oldAPIVer, true)
	c.Assert(err, chk.IsNil)
	cli := api.GetBlobService()
	cnt := cli.GetContainerReference("container")
	b := cnt.GetBlobReference("name")
	expiry := time.Time{}

	expectedParts := url.URL{
		Scheme: "https",
		Host:   "foo.blob.core.windows.net",
		Path:   "container/name",
		RawQuery: url.Values{
			"sv":  {oldAPIVer},
			"sig": {"/OXG7rWh08jYwtU03GzJM0DHZtidRGpC6g69rSGm3I0="},
			"sr":  {"b"},
			"sp":  {"r"},
			"se":  {"0001-01-01T00:00:00Z"},
		}.Encode()}

	u, err := b.GetSASURI(expiry, "r")
	c.Assert(err, chk.IsNil)
	sasParts, err := url.Parse(u)
	c.Assert(err, chk.IsNil)
	c.Assert(expectedParts.String(), chk.Equals, sasParts.String())
	c.Assert(expectedParts.Query(), chk.DeepEquals, sasParts.Query())
}

//Gets a SASURI for the entire container
func (s *BlobSASURISuite) TestGetBlobSASURIContainer(c *chk.C) {
	api, err := NewClient("foo", dummyMiniStorageKey, DefaultBaseURL, oldAPIVer, true)
	c.Assert(err, chk.IsNil)
	cli := api.GetBlobService()
	cnt := cli.GetContainerReference("container")
	b := cnt.GetBlobReference("")
	expiry := time.Time{}

	expectedParts := url.URL{
		Scheme: "https",
		Host:   "foo.blob.core.windows.net",
		Path:   "container",
		RawQuery: url.Values{
			"sv":  {oldAPIVer},
			"sig": {"KMjYyQODKp6uK9EKR3yGhO2M84e1LfoztypU32kHj4s="},
			"sr":  {"c"},
			"sp":  {"r"},
			"se":  {"0001-01-01T00:00:00Z"},
		}.Encode()}

	u, err := b.GetSASURI(expiry, "r")
	c.Assert(err, chk.IsNil)
	sasParts, err := url.Parse(u)
	c.Assert(err, chk.IsNil)
	c.Assert(expectedParts.String(), chk.Equals, sasParts.String())
	c.Assert(expectedParts.Query(), chk.DeepEquals, sasParts.Query())
}

func (s *BlobSASURISuite) TestGetBlobSASURIWithSignedIPAndProtocolValidAPIVersionPassed(c *chk.C) {
	api, err := NewClient("foo", dummyMiniStorageKey, DefaultBaseURL, newerAPIVer, true)
	c.Assert(err, chk.IsNil)
	cli := api.GetBlobService()
	cnt := cli.GetContainerReference("container")
	b := cnt.GetBlobReference("name")
	expiry := time.Time{}

	expectedParts := url.URL{
		Scheme: "https",
		Host:   "foo.blob.core.windows.net",
		Path:   "/container/name",
		RawQuery: url.Values{
			"sv":  {newerAPIVer},
			"sig": {"VBOYJmt89UuBRXrxNzmsCMoC+8PXX2yklV71QcL1BfM="},
			"sr":  {"b"},
			"sip": {"127.0.0.1"},
			"sp":  {"r"},
			"se":  {"0001-01-01T00:00:00Z"},
			"spr": {"https"},
		}.Encode()}

	u, err := b.GetSASURIWithSignedIPAndProtocol(expiry, "r", "127.0.0.1", true)
	c.Assert(err, chk.IsNil)
	sasParts, err := url.Parse(u)
	c.Assert(err, chk.IsNil)
	c.Assert(sasParts.Query(), chk.DeepEquals, expectedParts.Query())
}

// Trying to use SignedIP and Protocol but using an older version of the API.
// Should ignore the signedIP/protocol and just use what the older version requires.
func (s *BlobSASURISuite) TestGetBlobSASURIWithSignedIPAndProtocolUsingOldAPIVersion(c *chk.C) {
	api, err := NewClient("foo", dummyMiniStorageKey, DefaultBaseURL, oldAPIVer, true)
	c.Assert(err, chk.IsNil)
	cli := api.GetBlobService()
	cnt := cli.GetContainerReference("container")
	b := cnt.GetBlobReference("name")
	expiry := time.Time{}

	expectedParts := url.URL{
		Scheme: "https",
		Host:   "foo.blob.core.windows.net",
		Path:   "/container/name",
		RawQuery: url.Values{
			"sv":  {oldAPIVer},
			"sig": {"/OXG7rWh08jYwtU03GzJM0DHZtidRGpC6g69rSGm3I0="},
			"sr":  {"b"},
			"sp":  {"r"},
			"se":  {"0001-01-01T00:00:00Z"},
		}.Encode()}

	u, err := b.GetSASURIWithSignedIPAndProtocol(expiry, "r", "", true)
	c.Assert(err, chk.IsNil)
	sasParts, err := url.Parse(u)
	c.Assert(err, chk.IsNil)
	c.Assert(expectedParts.String(), chk.Equals, sasParts.String())
	c.Assert(expectedParts.Query(), chk.DeepEquals, sasParts.Query())
}

func (s *BlobSASURISuite) TestBlobSASURICorrectness(c *chk.C) {
	cli := getBlobClient(c)

	if cli.client.usesDummies() {
		c.Skip("As GetSASURI result depends on the account key, it is not practical to test it with a dummy key.")
	}

	simpleClient := &http.Client{}
	rec := cli.client.appendRecorder(c)
	simpleClient.Transport = rec
	defer rec.Stop()

	cnt := cli.GetContainerReference(containerName(c))
	c.Assert(cnt.Create(nil), chk.IsNil)
	b := cnt.GetBlobReference(contentWithSpecialChars(5))
	defer cnt.Delete(nil)

	body := content(100)
	expiry := fixedTime.UTC().Add(time.Hour)
	permissions := "r"

	c.Assert(b.putSingleBlockBlob(body), chk.IsNil)

	sasURI, err := b.GetSASURI(expiry, permissions)
	c.Assert(err, chk.IsNil)

	resp, err := simpleClient.Get(sasURI)
	c.Assert(err, chk.IsNil)

	blobResp, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	c.Assert(err, chk.IsNil)

	c.Assert(resp.StatusCode, chk.Equals, http.StatusOK)
	c.Assert(string(blobResp), chk.Equals, string(body))

}

func (s *BlobSASURISuite) Test_blobSASStringToSign(c *chk.C) {
	_, err := blobSASStringToSign("2012-02-12", "CS", "SE", "SP", "", "")
	c.Assert(err, chk.NotNil) // not implemented SAS for versions earlier than 2013-08-15

	out, err := blobSASStringToSign(oldAPIVer, "CS", "SE", "SP", "", "")
	c.Assert(err, chk.IsNil)
	c.Assert(out, chk.Equals, "SP\n\nSE\nCS\n\n2013-08-15\n\n\n\n\n")

	// check format for 2015-04-05 version
	out, err = blobSASStringToSign(newerAPIVer, "CS", "SE", "SP", "127.0.0.1", "https,http")
	c.Assert(err, chk.IsNil)
	c.Assert(out, chk.Equals, "SP\n\nSE\n/blobCS\n\n127.0.0.1\nhttps,http\n2015-04-05\n\n\n\n\n")
}
