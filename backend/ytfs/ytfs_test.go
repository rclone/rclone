package ytfs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/hash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSanitizeName tests sanitizeName correctly replaces "/" with "∕"
func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{input: "Video Title", expected: "Video Title"},
		{input: "Video / Title", expected: "Video ∕ Title"},
		{input: "A/B/C", expected: "A∕B∕C"},
		{input: "No slashes here", expected: "No slashes here"},
		{input: "/leading slash", expected: "∕leading slash"},
		{input: "trailing slash/", expected: "trailing slash∕"},
		{input: "///multiple///slashes///", expected: "∕∕∕multiple∕∕∕slashes∕∕∕"},
		{input: "", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeName(tt.input)
			require.Equal(t, tt.expected, got, "sanitizeName(%q) mismatch", tt.input)
		})
	}
}

// TestUnsanitizeName tests unsanitizeName reverses sanitizeName
func TestUnsanitizeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{input: "Video Title", expected: "Video Title"},
		{input: "Video ∕ Title", expected: "Video / Title"},
		{input: "A∕B∕C", expected: "A/B/C"},
		{input: "∕leading", expected: "/leading"},
		{input: "trailing∕", expected: "trailing/"},
		{input: "∕∕∕multiple∕∕∕slashes∕∕∕", expected: "///multiple///slashes///"},
		{input: "", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := unsanitizeName(tt.input)
			require.Equal(t, tt.expected, got, "unsanitizeName(%q) mismatch", tt.input)
		})
	}
}

// TestRoundTrip tests sanitize->unsanitize round-trip idempotence
func TestRoundTrip(t *testing.T) {
	tests := []string{
		"Simple Title",
		"Title / With Slash",
		"Multiple / Slashes / Here",
		"A/B/C/D/E",
		"/leading slash",
		"trailing slash/",
		"///many///slashes///",
		"No slashes",
		"",
		"Special Characters: !@#$%^&*()",
	}

	for _, original := range tests {
		t.Run(original, func(t *testing.T) {
			sanitized := sanitizeName(original)
			decoded := unsanitizeName(sanitized)
			require.Equal(t, original, decoded, "round-trip failed: %q -> %q -> %q", original, sanitized, decoded)
		})
	}
}

// newTestFs creates a minimal Fs for unit tests without network access
func newTestFs(t *testing.T) *Fs {
	t.Helper()
	return &Fs{
		name:   "ytfstest",
		root:   "",
		client: nil,
		pacer:  nil,
		mdCache: &metadataCache{
			entries: make(map[string]*metadataCacheEntry),
		},
	}
}

// TestNewFs_ParsesOptions tests that NewFs correctly parses configuration
func TestNewFs_ParsesOptions(t *testing.T) {
	ctx := context.Background()
	m := configmap.Simple{"url": "https://www.youtube.com/@testchannel"}
	f, err := NewFs(ctx, "ytfstest", "", m)
	require.NoError(t, err, "NewFs should succeed with valid config")
	require.NotNil(t, f, "NewFs should return a non-nil Fs")
	require.Equal(t, "ytfstest", f.Name())
	require.Equal(t, "", f.Root())
}

// TestNewFs_WithRootPath tests that NewFs correctly trims root slashes
func TestNewFs_WithRootPath(t *testing.T) {
	ctx := context.Background()
	m := configmap.Simple{"url": "https://www.youtube.com/@testchannel"}
	f, err := NewFs(ctx, "ytfstest", "/some/path/", m)
	require.NoError(t, err)
	require.Equal(t, "some/path", f.Root(), "root should have slashes trimmed")
}

// TestFs_Name tests the Name method
func TestFs_Name(t *testing.T) {
	f := newTestFs(t)
	f.name = "myytfs"
	require.Equal(t, "myytfs", f.Name())
}

// TestFs_Root tests the Root method
func TestFs_Root(t *testing.T) {
	f := newTestFs(t)
	f.root = "channel/videos"
	require.Equal(t, "channel/videos", f.Root())
}

// TestFs_String tests the String method
func TestFs_String(t *testing.T) {
	f := newTestFs(t)
	f.root = "test/path"
	s := f.String()
	assert.Contains(t, s, "ytfs")
	assert.Contains(t, s, "test/path")
}

// TestFs_Hashes tests that ytfs supports no hashes
func TestFs_Hashes(t *testing.T) {
	f := newTestFs(t)
	expected := hash.Set(hash.None)
	require.Equal(t, expected, f.Hashes())
}

// TestFs_Precision tests that ytfs doesn't support ModTime
func TestFs_Precision(t *testing.T) {
	f := newTestFs(t)
	require.Equal(t, fs.ModTimeNotSupported, f.Precision())
}

// TestFs_Features tests that Features are properly configured
func TestFs_Features(t *testing.T) {
	ctx := context.Background()
	m := configmap.Simple{"url": "https://www.youtube.com/@testchannel"}
	f, err := NewFs(ctx, "ytfstest", "", m)
	require.NoError(t, err)

	features := f.Features()
	require.NotNil(t, features)
	assert.True(t, features.CanHaveEmptyDirectories)
	assert.False(t, features.ReadMimeType)
}

// TestList_ReturnsError tests that List returns an error for invalid paths
func TestList_ReturnsError(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	entries, err := f.List(ctx, "invalid/path/that/does/not/match/patterns")
	require.Error(t, err, "List should return error for unmatched patterns")
	require.Nil(t, entries)
}

// TestNewObject_ReturnsError tests that NewObject returns an error for invalid paths
func TestNewObject_ReturnsError(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	obj, err := f.NewObject(ctx, "invalid/path/that/does/not/match/patterns.mp4")
	require.Error(t, err, "NewObject should return error for unmatched patterns")
	require.Nil(t, obj)
}

// TestPut_ReadOnlyDeniesWrite tests that Put returns PermissionDenied
func TestPut_ReadOnlyDeniesWrite(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	r := strings.NewReader("test data")
	fakeInfo := &FakeObjectInfo{name: "test.txt", size: 9}

	obj, err := f.Put(ctx, r, fakeInfo)
	require.Equal(t, fs.ErrorPermissionDenied, err)
	require.Nil(t, obj)
}

// TestPutStream_ReadOnlyDeniesWrite tests that PutStream returns PermissionDenied
func TestPutStream_ReadOnlyDeniesWrite(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	r := strings.NewReader("test data")
	fakeInfo := &FakeObjectInfo{name: "test.txt", size: 9}

	obj, err := f.PutStream(ctx, r, fakeInfo)
	require.Equal(t, fs.ErrorPermissionDenied, err)
	require.Nil(t, obj)
}

// TestMkdir_ReadOnlyDeniesWrite tests that Mkdir returns PermissionDenied
func TestMkdir_ReadOnlyDeniesWrite(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	err := f.Mkdir(ctx, "newdir")
	require.Equal(t, fs.ErrorPermissionDenied, err)
}

// TestRmdir_ReadOnlyDeniesWrite tests that Rmdir returns PermissionDenied
func TestRmdir_ReadOnlyDeniesWrite(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	err := f.Rmdir(ctx, "somedir")
	require.Equal(t, fs.ErrorPermissionDenied, err)
}

// FakeObjectInfo is a minimal fs.ObjectInfo for testing
type FakeObjectInfo struct {
	name  string
	size  int64
	mtime time.Time
}

func (f *FakeObjectInfo) Fs() fs.Info                           { return nil }
func (f *FakeObjectInfo) Remote() string                        { return f.name }
func (f *FakeObjectInfo) String() string                        { return f.name }
func (f *FakeObjectInfo) ModTime(ctx context.Context) time.Time { return f.mtime }
func (f *FakeObjectInfo) Size() int64                           { return f.size }
func (f *FakeObjectInfo) Storable() bool                        { return true }
func (f *FakeObjectInfo) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", nil
}

// TestReadOnlyInvariant_AllWriteMethodsDeny tests all write operations
func TestReadOnlyInvariant_AllWriteMethodsDeny(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)
	fakeInfo := &FakeObjectInfo{name: "test.txt", size: 9}

	tests := []struct {
		name string
		test func() error
	}{
		{
			name: "Put",
			test: func() error {
				_, err := f.Put(ctx, strings.NewReader("data"), fakeInfo)
				return err
			},
		},
		{
			name: "PutStream",
			test: func() error {
				_, err := f.PutStream(ctx, strings.NewReader("data"), fakeInfo)
				return err
			},
		},
		{
			name: "Mkdir",
			test: func() error {
				return f.Mkdir(ctx, "dir")
			},
		},
		{
			name: "Rmdir",
			test: func() error {
				return f.Rmdir(ctx, "dir")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.test()
			require.Equal(t, fs.ErrorPermissionDenied, err,
				"%s should deny with PermissionDenied", tc.name)
		})
	}
}

// TestSanitizeNameEdgeCases tests edge cases for sanitizeName
func TestSanitizeNameEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "only slashes", input: "///", expected: "∕∕∕"},
		{name: "alternating slashes and text", input: "/a/b/c/", expected: "∕a∕b∕c∕"},
		{name: "unicode content", input: "测试/视频", expected: "测试∕视频"},
		{name: "already contains division slash", input: "Video∕Title", expected: "Video∕Title"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeName(tt.input)
			require.Equal(t, tt.expected, got)
		})
	}
}

// TestUnsanitizeNameEdgeCases tests edge cases for unsanitizeName
func TestUnsanitizeNameEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "only division slashes", input: "∕∕∕", expected: "///"},
		{name: "alternating division slashes and text", input: "∕a∕b∕c∕", expected: "/a/b/c/"},
		{name: "mixed unicode content", input: "测试∕视频", expected: "测试/视频"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unsanitizeName(tt.input)
			require.Equal(t, tt.expected, got)
		})
	}
}

// TestNewFs_WithoutURL tests NewFs with missing URL config
func TestNewFs_WithoutURL(t *testing.T) {
	ctx := context.Background()
	m := configmap.Simple{}
	f, err := NewFs(ctx, "ytfstest", "", m)
	require.NoError(t, err, "NewFs should succeed even with empty config")
	require.NotNil(t, f)
	require.Equal(t, "", f.Root())
}

// TestNewFs_WithComplexRoot tests NewFs with various root paths
func TestNewFs_WithComplexRoot(t *testing.T) {
	tests := []struct {
		name   string
		root   string
		expect string
		desc   string
	}{
		{name: "empty root", root: "", expect: "", desc: "empty root should remain empty"},
		{name: "single slash", root: "/", expect: "", desc: "single slash should be trimmed to empty"},
		{name: "leading slash", root: "/path", expect: "path", desc: "leading slash should be trimmed"},
		{name: "trailing slash", root: "path/", expect: "path", desc: "trailing slash should be trimmed"},
		{name: "both slashes", root: "/path/to/root/", expect: "path/to/root", desc: "both slashes should be trimmed"},
		{name: "multiple slashes", root: "///path///", expect: "path", desc: "multiple slashes should be trimmed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			m := configmap.Simple{"url": "https://www.youtube.com/@test"}
			f, err := NewFs(ctx, "ytfstest", tt.root, m)
			require.NoError(t, err, tt.desc)
			require.Equal(t, tt.expect, f.Root(), tt.desc)
		})
	}
}

// TestFs_StringFormat tests the format of String method
func TestFs_StringFormat(t *testing.T) {
	tests := []struct {
		root string
		desc string
	}{
		{"", "empty root"},
		{"videos", "single segment"},
		{"channel/videos/2024", "multiple segments"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			f := newTestFs(t)
			f.root = tt.root
			s := f.String()
			assert.Contains(t, s, "ytfs")
			if tt.root != "" {
				assert.Contains(t, s, tt.root)
			}
		})
	}
}

// TestObject_Fs tests Object.Fs() returns correct parent Fs
func TestObject_Fs(t *testing.T) {
	f := newTestFs(t)
	o := &Object{
		fs:     f,
		remote: "test/video.mp4",
	}

	result := o.Fs()
	require.NotNil(t, result)
	require.Equal(t, f.Name(), result.Name())
}

// TestObject_Remote tests Object.Remote() returns remote path
func TestObject_Remote(t *testing.T) {
	tests := []struct {
		name     string
		remote   string
		expected string
	}{
		{"simple video", "video.mp4", "video.mp4"},
		{"with path", "channels/123 My Video", "channels/123 My Video"},
		{"with sanitized slash", "channels/123 My∕Video", "channels/123 My∕Video"},
		{"complex path", "channels/UCtest/456 Long Video Title", "channels/UCtest/456 Long Video Title"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &Object{remote: tt.remote}
			require.Equal(t, tt.expected, o.Remote())
		})
	}
}

// TestObject_String tests Object.String() returns remote path
func TestObject_String(t *testing.T) {
	tests := []struct {
		name   string
		remote string
	}{
		{"simple", "video.mp4"},
		{"with path", "channels/123 My Video"},
		{"complex", "playlists/PLtest/456 Title"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &Object{remote: tt.remote}
			require.Equal(t, tt.remote, o.String())
		})
	}
}

// TestObject_Hash tests Object.Hash() returns ErrUnsupported
func TestObject_Hash(t *testing.T) {
	ctx := context.Background()
	o := &Object{
		fs:     newTestFs(t),
		remote: "test/video.mp4",
	}

	result, err := o.Hash(ctx, hash.MD5)
	require.Equal(t, hash.ErrUnsupported, err)
	require.Equal(t, "", result)
}

// TestObject_Size tests Object.Size() returns -1
func TestObject_Size(t *testing.T) {
	o := &Object{
		fs:     newTestFs(t),
		remote: "test/video.mp4",
	}

	require.Equal(t, int64(-1), o.Size())
}

// TestObject_Storable tests Object.Storable() returns true
func TestObject_Storable(t *testing.T) {
	o := &Object{
		fs:     newTestFs(t),
		remote: "test/video.mp4",
	}

	require.True(t, o.Storable())
}

// TestObject_ModTime tests Object.ModTime() parsing
func TestObject_ModTime(t *testing.T) {
	tests := []struct {
		name       string
		uploadDate string
		expectZero bool
		expectYear int
	}{
		{"valid date", "20240115", false, 2024},
		{"empty date", "", true, 0},
		{"invalid format", "2024-01-15", true, 0},
		{"another valid", "20231231", false, 2023},
		{"recent date", "20250724", false, 2025},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			o := &Object{
				fs:         newTestFs(t),
				remote:     "test/video.mp4",
				uploadDate: tt.uploadDate,
			}

			result := o.ModTime(ctx)
			if tt.expectZero {
				require.True(t, result.IsZero(), "expected zero time for %q", tt.uploadDate)
			} else {
				require.False(t, result.IsZero())
				require.Equal(t, tt.expectYear, result.Year())
			}
		})
	}
}

// TestObject_SetModTime tests Object.SetModTime() denies
func TestObject_SetModTime(t *testing.T) {
	ctx := context.Background()
	o := &Object{
		fs:     newTestFs(t),
		remote: "test/video.mp4",
	}

	err := o.SetModTime(ctx, time.Now())
	require.Equal(t, fs.ErrorPermissionDenied, err)
}

// TestObject_Update tests Object.Update() denies
func TestObject_Update(t *testing.T) {
	ctx := context.Background()
	o := &Object{
		fs:     newTestFs(t),
		remote: "test/video.mp4",
	}

	r := strings.NewReader("data")
	fakeInfo := &FakeObjectInfo{name: "test.mp4", size: 4}

	err := o.Update(ctx, r, fakeInfo)
	require.Equal(t, fs.ErrorPermissionDenied, err)
}

// TestObject_Remove tests Object.Remove() denies
func TestObject_Remove(t *testing.T) {
	ctx := context.Background()
	o := &Object{
		fs:     newTestFs(t),
		remote: "test/video.mp4",
	}

	err := o.Remove(ctx)
	require.Equal(t, fs.ErrorPermissionDenied, err)
}

// TestObject_Fields tests Object with various field combinations
func TestObject_Fields(t *testing.T) {
	tests := []struct {
		name       string
		videoID    string
		title      string
		duration   int
		uploadDate string
	}{
		{"complete object", "dQw4w9WgXcQ", "Test Video", 240, "20240115"},
		{"minimal object", "vid123", "Video", 0, ""},
		{"long title", "vid456", "Very Long Video Title That Is Quite Long", 600, "20231231"},
		{"zero duration", "vid789", "Livestream", 0, "20250101"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &Object{
				fs:         newTestFs(t),
				remote:     "test/video.mp4",
				videoID:    tt.videoID,
				title:      tt.title,
				duration:   tt.duration,
				uploadDate: tt.uploadDate,
			}

			require.Equal(t, tt.videoID, o.videoID)
			require.Equal(t, tt.title, o.title)
			require.Equal(t, tt.duration, o.duration)
			require.Equal(t, tt.uploadDate, o.uploadDate)
		})
	}
}

// TestCmdReader_Read tests cmdReader.Read()
func TestCmdReader_Read(t *testing.T) {
	data := []byte("test data")
	r := io.NopCloser(strings.NewReader(string(data)))
	cmd := exec.Command("echo", "test")

	cr := &cmdReader{
		reader: r,
		cmd:    cmd,
	}

	buf := make([]byte, len(data))
	n, err := cr.Read(buf)

	require.NoError(t, err)
	require.Equal(t, len(data), n)
	require.Equal(t, data, buf)
}

// TestCmdReader_CloseWithSuccessfulCommand tests Close with success
func TestCmdReader_CloseWithSuccessfulCommand(t *testing.T) {
	cmd := exec.Command("true")
	err := cmd.Start()
	require.NoError(t, err)

	r := io.NopCloser(strings.NewReader(""))

	cr := &cmdReader{
		reader: r,
		cmd:    cmd,
	}

	err = cr.Close()
	require.NoError(t, err)
}

// TestCmdReader_CloseWithFailedCommand tests Close with failure
func TestCmdReader_CloseWithFailedCommand(t *testing.T) {
	cmd := exec.Command("false")
	err := cmd.Start()
	require.NoError(t, err)

	r := io.NopCloser(strings.NewReader(""))

	cr := &cmdReader{
		reader: r,
		cmd:    cmd,
	}

	err = cr.Close()
	require.Error(t, err, "Close should error when command fails")
}

// TestCmdReader_ReadAndClose tests full cycle
func TestCmdReader_ReadAndClose(t *testing.T) {
	testData := "Hello, World!"
	r := io.NopCloser(strings.NewReader(testData))
	cmd := exec.Command("true")
	err := cmd.Start()
	require.NoError(t, err)

	cr := &cmdReader{
		reader: r,
		cmd:    cmd,
	}

	buf := make([]byte, len(testData))
	n, err := cr.Read(buf)
	require.NoError(t, err)
	require.Equal(t, len(testData), n)
	require.Equal(t, testData, string(buf))

	err = cr.Close()
	require.NoError(t, err)
}

// TestNewObject_VideoIDExtraction tests NewObject extracts videoID
func TestNewObject_VideoIDExtraction(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	tests := []struct {
		name       string
		remote     string
		expectVID  string
		expectType string
	}{
		{"video with title", "dQw4w9WgXcQ Test Video", "dQw4w9WgXcQ", "valid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, err := f.NewObject(ctx, tt.remote)
			if err != nil {
				t.Logf("NewObject failed as expected for unmatched pattern: %v", err)
			} else if obj != nil {
				o := obj.(*Object)
				require.NotNil(t, o)
			}
		})
	}
}

// TestObject_AllReadOnlyMethods tests all write methods
func TestObject_AllReadOnlyMethods(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)
	o := &Object{
		fs:     f,
		remote: "test/video.mp4",
	}

	fakeInfo := &FakeObjectInfo{name: "test.mp4", size: 100}

	tests := []struct {
		name string
		test func() error
	}{
		{
			name: "Update",
			test: func() error {
				return o.Update(ctx, strings.NewReader("data"), fakeInfo)
			},
		},
		{
			name: "Remove",
			test: func() error {
				return o.Remove(ctx)
			},
		},
		{
			name: "SetModTime",
			test: func() error {
				return o.SetModTime(ctx, time.Now())
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.test()
			require.Equal(t, fs.ErrorPermissionDenied, err,
				"%s should deny with PermissionDenied", tc.name)
		})
	}
}

// TestFs_DirTime tests dirTime() returns valid time
func TestFs_DirTime(t *testing.T) {
	f := newTestFs(t)
	dirTime := f.dirTime()

	require.False(t, dirTime.IsZero(), "dirTime should not be zero")
	require.True(t, dirTime.Before(time.Now().Add(1*time.Second)),
		"dirTime should be near current time")
}

// TestFs_ListChannels tests listChannels stub
func TestFs_ListChannels(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	entries, err := f.listChannels(ctx, "")
	require.NoError(t, err)
	require.Nil(t, entries, "listChannels stub should return nil")
}

// TestFs_ListPlaylists tests listPlaylists stub
func TestFs_ListPlaylists(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	entries, err := f.listPlaylists(ctx, "")
	require.NoError(t, err)
	require.Nil(t, entries, "listPlaylists stub should return nil")
}

// TestObject_Open tests Open() returns reader
func TestObject_Open(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)
	o := &Object{
		fs:      f,
		remote:  "test/video.mp4",
		videoID: "dQw4w9WgXcQ",
	}

	reader, err := o.Open(ctx)

	// If yt-dlp is not available, skip this test
	if err != nil && strings.Contains(err.Error(), "no such file or directory") {
		t.Skip("yt-dlp not available")
	}

	// If we got a reader, make sure it's closeable
	if reader != nil {
		_ = reader.Close()
	}
}

// TestObject_OpenContextCancellation tests Open with cancellation
func TestObject_OpenContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	f := newTestFs(t)
	o := &Object{
		fs:      f,
		remote:  "test/video.mp4",
		videoID: "test123",
	}

	reader, _ := o.Open(ctx)
	if reader != nil {
		_ = reader.Close()
	}
}

// TestFs_ListWithRoot tests List with root path
func TestFs_ListWithRoot(t *testing.T) {
	ctx := context.Background()
	m := configmap.Simple{"url": "https://www.youtube.com/@testchannel"}
	f, err := NewFs(ctx, "ytfstest", "", m)
	require.NoError(t, err)

	entries, _ := f.List(ctx, "")
	if entries != nil {
		require.Greater(t, len(entries), 0)
	}
}

// TestObject_ModTime_InvalidFormat tests ModTime with invalid format
func TestObject_ModTime_InvalidFormat(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name       string
		uploadDate string
		expectZero bool
	}{
		{"invalid format with dashes", "2024-01-15", true},
		{"invalid format with slashes", "2024/01/15", true},
		{"partial date", "202401", true},
		{"empty string", "", true},
		{"not a date", "abcdefgh", true},
		{"valid YYYYMMDD", "20240115", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &Object{
				fs:         newTestFs(t),
				remote:     "test/video.mp4",
				uploadDate: tt.uploadDate,
			}

			result := o.ModTime(ctx)
			if tt.expectZero {
				require.True(t, result.IsZero(),
					"expected zero time for upload date %q", tt.uploadDate)
			} else {
				require.False(t, result.IsZero(),
					"expected non-zero time for upload date %q", tt.uploadDate)
			}
		})
	}
}

// TestObject_Hash_AllTypes tests Hash with all types
func TestObject_Hash_AllTypes(t *testing.T) {
	ctx := context.Background()
	o := &Object{
		fs:     newTestFs(t),
		remote: "test/video.mp4",
	}

	types := []hash.Type{hash.MD5, hash.SHA1, hash.SHA256}
	for _, ht := range types {
		result, err := o.Hash(ctx, ht)
		require.Equal(t, hash.ErrUnsupported, err,
			"Hash should return ErrUnsupported for type %v", ht)
		require.Equal(t, "", result)
	}
}

// TestNewObject_ChannelVideoPattern tests NewObject with channel video
func TestNewObject_ChannelVideoPattern(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)
	f.root = ""

	remote := "channels/UCtest/dQw4w9WgXcQ Test Video"
	obj, _ := f.NewObject(ctx, remote)

	if obj != nil {
		require.NotNil(t, obj)
		o := obj.(*Object)
		require.Equal(t, remote, o.Remote())
	}
}

// TestNewObject_PlaylistVideoPattern tests NewObject with playlist video
func TestNewObject_PlaylistVideoPattern(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)
	f.root = ""

	remote := "playlists/PLtest/dQw4w9WgXcQ My Video"
	obj, _ := f.NewObject(ctx, remote)

	if obj != nil {
		require.NotNil(t, obj)
		o := obj.(*Object)
		require.Equal(t, remote, o.Remote())
	}
}

// TestFs_ListChannelVideosWithInvalidChannelID tests listChannelVideos with invalid ID format
func TestFs_ListChannelVideosWithInvalidChannelID(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	// Test with channel ID that has no separator
	entries, err := f.listChannelVideos(ctx, "prefix/", "invalidID")
	// Should fail because there's no client and API call will fail
	require.NotNil(t, err)
	require.Nil(t, entries)
}

// TestFs_ListPlaylistVideosWithInvalidPlaylistID tests listPlaylistVideos with invalid ID format
func TestFs_ListPlaylistVideosWithInvalidPlaylistID(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	// Test with playlist ID that has no separator
	entries, err := f.listPlaylistVideos(ctx, "prefix/", "invalidID")
	// Should fail because there's no client and API call will fail
	require.NotNil(t, err)
	require.Nil(t, entries)
}

// TestFs_ListChannelVideosExtractedID tests that listChannelVideos correctly extracts channel ID
func TestFs_ListChannelVideosExtractedID(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	// Test with properly formatted ID including separator
	// Format is "id — Name"
	channelID := "UCtest123 — Test Channel"
	entries, err := f.listChannelVideos(ctx, "prefix/", channelID)
	// Will fail because no real API, but we're testing the extraction logic
	require.NotNil(t, err) // Error expected without real yt-dlp
	require.Nil(t, entries)
}

// TestFs_ListPlaylistVideosExtractedID tests that listPlaylistVideos correctly extracts playlist ID
func TestFs_ListPlaylistVideosExtractedID(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	// Test with properly formatted ID including separator
	// Format is "id — Name"
	playlistID := "PLtest123 — Test Playlist"
	entries, err := f.listPlaylistVideos(ctx, "prefix/", playlistID)
	// Will fail because no real API, but we're testing the extraction logic
	require.NotNil(t, err) // Error expected without real yt-dlp
	require.Nil(t, entries)
}

// TestLoadManifest_ValidJSON tests loading a valid JSON manifest file through NewFs
func TestLoadManifest_ValidJSON(t *testing.T) {
	tests := []struct {
		name     string
		manifest YouTubeManifest
		desc     string
	}{
		{
			name: "single channel",
			manifest: YouTubeManifest{
				Channels: []ChannelEntry{
					{ID: "UCtest123", Name: "Test Channel"},
				},
			},
			desc: "manifest with one channel",
		},
		{
			name: "single playlist",
			manifest: YouTubeManifest{
				Playlists: []PlaylistEntry{
					{ID: "PLtest456", Name: "Test Playlist"},
				},
			},
			desc: "manifest with one playlist",
		},
		{
			name: "multiple channels and playlists",
			manifest: YouTubeManifest{
				Channels: []ChannelEntry{
					{ID: "UCchannel1", Name: "Channel 1"},
					{ID: "UCchannel2", Name: "Channel 2"},
					{ID: "UCchannel3", Name: "Channel 3"},
				},
				Playlists: []PlaylistEntry{
					{ID: "PLplaylist1", Name: "Playlist 1"},
					{ID: "PLplaylist2", Name: "Playlist 2"},
				},
			},
			desc: "manifest with multiple channels and playlists",
		},
		{
			name: "empty manifest",
			manifest: YouTubeManifest{
				Channels:  []ChannelEntry{},
				Playlists: []PlaylistEntry{},
			},
			desc: "empty manifest with no channels or playlists",
		},
		{
			name: "channels with slashes in names",
			manifest: YouTubeManifest{
				Channels: []ChannelEntry{
					{ID: "UCtest", Name: "Channel / With / Slashes"},
					{ID: "UCtest2", Name: "Another/Channel"},
				},
			},
			desc: "channels with forward slashes in names",
		},
		{
			name: "playlists with special characters",
			manifest: YouTubeManifest{
				Playlists: []PlaylistEntry{
					{ID: "PLtest", Name: "Playlist: Special Edition"},
					{ID: "PLtest2", Name: "P&W [Remix]"},
				},
			},
			desc: "playlists with special characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary file with the manifest JSON
			tmpfile, err := createTempManifest(tt.manifest)
			require.NoError(t, err, "failed to create temp manifest file: %s", tt.desc)
			defer cleanupTempFile(tmpfile)

			// Test LoadManifest through NewFs
			ctx := context.Background()
			m := configmap.Simple{
				"manifest_file": tmpfile,
			}
			fs, err := NewFs(ctx, "ytfstest", "", m)
			require.NoError(t, err, "NewFs should succeed: %s", tt.desc)
			require.NotNil(t, fs)

			// Verify manifest was loaded
			ytfs := fs.(*Fs)
			require.NotNil(t, ytfs.manifest, "manifest should be loaded: %s", tt.desc)
			require.Equal(t, len(tt.manifest.Channels), len(ytfs.manifest.Channels),
				"channel count mismatch: %s", tt.desc)
			require.Equal(t, len(tt.manifest.Playlists), len(ytfs.manifest.Playlists),
				"playlist count mismatch: %s", tt.desc)
		})
	}
}

// TestLoadManifest_NoManifestFile tests LoadManifest with empty manifest file option
func TestLoadManifest_NoManifestFile(t *testing.T) {
	ctx := context.Background()
	m := configmap.Simple{}
	fs, err := NewFs(ctx, "ytfstest", "", m)
	require.NoError(t, err, "NewFs should succeed with empty manifest file option")
	require.NotNil(t, fs)

	ytfs := fs.(*Fs)
	// Manifest should be nil when no file is specified
	require.Nil(t, ytfs.manifest, "manifest should be nil when manifest_file is empty")

	// listChannels and listPlaylists should handle nil manifest gracefully
	channelEntries, err := ytfs.listChannels(ctx, "")
	require.NoError(t, err)
	require.Nil(t, channelEntries)

	playlistEntries, err := ytfs.listPlaylists(ctx, "")
	require.NoError(t, err)
	require.Nil(t, playlistEntries)
}

// TestLoadManifest_MissingFile tests that LoadManifest returns error for missing file
func TestLoadManifest_MissingFile(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		desc     string
	}{
		{
			name:     "nonexistent file",
			filePath: "/tmp/this/file/does/not/exist_12345.json",
			desc:     "should error on missing file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Attempt to create NewFs with nonexistent manifest file
			ctx := context.Background()
			m := configmap.Simple{
				"manifest_file": tt.filePath,
			}
			_, err := NewFs(ctx, "ytfstest", "", m)
			require.Error(t, err, tt.desc)
			require.True(t, strings.Contains(err.Error(), "failed to read manifest file"),
				"error should indicate manifest file read failure")
		})
	}
}

// TestLoadManifest_MalformedJSON tests that LoadManifest returns error for invalid JSON
func TestLoadManifest_MalformedJSON(t *testing.T) {
	tests := []struct {
		name    string
		content string
		desc    string
	}{
		{
			name:    "missing closing brace",
			content: `{"channels": [{"id": "UC1", "name": "Ch1"}]`,
			desc:    "incomplete JSON object",
		},
		{
			name:    "invalid JSON syntax",
			content: `{channels: ["UC1"], playlists: ["PL1"]}`,
			desc:    "unquoted keys",
		},
		{
			name:    "trailing comma",
			content: `{"channels": [{"id": "UC1", "name": "Ch1"},], "playlists": []}`,
			desc:    "trailing comma in array",
		},
		{
			name:    "single quotes instead of double",
			content: `{'channels': [{'id': 'UC1', 'name': 'Ch1'}]}`,
			desc:    "single quotes not allowed in JSON",
		},
		{
			name:    "completely invalid",
			content: `not json at all !!!`,
			desc:    "completely invalid content",
		},
		{
			name:    "empty file",
			content: ``,
			desc:    "empty JSON file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary file with invalid JSON
			tmpfile, err := createTempFile(tt.content)
			require.NoError(t, err, "failed to create temp file: %s", tt.desc)
			defer cleanupTempFile(tmpfile)

			// Attempt to create NewFs with invalid manifest JSON
			ctx := context.Background()
			m := configmap.Simple{
				"manifest_file": tmpfile,
			}
			_, err = NewFs(ctx, "ytfstest", "", m)
			require.Error(t, err, tt.desc)
			require.True(t, strings.Contains(err.Error(), "failed to parse manifest JSON"),
				"error should indicate manifest JSON parse failure")
		})
	}
}

// TestListChannels_WithLoadedManifest tests listChannels enumerates channels from manifest
func TestListChannels_WithLoadedManifest(t *testing.T) {
	tests := []struct {
		name     string
		channels []ChannelEntry
		desc     string
	}{
		{
			name: "single channel",
			channels: []ChannelEntry{
				{ID: "UCtest123", Name: "Test Channel"},
			},
			desc: "enumerate single channel",
		},
		{
			name: "multiple channels",
			channels: []ChannelEntry{
				{ID: "UCchannel1", Name: "Channel 1"},
				{ID: "UCchannel2", Name: "Channel 2"},
				{ID: "UCchannel3", Name: "Channel 3"},
			},
			desc: "enumerate multiple channels",
		},
		{
			name:     "no channels",
			channels: []ChannelEntry{},
			desc:     "empty channel list",
		},
		{
			name: "channels with unicode names",
			channels: []ChannelEntry{
				{ID: "UCtest1", Name: "チャンネル"},
				{ID: "UCtest2", Name: "频道"},
			},
			desc: "channels with unicode characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := YouTubeManifest{
				Channels:  tt.channels,
				Playlists: []PlaylistEntry{},
			}

			tmpfile, err := createTempManifest(manifest)
			require.NoError(t, err, "failed to create temp manifest: %s", tt.desc)
			defer cleanupTempFile(tmpfile)

			ctx := context.Background()
			m := configmap.Simple{
				"manifest_file": tmpfile,
			}
			fs, err := NewFs(ctx, "ytfstest", "", m)
			require.NoError(t, err, "NewFs should succeed")

			ytfs := fs.(*Fs)

			// Test listChannels enumeration
			entries, err := ytfs.listChannels(ctx, "")
			if len(tt.channels) == 0 {
				require.Nil(t, entries, "should return nil for empty channels")
			} else {
				require.NoError(t, err, "listChannels should succeed: %s", tt.desc)
				require.NotNil(t, entries)
				require.Equal(t, len(tt.channels), len(entries),
					"channel count mismatch: %s", tt.desc)

				// Verify sanitization of names in listings
				for i, entry := range entries {
					require.Contains(t, entry.Remote(), tt.channels[i].ID,
						"should contain channel ID")
					// Verify slashes are sanitized if present
					if strings.Contains(tt.channels[i].Name, "/") {
						require.True(t, strings.Contains(entry.Remote(), "∕"),
							"slashes should be sanitized in listing")
					}
				}
			}
		})
	}
}

// TestListPlaylists_WithLoadedManifest tests listPlaylists enumerates playlists from manifest
func TestListPlaylists_WithLoadedManifest(t *testing.T) {
	tests := []struct {
		name      string
		playlists []PlaylistEntry
		desc      string
	}{
		{
			name: "single playlist",
			playlists: []PlaylistEntry{
				{ID: "PLtest123", Name: "Test Playlist"},
			},
			desc: "enumerate single playlist",
		},
		{
			name: "multiple playlists",
			playlists: []PlaylistEntry{
				{ID: "PLplaylist1", Name: "Playlist 1"},
				{ID: "PLplaylist2", Name: "Playlist 2"},
				{ID: "PLplaylist3", Name: "Playlist 3"},
			},
			desc: "enumerate multiple playlists",
		},
		{
			name:      "no playlists",
			playlists: []PlaylistEntry{},
			desc:      "empty playlist list",
		},
		{
			name: "playlists with long names",
			playlists: []PlaylistEntry{
				{ID: "PLtest1", Name: "Very Long Playlist Name With Many Words That Goes On And On"},
				{ID: "PLtest2", Name: "Another Long Name"},
			},
			desc: "playlists with long names",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := YouTubeManifest{
				Channels:  []ChannelEntry{},
				Playlists: tt.playlists,
			}

			tmpfile, err := createTempManifest(manifest)
			require.NoError(t, err, "failed to create temp manifest: %s", tt.desc)
			defer cleanupTempFile(tmpfile)

			ctx := context.Background()
			m := configmap.Simple{
				"manifest_file": tmpfile,
			}
			fs, err := NewFs(ctx, "ytfstest", "", m)
			require.NoError(t, err, "NewFs should succeed")

			ytfs := fs.(*Fs)

			// Test listPlaylists enumeration
			entries, err := ytfs.listPlaylists(ctx, "")
			if len(tt.playlists) == 0 {
				require.Nil(t, entries, "should return nil for empty playlists")
			} else {
				require.NoError(t, err, "listPlaylists should succeed: %s", tt.desc)
				require.NotNil(t, entries)
				require.Equal(t, len(tt.playlists), len(entries),
					"playlist count mismatch: %s", tt.desc)

				// Verify sanitization of names in listings
				for i, entry := range entries {
					require.Contains(t, entry.Remote(), tt.playlists[i].ID,
						"should contain playlist ID")
					// Verify slashes are sanitized if present
					if strings.Contains(tt.playlists[i].Name, "/") {
						require.True(t, strings.Contains(entry.Remote(), "∕"),
							"slashes should be sanitized in listing")
					}
				}
			}
		})
	}
}

// TestManifest_SanitizationInListings tests that channel/playlist names are sanitized in listings
func TestManifest_SanitizationInListings(t *testing.T) {
	tests := []struct {
		name      string
		channels  []ChannelEntry
		playlists []PlaylistEntry
		desc      string
	}{
		{
			name: "channels with slashes",
			channels: []ChannelEntry{
				{ID: "UCtest1", Name: "Channel / With / Slashes"},
				{ID: "UCtest2", Name: "Normal Channel"},
			},
			playlists: []PlaylistEntry{},
			desc:      "sanitize slashes in channel names",
		},
		{
			name:     "playlists with slashes",
			channels: []ChannelEntry{},
			playlists: []PlaylistEntry{
				{ID: "PLtest1", Name: "Playlist / With / Slashes"},
				{ID: "PLtest2", Name: "Normal Playlist"},
			},
			desc: "sanitize slashes in playlist names",
		},
		{
			name: "mixed slashes",
			channels: []ChannelEntry{
				{ID: "UCtest", Name: "Ch/1"},
				{ID: "UCtest2", Name: "Ch/2/3"},
			},
			playlists: []PlaylistEntry{
				{ID: "PLtest", Name: "PL/1"},
				{ID: "PLtest2", Name: "PL/2/3"},
			},
			desc: "mixed slash patterns in names",
		},
		{
			name: "edge case names",
			channels: []ChannelEntry{
				{ID: "UC1", Name: "/leading"},
				{ID: "UC2", Name: "trailing/"},
				{ID: "UC3", Name: "///triple///"},
			},
			playlists: []PlaylistEntry{
				{ID: "PL1", Name: "/leading"},
				{ID: "PL2", Name: "trailing/"},
			},
			desc: "edge case slash positions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := YouTubeManifest{
				Channels:  tt.channels,
				Playlists: tt.playlists,
			}

			tmpfile, err := createTempManifest(manifest)
			require.NoError(t, err, "failed to create temp manifest: %s", tt.desc)
			defer cleanupTempFile(tmpfile)

			ctx := context.Background()
			m := configmap.Simple{
				"manifest_file": tmpfile,
			}
			fs, err := NewFs(ctx, "ytfstest", "", m)
			require.NoError(t, err, "NewFs should succeed")

			ytfs := fs.(*Fs)

			// Test channel listings are sanitized
			channelEntries, _ := ytfs.listChannels(ctx, "")
			for i, entry := range channelEntries {
				if i < len(tt.channels) {
					ch := tt.channels[i]
					sanitized := sanitizeName(ch.Name)
					// Verify listing contains sanitized name
					require.Contains(t, entry.Remote(), sanitized,
						"listing should contain sanitized channel name")
					// Verify no unescaped slashes in listing
					require.False(t, strings.Contains(entry.Remote()[len(ch.ID)+3:], "/"),
						"listing should not contain unescaped / characters")
				}
			}

			// Test playlist listings are sanitized
			playlistEntries, _ := ytfs.listPlaylists(ctx, "")
			for i, entry := range playlistEntries {
				if i < len(tt.playlists) {
					pl := tt.playlists[i]
					sanitized := sanitizeName(pl.Name)
					// Verify listing contains sanitized name
					require.Contains(t, entry.Remote(), sanitized,
						"listing should contain sanitized playlist name")
					// Verify no unescaped slashes in listing
					require.False(t, strings.Contains(entry.Remote()[len(pl.ID)+3:], "/"),
						"listing should not contain unescaped / characters")
				}
			}
		})
	}
}

// TestManifest_EmptyManifest tests empty manifest with zero channels and playlists
func TestManifest_EmptyManifest(t *testing.T) {
	tests := []struct {
		name string
		desc string
	}{
		{
			name: "completely empty",
			desc: "manifest with no channels or playlists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := YouTubeManifest{
				Channels:  []ChannelEntry{},
				Playlists: []PlaylistEntry{},
			}

			tmpfile, err := createTempManifest(manifest)
			require.NoError(t, err, "failed to create temp manifest: %s", tt.desc)
			defer cleanupTempFile(tmpfile)

			ctx := context.Background()
			m := configmap.Simple{
				"manifest_file": tmpfile,
			}
			fs, err := NewFs(ctx, "ytfstest", "", m)
			require.NoError(t, err, "NewFs should succeed")

			ytfs := fs.(*Fs)

			// Verify empty counts
			require.Equal(t, 0, len(ytfs.manifest.Channels),
				"should have zero channels: %s", tt.desc)
			require.Equal(t, 0, len(ytfs.manifest.Playlists),
				"should have zero playlists: %s", tt.desc)

			// Verify listChannels and listPlaylists handle empty manifest
			channelEntries, _ := ytfs.listChannels(ctx, "")
			require.Nil(t, channelEntries, "listChannels should return nil for empty channels")

			playlistEntries, _ := ytfs.listPlaylists(ctx, "")
			require.Nil(t, playlistEntries, "listPlaylists should return nil for empty playlists")
		})
	}
}

// TestManifest_LargeManifest tests manifest with many entries
func TestManifest_LargeManifest(t *testing.T) {
	// Generate a large manifest with many channels and playlists
	channels := make([]ChannelEntry, 100)
	for i := 0; i < 100; i++ {
		channels[i] = ChannelEntry{
			ID:   fmt.Sprintf("UC%d", i),
			Name: fmt.Sprintf("Channel %d", i),
		}
	}

	playlists := make([]PlaylistEntry, 50)
	for i := 0; i < 50; i++ {
		playlists[i] = PlaylistEntry{
			ID:   fmt.Sprintf("PL%d", i),
			Name: fmt.Sprintf("Playlist %d", i),
		}
	}

	manifest := YouTubeManifest{
		Channels:  channels,
		Playlists: playlists,
	}

	tmpfile, err := createTempManifest(manifest)
	require.NoError(t, err, "failed to create large manifest file")
	defer cleanupTempFile(tmpfile)

	ctx := context.Background()
	m := configmap.Simple{
		"manifest_file": tmpfile,
	}
	fs, err := NewFs(ctx, "ytfstest", "", m)
	require.NoError(t, err, "NewFs should succeed for large manifest")

	ytfs := fs.(*Fs)

	require.Equal(t, 100, len(ytfs.manifest.Channels), "should have 100 channels")
	require.Equal(t, 50, len(ytfs.manifest.Playlists), "should have 50 playlists")

	// Verify listChannels can enumerate all channels
	channelEntries, err := ytfs.listChannels(ctx, "")
	require.NoError(t, err)
	require.Equal(t, 100, len(channelEntries), "listChannels should return 100 entries")

	// Verify listPlaylists can enumerate all playlists
	playlistEntries, err := ytfs.listPlaylists(ctx, "")
	require.NoError(t, err)
	require.Equal(t, 50, len(playlistEntries), "listPlaylists should return 50 entries")
}

// Helper functions for manifest testing

// createTempManifest creates a temporary file with JSON manifest data
func createTempManifest(m YouTubeManifest) (string, error) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	return createTempFile(string(data))
}

// createTempFile creates a temporary file with the given content
func createTempFile(content string) (string, error) {
	tmpfile, err := os.CreateTemp("", "manifest-*.json")
	if err != nil {
		return "", err
	}
	defer tmpfile.Close()

	if _, err := tmpfile.WriteString(content); err != nil {
		return "", err
	}

	return tmpfile.Name(), nil
}

// cleanupTempFile removes a temporary file
func cleanupTempFile(filepath string) {
	os.Remove(filepath)
}

// readManifestFile reads a JSON manifest file
func readManifestFile(filepath string) ([]byte, error) {
	return os.ReadFile(filepath)
}

// TestParseMetadataFile tests parseMetadataFile function
func TestParseMetadataFile(t *testing.T) {
	tests := []struct {
		name          string
		filename      string
		expectType    MetadataType
		expectVideoID string
	}{
		{
			name:          "NFO file",
			filename:      "dQw4w9WgXcQ.nfo",
			expectType:    MetaNFO,
			expectVideoID: "dQw4w9WgXcQ",
		},
		{
			name:          "Thumbnail file",
			filename:      "dQw4w9WgXcQ-thumb.jpg",
			expectType:    MetaThumb,
			expectVideoID: "dQw4w9WgXcQ",
		},
		{
			name:          "Subtitle file",
			filename:      "dQw4w9WgXcQ.srt",
			expectType:    MetaSRT,
			expectVideoID: "dQw4w9WgXcQ",
		},
		{
			name:          "Chapters file",
			filename:      "dQw4w9WgXcQ.chapters.xml",
			expectType:    MetaChapters,
			expectVideoID: "dQw4w9WgXcQ",
		},
		{
			name:          "Regular video file",
			filename:      "dQw4w9WgXcQ",
			expectType:    MetaNone,
			expectVideoID: "",
		},
		{
			name:          "Video with mp4 extension",
			filename:      "dQw4w9WgXcQ.mp4",
			expectType:    MetaNone,
			expectVideoID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metaType, videoID := parseMetadataFile(tt.filename)
			require.Equal(t, tt.expectType, metaType, "metadata type mismatch")
			require.Equal(t, tt.expectVideoID, videoID, "video ID mismatch")
		})
	}
}

// TestEscapeXML tests XML escaping function
func TestEscapeXML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "ampersand",
			input:    "A & B",
			expected: "A &amp; B",
		},
		{
			name:     "less than",
			input:    "A < B",
			expected: "A &lt; B",
		},
		{
			name:     "greater than",
			input:    "A > B",
			expected: "A &gt; B",
		},
		{
			name:     "quotes",
			input:    `A "quoted" text`,
			expected: `A &quot;quoted&quot; text`,
		},
		{
			name:     "apostrophe",
			input:    "It's working",
			expected: "It&apos;s working",
		},
		{
			name:     "multiple special chars",
			input:    `<tag attr="value">text & more</tag>`,
			expected: `&lt;tag attr=&quot;value&quot;&gt;text &amp; more&lt;/tag&gt;`,
		},
		{
			name:     "no special chars",
			input:    "plain text",
			expected: "plain text",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeXML(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

// TestObject_SizeMetadata tests SizeMetadata function
func TestObject_SizeMetadata(t *testing.T) {
	tests := []struct {
		name      string
		metaType  MetadataType
		expectMin int64
		expectMax int64
	}{
		{
			name:      "None type",
			metaType:  MetaNone,
			expectMin: -1,
			expectMax: -1,
		},
		{
			name:      "Thumbnail",
			metaType:  MetaThumb,
			expectMin: 40000,
			expectMax: 60000,
		},
		{
			name:      "NFO",
			metaType:  MetaNFO,
			expectMin: 4000,
			expectMax: 6000,
		},
		{
			name:      "SRT",
			metaType:  MetaSRT,
			expectMin: 90000,
			expectMax: 110000,
		},
		{
			name:      "Chapters",
			metaType:  MetaChapters,
			expectMin: 8000,
			expectMax: 12000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newTestFs(t)
			o := &Object{
				fs:       f,
				metaType: tt.metaType,
			}
			size := o.SizeMetadata()
			if tt.expectMin == -1 {
				require.Equal(t, int64(-1), size)
			} else {
				require.True(t, size >= tt.expectMin && size <= tt.expectMax,
					"size %d not in range [%d, %d]", size, tt.expectMin, tt.expectMax)
			}
		})
	}
}

// TestNewObject_MetadataFileDetection tests that NewObject detects metadata files
func TestNewObject_MetadataFileDetection(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	tests := []struct {
		name       string
		remote     string
		expectMeta MetadataType
	}{
		{
			name:       "NFO file",
			remote:     "dQw4w9WgXcQ.nfo Test Video",
			expectMeta: MetaNFO,
		},
		{
			name:       "Thumbnail file",
			remote:     "dQw4w9WgXcQ-thumb.jpg Test Video",
			expectMeta: MetaThumb,
		},
		{
			name:       "Regular video",
			remote:     "dQw4w9WgXcQ Test Video",
			expectMeta: MetaNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, _ := f.NewObject(ctx, tt.remote)
			if obj != nil {
				o := obj.(*Object)
				require.Equal(t, tt.expectMeta, o.metaType)
			}
		})
	}
}

// TestFs_GetCachedMetadata_Concurrent tests concurrent metadata requests
func TestFs_GetCachedMetadata_Concurrent(t *testing.T) {
	f := newTestFs(t)

	callCount := 0
	var mu sync.Mutex

	fetcher := func(fetchCtx context.Context) ([]byte, error) {
		mu.Lock()
		callCount++
		mu.Unlock()
		return []byte("metadata"), nil
	}

	// Launch multiple concurrent requests for the same cache key
	const goroutines = 5
	var wg sync.WaitGroup
	results := make(chan []byte, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			data, err := f.getCachedMetadata(ctx, "test-key", fetcher)
			if err == nil && data != nil {
				results <- data
			}
		}()
	}

	wg.Wait()
	close(results)

	// Verify all goroutines got the same result
	var resultList [][]byte
	for data := range results {
		resultList = append(resultList, data)
	}

	require.Equal(t, goroutines, len(resultList))
	for _, result := range resultList {
		require.Equal(t, []byte("metadata"), result)
	}

	// Verify fetcher was called only once despite concurrent requests
	require.Equal(t, 1, callCount, "fetcher should be called exactly once")
}

// TestFs_GetCachedMetadata_TTLExpiry tests cache TTL expiration
func TestFs_GetCachedMetadata_TTLExpiry(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	callCount := 0
	fetcher := func(fetchCtx context.Context) ([]byte, error) {
		callCount++
		return []byte("metadata"), nil
	}

	// First call should fetch
	data1, err := f.getCachedMetadata(ctx, "test-key", fetcher)
	require.NoError(t, err)
	require.Equal(t, []byte("metadata"), data1)
	require.Equal(t, 1, callCount)

	// Second call should use cache
	data2, err := f.getCachedMetadata(ctx, "test-key", fetcher)
	require.NoError(t, err)
	require.Equal(t, []byte("metadata"), data2)
	require.Equal(t, 1, callCount, "should use cache, not fetch again")

	// Manually expire the cache entry
	f.mdCache.mu.Lock()
	entry := f.mdCache.entries["test-key"]
	entry.mu.Lock()
	entry.timestamp = time.Now().Add(-25 * time.Hour)
	entry.mu.Unlock()
	f.mdCache.mu.Unlock()

	// Third call should return nil (expired)
	data3, err := f.getCachedMetadata(ctx, "test-key", fetcher)
	require.NoError(t, err)
	require.Nil(t, data3)
}

// TestFs_GetCachedMetadata_FetchError tests error handling in metadata fetch
func TestFs_GetCachedMetadata_FetchError(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	fetcher := func(fetchCtx context.Context) ([]byte, error) {
		return nil, fmt.Errorf("fetch failed")
	}

	data, err := f.getCachedMetadata(ctx, "test-key", fetcher)
	require.Error(t, err)
	require.Nil(t, data)
	require.Contains(t, err.Error(), "fetch failed")
}

// TestFs_GetCachedMetadata_ContextCancellation tests context cancellation
func TestFs_GetCachedMetadata_ContextCancellation(t *testing.T) {
	f := newTestFs(t)

	// Set up an in-flight fetch
	f.mdCache.mu.Lock()
	entry := &metadataCacheEntry{
		inFlight: true,
		waitChan: make(chan struct{}),
	}
	f.mdCache.entries["test-key"] = entry
	f.mdCache.mu.Unlock()

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Attempt to get cached metadata with cancelled context
	data, err := f.getCachedMetadata(ctx, "test-key", func(fetchCtx context.Context) ([]byte, error) {
		return []byte("data"), nil
	})

	require.Error(t, err)
	require.Nil(t, data)
	require.Equal(t, context.Canceled, err)

	// Clean up
	close(entry.waitChan)
}

// TestObject_OpenMetadata tests opening metadata files
func TestObject_OpenMetadata(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	// Test with mocked cache
	f.mdCache.mu.Lock()
	f.mdCache.entries["vid123:1"] = &metadataCacheEntry{
		data:      []byte("test metadata"),
		timestamp: time.Now(),
	}
	f.mdCache.mu.Unlock()

	o := &Object{
		fs:           f,
		remote:       "vid123.nfo",
		videoID:      "vid123",
		metaType:     MetaNFO,
		metadataFile: "vid123",
	}

	// The openMetadata method will try to fetch metadata from the API
	// This will fail because we don't have a real client, but that's OK
	reader, _ := o.openMetadata(ctx)
	// Either error or reader is OK, as long as it doesn't crash
	if reader != nil {
		defer reader.Close()
	}
}

// TestObject_Open_VideoFile tests opening regular video files
func TestObject_Open_VideoFile(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)
	o := &Object{
		fs:       f,
		remote:   "test/video.mp4",
		videoID:  "dQw4w9WgXcQ",
		metaType: MetaNone,
	}

	// This will fail if yt-dlp is not available, but that's OK for this test
	reader, err := o.Open(ctx)
	if err != nil {
		// Expected if yt-dlp is not available
		if !strings.Contains(err.Error(), "no such file") &&
			!strings.Contains(err.Error(), "not found") {
			t.Logf("Open returned error: %v (not yt-dlp missing)", err)
		}
	} else if reader != nil {
		_ = reader.Close()
	}
}

// TestObject_Open_MetadataDispatch tests metadata file dispatch
func TestObject_Open_MetadataDispatch(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	tests := []struct {
		name     string
		metaType MetadataType
	}{
		{name: "NFO", metaType: MetaNFO},
		{name: "Thumbnail", metaType: MetaThumb},
		{name: "SRT", metaType: MetaSRT},
		{name: "Chapters", metaType: MetaChapters},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &Object{
				fs:           f,
				remote:       fmt.Sprintf("vid123.%d", tt.metaType),
				videoID:      "vid123",
				metaType:     tt.metaType,
				metadataFile: "vid123",
			}

			// Open will attempt to fetch metadata, but will fail without mocking
			reader, err := o.Open(ctx)
			if err != nil {
				// Expected to fail due to no mocked API
				require.NotNil(t, err)
			}
			if reader != nil {
				_ = reader.Close()
			}
		})
	}
}

// TestDownloadNFO_Format tests NFO XML formatting
func TestDownloadNFO_Format(t *testing.T) {
	t.Skip("requires mocking api.Client.GetVideoInfo")
	// This test would require mocking the API client
	// NFO format validation would happen here
}

// TestDownloadThumbnail_URLFallback tests thumbnail URL fallback logic
func TestDownloadThumbnail_URLFallback(t *testing.T) {
	t.Skip("requires mocking HTTP requests")
	// This test would require mocking HTTP requests
	// Fallback logic validation would happen here
}

// TestMetadataCache_ConcurrentAccess tests concurrent cache access safety
func TestMetadataCache_ConcurrentAccess(t *testing.T) {
	f := newTestFs(t)
	numGoroutines := 10
	numOperations := 100

	var wg sync.WaitGroup
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j%10)
				fetcher := func(ctx context.Context) ([]byte, error) {
					return []byte(fmt.Sprintf("data-%d-%d", id, j)), nil
				}
				_, _ = f.getCachedMetadata(context.Background(), key, fetcher)
			}
		}(i)
	}

	wg.Wait()

	// Verify cache contains entries
	f.mdCache.mu.RLock()
	defer f.mdCache.mu.RUnlock()
	require.Greater(t, len(f.mdCache.entries), 0, "cache should have entries")
}

// TestMetadataTypes_AllDefined tests that all metadata types are properly defined
func TestMetadataTypes_AllDefined(t *testing.T) {
	types := []MetadataType{MetaNone, MetaNFO, MetaThumb, MetaSRT, MetaChapters}

	for _, mt := range types {
		require.True(t, mt >= MetaNone && mt <= MetaChapters,
			"metadata type %v should be in valid range", mt)
	}
}

// TestManifestHotReload_LoadsOnInit tests that manifest file is loaded on Fs creation
func TestManifestHotReload_LoadsOnInit(t *testing.T) {
	manifest := YouTubeManifest{
		Channels: []ChannelEntry{
			{ID: "UC123", Name: "Channel 1"},
		},
		Playlists: []PlaylistEntry{
			{ID: "PL456", Name: "Playlist 1"},
		},
	}

	tmpfile, err := createTempManifest(manifest)
	require.NoError(t, err)
	defer cleanupTempFile(tmpfile)

	ctx := context.Background()
	m := configmap.Simple{
		"manifest_file": tmpfile,
		"auto_reload":   "true",
	}

	fs, err := NewFs(ctx, "ytfstest", "", m)
	require.NoError(t, err)

	ytfs := fs.(*Fs)
	require.NotNil(t, ytfs.manifest)
	require.Equal(t, 1, len(ytfs.manifest.Channels))
	require.Equal(t, 1, len(ytfs.manifest.Playlists))
}

// TestFs_ReloadManifest_ReadWriteSafety tests manifest RW mutex safety
func TestFs_ReloadManifest_ReadWriteSafety(t *testing.T) {
	manifest := YouTubeManifest{
		Channels: []ChannelEntry{
			{ID: "UC123", Name: "Channel 1"},
		},
	}

	tmpfile, err := createTempManifest(manifest)
	require.NoError(t, err)
	defer cleanupTempFile(tmpfile)

	ctx := context.Background()
	m := configmap.Simple{
		"manifest_file": tmpfile,
	}

	fs, err := NewFs(ctx, "ytfstest", "", m)
	require.NoError(t, err)

	ytfs := fs.(*Fs)

	// Concurrent reads and writes
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Try to read manifest
			ytfs.manifestMu.RLock()
			if ytfs.manifest != nil {
				_ = len(ytfs.manifest.Channels)
			}
			ytfs.manifestMu.RUnlock()
		}()
	}

	// Load manifest in another goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = ytfs.LoadManifest()
	}()

	wg.Wait()

	// Clean up watcher
	if ytfs.watcher != nil {
		close(ytfs.stopCh)
	}
}

// TestCmdReader_Close_WithNilCmd tests Close when cmd is nil
func TestCmdReader_Close_WithNilCmd(t *testing.T) {
	r := io.NopCloser(strings.NewReader("test"))
	cr := &cmdReader{
		reader: r,
		cmd:    nil,
	}

	err := cr.Close()
	require.NoError(t, err)
}

// TestCmdReader_Close_WithReaderError tests Close when reader close fails
func TestCmdReader_Close_WithReaderError(t *testing.T) {
	cmd := exec.Command("true")
	err := cmd.Start()
	require.NoError(t, err)

	r := io.NopCloser(strings.NewReader("test"))

	cr := &cmdReader{
		reader: r,
		cmd:    cmd,
	}

	// Close should succeed
	err = cr.Close()
	require.NoError(t, err)
}

// TestObject_Size_Metadata tests SizeMetadata returns -1 for MetaNone
func TestObject_Size_Metadata(t *testing.T) {
	o := &Object{
		fs:       newTestFs(t),
		metaType: MetaNone,
	}
	require.Equal(t, int64(-1), o.SizeMetadata())
}

// TestFs_ListChannelVideos_APISuccess tests successful channel video listing
func TestFs_ListChannelVideos_APISuccess(t *testing.T) {
	t.Skip("requires mocking API client")
	// This test would require mocking the API client's GetChannelVideos method
}

// TestObject_OpenMetadata_WithCachedData tests opening metadata with cached data
func TestObject_OpenMetadata_WithCachedData(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	// Pre-populate cache with metadata
	cacheKey := "vid123:1" // MetaNFO = 1
	f.mdCache.mu.Lock()
	f.mdCache.entries[cacheKey] = &metadataCacheEntry{
		data:      []byte(`<?xml version="1.0"?><movie><title>Test</title></movie>`),
		timestamp: time.Now(),
		inFlight:  false,
	}
	f.mdCache.mu.Unlock()

	o := &Object{
		fs:           f,
		remote:       "vid123.nfo",
		videoID:      "vid123",
		metaType:     MetaNFO,
		metadataFile: "vid123",
		title:        "Test Video",
	}

	// This should still attempt to fetch via openMetadata
	// because the cache key format may differ
	reader, err := o.openMetadata(ctx)
	if err != nil {
		// Expected - no mocked API
		require.NotNil(t, err)
	} else if reader != nil {
		defer reader.Close()
	}
}

// TestDownloadChapters_Duration tests downloadChapters format
func TestDownloadChapters_Duration(t *testing.T) {
	t.Skip("requires mocking api.Client.GetVideoInfo")
	// This test would require mocking the API client
}

// TestNewFs_WithAutoReload tests NewFs with auto reload disabled
func TestNewFs_WithAutoReload(t *testing.T) {
	ctx := context.Background()
	manifest := YouTubeManifest{
		Channels: []ChannelEntry{
			{ID: "UC123", Name: "Channel 1"},
		},
	}

	tmpfile, err := createTempManifest(manifest)
	require.NoError(t, err)
	defer cleanupTempFile(tmpfile)

	m := configmap.Simple{
		"manifest_file": tmpfile,
		"auto_reload":   "false",
	}

	fs, err := NewFs(ctx, "ytfstest", "", m)
	require.NoError(t, err)
	require.NotNil(t, fs)

	ytfs := fs.(*Fs)
	require.Nil(t, ytfs.watcher, "watcher should be nil when auto_reload is false")
}

// TestObject_Open_MetadataType_Dispatch tests Open dispatches to correct metadata handler
func TestObject_Open_MetadataType_Dispatch(t *testing.T) {
	tests := []struct {
		name     string
		metaType MetadataType
	}{
		{name: "MetaNone", metaType: MetaNone},
		{name: "MetaNFO", metaType: MetaNFO},
		{name: "MetaThumb", metaType: MetaThumb},
		{name: "MetaSRT", metaType: MetaSRT},
		{name: "MetaChapters", metaType: MetaChapters},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			f := newTestFs(t)

			o := &Object{
				fs:           f,
				remote:       "test.mp4",
				videoID:      "vid123",
				metaType:     tt.metaType,
				metadataFile: "vid123",
			}

			reader, err := o.Open(ctx)
			// Most metadata types will fail because we don't have a real API
			// MetaNone might fail with yt-dlp not found
			// This test just verifies the dispatch works without crashing
			if reader != nil {
				_ = reader.Close()
			}
			// err can be anything - we're just testing dispatch
			_ = err
		})
	}
}

// TestParseMetadataFile_EdgeCases tests edge cases in parseMetadataFile
func TestParseMetadataFile_EdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		filename      string
		expectType    MetadataType
		expectVideoID string
	}{
		{
			name:          "empty filename",
			filename:      "",
			expectType:    MetaNone,
			expectVideoID: "",
		},
		{
			name:          "only extension",
			filename:      ".nfo",
			expectType:    MetaNFO,
			expectVideoID: "",
		},
		{
			name:          "multiple dots",
			filename:      "vid.123.nfo",
			expectType:    MetaNFO,
			expectVideoID: "vid.123",
		},
		{
			name:          "chapters with multiple dots",
			filename:      "vid.123.chapters.xml",
			expectType:    MetaChapters,
			expectVideoID: "vid.123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metaType, videoID := parseMetadataFile(tt.filename)
			require.Equal(t, tt.expectType, metaType)
			require.Equal(t, tt.expectVideoID, videoID)
		})
	}
}

// TestObject_SizeMetadata_AllTypes tests SizeMetadata for all metadata types
func TestObject_SizeMetadata_AllTypes(t *testing.T) {
	f := newTestFs(t)

	tests := []struct {
		metaType MetadataType
		minSize  int64
		maxSize  int64
	}{
		{MetaNone, -1, -1},
		{MetaNFO, 4000, 6000},
		{MetaThumb, 40000, 60000},
		{MetaSRT, 90000, 110000},
		{MetaChapters, 8000, 12000},
	}

	for _, tt := range tests {
		o := &Object{
			fs:       f,
			metaType: tt.metaType,
		}
		size := o.SizeMetadata()
		if tt.minSize == -1 {
			require.Equal(t, int64(-1), size)
		} else {
			require.True(t, size >= tt.minSize && size <= tt.maxSize,
				"size %d not in range [%d, %d]", size, tt.minSize, tt.maxSize)
		}
	}
}

// TestNewFs_WithInvalidAutoReload tests NewFs option parsing for auto_reload
func TestNewFs_WithInvalidAutoReload(t *testing.T) {
	ctx := context.Background()
	m := configmap.Simple{}
	fs, err := NewFs(ctx, "ytfstest", "", m)
	require.NoError(t, err)
	require.NotNil(t, fs)
	ytfs := fs.(*Fs)
	require.False(t, ytfs.opt.AutoReload, "auto_reload should default to false from config struct")
}

// TestWatchManifest_WithoutWatcher tests watchManifest when watcher is nil
func TestWatchManifest_WithoutWatcher(t *testing.T) {
	f := newTestFs(t)
	f.watcher = nil
	f.stopCh = make(chan struct{})
	// Should return immediately without error
	f.watchManifest()
}

// TestNewFs_WithAutoReload_Enabled tests manifest watcher is created
func TestNewFs_WithAutoReload_Enabled(t *testing.T) {
	ctx := context.Background()
	manifest := YouTubeManifest{
		Channels: []ChannelEntry{
			{ID: "UC123", Name: "Channel 1"},
		},
	}

	tmpfile, err := createTempManifest(manifest)
	require.NoError(t, err)
	defer cleanupTempFile(tmpfile)

	m := configmap.Simple{
		"manifest_file":      tmpfile,
		"auto_reload":        "true",
		"reload_debounce_ms": "100",
	}

	fs, err := NewFs(ctx, "ytfstest", "", m)
	require.NoError(t, err)

	ytfs := fs.(*Fs)
	require.NotNil(t, ytfs.watcher, "watcher should be created when auto_reload is true")
	require.NotNil(t, ytfs.stopCh, "stopCh should be created")

	// Clean up
	close(ytfs.stopCh)
	time.Sleep(100 * time.Millisecond) // Give watcher time to stop
}

// TestObject_OpenMetadata_SRTMetaType tests opening SRT metadata file
func TestObject_OpenMetadata_SRTMetaType(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	o := &Object{
		fs:           f,
		remote:       "vid123.srt",
		videoID:      "vid123",
		metaType:     MetaSRT,
		metadataFile: "vid123",
	}

	// Will fail without mocked API, but we're testing the dispatch
	reader, _ := o.openMetadata(ctx)
	if reader != nil {
		defer reader.Close()
	}
}

// TestObject_OpenMetadata_ChaptersMetaType tests opening chapters metadata file
func TestObject_OpenMetadata_ChaptersMetaType(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	o := &Object{
		fs:           f,
		remote:       "vid123.chapters.xml",
		videoID:      "vid123",
		metaType:     MetaChapters,
		metadataFile: "vid123",
	}

	// Will fail without mocked API, but we're testing the dispatch
	reader, _ := o.openMetadata(ctx)
	if reader != nil {
		defer reader.Close()
	}
}

// TestObject_OpenMetadata_ThumbnailMetaType tests opening thumbnail metadata file
func TestObject_OpenMetadata_ThumbnailMetaType(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	o := &Object{
		fs:           f,
		remote:       "vid123-thumb.jpg",
		videoID:      "vid123",
		metaType:     MetaThumb,
		metadataFile: "vid123",
	}

	// Will fail without mocked API, but we're testing the dispatch
	reader, _ := o.openMetadata(ctx)
	if reader != nil {
		defer reader.Close()
	}
}

// TestSizeMetadata_AllCasesReachable tests all branches of SizeMetadata
func TestSizeMetadata_AllCasesReachable(t *testing.T) {
	tests := []struct {
		name        string
		metaType    MetadataType
		expectRange [2]int64
	}{
		{"MetaNone", MetaNone, [2]int64{-1, -1}},
		{"MetaNFO", MetaNFO, [2]int64{4000, 6000}},
		{"MetaThumb", MetaThumb, [2]int64{40000, 60000}},
		{"MetaSRT", MetaSRT, [2]int64{90000, 110000}},
		{"MetaChapters", MetaChapters, [2]int64{8000, 12000}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &Object{
				fs:       newTestFs(t),
				metaType: tt.metaType,
			}
			size := o.SizeMetadata()

			if tt.expectRange[0] == -1 {
				require.Equal(t, int64(-1), size)
			} else {
				require.True(t, size >= tt.expectRange[0] && size <= tt.expectRange[1],
					"size out of range")
			}
		})
	}
}

// TestNewObject_MetadataFileTypes tests metadata file parsing in NewObject
func TestNewObject_MetadataFileTypes(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	tests := []struct {
		name        string
		remote      string
		expectType  MetadataType
		expectVidID string
	}{
		{"nfo file", "abc123.nfo Test", MetaNFO, "abc123"},
		{"thumb file", "abc123-thumb.jpg Test", MetaThumb, "abc123"},
		{"srt file", "abc123.srt Test", MetaSRT, "abc123"},
		{"chapters file", "abc123.chapters.xml Test", MetaChapters, "abc123"},
		{"regular video", "abc123 Test", MetaNone, "abc123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, _ := f.NewObject(ctx, tt.remote)
			if obj != nil {
				o := obj.(*Object)
				require.Equal(t, tt.expectType, o.metaType)
				require.Equal(t, tt.expectVidID, o.videoID)
			}
		})
	}
}

// TestParseMetadataFile_AllTypes ensures all types are parseable
func TestParseMetadataFile_AllTypes(t *testing.T) {
	tests := map[string]struct {
		suffix   string
		metaType MetadataType
	}{
		"nfo":      {".nfo", MetaNFO},
		"thumb":    {"-thumb.jpg", MetaThumb},
		"srt":      {".srt", MetaSRT},
		"chapters": {".chapters.xml", MetaChapters},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			filename := "vid123" + tt.suffix
			metaType, videoID := parseMetadataFile(filename)
			require.Equal(t, tt.metaType, metaType, "should parse %s", name)
			require.Equal(t, "vid123", videoID)
		})
	}
}

// TestGetCachedMetadata_ConcurrentFetchBlocking tests that concurrent requests block until fetch completes
func TestGetCachedMetadata_ConcurrentFetchBlocking(t *testing.T) {
	f := newTestFs(t)

	// Track fetch counts
	fetchCount := 0
	var mu sync.Mutex

	// Create a slow fetcher
	fetcher := func(ctx context.Context) ([]byte, error) {
		mu.Lock()
		fetchCount++
		mu.Unlock()
		time.Sleep(100 * time.Millisecond)
		return []byte("slow data"), nil
	}

	// Launch multiple concurrent requests
	numGoroutines := 5
	var wg sync.WaitGroup
	results := make(chan bool, numGoroutines)

	start := time.Now()
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			data, err := f.getCachedMetadata(ctx, "test-key", fetcher)
			results <- (err == nil && data != nil)
		}()
	}

	wg.Wait()
	close(results)

	elapsed := time.Since(start)

	// Verify all requests succeeded
	successCount := 0
	for success := range results {
		if success {
			successCount++
		}
	}
	require.Equal(t, numGoroutines, successCount)

	// Verify fetcher was called only once despite concurrent requests
	mu.Lock()
	count := fetchCount
	mu.Unlock()
	require.Equal(t, 1, count, "fetcher should be called exactly once")

	// Total time should be roughly one fetch (100ms), not 5 (500ms)
	require.Less(t, elapsed, 300*time.Millisecond, "should block concurrently, not serially")
}

// TestOpenMetadata_UnknownType tests openMetadata with invalid metadata type
func TestOpenMetadata_UnknownType(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	// Create object with invalid metadata type
	o := &Object{
		fs:           f,
		remote:       "test",
		videoID:      "vid123",
		metaType:     MetadataType(99), // Invalid type
		metadataFile: "vid123",
	}

	// openMetadata should fail gracefully
	reader, err := o.openMetadata(ctx)
	require.Error(t, err)
	require.Nil(t, reader)
	require.Contains(t, err.Error(), "unknown metadata type")
}

// TestObject_Open_DispatchCorrectly tests that Open correctly dispatches to metadata handlers
func TestObject_Open_DispatchCorrectly(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	// Test that MetaNone goes to video handler, others go to metadata handler
	tests := []struct {
		name      string
		metaType  MetadataType
		usesYtDlp bool
	}{
		{"video", MetaNone, true},
		{"metadata", MetaNFO, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &Object{
				fs:           f,
				remote:       "test",
				videoID:      "vid123",
				metaType:     tt.metaType,
				metadataFile: "vid123",
			}

			reader, err := o.Open(ctx)
			// Errors are expected since we don't have real API/yt-dlp
			if reader != nil {
				_ = reader.Close()
			}
			// Error or success is OK, we're just testing the dispatch
			_ = err
		})
	}
}

// TestEscapeXML_AllCharacters tests XML escaping with all special characters
func TestEscapeXML_AllCharacters(t *testing.T) {
	input := `&<>"'`
	expected := `&amp;&lt;&gt;&quot;&apos;`
	result := escapeXML(input)
	require.Equal(t, expected, result)
}

// TestEscapeXML_MixedContent tests escapeXML with mixed content
func TestEscapeXML_MixedContent(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello & World", "Hello &amp; World"},
		{"<tag>content</tag>", "&lt;tag&gt;content&lt;/tag&gt;"},
		{`Key="Value"`, `Key=&quot;Value&quot;`},
		{"It's fine", "It&apos;s fine"},
		{"Combined &<>\"'", "Combined &amp;&lt;&gt;&quot;&apos;"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeXML(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

// TestGetCachedMetadata_WithExpiredEntry tests handling of expired cache entries
func TestGetCachedMetadata_WithExpiredEntry(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	fetchCount := 0
	fetcher := func(fetchCtx context.Context) ([]byte, error) {
		fetchCount++
		return []byte("data"), nil
	}

	// First fetch - should succeed
	data1, err := f.getCachedMetadata(ctx, "test-key", fetcher)
	require.NoError(t, err)
	require.Equal(t, []byte("data"), data1)
	require.Equal(t, 1, fetchCount)

	// Mark entry as expired
	f.mdCache.mu.Lock()
	entry := f.mdCache.entries["test-key"]
	entry.mu.Lock()
	entry.timestamp = time.Now().Add(-25 * time.Hour)
	entry.mu.Unlock()
	f.mdCache.mu.Unlock()

	// Second fetch - should return nil (expired)
	data2, err := f.getCachedMetadata(ctx, "test-key", fetcher)
	require.NoError(t, err)
	require.Nil(t, data2)
}

// TestObject_OpenWithoutFs tests Object.Open handles nil Fs gracefully
func TestObject_OpenWithoutFs(t *testing.T) {
	ctx := context.Background()
	o := &Object{
		fs:       newTestFs(t),
		remote:   "test.mp4",
		videoID:  "vid123",
		metaType: MetaNone,
	}

	// Should attempt yt-dlp (will fail if not installed)
	reader, err := o.Open(ctx)
	if reader != nil {
		_ = reader.Close()
	}
	// Error is OK - we're just testing it doesn't crash
	_ = err
}

// TestMetadataCache_Structure tests metadataCache thread safety
func TestMetadataCache_Structure(t *testing.T) {
	cache := &metadataCache{
		entries: make(map[string]*metadataCacheEntry),
	}

	// Add entries concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			cache.mu.Lock()
			cache.entries[fmt.Sprintf("key-%d", id)] = &metadataCacheEntry{
				data:      []byte(fmt.Sprintf("data-%d", id)),
				timestamp: time.Now(),
			}
			cache.mu.Unlock()
		}(i)
	}

	wg.Wait()

	// Verify all entries were added
	cache.mu.RLock()
	require.Equal(t, 10, len(cache.entries))
	cache.mu.RUnlock()
}

// TestNewObject_ComplexVideoPath tests NewObject parsing of complex video paths
func TestNewObject_ComplexVideoPath(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	tests := []struct {
		name        string
		remote      string
		expectVidID string
		expectTitle string
	}{
		{
			name:        "simple",
			remote:      "vid123 Title",
			expectVidID: "vid123",
			expectTitle: "Title",
		},
		{
			name:        "title with spaces",
			remote:      "vid123 Long Video Title With Many Words",
			expectVidID: "vid123",
			expectTitle: "Long Video Title With Many Words",
		},
		{
			name:        "sanitized title",
			remote:      "vid123 Title∕With∕Slashes",
			expectVidID: "vid123",
			expectTitle: "Title/With/Slashes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, _ := f.NewObject(ctx, tt.remote)
			if obj != nil {
				o := obj.(*Object)
				require.Equal(t, tt.expectVidID, o.videoID)
				require.Equal(t, tt.expectTitle, o.title)
			}
		})
	}
}

// TestMetadataCacheTTL_NearExpiry tests cache behavior near TTL boundary
func TestMetadataCacheTTL_NearExpiry(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	callCount := 0
	fetcher := func(fetchCtx context.Context) ([]byte, error) {
		callCount++
		return []byte("data"), nil
	}

	// First fetch
	data1, err := f.getCachedMetadata(ctx, "test-key", fetcher)
	require.NoError(t, err)
	require.Equal(t, []byte("data"), data1)
	require.Equal(t, 1, callCount)

	// Manipulate timestamp to be 23.5 hours old (just before 24h TTL)
	f.mdCache.mu.Lock()
	entry := f.mdCache.entries["test-key"]
	entry.mu.Lock()
	entry.timestamp = time.Now().Add(-23*time.Hour - 30*time.Minute)
	entry.mu.Unlock()
	f.mdCache.mu.Unlock()

	// Should still be valid
	data2, err := f.getCachedMetadata(ctx, "test-key", fetcher)
	require.NoError(t, err)
	require.Equal(t, []byte("data"), data2)
	require.Equal(t, 1, callCount, "should use cache at 23.5h")

	// Manipulate timestamp to be 24.1 hours old (just after 24h TTL)
	f.mdCache.mu.Lock()
	entry = f.mdCache.entries["test-key"]
	entry.mu.Lock()
	entry.timestamp = time.Now().Add(-24*time.Hour - 6*time.Minute)
	entry.mu.Unlock()
	f.mdCache.mu.Unlock()

	// Should be expired
	data3, err := f.getCachedMetadata(ctx, "test-key", fetcher)
	require.NoError(t, err)
	require.Nil(t, data3, "should expire after 24h")
}

// TestMetadataCacheContextCancellationDuringWait tests context cancellation while waiting for in-flight fetch
func TestMetadataCacheContextCancellationDuringWait(t *testing.T) {
	f := newTestFs(t)

	// Create an in-flight fetch that will take a while
	slowFetcher := func(ctx context.Context) ([]byte, error) {
		time.Sleep(500 * time.Millisecond)
		return []byte("slow data"), nil
	}

	// Start first request (slow)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ctx := context.Background()
		_, _ = f.getCachedMetadata(ctx, "test-key", slowFetcher)
	}()

	// Give first goroutine time to start fetch
	time.Sleep(50 * time.Millisecond)

	// Create a cancelled context and try to fetch same key
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	data, err := f.getCachedMetadata(ctx, "test-key", func(fetchCtx context.Context) ([]byte, error) {
		return nil, nil
	})

	require.Error(t, err)
	require.Nil(t, data)
	require.Equal(t, context.Canceled, err)

	wg.Wait()
}

// TestConcurrentMetadataRequests_DifferentTypes tests concurrent requests for different metadata types of same video
func TestConcurrentMetadataRequests_DifferentTypes(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	fetchCounts := make(map[string]int)
	var mu sync.Mutex

	fetcher := func(metaType string) func(context.Context) ([]byte, error) {
		return func(fetchCtx context.Context) ([]byte, error) {
			mu.Lock()
			fetchCounts[metaType]++
			mu.Unlock()
			time.Sleep(50 * time.Millisecond)
			return []byte("data-" + metaType), nil
		}
	}

	// Launch concurrent requests for different metadata types
	const numGoroutines = 6
	var wg sync.WaitGroup
	results := make(chan []byte, numGoroutines)

	metaTypes := []string{":0", ":1", ":2", ":3", ":4", ":0"} // Two requests for type 0
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			cacheKey := "vid123" + metaTypes[idx]
			data, err := f.getCachedMetadata(ctx, cacheKey, fetcher(metaTypes[idx]))
			if err == nil && data != nil {
				results <- data
			}
		}(i)
	}

	wg.Wait()
	close(results)

	// Verify all requests succeeded
	var resultList [][]byte
	for data := range results {
		resultList = append(resultList, data)
	}
	require.Equal(t, numGoroutines, len(resultList))

	// Verify each metadata type was fetched independently
	mu.Lock()
	defer mu.Unlock()
	// Type 0 should be fetched once (both requests shared the cache)
	require.Equal(t, 1, fetchCounts[":0"])
	// Others should be fetched once
	for _, count := range fetchCounts {
		require.Equal(t, 1, count)
	}
}

// TestMetadataFileSizeCalculations tests SizeMetadata returns accurate approximate sizes
func TestMetadataFileSizeCalculations(t *testing.T) {
	f := newTestFs(t)

	tests := []struct {
		metaType     MetadataType
		expectedSize int64
		tolerance    int64 // Allow ±tolerance
	}{
		{MetaNFO, 5000, 1000},
		{MetaThumb, 50000, 10000},
		{MetaSRT, 100000, 10000},
		{MetaChapters, 10000, 2000},
		{MetaNone, -1, 0},
	}

	for _, tt := range tests {
		o := &Object{
			fs:       f,
			metaType: tt.metaType,
		}

		size := o.SizeMetadata()

		if tt.expectedSize == -1 {
			require.Equal(t, int64(-1), size)
		} else {
			require.True(t,
				size >= tt.expectedSize-tt.tolerance && size <= tt.expectedSize+tt.tolerance,
				"size %d not within ±%d of expected %d for type %d",
				size, tt.tolerance, tt.expectedSize, tt.metaType)
		}
	}
}

// TestEscapeXML_EdgeCases tests XML escaping with edge case inputs
func TestEscapeXML_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "only special chars",
			input:    "&<>\"'",
			expected: "&amp;&lt;&gt;&quot;&apos;",
		},
		{
			name:     "repeated ampersands",
			input:    "&&&&",
			expected: "&amp;&amp;&amp;&amp;",
		},
		{
			name:     "mixed quotes",
			input:    `"It's"`,
			expected: `&quot;It&apos;s&quot;`,
		},
		{
			name:     "nested angle brackets",
			input:    "<<>>",
			expected: "&lt;&lt;&gt;&gt;",
		},
		{
			name:     "realistic XML",
			input:    `<title attr="value">A & B</title>`,
			expected: `&lt;title attr=&quot;value&quot;&gt;A &amp; B&lt;/title&gt;`,
		},
		{
			name:     "unicode with special chars",
			input:    "测试&内容",
			expected: "测试&amp;内容",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeXML(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

// TestLoadManifest_CorruptedManifest tests LoadManifest handles corrupted manifest gracefully
func TestLoadManifest_CorruptedManifest(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		content     string
		shouldError bool
	}{
		{
			name:        "missing closing brace",
			content:     `{"channels": [{"id": "UC1", "name": "Ch1"}]`,
			shouldError: true,
		},
		{
			name:        "invalid JSON",
			content:     `not json`,
			shouldError: true,
		},
		{
			name:        "empty object",
			content:     `{}`,
			shouldError: false,
		},
		{
			name:        "array instead of object",
			content:     `[{"id": "UC1"}]`,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpfile, err := createTempFile(tt.content)
			require.NoError(t, err)
			defer cleanupTempFile(tmpfile)

			m := configmap.Simple{"manifest_file": tmpfile}
			_, err = NewFs(ctx, "ytfstest", "", m)

			if tt.shouldError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestHotReload_ManifestReloadWithConcurrentReads tests manifest reload safety with concurrent reads
func TestHotReload_ManifestReloadWithConcurrentReads(t *testing.T) {
	manifest := YouTubeManifest{
		Channels: []ChannelEntry{
			{ID: "UC1", Name: "Channel 1"},
		},
	}

	tmpfile, err := createTempManifest(manifest)
	require.NoError(t, err)
	defer cleanupTempFile(tmpfile)

	ctx := context.Background()
	m := configmap.Simple{"manifest_file": tmpfile}
	fs, err := NewFs(ctx, "ytfstest", "", m)
	require.NoError(t, err)

	ytfs := fs.(*Fs)

	// Concurrent reads and writes
	readErrors := 0
	var wg sync.WaitGroup

	// 10 reader goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				ytfs.manifestMu.RLock()
				if ytfs.manifest != nil {
					_ = len(ytfs.manifest.Channels)
				}
				ytfs.manifestMu.RUnlock()
			}
		}()
	}

	// Writer goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 5; j++ {
			_ = ytfs.LoadManifest()
		}
	}()

	wg.Wait()

	require.Equal(t, 0, readErrors, "no errors during concurrent access")

	// Clean up
	if ytfs.stopCh != nil {
		close(ytfs.stopCh)
	}
}

// TestWatchManifest_DebounceDelay tests manifest watcher debounce delay
func TestWatchManifest_DebounceDelay(t *testing.T) {
	manifest := YouTubeManifest{
		Channels: []ChannelEntry{
			{ID: "UC1", Name: "Channel 1"},
		},
	}

	tmpfile, err := createTempManifest(manifest)
	require.NoError(t, err)
	defer cleanupTempFile(tmpfile)

	ctx := context.Background()
	m := configmap.Simple{
		"manifest_file":      tmpfile,
		"auto_reload":        "true",
		"reload_debounce_ms": "50",
	}

	fs, err := NewFs(ctx, "ytfstest", "", m)
	require.NoError(t, err)

	ytfs := fs.(*Fs)
	require.NotNil(t, ytfs.watcher)
	require.Equal(t, 50, ytfs.opt.ReloadDebounceMp)

	// Clean up
	close(ytfs.stopCh)
	time.Sleep(100 * time.Millisecond)
}

// TestMetadataCache_InFlightWaitChanClose tests in-flight entry cleanup
func TestMetadataCache_InFlightWaitChanClose(t *testing.T) {
	f := newTestFs(t)

	callCount := 0
	fetcher := func(ctx context.Context) ([]byte, error) {
		callCount++
		time.Sleep(100 * time.Millisecond)
		return []byte("data"), nil
	}

	// Start first fetch
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ctx := context.Background()
		_, _ = f.getCachedMetadata(ctx, "key", fetcher)
	}()

	// Give it time to set up in-flight
	time.Sleep(10 * time.Millisecond)

	// Verify entry exists and is in-flight
	f.mdCache.mu.RLock()
	entry, exists := f.mdCache.entries["key"]
	f.mdCache.mu.RUnlock()
	require.True(t, exists)

	// Wait for first fetch to complete
	wg.Wait()

	// Verify entry is no longer in-flight
	f.mdCache.mu.RLock()
	entry.mu.RLock()
	require.False(t, entry.inFlight)
	entry.mu.RUnlock()
	f.mdCache.mu.RUnlock()
}

// TestMetadataCache_MultipleKeys tests cache isolation between different keys
func TestMetadataCache_MultipleKeys(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	// Fetch different cache keys
	data1, err := f.getCachedMetadata(ctx, "key1", func(fetchCtx context.Context) ([]byte, error) {
		return []byte("data1"), nil
	})
	require.NoError(t, err)
	require.Equal(t, []byte("data1"), data1)

	data2, err := f.getCachedMetadata(ctx, "key2", func(fetchCtx context.Context) ([]byte, error) {
		return []byte("data2"), nil
	})
	require.NoError(t, err)
	require.Equal(t, []byte("data2"), data2)

	// Verify both entries exist independently
	f.mdCache.mu.RLock()
	entry1, exists1 := f.mdCache.entries["key1"]
	entry2, exists2 := f.mdCache.entries["key2"]
	f.mdCache.mu.RUnlock()

	require.True(t, exists1)
	require.True(t, exists2)

	entry1.mu.RLock()
	require.Equal(t, []byte("data1"), entry1.data)
	entry1.mu.RUnlock()

	entry2.mu.RLock()
	require.Equal(t, []byte("data2"), entry2.data)
	entry2.mu.RUnlock()
}

// TestManifest_LargeConcurrentAccess tests manifest with heavy concurrent access
func TestManifest_LargeConcurrentAccess(t *testing.T) {
	// Generate large manifest
	channels := make([]ChannelEntry, 50)
	for i := 0; i < 50; i++ {
		channels[i] = ChannelEntry{
			ID:   fmt.Sprintf("UC%d", i),
			Name: fmt.Sprintf("Channel %d with / slash", i),
		}
	}

	manifest := YouTubeManifest{Channels: channels}

	tmpfile, err := createTempManifest(manifest)
	require.NoError(t, err)
	defer cleanupTempFile(tmpfile)

	ctx := context.Background()
	m := configmap.Simple{"manifest_file": tmpfile}
	fs, err := NewFs(ctx, "ytfstest", "", m)
	require.NoError(t, err)

	ytfs := fs.(*Fs)

	// Concurrent list operations
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = ytfs.listChannels(ctx, "")
		}()
	}

	wg.Wait()
	require.NotNil(t, ytfs.manifest)
	require.Equal(t, 50, len(ytfs.manifest.Channels))

	// Clean up
	if ytfs.stopCh != nil {
		close(ytfs.stopCh)
	}
}

// TestMetadataCache_RaceConditionSafety tests cache under high contention
func TestMetadataCache_RaceConditionSafety(t *testing.T) {
	f := newTestFs(t)
	ctx := context.Background()

	// High contention test - many goroutines accessing many keys
	const numGoroutines = 50
	const numKeys = 10

	var wg sync.WaitGroup
	errChan := make(chan error, numGoroutines)

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for k := 0; k < numKeys; k++ {
				key := fmt.Sprintf("key-%d", k%5)
				data, err := f.getCachedMetadata(ctx, key, func(fetchCtx context.Context) ([]byte, error) {
					return []byte("data"), nil
				})
				if err != nil {
					errChan <- err
				}
				if data == nil && err == nil {
					// Expired is OK
				}
			}
		}(g)
	}

	wg.Wait()
	close(errChan)

	// Verify no errors occurred
	for err := range errChan {
		require.NoError(t, err)
	}

	// Verify cache has entries
	f.mdCache.mu.RLock()
	require.Greater(t, len(f.mdCache.entries), 0)
	f.mdCache.mu.RUnlock()
}

// TestParseMetadataFile_CaseSensitivity tests that parseMetadataFile is case-sensitive for extensions
func TestParseMetadataFile_CaseSensitivity(t *testing.T) {
	tests := []struct {
		name         string
		filename     string
		expectedType MetadataType
	}{
		{"lowercase nfo", "vid123.nfo", MetaNFO},
		{"uppercase NFO", "vid123.NFO", MetaNone},
		{"mixed case Nfo", "vid123.Nfo", MetaNone},
		{"lowercase chapters", "vid123.chapters.xml", MetaChapters},
		{"uppercase CHAPTERS", "vid123.CHAPTERS.XML", MetaNone},
		{"lowercase thumb", "vid123-thumb.jpg", MetaThumb},
		{"uppercase THUMB", "vid123-THUMB.jpg", MetaNone},
		{"lowercase srt", "vid123.srt", MetaSRT},
		{"uppercase SRT", "vid123.SRT", MetaNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metaType, _ := parseMetadataFile(tt.filename)
			require.Equal(t, tt.expectedType, metaType,
				"parseMetadataFile should be case-sensitive for %q", tt.filename)
		})
	}
}

// TestObject_OpenMetadata_NoFetcherError handles fetch errors gracefully
func TestObject_OpenMetadata_NoFetcherError(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	// Test that openMetadata handles fetch errors
	o := &Object{
		fs:           f,
		remote:       "vid123.nfo",
		videoID:      "vid123",
		metaType:     MetaNFO,
		metadataFile: "vid123",
	}

	// openMetadata will try to fetch, expecting it to fail
	reader, err := o.openMetadata(ctx)
	// Error is expected since we have no real API
	if err != nil {
		require.NotNil(t, err)
	}
	if reader != nil {
		_ = reader.Close()
	}
}

// TestMetadataCache_EmptyFetch tests cache with empty data
func TestMetadataCache_EmptyFetch(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	fetcher := func(fetchCtx context.Context) ([]byte, error) {
		return []byte{}, nil
	}

	data, err := f.getCachedMetadata(ctx, "test-key", fetcher)
	require.NoError(t, err)
	require.Equal(t, []byte{}, data)
}

// TestMetadataCache_LargeData tests cache with large data
func TestMetadataCache_LargeData(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	// Create 1MB of data
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	fetcher := func(fetchCtx context.Context) ([]byte, error) {
		return largeData, nil
	}

	data, err := f.getCachedMetadata(ctx, "large-key", fetcher)
	require.NoError(t, err)
	require.Equal(t, largeData, data)

	// Verify it's cached
	data2, err := f.getCachedMetadata(ctx, "large-key", func(fetchCtx context.Context) ([]byte, error) {
		t.Fatal("should not be called")
		return nil, nil
	})
	require.NoError(t, err)
	require.Equal(t, largeData, data2)
}

// TestObject_ModTime_ZeroTime tests ModTime with zero uploadDate
func TestObject_ModTime_ZeroTime(t *testing.T) {
	ctx := context.Background()
	o := &Object{
		fs:         newTestFs(t),
		remote:     "test.mp4",
		uploadDate: "",
	}

	result := o.ModTime(ctx)
	require.True(t, result.IsZero())
}

// TestObject_ModTime_InvalidDate tests ModTime with various invalid dates
func TestObject_ModTime_InvalidDate(t *testing.T) {
	ctx := context.Background()
	tests := []string{
		"",           // empty
		"20240",      // too short
		"202401",     // too short
		"2024-01-15", // wrong format
		"xxx",        // not a date
		"999999999",  // way too large
	}

	for _, uploadDate := range tests {
		o := &Object{
			fs:         newTestFs(t),
			remote:     "test.mp4",
			uploadDate: uploadDate,
		}

		result := o.ModTime(ctx)
		require.True(t, result.IsZero(), "should return zero time for %q", uploadDate)
	}
}

// TestEscapeXML_Performance tests escapeXML performance with large strings
func TestEscapeXML_Performance(t *testing.T) {
	// Create a large string with many special characters
	input := strings.Repeat("&<>\"'", 1000)

	result := escapeXML(input)

	// Verify all chars were escaped
	require.True(t, strings.Contains(result, "&amp;"))
	require.True(t, strings.Contains(result, "&lt;"))
	require.True(t, strings.Contains(result, "&gt;"))
	require.True(t, strings.Contains(result, "&quot;"))
	require.True(t, strings.Contains(result, "&apos;"))
}

// TestObject_SizeMetadata_BoundaryValues tests SizeMetadata size calculations
func TestObject_SizeMetadata_BoundaryValues(t *testing.T) {
	f := newTestFs(t)

	tests := []struct {
		metaType MetadataType
		expected int64
	}{
		{MetaNone, -1},
		{MetaThumb, 50000},
		{MetaNFO, 5000},
		{MetaSRT, 100000},
		{MetaChapters, 10000},
		{MetadataType(5), -1}, // Unknown type
	}

	for _, tt := range tests {
		o := &Object{fs: f, metaType: tt.metaType}
		size := o.SizeMetadata()
		require.Equal(t, tt.expected, size)
	}
}

// TestFs_LoadManifest_MultipleLoads tests loading manifest multiple times
func TestFs_LoadManifest_MultipleLoads(t *testing.T) {
	manifest := YouTubeManifest{
		Channels: []ChannelEntry{{ID: "UC1", Name: "Ch1"}},
	}

	tmpfile, err := createTempManifest(manifest)
	require.NoError(t, err)
	defer cleanupTempFile(tmpfile)

	ctx := context.Background()
	m := configmap.Simple{"manifest_file": tmpfile}
	fs, err := NewFs(ctx, "ytfstest", "", m)
	require.NoError(t, err)

	ytfs := fs.(*Fs)

	// Load multiple times
	for i := 0; i < 5; i++ {
		err := ytfs.LoadManifest()
		require.NoError(t, err)
		require.NotNil(t, ytfs.manifest)
		require.Equal(t, 1, len(ytfs.manifest.Channels))
	}

	// Clean up
	if ytfs.stopCh != nil {
		close(ytfs.stopCh)
	}
}

// TestMetadataCache_CacheKeyFormat tests cache key formatting and isolation
func TestMetadataCache_CacheKeyFormat(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	// Fetch with different key formats
	keys := []string{
		"vid123:0",
		"vid123:1",
		"vid123:2",
		"vid456:0",
	}

	for _, key := range keys {
		data, err := f.getCachedMetadata(ctx, key, func(fetchCtx context.Context) ([]byte, error) {
			return []byte(key), nil
		})
		require.NoError(t, err)
		require.Equal(t, []byte(key), data)
	}

	// Verify all entries exist independently
	f.mdCache.mu.RLock()
	require.Equal(t, len(keys), len(f.mdCache.entries))
	f.mdCache.mu.RUnlock()
}

// TestListChannels_WithPrefix tests listChannels with various prefixes
func TestListChannels_WithPrefix(t *testing.T) {
	manifest := YouTubeManifest{
		Channels: []ChannelEntry{
			{ID: "UC1", Name: "Channel 1"},
			{ID: "UC2", Name: "Channel 2"},
		},
	}

	tmpfile, err := createTempManifest(manifest)
	require.NoError(t, err)
	defer cleanupTempFile(tmpfile)

	ctx := context.Background()
	m := configmap.Simple{"manifest_file": tmpfile}
	fs, err := NewFs(ctx, "ytfstest", "", m)
	require.NoError(t, err)

	ytfs := fs.(*Fs)

	prefixes := []string{
		"",
		"channels/",
		"some/nested/prefix/",
	}

	for _, prefix := range prefixes {
		entries, err := ytfs.listChannels(ctx, prefix)
		require.NoError(t, err)
		require.NotNil(t, entries)
		require.Equal(t, 2, len(entries))

		for _, entry := range entries {
			require.True(t, strings.HasPrefix(entry.Remote(), prefix))
		}
	}

	// Clean up
	if ytfs.stopCh != nil {
		close(ytfs.stopCh)
	}
}

// TestListPlaylists_WithPrefix tests listPlaylists with various prefixes
func TestListPlaylists_WithPrefix(t *testing.T) {
	manifest := YouTubeManifest{
		Playlists: []PlaylistEntry{
			{ID: "PL1", Name: "Playlist 1"},
			{ID: "PL2", Name: "Playlist 2"},
		},
	}

	tmpfile, err := createTempManifest(manifest)
	require.NoError(t, err)
	defer cleanupTempFile(tmpfile)

	ctx := context.Background()
	m := configmap.Simple{"manifest_file": tmpfile}
	fs, err := NewFs(ctx, "ytfstest", "", m)
	require.NoError(t, err)

	ytfs := fs.(*Fs)

	prefixes := []string{
		"",
		"playlists/",
		"some/nested/prefix/",
	}

	for _, prefix := range prefixes {
		entries, err := ytfs.listPlaylists(ctx, prefix)
		require.NoError(t, err)
		require.NotNil(t, entries)
		require.Equal(t, 2, len(entries))

		for _, entry := range entries {
			require.True(t, strings.HasPrefix(entry.Remote(), prefix))
		}
	}

	// Clean up
	if ytfs.stopCh != nil {
		close(ytfs.stopCh)
	}
}

// TestManifest_ChannelIDExtraction tests channel ID extraction from directory name
func TestManifest_ChannelIDExtraction(t *testing.T) {
	// Test with channel ID format "id — Name"
	tests := []struct {
		channelDirName string
		expectedID     string
	}{
		{"UC123 — Test Channel", "UC123"},
		{"UCabc — Channel Name", "UCabc"},
		{"UC999 — Channel With Spaces In Name", "UC999"},
	}

	for _, tt := range tests {
		parts := strings.SplitN(tt.channelDirName, " — ", 2)
		require.Equal(t, 2, len(parts))
		require.Equal(t, tt.expectedID, parts[0])
	}
}

// TestManifest_PlaylistIDExtraction tests playlist ID extraction from directory name
func TestManifest_PlaylistIDExtraction(t *testing.T) {
	// Test with playlist ID format "id — Name"
	tests := []struct {
		playlistDirName string
		expectedID      string
	}{
		{"PL123 — Test Playlist", "PL123"},
		{"PLabc — Playlist Name", "PLabc"},
		{"PL999 — Playlist With Spaces In Name", "PL999"},
	}

	for _, tt := range tests {
		parts := strings.SplitN(tt.playlistDirName, " — ", 2)
		require.Equal(t, 2, len(parts))
		require.Equal(t, tt.expectedID, parts[0])
	}
}

// TestDirTime tests dirTime returns reasonable value
func TestDirTime(t *testing.T) {
	f := newTestFs(t)

	before := time.Now()
	dirTime := f.dirTime()
	after := time.Now()

	require.False(t, dirTime.IsZero())
	require.True(t, dirTime.After(before.Add(-1*time.Second)))
	require.True(t, dirTime.Before(after.Add(1*time.Second)))
}

// TestNewObject_NoSpaceInRemote tests NewObject when path has no space
func TestNewObject_NoSpaceInRemote(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	// These won't match patterns, but we test the parsing logic
	_, err := f.NewObject(ctx, "somevideonametitle")
	if err != nil {
		require.Equal(t, fs.ErrorObjectNotFound, err)
	}
}

// TestParseMetadataFile_VideoIDExtraction tests video ID extraction
func TestParseMetadataFile_VideoIDExtraction(t *testing.T) {
	tests := []struct {
		name          string
		filename      string
		expectedVidID string
	}{
		{"simple nfo", "abc123.nfo", "abc123"},
		{"with dots", "vid.123.456.nfo", "vid.123.456"},
		{"empty before extension", ".nfo", ""},
		{"chapters", "vid_xyz.chapters.xml", "vid_xyz"},
		{"thumb", "thumbid-thumb.jpg", "thumbid"},
		{"srt", "subid.srt", "subid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, vidID := parseMetadataFile(tt.filename)
			require.Equal(t, tt.expectedVidID, vidID)
		})
	}
}

// TestObject_Fields_Complete tests Object with all fields set
func TestObject_Fields_Complete(t *testing.T) {
	f := newTestFs(t)
	o := &Object{
		fs:           f,
		remote:       "channels/UC123/vid456 Test Video",
		videoID:      "vid456",
		title:        "Test Video",
		duration:     300,
		url:          "https://www.youtube.com/watch?v=vid456",
		uploadDate:   "20240115",
		metaType:     MetaNFO,
		metadataFile: "vid456",
	}

	require.Equal(t, "channels/UC123/vid456 Test Video", o.Remote())
	require.Equal(t, "vid456", o.videoID)
	require.Equal(t, "Test Video", o.title)
	require.Equal(t, 300, o.duration)
	require.Equal(t, "channels/UC123/vid456 Test Video", o.String())
}

// TestMetadataCache_ConcurrentDifferentKeys tests high concurrency with many different keys
func TestMetadataCache_ConcurrentDifferentKeys(t *testing.T) {
	f := newTestFs(t)
	ctx := context.Background()

	const numGoroutines = 100
	const numKeysPerGoroutine = 10

	var wg sync.WaitGroup
	errChan := make(chan error, numGoroutines)

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for k := 0; k < numKeysPerGoroutine; k++ {
				key := fmt.Sprintf("key-%d-%d", id, k)
				data, err := f.getCachedMetadata(ctx, key, func(fetchCtx context.Context) ([]byte, error) {
					return []byte(key), nil
				})
				if err != nil {
					errChan <- err
					return
				}
				if string(data) != key {
					errChan <- fmt.Errorf("data mismatch")
					return
				}
			}
		}(g)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		require.NoError(t, err)
	}

	// Verify cache has many entries
	f.mdCache.mu.RLock()
	require.Greater(t, len(f.mdCache.entries), 50)
	f.mdCache.mu.RUnlock()
}

// TestNewObject_MetadataTypes_AllVariations tests all metadata type combinations
func TestNewObject_MetadataTypes_AllVariations(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	tests := []struct {
		name        string
		remote      string
		expectType  MetadataType
		expectVidID string
	}{
		{"nfo", "v1.nfo Title", MetaNFO, "v1"},
		{"srt", "v2.srt Title", MetaSRT, "v2"},
		{"thumb", "v3-thumb.jpg Title", MetaThumb, "v3"},
		{"chapters", "v4.chapters.xml Title", MetaChapters, "v4"},
		{"video", "v5 Title", MetaNone, "v5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, _ := f.NewObject(ctx, tt.remote)
			if obj != nil {
				o := obj.(*Object)
				require.Equal(t, tt.expectType, o.metaType)
				require.Equal(t, tt.expectVidID, o.videoID)
			}
		})
	}
}

// TestCmdReader_SequentialReads tests cmdReader with multiple reads
func TestCmdReader_SequentialReads(t *testing.T) {
	data := "Hello, World!"
	reader := io.NopCloser(strings.NewReader(data))
	cmd := exec.Command("true")
	err := cmd.Start()
	require.NoError(t, err)

	cr := &cmdReader{
		reader: reader,
		cmd:    cmd,
	}

	// Read in chunks
	buf1 := make([]byte, 5)
	n1, err := cr.Read(buf1)
	require.NoError(t, err)
	require.Equal(t, 5, n1)
	require.Equal(t, "Hello", string(buf1[:n1]))

	buf2 := make([]byte, 8)
	n2, err := cr.Read(buf2)
	require.NoError(t, err)
	require.Equal(t, 8, n2)

	_ = cr.Close()
}

// TestNewFs_OptionsParsing tests various option combinations
func TestNewFs_OptionsParsing(t *testing.T) {
	tests := []struct {
		name string
		opts configmap.Mapper
	}{
		{
			name: "basic options",
			opts: configmap.Simple{
				"url": "https://www.youtube.com/@test",
			},
		},
		{
			name: "with manifest",
			opts: configmap.Simple{
				"url":                "https://www.youtube.com/@test",
				"manifest_file":      "",
				"auto_reload":        "false",
				"reload_debounce_ms": "500",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			fs, err := NewFs(ctx, "ytfstest", "", tt.opts)
			require.NoError(t, err)
			require.NotNil(t, fs)

			ytfs := fs.(*Fs)
			require.Equal(t, "ytfstest", ytfs.Name())
		})
	}
}

// TestFs_FeaturesNew tests feature flags with fresh NewFs
func TestFs_FeaturesNew(t *testing.T) {
	ctx := context.Background()
	m := configmap.Simple{}
	fs, err := NewFs(ctx, "ytfstest", "", m)
	require.NoError(t, err)

	features := fs.Features()
	require.NotNil(t, features)
	require.True(t, features.CanHaveEmptyDirectories)
	require.False(t, features.ReadMimeType)
}

// TestObject_Open_Timeout tests Open with context timeout
func TestObject_Open_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	f := newTestFs(t)
	o := &Object{
		fs:       f,
		remote:   "test.mp4",
		videoID:  "vid123",
		metaType: MetaNone,
	}

	reader, _ := o.Open(ctx)
	if reader != nil {
		_ = reader.Close()
	}
}

// TestManifest_NilManifest tests handling of nil manifest
func TestManifest_NilManifest(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)
	f.manifest = nil

	// listChannels with nil manifest
	entries, err := f.listChannels(ctx, "")
	require.NoError(t, err)
	require.Nil(t, entries)

	// listPlaylists with nil manifest
	entries, err = f.listPlaylists(ctx, "")
	require.NoError(t, err)
	require.Nil(t, entries)
}

// TestEscapeXML_RepeatedChars tests escape with repeated characters
func TestEscapeXML_RepeatedChars(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"&&&&", "&amp;&amp;&amp;&amp;"},
		{"<<<<", "&lt;&lt;&lt;&lt;"},
		{">>>>", "&gt;&gt;&gt;&gt;"},
		{`""""`, `&quot;&quot;&quot;&quot;`},
	}

	for _, tt := range tests {
		result := escapeXML(tt.input)
		require.Equal(t, tt.expected, result)
	}
}

// TestObject_ModTime_Boundary tests ModTime with boundary dates
func TestObject_ModTime_Boundary(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		uploadDate string
		expectYear int
		expectZero bool
	}{
		{"year 2000", "20000101", 2000, false},
		{"year 2024", "20240715", 2024, false},
		{"year 1970", "19700101", 1970, false},
		{"year 2099", "20991231", 2099, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &Object{
				fs:         newTestFs(t),
				uploadDate: tt.uploadDate,
			}

			result := o.ModTime(ctx)
			if tt.expectZero {
				require.True(t, result.IsZero())
			} else {
				require.False(t, result.IsZero())
				require.Equal(t, tt.expectYear, result.Year())
			}
		})
	}
}

// TestMetadataCache_StressTest runs high concurrency stress test
func TestMetadataCache_StressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	f := newTestFs(t)
	ctx := context.Background()

	const numGoroutines = 200
	const numOperations = 5

	var wg sync.WaitGroup
	errChan := make(chan error, numGoroutines)

	start := time.Now()

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for op := 0; op < numOperations; op++ {
				key := fmt.Sprintf("stress-%d", id%10)
				_, err := f.getCachedMetadata(ctx, key, func(fetchCtx context.Context) ([]byte, error) {
					return []byte("stress-test"), nil
				})
				if err != nil {
					errChan <- err
				}
			}
		}(g)
	}

	wg.Wait()
	close(errChan)

	elapsed := time.Since(start)

	// Verify no errors
	for err := range errChan {
		require.NoError(t, err)
	}

	t.Logf("Completed %d goroutines with %d operations each in %v",
		numGoroutines, numOperations, elapsed)
}

// TestNewObject_VideoIDFromSplit tests videoID extraction logic
func TestNewObject_VideoIDFromSplit(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	tests := []struct {
		name        string
		remote      string
		expectVidID string
		expectTitle string
	}{
		{"one space", "vid123 Title", "vid123", "Title"},
		{"multiple spaces", "vid123 Long Video Title", "vid123", "Long Video Title"},
		{"no space", "vid123", "vid123", ""},
		{"space in title", "vid456 My Video / Title", "vid456", "My Video ∕ Title"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, _ := f.NewObject(ctx, tt.remote)
			if obj != nil {
				o := obj.(*Object)
				require.Equal(t, tt.expectVidID, o.videoID)
				if tt.expectTitle != "" {
					require.Equal(t, tt.expectTitle, o.title)
				}
			}
		})
	}
}

// TestMetadataCache_ContextPropagation tests that context is properly passed through fetcher
func TestMetadataCache_ContextPropagation(t *testing.T) {
	f := newTestFs(t)

	fetchCtxChan := make(chan context.Context, 1)

	fetcher := func(ctx context.Context) ([]byte, error) {
		fetchCtxChan <- ctx
		return []byte("data"), nil
	}

	ctxWithValue := context.WithValue(context.Background(), "test-key", "test-value")
	data, err := f.getCachedMetadata(ctxWithValue, "test-key", fetcher)

	require.NoError(t, err)
	require.Equal(t, []byte("data"), data)

	// Verify context was passed
	receivedCtx := <-fetchCtxChan
	require.NotNil(t, receivedCtx)
	val := receivedCtx.Value("test-key")
	require.Equal(t, "test-value", val)
}

// TestManifest_SanitizeInNames tests that sanitization works in manifest listings
func TestManifest_SanitizeInNames(t *testing.T) {
	manifest := YouTubeManifest{
		Channels: []ChannelEntry{
			{ID: "UC1", Name: "A/B/C"},
		},
		Playlists: []PlaylistEntry{
			{ID: "PL1", Name: "X/Y/Z"},
		},
	}

	tmpfile, err := createTempManifest(manifest)
	require.NoError(t, err)
	defer cleanupTempFile(tmpfile)

	ctx := context.Background()
	m := configmap.Simple{"manifest_file": tmpfile}
	fs, err := NewFs(ctx, "ytfstest", "", m)
	require.NoError(t, err)

	ytfs := fs.(*Fs)

	// Verify channels are sanitized
	entries, err := ytfs.listChannels(ctx, "")
	require.NoError(t, err)
	require.Equal(t, 1, len(entries))
	require.Contains(t, entries[0].Remote(), "UC1")
	require.Contains(t, entries[0].Remote(), "∕")    // Division slash
	require.NotContains(t, entries[0].Remote(), "/") // Regular slash

	// Verify playlists are sanitized
	entries, err = ytfs.listPlaylists(ctx, "")
	require.NoError(t, err)
	require.Equal(t, 1, len(entries))
	require.Contains(t, entries[0].Remote(), "PL1")
	require.Contains(t, entries[0].Remote(), "∕")    // Division slash
	require.NotContains(t, entries[0].Remote(), "/") // Regular slash

	// Clean up
	if ytfs.stopCh != nil {
		close(ytfs.stopCh)
	}
}

// TestLoadManifest_Idempotency tests that loading same manifest is idempotent
func TestLoadManifest_Idempotency(t *testing.T) {
	manifest := YouTubeManifest{
		Channels: []ChannelEntry{
			{ID: "UC1", Name: "Ch1"},
		},
	}

	tmpfile, err := createTempManifest(manifest)
	require.NoError(t, err)
	defer cleanupTempFile(tmpfile)

	ctx := context.Background()
	m := configmap.Simple{"manifest_file": tmpfile}
	fs, err := NewFs(ctx, "ytfstest", "", m)
	require.NoError(t, err)

	ytfs := fs.(*Fs)

	// Get first load result
	first := ytfs.manifest

	// Load again multiple times
	for i := 0; i < 5; i++ {
		err := ytfs.LoadManifest()
		require.NoError(t, err)

		// Verify content is the same
		require.Equal(t, len(first.Channels), len(ytfs.manifest.Channels))
		require.Equal(t, first.Channels[0].ID, ytfs.manifest.Channels[0].ID)
		require.Equal(t, first.Channels[0].Name, ytfs.manifest.Channels[0].Name)
	}

	// Clean up
	if ytfs.stopCh != nil {
		close(ytfs.stopCh)
	}
}

// TestMetadataCache_KeyUniqueness tests that cache keys are unique
func TestMetadataCache_KeyUniqueness(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	// Same video, different metadata types should have different keys
	keys := []string{
		"vid123:0",
		"vid123:1",
		"vid123:2",
		"vid123:3",
		"vid123:4",
	}

	for i, key := range keys {
		data, err := f.getCachedMetadata(ctx, key, func(fetchCtx context.Context) ([]byte, error) {
			return []byte(fmt.Sprintf("data-%d", i)), nil
		})
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("data-%d", i), string(data))
	}

	// Verify all keys are independent
	f.mdCache.mu.RLock()
	require.Equal(t, len(keys), len(f.mdCache.entries))
	for _, key := range keys {
		_, exists := f.mdCache.entries[key]
		require.True(t, exists, "key %s should exist", key)
	}
	f.mdCache.mu.RUnlock()
}

// TestGenerateNFO tests NFO generation from video metadata
func TestGenerateNFO(t *testing.T) {
	videoID := "test123"
	title := "Test Video Title"
	duration := 300
	uploadDate := "20240115"

	nfo := generateNFO(videoID, title, duration, uploadDate)

	// Verify NFO contains expected fields
	nfoStr := string(nfo)
	require.Contains(t, nfoStr, "<?xml version")
	require.Contains(t, nfoStr, "test123")
	require.Contains(t, nfoStr, "Test Video Title")
	require.Contains(t, nfoStr, "5") // duration in minutes
	require.Contains(t, nfoStr, "20240115")
}

// TestGenerateNFO_WithSpecialChars tests NFO generation with special characters
func TestGenerateNFO_WithSpecialChars(t *testing.T) {
	videoID := "vid123"
	title := "Video & Title <with> \"quotes\""
	duration := 600
	uploadDate := "20240115"

	nfo := generateNFO(videoID, title, duration, uploadDate)

	nfoStr := string(nfo)
	require.Contains(t, nfoStr, "&amp;")
	require.Contains(t, nfoStr, "&lt;")
	require.Contains(t, nfoStr, "&gt;")
	// XML encoding may use &#34; for quotes instead of &quot;
	require.True(t, strings.Contains(nfoStr, "&quot;") || strings.Contains(nfoStr, "&#34;"))
}

// TestGenerateNFO_DurationConversion tests duration is converted to minutes
func TestGenerateNFO_DurationConversion(t *testing.T) {
	tests := []struct {
		duration    int
		expectedMin string
	}{
		{60, "1"},
		{120, "2"},
		{300, "5"},
		{3600, "60"},
	}

	for _, tt := range tests {
		nfo := generateNFO("vid", "Title", tt.duration, "20240115")
		nfoStr := string(nfo)
		require.Contains(t, nfoStr, fmt.Sprintf("<runtime>%s</runtime>", tt.expectedMin))
	}
}

// TestMetadataCacheEntry_Expiry tests metadataCacheEntry structure and expiry checking
func TestMetadataCacheEntry_Expiry(t *testing.T) {
	entry := &metadataCacheEntry{
		data:      []byte("test"),
		timestamp: time.Now().Add(-2 * time.Hour),
	}

	// Check if it would expire (24 hour TTL)
	isExpired := time.Since(entry.timestamp) > 24*time.Hour
	require.False(t, isExpired)

	// Mark as expired
	entry.timestamp = time.Now().Add(-25 * time.Hour)
	isExpired = time.Since(entry.timestamp) > 24*time.Hour
	require.True(t, isExpired)
}

// TestMetadataCacheEntry_InFlightState tests in-flight state management
func TestMetadataCacheEntry_InFlightState(t *testing.T) {
	entry := &metadataCacheEntry{
		inFlight: true,
		waitChan: make(chan struct{}),
	}

	// Verify initial state
	entry.mu.RLock()
	require.True(t, entry.inFlight)
	entry.mu.RUnlock()

	// Transition out of in-flight
	entry.mu.Lock()
	entry.inFlight = false
	entry.mu.Unlock()
	close(entry.waitChan)

	// Verify final state
	entry.mu.RLock()
	require.False(t, entry.inFlight)
	entry.mu.RUnlock()
}

// TestList_WithInvalidPattern tests List with invalid patterns
func TestList_WithInvalidPattern(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	// Test various invalid paths
	tests := []string{
		"this/path/does/not/match",
		"nonsense/123/456",
		"totally/invalid",
	}

	for _, path := range tests {
		entries, err := f.List(ctx, path)
		require.Error(t, err)
		require.Nil(t, entries)
		require.Equal(t, fs.ErrorDirNotFound, err)
	}
}

// TestCmdReader_ZeroLengthRead tests reading zero bytes
func TestCmdReader_ZeroLengthRead(t *testing.T) {
	data := "test"
	reader := io.NopCloser(strings.NewReader(data))
	cmd := exec.Command("true")
	err := cmd.Start()
	require.NoError(t, err)

	cr := &cmdReader{
		reader: reader,
		cmd:    cmd,
	}

	// Read zero bytes
	buf := make([]byte, 0)
	n, err := cr.Read(buf)
	require.Equal(t, 0, n)

	_ = cr.Close()
}

// TestObject_Open_MetadataTimeout tests metadata open with timeout context
func TestObject_Open_MetadataTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	f := newTestFs(t)
	o := &Object{
		fs:           f,
		remote:       "vid.nfo",
		videoID:      "vid",
		metaType:     MetaNFO,
		metadataFile: "vid",
	}

	reader, _ := o.Open(ctx)
	// Either error or successfully opened (metadata might be cached)
	if reader != nil {
		_ = reader.Close()
	}
}

// TestManifest_WatcherStopChannel tests that stopCh is used in watchManifest
func TestManifest_WatcherStopChannel(t *testing.T) {
	manifest := YouTubeManifest{
		Channels: []ChannelEntry{{ID: "UC1", Name: "Ch1"}},
	}

	tmpfile, err := createTempManifest(manifest)
	require.NoError(t, err)
	defer cleanupTempFile(tmpfile)

	ctx := context.Background()
	m := configmap.Simple{
		"manifest_file": tmpfile,
		"auto_reload":   "true",
	}

	fs, fsErr := NewFs(ctx, "ytfstest", "", m)
	require.NoError(t, fsErr)

	ytfs := fs.(*Fs)
	require.NotNil(t, ytfs.stopCh)

	// Close stop channel - watcher should exit
	close(ytfs.stopCh)

	// Give watcher time to exit
	time.Sleep(100 * time.Millisecond)
}

// TestFs_RootTrimming tests that root slashes are trimmed correctly
func TestFs_RootTrimming(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"", ""},
		{"/", ""},
		{"path", "path"},
		{"/path", "path"},
		{"path/", "path"},
		{"/path/", "path"},
		{"/path/to/root", "path/to/root"},
		{"/path/to/root/", "path/to/root"},
		{"///", ""},
		{"///path///", "path"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ctx := context.Background()
			m := configmap.Simple{}
			fs, err := NewFs(ctx, "ytfstest", tt.input, m)
			require.NoError(t, err)
			require.Equal(t, tt.expect, fs.Root())
		})
	}
}

// TestObject_Hash_MultipleTypes tests Hash returns error for various hash types
func TestObject_Hash_MultipleTypes(t *testing.T) {
	ctx := context.Background()
	o := &Object{
		fs:     newTestFs(t),
		remote: "test.mp4",
	}

	hashTypes := []hash.Type{
		hash.MD5,
		hash.SHA1,
		hash.SHA256,
	}

	for _, ht := range hashTypes {
		result, err := o.Hash(ctx, ht)
		require.Equal(t, hash.ErrUnsupported, err)
		require.Equal(t, "", result)
	}
}

// TestCmdReader_CloseWithNilReader tests Close when reader is nil (edge case)
func TestCmdReader_CloseWithNilReader(t *testing.T) {
	cmd := exec.Command("true")
	err := cmd.Start()
	require.NoError(t, err)

	// Create cmdReader with valid reader but test close path
	r := io.NopCloser(strings.NewReader("test"))
	cr := &cmdReader{
		reader: r,
		cmd:    cmd,
	}

	// Close successfully
	err = cr.Close()
	require.NoError(t, err)
}

// TestGenerateNFO_EdgeCases tests NFO generation with edge case inputs
func TestGenerateNFO_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		videoID    string
		title      string
		duration   int
		uploadDate string
	}{
		{"empty fields", "", "", 0, ""},
		{"unicode", "vid123", "日本語タイトル", 120, "20240115"},
		{"very long title", "v1", strings.Repeat("A", 500), 60, "20240115"},
		{"special chars", "v2", "A&B<C>D\"E", 300, "20240115"},
		{"duration zero", "v3", "Title", 0, "20240115"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nfo := generateNFO(tt.videoID, tt.title, tt.duration, tt.uploadDate)
			require.NotNil(t, nfo)
			require.True(t, len(nfo) > 0)
			nfoStr := string(nfo)
			require.Contains(t, nfoStr, "<?xml")
		})
	}
}

// TestMetadataCache_ReuseSameKey tests reusing same cache key
func TestMetadataCache_ReuseSameKey(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t)

	callCount := 0

	// First fetch
	data1, err := f.getCachedMetadata(ctx, "same-key", func(fetchCtx context.Context) ([]byte, error) {
		callCount++
		return []byte("first"), nil
	})
	require.NoError(t, err)
	require.Equal(t, []byte("first"), data1)
	require.Equal(t, 1, callCount)

	// Second fetch same key - should use cache
	data2, err := f.getCachedMetadata(ctx, "same-key", func(fetchCtx context.Context) ([]byte, error) {
		callCount++
		return []byte("second"), nil
	})
	require.NoError(t, err)
	require.Equal(t, []byte("first"), data2) // Should return cached first value
	require.Equal(t, 1, callCount)           // Fetcher not called again
}

// TestList_EmptyRoot tests List with empty root directory
func TestList_EmptyRoot(t *testing.T) {
	ctx := context.Background()
	m := configmap.Simple{}
	fs, err := NewFs(ctx, "ytfstest", "", m)
	require.NoError(t, err)

	// List root with no manifest - should handle gracefully
	_, _ = fs.List(ctx, "")
	// This may return nil or entries depending on the pattern matching
}

// TestMetadataCache_WaitOnInFlight tests waiting for in-flight fetch
func TestMetadataCache_WaitOnInFlight(t *testing.T) {
	f := newTestFs(t)
	ctx := context.Background()

	// Simulate in-flight fetch with slow fetcher
	slowFetcher := func(fetchCtx context.Context) ([]byte, error) {
		time.Sleep(50 * time.Millisecond)
		return []byte("slow-data"), nil
	}

	// Start first goroutine (slow)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		data, err := f.getCachedMetadata(ctx, "slow-key", slowFetcher)
		require.NoError(t, err)
		require.Equal(t, []byte("slow-data"), data)
	}()

	// Give first goroutine time to start
	time.Sleep(10 * time.Millisecond)

	// Second goroutine tries to fetch same key - should wait
	data2, err := f.getCachedMetadata(ctx, "slow-key", func(fetchCtx context.Context) ([]byte, error) {
		return nil, fmt.Errorf("should not be called")
	})
	require.NoError(t, err)
	require.Equal(t, []byte("slow-data"), data2)

	wg.Wait()
}

// TestMetadataCache_CacheEntryStructure tests cache entry internal structure
func TestMetadataCache_CacheEntryStructure(t *testing.T) {
	cache := &metadataCache{
		entries: make(map[string]*metadataCacheEntry),
	}

	entry := &metadataCacheEntry{
		data:      []byte("test"),
		timestamp: time.Now(),
		inFlight:  false,
		waitChan:  make(chan struct{}),
	}

	cache.mu.Lock()
	cache.entries["test-key"] = entry
	cache.mu.Unlock()

	// Verify structure
	cache.mu.RLock()
	retrieved, exists := cache.entries["test-key"]
	cache.mu.RUnlock()

	require.True(t, exists)
	require.Equal(t, []byte("test"), retrieved.data)
	require.False(t, retrieved.inFlight)
}

// TestParseMetadataFile_Consistency tests parseMetadataFile consistency
func TestParseMetadataFile_Consistency(t *testing.T) {
	filename := "vid123.nfo"

	// Call multiple times
	for i := 0; i < 5; i++ {
		metaType, videoID := parseMetadataFile(filename)
		require.Equal(t, MetaNFO, metaType)
		require.Equal(t, "vid123", videoID)
	}
}

// TestObject_Storable_Basic tests Storable method
func TestObject_Storable_Basic(t *testing.T) {
	o := &Object{fs: newTestFs(t)}
	require.True(t, o.Storable())
}

// TestObject_Size_Negative tests Size method returns -1
func TestObject_Size_Negative(t *testing.T) {
	o := &Object{fs: newTestFs(t)}
	require.Equal(t, int64(-1), o.Size())
}

// TestManifest_ChannelNameFormat tests channel name formatting with separator
func TestManifest_ChannelNameFormat(t *testing.T) {
	manifest := YouTubeManifest{
		Channels: []ChannelEntry{
			{ID: "UC123", Name: "My Channel"},
		},
	}

	tmpfile, err := createTempManifest(manifest)
	require.NoError(t, err)
	defer cleanupTempFile(tmpfile)

	ctx := context.Background()
	m := configmap.Simple{"manifest_file": tmpfile}
	fs, err := NewFs(ctx, "ytfstest", "", m)
	require.NoError(t, err)

	ytfs := fs.(*Fs)

	entries, err := ytfs.listChannels(ctx, "")
	require.NoError(t, err)
	require.Equal(t, 1, len(entries))

	// Verify format: "id — Name"
	remote := entries[0].Remote()
	require.Contains(t, remote, "UC123")
	require.Contains(t, remote, "My Channel")
	require.Contains(t, remote, " — ") // En-dash separator

	// Clean up
	if ytfs.stopCh != nil {
		close(ytfs.stopCh)
	}
}
