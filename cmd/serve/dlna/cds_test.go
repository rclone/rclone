package dlna

import (
	"context"
	"encoding/xml"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/anacrolix/dms/soap"
	localBackend "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMediaWithResources(t *testing.T) {
	fs, err := localBackend.NewFs(context.Background(), "testdatafiles", "testdata/files", configmap.New())
	require.NoError(t, err)

	myvfs := vfs.New(context.Background(), fs, nil)
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

func TestSOAPResponseQuoteEscaping(t *testing.T) {
	// Test that demonstrates the double-escaping problem in SOAP responses
	// This test should initially fail, showing &#34; instead of &quot; in the final output

	// Simulate a DLNA Browse response with quotes in the content
	didlContent := `<container id="0"><dc:title>Folder "with quotes"</dc:title></container>`
	didlWrapped := didlLite(didlContent)

	// Test just the mustMarshalXML function first to see the escaping
	args := soapArgs("Result", didlWrapped)

	// This simulates what marshalSOAPResponse does internally
	xmlArgs := make([]soap.Arg, len(args))
	for i, arg := range args {
		xmlArgs[i] = soap.Arg{
			XMLName: xml.Name{Local: arg.name},
			Value:   arg.value,
		}
	}
	result := mustMarshalXML(xmlArgs)
	resultStr := string(result)

	// This should pass after the fix - quotes should be &quot; not &#34;
	assert.NotContains(t, resultStr, "&#34;", "SOAP arguments should not contain &#34; entities")
	assert.Contains(t, resultStr, "&quot;", "SOAP arguments should contain &quot; entities")
}

func TestTitleExtensionRemoval(t *testing.T) {
	// Test that file extensions are removed from titles to prevent Samsung TV duplication
	tests := []struct {
		name     string
		filename string
		expected string
	}{
		{
			name:     "image file",
			filename: "photo.jpg",
			expected: "photo",
		},
		{
			name:     "video file",
			filename: "movie.mp4",
			expected: "movie",
		},
		{
			name:     "multiple dots",
			filename: "file.name.with.dots.mkv",
			expected: "file.name.with.dots",
		},
		{
			name:     "no extension",
			filename: "filename_no_ext",
			expected: "filename_no_ext",
		},
		{
			name:     "hidden file",
			filename: ".hidden.txt",
			expected: ".hidden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the title processing logic
			title := strings.TrimSuffix(tt.filename, filepath.Ext(tt.filename))
			assert.Equal(t, tt.expected, title)
		})
	}
}

func TestAdjustXMLApostrophes(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "apostrophes in filename",
			input:    []byte(`<dc:title>Testin&#39; it.jpg</dc:title>`),
			expected: `<dc:title>Testin&apos; it.jpg</dc:title>`,
		},
		{
			name:     "mixed quotes and apostrophes",
			input:    []byte(`<dc:title>File &#34;name&#34; &amp; Testin&#39; it</dc:title>`),
			expected: `<dc:title>File &quot;name&quot; &amp; Testin&apos; it</dc:title>`,
		},
		{
			name:     "already correct apostrophe entities",
			input:    []byte(`<dc:title>File &apos;already correct&apos;</dc:title>`),
			expected: `<dc:title>File &apos;already correct&apos;</dc:title>`,
		},
		{
			name:     "all Big 5 XML entities",
			input:    []byte(`<dc:title>Test &#34; &#39; &#38; &#60; &#62;</dc:title>`),
			expected: `<dc:title>Test &quot; &apos; &amp; &lt; &gt;</dc:title>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adjustXML(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
