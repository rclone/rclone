package vfs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type controlledListFs struct {
	fs.Fs
	block        atomic.Bool
	fail         atomic.Bool
	ignoreCancel atomic.Bool
	wantConfig   *fs.ConfigInfo
	started      chan struct{}
	release      chan struct{}
	returned     chan struct{}
	once         sync.Once
	returnOnce   sync.Once
}

func (f *controlledListFs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	if f.wantConfig != nil && fs.GetConfig(ctx) != f.wantConfig {
		return nil, errors.New("listing context did not preserve VFS config")
	}
	if f.block.Load() {
		f.once.Do(func() { close(f.started) })
		if f.ignoreCancel.Load() {
			<-f.release
		} else {
			select {
			case <-f.release:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}
	if f.fail.Load() {
		return nil, errors.New("injected list failure")
	}
	entries, err := f.Fs.List(ctx, dir)
	if f.returned != nil {
		f.returnOnce.Do(func() { close(f.returned) })
	}
	return entries, err
}

func nodeNames(nodes Nodes) []string {
	names := make([]string, 0, len(nodes))
	for _, node := range nodes {
		names = append(names, node.Name())
	}
	sort.Strings(names)
	return names
}

func dirCreate(t *testing.T) (r *fstest.Run, vfs *VFS, dir *Dir, item fstest.Item) {
	r, vfs = newTestVFS(t)

	file1 := r.WriteObject(context.Background(), "dir/file1", "file1 contents", t1)
	r.CheckRemoteItems(t, file1)

	node, err := vfs.Stat("dir")
	require.NoError(t, err)
	require.True(t, node.IsDir())

	return r, vfs, node.(*Dir), file1
}

func TestDirMethods(t *testing.T) {
	_, vfs, dir, _ := dirCreate(t)

	// String
	assert.Equal(t, "dir/", dir.String())
	assert.Equal(t, "<nil *Dir>", (*Dir)(nil).String())

	// IsDir
	assert.Equal(t, true, dir.IsDir())

	// IsFile
	assert.Equal(t, false, dir.IsFile())

	// Mode
	assert.Equal(t, os.FileMode(vfs.Opt.DirPerms), dir.Mode())

	// Name
	assert.Equal(t, "dir", dir.Name())

	// Path
	assert.Equal(t, "dir", dir.Path())

	// Sys
	assert.Equal(t, nil, dir.Sys())

	// SetSys
	dir.SetSys(42)
	assert.Equal(t, 42, dir.Sys())

	// Inode
	assert.NotEqual(t, uint64(0), dir.Inode())

	// Node
	assert.Equal(t, dir, dir.Node())

	// ModTime
	assert.WithinDuration(t, t1, dir.ModTime(), 100*365*24*60*60*time.Second)

	// Size
	assert.Equal(t, int64(0), dir.Size())

	// Sync
	assert.NoError(t, dir.Sync())

	// DirEntry
	assert.Equal(t, dir.entry, dir.DirEntry())

	// VFS
	assert.Equal(t, vfs, dir.VFS())
}

func TestDirReadDirSWRKeepsSnapshotReadable(t *testing.T) {
	r := fstest.NewRun(t)
	r.WriteObject(context.Background(), "old.txt", "old", t1)
	f := &controlledListFs{
		Fs:      r.Fremote,
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	vfs := New(context.Background(), f, nil)
	t.Cleanup(func() { cleanupVFS(t, vfs) })
	root, err := vfs.Root()
	require.NoError(t, err)
	initial, err := root.ReadDirAll()
	require.NoError(t, err)
	assert.Equal(t, []string{"old.txt"}, nodeNames(initial))

	r.WriteObject(context.Background(), "new.txt", "new", t2)
	f.wantConfig = fs.GetConfig(vfs.ctx)
	f.block.Store(true)
	done := make(chan error, 1)
	go func() { done <- root.readDirSWR(context.Background()) }()
	select {
	case <-f.started:
	case <-time.After(5 * time.Second):
		t.Fatal("background listing did not start")
	}

	readDone := make(chan Nodes, 1)
	go func() {
		nodes, readErr := root.ReadDirAll()
		if readErr != nil {
			readDone <- nil
			return
		}
		readDone <- nodes
	}()
	select {
	case nodes := <-readDone:
		assert.Equal(t, []string{"old.txt"}, nodeNames(nodes))
	case <-time.After(5 * time.Second):
		t.Fatal("cached directory read blocked behind background listing")
	}

	close(f.release)
	require.NoError(t, <-done)
	updated, err := root.ReadDirAll()
	require.NoError(t, err)
	assert.Equal(t, []string{"new.txt", "old.txt"}, nodeNames(updated))
}

func TestDirReadDirSWRRequiresSnapshot(t *testing.T) {
	r := fstest.NewRun(t)
	vfs := New(context.Background(), r.Fremote, nil)
	t.Cleanup(func() { cleanupVFS(t, vfs) })
	root, err := vfs.Root()
	require.NoError(t, err)
	require.EqualError(t, root.readDirSWR(context.Background()), "directory cache has no existing snapshot")
}

func TestDirReadDirSWRReportsConcurrentRefresh(t *testing.T) {
	r := fstest.NewRun(t)
	r.WriteObject(context.Background(), "old.txt", "old", t1)
	f := &controlledListFs{
		Fs:      r.Fremote,
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	vfs := New(context.Background(), f, nil)
	t.Cleanup(func() { cleanupVFS(t, vfs) })
	root, err := vfs.Root()
	require.NoError(t, err)
	_, err = root.ReadDirAll()
	require.NoError(t, err)

	f.block.Store(true)
	done := make(chan error, 1)
	go func() { done <- root.readDirSWR(context.Background()) }()
	select {
	case <-f.started:
	case <-time.After(5 * time.Second):
		t.Fatal("background listing did not start")
	}
	require.EqualError(t, root.readDirSWR(context.Background()), "directory refresh already in progress")
	close(f.release)
	require.NoError(t, <-done)
}

func TestDirReadDirSWRReportsRecursiveRefresh(t *testing.T) {
	r := fstest.NewRun(t)
	r.WriteObject(context.Background(), "old.txt", "old", t1)
	vfs := New(context.Background(), r.Fremote, nil)
	t.Cleanup(func() { cleanupVFS(t, vfs) })
	root, err := vfs.Root()
	require.NoError(t, err)
	_, err = root.ReadDirAll()
	require.NoError(t, err)

	vfs.refreshMu.Lock()
	err = root.readDirSWR(context.Background())
	vfs.refreshMu.Unlock()
	require.EqualError(t, err, "recursive directory refresh already in progress")
}

func TestDirReadDirSWRFailurePreservesSnapshot(t *testing.T) {
	r := fstest.NewRun(t)
	r.WriteObject(context.Background(), "old.txt", "old", t1)
	f := &controlledListFs{
		Fs:      r.Fremote,
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	vfs := New(context.Background(), f, nil)
	t.Cleanup(func() { cleanupVFS(t, vfs) })
	root, err := vfs.Root()
	require.NoError(t, err)
	_, err = root.ReadDirAll()
	require.NoError(t, err)
	root.mu.RLock()
	readBefore := root.read
	oldNode := root.items["old.txt"]
	root.mu.RUnlock()

	f.fail.Store(true)
	err = root.readDirSWR(context.Background())
	require.EqualError(t, err, "injected list failure")
	root.mu.RLock()
	assert.Equal(t, readBefore, root.read)
	assert.Same(t, oldNode, root.items["old.txt"])
	root.mu.RUnlock()
	nodes, err := root.ReadDirAll()
	require.NoError(t, err)
	assert.Equal(t, []string{"old.txt"}, nodeNames(nodes))
}

func TestDirReadDirSWRCancellationReleasesRefresh(t *testing.T) {
	r := fstest.NewRun(t)
	r.WriteObject(context.Background(), "old.txt", "old", t1)
	f := &controlledListFs{
		Fs:      r.Fremote,
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	vfs := New(context.Background(), f, nil)
	t.Cleanup(func() { cleanupVFS(t, vfs) })
	root, err := vfs.Root()
	require.NoError(t, err)
	_, err = root.ReadDirAll()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	f.block.Store(true)
	done := make(chan error, 1)
	go func() { done <- root.readDirSWR(ctx) }()
	select {
	case <-f.started:
	case <-time.After(5 * time.Second):
		t.Fatal("background listing did not start")
	}
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)

	f.block.Store(false)
	require.NoError(t, root.readDirSWR(context.Background()))
}

func TestDirReadDirSWRRejectsResultWhenBackendIgnoresCancellation(t *testing.T) {
	r := fstest.NewRun(t)
	r.WriteObject(context.Background(), "old.txt", "old", t1)
	f := &controlledListFs{
		Fs:      r.Fremote,
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	vfs := New(context.Background(), f, nil)
	t.Cleanup(func() { cleanupVFS(t, vfs) })
	root, err := vfs.Root()
	require.NoError(t, err)
	_, err = root.ReadDirAll()
	require.NoError(t, err)
	r.WriteObject(context.Background(), "new.txt", "new", t2)

	ctx, cancel := context.WithCancel(context.Background())
	f.block.Store(true)
	f.ignoreCancel.Store(true)
	done := make(chan error, 1)
	go func() { done <- root.readDirSWR(ctx) }()
	select {
	case <-f.started:
	case <-time.After(5 * time.Second):
		t.Fatal("background listing did not start")
	}
	cancel()
	close(f.release)
	require.ErrorIs(t, <-done, context.Canceled)
	nodes, err := root.ReadDirAll()
	require.NoError(t, err)
	assert.Equal(t, []string{"old.txt"}, nodeNames(nodes))
}

func TestDirReadDirSWRRejectsCancellationWhileWaitingToMerge(t *testing.T) {
	r := fstest.NewRun(t)
	r.WriteObject(context.Background(), "old.txt", "old", t1)
	f := &controlledListFs{
		Fs:      r.Fremote,
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	vfs := New(context.Background(), f, nil)
	t.Cleanup(func() { cleanupVFS(t, vfs) })
	root, err := vfs.Root()
	require.NoError(t, err)
	_, err = root.ReadDirAll()
	require.NoError(t, err)
	r.WriteObject(context.Background(), "new.txt", "new", t2)

	ctx, cancel := context.WithCancel(context.Background())
	f.block.Store(true)
	f.ignoreCancel.Store(true)
	f.returned = make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- root.readDirSWR(ctx) }()
	select {
	case <-f.started:
	case <-time.After(5 * time.Second):
		t.Fatal("background listing did not start")
	}
	root.mu.Lock()
	close(f.release)
	select {
	case <-f.returned:
	case <-time.After(5 * time.Second):
		root.mu.Unlock()
		t.Fatal("backend listing did not return")
	}
	cancel()
	root.mu.Unlock()
	require.ErrorIs(t, <-done, context.Canceled)
	nodes, err := root.ReadDirAll()
	require.NoError(t, err)
	assert.Equal(t, []string{"old.txt"}, nodeNames(nodes))
}

func TestDirReadDirSWRRejectsResultAfterVFSCancellation(t *testing.T) {
	r := fstest.NewRun(t)
	r.WriteObject(context.Background(), "old.txt", "old", t1)
	f := &controlledListFs{
		Fs:      r.Fremote,
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	vfs := New(context.Background(), f, nil)
	t.Cleanup(func() { cleanupVFS(t, vfs) })
	root, err := vfs.Root()
	require.NoError(t, err)
	_, err = root.ReadDirAll()
	require.NoError(t, err)
	r.WriteObject(context.Background(), "new.txt", "new", t2)

	f.block.Store(true)
	f.ignoreCancel.Store(true)
	done := make(chan error, 1)
	go func() { done <- root.readDirSWR(context.Background()) }()
	select {
	case <-f.started:
	case <-time.After(5 * time.Second):
		t.Fatal("background listing did not start")
	}
	vfs.cancel()
	close(f.release)
	require.ErrorIs(t, <-done, context.Canceled)
	nodes, err := root.ReadDirAll()
	require.NoError(t, err)
	assert.Equal(t, []string{"old.txt"}, nodeNames(nodes))
}

func TestDirReadDirSWRDiscardsInvalidatedResult(t *testing.T) {
	r := fstest.NewRun(t)
	r.WriteObject(context.Background(), "old.txt", "old", t1)
	f := &controlledListFs{
		Fs:      r.Fremote,
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	vfs := New(context.Background(), f, nil)
	t.Cleanup(func() { cleanupVFS(t, vfs) })
	root, err := vfs.Root()
	require.NoError(t, err)
	_, err = root.ReadDirAll()
	require.NoError(t, err)
	r.WriteObject(context.Background(), "new.txt", "new", t2)

	f.block.Store(true)
	done := make(chan error, 1)
	go func() { done <- root.readDirSWR(context.Background()) }()
	select {
	case <-f.started:
	case <-time.After(5 * time.Second):
		t.Fatal("background listing did not start")
	}
	root.invalidateDir("")
	close(f.release)
	require.EqualError(t, <-done, "directory cache changed during refresh of \"\"")

	root.mu.RLock()
	assert.True(t, root.read.IsZero())
	assert.NotNil(t, root.items["old.txt"])
	assert.Nil(t, root.items["new.txt"])
	root.mu.RUnlock()
}

func TestDirReadDirSWRDiscardsConcurrentVirtualDelete(t *testing.T) {
	r := fstest.NewRun(t)
	r.WriteObject(context.Background(), "old.txt", "old", t1)
	f := &controlledListFs{
		Fs:      r.Fremote,
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	vfs := New(context.Background(), f, nil)
	t.Cleanup(func() { cleanupVFS(t, vfs) })
	root, err := vfs.Root()
	require.NoError(t, err)
	_, err = root.ReadDirAll()
	require.NoError(t, err)
	r.WriteObject(context.Background(), "new.txt", "new", t2)

	f.block.Store(true)
	done := make(chan error, 1)
	go func() { done <- root.readDirSWR(context.Background()) }()
	select {
	case <-f.started:
	case <-time.After(5 * time.Second):
		t.Fatal("background listing did not start")
	}
	root.DelVirtual("old.txt")
	close(f.release)
	require.EqualError(t, <-done, "directory cache changed during refresh of \"\"")

	root.mu.RLock()
	assert.NotZero(t, root.read)
	assert.Nil(t, root.items["old.txt"])
	assert.Nil(t, root.items["new.txt"])
	root.mu.RUnlock()
}

func TestDirReadDirSWRDiscardsCompetingSnapshotMerge(t *testing.T) {
	r := fstest.NewRun(t)
	r.WriteObject(context.Background(), "old.txt", "old", t1)
	f := &controlledListFs{
		Fs:      r.Fremote,
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	vfs := New(context.Background(), f, nil)
	t.Cleanup(func() { cleanupVFS(t, vfs) })
	root, err := vfs.Root()
	require.NoError(t, err)
	_, err = root.ReadDirAll()
	require.NoError(t, err)
	r.WriteObject(context.Background(), "new.txt", "new", t2)

	f.block.Store(true)
	done := make(chan error, 1)
	go func() { done <- root.readDirSWR(context.Background()) }()
	select {
	case <-f.started:
	case <-time.After(5 * time.Second):
		t.Fatal("background listing did not start")
	}
	root.mu.Lock()
	oldEntry := root.items["old.txt"].DirEntry()
	mergeErr := root._readDirFromEntries(fs.DirEntries{oldEntry}, nil, time.Time{})
	root.mu.Unlock()
	require.NoError(t, mergeErr)
	close(f.release)
	require.EqualError(t, <-done, "directory cache changed during refresh of \"\"")

	nodes, err := root.ReadDirAll()
	require.NoError(t, err)
	assert.Equal(t, []string{"old.txt"}, nodeNames(nodes))
}

func TestDirReadDirSWRDiscardsDetachedDirectory(t *testing.T) {
	r := fstest.NewRun(t)
	r.WriteObject(context.Background(), "dir/old.txt", "old", t1)
	f := &controlledListFs{
		Fs:      r.Fremote,
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	vfs := New(context.Background(), f, nil)
	t.Cleanup(func() { cleanupVFS(t, vfs) })
	_, err := vfs.Stat("dir/old.txt")
	require.NoError(t, err)
	root, err := vfs.Root()
	require.NoError(t, err)
	dir := root.cachedDir("dir")
	require.NotNil(t, dir)

	f.block.Store(true)
	done := make(chan error, 1)
	go func() { done <- dir.readDirSWR(context.Background()) }()
	select {
	case <-f.started:
	case <-time.After(5 * time.Second):
		t.Fatal("background listing did not start")
	}
	root.mu.Lock()
	mergeErr := root._readDirFromEntries(nil, nil, time.Time{})
	root.mu.Unlock()
	require.NoError(t, mergeErr)
	close(f.release)
	require.EqualError(t, <-done, "directory cache changed during refresh of \"dir\"")
	assert.Nil(t, root.cachedDir("dir"))
}

func TestDirReadDirSWRDiscardsResultAfterRename(t *testing.T) {
	r := fstest.NewRun(t)
	r.WriteObject(context.Background(), "dir/old.txt", "old", t1)
	f := &controlledListFs{
		Fs:      r.Fremote,
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	vfs := New(context.Background(), f, nil)
	t.Cleanup(func() { cleanupVFS(t, vfs) })
	_, err := vfs.Stat("dir/old.txt")
	require.NoError(t, err)
	root, err := vfs.Root()
	require.NoError(t, err)
	dir := root.cachedDir("dir")
	require.NotNil(t, dir)

	f.block.Store(true)
	done := make(chan error, 1)
	go func() { done <- dir.readDirSWR(context.Background()) }()
	select {
	case <-f.started:
	case <-time.After(5 * time.Second):
		t.Fatal("background listing did not start")
	}
	root.mu.Lock()
	dir.renameTree("renamed")
	root.mu.Unlock()
	close(f.release)
	require.EqualError(t, <-done, "directory cache changed during refresh of \"dir\"")
	assert.Same(t, dir, root.cachedDir("renamed"))
}

func TestDirReadDirSWRSerializesRecursiveRefresh(t *testing.T) {
	r := fstest.NewRun(t)
	r.WriteObject(context.Background(), "dir/old.txt", "old", t1)
	f := &controlledListFs{
		Fs:      r.Fremote,
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	vfs := New(context.Background(), f, nil)
	t.Cleanup(func() { cleanupVFS(t, vfs) })
	_, err := vfs.Stat("dir/old.txt")
	require.NoError(t, err)
	root, err := vfs.Root()
	require.NoError(t, err)
	dir := root.cachedDir("dir")
	require.NotNil(t, dir)

	f.block.Store(true)
	done := make(chan error, 1)
	go func() { done <- dir.readDirSWR(context.Background()) }()
	select {
	case <-f.started:
	case <-time.After(5 * time.Second):
		t.Fatal("background listing did not start")
	}
	if vfs.refreshMu.TryLock() {
		vfs.refreshMu.Unlock()
		t.Fatal("recursive refresh lock was available during directory refresh")
	}
	close(f.release)
	require.NoError(t, <-done)
	require.NoError(t, root.readDirTree())
}

func TestDirReadDirSWRSerializesCacheCleanup(t *testing.T) {
	r := fstest.NewRun(t)
	r.WriteObject(context.Background(), "old.txt", "old", t1)
	f := &controlledListFs{
		Fs:      r.Fremote,
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	vfs := New(context.Background(), f, nil)
	t.Cleanup(func() { cleanupVFS(t, vfs) })
	root, err := vfs.Root()
	require.NoError(t, err)
	_, err = root.ReadDirAll()
	require.NoError(t, err)
	r.WriteObject(context.Background(), "new.txt", "new", t2)
	root.mu.Lock()
	root.read = time.Now().Add(-3 * time.Duration(vfs.Opt.DirCacheTime))
	root.mu.Unlock()

	f.block.Store(true)
	refreshDone := make(chan error, 1)
	go func() { refreshDone <- root.readDirSWR(context.Background()) }()
	select {
	case <-f.started:
	case <-time.After(5 * time.Second):
		t.Fatal("background listing did not start")
	}
	if root.refreshMu.TryLock() {
		root.refreshMu.Unlock()
		t.Fatal("cache cleanup lock was available during background refresh")
	}
	nodes, err := root.ReadDirAll()
	require.NoError(t, err)
	assert.Equal(t, []string{"old.txt"}, nodeNames(nodes))

	close(f.release)
	require.NoError(t, <-refreshDone)
	root.cacheCleanup()
	updated, err := root.ReadDirAll()
	require.NoError(t, err)
	assert.Equal(t, []string{"new.txt", "old.txt"}, nodeNames(updated))
}

func TestDirReadDirSWRSerializesAncestorCacheCleanup(t *testing.T) {
	r := fstest.NewRun(t)
	r.WriteObject(context.Background(), "dir/old.txt", "old", t1)
	f := &controlledListFs{
		Fs:      r.Fremote,
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	vfs := New(context.Background(), f, nil)
	t.Cleanup(func() { cleanupVFS(t, vfs) })
	_, err := vfs.Stat("dir/old.txt")
	require.NoError(t, err)
	root, err := vfs.Root()
	require.NoError(t, err)
	dir := root.cachedDir("dir")
	require.NotNil(t, dir)
	r.WriteObject(context.Background(), "dir/new.txt", "new", t2)

	f.block.Store(true)
	refreshDone := make(chan error, 1)
	go func() { refreshDone <- dir.readDirSWR(context.Background()) }()
	select {
	case <-f.started:
	case <-time.After(5 * time.Second):
		t.Fatal("child background listing did not start")
	}
	if dir.refreshMu.TryLock() {
		dir.refreshMu.Unlock()
		t.Fatal("child cleanup lock was available during background refresh")
	}
	parentReadDone := make(chan Nodes, 1)
	go func() {
		nodes, readErr := root.ReadDirAll()
		if readErr != nil {
			parentReadDone <- nil
			return
		}
		parentReadDone <- nodes
	}()
	select {
	case nodes := <-parentReadDone:
		assert.Equal(t, []string{"dir"}, nodeNames(nodes))
	case <-time.After(5 * time.Second):
		t.Fatal("ancestor read blocked behind child background refresh")
	}
	nodes, err := dir.ReadDirAll()
	require.NoError(t, err)
	assert.Equal(t, []string{"old.txt"}, nodeNames(nodes))

	close(f.release)
	require.NoError(t, <-refreshDone)
	root.refreshMu.Lock()
	root.forgetAllForCleanup()
	root.refreshMu.Unlock()
}

func TestDirForgetAll(t *testing.T) {
	_, vfs, dir, file1 := dirCreate(t)

	// Make sure / and dir are in cache
	_, err := vfs.Stat(file1.Path)
	require.NoError(t, err)

	root, err := vfs.Root()
	require.NoError(t, err)

	assert.Equal(t, 1, len(root.items))
	assert.Equal(t, 1, len(dir.items))
	assert.False(t, root.read.IsZero())
	assert.False(t, dir.read.IsZero())

	dir.ForgetAll()
	assert.Equal(t, 1, len(root.items))
	assert.Equal(t, 0, len(dir.items))
	assert.False(t, root.read.IsZero())
	assert.True(t, dir.read.IsZero())

	root.ForgetAll()
	assert.Equal(t, 0, len(root.items))
	assert.Equal(t, 0, len(dir.items))
	assert.True(t, root.read.IsZero())
}

func TestDirForgetPath(t *testing.T) {
	_, vfs, dir, file1 := dirCreate(t)

	// Make sure / and dir are in cache
	_, err := vfs.Stat(file1.Path)
	require.NoError(t, err)

	root, err := vfs.Root()
	require.NoError(t, err)

	assert.Equal(t, 1, len(root.items))
	assert.Equal(t, 1, len(dir.items))
	assert.False(t, root.read.IsZero())
	assert.False(t, dir.read.IsZero())

	root.ForgetPath("dir/notfound", fs.EntryObject)
	assert.Equal(t, 1, len(root.items))
	assert.Equal(t, 1, len(dir.items))
	assert.False(t, root.read.IsZero())
	assert.True(t, dir.read.IsZero())

	root.ForgetPath("dir", fs.EntryDirectory)
	assert.Equal(t, 1, len(root.items))
	assert.Equal(t, 0, len(dir.items))
	assert.True(t, root.read.IsZero())

	root.ForgetPath("not/in/cache", fs.EntryDirectory)
	assert.Equal(t, 1, len(root.items))
	assert.Equal(t, 0, len(dir.items))
}

func TestDirWalk(t *testing.T) {
	r, vfs, _, file1 := dirCreate(t)

	file2 := r.WriteObject(context.Background(), "fil/a/b/c", "super long file", t1)
	r.CheckRemoteItems(t, file1, file2)

	root, err := vfs.Root()
	require.NoError(t, err)

	// Forget the cache since we put another object in
	root.ForgetAll()

	// Read the directories in
	_, err = vfs.Stat("dir")
	require.NoError(t, err)
	_, err = vfs.Stat("fil/a/b")
	require.NoError(t, err)
	fil, err := vfs.Stat("fil")
	require.NoError(t, err)

	var result []string
	fn := func(d *Dir) {
		result = append(result, d.path)
	}

	result = nil
	root.walk(fn)
	sort.Strings(result) // sort as there is a map traversal involved
	assert.Equal(t, []string{"", "dir", "fil", "fil/a", "fil/a/b"}, result)

	assert.Nil(t, root.cachedDir("not found"))
	if dir := root.cachedDir("dir"); assert.NotNil(t, dir) {
		result = nil
		dir.walk(fn)
		assert.Equal(t, []string{"dir"}, result)
	}
	if dir := root.cachedDir("fil"); assert.NotNil(t, dir) {
		result = nil
		dir.walk(fn)
		assert.Equal(t, []string{"fil/a/b", "fil/a", "fil"}, result)
	}
	if dir := fil.(*Dir); assert.NotNil(t, dir) {
		result = nil
		dir.walk(fn)
		assert.Equal(t, []string{"fil/a/b", "fil/a", "fil"}, result)
	}
	if dir := root.cachedDir("fil/a"); assert.NotNil(t, dir) {
		result = nil
		dir.walk(fn)
		assert.Equal(t, []string{"fil/a/b", "fil/a"}, result)
	}
	if dir := fil.(*Dir).cachedDir("a"); assert.NotNil(t, dir) {
		result = nil
		dir.walk(fn)
		assert.Equal(t, []string{"fil/a/b", "fil/a"}, result)
	}
	if dir := root.cachedDir("fil/a"); assert.NotNil(t, dir) {
		result = nil
		dir.walk(fn)
		assert.Equal(t, []string{"fil/a/b", "fil/a"}, result)
	}
	if dir := root.cachedDir("fil/a/b"); assert.NotNil(t, dir) {
		result = nil
		dir.walk(fn)
		assert.Equal(t, []string{"fil/a/b"}, result)
	}
}

func TestDirSetModTime(t *testing.T) {
	_, vfs, dir, _ := dirCreate(t)

	err := dir.SetModTime(t1)
	require.NoError(t, err)
	assert.WithinDuration(t, t1, dir.ModTime(), time.Second)

	err = dir.SetModTime(t2)
	require.NoError(t, err)
	assert.WithinDuration(t, t2, dir.ModTime(), time.Second)

	vfs.Opt.ReadOnly = true
	err = dir.SetModTime(t2)
	assert.Equal(t, EROFS, err)
}

func TestDirStat(t *testing.T) {
	_, _, dir, _ := dirCreate(t)

	node, err := dir.Stat("file1")
	require.NoError(t, err)
	_, ok := node.(*File)
	assert.True(t, ok)
	assert.Equal(t, int64(14), node.Size())
	assert.Equal(t, "file1", node.Name())

	_, err = dir.Stat("not found")
	assert.Equal(t, ENOENT, err)
}

// This lists dir and checks the listing is as expected
func checkListing(t *testing.T, dir *Dir, want []string) {
	var got []string
	nodes, err := dir.ReadDirAll()
	require.NoError(t, err)
	for _, node := range nodes {
		got = append(got, fmt.Sprintf("%s,%d,%v", node.Name(), node.Size(), node.IsDir()))
	}
	assert.Equal(t, want, got)
}

func TestDirReadDirAll(t *testing.T) {
	r, vfs := newTestVFS(t)

	file1 := r.WriteObject(context.Background(), "dir/file1", "file1 contents", t1)
	file2 := r.WriteObject(context.Background(), "dir/file2", "file2- contents", t2)
	file3 := r.WriteObject(context.Background(), "dir/subdir/file3", "file3-- contents", t3)
	r.CheckRemoteItems(t, file1, file2, file3)

	node, err := vfs.Stat("dir")
	require.NoError(t, err)
	dir := node.(*Dir)

	checkListing(t, dir, []string{"file1,14,false", "file2,15,false", "subdir,0,true"})

	node, err = vfs.Stat("")
	require.NoError(t, err)
	root := node.(*Dir)

	checkListing(t, root, []string{"dir,0,true"})

	node, err = vfs.Stat("dir/subdir")
	require.NoError(t, err)
	subdir := node.(*Dir)

	checkListing(t, subdir, []string{"file3,16,false"})

	t.Run("Virtual", func(t *testing.T) {
		// Add some virtual entries and check what happens
		dir.AddVirtual("virtualFile", 17, false)
		dir.AddVirtual("virtualDir", 0, true)
		// Remove some existing entries
		dir.DelVirtual("file2")
		dir.DelVirtual("subdir")

		checkListing(t, dir, []string{"file1,14,false", "virtualDir,0,true", "virtualFile,17,false"})

		// Now action the deletes and uploads
		_ = r.WriteObject(context.Background(), "dir/virtualFile", "virtualFile contents", t1)
		_ = r.WriteObject(context.Background(), "dir/virtualDir/testFile", "testFile contents", t1)
		o, err := r.Fremote.NewObject(context.Background(), "dir/file2")
		require.NoError(t, err)
		require.NoError(t, o.Remove(context.Background()))
		require.NoError(t, operations.Purge(context.Background(), r.Fremote, "dir/subdir"))

		// Force a directory reload...
		dir.invalidateDir("dir")

		checkListing(t, dir, []string{"file1,14,false", "virtualDir,0,true", "virtualFile,20,false"})

		// check no virtuals left
		dir.mu.Lock()
		assert.Nil(t, dir.virtual)
		dir.mu.Unlock()

		// Add some virtual entries and check what happens
		dir.AddVirtual("virtualFile2", 100, false)
		dir.AddVirtual("virtualDir2", 0, true)
		// Remove some existing entries
		dir.DelVirtual("file1")

		checkListing(t, dir, []string{"virtualDir,0,true", "virtualDir2,0,true", "virtualFile,20,false", "virtualFile2,100,false"})

		// Force a directory reload...
		dir.invalidateDir("dir")

		want := []string{"file1,14,false", "virtualDir,0,true", "virtualDir2,0,true", "virtualFile,20,false", "virtualFile2,100,false"}
		features := r.Fremote.Features()
		if features.CanHaveEmptyDirectories {
			// snip out virtualDir2 which will only be present if can't have empty dirs
			want = slices.Delete(want, 2, 3)
		}
		checkListing(t, dir, want)

		// Check that forgetting the root doesn't invalidate the virtual entries
		root.ForgetAll()

		checkListing(t, dir, want)
	})
}

func TestDirOpen(t *testing.T) {
	_, _, dir, _ := dirCreate(t)

	fd, err := dir.Open(os.O_RDONLY)
	require.NoError(t, err)
	_, ok := fd.(*DirHandle)
	assert.True(t, ok)
	require.NoError(t, fd.Close())

	_, err = dir.Open(os.O_WRONLY)
	assert.Equal(t, EPERM, err)
}

func TestDirCreate(t *testing.T) {
	_, vfs, dir, _ := dirCreate(t)

	origModTime := dir.ModTime()
	time.Sleep(100 * time.Millisecond) // for low rez Windows timers
	file, err := dir.Create("potato", os.O_WRONLY|os.O_CREATE)
	require.NoError(t, err)
	assert.Equal(t, int64(0), file.Size())
	assert.True(t, dir.ModTime().After(origModTime))

	fd, err := file.Open(os.O_WRONLY | os.O_CREATE)
	require.NoError(t, err)

	// FIXME Note that this fails with the current implementation
	// until the file has been opened.

	// file2, err := vfs.Stat("dir/potato")
	// require.NoError(t, err)
	// assert.Equal(t, file, file2)

	n, err := fd.Write([]byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, 5, n)

	require.NoError(t, fd.Close())

	file2, err := vfs.Stat("dir/potato")
	require.NoError(t, err)
	assert.Equal(t, int64(5), file2.Size())

	// Try creating the file again - make sure we get the same file node
	file3, err := dir.Create("potato", os.O_RDWR|os.O_CREATE)
	require.NoError(t, err)
	assert.Equal(t, int64(5), file3.Size())
	assert.Equal(t, fmt.Sprintf("%p", file), fmt.Sprintf("%p", file3), "didn't return same node")

	// Test read only fs creating new
	vfs.Opt.ReadOnly = true
	_, err = dir.Create("sausage", os.O_WRONLY|os.O_CREATE)
	assert.Equal(t, EROFS, err)
}

func TestDirMkdir(t *testing.T) {
	r, vfs, dir, file1 := dirCreate(t)

	_, err := dir.Mkdir("file1")
	assert.Error(t, err)

	origModTime := dir.ModTime()
	time.Sleep(100 * time.Millisecond) // for low rez Windows timers
	sub, err := dir.Mkdir("sub")
	assert.NoError(t, err)
	assert.True(t, dir.ModTime().After(origModTime))

	// check the vfs
	checkListing(t, dir, []string{"file1,14,false", "sub,0,true"})
	checkListing(t, sub, []string(nil))

	// check the underlying r.Fremote
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1}, []string{"dir", "dir/sub"}, r.Fremote.Precision())

	vfs.Opt.ReadOnly = true
	_, err = dir.Mkdir("sausage")
	assert.Equal(t, EROFS, err)
}

func TestDirMkdirSub(t *testing.T) {
	r, vfs, dir, file1 := dirCreate(t)

	_, err := dir.Mkdir("file1")
	assert.Error(t, err)

	sub, err := dir.Mkdir("sub")
	assert.NoError(t, err)

	subsub, err := sub.Mkdir("subsub")
	assert.NoError(t, err)

	// check the vfs
	checkListing(t, dir, []string{"file1,14,false", "sub,0,true"})
	checkListing(t, sub, []string{"subsub,0,true"})
	checkListing(t, subsub, []string(nil))

	// check the underlying r.Fremote
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1}, []string{"dir", "dir/sub", "dir/sub/subsub"}, r.Fremote.Precision())

	vfs.Opt.ReadOnly = true
	_, err = dir.Mkdir("sausage")
	assert.Equal(t, EROFS, err)
}

func TestDirRemove(t *testing.T) {
	r, vfs, dir, _ := dirCreate(t)

	// check directory is there
	node, err := vfs.Stat("dir")
	require.NoError(t, err)
	assert.True(t, node.IsDir())

	err = dir.Remove()
	assert.Equal(t, ENOTEMPTY, err)

	// Delete the sub file
	node, err = vfs.Stat("dir/file1")
	require.NoError(t, err)
	err = node.Remove()
	require.NoError(t, err)

	// Remove the now empty directory
	err = dir.Remove()
	require.NoError(t, err)

	// check directory is not there
	_, err = vfs.Stat("dir")
	assert.Equal(t, ENOENT, err)

	// check the vfs
	root, err := vfs.Root()
	require.NoError(t, err)
	checkListing(t, root, []string(nil))

	// check the underlying r.Fremote
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{}, []string{}, r.Fremote.Precision())

	// read only check
	vfs.Opt.ReadOnly = true
	err = dir.Remove()
	assert.Equal(t, EROFS, err)
}

func TestDirRemoveAll(t *testing.T) {
	r, vfs, dir, _ := dirCreate(t)

	// Remove the directory and contents
	err := dir.RemoveAll()
	require.NoError(t, err)

	// check the vfs
	root, err := vfs.Root()
	require.NoError(t, err)
	checkListing(t, root, []string(nil))

	// check the underlying r.Fremote
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{}, []string{}, r.Fremote.Precision())

	// read only check
	vfs.Opt.ReadOnly = true
	err = dir.RemoveAll()
	assert.Equal(t, EROFS, err)
}

func TestDirRemoveName(t *testing.T) {
	r, vfs, dir, _ := dirCreate(t)

	origModTime := dir.ModTime()
	time.Sleep(100 * time.Millisecond) // for low rez Windows timers
	err := dir.RemoveName("file1")
	require.NoError(t, err)
	assert.True(t, dir.ModTime().After(origModTime))
	checkListing(t, dir, []string(nil))
	root, err := vfs.Root()
	require.NoError(t, err)
	checkListing(t, root, []string{"dir,0,true"})

	// check the underlying r.Fremote
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{}, []string{"dir"}, r.Fremote.Precision())

	// read only check
	vfs.Opt.ReadOnly = true
	err = dir.RemoveName("potato")
	assert.Equal(t, EROFS, err)
}

func TestDirRename(t *testing.T) {
	r, vfs, dir, file1 := dirCreate(t)

	features := r.Fremote.Features()
	if features.DirMove == nil && features.Move == nil && features.Copy == nil {
		t.Skip("can't rename directories")
	}

	file3 := r.WriteObject(context.Background(), "dir/file3", "file3 contents!", t1)
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1, file3}, []string{"dir"}, r.Fremote.Precision())

	root, err := vfs.Root()
	require.NoError(t, err)

	err = dir.Rename("not found", "tuba", dir)
	assert.Equal(t, ENOENT, err)

	// Rename a directory
	err = root.Rename("dir", "dir2", root)
	assert.NoError(t, err)
	checkListing(t, root, []string{"dir2,0,true"})
	checkListing(t, dir, []string{"file1,14,false", "file3,15,false"})

	// check the underlying r.Fremote
	file1.Path = "dir2/file1"
	file3.Path = "dir2/file3"
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1, file3}, []string{"dir2"}, r.Fremote.Precision())

	// refetch dir
	node, err := vfs.Stat("dir2")
	assert.NoError(t, err)
	dir = node.(*Dir)

	// Rename a file
	origModTime := dir.ModTime()
	time.Sleep(100 * time.Millisecond) // for low rez Windows timers
	err = dir.Rename("file1", "file2", root)
	assert.NoError(t, err)
	assert.True(t, dir.ModTime().After(origModTime))
	checkListing(t, root, []string{"dir2,0,true", "file2,14,false"})
	checkListing(t, dir, []string{"file3,15,false"})

	// check the underlying r.Fremote
	file1.Path = "file2"
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1, file3}, []string{"dir2"}, r.Fremote.Precision())

	// Rename a file on top of another file
	err = root.Rename("file2", "file3", dir)
	assert.NoError(t, err)
	checkListing(t, root, []string{"dir2,0,true"})
	checkListing(t, dir, []string{"file3,14,false"})

	// check the underlying r.Fremote
	file1.Path = "dir2/file3"
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1}, []string{"dir2"}, r.Fremote.Precision())

	// rename an empty directory
	_, err = root.Mkdir("empty directory")
	assert.NoError(t, err)
	checkListing(t, root, []string{
		"dir2,0,true",
		"empty directory,0,true",
	})
	err = root.Rename("empty directory", "renamed empty directory", root)
	assert.NoError(t, err)
	checkListing(t, root, []string{
		"dir2,0,true",
		"renamed empty directory,0,true",
	})
	// ...we don't check the underlying f.Fremote because on
	// bucket-based remotes the directory won't be there

	// read only check
	vfs.Opt.ReadOnly = true
	err = dir.Rename("potato", "tuba", dir)
	assert.Equal(t, EROFS, err)

	// Rename a dir, check that key was correctly renamed in dir.parent.items
	vfs.Opt.ReadOnly = false
	_, ok := dir.parent.items["dir2"]
	assert.True(t, ok, "dir.parent.items should have 'dir2' key before rename")
	_, ok = dir.parent.items["dir3"]
	assert.False(t, ok, "dir.parent.items should not have 'dir3' key before rename")
	dir.renameTree("dir3") // rename dir2 to dir3
	_, ok = dir.parent.items["dir2"]
	assert.False(t, ok, "dir.parent.items should not have 'dir2' key after rename")
	d, ok := dir.parent.items["dir3"]
	assert.True(t, ok, fmt.Sprintf("expected to find 'dir3' key in dir.parent.items after rename, got %v", dir.parent.items))
	assert.Equal(t, dir, d, `expected renamed dir to match value of dir.parent.items["dir3"]`)
}

func TestDirStructSize(t *testing.T) {
	t.Logf("Dir struct has size %d bytes", unsafe.Sizeof(Dir{}))
}

// Check that open files appear in the directory listing properly after a forget
func TestDirFileOpen(t *testing.T) {
	_, vfs, dir, _ := dirCreate(t)

	assert.False(t, dir.hasVirtual())
	assert.False(t, dir.parent.hasVirtual())

	_, err := dir.Mkdir("sub")
	require.NoError(t, err)

	assert.True(t, dir.hasVirtual())
	assert.True(t, dir.parent.hasVirtual())

	fd0, err := vfs.Create("dir/sub/file0")
	require.NoError(t, err)
	_, err = fd0.Write([]byte("hello"))
	require.NoError(t, err)
	defer func() {
		require.NoError(t, fd0.Close())
	}()

	fd2, err := vfs.Create("dir/sub/file2")
	require.NoError(t, err)
	_, err = fd2.Write([]byte("hello world!"))
	require.NoError(t, err)
	require.NoError(t, fd2.Close())
	assert.True(t, dir.hasVirtual())

	assert.True(t, dir.hasVirtual())
	assert.True(t, dir.parent.hasVirtual())

	// Now forget the directory
	hasVirtual := dir.parent.ForgetAll()
	assert.True(t, hasVirtual)

	assert.True(t, dir.hasVirtual())
	assert.True(t, dir.parent.hasVirtual())

	// Check the files can still be found
	fi, err := vfs.Stat("dir/sub/file0")
	require.NoError(t, err)
	assert.Equal(t, int64(5), fi.Size())

	fi, err = vfs.Stat("dir/sub/file2")
	require.NoError(t, err)
	assert.Equal(t, int64(12), fi.Size())
}

func TestDirEntryModTimeInvalidation(t *testing.T) {
	r, vfs := newTestVFS(t)
	features := r.Fremote.Features()
	if !features.DirModTimeUpdatesOnWrite {
		t.Skip("Need DirModTimeUpdatesOnWrite")
	}
	if features.IsLocal && runtime.GOOS == "windows" {
		t.Skip("dirent modtime is unreliable on Windows filesystems")
	}

	r.WriteObject(context.Background(), "dir/file1", "file1 contents", t1)

	// Read the modtime of the directory fresh
	vfs.FlushDirCache()
	node, err := vfs.Stat("dir")
	require.NoError(t, err)
	modTime1 := node.(*Dir).DirEntry().ModTime(context.Background())

	// Wait some time (we wait for Precision+10%), then write another file
	// which should update the ModTime of the directory.
	prec := (11 * vfs.f.Precision()) / 10
	time.Sleep(max(100*time.Millisecond, prec))
	r.WriteObject(context.Background(), "dir/file2", "file2 contents", t2)

	// Read the modtime of the directory fresh again - it should have changed
	vfs.FlushDirCache()
	node2, err := vfs.Stat("dir")
	require.NoError(t, err)
	modTime2 := node2.(*Dir).DirEntry().ModTime(context.Background())

	// ModTime of directory must be different after second file was written.
	if modTime1.Equal(modTime2) {
		t.Error("ModTime not invalidated")
	}
}

func TestDirMetadataExtension(t *testing.T) {
	r, vfs, dir, _ := dirCreate(t)
	root, err := vfs.Root()
	require.NoError(t, err)
	features := r.Fremote.Features()

	checkListing(t, dir, []string{"file1,14,false"})
	checkListing(t, root, []string{"dir,0,true"})

	node, err := vfs.Stat("dir/file1")
	require.NoError(t, err)
	require.True(t, node.IsFile())

	node, err = vfs.Stat("dir")
	require.NoError(t, err)
	require.True(t, node.IsDir())

	// Check metadata files do not exist
	_, err = vfs.Stat("dir/file1.metadata")
	require.Error(t, err, ENOENT)
	_, err = vfs.Stat("dir.metadata")
	require.Error(t, err, ENOENT)

	// Configure metadata extension
	vfs.Opt.MetadataExtension = ".metadata"

	// Check metadata for file does exist
	node, err = vfs.Stat("dir/file1.metadata")
	require.NoError(t, err)
	require.True(t, node.IsFile())
	size := node.Size()
	assert.Greater(t, size, int64(1))
	modTime := node.ModTime()

	// ...and is now in the listing
	checkListing(t, dir, []string{"file1,14,false", fmt.Sprintf("file1.metadata,%d,false", size)})

	// ...and is a JSON blob with correct "mtime" key
	blob, err := vfs.ReadFile("dir/file1.metadata")
	require.NoError(t, err)
	var metadata map[string]string
	err = json.Unmarshal(blob, &metadata)
	require.NoError(t, err)
	if features.ReadMetadata {
		assert.Equal(t, modTime.Format(time.RFC3339Nano), metadata["mtime"])
	}

	// Check metadata for dir does exist
	node, err = vfs.Stat("dir.metadata")
	require.NoError(t, err)
	require.True(t, node.IsFile())
	size = node.Size()
	assert.Greater(t, size, int64(1))
	modTime = node.ModTime()

	// ...and is now in the listing
	checkListing(t, root, []string{"dir,0,true", fmt.Sprintf("dir.metadata,%d,false", size)})

	// ...and is a JSON blob with correct "mtime" key
	blob, err = vfs.ReadFile("dir.metadata")
	require.NoError(t, err)
	clear(metadata)
	err = json.Unmarshal(blob, &metadata)
	require.NoError(t, err)
	if features.ReadDirMetadata {
		assert.Equal(t, modTime.Format(time.RFC3339Nano), metadata["mtime"])
	}
}
