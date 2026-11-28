package s3

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConditionalWrites(t *testing.T) {
	for _, backend := range []struct {
		name, remote string
		mode         vfscommon.CacheMode
		inMemory     bool
		disable      []string
	}{
		{name: "Local"},
		{name: "Memory", remote: ":memory,description=conditional:"},
		{name: "Minimal", mode: vfscommon.CacheModeMinimal},
		{name: "Writes", mode: vfscommon.CacheModeWrites},
		{name: "Full", mode: vfscommon.CacheModeFull},
		{name: "MemoryWrites", remote: ":memory,description=conditional-writes:", mode: vfscommon.CacheModeWrites},
		{name: "NoMove", remote: ":memory,description=conditional-no-move:", mode: vfscommon.CacheModeWrites, disable: []string{"Copy"}},
		{name: "InMemoryMultipart", inMemory: true},
	} {
		t.Run(backend.name, func(t *testing.T) {
			opt := vfscommon.Opt
			opt.CacheMode = backend.mode
			opt.WriteBack = 0
			core, f, bucket := newMultipartTestServerVFS(t, backend.remote, backend.inMemory, nil, &opt, backend.disable...)
			ctx := context.Background()
			const original, replacement = "old contents", "new contents"
			_, err := core.PutObject(ctx, bucket, "source", strings.NewReader(replacement), int64(len(replacement)), "", "", minio.PutObjectOptions{UserMetadata: map[string]string{"marker": "new"}})
			require.NoError(t, err)
			for _, method := range []string{"Put", "Copy", "Multipart"} {
				t.Run(method, func(t *testing.T) {
					for _, test := range []struct {
						name        string
						exists      bool
						match, none string
						status      int
						code        string
					}{
						{"Match", true, "current", "", 200, ""},
						{"Stale", true, "stale", "", 412, "PreconditionFailed"},
						{"Missing", false, "stale", "", 404, "NoSuchKey"},
						{"Exists", true, "*", "", 200, ""},
						{"MissingExists", false, "*", "", 404, "NoSuchKey"},
						{"Create", false, "", "*", 200, ""},
						{"AlreadyExists", true, "", "*", 412, "PreconditionFailed"},
						{"NoneMatches", true, "", "current", 412, "PreconditionFailed"},
						{"NoneDiffers", true, "", "different", 200, ""},
						{"Both", true, "current", "*", 412, "PreconditionFailed"},
					} {
						t.Run(test.name, func(t *testing.T) {
							key := method + "-" + test.name
							var etag string
							if test.exists {
								_, err := core.PutObject(ctx, bucket, key, strings.NewReader(original), int64(len(original)), "", "", minio.PutObjectOptions{UserMetadata: map[string]string{"marker": "old"}})
								require.NoError(t, err)
								info, err := core.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
								require.NoError(t, err)
								etag = info.ETag
							}
							options := minio.PutObjectOptions{UserMetadata: map[string]string{"marker": "new"}}
							if test.match != "" {
								value := test.match
								if value == "current" {
									value = etag
								}
								options.SetMatchETag(value)
							}
							if test.none != "" {
								value := test.none
								if value == "current" {
									value = etag
								}
								options.SetMatchETagExcept(value)
							}
							err := conditionalClientUpload(ctx, core, method, bucket, key, replacement, options)
							want, marker := replacement, "new"
							if test.status == 200 {
								require.NoError(t, err)
							} else {
								require.Error(t, err)
								response := minio.ToErrorResponse(err)
								assert.Equal(t, test.status, response.StatusCode)
								assert.Equal(t, test.code, response.Code)
								assert.NotEmpty(t, response.RequestID)
								want, marker = original, "old"
								if !test.exists {
									_, err := core.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
									require.Error(t, err)
									assert.Equal(t, "NoSuchKey", minio.ToErrorResponse(err).Code)
									return
								}
							}
							r, info, _, err := core.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
							require.NoError(t, err)
							got, err := io.ReadAll(r)
							require.NoError(t, r.Close())
							require.NoError(t, err)
							assert.Equal(t, want, string(got))
							assert.Equal(t, marker, info.Metadata.Get("X-Amz-Meta-Marker"))
							if test.status != 200 {
								assert.Equal(t, etag, info.ETag)
							}
							waitForContent(t, f, bucket, key, []byte(want))
						})
					}
				})
			}
		})
	}
}

func conditionalClientUpload(ctx context.Context, core *minio.Core, method, bucket, key, data string, opt minio.PutObjectOptions) error {
	switch method {
	case "Put":
		_, err := core.PutObject(ctx, bucket, key, strings.NewReader(data), int64(len(data)), "", "", opt)
		return err
	case "Copy":
		// Core.CopyObject takes conditional headers through its metadata argument.
		headers := make(map[string]string)
		for key, values := range opt.Header() {
			headers[key] = strings.Join(values, ",")
		}
		_, err := core.CopyObject(ctx, bucket, "source", bucket, key, headers, minio.CopySrcOptions{}, opt)
		return err
	default:
		id, err := core.NewMultipartUpload(ctx, bucket, key, minio.PutObjectOptions{UserMetadata: opt.UserMetadata})
		if err != nil {
			return err
		}
		part, err := core.PutObjectPart(ctx, bucket, key, id, 1, strings.NewReader(data), int64(len(data)), minio.PutObjectPartOptions{})
		if err == nil {
			_, err = core.CompleteMultipartUpload(ctx, bucket, key, id, []minio.CompletePart{{PartNumber: 1, ETag: part.ETag}}, opt)
		}
		if err != nil {
			_ = core.AbortMultipartUpload(ctx, bucket, key, id)
		}
		return err
	}
}

func TestConditionalClientRace(t *testing.T) {
	for _, remote := range []string{"", ":memory,description=conditional-race:"} {
		for _, mode := range []vfscommon.CacheMode{vfscommon.CacheModeOff, vfscommon.CacheModeWrites} {
			t.Run(fmt.Sprintf("%s/%s", remote, mode), func(t *testing.T) {
				vfsOpt := vfscommon.Opt
				vfsOpt.CacheMode = mode
				vfsOpt.WriteBack = 0
				core, _, bucket := newMultipartTestServerVFS(t, remote, false, nil, &vfsOpt)
				for _, create := range []bool{false, true} {
					t.Run(fmt.Sprintf("Create=%t", create), func(t *testing.T) {
						ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
						defer cancel()
						key := fmt.Sprintf("race-%t", create)
						opt := minio.PutObjectOptions{}
						if create {
							opt.SetMatchETagExcept("*")
						} else {
							_, err := core.PutObject(ctx, bucket, key, strings.NewReader("old"), 3, "", "", minio.PutObjectOptions{})
							require.NoError(t, err)
							info, err := core.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
							require.NoError(t, err)
							opt.SetMatchETag(info.ETag)
						}
						type result struct {
							data string
							err  error
						}
						const writers = 8
						start := make(chan struct{})
						results := make(chan result, writers)
						for i := range writers {
							go func() {
								<-start
								data := fmt.Sprintf("writer-%d", i)
								err := conditionalClientUpload(ctx, core, "Put", bucket, key, data, opt)
								results <- result{data, err}
							}()
						}
						close(start)
						var winner string
						successes := 0
						for range writers {
							got := <-results
							if got.err == nil {
								winner = got.data
								successes++
							} else {
								assert.Equal(t, "PreconditionFailed", minio.ToErrorResponse(got.err).Code)
							}
						}
						require.Equal(t, 1, successes)
						r, _, _, err := core.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
						require.NoError(t, err)
						got, err := io.ReadAll(r)
						require.NoError(t, r.Close())
						require.NoError(t, err)
						assert.Equal(t, winner, string(got))
					})
				}
			})
		}
	}
}

func TestConditionalDirtyOverwrite(t *testing.T) {
	opt := vfscommon.Opt
	opt.CacheMode = vfscommon.CacheModeWrites
	opt.WriteBack = fs.Duration(time.Hour)
	core, f, bucket := newMultipartTestServerVFS(t, ":memory,description="+fmt.Sprint(time.Now().UnixNano())+":", false, nil, &opt, "Copy")
	ctx := context.Background()
	_, err := f.Put(ctx, strings.NewReader("old"), object.NewStaticObjectInfo(bucket+"/key", time.Now(), 3, true, nil, f))
	require.NoError(t, err)
	old, err := core.StatObject(ctx, bucket, "key", minio.StatObjectOptions{})
	require.NoError(t, err)
	put := minio.PutObjectOptions{}
	put.SetMatchETag(old.ETag)
	_, err = core.PutObject(ctx, bucket, "key", strings.NewReader("new"), 3, "", "", put)
	require.NoError(t, err)
	_, err = core.PutObject(ctx, bucket, "key", strings.NewReader("bad"), 3, "", "", put)
	require.Equal(t, 412, minio.ToErrorResponse(err).StatusCode)
}

func TestConditionalMultipartValidator(t *testing.T) {
	core, _, bucket := newMultipartTestServer(t, false)
	ctx := context.Background()
	id, err := core.NewMultipartUpload(ctx, bucket, "key", minio.PutObjectOptions{})
	require.NoError(t, err)
	part, err := core.PutObjectPart(ctx, bucket, "key", id, 1, strings.NewReader("new"), 3, minio.PutObjectPartOptions{})
	require.NoError(t, err)
	result, err := core.CompleteMultipartUpload(ctx, bucket, "key", id, []minio.CompletePart{{PartNumber: 1, ETag: part.ETag}}, minio.PutObjectOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, result.ETag)
	info, err := core.StatObject(ctx, bucket, "key", minio.StatObjectOptions{})
	require.NoError(t, err)
	require.Equal(t, result.ETag, info.ETag)
	r, info, _, err := core.GetObject(ctx, bucket, "key", minio.GetObjectOptions{})
	require.NoError(t, err)
	require.NoError(t, r.Close())
	require.Equal(t, result.ETag, info.ETag)
	put := minio.PutObjectOptions{}
	put.SetMatchETagExcept(info.ETag)
	_, err = core.PutObject(ctx, bucket, "key", strings.NewReader("bad"), 3, "", "", put)
	require.Equal(t, 412, minio.ToErrorResponse(err).StatusCode)
	put = minio.PutObjectOptions{}
	put.SetMatchETag(info.ETag)
	_, err = core.PutObject(ctx, bucket, "key", strings.NewReader("two"), 3, "", "", put)
	require.NoError(t, err)
	info, err = core.StatObject(ctx, bucket, "key", minio.StatObjectOptions{})
	require.NoError(t, err)
	require.NotEqual(t, result.ETag, info.ETag)
}
