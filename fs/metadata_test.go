package fs_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fstest/mockfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadataSet(t *testing.T) {
	var m fs.Metadata
	assert.Nil(t, m)
	m.Set("key", "value")
	assert.NotNil(t, m)
	assert.Equal(t, "value", m["key"])
	m.Set("key", "value2")
	assert.Equal(t, "value2", m["key"])
}

func TestCheckUploadPreservation(t *testing.T) {
	for _, tc := range []struct {
		name                                                                string
		posix, symlinks, local, writePOSIX, writeSymlinks, overlay, wantErr bool
	}{
		{"Disabled", false, false, false, false, false, false, false},
		{"POSIX", true, false, true, true, false, false, false},
		{"Symlinks", false, true, true, false, true, false, false},
		{"Both", true, true, true, true, true, false, false},
		{"RemoteSource", true, true, false, true, true, false, true},
		{"UnsupportedPOSIX", true, false, true, false, true, false, true},
		{"UnsupportedSymlinks", false, true, true, true, false, false, true},
		{"Wrapper", true, true, true, true, true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, ci := fs.AddConfig(context.Background())
			ci.PreservePOSIXProperties, ci.PreserveSymlinks = tc.posix, tc.symlinks
			src, err := mockfs.NewFs(ctx, "source", "", nil)
			require.NoError(t, err)
			dst, err := mockfs.NewFs(ctx, "destination", "", nil)
			require.NoError(t, err)
			src.Features().IsLocal = tc.local
			dst.Features().WritePOSIXProperties = tc.writePOSIX
			dst.Features().WriteSymlinks = tc.writeSymlinks
			dst.Features().Overlay = tc.overlay
			err = fs.CheckUploadPreservation(ctx, dst, src)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMetadataMerge(t *testing.T) {
	for _, test := range []struct {
		in    fs.Metadata
		merge fs.Metadata
		want  fs.Metadata
	}{
		{
			in:    fs.Metadata{},
			merge: fs.Metadata{},
			want:  fs.Metadata{},
		}, {
			in:    nil,
			merge: nil,
			want:  nil,
		}, {
			in:    nil,
			merge: fs.Metadata{},
			want:  nil,
		}, {
			in:    nil,
			merge: fs.Metadata{"a": "1", "b": "2"},
			want:  fs.Metadata{"a": "1", "b": "2"},
		}, {
			in:    fs.Metadata{"a": "1", "b": "2"},
			merge: nil,
			want:  fs.Metadata{"a": "1", "b": "2"},
		}, {
			in:    fs.Metadata{"a": "1", "b": "2"},
			merge: fs.Metadata{"b": "B", "c": "3"},
			want:  fs.Metadata{"a": "1", "b": "B", "c": "3"},
		},
	} {
		what := fmt.Sprintf("in=%v, merge=%v", test.in, test.merge)
		test.in.Merge(test.merge)
		assert.Equal(t, test.want, test.in, what)
	}
}

func TestMetadataMergeOptions(t *testing.T) {
	for _, test := range []struct {
		in   fs.Metadata
		opts []fs.OpenOption
		want fs.Metadata
	}{
		{
			opts: []fs.OpenOption{},
			want: nil,
		}, {
			opts: []fs.OpenOption{&fs.HTTPOption{}},
			want: nil,
		}, {
			opts: []fs.OpenOption{fs.MetadataOption{"a": "1", "b": "2"}},
			want: fs.Metadata{"a": "1", "b": "2"},
		}, {
			opts: []fs.OpenOption{
				&fs.HTTPOption{},
				fs.MetadataOption{"a": "1", "b": "2"},
				fs.MetadataOption{"b": "B", "c": "3"},
				&fs.HTTPOption{},
			},
			want: fs.Metadata{"a": "1", "b": "B", "c": "3"},
		}, {
			in: fs.Metadata{"a": "first", "z": "OK"},
			opts: []fs.OpenOption{
				&fs.HTTPOption{},
				fs.MetadataOption{"a": "1", "b": "2"},
				fs.MetadataOption{"b": "B", "c": "3"},
				&fs.HTTPOption{},
			},
			want: fs.Metadata{"a": "1", "b": "B", "c": "3", "z": "OK"},
		},
	} {
		what := fmt.Sprintf("in=%v, opts=%v", test.in, test.opts)
		test.in.MergeOptions(test.opts)
		assert.Equal(t, test.want, test.in, what)
	}
}

func TestMetadataMapper(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	ci.Metadata = true
	require.NoError(t, ci.MetadataMapper.Set("go run metadata_mapper_code.go"))
	now := time.Date(2001, 2, 3, 4, 5, 6, 7, time.UTC)
	f, err := mockfs.NewFs(ctx, "dstFs", "dstFsRoot", nil)
	require.NoError(t, err)

	t.Run("Normal", func(t *testing.T) {
		o := object.NewMemoryObject("file.txt", now, []byte("hello")).WithMetadata(fs.Metadata{
			"key1": "potato",
			"key2": "sausage",
			"key3": "gravy",
		})
		metadata, err := fs.GetMetadataOptions(ctx, f, o, nil)
		require.NoError(t, err)
		assert.Equal(t, fs.Metadata{
			"key0": "cabbage",
			"key1": "two potato",
			"key2": "sausage",
		}, metadata)
	})

	t.Run("Error", func(t *testing.T) {
		o := object.NewMemoryObject("file.txt", now, []byte("hello")).WithMetadata(fs.Metadata{
			"error": "Red Alert",
		})
		metadata, err := fs.GetMetadataOptions(ctx, f, o, nil)
		require.Error(t, err)
		assert.ErrorContains(t, err, "Red Alert")
		require.Nil(t, metadata)
	})

	t.Run("Merge", func(t *testing.T) {
		o := object.NewMemoryObject("file.txt", now, []byte("hello")).WithMetadata(fs.Metadata{
			"key1": "potato",
			"key2": "sausage",
			"key3": "gravy",
		})
		metadata, err := fs.GetMetadataOptions(ctx, f, o, []fs.OpenOption{fs.MetadataOption(fs.Metadata{
			"option": "optionValue",
			"key1":   "new potato",
			"key2":   "salami",
		})})
		require.NoError(t, err)
		assert.Equal(t, fs.Metadata{
			"key0":   "cabbage",
			"key1":   "two new potato",
			"key2":   "salami",
			"option": "optionValue",
		}, metadata)
	})
}
