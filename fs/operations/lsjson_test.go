package operations_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compare a and b in a file system independent way
func compareListJSONItem(t *testing.T, a, b *operations.ListJSONItem, precision time.Duration) {
	assert.Equal(t, a.Path, b.Path, "Path")
	assert.Equal(t, a.Name, b.Name, "Name")
	// assert.Equal(t, a.EncryptedPath, b.EncryptedPath, "EncryptedPath")
	// assert.Equal(t, a.Encrypted, b.Encrypted, "Encrypted")
	if !a.IsDir {
		assert.Equal(t, a.Size, b.Size, "Size")
	}
	// assert.Equal(t, a.MimeType, a.Mib.MimeType, "MimeType")
	if !a.IsDir {
		fstest.AssertTimeEqualWithPrecision(t, "ListJSON", a.ModTime.When, b.ModTime.When, precision)
	}
	assert.Equal(t, a.IsDir, b.IsDir, "IsDir")
	// assert.Equal(t, a.Hashes, a.b.Hashes, "Hashes")
	// assert.Equal(t, a.ID, b.ID, "ID")
	// assert.Equal(t, a.OrigID, a.b.OrigID, "OrigID")
	// assert.Equal(t, a.Tier, b.Tier, "Tier")
	// assert.Equal(t, a.IsBucket, a.Isb.IsBucket, "IsBucket")
}

func TestListJSON(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	file1 := r.WriteBoth(ctx, "file1", "file1", t1)
	file2 := r.WriteBoth(ctx, "sub/file2", "sub/file2", t2)

	r.CheckRemoteItems(t, file1, file2)
	precision := fs.GetModifyWindow(ctx, r.Fremote)

	for _, test := range []struct {
		name   string
		remote string
		opt    operations.ListJSONOpt
		want   []*operations.ListJSONItem
	}{
		{
			name: "Default",
			opt:  operations.ListJSONOpt{},
			want: []*operations.ListJSONItem{{
				Path:    "file1",
				Name:    "file1",
				Size:    5,
				ModTime: operations.Timestamp{When: t1},
				IsDir:   false,
			}, {
				Path:  "sub",
				Name:  "sub",
				IsDir: true,
			}},
		}, {
			name: "FilesOnly",
			opt: operations.ListJSONOpt{
				FilesOnly: true,
			},
			want: []*operations.ListJSONItem{{
				Path:    "file1",
				Name:    "file1",
				Size:    5,
				ModTime: operations.Timestamp{When: t1},
				IsDir:   false,
			}},
		}, {
			name: "DirsOnly",
			opt: operations.ListJSONOpt{
				DirsOnly: true,
			},
			want: []*operations.ListJSONItem{{
				Path:  "sub",
				Name:  "sub",
				IsDir: true,
			}},
		}, {
			name: "Recurse",
			opt: operations.ListJSONOpt{
				Recurse: true,
			},
			want: []*operations.ListJSONItem{{
				Path:    "file1",
				Name:    "file1",
				Size:    5,
				ModTime: operations.Timestamp{When: t1},
				IsDir:   false,
			}, {
				Path:  "sub",
				Name:  "sub",
				IsDir: true,
			}, {
				Path:    "sub/file2",
				Name:    "file2",
				Size:    9,
				ModTime: operations.Timestamp{When: t2},
				IsDir:   false,
			}},
		}, {
			name:   "SubDir",
			remote: "sub",
			opt:    operations.ListJSONOpt{},
			want: []*operations.ListJSONItem{{
				Path:    "sub/file2",
				Name:    "file2",
				Size:    9,
				ModTime: operations.Timestamp{When: t2},
				IsDir:   false,
			}},
		}, {
			name: "NoModTime",
			opt: operations.ListJSONOpt{
				FilesOnly: true,
				NoModTime: true,
			},
			want: []*operations.ListJSONItem{{
				Path:    "file1",
				Name:    "file1",
				Size:    5,
				ModTime: operations.Timestamp{When: time.Time{}},
				IsDir:   false,
			}},
		}, {
			name: "NoMimeType",
			opt: operations.ListJSONOpt{
				FilesOnly:  true,
				NoMimeType: true,
			},
			want: []*operations.ListJSONItem{{
				Path:    "file1",
				Name:    "file1",
				Size:    5,
				ModTime: operations.Timestamp{When: t1},
				IsDir:   false,
			}},
		}, {
			name: "ShowHash",
			opt: operations.ListJSONOpt{
				FilesOnly: true,
				ShowHash:  true,
			},
			want: []*operations.ListJSONItem{{
				Path:    "file1",
				Name:    "file1",
				Size:    5,
				ModTime: operations.Timestamp{When: t1},
				IsDir:   false,
			}},
		}, {
			name: "HashTypes",
			opt: operations.ListJSONOpt{
				FilesOnly: true,
				ShowHash:  true,
				HashTypes: []string{"MD5"},
			},
			want: []*operations.ListJSONItem{{
				Path:    "file1",
				Name:    "file1",
				Size:    5,
				ModTime: operations.Timestamp{When: t1},
				IsDir:   false,
			}},
		}, {
			name: "Metadata",
			opt: operations.ListJSONOpt{
				FilesOnly: false,
				Metadata:  true,
			},
			want: []*operations.ListJSONItem{{
				Path:    "file1",
				Name:    "file1",
				Size:    5,
				ModTime: operations.Timestamp{When: t1},
				IsDir:   false,
			}, {
				Path:  "sub",
				Name:  "sub",
				IsDir: true,
			}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var got []*operations.ListJSONItem
			require.NoError(t, operations.ListJSON(ctx, r.Fremote, test.remote, &test.opt, func(item *operations.ListJSONItem) error {
				got = append(got, item)
				return nil
			}))
			sort.Slice(got, func(i, j int) bool {
				return got[i].Path < got[j].Path
			})
			require.Equal(t, len(test.want), len(got), "Wrong number of results")
			for i := range test.want {
				compareListJSONItem(t, test.want[i], got[i], precision)
				if test.opt.NoMimeType {
					assert.Equal(t, "", got[i].MimeType)
				} else {
					assert.NotEqual(t, "", got[i].MimeType)
				}
				if test.opt.Metadata {
					features := r.Fremote.Features()
					if features.ReadMetadata && !got[i].IsDir {
						assert.Greater(t, len(got[i].Metadata), 0, "Expecting metadata for file")
					}
					if features.ReadDirMetadata && got[i].IsDir {
						assert.Greater(t, len(got[i].Metadata), 0, "Expecting metadata for dir")
					}
				}
				if test.opt.ShowHash {
					hashes := got[i].Hashes
					assert.NotNil(t, hashes)
					if len(test.opt.HashTypes) > 0 && len(hashes) > 0 {
						assert.Equal(t, 1, len(hashes))
					}
					if hashes["crc32"] != "" {
						assert.Equal(t, "9ee760e5", hashes["crc32"])
					}
					if hashes["dropbox"] != "" {
						assert.Equal(t, "f4d62afeaee6f35d3efdd8c66623360395165473bcc958f835343eb3f542f983", hashes["dropbox"])
					}
					if hashes["mailru"] != "" {
						assert.Equal(t, "66696c6531000000000000000000000000000000", hashes["mailru"])
					}
					if hashes["md5"] != "" {
						assert.Equal(t, "826e8142e6baabe8af779f5f490cf5f5", hashes["md5"])
					}
					if hashes["quickxor"] != "" {
						assert.Equal(t, "6648031bca100300000000000500000000000000", hashes["quickxor"])
					}
					if hashes["sha1"] != "" {
						assert.Equal(t, "60b27f004e454aca81b0480209cce5081ec52390", hashes["sha1"])
					}
					if hashes["sha256"] != "" {
						assert.Equal(t, "c147efcfc2d7ea666a9e4f5187b115c90903f0fc896a56df9a6ef5d8f3fc9f31", hashes["sha256"])
					}
					if hashes["whirlpool"] != "" {
						assert.Equal(t, "02fa11755b6470bfc5aab6d94cde5cf2939474fb5b0ebbf8ddf3d32bf06aa438eb92eac097047c02017dc1c317ee83fa8a2717ca4d544b4ee75b3231d1c466b0", hashes["whirlpool"])
					}
				} else {
					assert.Nil(t, got[i].Hashes)
				}
			}
		})
	}
}

func TestStatJSON(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	file1 := r.WriteBoth(ctx, "file1", "file1", t1)
	file2 := r.WriteBoth(ctx, "sub/file2", "sub/file2", t2)

	r.CheckRemoteItems(t, file1, file2)
	precision := fs.GetModifyWindow(ctx, r.Fremote)

	for _, test := range []struct {
		name   string
		remote string
		opt    operations.ListJSONOpt
		want   *operations.ListJSONItem
	}{
		{
			name:   "Root",
			remote: "",
			opt:    operations.ListJSONOpt{},
			want: &operations.ListJSONItem{
				Path:  "",
				Name:  "",
				IsDir: true,
			},
		}, {
			name:   "RootFilesOnly",
			remote: "",
			opt: operations.ListJSONOpt{
				FilesOnly: true,
			},
			want: nil,
		}, {
			name:   "RootDirsOnly",
			remote: "",
			opt: operations.ListJSONOpt{
				DirsOnly: true,
			},
			want: &operations.ListJSONItem{
				Path:  "",
				Name:  "",
				IsDir: true,
			},
		}, {
			name:   "Dir",
			remote: "sub",
			opt:    operations.ListJSONOpt{},
			want: &operations.ListJSONItem{
				Path:  "sub",
				Name:  "sub",
				IsDir: true,
			},
		}, {
			name:   "DirWithTrailingSlash",
			remote: "sub/",
			opt:    operations.ListJSONOpt{},
			want: &operations.ListJSONItem{
				Path:  "sub",
				Name:  "sub",
				IsDir: true,
			},
		}, {
			name:   "File",
			remote: "file1",
			opt:    operations.ListJSONOpt{},
			want: &operations.ListJSONItem{
				Path:    "file1",
				Name:    "file1",
				Size:    5,
				ModTime: operations.Timestamp{When: t1},
				IsDir:   false,
			},
		}, {
			name:   "NotFound",
			remote: "notfound",
			opt:    operations.ListJSONOpt{},
			want:   nil,
		}, {
			name:   "DirFilesOnly",
			remote: "sub",
			opt: operations.ListJSONOpt{
				FilesOnly: true,
			},
			want: nil,
		}, {
			name:   "FileFilesOnly",
			remote: "file1",
			opt: operations.ListJSONOpt{
				FilesOnly: true,
			},
			want: &operations.ListJSONItem{
				Path:    "file1",
				Name:    "file1",
				Size:    5,
				ModTime: operations.Timestamp{When: t1},
				IsDir:   false,
			},
		}, {
			name:   "NotFoundFilesOnly",
			remote: "notfound",
			opt: operations.ListJSONOpt{
				FilesOnly: true,
			},
			want: nil,
		}, {
			name:   "DirDirsOnly",
			remote: "sub",
			opt: operations.ListJSONOpt{
				DirsOnly: true,
			},
			want: &operations.ListJSONItem{
				Path:  "sub",
				Name:  "sub",
				IsDir: true,
			},
		}, {
			name:   "FileDirsOnly",
			remote: "file1",
			opt: operations.ListJSONOpt{
				DirsOnly: true,
			},
			want: nil,
		}, {
			name:   "NotFoundDirsOnly",
			remote: "notfound",
			opt: operations.ListJSONOpt{
				DirsOnly: true,
			},
			want: nil,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := operations.StatJSON(ctx, r.Fremote, test.remote, &test.opt)
			require.NoError(t, err)
			if test.want == nil {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			compareListJSONItem(t, test.want, got, precision)
		})
	}

	t.Run("RootNotFound", func(t *testing.T) {
		f, err := fs.NewFs(ctx, r.FremoteName+"/notfound")
		require.NoError(t, err)
		_, err = operations.StatJSON(ctx, f, "", &operations.ListJSONOpt{})
		// This should return an error except for bucket based remotes
		assert.True(t, err != nil || f.Features().BucketBased, "Need an error for non bucket based backends")
	})
}
