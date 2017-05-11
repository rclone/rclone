/*
** Copyright (c) 2014 Arnaud Ysmal.  All Rights Reserved.
**
** Redistribution and use in source and binary forms, with or without
** modification, are permitted provided that the following conditions
** are met:
** 1. Redistributions of source code must retain the above copyright
**    notice, this list of conditions and the following disclaimer.
** 2. Redistributions in binary form must reproduce the above copyright
**    notice, this list of conditions and the following disclaimer in the
**    documentation and/or other materials provided with the distribution.
**
** THIS SOFTWARE IS PROVIDED BY THE AUTHOR ``AS IS'' AND ANY EXPRESS
** OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
** WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
** DISCLAIMED. IN NO EVENT SHALL THE AUTHOR OR CONTRIBUTORS BE LIABLE
** FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
** DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
** SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
** HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT
** LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY
** OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF
** SUCH DAMAGE.
 */

package dropbox

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strconv"
	"testing"
	"time"
)

var dirEntry = Entry{Size: "0 bytes", Revision: "1f477dd351f", ThumbExists: false, Bytes: 0,
	Modified: DBTime(time.Date(2011, time.August, 10, 18, 21, 30, 0, time.UTC)),
	Path:     "/testdir", IsDir: true, Icon: "folder", Root: "auto"}

var fileEntry = Entry{Size: "0 bytes", Revision: "1f33043551f", ThumbExists: false, Bytes: 0,
	Modified: DBTime(time.Date(2011, time.August, 10, 18, 21, 30, 0, time.UTC)),
	Path:     "/testfile", IsDir: false, Icon: "page_white_text",
	Root: "auto", MimeType: "text/plain"}

type FakeHTTP struct {
	t            *testing.T
	Method       string
	Host         string
	Path         string
	Params       map[string]string
	RequestData  []byte
	ResponseData []byte
}

func (f FakeHTTP) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	if resp, err = f.checkRequest(req); err != nil {
		f.t.Errorf("%s", err)
	}
	return resp, err
}

func (f FakeHTTP) checkRequest(r *http.Request) (*http.Response, error) {
	var va []string
	var ok bool

	if r.Method != f.Method {
		return nil, fmt.Errorf("wrong method")
	}
	if r.URL.Scheme != "https" || r.URL.Host != f.Host || r.URL.Path != f.Path {
		return nil, fmt.Errorf("wrong URL %s://%s%s", r.URL.Scheme, r.URL.Host, r.URL.Path)
	}
	vals := r.URL.Query()
	if len(vals) != len(f.Params) {
		return nil, fmt.Errorf("wrong number of parameters got %d expected %d", len(vals), len(f.Params))
	}
	for k, v := range f.Params {
		if va, ok = vals[k]; !ok || len(va) != 1 {
			return nil, fmt.Errorf("wrong parameters %s", k)
		} else if va[0] != v {
			return nil, fmt.Errorf("wrong parameters %s expected %s received %s", k, v, va[0])
		}
	}
	if len(f.RequestData) != 0 {
		var buf []byte
		var err error

		if buf, err = ioutil.ReadAll(r.Body); err != nil {
			return nil, err
		}
		if !bytes.Equal(buf, f.RequestData) {
			return nil, fmt.Errorf("wrong request body")
		}
	}

	return &http.Response{Status: "200 OK", StatusCode: 200,
		Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
		ContentLength: int64(len(f.ResponseData)), Body: ioutil.NopCloser(bytes.NewReader(f.ResponseData))}, nil
}

// Downloading a file
func Example() {
	db := NewDropbox()
	db.SetAppInfo("application id", "application secret")
	db.SetAccessToken("your secret token for this application")
	db.DownloadToFile("file on Dropbox", "local destination", "revision of the file on Dropbox")
}

func newDropbox(t *testing.T) *Dropbox {
	db := NewDropbox()
	db.SetAppInfo("dummyappkey", "dummyappsecret")
	db.SetAccessToken("dummyoauthtoken")
	return db
}

func TestAccountInfo(t *testing.T) {
	var err error
	var db *Dropbox
	var received *Account

	db = newDropbox(t)

	expected := Account{ReferralLink: "https://www.dropbox.com/referrals/r1a2n3d4m5s6t7", DisplayName: "John P. User", Country: "US", UID: 12345678}
	expected.QuotaInfo.Shared = 253738410565
	expected.QuotaInfo.Quota = 107374182400000
	expected.QuotaInfo.Normal = 680031877871
	js, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("could not run test marshalling issue")
	}

	http.DefaultClient = &http.Client{
		Transport: FakeHTTP{
			t:            t,
			Method:       "GET",
			Host:         "api.dropbox.com",
			Path:         "/1/account/info",
			Params:       map[string]string{"locale": "en"},
			ResponseData: js,
		},
	}

	if received, err = db.GetAccountInfo(); err != nil {
		t.Errorf("API error: %s", err)
	} else if !reflect.DeepEqual(expected, *received) {
		t.Errorf("got %#v expected %#v", *received, expected)
	}
}

func TestCopy(t *testing.T) {
	var err error
	var db *Dropbox
	var received *Entry
	var from, to string
	var fake FakeHTTP

	expected := fileEntry
	from = expected.Path[1:]
	to = from + ".1"
	expected.Path = to

	js, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("could not run test marshalling issue")
	}

	fake = FakeHTTP{
		t:      t,
		Method: "POST",
		Host:   "api.dropbox.com",
		Path:   "/1/fileops/copy",
		Params: map[string]string{
			"root":      "auto",
			"from_path": from,
			"to_path":   to,
			"locale":    "en",
		},
		ResponseData: js,
	}
	db = newDropbox(t)
	http.DefaultClient = &http.Client{
		Transport: fake,
	}

	if received, err = db.Copy(from, to, false); err != nil {
		t.Errorf("API error: %s", err)
	} else if !reflect.DeepEqual(expected, *received) {
		t.Errorf("got %#v expected %#v", *received, expected)
	}

	delete(fake.Params, "from_path")
	fake.Params["from_copy_ref"] = from
	if received, err = db.Copy(from, to, true); err != nil {
		t.Errorf("API error: %s", err)
	} else if !reflect.DeepEqual(expected, *received) {
		t.Errorf("got %#v expected %#v", *received, expected)
	}
}

func TestCopyRef(t *testing.T) {
	var err error
	var db *Dropbox
	var received *CopyRef
	var filename string

	filename = "dummyfile"
	db = newDropbox(t)

	expected := CopyRef{CopyRef: "z1X6ATl6aWtzOGq0c3g5Ng", Expires: "Fri, 31 Jan 2042 21:01:05 +0000"}
	js, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("could not run test due to marshalling issue")
	}

	http.DefaultClient = &http.Client{
		Transport: FakeHTTP{
			Method:       "GET",
			Host:         "api.dropbox.com",
			Path:         "/1/copy_ref/auto/" + filename,
			t:            t,
			Params:       map[string]string{"locale": "en"},
			ResponseData: js,
		},
	}
	if received, err = db.CopyRef(filename); err != nil {
		t.Errorf("API error: %s", err)
	} else if !reflect.DeepEqual(expected, *received) {
		t.Errorf("got %#v expected %#v", *received, expected)
	}
}

func TestCreateFolder(t *testing.T) {
	var err error
	var db *Dropbox
	var received *Entry
	var foldername string

	expected := dirEntry
	foldername = expected.Path[1:]

	js, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("could not run test due to marshalling issue")
	}

	db = newDropbox(t)
	http.DefaultClient = &http.Client{
		Transport: FakeHTTP{
			Method: "POST",
			Host:   "api.dropbox.com",
			Path:   "/1/fileops/create_folder",
			Params: map[string]string{
				"root":   "auto",
				"path":   foldername,
				"locale": "en",
			},
			t:            t,
			ResponseData: js,
		},
	}
	if received, err = db.CreateFolder(foldername); err != nil {
		t.Errorf("API error: %s", err)
	} else if !reflect.DeepEqual(expected, *received) {
		t.Errorf("got %#v expected %#v", *received, expected)
	}
}

func TestDelete(t *testing.T) {
	var err error
	var db *Dropbox
	var received *Entry
	var path string

	expected := dirEntry
	expected.IsDeleted = true
	path = expected.Path[1:]

	js, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("could not run test marshalling issue")
	}

	db = newDropbox(t)
	http.DefaultClient = &http.Client{
		Transport: FakeHTTP{
			t:      t,
			Method: "POST",
			Host:   "api.dropbox.com",
			Path:   "/1/fileops/delete",
			Params: map[string]string{
				"root":   "auto",
				"path":   path,
				"locale": "en",
			},
			ResponseData: js,
		},
	}
	if received, err = db.Delete(path); err != nil {
		t.Errorf("API error: %s", err)
	} else if !reflect.DeepEqual(expected, *received) {
		t.Errorf("got %#v expected %#v", *received, expected)
	}
}

func TestFilesPut(t *testing.T) {
	var err error
	var db *Dropbox
	var received *Entry
	var filename string
	var content, js []byte
	var fake FakeHTTP

	filename = "test.txt"
	content = []byte("file content")

	expected := Entry{Size: strconv.FormatInt(int64(len(content)), 10), Revision: "35e97029684fe", ThumbExists: false, Bytes: int64(len(content)),
		Modified: DBTime(time.Date(2011, time.July, 19, 21, 55, 38, 0, time.UTC)), Path: "/" + filename, IsDir: false, Icon: "page_white_text",
		Root: "auto", MimeType: "text/plain"}

	js, err = json.Marshal(expected)
	if err != nil {
		t.Fatalf("could not run test marshalling issue")
	}

	fake = FakeHTTP{
		t:      t,
		Method: "PUT",
		Host:   "api-content.dropbox.com",
		Path:   "/1/files_put/auto/" + filename,
		Params: map[string]string{
			"locale":    "en",
			"overwrite": "false",
		},
		ResponseData: js,
		RequestData:  content,
	}

	db = newDropbox(t)
	http.DefaultClient = &http.Client{
		Transport: fake,
	}

	received, err = db.FilesPut(ioutil.NopCloser(bytes.NewBuffer(content)), int64(len(content)), filename, false, "")
	if err != nil {
		t.Errorf("API error: %s", err)
	} else if !reflect.DeepEqual(expected, *received) {
		t.Errorf("got %#v expected %#v", *received, expected)
	}

	fake.Params["parent_rev"] = "12345"
	received, err = db.FilesPut(ioutil.NopCloser(bytes.NewBuffer(content)), int64(len(content)), filename, false, "12345")
	if err != nil {
		t.Errorf("API error: %s", err)
	} else if !reflect.DeepEqual(expected, *received) {
		t.Errorf("got %#v expected %#v", *received, expected)
	}

	fake.Params["overwrite"] = "true"
	received, err = db.FilesPut(ioutil.NopCloser(bytes.NewBuffer(content)), int64(len(content)), filename, true, "12345")
	if err != nil {
		t.Errorf("API error: %s", err)
	} else if !reflect.DeepEqual(expected, *received) {
		t.Errorf("got %#v expected %#v", *received, expected)
	}

	_, err = db.FilesPut(ioutil.NopCloser(bytes.NewBuffer(content)), int64(MaxPutFileSize+1), filename, true, "12345")
	if err == nil {
		t.Errorf("size > %d bytes must returns an error", MaxPutFileSize)
	}
}

func TestMedia(t *testing.T) {
	var err error
	var db *Dropbox
	var received *Link
	var filename string

	filename = "dummyfile"
	db = newDropbox(t)

	expected := Link{Expires: DBTime(time.Date(2011, time.August, 10, 18, 21, 30, 0, time.UTC)), URL: "https://dl.dropboxusercontent.com/1/view/abcdefghijk/example"}
	js, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("could not run test due to marshalling issue: %s", err)
	}

	http.DefaultClient = &http.Client{
		Transport: FakeHTTP{
			Method:       "POST",
			Host:         "api.dropbox.com",
			Path:         "/1/media/auto/" + filename,
			Params:       map[string]string{"locale": "en"},
			t:            t,
			ResponseData: js,
		},
	}
	if received, err = db.Media(filename); err != nil {
		t.Errorf("API error: %s", err)
	} else if !reflect.DeepEqual(expected, *received) {
		t.Errorf("got %#v expected %#v", *received, expected)
	}
}

func TestMetadata(t *testing.T) {
	var err error
	var db *Dropbox
	var received *Entry
	var path string
	var fake FakeHTTP

	expected := fileEntry
	path = expected.Path[1:]

	js, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("could not run test marshalling issue")
	}

	fake = FakeHTTP{
		t:      t,
		Method: "GET",
		Host:   "api.dropbox.com",
		Path:   "/1/metadata/auto/" + path,
		Params: map[string]string{
			"list":            "false",
			"include_deleted": "false",
			"file_limit":      "10",
			"locale":          "en",
		},
		ResponseData: js,
	}
	db = newDropbox(t)
	http.DefaultClient = &http.Client{
		Transport: fake,
	}

	if received, err = db.Metadata(path, false, false, "", "", 10); err != nil {
		t.Errorf("API error: %s", err)
	} else if !reflect.DeepEqual(expected, *received) {
		t.Errorf("got %#v expected %#v", *received, expected)
	}

	fake.Params["list"] = "true"
	if received, err = db.Metadata(path, true, false, "", "", 10); err != nil {
		t.Errorf("API error: %s", err)
	} else if !reflect.DeepEqual(expected, *received) {
		t.Errorf("got %#v expected %#v", *received, expected)
	}

	fake.Params["include_deleted"] = "true"
	if received, err = db.Metadata(path, true, true, "", "", 10); err != nil {
		t.Errorf("API error: %s", err)
	} else if !reflect.DeepEqual(expected, *received) {
		t.Errorf("got %#v expected %#v", *received, expected)
	}

	fake.Params["file_limit"] = "20"
	if received, err = db.Metadata(path, true, true, "", "", 20); err != nil {
		t.Errorf("API error: %s", err)
	} else if !reflect.DeepEqual(expected, *received) {
		t.Errorf("got %#v expected %#v", *received, expected)
	}

	fake.Params["rev"] = "12345"
	if received, err = db.Metadata(path, true, true, "", "12345", 20); err != nil {
		t.Errorf("API error: %s", err)
	} else if !reflect.DeepEqual(expected, *received) {
		t.Errorf("got %#v expected %#v", *received, expected)
	}

	fake.Params["hash"] = "6789"
	if received, err = db.Metadata(path, true, true, "6789", "12345", 20); err != nil {
		t.Errorf("API error: %s", err)
	} else if !reflect.DeepEqual(expected, *received) {
		t.Errorf("got %#v expected %#v", *received, expected)
	}

	fake.Params["file_limit"] = "10000"
	if received, err = db.Metadata(path, true, true, "6789", "12345", 0); err != nil {
		t.Errorf("API error: %s", err)
	} else if !reflect.DeepEqual(expected, *received) {
		t.Errorf("got %#v expected %#v", *received, expected)
	}

	fake.Params["file_limit"] = strconv.FormatInt(int64(MetadataLimitMax), 10)
	if received, err = db.Metadata(path, true, true, "6789", "12345", MetadataLimitMax+1); err != nil {
		t.Errorf("API error: %s", err)
	} else if !reflect.DeepEqual(expected, *received) {
		t.Errorf("got %#v expected %#v", *received, expected)
	}
}

func TestMove(t *testing.T) {
	var err error
	var db *Dropbox
	var received *Entry
	var from, to string

	expected := fileEntry
	from = expected.Path[1:]
	to = from + ".1"
	expected.Path = to

	js, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("could not run test marshalling issue")
	}

	db = newDropbox(t)
	http.DefaultClient = &http.Client{
		Transport: FakeHTTP{
			t:      t,
			Method: "POST",
			Host:   "api.dropbox.com",
			Path:   "/1/fileops/move",
			Params: map[string]string{
				"root":      "auto",
				"from_path": from,
				"to_path":   to,
				"locale":    "en",
			},
			ResponseData: js,
		},
	}
	if received, err = db.Move(from, to); err != nil {
		t.Errorf("API error: %s", err)
	} else if !reflect.DeepEqual(expected, *received) {
		t.Errorf("got %#v expected %#v", *received, expected)
	}
}

func TestRestore(t *testing.T) {
	var err error
	var db *Dropbox
	var received *Entry
	var path string

	expected := fileEntry
	path = expected.Path[1:]

	js, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("could not run test marshalling issue")
	}

	db = newDropbox(t)
	http.DefaultClient = &http.Client{
		Transport: FakeHTTP{
			t:      t,
			Method: "POST",
			Host:   "api.dropbox.com",
			Path:   "/1/restore/auto/" + path,
			Params: map[string]string{
				"rev":    expected.Revision,
				"locale": "en",
			},
			ResponseData: js,
		},
	}
	if received, err = db.Restore(path, expected.Revision); err != nil {
		t.Errorf("API error: %s", err)
	} else if !reflect.DeepEqual(expected, *received) {
		t.Errorf("got %#v expected %#v", *received, expected)
	}
}

func TestRevisions(t *testing.T) {
	var err error
	var db *Dropbox
	var received []Entry
	var path string
	var fake FakeHTTP

	expected := []Entry{fileEntry}
	path = expected[0].Path[1:]

	js, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("could not run test marshalling issue")
	}

	fake = FakeHTTP{
		t:      t,
		Method: "GET",
		Host:   "api.dropbox.com",
		Path:   "/1/revisions/auto/" + path,
		Params: map[string]string{
			"rev_limit": "10",
			"locale":    "en",
		},
		ResponseData: js,
	}
	db = newDropbox(t)
	http.DefaultClient = &http.Client{
		Transport: fake,
	}

	if received, err = db.Revisions(path, 10); err != nil {
		t.Errorf("API error: %s", err)
	} else if !reflect.DeepEqual(expected, received) {
		t.Errorf("got %#v expected %#v", received, expected)
	}

	fake.Params["rev_limit"] = strconv.FormatInt(int64(RevisionsLimitDefault), 10)
	if received, err = db.Revisions(path, 0); err != nil {
		t.Errorf("API error: %s", err)
	} else if !reflect.DeepEqual(expected, received) {
		t.Errorf("got %#v expected %#v", received, expected)
	}

	fake.Params["rev_limit"] = strconv.FormatInt(int64(RevisionsLimitMax), 10)
	if received, err = db.Revisions(path, RevisionsLimitMax+1); err != nil {
		t.Errorf("API error: %s", err)
	} else if !reflect.DeepEqual(expected, received) {
		t.Errorf("got %#v expected %#v", received, expected)
	}
}

func TestSearch(t *testing.T) {
	var err error
	var db *Dropbox
	var received []Entry
	var dirname string

	dirname = "dummy"
	db = newDropbox(t)

	expected := []Entry{Entry{Size: "0 bytes", Revision: "35c1f029684fe", ThumbExists: false, Bytes: 0,
		Modified: DBTime(time.Date(2011, time.August, 10, 18, 21, 30, 0, time.UTC)), Path: "/" + dirname + "/dummyfile", IsDir: false, Icon: "page_white_text",
		Root: "auto", MimeType: "text/plain"}}
	js, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("could not run test due to marshalling issue")
	}

	fake := FakeHTTP{
		Method: "GET",
		Host:   "api.dropbox.com",
		Path:   "/1/search/auto/" + dirname,
		t:      t,
		Params: map[string]string{
			"locale":          "en",
			"query":           "foo bar",
			"file_limit":      "10",
			"include_deleted": "false",
		},
		ResponseData: js,
	}
	http.DefaultClient = &http.Client{
		Transport: fake,
	}

	if received, err = db.Search(dirname, "foo bar", 10, false); err != nil {
		t.Errorf("API error: %s", err)
	} else if !reflect.DeepEqual(expected, received) {
		t.Errorf("got %#v expected %#v", received, expected)
	}

	fake.Params["include_deleted"] = "true"
	if received, err = db.Search(dirname, "foo bar", 10, true); err != nil {
		t.Errorf("API error: %s", err)
	} else if !reflect.DeepEqual(expected, received) {
		t.Errorf("got %#v expected %#v", received, expected)
	}

	fake.Params["file_limit"] = strconv.FormatInt(int64(SearchLimitDefault), 10)
	if received, err = db.Search(dirname, "foo bar", 0, true); err != nil {
		t.Errorf("API error: %s", err)
	} else if !reflect.DeepEqual(expected, received) {
		t.Errorf("got %#v expected %#v", received, expected)
	}

	if received, err = db.Search(dirname, "foo bar", SearchLimitMax+1, true); err != nil {
		t.Errorf("API error: %s", err)
	} else if !reflect.DeepEqual(expected, received) {
		t.Errorf("got %#v expected %#v", received, expected)
	}
}

func TestShares(t *testing.T) {
	var err error
	var db *Dropbox
	var received *Link
	var filename string

	filename = "dummyfile"
	db = newDropbox(t)

	expected := Link{Expires: DBTime(time.Date(2011, time.August, 10, 18, 21, 30, 0, time.UTC)), URL: "https://db.tt/c0mFuu1Y"}
	js, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("could not run test due to marshalling issue")
	}
	fake := FakeHTTP{
		Method: "POST",
		Host:   "api.dropbox.com",
		Path:   "/1/shares/auto/" + filename,
		Params: map[string]string{
			"locale":    "en",
			"short_url": "false",
		},
		t:            t,
		ResponseData: js,
	}
	http.DefaultClient = &http.Client{
		Transport: fake,
	}

	if received, err = db.Shares(filename, false); err != nil {
		t.Errorf("API error: %s", err)
	} else if !reflect.DeepEqual(expected, *received) {
		t.Errorf("got %#v expected %#v", *received, expected)
	}

	fake.Params["short_url"] = "true"
	if received, err = db.Shares(filename, true); err != nil {
		t.Errorf("API error: %s", err)
	} else if !reflect.DeepEqual(expected, *received) {
		t.Errorf("got %#v expected %#v", *received, expected)
	}
}

func TestLatestCursor(t *testing.T) {
	tab := []struct {
		prefix    string
		mediaInfo bool
	}{
		{
			prefix:    "",
			mediaInfo: false,
		},
		{
			prefix:    "/some",
			mediaInfo: false,
		},
		{
			prefix:    "",
			mediaInfo: true,
		},
		{
			prefix:    "/some",
			mediaInfo: true,
		},
	}

	expected := Cursor{Cursor: "some"}
	cur, err := json.Marshal(expected)
	if err != nil {
		t.Fatal("Failed to JSON encode Cursor")
	}

	for _, testCase := range tab {
		db := newDropbox(t)
		fake := FakeHTTP{
			Method: "POST",
			Host:   "api.dropbox.com",
			Path:   "/1/delta/latest_cursor",
			t:      t,
			Params: map[string]string{
				"locale": "en",
			},
			ResponseData: cur,
		}

		if testCase.prefix != "" {
			fake.Params["path_prefix"] = testCase.prefix
		}

		if testCase.mediaInfo {
			fake.Params["include_media_info"] = "true"
		}

		http.DefaultClient = &http.Client{
			Transport: fake,
		}

		if received, err := db.LatestCursor(testCase.prefix, testCase.mediaInfo); err != nil {
			t.Errorf("API error: %s", err)
		} else if !reflect.DeepEqual(expected, *received) {
			t.Errorf("got %#v expected %#v", *received, expected)
		}
	}
}
