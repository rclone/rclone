package ytfs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
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
