//go:build unix

package nfs

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/vfs"
	"github.com/willscott/go-nfs"
	nfshelper "github.com/willscott/go-nfs/helpers"
)

// Handler returns a NFS backing that exposes a given file system in response to all mount requests.
type Handler struct {
	vfs     *vfs.VFS
	opt     *Options
	billyFS *FS
	Cache
}

// NewHandler creates a handler for the provided filesystem
func NewHandler(vfs *vfs.VFS, opt *Options) (nfs.Handler, error) {
	handler := &Handler{
		vfs:     vfs,
		opt:     opt,
		billyFS: &FS{vfs: vfs},
	}
	handler.opt.HandleLimit = handler.opt.Limit()
	err := handler.setCache()
	if err != nil {
		return nil, fmt.Errorf("failed to make cache: %w", err)
	}
	handler.Cache = nfshelper.NewCachingHandler(handler, handler.opt.HandleLimit)
	nfs.SetLogger(&logIntercepter{Level: nfs.DebugLevel})
	return handler, nil
}

// Mount backs Mount RPC Requests, allowing for access control policies.
func (h *Handler) Mount(ctx context.Context, conn net.Conn, req nfs.MountRequest) (status nfs.MountStatus, hndl billy.Filesystem, auths []nfs.AuthFlavor) {
	auths = []nfs.AuthFlavor{nfs.AuthFlavorNull}
	return nfs.MountStatusOk, h.billyFS, auths
}

// Change provides an interface for updating file attributes.
func (h *Handler) Change(fs billy.Filesystem) billy.Change {
	if c, ok := fs.(billy.Change); ok {
		return c
	}
	return nil
}

// FSStat provides information about a filesystem.
func (h *Handler) FSStat(ctx context.Context, f billy.Filesystem, s *nfs.FSStat) error {
	total, _, free := h.vfs.Statfs()
	s.TotalSize = uint64(total)
	s.FreeSize = uint64(free)
	s.AvailableSize = uint64(free)
	return nil
}

// ToHandle takes a file and represents it with an opaque handle to reference it.
// In stateless nfs (when it's serving a unix fs) this can be the device + inode
// but we can generalize with a stateful local cache of handed out IDs.
func (h *Handler) ToHandle(f billy.Filesystem, s []string) (b []byte) {
	defer log.Trace("nfs", "path=%q", s)("handle=%X", &b)
	return h.Cache.ToHandle(f, s)
}

// FromHandle converts from an opaque handle to the file it represents
func (h *Handler) FromHandle(b []byte) (f billy.Filesystem, s []string, err error) {
	defer log.Trace("nfs", "handle=%X", b)("path=%q, err=%v", &s, &err)
	return h.Cache.FromHandle(b)
}

// HandleLimit exports how many file handles can be safely stored by this cache.
func (h *Handler) HandleLimit() int {
	return h.Cache.HandleLimit()
}

// Invalidate the handle passed - used on rename and delete
func (h *Handler) InvalidateHandle(f billy.Filesystem, b []byte) (err error) {
	defer log.Trace("nfs", "handle=%X", b)("err=%v", &err)
	return h.Cache.InvalidateHandle(f, b)
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

// logIntercepter intercepts noisy go-nfs logs and reroutes them to DEBUG
type logIntercepter struct {
	Level nfs.LogLevel
}

// Intercept intercepts go-nfs logs and calls fs.Debugf instead
func (l *logIntercepter) Intercept(args ...interface{}) {
	args = append([]interface{}{"[NFS DEBUG] "}, args...)
	argsS := fmt.Sprint(args...)
	fs.Debugf(nil, "%v", argsS)
}

// Interceptf intercepts go-nfs logs and calls fs.Debugf instead
func (l *logIntercepter) Interceptf(format string, args ...interface{}) {
	argsS := fmt.Sprint(args...)
	// bit of a workaround... the real fix is probably https://github.com/willscott/go-nfs/pull/28
	if strings.Contains(argsS, "mount.Umnt") {
		onUnmount()
	}

	fs.Debugf(nil, "[NFS DEBUG] "+format, args...)
}

// Debug reroutes go-nfs Debug messages to Intercept
func (l *logIntercepter) Debug(args ...interface{}) {
	l.Intercept(args...)
}

// Debugf reroutes go-nfs Debugf messages to Interceptf
func (l *logIntercepter) Debugf(format string, args ...interface{}) {
	l.Interceptf(format, args...)
}

// Error reroutes go-nfs Error messages to Intercept
func (l *logIntercepter) Error(args ...interface{}) {
	l.Intercept(args...)
}

// Errorf reroutes go-nfs Errorf messages to Interceptf
func (l *logIntercepter) Errorf(format string, args ...interface{}) {
	l.Interceptf(format, args...)
}

// Fatal reroutes go-nfs Fatal messages to Intercept
func (l *logIntercepter) Fatal(args ...interface{}) {
	l.Intercept(args...)
}

// Fatalf reroutes go-nfs Fatalf messages to Interceptf
func (l *logIntercepter) Fatalf(format string, args ...interface{}) {
	l.Interceptf(format, args...)
}

// GetLevel returns the nfs.LogLevel
func (l *logIntercepter) GetLevel() nfs.LogLevel {
	return l.Level
}

// Info reroutes go-nfs Info messages to Intercept
func (l *logIntercepter) Info(args ...interface{}) {
	l.Intercept(args...)
}

// Infof reroutes go-nfs Infof messages to Interceptf
func (l *logIntercepter) Infof(format string, args ...interface{}) {
	l.Interceptf(format, args...)
}

// Panic reroutes go-nfs Panic messages to Intercept
func (l *logIntercepter) Panic(args ...interface{}) {
	l.Intercept(args...)
}

// Panicf reroutes go-nfs Panicf messages to Interceptf
func (l *logIntercepter) Panicf(format string, args ...interface{}) {
	l.Interceptf(format, args...)
}

// ParseLevel parses the nfs.LogLevel
func (l *logIntercepter) ParseLevel(level string) (nfs.LogLevel, error) {
	return nfs.Log.ParseLevel(level)
}

// Print reroutes go-nfs Print messages to Intercept
func (l *logIntercepter) Print(args ...interface{}) {
	l.Intercept(args...)
}

// Printf reroutes go-nfs Printf messages to Intercept
func (l *logIntercepter) Printf(format string, args ...interface{}) {
	l.Interceptf(format, args...)
}

// SetLevel sets the nfs.LogLevel
func (l *logIntercepter) SetLevel(level nfs.LogLevel) {
	l.Level = level
}

// Trace reroutes go-nfs Trace messages to Intercept
func (l *logIntercepter) Trace(args ...interface{}) {
	l.Intercept(args...)
}

// Tracef reroutes go-nfs Tracef messages to Interceptf
func (l *logIntercepter) Tracef(format string, args ...interface{}) {
	l.Interceptf(format, args...)
}

// Warn reroutes go-nfs Warn messages to Intercept
func (l *logIntercepter) Warn(args ...interface{}) {
	l.Intercept(args...)
}

// Warnf reroutes go-nfs Warnf messages to Interceptf
func (l *logIntercepter) Warnf(format string, args ...interface{}) {
	l.Interceptf(format, args...)
}
