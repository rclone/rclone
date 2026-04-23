package s3

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/bucket"
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/lib/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gz(t *testing.T, s string) string {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write([]byte(s))
	require.NoError(t, err)
	err = zw.Close()
	require.NoError(t, err)
	return buf.String()
}

func md5sum(t *testing.T, s string) string {
	hash := md5.Sum([]byte(s))
	return fmt.Sprintf("%x", hash)
}

func (f *Fs) InternalTestMetadata(t *testing.T) {
	ctx := context.Background()
	original := random.String(1000)
	contents := gz(t, original)

	item := fstest.NewItem("test-metadata", contents, fstest.Time("2001-05-06T04:05:06.499999999Z"))
	btime := time.Now()
	metadata := fs.Metadata{
		"cache-control":       "no-cache",
		"content-disposition": "inline",
		"content-encoding":    "gzip",
		"content-language":    "en-US",
		"content-type":        "text/plain",
		"mtime":               "2009-05-06T04:05:06.499999999Z",
		// "tier" - read only
		// "btime" - read only
	}
	// Cloudflare insists on decompressing `Content-Encoding: gzip` unless
	// `Cache-Control: no-transform` is supplied. This is a deviation from
	// AWS but we fudge the tests here rather than breaking peoples
	// expectations of what Cloudflare does.
	//
	// This can always be overridden by using
	// `--header-upload "Cache-Control: no-transform"`
	if f.opt.Provider == "Cloudflare" {
		metadata["cache-control"] = "no-transform"
	}
	obj := fstests.PutTestContentsMetadata(ctx, t, f, &item, true, contents, true, "text/html", metadata)
	defer func() {
		assert.NoError(t, obj.Remove(ctx))
	}()
	o := obj.(*Object)
	gotMetadata, err := o.Metadata(ctx)
	require.NoError(t, err)
	for k, v := range metadata {
		got := gotMetadata[k]
		switch k {
		case "mtime":
			assert.True(t, fstest.Time(v).Equal(fstest.Time(got)))
		case "btime":
			gotBtime := fstest.Time(got)
			dt := gotBtime.Sub(btime)
			assert.True(t, dt < time.Minute && dt > -time.Minute, fmt.Sprintf("btime more than 1 minute out want %v got %v delta %v", btime, gotBtime, dt))
			assert.True(t, fstest.Time(v).Equal(fstest.Time(got)))
		case "tier":
			assert.NotEqual(t, "", got)
		default:
			assert.Equal(t, v, got, k)
		}
	}

	t.Run("GzipEncoding", func(t *testing.T) {
		// Test that the gzipped file we uploaded can be
		// downloaded with and without decompression
		checkDownload := func(wantContents string, wantSize int64, wantHash string) {
			gotContents := fstests.ReadObject(ctx, t, o, -1)
			assert.Equal(t, wantContents, gotContents)
			assert.Equal(t, wantSize, o.Size())
			gotHash, err := o.Hash(ctx, hash.MD5)
			require.NoError(t, err)
			assert.Equal(t, wantHash, gotHash)
		}

		t.Run("NoDecompress", func(t *testing.T) {
			checkDownload(contents, int64(len(contents)), md5sum(t, contents))
		})
		t.Run("Decompress", func(t *testing.T) {
			f.opt.Decompress = true
			defer func() {
				f.opt.Decompress = false
			}()
			checkDownload(original, -1, "")
		})

	})
}

func (f *Fs) InternalTestNoHead(t *testing.T) {
	ctx := context.Background()
	// Set NoHead for this test
	f.opt.NoHead = true
	defer func() {
		f.opt.NoHead = false
	}()
	contents := random.String(1000)
	item := fstest.NewItem("test-no-head", contents, fstest.Time("2001-05-06T04:05:06.499999999Z"))
	obj := fstests.PutTestContents(ctx, t, f, &item, contents, true)
	defer func() {
		assert.NoError(t, obj.Remove(ctx))
	}()
	// PutTestcontents checks the received object

}

func TestVersionLess(t *testing.T) {
	key1 := "key1"
	key2 := "key2"
	t1 := fstest.Time("2022-01-21T12:00:00+01:00")
	t2 := fstest.Time("2022-01-21T12:00:01+01:00")
	for n, test := range []struct {
		a, b *types.ObjectVersion
		want bool
	}{
		{a: nil, b: nil, want: true},
		{a: &types.ObjectVersion{Key: &key1, LastModified: &t1}, b: nil, want: false},
		{a: nil, b: &types.ObjectVersion{Key: &key1, LastModified: &t1}, want: true},
		{a: &types.ObjectVersion{Key: &key1, LastModified: &t1}, b: &types.ObjectVersion{Key: &key1, LastModified: &t1}, want: false},
		{a: &types.ObjectVersion{Key: &key1, LastModified: &t1}, b: &types.ObjectVersion{Key: &key1, LastModified: &t2}, want: false},
		{a: &types.ObjectVersion{Key: &key1, LastModified: &t2}, b: &types.ObjectVersion{Key: &key1, LastModified: &t1}, want: true},
		{a: &types.ObjectVersion{Key: &key1, LastModified: &t1}, b: &types.ObjectVersion{Key: &key2, LastModified: &t1}, want: true},
		{a: &types.ObjectVersion{Key: &key2, LastModified: &t1}, b: &types.ObjectVersion{Key: &key1, LastModified: &t1}, want: false},
		{a: &types.ObjectVersion{Key: &key1, LastModified: &t1, IsLatest: aws.Bool(false)}, b: &types.ObjectVersion{Key: &key1, LastModified: &t1}, want: false},
		{a: &types.ObjectVersion{Key: &key1, LastModified: &t1, IsLatest: aws.Bool(true)}, b: &types.ObjectVersion{Key: &key1, LastModified: &t1}, want: true},
		{a: &types.ObjectVersion{Key: &key1, LastModified: &t1, IsLatest: aws.Bool(false)}, b: &types.ObjectVersion{Key: &key1, LastModified: &t1, IsLatest: aws.Bool(true)}, want: false},
	} {
		got := versionLess(test.a, test.b)
		assert.Equal(t, test.want, got, fmt.Sprintf("%d: %+v", n, test))
	}
}

func TestMergeDeleteMarkers(t *testing.T) {
	key1 := "key1"
	key2 := "key2"
	t1 := fstest.Time("2022-01-21T12:00:00+01:00")
	t2 := fstest.Time("2022-01-21T12:00:01+01:00")
	for n, test := range []struct {
		versions []types.ObjectVersion
		markers  []types.DeleteMarkerEntry
		want     []types.ObjectVersion
	}{
		{
			versions: []types.ObjectVersion{},
			markers:  []types.DeleteMarkerEntry{},
			want:     []types.ObjectVersion{},
		},
		{
			versions: []types.ObjectVersion{
				{
					Key:          &key1,
					LastModified: &t1,
				},
			},
			markers: []types.DeleteMarkerEntry{},
			want: []types.ObjectVersion{
				{
					Key:          &key1,
					LastModified: &t1,
				},
			},
		},
		{
			versions: []types.ObjectVersion{},
			markers: []types.DeleteMarkerEntry{
				{
					Key:          &key1,
					LastModified: &t1,
				},
			},
			want: []types.ObjectVersion{
				{
					Key:          &key1,
					LastModified: &t1,
					Size:         isDeleteMarker,
				},
			},
		},
		{
			versions: []types.ObjectVersion{
				{
					Key:          &key1,
					LastModified: &t2,
				},
				{
					Key:          &key2,
					LastModified: &t2,
				},
			},
			markers: []types.DeleteMarkerEntry{
				{
					Key:          &key1,
					LastModified: &t1,
				},
			},
			want: []types.ObjectVersion{
				{
					Key:          &key1,
					LastModified: &t2,
				},
				{
					Key:          &key1,
					LastModified: &t1,
					Size:         isDeleteMarker,
				},
				{
					Key:          &key2,
					LastModified: &t2,
				},
			},
		},
	} {
		got := mergeDeleteMarkers(test.versions, test.markers)
		assert.Equal(t, test.want, got, fmt.Sprintf("%d: %+v", n, test))
	}
}

func TestRemoveAWSChunked(t *testing.T) {
	ps := func(s string) *string {
		return &s
	}
	tests := []struct {
		name string
		in   *string
		want *string
	}{
		{"nil", nil, nil},
		{"empty", ps(""), nil},
		{"only aws", ps("aws-chunked"), nil},
		{"leading aws", ps("aws-chunked, gzip"), ps("gzip")},
		{"trailing aws", ps("gzip, aws-chunked"), ps("gzip")},
		{"middle aws", ps("gzip, aws-chunked, br"), ps("gzip,br")},
		{"case insensitive", ps("GZip, AwS-ChUnKeD, Br"), ps("GZip,Br")},
		{"duplicates", ps("aws-chunked , aws-chunked"), nil},
		{"no aws normalize spaces", ps(" gzip ,  br "), ps(" gzip ,  br ")},
		{"surrounding spaces", ps("  aws-chunked  "), nil},
		{"no change", ps("gzip, br"), ps("gzip, br")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := removeAWSChunked(tc.in)
			check := func(want, got *string) {
				t.Helper()
				if tc.want == nil {
					assert.Nil(t, got)
				} else {
					require.NotNil(t, got)
					assert.Equal(t, *tc.want, *got)
				}
			}
			check(tc.want, got)
			// Idempotent
			got2 := removeAWSChunked(got)
			check(got, got2)
		})
	}
}

func (f *Fs) InternalTestVersions(t *testing.T) {
	ctx := context.Background()

	// Enable versioning for this bucket during this test
	_, err := f.setGetVersioning(ctx, "Enabled")
	if err != nil {
		t.Skipf("Couldn't enable versioning: %v", err)
	}
	defer func() {
		// Disable versioning for this bucket
		_, err := f.setGetVersioning(ctx, "Suspended")
		assert.NoError(t, err)
	}()

	// Small pause to make the LastModified different since AWS
	// only seems to track them to 1 second granularity
	time.Sleep(2 * time.Second)

	// Create an object
	const dirName = "versions"
	const fileName = dirName + "/" + "test-versions.txt"
	contents := random.String(100)
	item := fstest.NewItem(fileName, contents, fstest.Time("2001-05-06T04:05:06.499999999Z"))
	obj := fstests.PutTestContents(ctx, t, f, &item, contents, true)
	defer func() {
		assert.NoError(t, obj.Remove(ctx))
	}()

	// Small pause
	time.Sleep(2 * time.Second)

	// Remove it
	assert.NoError(t, obj.Remove(ctx))

	// Small pause to make the LastModified different since AWS only seems to track them to 1 second granularity
	time.Sleep(2 * time.Second)

	// And create it with different size and contents
	newContents := random.String(101)
	newItem := fstest.NewItem(fileName, newContents, fstest.Time("2002-05-06T04:05:06.499999999Z"))
	newObj := fstests.PutTestContents(ctx, t, f, &newItem, newContents, true)

	t.Run("Versions", func(t *testing.T) {
		// Set --s3-versions for this test
		f.opt.Versions = true
		defer func() {
			f.opt.Versions = false
		}()

		// Read the contents
		entries, err := f.List(ctx, dirName)
		require.NoError(t, err)
		tests := 0
		var fileNameVersion string
		for _, entry := range entries {
			t.Log(entry)
			remote := entry.Remote()
			if remote == fileName {
				t.Run("ReadCurrent", func(t *testing.T) {
					assert.Equal(t, newContents, fstests.ReadObject(ctx, t, entry.(fs.Object), -1))
				})
				tests++
			} else if versionTime, p := version.Remove(remote); !versionTime.IsZero() && p == fileName {
				t.Run("ReadVersion", func(t *testing.T) {
					assert.Equal(t, contents, fstests.ReadObject(ctx, t, entry.(fs.Object), -1))
				})
				assert.WithinDuration(t, obj.(*Object).lastModified, versionTime, time.Second, "object time must be with 1 second of version time")
				fileNameVersion = remote
				tests++
			}
		}
		assert.Equal(t, 2, tests, "object missing from listing")

		// Check we can read the object with a version suffix
		t.Run("NewObject", func(t *testing.T) {
			o, err := f.NewObject(ctx, fileNameVersion)
			require.NoError(t, err)
			require.NotNil(t, o)
			assert.Equal(t, int64(100), o.Size(), o.Remote())
		})

		// Check we can make a NewFs from that object with a version suffix
		t.Run("NewFs", func(t *testing.T) {
			newPath := bucket.Join(fs.ConfigStringFull(f), fileNameVersion)
			// Make sure --s3-versions is set in the config of the new remote
			fs.Debugf(nil, "oldPath = %q", newPath)
			lastColon := strings.LastIndex(newPath, ":")
			require.True(t, lastColon >= 0)
			newPath = newPath[:lastColon] + ",versions" + newPath[lastColon:]
			fs.Debugf(nil, "newPath = %q", newPath)
			fNew, err := cache.Get(ctx, newPath)
			// This should return pointing to a file
			require.Equal(t, fs.ErrorIsFile, err)
			require.NotNil(t, fNew)
			// With the directory the directory above
			assert.Equal(t, dirName, path.Base(fs.ConfigStringFull(fNew)))
		})
	})

	t.Run("VersionAt", func(t *testing.T) {
		// We set --s3-version-at for this test so make sure we reset it at the end
		defer func() {
			f.opt.VersionAt = fs.Time{}
		}()

		var (
			firstObjectTime  = obj.(*Object).lastModified
			secondObjectTime = newObj.(*Object).lastModified
		)

		for _, test := range []struct {
			what     string
			at       time.Time
			want     []fstest.Item
			wantErr  error
			wantSize int64
		}{
			{
				what:    "Before",
				at:      firstObjectTime.Add(-time.Second),
				want:    fstests.InternalTestFiles,
				wantErr: fs.ErrorObjectNotFound,
			},
			{
				what:     "AfterOne",
				at:       firstObjectTime.Add(time.Second),
				want:     append([]fstest.Item{item}, fstests.InternalTestFiles...),
				wantSize: 100,
			},
			{
				what:    "AfterDelete",
				at:      secondObjectTime.Add(-time.Second),
				want:    fstests.InternalTestFiles,
				wantErr: fs.ErrorObjectNotFound,
			},
			{
				what:     "AfterTwo",
				at:       secondObjectTime.Add(time.Second),
				want:     append([]fstest.Item{newItem}, fstests.InternalTestFiles...),
				wantSize: 101,
			},
		} {
			t.Run(test.what, func(t *testing.T) {
				f.opt.VersionAt = fs.Time(test.at)
				t.Run("List", func(t *testing.T) {
					fstest.CheckListing(t, f, test.want)
				})
				t.Run("NewObject", func(t *testing.T) {
					gotObj, gotErr := f.NewObject(ctx, fileName)
					assert.Equal(t, test.wantErr, gotErr)
					if gotErr == nil {
						assert.Equal(t, test.wantSize, gotObj.Size())
					}
				})
			})
		}
	})

	t.Run("Mkdir", func(t *testing.T) {
		// Test what happens when we create a bucket we already own and see whether the
		// quirk is set correctly
		req := s3.CreateBucketInput{
			Bucket: &f.rootBucket,
			ACL:    types.BucketCannedACL(f.opt.BucketACL),
		}
		if f.opt.LocationConstraint != "" {
			req.CreateBucketConfiguration = &types.CreateBucketConfiguration{
				LocationConstraint: types.BucketLocationConstraint(f.opt.LocationConstraint),
			}
		}
		err := f.pacer.Call(func() (bool, error) {
			_, err := f.c.CreateBucket(ctx, &req)
			return f.shouldRetry(ctx, err)
		})
		var errString string
		var awsError smithy.APIError
		if err == nil {
			errString = "No Error"
		} else if errors.As(err, &awsError) {
			errString = awsError.ErrorCode()
		} else {
			assert.Fail(t, "Unknown error %T %v", err, err)
		}
		t.Logf("Creating a bucket we already have created returned code: %s", errString)
		switch errString {
		case "BucketAlreadyExists":
			assert.False(t, f.opt.UseAlreadyExists.Value, "Need to clear UseAlreadyExists quirk")
		case "No Error", "BucketAlreadyOwnedByYou":
			assert.True(t, f.opt.UseAlreadyExists.Value, "Need to set UseAlreadyExists quirk")
		default:
			assert.Fail(t, "Unknown error string %q", errString)
		}
	})

	t.Run("Cleanup", func(t *testing.T) {
		require.NoError(t, f.CleanUpHidden(ctx))
		items := append([]fstest.Item{newItem}, fstests.InternalTestFiles...)
		fstest.CheckListing(t, f, items)
		// Set --s3-versions for this test
		f.opt.Versions = true
		defer func() {
			f.opt.Versions = false
		}()
		fstest.CheckListing(t, f, items)
	})

	// Purge gets tested later
}

func (f *Fs) InternalTestObjectLock(t *testing.T) {
	if !f.opt.ObjectLockSupported.Value {
		t.Skip("Object Lock not supported by this provider (quirk object_lock_supported = false)")
	}
	ctx := context.Background()

	// Create a temporary bucket with Object Lock enabled to test on.
	// This exercises our BucketObjectLockEnabled option and isolates
	// the test from the main test bucket.
	lockBucket := f.rootBucket + "-object-lock-" + random.String(8)
	lockBucket = strings.ToLower(lockBucket)

	// Try to create bucket with Object Lock enabled
	objectLockEnabled := true
	req := s3.CreateBucketInput{
		Bucket:                     &lockBucket,
		ACL:                        types.BucketCannedACL(f.opt.BucketACL),
		ObjectLockEnabledForBucket: &objectLockEnabled,
	}
	if f.opt.LocationConstraint != "" {
		req.CreateBucketConfiguration = &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(f.opt.LocationConstraint),
		}
	}
	err := f.pacer.Call(func() (bool, error) {
		_, err := f.c.CreateBucket(ctx, &req)
		return f.shouldRetry(ctx, err)
	})
	if err != nil {
		t.Skipf("Object Lock not supported by this provider: CreateBucket with Object Lock failed: %v", err)
	}

	// Verify Object Lock is actually enabled on the new bucket.
	// Some S3-compatible servers (e.g. rclone serve s3) accept the
	// ObjectLockEnabledForBucket flag but don't actually implement Object Lock.
	var lockCfg *s3.GetObjectLockConfigurationOutput
	err = f.pacer.Call(func() (bool, error) {
		var err error
		lockCfg, err = f.c.GetObjectLockConfiguration(ctx, &s3.GetObjectLockConfigurationInput{
			Bucket: &lockBucket,
		})
		return f.shouldRetry(ctx, err)
	})
	if err != nil || lockCfg.ObjectLockConfiguration == nil ||
		lockCfg.ObjectLockConfiguration.ObjectLockEnabled != types.ObjectLockEnabledEnabled {
		_ = f.pacer.Call(func() (bool, error) {
			_, err := f.c.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: &lockBucket})
			return f.shouldRetry(ctx, err)
		})
		t.Skipf("Object Lock not functional on this provider (GetObjectLockConfiguration: %v)", err)
	}

	// Switch f to use the Object Lock bucket for this test
	oldBucket := f.rootBucket
	oldRoot := f.root
	oldRootDir := f.rootDirectory
	f.rootBucket = lockBucket
	f.root = lockBucket
	f.rootDirectory = ""
	defer func() {
		f.rootBucket = oldBucket
		f.root = oldRoot
		f.rootDirectory = oldRootDir
	}()

	// Helper to remove an object with Object Lock protection
	removeLocked := func(t *testing.T, obj fs.Object) {
		t.Helper()
		o := obj.(*Object)
		// Remove legal hold if present
		_ = o.setObjectLegalHold(ctx, types.ObjectLockLegalHoldStatusOff)
		// Enable bypass governance retention for deletion
		o.fs.opt.BypassGovernanceRetention = true
		err := obj.Remove(ctx)
		o.fs.opt.BypassGovernanceRetention = false
		assert.NoError(t, err)
	}

	// Clean up the temporary bucket after all sub-tests
	defer func() {
		// List and remove all object versions
		var objectVersions []types.ObjectIdentifier
		listReq := &s3.ListObjectVersionsInput{Bucket: &lockBucket}
		for {
			var resp *s3.ListObjectVersionsOutput
			err := f.pacer.Call(func() (bool, error) {
				var err error
				resp, err = f.c.ListObjectVersions(ctx, listReq)
				return f.shouldRetry(ctx, err)
			})
			if err != nil {
				t.Logf("Failed to list object versions for cleanup: %v", err)
				break
			}
			for _, v := range resp.Versions {
				objectVersions = append(objectVersions, types.ObjectIdentifier{
					Key:       v.Key,
					VersionId: v.VersionId,
				})
			}
			for _, m := range resp.DeleteMarkers {
				objectVersions = append(objectVersions, types.ObjectIdentifier{
					Key:       m.Key,
					VersionId: m.VersionId,
				})
			}
			if !aws.ToBool(resp.IsTruncated) {
				break
			}
			listReq.KeyMarker = resp.NextKeyMarker
			listReq.VersionIdMarker = resp.NextVersionIdMarker
		}
		if len(objectVersions) > 0 {
			bypass := true
			_ = f.pacer.Call(func() (bool, error) {
				_, err := f.c.DeleteObjects(ctx, &s3.DeleteObjectsInput{
					Bucket:                    &lockBucket,
					BypassGovernanceRetention: &bypass,
					Delete: &types.Delete{
						Objects: objectVersions,
						Quiet:   aws.Bool(true),
					},
				})
				return f.shouldRetry(ctx, err)
			})
		}
		_ = f.pacer.Call(func() (bool, error) {
			_, err := f.c.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: &lockBucket})
			return f.shouldRetry(ctx, err)
		})
	}()

	retainUntilDate := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)

	t.Run("Retention", func(t *testing.T) {
		// Set Object Lock options for this test
		f.opt.ObjectLockMode = "GOVERNANCE"
		f.opt.ObjectLockRetainUntilDate = retainUntilDate.Format(time.RFC3339)
		defer func() {
			f.opt.ObjectLockMode = ""
			f.opt.ObjectLockRetainUntilDate = ""
		}()

		// Upload an object with Object Lock retention
		contents := random.String(100)
		item := fstest.NewItem("test-object-lock-retention", contents, fstest.Time("2001-05-06T04:05:06.499999999Z"))
		obj := fstests.PutTestContents(ctx, t, f, &item, contents, true)
		defer func() {
			removeLocked(t, obj)
		}()

		// Read back metadata and verify Object Lock settings
		o := obj.(*Object)
		gotMetadata, err := o.Metadata(ctx)
		require.NoError(t, err)

		assert.Equal(t, "GOVERNANCE", gotMetadata["object-lock-mode"])
		gotRetainDate, err := time.Parse(time.RFC3339, gotMetadata["object-lock-retain-until-date"])
		require.NoError(t, err)
		assert.WithinDuration(t, retainUntilDate, gotRetainDate, time.Second)
	})

	t.Run("LegalHold", func(t *testing.T) {
		// Set Object Lock legal hold option
		f.opt.ObjectLockLegalHoldStatus = "ON"
		defer func() {
			f.opt.ObjectLockLegalHoldStatus = ""
		}()

		// Upload an object with legal hold
		contents := random.String(100)
		item := fstest.NewItem("test-object-lock-legal-hold", contents, fstest.Time("2001-05-06T04:05:06.499999999Z"))
		obj := fstests.PutTestContents(ctx, t, f, &item, contents, true)
		defer func() {
			removeLocked(t, obj)
		}()

		// Verify legal hold is ON
		o := obj.(*Object)
		gotMetadata, err := o.Metadata(ctx)
		require.NoError(t, err)
		assert.Equal(t, "ON", gotMetadata["object-lock-legal-hold-status"])

		// Set legal hold to OFF
		err = o.setObjectLegalHold(ctx, types.ObjectLockLegalHoldStatusOff)
		require.NoError(t, err)

		// Clear cached metadata and re-read
		o.meta = nil
		gotMetadata, err = o.Metadata(ctx)
		require.NoError(t, err)
		assert.Equal(t, "OFF", gotMetadata["object-lock-legal-hold-status"])
	})

	t.Run("SetAfterUpload", func(t *testing.T) {
		// Test the post-upload API path (PutObjectRetention + PutObjectLegalHold)
		f.opt.ObjectLockSetAfterUpload = true
		f.opt.ObjectLockMode = "GOVERNANCE"
		f.opt.ObjectLockRetainUntilDate = retainUntilDate.Format(time.RFC3339)
		f.opt.ObjectLockLegalHoldStatus = "ON"
		defer func() {
			f.opt.ObjectLockSetAfterUpload = false
			f.opt.ObjectLockMode = ""
			f.opt.ObjectLockRetainUntilDate = ""
			f.opt.ObjectLockLegalHoldStatus = ""
		}()

		// Upload an object - lock applied AFTER upload via separate API calls
		contents := random.String(100)
		item := fstest.NewItem("test-object-lock-after-upload", contents, fstest.Time("2001-05-06T04:05:06.499999999Z"))
		obj := fstests.PutTestContents(ctx, t, f, &item, contents, true)
		defer func() {
			removeLocked(t, obj)
		}()

		// Verify all Object Lock settings were applied
		o := obj.(*Object)
		gotMetadata, err := o.Metadata(ctx)
		require.NoError(t, err)

		assert.Equal(t, "GOVERNANCE", gotMetadata["object-lock-mode"])
		gotRetainDate, err := time.Parse(time.RFC3339, gotMetadata["object-lock-retain-until-date"])
		require.NoError(t, err)
		assert.WithinDuration(t, retainUntilDate, gotRetainDate, time.Second)
		assert.Equal(t, "ON", gotMetadata["object-lock-legal-hold-status"])
	})

	t.Run("Multipart", func(t *testing.T) {
		// Force multipart upload by setting a very low cutoff
		oldCutoff := f.opt.UploadCutoff
		f.opt.UploadCutoff = fs.SizeSuffix(1)
		f.opt.ObjectLockMode = "GOVERNANCE"
		f.opt.ObjectLockRetainUntilDate = retainUntilDate.Format(time.RFC3339)
		defer func() {
			f.opt.UploadCutoff = oldCutoff
			f.opt.ObjectLockMode = ""
			f.opt.ObjectLockRetainUntilDate = ""
		}()

		contents := random.String(100)
		item := fstest.NewItem("test-object-lock-multipart", contents, fstest.Time("2001-05-06T04:05:06.499999999Z"))
		obj := fstests.PutTestContents(ctx, t, f, &item, contents, true)
		defer func() {
			removeLocked(t, obj)
		}()

		o := obj.(*Object)
		gotMetadata, err := o.Metadata(ctx)
		require.NoError(t, err)
		assert.Equal(t, "GOVERNANCE", gotMetadata["object-lock-mode"])
	})

	t.Run("Presigned", func(t *testing.T) {
		// Use presigned request upload path
		f.opt.UsePresignedRequest = true
		f.opt.ObjectLockMode = "GOVERNANCE"
		f.opt.ObjectLockRetainUntilDate = retainUntilDate.Format(time.RFC3339)
		defer func() {
			f.opt.UsePresignedRequest = false
			f.opt.ObjectLockMode = ""
			f.opt.ObjectLockRetainUntilDate = ""
		}()

		contents := random.String(100)
		item := fstest.NewItem("test-object-lock-presigned", contents, fstest.Time("2001-05-06T04:05:06.499999999Z"))
		obj := fstests.PutTestContents(ctx, t, f, &item, contents, true)
		defer func() {
			removeLocked(t, obj)
		}()

		o := obj.(*Object)
		gotMetadata, err := o.Metadata(ctx)
		require.NoError(t, err)
		assert.Equal(t, "GOVERNANCE", gotMetadata["object-lock-mode"])
	})
}

func (f *Fs) InternalTest(t *testing.T) {
	t.Run("Metadata", f.InternalTestMetadata)
	t.Run("NoHead", f.InternalTestNoHead)
	t.Run("Versions", f.InternalTestVersions)
	t.Run("ObjectLock", f.InternalTestObjectLock)
}

var _ fstests.InternalTester = (*Fs)(nil)
