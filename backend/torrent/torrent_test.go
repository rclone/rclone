package torrent_test

import (
	"context"
	"crypto/sha1"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rclone/rclone/backend/torrent"
)

func TestMain(m *testing.M) {
	fstest.TestMain(m)
}

// createTestTorrent creates a test torrent file with specific content
func createTestTorrent(t *testing.T, name string, content []byte) string {
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	contentPath := filepath.Join(dir, "test.txt")

	// Create test content
	require.NoError(t, os.WriteFile(contentPath, content, 0644))

	// Create torrent file metadata
	mi := &metainfo.MetaInfo{
		CreatedBy:    "rclone test",
		CreationDate: time.Now().Unix(),
	}

	// Create info section
	info := metainfo.Info{
		PieceLength: 16384,
		Name:        "test.txt",
		Length:      int64(len(content)),
		Files: []metainfo.FileInfo{
			{
				Length: int64(len(content)),
				Path:   []string{"test.txt"},
			},
		},
	}

	// Generate pieces
	nPieces := (len(content) + int(info.PieceLength) - 1) / int(info.PieceLength)
	info.Pieces = make([]byte, nPieces*20)
	for i := 0; i < nPieces; i++ {
		start := i * int(info.PieceLength)
		end := start + int(info.PieceLength)
		if end > len(content) {
			end = len(content)
		}
		piece := content[start:end]
		hash := sha1.Sum(piece)
		copy(info.Pieces[i*20:], hash[:])
	}

	// Marshal info dictionary
	infoBytes, err := bencode.Marshal(info)
	require.NoError(t, err)
	mi.InfoBytes = infoBytes

	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	require.NoError(t, mi.Write(f))
	return path
}

// TestBasicOperations tests basic functionality
func TestBasicOperations(t *testing.T) {
	const testContent = "test content for torrent\n"

	rootDir := t.TempDir()
	m := configmap.Simple{
		"root_directory":   rootDir,
		"piece_read_ahead": "5",
	}

	f, err := torrent.NewFs(context.Background(), "test:", rootDir, m)
	require.NoError(t, err)

	// Create and add test torrent
	torrentPath := createTestTorrent(t, "test1.torrent", []byte(testContent))
	require.NoError(t, copyFile(torrentPath, filepath.Join(rootDir, "test1.torrent")))

	// Test listing
	entries, err := f.List(context.Background(), "")
	require.NoError(t, err)
	assert.Len(t, entries, 1)

	// Test reading
	obj, err := f.NewObject(context.Background(), "test.txt")
	require.NoError(t, err)

	reader, err := obj.Open(context.Background())
	require.NoError(t, err)
	defer reader.Close()

	content, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, testContent, string(content))
}

// TestBandwidthControl tests bandwidth limiting
func TestBandwidthControl(t *testing.T) {
	rootDir := t.TempDir()
	m := configmap.Simple{
		"root_directory":     rootDir,
		"max_download_speed": "1024",
		"max_upload_speed":   "512",
	}

	f, err := torrent.NewFs(context.Background(), "test:", rootDir, m)
	require.NoError(t, err)

	content := make([]byte, 1024*1024) // 1MB of data
	torrentPath := createTestTorrent(t, "test.torrent", content)
	require.NoError(t, copyFile(torrentPath, filepath.Join(rootDir, "test.torrent")))

	obj, err := f.NewObject(context.Background(), "test.txt")
	require.NoError(t, err)

	reader, err := obj.Open(context.Background())
	require.NoError(t, err)
	defer reader.Close()

	buf := make([]byte, 1024*512) // Read 512KB
	start := time.Now()
	_, err = reader.Read(buf)
	require.NoError(t, err)
	duration := time.Since(start)

	// Should take at least 500ms due to rate limiting
	assert.Greater(t, duration, time.Millisecond*500)
}

// TestCleanup tests automatic torrent cleanup
func TestCleanup(t *testing.T) {
	rootDir := t.TempDir()
	m := configmap.Simple{
		"root_directory":  rootDir,
		"cleanup_timeout": "1",
	}

	f, err := torrent.NewFs(context.Background(), "test:", rootDir, m)
	require.NoError(t, err)

	torrentPath := createTestTorrent(t, "test.torrent", []byte("test content"))
	require.NoError(t, copyFile(torrentPath, filepath.Join(rootDir, "test.torrent")))

	// Initial check
	entries, err := f.List(context.Background(), "")
	require.NoError(t, err)
	assert.Len(t, entries, 1, "Should have one entry initially")

	// Wait for cleanup
	time.Sleep(time.Minute * 2)

	// Verify cleanup
	entries, err = f.List(context.Background(), "")
	require.NoError(t, err)
	assert.Empty(t, entries, "Should have no entries after cleanup")
}

// TestNestedDirectories tests directory structure handling
func TestNestedDirectories(t *testing.T) {
	rootDir := t.TempDir()
	m := configmap.Simple{
		"root_directory": rootDir,
	}

	f, err := torrent.NewFs(context.Background(), "test:", rootDir, m)
	require.NoError(t, err)

	// Create nested directory structure
	require.NoError(t, os.MkdirAll(filepath.Join(rootDir, "movies/action"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(rootDir, "movies/drama"), 0755))

	// Create test torrents in different directories
	content1 := []byte("action movie content")
	torrent1 := createTestTorrent(t, "action.torrent", content1)
	require.NoError(t, copyFile(torrent1, filepath.Join(rootDir, "movies/action/movie1.torrent")))

	content2 := []byte("drama movie content")
	torrent2 := createTestTorrent(t, "drama.torrent", content2)
	require.NoError(t, copyFile(torrent2, filepath.Join(rootDir, "movies/drama/movie2.torrent")))

	// Test root listing
	entries, err := f.List(context.Background(), "")
	require.NoError(t, err)
	assert.Len(t, entries, 1, "Should show 'movies' directory")

	// Test nested listings
	entries, err = f.List(context.Background(), "movies")
	require.NoError(t, err)
	assert.Len(t, entries, 2, "Should show 'action' and 'drama' directories")

	// Test file access
	obj1, err := f.NewObject(context.Background(), "movies/action/test.txt")
	require.NoError(t, err)
	reader1, err := obj1.Open(context.Background())
	require.NoError(t, err)
	content, err := io.ReadAll(reader1)
	require.NoError(t, err)
	assert.Equal(t, "action movie content", string(content))
	reader1.Close()

	obj2, err := f.NewObject(context.Background(), "movies/drama/test.txt")
	require.NoError(t, err)
	reader2, err := obj2.Open(context.Background())
	require.NoError(t, err)
	content, err = io.ReadAll(reader2)
	require.NoError(t, err)
	assert.Equal(t, "drama movie content", string(content))
	reader2.Close()
}

// Helper function to copy a file
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
