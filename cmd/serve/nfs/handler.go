//go:build unix
// +build unix

package nfs

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
	"github.com/willscott/go-nfs"
	nfshelper "github.com/willscott/go-nfs/helpers"
)

// NewBackendAuthHandler creates a handler for the provided filesystem
func NewBackendAuthHandler(vfs *vfs.VFS, opt *Options) nfs.Handler {
	handler := &BackendAuthHandler{
		vfs: vfs,
		opt: opt,
	}
	handler.opt.HandleLimit = handler.opt.Limit()
	handler.cache = cacheHelper(handler, handler.HandleLimit())
	return handler
}

// BackendAuthHandler returns a NFS backing that exposes a given file system in response to all mount requests.
type BackendAuthHandler struct {
	vfs   *vfs.VFS
	opt   *Options
	cache nfs.Handler
}

// Mount backs Mount RPC Requests, allowing for access control policies.
func (h *BackendAuthHandler) Mount(ctx context.Context, conn net.Conn, req nfs.MountRequest) (status nfs.MountStatus, hndl billy.Filesystem, auths []nfs.AuthFlavor) {
	status = nfs.MountStatusOk
	hndl = &FS{vfs: h.vfs}
	auths = []nfs.AuthFlavor{nfs.AuthFlavorNull}
	return
}

// Change provides an interface for updating file attributes.
func (h *BackendAuthHandler) Change(fs billy.Filesystem) billy.Change {
	if c, ok := fs.(billy.Change); ok {
		return c
	}
	return nil
}

// FSStat provides information about a filesystem.
func (h *BackendAuthHandler) FSStat(ctx context.Context, f billy.Filesystem, s *nfs.FSStat) error {
	total, _, free := h.vfs.Statfs()
	s.TotalSize = uint64(total)
	s.FreeSize = uint64(free)
	s.AvailableSize = uint64(free)
	return nil
}

// ToHandle handled by CachingHandler
func (h *BackendAuthHandler) ToHandle(f billy.Filesystem, s []string) []byte {
	return h.cache.ToHandle(f, s)
}

// FromHandle handled by CachingHandler
func (h *BackendAuthHandler) FromHandle(b []byte) (billy.Filesystem, []string, error) {
	return h.cache.FromHandle(b)
}

// HandleLimit handled by cachingHandler
func (h *BackendAuthHandler) HandleLimit() int {
	return h.opt.HandleLimit
}

// InvalidateHandle is called on removes or renames
func (h *BackendAuthHandler) InvalidateHandle(billy.Filesystem, []byte) error {
	return nil
}

func newHandler(vfs *vfs.VFS, opt *Options) nfs.Handler {
	handler := NewBackendAuthHandler(vfs, opt)
	nfs.SetLogger(&LogIntercepter{Level: nfs.DebugLevel})
	return handler
}

func cacheHelper(handler nfs.Handler, limit int) nfs.Handler {
	cacheHelper := nfshelper.NewCachingHandler(handler, limit)
	return cacheHelper
}

// Limit overrides the --nfs-cache-handle-limit value if out-of-range
func (o *Options) Limit() int {
	if o.HandleLimit < 0 {
		return 1000000
	}
	if o.HandleLimit <= 5 {
		return 5
	}
	return o.HandleLimit
}

// OnUnmountFunc registers a function to call when externally unmounted
var OnUnmountFunc func()

func onUnmount() {
	fs.Infof(nil, "unmount detected")
	if OnUnmountFunc != nil {
		OnUnmountFunc()
	}
}

// LogIntercepter intercepts noisy go-nfs logs and reroutes them to DEBUG
type LogIntercepter struct {
	Level nfs.LogLevel
}

// Intercept intercepts go-nfs logs and calls fs.Debugf instead
func (l *LogIntercepter) Intercept(args ...interface{}) {
	args = append([]interface{}{"[NFS DEBUG] "}, args...)
	argsS := fmt.Sprint(args...)
	fs.Debugf(nil, "%v", argsS)
}

// Interceptf intercepts go-nfs logs and calls fs.Debugf instead
func (l *LogIntercepter) Interceptf(format string, args ...interface{}) {
	argsS := fmt.Sprint(args...)
	// bit of a workaround... the real fix is probably https://github.com/willscott/go-nfs/pull/28
	if strings.Contains(argsS, "mount.Umnt") {
		onUnmount()
	}

	fs.Debugf(nil, "[NFS DEBUG] "+format, args...)
}

// Debug reroutes go-nfs Debug messages to Intercept
func (l *LogIntercepter) Debug(args ...interface{}) {
	l.Intercept(args...)
}

// Debugf reroutes go-nfs Debugf messages to Interceptf
func (l *LogIntercepter) Debugf(format string, args ...interface{}) {
	l.Interceptf(format, args...)
}

// Error reroutes go-nfs Error messages to Intercept
func (l *LogIntercepter) Error(args ...interface{}) {
	l.Intercept(args...)
}

// Errorf reroutes go-nfs Errorf messages to Interceptf
func (l *LogIntercepter) Errorf(format string, args ...interface{}) {
	l.Interceptf(format, args...)
}

// Fatal reroutes go-nfs Fatal messages to Intercept
func (l *LogIntercepter) Fatal(args ...interface{}) {
	l.Intercept(args...)
}

// Fatalf reroutes go-nfs Fatalf messages to Interceptf
func (l *LogIntercepter) Fatalf(format string, args ...interface{}) {
	l.Interceptf(format, args...)
}

// GetLevel returns the nfs.LogLevel
func (l *LogIntercepter) GetLevel() nfs.LogLevel {
	return l.Level
}

// Info reroutes go-nfs Info messages to Intercept
func (l *LogIntercepter) Info(args ...interface{}) {
	l.Intercept(args...)
}

// Infof reroutes go-nfs Infof messages to Interceptf
func (l *LogIntercepter) Infof(format string, args ...interface{}) {
	l.Interceptf(format, args...)
}

// Panic reroutes go-nfs Panic messages to Intercept
func (l *LogIntercepter) Panic(args ...interface{}) {
	l.Intercept(args...)
}

// Panicf reroutes go-nfs Panicf messages to Interceptf
func (l *LogIntercepter) Panicf(format string, args ...interface{}) {
	l.Interceptf(format, args...)
}

// ParseLevel parses the nfs.LogLevel
func (l *LogIntercepter) ParseLevel(level string) (nfs.LogLevel, error) {
	return nfs.Log.ParseLevel(level)
}

// Print reroutes go-nfs Print messages to Intercept
func (l *LogIntercepter) Print(args ...interface{}) {
	l.Intercept(args...)
}

// Printf reroutes go-nfs Printf messages to Intercept
func (l *LogIntercepter) Printf(format string, args ...interface{}) {
	l.Interceptf(format, args...)
}

// SetLevel sets the nfs.LogLevel
func (l *LogIntercepter) SetLevel(level nfs.LogLevel) {
	l.Level = level
}

// Trace reroutes go-nfs Trace messages to Intercept
func (l *LogIntercepter) Trace(args ...interface{}) {
	l.Intercept(args...)
}

// Tracef reroutes go-nfs Tracef messages to Interceptf
func (l *LogIntercepter) Tracef(format string, args ...interface{}) {
	l.Interceptf(format, args...)
}

// Warn reroutes go-nfs Warn messages to Intercept
func (l *LogIntercepter) Warn(args ...interface{}) {
	l.Intercept(args...)
}

// Warnf reroutes go-nfs Warnf messages to Interceptf
func (l *LogIntercepter) Warnf(format string, args ...interface{}) {
	l.Interceptf(format, args...)
}
