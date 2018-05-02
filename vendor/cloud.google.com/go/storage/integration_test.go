// Copyright 2014 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package storage

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"hash/crc32"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/net/context"

	"cloud.google.com/go/iam"
	"cloud.google.com/go/internal/testutil"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"
	itesting "google.golang.org/api/iterator/testing"
	"google.golang.org/api/option"
)

const testPrefix = "go-integration-test"

var (
	uidSpace   = testutil.NewUIDSpace(testPrefix)
	bucketName = uidSpace.New()
)

func TestMain(m *testing.M) {
	integrationTest := initIntegrationTest()
	exit := m.Run()
	if integrationTest {
		if err := cleanup(); err != nil {
			// No need to be loud if cleanup() fails; we'll get
			// any undeleted buckets next time.
			log.Printf("Post-test cleanup failed: %v\n", err)
		}
	}
	os.Exit(exit)
}

// If integration tests will be run, create a unique bucket for them.
func initIntegrationTest() bool {
	flag.Parse() // needed for testing.Short()
	ctx := context.Background()
	if testing.Short() {
		return false
	}
	client := config(ctx)
	if client == nil {
		return false
	}
	defer client.Close()
	if err := client.Bucket(bucketName).Create(ctx, testutil.ProjID(), nil); err != nil {
		log.Fatalf("creating bucket %q: %v", bucketName, err)
	}
	return true
}

// testConfig returns the Client used to access GCS. testConfig skips
// the current test if credentials are not available or when being run
// in Short mode.
func testConfig(ctx context.Context, t *testing.T) *Client {
	if testing.Short() {
		t.Skip("Integration tests skipped in short mode")
	}
	client := config(ctx)
	if client == nil {
		t.Skip("Integration tests skipped. See CONTRIBUTING.md for details")
	}
	return client
}

// config is like testConfig, but it doesn't need a *testing.T.
func config(ctx context.Context) *Client {
	ts := testutil.TokenSource(ctx, ScopeFullControl)
	if ts == nil {
		return nil
	}
	p := testutil.ProjID()
	if p == "" {
		log.Fatal("The project ID must be set. See CONTRIBUTING.md for details")
	}
	client, err := NewClient(ctx, option.WithTokenSource(ts))
	if err != nil {
		log.Fatalf("NewClient: %v", err)
	}
	return client
}

func TestIntegration_BucketMethods(t *testing.T) {
	ctx := context.Background()
	client := testConfig(ctx, t)
	defer client.Close()

	projectID := testutil.ProjID()
	newBucketName := uidSpace.New()
	b := client.Bucket(newBucketName)
	// Test Create and Delete.
	if err := b.Create(ctx, projectID, nil); err != nil {
		t.Fatalf("Bucket(%v).Create(%v, %v) failed: %v", newBucketName, projectID, nil, err)
	}
	attrs, err := b.Attrs(ctx)
	if err != nil {
		t.Error(err)
	} else {
		if got, want := attrs.MetaGeneration, int64(1); got != want {
			t.Errorf("got metagen %d, want %d", got, want)
		}
		if got, want := attrs.StorageClass, "STANDARD"; got != want {
			t.Errorf("got storage class %q, want %q", got, want)
		}
		if attrs.VersioningEnabled {
			t.Error("got versioning enabled, wanted it disabled")
		}
	}
	if err := client.Bucket(newBucketName).Delete(ctx); err != nil {
		t.Errorf("Bucket(%v).Delete failed: %v", newBucketName, err)
	}

	// Test Create and Delete with attributes.
	labels := map[string]string{
		"l1":    "v1",
		"empty": "",
	}
	attrs = &BucketAttrs{
		StorageClass:      "NEARLINE",
		VersioningEnabled: true,
		Labels:            labels,
		Lifecycle: Lifecycle{
			Rules: []LifecycleRule{{
				Action: LifecycleAction{
					Type:         SetStorageClassAction,
					StorageClass: "NEARLINE",
				},
				Condition: LifecycleCondition{
					AgeInDays:             10,
					Liveness:              Archived,
					CreatedBefore:         time.Date(2017, 1, 1, 0, 0, 0, 0, time.UTC),
					MatchesStorageClasses: []string{"MULTI_REGIONAL", "STANDARD"},
					NumNewerVersions:      3,
				},
			}, {
				Action: LifecycleAction{
					Type: DeleteAction,
				},
				Condition: LifecycleCondition{
					AgeInDays:             30,
					Liveness:              Live,
					CreatedBefore:         time.Date(2017, 1, 1, 0, 0, 0, 0, time.UTC),
					MatchesStorageClasses: []string{"NEARLINE"},
					NumNewerVersions:      10,
				},
			}},
		},
	}
	if err := client.Bucket(newBucketName).Create(ctx, projectID, attrs); err != nil {
		t.Fatalf("Bucket(%v).Create(%v, %+v) failed: %v", newBucketName, projectID, attrs, err)
	}
	attrs, err = b.Attrs(ctx)
	if err != nil {
		t.Error(err)
	} else {
		if got, want := attrs.MetaGeneration, int64(1); got != want {
			t.Errorf("got metagen %d, want %d", got, want)
		}
		if got, want := attrs.StorageClass, "NEARLINE"; got != want {
			t.Errorf("got storage class %q, want %q", got, want)
		}
		if !attrs.VersioningEnabled {
			t.Error("got versioning disabled, wanted it enabled")
		}
		if got, want := attrs.Labels, labels; !testutil.Equal(got, want) {
			t.Errorf("labels: got %v, want %v", got, want)
		}
	}
	if err := client.Bucket(newBucketName).Delete(ctx); err != nil {
		t.Errorf("Bucket(%v).Delete failed: %v", newBucketName, err)
	}
}

func TestIntegration_BucketUpdate(t *testing.T) {
	ctx := context.Background()
	client := testConfig(ctx, t)
	defer client.Close()

	b := client.Bucket(bucketName)
	attrs, err := b.Attrs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if attrs.VersioningEnabled {
		t.Fatal("bucket should not have versioning by default")
	}
	if len(attrs.Labels) > 0 {
		t.Fatal("bucket should not have labels initially")
	}

	// Using empty BucketAttrsToUpdate should be a no-nop.
	attrs, err = b.Update(ctx, BucketAttrsToUpdate{})
	if err != nil {
		t.Fatal(err)
	}
	if attrs.VersioningEnabled {
		t.Fatal("should not have versioning")
	}
	if len(attrs.Labels) > 0 {
		t.Fatal("should not have labels")
	}

	// Turn on versioning, add some labels.
	ua := BucketAttrsToUpdate{VersioningEnabled: true}
	ua.SetLabel("l1", "v1")
	ua.SetLabel("empty", "")
	attrs, err = b.Update(ctx, ua)
	if err != nil {
		t.Fatal(err)
	}
	if !attrs.VersioningEnabled {
		t.Fatal("should have versioning now")
	}
	wantLabels := map[string]string{
		"l1":    "v1",
		"empty": "",
	}
	if !testutil.Equal(attrs.Labels, wantLabels) {
		t.Fatalf("got %v, want %v", attrs.Labels, wantLabels)
	}

	// Turn  off versioning again; add and remove some more labels.
	ua = BucketAttrsToUpdate{VersioningEnabled: false}
	ua.SetLabel("l1", "v2")   // update
	ua.SetLabel("new", "new") // create
	ua.DeleteLabel("empty")   // delete
	ua.DeleteLabel("absent")  // delete non-existent
	attrs, err = b.Update(ctx, ua)
	if err != nil {
		t.Fatal(err)
	}
	if attrs.VersioningEnabled {
		t.Fatal("should have versioning off")
	}
	wantLabels = map[string]string{
		"l1":  "v2",
		"new": "new",
	}
	if !testutil.Equal(attrs.Labels, wantLabels) {
		t.Fatalf("got %v, want %v", attrs.Labels, wantLabels)
	}
}

func TestIntegration_ConditionalDelete(t *testing.T) {
	ctx := context.Background()
	client := testConfig(ctx, t)
	defer client.Close()

	o := client.Bucket(bucketName).Object("conddel")

	wc := o.NewWriter(ctx)
	wc.ContentType = "text/plain"
	if _, err := wc.Write([]byte("foo")); err != nil {
		t.Fatal(err)
	}
	if err := wc.Close(); err != nil {
		t.Fatal(err)
	}

	gen := wc.Attrs().Generation
	metaGen := wc.Attrs().Metageneration

	if err := o.Generation(gen - 1).Delete(ctx); err == nil {
		t.Fatalf("Unexpected successful delete with Generation")
	}
	if err := o.If(Conditions{MetagenerationMatch: metaGen + 1}).Delete(ctx); err == nil {
		t.Fatalf("Unexpected successful delete with IfMetaGenerationMatch")
	}
	if err := o.If(Conditions{MetagenerationNotMatch: metaGen}).Delete(ctx); err == nil {
		t.Fatalf("Unexpected successful delete with IfMetaGenerationNotMatch")
	}
	if err := o.Generation(gen).Delete(ctx); err != nil {
		t.Fatalf("final delete failed: %v", err)
	}
}

func TestIntegration_Objects(t *testing.T) {
	// TODO(jba): Use subtests (Go 1.7).
	ctx := context.Background()
	client := testConfig(ctx, t)
	defer client.Close()

	bkt := client.Bucket(bucketName)

	const defaultType = "text/plain"

	// Populate object names and make a map for their contents.
	objects := []string{
		"obj1",
		"obj2",
		"obj/with/slashes",
	}
	contents := make(map[string][]byte)

	// Test Writer.
	for _, obj := range objects {
		c := randomContents()
		if err := writeObject(ctx, bkt.Object(obj), defaultType, c); err != nil {
			t.Errorf("Write for %v failed with %v", obj, err)
		}
		contents[obj] = c
	}

	testObjectIterator(t, bkt, objects)

	// Test Reader.
	for _, obj := range objects {
		rc, err := bkt.Object(obj).NewReader(ctx)
		if err != nil {
			t.Errorf("Can't create a reader for %v, errored with %v", obj, err)
			continue
		}
		if !rc.checkCRC {
			t.Errorf("%v: not checking CRC", obj)
		}
		slurp, err := ioutil.ReadAll(rc)
		if err != nil {
			t.Errorf("Can't ReadAll object %v, errored with %v", obj, err)
		}
		if got, want := slurp, contents[obj]; !bytes.Equal(got, want) {
			t.Errorf("Contents (%q) = %q; want %q", obj, got, want)
		}
		if got, want := rc.Size(), len(contents[obj]); got != int64(want) {
			t.Errorf("Size (%q) = %d; want %d", obj, got, want)
		}
		if got, want := rc.ContentType(), "text/plain"; got != want {
			t.Errorf("ContentType (%q) = %q; want %q", obj, got, want)
		}
		if got, want := rc.CacheControl(), "public, max-age=60"; got != want {
			t.Errorf("CacheControl (%q) = %q; want %q", obj, got, want)
		}
		rc.Close()

		// Check early close.
		buf := make([]byte, 1)
		rc, err = bkt.Object(obj).NewReader(ctx)
		if err != nil {
			t.Fatalf("%v: %v", obj, err)
		}
		_, err = rc.Read(buf)
		if err != nil {
			t.Fatalf("%v: %v", obj, err)
		}
		if got, want := buf, contents[obj][:1]; !bytes.Equal(got, want) {
			t.Errorf("Contents[0] (%q) = %q; want %q", obj, got, want)
		}
		if err := rc.Close(); err != nil {
			t.Errorf("%v Close: %v", obj, err)
		}
	}

	obj := objects[0]
	objlen := int64(len(contents[obj]))
	// Test Range Reader.
	for i, r := range []struct {
		offset, length, want int64
	}{
		{0, objlen, objlen},
		{0, objlen / 2, objlen / 2},
		{objlen / 2, objlen, objlen / 2},
		{0, 0, 0},
		{objlen / 2, 0, 0},
		{objlen / 2, -1, objlen / 2},
		{0, objlen * 2, objlen},
	} {
		rc, err := bkt.Object(obj).NewRangeReader(ctx, r.offset, r.length)
		if err != nil {
			t.Errorf("%d: Can't create a range reader for %v, errored with %v", i, obj, err)
			continue
		}
		if rc.Size() != objlen {
			t.Errorf("%d: Reader has a content-size of %d, want %d", i, rc.Size(), objlen)
		}
		if rc.Remain() != r.want {
			t.Errorf("%d: Reader's available bytes reported as %d, want %d", i, rc.Remain(), r.want)
		}
		slurp, err := ioutil.ReadAll(rc)
		if err != nil {
			t.Errorf("%d:Can't ReadAll object %v, errored with %v", i, obj, err)
			continue
		}
		if len(slurp) != int(r.want) {
			t.Errorf("%d:RangeReader (%d, %d): Read %d bytes, wanted %d bytes", i, r.offset, r.length, len(slurp), r.want)
			continue
		}
		if got, want := slurp, contents[obj][r.offset:r.offset+r.want]; !bytes.Equal(got, want) {
			t.Errorf("RangeReader (%d, %d) = %q; want %q", r.offset, r.length, got, want)
		}
		rc.Close()
	}

	// Test content encoding
	const zeroCount = 20 << 20
	w := bkt.Object("gzip-test").NewWriter(ctx)
	w.ContentEncoding = "gzip"
	gw := gzip.NewWriter(w)
	if _, err := io.Copy(gw, io.LimitReader(zeros{}, zeroCount)); err != nil {
		t.Fatalf("io.Copy, upload: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Errorf("gzip.Close(): %v", err)
	}
	if err := w.Close(); err != nil {
		t.Errorf("w.Close(): %v", err)
	}
	r, err := bkt.Object("gzip-test").NewReader(ctx)
	if err != nil {
		t.Fatalf("NewReader(gzip-test): %v", err)
	}
	n, err := io.Copy(ioutil.Discard, r)
	if err != nil {
		t.Errorf("io.Copy, download: %v", err)
	}
	if n != zeroCount {
		t.Errorf("downloaded bad data: got %d bytes, want %d", n, zeroCount)
	}

	// Test NotFound.
	_, err = bkt.Object("obj-not-exists").NewReader(ctx)
	if err != ErrObjectNotExist {
		t.Errorf("Object should not exist, err found to be %v", err)
	}

	objName := objects[0]

	// Test NewReader googleapi.Error.
	// Since a 429 or 5xx is hard to cause, we trigger a 416.
	realLen := len(contents[objName])
	_, err = bkt.Object(objName).NewRangeReader(ctx, int64(realLen*2), 10)
	if err, ok := err.(*googleapi.Error); !ok {
		t.Error("NewRangeReader did not return a googleapi.Error")
	} else {
		if err.Code != 416 {
			t.Errorf("Code = %d; want %d", err.Code, 416)
		}
		if len(err.Header) == 0 {
			t.Error("Missing googleapi.Error.Header")
		}
		if len(err.Body) == 0 {
			t.Error("Missing googleapi.Error.Body")
		}
	}

	// Test StatObject.
	o, err := bkt.Object(objName).Attrs(ctx)
	if err != nil {
		t.Error(err)
	}
	if got, want := o.Name, objName; got != want {
		t.Errorf("Name (%v) = %q; want %q", objName, got, want)
	}
	if got, want := o.ContentType, defaultType; got != want {
		t.Errorf("ContentType (%v) = %q; want %q", objName, got, want)
	}
	created := o.Created
	// Check that the object is newer than its containing bucket.
	bAttrs, err := bkt.Attrs(ctx)
	if err != nil {
		t.Error(err)
	}
	if o.Created.Before(bAttrs.Created) {
		t.Errorf("Object %v is older than its containing bucket, %v", o, bAttrs)
	}

	// Test object copy.
	copyName := "copy-" + objName
	copyObj, err := bkt.Object(copyName).CopierFrom(bkt.Object(objName)).Run(ctx)
	if err != nil {
		t.Errorf("Copier.Run failed with %v", err)
	} else if !namesEqual(copyObj, bucketName, copyName) {
		t.Errorf("Copy object bucket, name: got %q.%q, want %q.%q",
			copyObj.Bucket, copyObj.Name, bucketName, copyName)
	}

	// Copying with attributes.
	const contentEncoding = "identity"
	copier := bkt.Object(copyName).CopierFrom(bkt.Object(objName))
	copier.ContentEncoding = contentEncoding
	copyObj, err = copier.Run(ctx)
	if err != nil {
		t.Errorf("Copier.Run failed with %v", err)
	} else {
		if !namesEqual(copyObj, bucketName, copyName) {
			t.Errorf("Copy object bucket, name: got %q.%q, want %q.%q",
				copyObj.Bucket, copyObj.Name, bucketName, copyName)
		}
		if copyObj.ContentEncoding != contentEncoding {
			t.Errorf("Copy ContentEncoding: got %q, want %q", copyObj.ContentEncoding, contentEncoding)
		}
	}

	// Test UpdateAttrs.
	metadata := map[string]string{"key": "value"}
	updated, err := bkt.Object(objName).Update(ctx, ObjectAttrsToUpdate{
		ContentType:     "text/html",
		ContentLanguage: "en",
		Metadata:        metadata,
		ACL:             []ACLRule{{Entity: "domain-google.com", Role: RoleReader}},
	})
	if err != nil {
		t.Errorf("UpdateAttrs failed with %v", err)
	} else {
		if got, want := updated.ContentType, "text/html"; got != want {
			t.Errorf("updated.ContentType == %q; want %q", got, want)
		}
		if got, want := updated.ContentLanguage, "en"; got != want {
			t.Errorf("updated.ContentLanguage == %q; want %q", updated.ContentLanguage, want)
		}
		if got, want := updated.Metadata, metadata; !testutil.Equal(got, want) {
			t.Errorf("updated.Metadata == %+v; want %+v", updated.Metadata, want)
		}
		if got, want := updated.Created, created; got != want {
			t.Errorf("updated.Created == %q; want %q", got, want)
		}
		if !updated.Created.Before(updated.Updated) {
			t.Errorf("updated.Updated should be newer than update.Created")
		}
	}
	// Delete ContentType and ContentLanguage.
	updated, err = bkt.Object(objName).Update(ctx, ObjectAttrsToUpdate{
		ContentType:     "",
		ContentLanguage: "",
		Metadata:        map[string]string{},
	})
	if err != nil {
		t.Errorf("UpdateAttrs failed with %v", err)
	} else {
		if got, want := updated.ContentType, ""; got != want {
			t.Errorf("updated.ContentType == %q; want %q", got, want)
		}
		if got, want := updated.ContentLanguage, ""; got != want {
			t.Errorf("updated.ContentLanguage == %q; want %q", updated.ContentLanguage, want)
		}
		if updated.Metadata != nil {
			t.Errorf("updated.Metadata == %+v; want nil", updated.Metadata)
		}
		if got, want := updated.Created, created; got != want {
			t.Errorf("updated.Created == %q; want %q", got, want)
		}
		if !updated.Created.Before(updated.Updated) {
			t.Errorf("updated.Updated should be newer than update.Created")
		}
	}

	// Test checksums.
	checksumCases := []struct {
		name     string
		contents [][]byte
		size     int64
		md5      string
		crc32c   uint32
	}{
		{
			name:     "checksum-object",
			contents: [][]byte{[]byte("hello"), []byte("world")},
			size:     10,
			md5:      "fc5e038d38a57032085441e7fe7010b0",
			crc32c:   1456190592,
		},
		{
			name:     "zero-object",
			contents: [][]byte{},
			size:     0,
			md5:      "d41d8cd98f00b204e9800998ecf8427e",
			crc32c:   0,
		},
	}
	for _, c := range checksumCases {
		wc := bkt.Object(c.name).NewWriter(ctx)
		for _, data := range c.contents {
			if _, err := wc.Write(data); err != nil {
				t.Errorf("Write(%q) failed with %q", data, err)
			}
		}
		if err = wc.Close(); err != nil {
			t.Errorf("%q: close failed with %q", c.name, err)
		}
		obj := wc.Attrs()
		if got, want := obj.Size, c.size; got != want {
			t.Errorf("Object (%q) Size = %v; want %v", c.name, got, want)
		}
		if got, want := fmt.Sprintf("%x", obj.MD5), c.md5; got != want {
			t.Errorf("Object (%q) MD5 = %q; want %q", c.name, got, want)
		}
		if got, want := obj.CRC32C, c.crc32c; got != want {
			t.Errorf("Object (%q) CRC32C = %v; want %v", c.name, got, want)
		}
	}

	// Test public ACL.
	publicObj := objects[0]
	if err = bkt.Object(publicObj).ACL().Set(ctx, AllUsers, RoleReader); err != nil {
		t.Errorf("PutACLEntry failed with %v", err)
	}
	publicClient, err := NewClient(ctx, option.WithHTTPClient(http.DefaultClient))
	if err != nil {
		t.Fatal(err)
	}

	slurp, err := readObject(ctx, publicClient.Bucket(bucketName).Object(publicObj))
	if err != nil {
		t.Errorf("readObject failed with %v", err)
	} else if !bytes.Equal(slurp, contents[publicObj]) {
		t.Errorf("Public object's content: got %q, want %q", slurp, contents[publicObj])
	}

	// Test writer error handling.
	wc := publicClient.Bucket(bucketName).Object(publicObj).NewWriter(ctx)
	if _, err := wc.Write([]byte("hello")); err != nil {
		t.Errorf("Write unexpectedly failed with %v", err)
	}
	if err = wc.Close(); err == nil {
		t.Error("Close expected an error, found none")
	}

	// Test deleting the copy object.
	if err := bkt.Object(copyName).Delete(ctx); err != nil {
		t.Errorf("Deletion of %v failed with %v", copyName, err)
	}
	// Deleting it a second time should return ErrObjectNotExist.
	if err := bkt.Object(copyName).Delete(ctx); err != ErrObjectNotExist {
		t.Errorf("second deletion of %v = %v; want ErrObjectNotExist", copyName, err)
	}
	_, err = bkt.Object(copyName).Attrs(ctx)
	if err != ErrObjectNotExist {
		t.Errorf("Copy is expected to be deleted, stat errored with %v", err)
	}

	// Test object composition.
	var compSrcs []*ObjectHandle
	var wantContents []byte
	for _, obj := range objects {
		compSrcs = append(compSrcs, bkt.Object(obj))
		wantContents = append(wantContents, contents[obj]...)
	}
	checkCompose := func(obj *ObjectHandle, wantContentType string) {
		rc, err := obj.NewReader(ctx)
		if err != nil {
			t.Fatalf("NewReader: %v", err)
		}
		slurp, err = ioutil.ReadAll(rc)
		if err != nil {
			t.Fatalf("ioutil.ReadAll: %v", err)
		}
		defer rc.Close()
		if !bytes.Equal(slurp, wantContents) {
			t.Errorf("Composed object contents\ngot:  %q\nwant: %q", slurp, wantContents)
		}
		if got := rc.ContentType(); got != wantContentType {
			t.Errorf("Composed object content-type = %q, want %q", got, wantContentType)
		}
	}

	// Compose should work even if the user sets no destination attributes.
	compDst := bkt.Object("composed1")
	c := compDst.ComposerFrom(compSrcs...)
	if _, err := c.Run(ctx); err != nil {
		t.Fatalf("ComposeFrom error: %v", err)
	}
	checkCompose(compDst, "application/octet-stream")

	// It should also work if we do.
	compDst = bkt.Object("composed2")
	c = compDst.ComposerFrom(compSrcs...)
	c.ContentType = "text/json"
	if _, err := c.Run(ctx); err != nil {
		t.Fatalf("ComposeFrom error: %v", err)
	}
	checkCompose(compDst, "text/json")
}

func namesEqual(obj *ObjectAttrs, bucketName, objectName string) bool {
	return obj.Bucket == bucketName && obj.Name == objectName
}

func testObjectIterator(t *testing.T, bkt *BucketHandle, objects []string) {
	ctx := context.Background()
	// Collect the list of items we expect: ObjectAttrs in lexical order by name.
	names := make([]string, len(objects))
	copy(names, objects)
	sort.Strings(names)
	var attrs []*ObjectAttrs
	for _, name := range names {
		attr, err := bkt.Object(name).Attrs(ctx)
		if err != nil {
			t.Errorf("Object(%q).Attrs: %v", name, err)
			return
		}
		attrs = append(attrs, attr)
	}
	msg, ok := itesting.TestIterator(attrs,
		func() interface{} { return bkt.Objects(ctx, &Query{Prefix: "obj"}) },
		func(it interface{}) (interface{}, error) { return it.(*ObjectIterator).Next() })
	if !ok {
		t.Errorf("ObjectIterator.Next: %s", msg)
	}
	// TODO(jba): test query.Delimiter != ""
}

func TestIntegration_SignedURL(t *testing.T) {
	// To test SignedURL, we need a real user email and private key. Extract them
	// from the JSON key file.
	jwtConf, err := testutil.JWTConfig()
	if err != nil {
		t.Fatal(err)
	}
	if jwtConf == nil {
		t.Skip("JSON key file is not present")
	}

	ctx := context.Background()
	client := testConfig(ctx, t)
	defer client.Close()

	bkt := client.Bucket(bucketName)
	obj := "signedURL"
	contents := []byte("This is a test of SignedURL.\n")
	md5 := "Jyxvgwm9n2MsrGTMPbMeYA==" // base64-encoded MD5 of contents
	if err := writeObject(ctx, bkt.Object(obj), "text/plain", contents); err != nil {
		t.Fatalf("writing: %v", err)
	}
	for _, test := range []struct {
		desc    string
		opts    SignedURLOptions
		headers map[string][]string
		fail    bool
	}{
		{
			desc: "basic",
		},
		{
			desc:    "MD5 sent and matches",
			opts:    SignedURLOptions{MD5: md5},
			headers: map[string][]string{"Content-MD5": {md5}},
		},
		{
			desc: "MD5 not sent",
			opts: SignedURLOptions{MD5: md5},
			fail: true,
		},
		{
			desc:    "Content-Type sent and matches",
			opts:    SignedURLOptions{ContentType: "text/plain"},
			headers: map[string][]string{"Content-Type": {"text/plain"}},
		},
		{
			desc:    "Content-Type sent but does not match",
			opts:    SignedURLOptions{ContentType: "text/plain"},
			headers: map[string][]string{"Content-Type": {"application/json"}},
			fail:    true,
		},
		{
			desc: "Canonical headers sent and match",
			opts: SignedURLOptions{Headers: []string{
				" X-Goog-Foo: Bar baz ",
				"X-Goog-Novalue", // ignored: no value
				"X-Google-Foo",   // ignored: wrong prefix
			}},
			headers: map[string][]string{"X-Goog-foo": {"Bar baz  "}},
		},
		{
			desc:    "Canonical headers sent but don't match",
			opts:    SignedURLOptions{Headers: []string{" X-Goog-Foo: Bar baz"}},
			headers: map[string][]string{"X-Goog-Foo": {"bar baz"}},
			fail:    true,
		},
	} {
		opts := test.opts
		opts.GoogleAccessID = jwtConf.Email
		opts.PrivateKey = jwtConf.PrivateKey
		opts.Method = "GET"
		opts.Expires = time.Now().Add(time.Hour)
		u, err := SignedURL(bucketName, obj, &opts)
		if err != nil {
			t.Errorf("%s: SignedURL: %v", test.desc, err)
			continue
		}
		got, err := getURL(u, test.headers)
		if err != nil && !test.fail {
			t.Errorf("%s: getURL %q: %v", test.desc, u, err)
		} else if err == nil && !bytes.Equal(got, contents) {
			t.Errorf("%s: got %q, want %q", test.desc, got, contents)
		}
	}
}

// Make a GET request to a URL using an unauthenticated client, and return its contents.
func getURL(url string, headers map[string][]string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header = headers
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("code=%d, body=%s", res.StatusCode, string(bytes))
	}
	return bytes, nil
}

func TestIntegration_ACL(t *testing.T) {
	ctx := context.Background()
	client := testConfig(ctx, t)
	defer client.Close()

	bkt := client.Bucket(bucketName)

	entity := ACLEntity("domain-google.com")
	rule := ACLRule{Entity: entity, Role: RoleReader}
	if err := bkt.DefaultObjectACL().Set(ctx, entity, RoleReader); err != nil {
		t.Errorf("Can't put default ACL rule for the bucket, errored with %v", err)
	}
	acl, err := bkt.DefaultObjectACL().List(ctx)
	if err != nil {
		t.Errorf("DefaultObjectACL.List for bucket %q: %v", bucketName, err)
	} else if !hasRule(acl, rule) {
		t.Errorf("default ACL missing %#v", rule)
	}
	aclObjects := []string{"acl1", "acl2"}
	for _, obj := range aclObjects {
		c := randomContents()
		if err := writeObject(ctx, bkt.Object(obj), "", c); err != nil {
			t.Errorf("Write for %v failed with %v", obj, err)
		}
	}
	name := aclObjects[0]
	o := bkt.Object(name)
	acl, err = o.ACL().List(ctx)
	if err != nil {
		t.Errorf("Can't retrieve ACL of %v", name)
	} else if !hasRule(acl, rule) {
		t.Errorf("object ACL missing %+v", rule)
	}
	if err := o.ACL().Delete(ctx, entity); err != nil {
		t.Errorf("object ACL: could not delete entity %s", entity)
	}
	// Delete the default ACL rule. We can't move this code earlier in the
	// test, because the test depends on the fact that the object ACL inherits
	// it.
	if err := bkt.DefaultObjectACL().Delete(ctx, entity); err != nil {
		t.Errorf("default ACL: could not delete entity %s", entity)
	}

	entity2 := ACLEntity("user-jbd@google.com")
	rule2 := ACLRule{Entity: entity2, Role: RoleReader}
	if err := bkt.ACL().Set(ctx, entity2, RoleReader); err != nil {
		t.Errorf("Error while putting bucket ACL rule: %v", err)
	}
	bACL, err := bkt.ACL().List(ctx)
	if err != nil {
		t.Errorf("Error while getting the ACL of the bucket: %v", err)
	} else if !hasRule(bACL, rule2) {
		t.Errorf("bucket ACL missing %+v", rule2)
	}
	if err := bkt.ACL().Delete(ctx, entity2); err != nil {
		t.Errorf("Error while deleting bucket ACL rule: %v", err)
	}

}

func hasRule(acl []ACLRule, rule ACLRule) bool {
	for _, r := range acl {
		if r == rule {
			return true
		}
	}
	return false
}

func TestIntegration_ValidObjectNames(t *testing.T) {
	ctx := context.Background()
	client := testConfig(ctx, t)
	defer client.Close()

	bkt := client.Bucket(bucketName)

	validNames := []string{
		"gopher",
		"Гоферови",
		"a",
		strings.Repeat("a", 1024),
	}
	for _, name := range validNames {
		if err := writeObject(ctx, bkt.Object(name), "", []byte("data")); err != nil {
			t.Errorf("Object %q write failed: %v. Want success", name, err)
			continue
		}
		defer bkt.Object(name).Delete(ctx)
	}

	invalidNames := []string{
		"", // Too short.
		strings.Repeat("a", 1025), // Too long.
		"new\nlines",
		"bad\xffunicode",
	}
	for _, name := range invalidNames {
		// Invalid object names will either cause failure during Write or Close.
		if err := writeObject(ctx, bkt.Object(name), "", []byte("data")); err != nil {
			continue
		}
		defer bkt.Object(name).Delete(ctx)
		t.Errorf("%q should have failed. Didn't", name)
	}
}

func TestIntegration_WriterContentType(t *testing.T) {
	ctx := context.Background()
	client := testConfig(ctx, t)
	defer client.Close()

	obj := client.Bucket(bucketName).Object("content")
	testCases := []struct {
		content           string
		setType, wantType string
	}{
		{
			content:  "It was the best of times, it was the worst of times.",
			wantType: "text/plain; charset=utf-8",
		},
		{
			content:  "<html><head><title>My first page</title></head></html>",
			wantType: "text/html; charset=utf-8",
		},
		{
			content:  "<html><head><title>My first page</title></head></html>",
			setType:  "text/html",
			wantType: "text/html",
		},
		{
			content:  "<html><head><title>My first page</title></head></html>",
			setType:  "image/jpeg",
			wantType: "image/jpeg",
		},
	}
	for i, tt := range testCases {
		if err := writeObject(ctx, obj, tt.setType, []byte(tt.content)); err != nil {
			t.Errorf("writing #%d: %v", i, err)
		}
		attrs, err := obj.Attrs(ctx)
		if err != nil {
			t.Errorf("obj.Attrs: %v", err)
			continue
		}
		if got := attrs.ContentType; got != tt.wantType {
			t.Errorf("Content-Type = %q; want %q\nContent: %q\nSet Content-Type: %q", got, tt.wantType, tt.content, tt.setType)
		}
	}
}

func TestIntegration_ZeroSizedObject(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := testConfig(ctx, t)
	defer client.Close()

	obj := client.Bucket(bucketName).Object("zero")

	// Check writing it works as expected.
	w := obj.NewWriter(ctx)
	if err := w.Close(); err != nil {
		t.Fatalf("Writer.Close: %v", err)
	}
	defer obj.Delete(ctx)

	// Check we can read it too.
	body, err := readObject(ctx, obj)
	if err != nil {
		t.Fatalf("readObject: %v", err)
	}
	if len(body) != 0 {
		t.Errorf("Body is %v, want empty []byte{}", body)
	}
}

func TestIntegration_Encryption(t *testing.T) {
	// This function tests customer-supplied encryption keys for all operations
	// involving objects. Bucket and ACL operations aren't tested because they
	// aren't affected by customer encryption. Neither is deletion.
	ctx := context.Background()
	client := testConfig(ctx, t)
	defer client.Close()

	obj := client.Bucket(bucketName).Object("customer-encryption")
	key := []byte("my-secret-AES-256-encryption-key")
	keyHash := sha256.Sum256(key)
	keyHashB64 := base64.StdEncoding.EncodeToString(keyHash[:])
	key2 := []byte("My-Secret-AES-256-Encryption-Key")
	contents := "top secret."

	checkMetadataCall := func(msg string, f func(o *ObjectHandle) (*ObjectAttrs, error)) {
		// Performing a metadata operation without the key should succeed.
		attrs, err := f(obj)
		if err != nil {
			t.Fatalf("%s: %v", msg, err)
		}
		// The key hash should match...
		if got, want := attrs.CustomerKeySHA256, keyHashB64; got != want {
			t.Errorf("%s: key hash: got %q, want %q", msg, got, want)
		}
		// ...but CRC and MD5 should not be present.
		if attrs.CRC32C != 0 {
			t.Errorf("%s: CRC: got %v, want 0", msg, attrs.CRC32C)
		}
		if len(attrs.MD5) > 0 {
			t.Errorf("%s: MD5: got %v, want len == 0", msg, attrs.MD5)
		}

		// Performing a metadata operation with the key should succeed.
		attrs, err = f(obj.Key(key))
		if err != nil {
			t.Fatalf("%s: %v", msg, err)
		}
		// Check the key and content hashes.
		if got, want := attrs.CustomerKeySHA256, keyHashB64; got != want {
			t.Errorf("%s: key hash: got %q, want %q", msg, got, want)
		}
		if attrs.CRC32C == 0 {
			t.Errorf("%s: CRC: got 0, want non-zero", msg)
		}
		if len(attrs.MD5) == 0 {
			t.Errorf("%s: MD5: got len == 0, want len > 0", msg)
		}
	}

	checkRead := func(msg string, o *ObjectHandle, k []byte, wantContents string) {
		// Reading the object without the key should fail.
		if _, err := readObject(ctx, o); err == nil {
			t.Errorf("%s: reading without key: want error, got nil", msg)
		}
		// Reading the object with the key should succeed.
		got, err := readObject(ctx, o.Key(k))
		if err != nil {
			t.Fatalf("%s: %v", msg, err)
		}
		gotContents := string(got)
		// And the contents should match what we wrote.
		if gotContents != wantContents {
			t.Errorf("%s: contents: got %q, want %q", msg, gotContents, wantContents)
		}
	}

	checkReadUnencrypted := func(msg string, obj *ObjectHandle, wantContents string) {
		got, err := readObject(ctx, obj)
		if err != nil {
			t.Fatalf("%s: %v", msg, err)
		}
		gotContents := string(got)
		if gotContents != wantContents {
			t.Errorf("%s: got %q, want %q", msg, gotContents, wantContents)
		}
	}

	// Write to obj using our own encryption key, which is a valid 32-byte
	// AES-256 key.
	w := obj.Key(key).NewWriter(ctx)
	w.Write([]byte(contents))
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	checkMetadataCall("Attrs", func(o *ObjectHandle) (*ObjectAttrs, error) {
		return o.Attrs(ctx)
	})

	checkMetadataCall("Update", func(o *ObjectHandle) (*ObjectAttrs, error) {
		return o.Update(ctx, ObjectAttrsToUpdate{ContentLanguage: "en"})
	})

	checkRead("first object", obj, key, contents)

	obj2 := client.Bucket(bucketName).Object("customer-encryption-2")
	// Copying an object without the key should fail.
	if _, err := obj2.CopierFrom(obj).Run(ctx); err == nil {
		t.Fatal("want error, got nil")
	}
	// Copying an object with the key should succeed.
	if _, err := obj2.CopierFrom(obj.Key(key)).Run(ctx); err != nil {
		t.Fatal(err)
	}
	// The destination object is not encrypted; we can read it without a key.
	checkReadUnencrypted("copy dest", obj2, contents)

	// Providing a key on the destination but not the source should fail,
	// since the source is encrypted.
	if _, err := obj2.Key(key2).CopierFrom(obj).Run(ctx); err == nil {
		t.Fatal("want error, got nil")
	}

	// But copying with keys for both source and destination should succeed.
	if _, err := obj2.Key(key2).CopierFrom(obj.Key(key)).Run(ctx); err != nil {
		t.Fatal(err)
	}
	// And the destination should be encrypted, meaning we can only read it
	// with a key.
	checkRead("copy destination", obj2, key2, contents)

	// Change obj2's key to prepare for compose, where all objects must have
	// the same key. Also illustrates key rotation: copy an object to itself
	// with a different key.
	if _, err := obj2.Key(key).CopierFrom(obj2.Key(key2)).Run(ctx); err != nil {
		t.Fatal(err)
	}
	obj3 := client.Bucket(bucketName).Object("customer-encryption-3")
	// Composing without keys should fail.
	if _, err := obj3.ComposerFrom(obj, obj2).Run(ctx); err == nil {
		t.Fatal("want error, got nil")
	}
	// Keys on the source objects result in an error.
	if _, err := obj3.ComposerFrom(obj.Key(key), obj2).Run(ctx); err == nil {
		t.Fatal("want error, got nil")
	}
	// A key on the destination object both decrypts the source objects
	// and encrypts the destination.
	if _, err := obj3.Key(key).ComposerFrom(obj, obj2).Run(ctx); err != nil {
		t.Fatalf("got %v, want nil", err)
	}
	// Check that the destination in encrypted.
	checkRead("compose destination", obj3, key, contents+contents)

	// You can't compose one or more unencrypted source objects into an
	// encrypted destination object.
	_, err := obj2.CopierFrom(obj2.Key(key)).Run(ctx) // unencrypt obj2
	if err != nil {
		t.Fatal(err)
	}
	if _, err := obj3.Key(key).ComposerFrom(obj2).Run(ctx); err == nil {
		t.Fatal("got nil, want error")
	}
}

func TestIntegration_NonexistentBucket(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := testConfig(ctx, t)
	defer client.Close()

	bkt := client.Bucket(uidSpace.New())
	if _, err := bkt.Attrs(ctx); err != ErrBucketNotExist {
		t.Errorf("Attrs: got %v, want ErrBucketNotExist", err)
	}
	it := bkt.Objects(ctx, nil)
	if _, err := it.Next(); err != ErrBucketNotExist {
		t.Errorf("Objects: got %v, want ErrBucketNotExist", err)
	}
}

func TestIntegration_PerObjectStorageClass(t *testing.T) {
	const (
		defaultStorageClass = "STANDARD"
		newStorageClass     = "MULTI_REGIONAL"
	)
	ctx := context.Background()
	client := testConfig(ctx, t)
	defer client.Close()

	bkt := client.Bucket(bucketName)

	// The bucket should have the default storage class.
	battrs, err := bkt.Attrs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if battrs.StorageClass != defaultStorageClass {
		t.Fatalf("bucket storage class: got %q, want %q",
			battrs.StorageClass, defaultStorageClass)
	}
	// Write an object; it should start with the bucket's storage class.
	obj := bkt.Object("posc")
	if err := writeObject(ctx, obj, "", []byte("foo")); err != nil {
		t.Fatal(err)
	}
	oattrs, err := obj.Attrs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if oattrs.StorageClass != defaultStorageClass {
		t.Fatalf("object storage class: got %q, want %q",
			oattrs.StorageClass, defaultStorageClass)
	}
	// Now use Copy to change the storage class.
	copier := obj.CopierFrom(obj)
	copier.StorageClass = newStorageClass
	oattrs2, err := copier.Run(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if oattrs2.StorageClass != newStorageClass {
		t.Fatalf("new object storage class: got %q, want %q",
			oattrs2.StorageClass, newStorageClass)
	}

	// We can also write a new object using a non-default storage class.
	obj2 := bkt.Object("posc2")
	w := obj2.NewWriter(ctx)
	w.StorageClass = newStorageClass
	if _, err := w.Write([]byte("xxx")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if w.Attrs().StorageClass != newStorageClass {
		t.Fatalf("new object storage class: got %q, want %q",
			w.Attrs().StorageClass, newStorageClass)
	}
}

func TestIntegration_BucketInCopyAttrs(t *testing.T) {
	// Confirm that if bucket is included in the object attributes of a rewrite
	// call, but object name and content-type aren't, then we get an error. See
	// the comment in Copier.Run.
	ctx := context.Background()
	client := testConfig(ctx, t)
	defer client.Close()

	bkt := client.Bucket(bucketName)
	obj := bkt.Object("bucketInCopyAttrs")
	if err := writeObject(ctx, obj, "", []byte("foo")); err != nil {
		t.Fatal(err)
	}
	copier := obj.CopierFrom(obj)
	rawObject := copier.ObjectAttrs.toRawObject(bucketName)
	_, err := copier.callRewrite(ctx, rawObject)
	if err == nil {
		t.Errorf("got nil, want error")
	}
}

func TestIntegration_NoUnicodeNormalization(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := testConfig(ctx, t)
	defer client.Close()
	bkt := client.Bucket("storage-library-test-bucket")

	for _, tst := range []struct {
		nameQuoted, content string
	}{
		{`"Caf\u00e9"`, "Normalization Form C"},
		{`"Cafe\u0301"`, "Normalization Form D"},
	} {
		name, err := strconv.Unquote(tst.nameQuoted)
		if err != nil {
			t.Fatalf("invalid name: %s: %v", tst.nameQuoted, err)
		}
		got, err := readObject(ctx, bkt.Object(name))
		if err != nil {
			t.Fatal(err)
		}
		if g := string(got); g != tst.content {
			t.Errorf("content of %s is %q, want %q", tst.nameQuoted, g, tst.content)
		}
	}
}

func TestIntegration_HashesOnUpload(t *testing.T) {
	// Check that the user can provide hashes on upload, and that these are checked.
	if testing.Short() {
		t.Skip("Integration tests skipped in short mode")
	}
	ctx := context.Background()
	client := testConfig(ctx, t)
	if client == nil {
		t.Skip("Integration tests skipped. See CONTRIBUTING.md for details")
	}
	defer client.Close()
	obj := client.Bucket(bucketName).Object("hashesOnUpload-1")
	data := []byte("I can't wait to be verified")

	write := func(w *Writer) error {
		if _, err := w.Write(data); err != nil {
			w.Close()
			return err
		}
		return w.Close()
	}

	crc32c := crc32.Checksum(data, crc32cTable)
	// The correct CRC should succeed.
	w := obj.NewWriter(ctx)
	w.CRC32C = crc32c
	w.SendCRC32C = true
	if err := write(w); err != nil {
		t.Fatal(err)
	}

	// If we change the CRC, validation should fail.
	w = obj.NewWriter(ctx)
	w.CRC32C = crc32c + 1
	w.SendCRC32C = true
	if err := write(w); err == nil {
		t.Fatal("write with bad CRC32c: want error, got nil")
	}

	// If we have the wrong CRC but forget to send it, we succeed.
	w = obj.NewWriter(ctx)
	w.CRC32C = crc32c + 1
	if err := write(w); err != nil {
		t.Fatal(err)
	}

	// MD5
	md5 := md5.Sum(data)
	// The correct MD5 should succeed.
	w = obj.NewWriter(ctx)
	w.MD5 = md5[:]
	if err := write(w); err != nil {
		t.Fatal(err)
	}

	// If we change the MD5, validation should fail.
	w = obj.NewWriter(ctx)
	w.MD5 = append([]byte(nil), md5[:]...)
	w.MD5[0]++
	if err := write(w); err == nil {
		t.Fatal("write with bad MD5: want error, got nil")
	}
}

func TestIntegration_BucketIAM(t *testing.T) {
	ctx := context.Background()
	client := testConfig(ctx, t)
	defer client.Close()

	bkt := client.Bucket(bucketName)

	// This bucket is unique to this test run. So we don't have
	// to worry about other runs interfering with our IAM policy
	// changes.

	member := "projectViewer:" + testutil.ProjID()
	role := iam.RoleName("roles/storage.objectViewer")
	// Get the bucket's IAM policy.
	policy, err := bkt.IAM().Policy(ctx)
	if err != nil {
		t.Fatalf("Getting policy: %v", err)
	}
	// The member should not have the role.
	if policy.HasRole(member, role) {
		t.Errorf("member %q has role %q", member, role)
	}
	// Change the policy.
	policy.Add(member, role)
	if err := bkt.IAM().SetPolicy(ctx, policy); err != nil {
		t.Fatalf("SetPolicy: %v", err)
	}
	// Confirm that the binding was added.
	policy, err = bkt.IAM().Policy(ctx)
	if err != nil {
		t.Fatalf("Getting policy: %v", err)
	}
	if !policy.HasRole(member, role) {
		t.Errorf("member %q does not have role %q", member, role)
	}

	// Check TestPermissions.
	// This client should have all these permissions (and more).
	perms := []string{"storage.buckets.get", "storage.buckets.delete"}
	got, err := bkt.IAM().TestPermissions(ctx, perms)
	if err != nil {
		t.Fatalf("TestPermissions: %v", err)
	}
	sort.Strings(perms)
	sort.Strings(got)
	if !testutil.Equal(got, perms) {
		t.Errorf("got %v, want %v", got, perms)
	}
}

func TestIntegration_RequesterPays(t *testing.T) {
	// This test needs a second project and user (token source) to test
	// all possibilities. Since we need these things for Firestore already,
	// we use them here.
	//
	// There are up to three entities involved in a requester-pays call:
	//
	// 1. The user making the request. Here, we use
	//    a. The account used to create the token source used for all our
	//       integration tests (see testutil.TokenSource).
	//    b. The account used for the Firestore tests.
	// 2. The project that owns the requester-pays bucket. Here, that
	//    is the test project ID (see testutil.ProjID).
	// 3. The project provided as the userProject parameter of the request;
	//    the project to be billed. This test uses:
	//    a. The project that owns the requester-pays bucket (same as (2))
	//    b. Another project (the Firestore project).
	//
	// The following must hold for this test to work:
	// - (1a) must have resourcemanager.projects.createBillingAssignment permission
	//       (Owner role) on (2) (the project, not the bucket).
	// - (1b) must NOT have that permission on (2).
	// - (1b) must have serviceusage.services.use permission (Editor role) on (3b).
	// - (1b) must NOT have that permission on (3a).
	// - (1a) must NOT have that permission on (3b).
	const wantErrorCode = 400

	ctx := context.Background()
	client := testConfig(ctx, t)
	defer client.Close()
	bucketName2 := uidSpace.New()
	b := client.Bucket(bucketName2)
	projID := testutil.ProjID()
	// Use Firestore project as a project that does not contain the bucket.
	otherProjID := os.Getenv(envFirestoreProjID)
	if otherProjID == "" {
		t.Fatalf("need a second project (env var %s)", envFirestoreProjID)
	}
	ts := testutil.TokenSourceEnv(ctx, envFirestorePrivateKey, ScopeFullControl)
	if ts == nil {
		t.Fatalf("need a second account (env var %s)", envFirestorePrivateKey)
	}
	otherClient, err := NewClient(ctx, option.WithTokenSource(ts))
	if err != nil {
		t.Fatal(err)
	}
	defer otherClient.Close()
	ob := otherClient.Bucket(bucketName2)
	user, err := keyFileEmail(os.Getenv("GCLOUD_TESTS_GOLANG_KEY"))
	if err != nil {
		t.Fatal(err)
	}
	otherUser, err := keyFileEmail(os.Getenv(envFirestorePrivateKey))
	if err != nil {
		t.Fatal(err)
	}

	// Create a requester-pays bucket. The bucket is contained in the project projID.
	if err := b.Create(ctx, projID, &BucketAttrs{RequesterPays: true}); err != nil {
		t.Fatal(err)
	}
	if err := b.ACL().Set(ctx, ACLEntity("user-"+otherUser), RoleOwner); err != nil {
		t.Fatal(err)
	}

	// Extract the error code from err if it's a googleapi.Error.
	errCode := func(err error) int {
		if err == nil {
			return 0
		}
		if err, ok := err.(*googleapi.Error); ok {
			return err.Code
		}
		return -1
	}

	// Call f under various conditions.
	// Here b and ob refer to the same bucket, but b is bound to client,
	// while ob is bound to otherClient. The clients differ in their credentials,
	// i.e. the identity of the user making the RPC: b's user is an Owner on the
	// bucket's containing project, ob's is not.
	call := func(msg string, f func(*BucketHandle) error) {
		// user: an Owner on the containing project
		// userProject: absent
		// result: success, by the rule permitting access by owners of the containing bucket.
		if err := f(b); err != nil {
			t.Errorf("%s: %v, want nil\n"+
				"confirm that %s is an Owner on %s",
				msg, err, user, projID)
		}
		// user: an Owner on the containing project
		// userProject: containing project
		// result: success, by the same rule as above; userProject is unnecessary but allowed.
		if err := f(b.UserProject(projID)); err != nil {
			t.Errorf("%s: got %v, want nil", msg, err)
		}
		// user: not an Owner on the containing project
		// userProject: absent
		// result: failure, by the standard requester-pays rule
		err := f(ob)
		if got, want := errCode(err), wantErrorCode; got != want {
			t.Errorf("%s: got error %s, want code %d\n"+
				"confirm that %s is NOT an Owner on %s",
				msg, err, want, otherUser, projID)
		}
		// user: not an Owner on the containing project
		// userProject: not the containing one, but user has Editor role on it
		// result: success, by the standard requester-pays rule
		if err := f(ob.UserProject(otherProjID)); err != nil {
			t.Errorf("%s: got %v, want nil\n"+
				"confirm that %s is an Editor on %s and that that project has billing enabled",
				msg, err, otherUser, otherProjID)
		}
		// user: not an Owner on the containing project
		// userProject: the containing one, on which the user does NOT have Editor permission.
		// result: failure
		err = f(ob.UserProject("veener-jba"))
		if got, want := errCode(err), 403; got != want {
			t.Errorf("%s: got error %s, want code %d\n"+
				"confirm that %s is NOT an Editor on %s",
				msg, err, want, otherUser, "veener-jba")
		}
	}

	// Getting its attributes requires a user project.
	var attrs *BucketAttrs
	call("Bucket attrs", func(b *BucketHandle) error {
		a, err := b.Attrs(ctx)
		if a != nil {
			attrs = a
		}
		return err
	})
	if attrs != nil {
		if got, want := attrs.RequesterPays, true; got != want {
			t.Fatalf("attr.RequesterPays = %t, want %t", got, want)
		}
	}
	// Object operations.
	call("write object", func(b *BucketHandle) error {
		return writeObject(ctx, b.Object("foo"), "text/plain", []byte("hello"))
	})
	call("read object", func(b *BucketHandle) error {
		_, err := readObject(ctx, b.Object("foo"))
		return err
	})
	call("object attrs", func(b *BucketHandle) error {
		_, err := b.Object("foo").Attrs(ctx)
		return err
	})
	call("update object", func(b *BucketHandle) error {
		_, err := b.Object("foo").Update(ctx, ObjectAttrsToUpdate{ContentLanguage: "en"})
		return err
	})

	// ACL operations.
	entity := ACLEntity("domain-google.com")
	call("bucket acl set", func(b *BucketHandle) error {
		return b.ACL().Set(ctx, entity, RoleReader)
	})
	call("bucket acl list", func(b *BucketHandle) error {
		_, err := b.ACL().List(ctx)
		return err
	})
	call("bucket acl delete", func(b *BucketHandle) error {
		err := b.ACL().Delete(ctx, entity)
		if errCode(err) == 404 {
			// Since we call the function multiple times, it will
			// fail with NotFound for all but the first.
			return nil
		}
		return err
	})
	call("default object acl set", func(b *BucketHandle) error {
		return b.DefaultObjectACL().Set(ctx, entity, RoleReader)
	})
	call("default object acl list", func(b *BucketHandle) error {
		_, err := b.DefaultObjectACL().List(ctx)
		return err
	})
	call("default object acl delete", func(b *BucketHandle) error {
		err := b.DefaultObjectACL().Delete(ctx, entity)
		if errCode(err) == 404 {
			return nil
		}
		return err
	})
	call("object acl set", func(b *BucketHandle) error {
		return b.Object("foo").ACL().Set(ctx, entity, RoleReader)
	})
	call("object acl list", func(b *BucketHandle) error {
		_, err := b.Object("foo").ACL().List(ctx)
		return err
	})
	call("object acl delete", func(b *BucketHandle) error {
		err := b.Object("foo").ACL().Delete(ctx, entity)
		if errCode(err) == 404 {
			return nil
		}
		return err
	})

	// Copy and compose.
	call("copy", func(b *BucketHandle) error {
		_, err := b.Object("copy").CopierFrom(b.Object("foo")).Run(ctx)
		return err
	})
	call("compose", func(b *BucketHandle) error {
		_, err := b.Object("compose").ComposerFrom(b.Object("foo"), b.Object("copy")).Run(ctx)
		return err
	})

	// Deletion.
	call("delete object", func(b *BucketHandle) error {
		err := b.Object("foo").Delete(ctx)
		if err == ErrObjectNotExist {
			return nil
		}
		return err
	})
	for _, obj := range []string{"copy", "compose"} {
		if err := b.UserProject(projID).Object(obj).Delete(ctx); err != nil {
			t.Fatalf("could not delete %q: %v", obj, err)
		}
	}
	if err := b.Delete(ctx); err != nil {
		t.Fatalf("deleting bucket: %v", err)
	}
}

// TODO(jba): move to testutil, factor out from firestore/integration_test.go.
const (
	envFirestoreProjID     = "GCLOUD_TESTS_GOLANG_FIRESTORE_PROJECT_ID"
	envFirestorePrivateKey = "GCLOUD_TESTS_GOLANG_FIRESTORE_KEY"
)

func keyFileEmail(filename string) (string, error) {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}
	var v struct {
		ClientEmail string `json:"client_email"`
	}
	if err := json.Unmarshal(bytes, &v); err != nil {
		return "", err
	}
	return v.ClientEmail, nil
}

func TestNotifications(t *testing.T) {
	ctx := context.Background()
	client := testConfig(ctx, t)
	defer client.Close()
	bkt := client.Bucket(bucketName)

	checkNotifications := func(msg string, want map[string]*Notification) {
		got, err := bkt.Notifications(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if diff := testutil.Diff(got, want); diff != "" {
			t.Errorf("%s: got=-, want=+:\n%s", msg, diff)
		}
	}
	checkNotifications("initial", map[string]*Notification{})

	nArg := &Notification{
		TopicProjectID: testutil.ProjID(),
		TopicID:        "go-storage-notification-test",
		PayloadFormat:  NoPayload,
	}
	n, err := bkt.AddNotification(ctx, nArg)
	if err != nil {
		t.Fatal(err)
	}
	nArg.ID = n.ID
	if !testutil.Equal(n, nArg) {
		t.Errorf("got %+v, want %+v", n, nArg)
	}
	checkNotifications("after add", map[string]*Notification{n.ID: n})

	if err := bkt.DeleteNotification(ctx, n.ID); err != nil {
		t.Fatal(err)
	}
	checkNotifications("after delete", map[string]*Notification{})
}

func TestIntegration_Public(t *testing.T) {
	// Confirm that an unauthenticated client can access a public bucket.

	// See https://cloud.google.com/storage/docs/public-datasets/landsat
	const landsatBucket = "gcp-public-data-landsat"
	const landsatPrefix = "LC08/PRE/044/034/LC80440342016259LGN00/"
	const landsatObject = landsatPrefix + "LC80440342016259LGN00_MTL.txt"

	// Create an unauthenticated client.
	ctx := context.Background()
	client, err := NewClient(ctx, option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	bkt := client.Bucket(landsatBucket)
	obj := bkt.Object(landsatObject)

	// Read a public object.
	bytes, err := readObject(ctx, obj)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(bytes), 7903; got != want {
		t.Errorf("len(bytes) = %d, want %d", got, want)
	}

	// List objects in a public bucket.
	iter := bkt.Objects(ctx, &Query{Prefix: landsatPrefix})
	gotCount := 0
	for {
		_, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		gotCount++
	}
	if wantCount := 13; gotCount != wantCount {
		t.Errorf("object count: got %d, want %d", gotCount, wantCount)
	}

	errCode := func(err error) int {
		if err, ok := err.(*googleapi.Error); !ok {
			return -1
		} else {
			return err.Code
		}
	}

	// Reading from or writing to a non-public bucket fails.
	c := testConfig(ctx, t)
	defer c.Close()
	nonPublicObj := client.Bucket(bucketName).Object("noauth")
	// Oddly, reading returns 403 but writing returns 401.
	_, err = readObject(ctx, nonPublicObj)
	if got, want := errCode(err), 403; got != want {
		t.Errorf("got code %d; want %d\nerror: %v", got, want, err)
	}
	err = writeObject(ctx, nonPublicObj, "text/plain", []byte("b"))
	if got, want := errCode(err), 401; got != want {
		t.Errorf("got code %d; want %d\nerror: %v", got, want, err)
	}
}

func TestIntegration_ReadCRC(t *testing.T) {
	// Test that the checksum is handled correctly when reading files.
	// For gzipped files, see https://github.com/GoogleCloudPlatform/google-cloud-dotnet/issues/1641.
	if testing.Short() {
		t.Skip("Integration tests skipped in short mode")
	}
	const (
		// This is an uncompressed file.
		// See https://cloud.google.com/storage/docs/public-datasets/landsat
		uncompressedBucket = "gcp-public-data-landsat"
		uncompressedObject = "LC08/PRE/044/034/LC80440342016259LGN00/LC80440342016259LGN00_MTL.txt"

		gzippedBucket   = "storage-library-test-bucket"
		gzippedObject   = "gzipped-text.txt"
		gzippedContents = "hello world" // uncompressed contents of the file
	)
	ctx := context.Background()
	client, err := NewClient(ctx, option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	for _, test := range []struct {
		desc           string
		obj            *ObjectHandle
		offset, length int64
		readCompressed bool // don't decompress a gzipped file

		wantErr     bool
		wantCheck   bool // Should Reader try to check the CRC?
		wantChecked bool // Did Reader actually check the CRC?
	}{
		{
			desc:           "uncompressed, entire file",
			obj:            client.Bucket(uncompressedBucket).Object(uncompressedObject),
			offset:         0,
			length:         -1,
			readCompressed: false,
			wantCheck:      true,
			wantChecked:    true,
		},
		{
			desc:           "uncompressed, entire file, don't decompress",
			obj:            client.Bucket(uncompressedBucket).Object(uncompressedObject),
			offset:         0,
			length:         -1,
			readCompressed: true,
			wantCheck:      true,
			wantChecked:    true,
		},
		{
			desc:           "uncompressed, suffix",
			obj:            client.Bucket(uncompressedBucket).Object(uncompressedObject),
			offset:         1,
			length:         -1,
			readCompressed: false,
			wantCheck:      false,
			wantChecked:    false,
		},
		{
			desc:           "uncompressed, prefix",
			obj:            client.Bucket(uncompressedBucket).Object(uncompressedObject),
			offset:         0,
			length:         18,
			readCompressed: false,
			wantCheck:      false,
			wantChecked:    false,
		},
		{
			// When a gzipped file is unzipped by GCS, we can't verify the checksum
			// because it was computed against the zipped contents. There is no
			// header that indicates that a gzipped file is being served unzipped.
			// But our CRC check only happens if there is a Content-Length header,
			// and that header is absent for this read.
			desc:           "compressed, entire file, server unzips",
			obj:            client.Bucket(gzippedBucket).Object(gzippedObject),
			offset:         0,
			length:         -1,
			readCompressed: false,
			wantCheck:      true,
			wantChecked:    false,
		},
		{
			// When we read a gzipped file uncompressed, it's like reading a regular file:
			// the served content and the CRC match.
			desc:           "compressed, entire file, read compressed",
			obj:            client.Bucket(gzippedBucket).Object(gzippedObject),
			offset:         0,
			length:         -1,
			readCompressed: true,
			wantCheck:      true,
			wantChecked:    true,
		},
		{
			desc:           "compressed, partial, server unzips",
			obj:            client.Bucket(gzippedBucket).Object(gzippedObject),
			offset:         1,
			length:         8,
			readCompressed: false,
			wantErr:        true, // GCS can't serve part of a gzipped object
			wantCheck:      false,
			wantChecked:    false,
		},
		{
			desc:           "compressed, partial, read compressed",
			obj:            client.Bucket(gzippedBucket).Object(gzippedObject),
			offset:         1,
			length:         8,
			readCompressed: true,
			wantCheck:      false,
			wantChecked:    false,
		},
	} {
		obj := test.obj.ReadCompressed(test.readCompressed)
		r, err := obj.NewRangeReader(ctx, test.offset, test.length)
		if err != nil {
			if test.wantErr {
				continue
			}
			t.Fatalf("%s: %v", test.desc, err)
		}
		if got, want := r.checkCRC, test.wantCheck; got != want {
			t.Errorf("%s, checkCRC: got %t, want %t", test.desc, got, want)
		}
		_, err = ioutil.ReadAll(r)
		_ = r.Close()
		if err != nil {
			t.Fatalf("%s: %v", test.desc, err)
		}
		if got, want := r.checkedCRC, test.wantChecked; got != want {
			t.Errorf("%s, checkedCRC: got %t, want %t", test.desc, got, want)
		}
	}
}

func TestIntegration_CancelWrite(t *testing.T) {
	// Verify that canceling the writer's context immediately stops uploading an object.
	if testing.Short() {
		t.Skip("Integration tests skipped in short mode")
	}
	ctx := context.Background()
	client := testConfig(ctx, t)
	defer client.Close()
	bkt := client.Bucket(bucketName)

	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	obj := bkt.Object("cancel-write")
	w := obj.NewWriter(cctx)
	w.ChunkSize = googleapi.MinUploadChunkSize
	buf := make([]byte, w.ChunkSize)
	// Write the first chunk. This is read in its entirety before sending the request
	// (see google.golang.org/api/gensupport.PrepareUpload), so we expect it to return
	// without error.
	_, err := w.Write(buf)
	if err != nil {
		t.Fatal(err)
	}
	// Now cancel the context.
	cancel()
	// The next Write should return context.Canceled.
	_, err = w.Write(buf)
	if err != context.Canceled {
		t.Fatalf("got %v, wanted context.Canceled", err)
	}
	// The Close should too.
	err = w.Close()
	if err != context.Canceled {
		t.Fatalf("got %v, wanted context.Canceled", err)
	}
}

func TestIntegration_UpdateCORS(t *testing.T) {
	if testing.Short() {
		t.Skip("Integration tests skipped in short mode")
	}

	ctx := context.Background()
	client := testConfig(ctx, t)
	defer client.Close()

	initialSettings := []CORS{
		{
			MaxAge:          time.Hour,
			Methods:         []string{"POST"},
			Origins:         []string{"some-origin.com"},
			ResponseHeaders: []string{"foo-bar"},
		},
	}

	for _, test := range []struct {
		input []CORS
		want  []CORS
	}{
		{
			input: []CORS{
				{
					MaxAge:          time.Hour,
					Methods:         []string{"GET"},
					Origins:         []string{"*"},
					ResponseHeaders: []string{"some-header"},
				},
			},
			want: []CORS{
				{
					MaxAge:          time.Hour,
					Methods:         []string{"GET"},
					Origins:         []string{"*"},
					ResponseHeaders: []string{"some-header"},
				},
			},
		},
		{
			input: []CORS{},
			want:  nil,
		},
		{
			input: nil,
			want: []CORS{
				{
					MaxAge:          time.Hour,
					Methods:         []string{"POST"},
					Origins:         []string{"some-origin.com"},
					ResponseHeaders: []string{"foo-bar"},
				},
			},
		},
	} {
		bkt := client.Bucket(uidSpace.New())
		defer func(b *BucketHandle) {
			err := b.Delete(ctx)
			if err != nil {
				t.Fatal(err)
			}
		}(bkt)
		err := bkt.Create(ctx, testutil.ProjID(), &BucketAttrs{CORS: initialSettings})
		if err != nil {
			t.Fatal(err)
		}

		_, err = bkt.Update(ctx, BucketAttrsToUpdate{CORS: test.input})
		if err != nil {
			t.Fatal(err)
		}

		attrs, err := bkt.Attrs(ctx)
		if err != nil {
			t.Fatal(err)
		}

		if diff := testutil.Diff(attrs.CORS, test.want); diff != "" {
			t.Errorf("input: %v\ngot=-, want=+:\n%s", test.input, diff)
		}
	}
}

func TestIntegration_UpdateRetentionPolicy(t *testing.T) {
	if testing.Short() {
		t.Skip("Integration tests skipped in short mode")
	}

	ctx := context.Background()
	client := testConfig(ctx, t)
	defer client.Close()

	initial := &RetentionPolicy{RetentionPeriod: time.Minute}

	for _, test := range []struct {
		input *RetentionPolicy
		want  *RetentionPolicy
	}{
		{ // Update
			input: &RetentionPolicy{RetentionPeriod: time.Hour},
			want:  &RetentionPolicy{RetentionPeriod: time.Hour},
		},
		{ // Update even with timestamp (EffectiveTime should be ignored)
			input: &RetentionPolicy{RetentionPeriod: time.Hour, EffectiveTime: time.Now()},
			want:  &RetentionPolicy{RetentionPeriod: time.Hour},
		},
		{ // Remove
			input: &RetentionPolicy{},
			want:  nil,
		},
		{ // Remove even with timestamp (EffectiveTime should be ignored)
			input: &RetentionPolicy{EffectiveTime: time.Now()},
			want:  nil,
		},
		{ // Ignore
			input: nil,
			want:  initial,
		},
	} {
		bkt := client.Bucket(uidSpace.New())
		err := bkt.Create(ctx, testutil.ProjID(), &BucketAttrs{RetentionPolicy: initial})
		if err != nil {
			t.Fatal(err)
		}

		defer func() {
			if err := bkt.Delete(ctx); err != nil {
				t.Fatal(err)
			}
		}()

		_, err = bkt.Update(ctx, BucketAttrsToUpdate{RetentionPolicy: test.input})
		if err != nil {
			t.Fatal(err)
		}

		attrs, err := bkt.Attrs(ctx)
		if err != nil {
			t.Fatal(err)
		}

		if attrs.RetentionPolicy != nil && attrs.RetentionPolicy.EffectiveTime.Unix() == 0 {
			// Should be set by the server and parsed by the client
			t.Fatal("EffectiveTime should be set, but it was not")
		}
		if diff := testutil.Diff(attrs.RetentionPolicy, test.want, cmpopts.IgnoreTypes(time.Time{})); diff != "" {
			t.Errorf("input: %v\ngot=-, want=+:\n%s", test.input, diff)
		}
	}
}

func TestIntegration_DeleteObjectInBucketWithRetentionPolicy(t *testing.T) {
	if testing.Short() {
		t.Skip("Integration tests skipped in short mode")
	}

	ctx := context.Background()
	client := testConfig(ctx, t)
	defer client.Close()

	bkt := client.Bucket(uidSpace.New())
	err := bkt.Create(ctx, testutil.ProjID(), &BucketAttrs{RetentionPolicy: &RetentionPolicy{RetentionPeriod: 25 * time.Hour}})
	if err != nil {
		t.Fatal(err)
	}

	oh := bkt.Object("some-object")
	if err = writeObject(ctx, oh, "text/plain", []byte("hello world")); err != nil {
		t.Fatal(err)
	}

	err = oh.Delete(ctx)
	if err == nil {
		t.Fatal("expected to err deleting an object in a bucket with retention period, but got nil")
	}

	// Remove the retention period
	_, err = bkt.Update(ctx, BucketAttrsToUpdate{RetentionPolicy: &RetentionPolicy{RetentionPeriod: 0}})
	if err != nil {
		t.Fatal(err)
	}

	err = oh.Delete(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := bkt.Delete(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestIntegration_LockBucket(t *testing.T) {
	if testing.Short() {
		t.Skip("Integration tests skipped in short mode")
	}

	ctx := context.Background()
	client := testConfig(ctx, t)
	defer client.Close()

	bkt := client.Bucket(uidSpace.New())
	err := bkt.Create(ctx, testutil.ProjID(), &BucketAttrs{RetentionPolicy: &RetentionPolicy{RetentionPeriod: time.Hour * 25}})
	if err != nil {
		t.Fatal(err)
	}

	attrs, err := bkt.Attrs(ctx)
	if err != nil {
		t.Fatal(err)
	}

	err = bkt.If(BucketConditions{MetagenerationMatch: attrs.MetaGeneration}).LockRetentionPolicy(ctx)
	if err != nil {
		t.Fatal("could not lock", err)
	}

	_, err = bkt.Update(ctx, BucketAttrsToUpdate{RetentionPolicy: &RetentionPolicy{RetentionPeriod: time.Hour}})
	if err == nil {
		t.Fatal("Expected error updating locked bucket, got nil")
	}
}

func TestIntegration_LockBucket_MetagenerationRequired(t *testing.T) {
	if testing.Short() {
		t.Skip("Integration tests skipped in short mode")
	}

	ctx := context.Background()
	client := testConfig(ctx, t)
	defer client.Close()

	bkt := client.Bucket(uidSpace.New())
	err := bkt.Create(ctx, testutil.ProjID(), &BucketAttrs{RetentionPolicy: &RetentionPolicy{RetentionPeriod: time.Hour * 25}})
	if err != nil {
		t.Fatal(err)
	}

	err = bkt.LockRetentionPolicy(ctx)
	if err == nil {
		t.Fatal("expected error locking bucket without metageneration condition, got nil")
	}
}

func writeObject(ctx context.Context, obj *ObjectHandle, contentType string, contents []byte) error {
	w := obj.NewWriter(ctx)
	w.ContentType = contentType
	w.CacheControl = "public, max-age=60"
	if contents != nil {
		if _, err := w.Write(contents); err != nil {
			_ = w.Close()
			return err
		}
	}
	return w.Close()
}

func readObject(ctx context.Context, obj *ObjectHandle) ([]byte, error) {
	r, err := obj.NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return ioutil.ReadAll(r)
}

// cleanup deletes the bucket used for testing, as well as old
// testing buckets that weren't cleaned previously.
func cleanup() error {
	if testing.Short() {
		return nil // Don't clean up in short mode.
	}
	ctx := context.Background()
	client := config(ctx)
	if client == nil {
		return nil // Don't cleanup if we're not configured correctly.
	}
	defer client.Close()
	if err := killBucket(ctx, client, bucketName); err != nil {
		return err
	}

	// Delete buckets whose name begins with our test prefix, and which were
	// created a while ago. (Unfortunately GCS doesn't provide last-modified
	// time, which would be a better way to check for staleness.)
	const expireAge = 24 * time.Hour
	projectID := testutil.ProjID()
	it := client.Buckets(ctx, projectID)
	it.Prefix = testPrefix
	for {
		bktAttrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		if time.Since(bktAttrs.Created) > expireAge {
			log.Printf("deleting bucket %q, which is more than %s old", bktAttrs.Name, expireAge)
			if err := killBucket(ctx, client, bktAttrs.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

// killBucket deletes a bucket and all its objects.
func killBucket(ctx context.Context, client *Client, bucketName string) error {
	bkt := client.Bucket(bucketName)
	// Bucket must be empty to delete.
	it := bkt.Objects(ctx, nil)
	for {
		objAttrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		if err := bkt.Object(objAttrs.Name).Delete(ctx); err != nil {
			return fmt.Errorf("deleting %q: %v", bucketName+"/"+objAttrs.Name, err)
		}
	}
	// GCS is eventually consistent, so this delete may fail because the
	// replica still sees an object in the bucket. We log the error and expect
	// a later test run to delete the bucket.
	if err := bkt.Delete(ctx); err != nil {
		log.Printf("deleting %q: %v", bucketName, err)
	}
	return nil
}

func randomContents() []byte {
	h := md5.New()
	io.WriteString(h, fmt.Sprintf("hello world%d", rand.Intn(100000)))
	return h.Sum(nil)
}

type zeros struct{}

func (zeros) Read(p []byte) (int, error) { return len(p), nil }
