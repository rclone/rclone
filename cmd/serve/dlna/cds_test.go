package dlna

import (
	"context"
	"sort"
	"testing"

	localBackend "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMediaWithResources(t *testing.T) {
	fs, err := localBackend.NewFs(context.Background(), "testdatafiles", "testdata/files", configmap.New())
	require.NoError(t, err)

	myvfs := vfs.New(fs, nil)
	{
		rootNode, err := myvfs.Stat("")
		require.NoError(t, err)

		rootDir := rootNode.(*vfs.Dir)
		dirEntries, err := rootDir.ReadDirAll()
		require.NoError(t, err)

		mediaItems, assocResources := mediaWithResources(dirEntries)

		// ensure mediaItems contains some items we care about.
		// We specifically check that the .mp4 file and a child directory is kept.
		var videoMp4 *vfs.Node
		foundSubdir := false
		for _, mediaItem := range mediaItems {
			if mediaItem.Name() == "video.mp4" {
				videoMp4 = &mediaItem
			} else if mediaItem.Name() == "subdir" {
				foundSubdir = true
			}
		}

		assert.True(t, videoMp4 != nil, "expected mp4 to be found")
		assert.True(t, foundSubdir, "expected subdir to be found")

		assocVideoResource, ok := assocResources[*videoMp4]
		require.True(t, ok, "expected video.mp4 to have assoc video resource")

		// ensure both video.en.srt and video.srt are in assocVideoResource.
		assocVideoResourceNames := make([]string, 0)
		for _, e := range assocVideoResource {
			assocVideoResourceNames = append(assocVideoResourceNames, e.Name())
		}
		sort.Strings(assocVideoResourceNames)
		assert.Equal(t, []string{"video.en.srt", "video.srt"}, assocVideoResourceNames)
	}
	// Now test inside subdir2.
	// This directory only contains a video.mp4 file, but as it also contains a
	// "Subs" subdir, `mediaWithResources` is called with its children appended,
	// causing the media items are appropriately populated.
	{
		rootNode, err := myvfs.Stat("subdir2")
		require.NoError(t, err)

		subtitleNode, err := myvfs.Stat("subdir2/Subs")
		require.NoError(t, err)

		rootDir := rootNode.(*vfs.Dir)
		subtitleDir := subtitleNode.(*vfs.Dir)

		dirEntries, err := rootDir.ReadDirAll()
		require.NoError(t, err)

		subtitleEntries, err := subtitleDir.ReadDirAll()
		require.NoError(t, err)

		dirEntries = append(dirEntries, subtitleEntries...)

		mediaItems, assocResources := mediaWithResources(dirEntries)

		// ensure mediaItems contains some items we care about.
		// We specifically check that the .mp4 file is kept.
		var videoMp4 *vfs.Node
		for _, mediaItem := range mediaItems {
			if mediaItem.Name() == "video.mp4" {
				videoMp4 = &mediaItem
			}
		}

		assert.True(t, videoMp4 != nil, "expected mp4 to be found")

		assocVideoResource, ok := assocResources[*videoMp4]
		require.True(t, ok, "expected video.mp4 to have assoc video resource")

		// ensure both video.en.srt and video.srt are in assocVideoResource.
		assocVideoResourceNames := make([]string, 0)
		for _, e := range assocVideoResource {
			assocVideoResourceNames = append(assocVideoResourceNames, e.Name())
		}
		sort.Strings(assocVideoResourceNames)
		assert.Equal(t, []string{"video.en.srt", "video.srt"}, assocVideoResourceNames)
	}

	// Now test subdir3. It contains a video.mpv, as well as Sub/video.{idx,sub}.
	{
		rootNode, err := myvfs.Stat("subdir3")
		require.NoError(t, err)

		subtitleNode, err := myvfs.Stat("subdir3/Subs")
		require.NoError(t, err)

		rootDir := rootNode.(*vfs.Dir)
		subtitleDir := subtitleNode.(*vfs.Dir)

		dirEntries, err := rootDir.ReadDirAll()
		require.NoError(t, err)

		subtitleEntries, err := subtitleDir.ReadDirAll()
		require.NoError(t, err)

		dirEntries = append(dirEntries, subtitleEntries...)

		mediaItems, assocResources := mediaWithResources(dirEntries)

		// ensure mediaItems contains some items we care about.
		// We specifically check that the .mp4 file is kept.
		var videoMp4 *vfs.Node
		for _, mediaItem := range mediaItems {
			if mediaItem.Name() == "video.mp4" {
				videoMp4 = &mediaItem
			}
		}

		assert.True(t, videoMp4 != nil, "expected mp4 to be found")

		// test assocResources to point from the video file to the subtitles
		assocVideoResource, ok := assocResources[*videoMp4]
		require.True(t, ok, "expected video.mp4 to have assoc video resource")

		// ensure both video.idx and video.sub are in assocVideoResource.
		assocVideoResourceNames := make([]string, 0)
		for _, e := range assocVideoResource {
			assocVideoResourceNames = append(assocVideoResourceNames, e.Name())
		}
		sort.Strings(assocVideoResourceNames)
		assert.Equal(t, []string{"video.idx", "video.sub"}, assocVideoResourceNames)
	}

}
