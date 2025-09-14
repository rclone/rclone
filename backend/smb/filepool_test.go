package smb

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/cloudsoda/go-smb2"
	"github.com/stretchr/testify/assert"
)

// Mock Fs that implements FsInterface
type mockFs struct {
	mu                  sync.Mutex
	putConnectionCalled bool
	putConnectionErr    error
	getConnectionCalled bool
	getConnectionErr    error
	getConnectionResult *conn
	removeSessionCalled bool
}

func (m *mockFs) putConnection(pc **conn, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.putConnectionCalled = true
	m.putConnectionErr = err
}

func (m *mockFs) getConnection(ctx context.Context, share string) (*conn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getConnectionCalled = true
	if m.getConnectionErr != nil {
		return nil, m.getConnectionErr
	}
	if m.getConnectionResult != nil {
		return m.getConnectionResult, nil
	}
	return &conn{}, nil
}

func (m *mockFs) removeSession() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removeSessionCalled = true
}

func (m *mockFs) isPutConnectionCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.putConnectionCalled
}

func (m *mockFs) getPutConnectionErr() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.putConnectionErr
}

func (m *mockFs) isGetConnectionCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getConnectionCalled
}

func newMockFs() *mockFs {
	return &mockFs{}
}

// Helper function to create a mock file
func newMockFile() *file {
	return &file{
		File: &smb2.File{},
		c:    &conn{},
	}
}

// Test filePool creation
func TestNewFilePool(t *testing.T) {
	ctx := context.Background()
	fs := newMockFs()
	share := "testshare"
	path := "/test/path"

	pool := newFilePool(ctx, fs, share, path)

	assert.NotNil(t, pool)
	assert.Equal(t, ctx, pool.ctx)
	assert.Equal(t, fs, pool.fs)
	assert.Equal(t, share, pool.share)
	assert.Equal(t, path, pool.path)
	assert.Empty(t, pool.pool)
}

// Test getting file from pool when pool has files
func TestFilePool_Get_FromPool(t *testing.T) {
	ctx := context.Background()
	fs := newMockFs()
	pool := newFilePool(ctx, fs, "testshare", "/test/path")

	// Add a mock file to the pool
	mockFile := newMockFile()
	pool.pool = append(pool.pool, mockFile)

	// Get file from pool
	f, err := pool.get()

	assert.NoError(t, err)
	assert.NotNil(t, f)
	assert.Equal(t, mockFile, f)
	assert.Empty(t, pool.pool)
}

// Test getting file when pool is empty
func TestFilePool_Get_EmptyPool(t *testing.T) {
	ctx := context.Background()
	fs := newMockFs()

	// Set up the mock to return an error from getConnection
	// This tests that the pool calls getConnection when empty
	fs.getConnectionErr = errors.New("connection failed")

	pool := newFilePool(ctx, fs, "testshare", "test/path")

	// This should call getConnection and return the error
	f, err := pool.get()
	assert.Error(t, err)
	assert.Nil(t, f)
	assert.True(t, fs.isGetConnectionCalled())
	assert.Equal(t, "connection failed", err.Error())
}

// Test putting file successfully
func TestFilePool_Put_Success(t *testing.T) {
	ctx := context.Background()
	fs := newMockFs()
	pool := newFilePool(ctx, fs, "testshare", "/test/path")

	mockFile := newMockFile()

	pool.put(mockFile, nil)

	assert.Len(t, pool.pool, 1)
	assert.Equal(t, mockFile, pool.pool[0])
}

// Test putting file with error
func TestFilePool_Put_WithError(t *testing.T) {
	ctx := context.Background()
	fs := newMockFs()
	pool := newFilePool(ctx, fs, "testshare", "/test/path")

	mockFile := newMockFile()

	pool.put(mockFile, errors.New("write error"))

	// Should call putConnection with error
	assert.True(t, fs.isPutConnectionCalled())
	assert.Equal(t, errors.New("write error"), fs.getPutConnectionErr())
	assert.Empty(t, pool.pool)
}

// Test putting nil file
func TestFilePool_Put_NilFile(t *testing.T) {
	ctx := context.Background()
	fs := newMockFs()
	pool := newFilePool(ctx, fs, "testshare", "/test/path")

	// Should not panic
	pool.put(nil, nil)
	pool.put(nil, errors.New("some error"))

	assert.Empty(t, pool.pool)
}

// Test draining pool with files
func TestFilePool_Drain_WithFiles(t *testing.T) {
	ctx := context.Background()
	fs := newMockFs()
	pool := newFilePool(ctx, fs, "testshare", "/test/path")

	// Add mock files to pool
	mockFile1 := newMockFile()
	mockFile2 := newMockFile()
	pool.pool = append(pool.pool, mockFile1, mockFile2)

	// Before draining
	assert.Len(t, pool.pool, 2)

	_ = pool.drain()
	assert.Empty(t, pool.pool)
}

// Test concurrent access to pool
func TestFilePool_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	fs := newMockFs()
	pool := newFilePool(ctx, fs, "testshare", "/test/path")

	const numGoroutines = 10
	for range numGoroutines {
		mockFile := newMockFile()
		pool.pool = append(pool.pool, mockFile)
	}

	// Test concurrent get operations
	done := make(chan bool, numGoroutines)

	for range numGoroutines {
		go func() {
			defer func() { done <- true }()

			f, err := pool.get()
			if err == nil {
				pool.put(f, nil)
			}
		}()
	}

	for range numGoroutines {
		<-done
	}

	// Pool should be in a consistent after the concurrence access
	assert.Len(t, pool.pool, numGoroutines)
}
