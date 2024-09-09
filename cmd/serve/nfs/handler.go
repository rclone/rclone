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
)

// Handler returns a NFS backing that exposes a given file system in response to all mount requests.
type Handler struct {
	vfs     *vfs.VFS
	opt     Options
	billyFS *FS
	Cache
}

// NewHandler creates a handler for the provided filesystem
func NewHandler(ctx context.Context, vfs *vfs.VFS, opt *Options) (handler nfs.Handler, err error) {
	ci := fs.GetConfig(ctx)
	h := &Handler{
		vfs:     vfs,
		opt:     *opt,
		billyFS: &FS{vfs: vfs},
	}
	h.opt.HandleLimit = h.opt.Limit()
	h.Cache, err = h.getCache()
	if err != nil {
		return nil, fmt.Errorf("failed to make cache: %w", err)
	}
	var level nfs.LogLevel
	switch {
	case ci.LogLevel >= fs.LogLevelDebug: // Debug level, needs -vv
		level = nfs.TraceLevel
	case ci.LogLevel >= fs.LogLevelInfo: // Transfers, needs -v
		level = nfs.InfoLevel
	case ci.LogLevel >= fs.LogLevelNotice: // Normal logging, -q suppresses
		level = nfs.WarnLevel
	case ci.LogLevel >= fs.LogLevelError: // Error - can't be suppressed
		level = nfs.ErrorLevel
	default:
		level = nfs.WarnLevel
	}
	nfs.SetLogger(&logger{level: level})
	return h, nil
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

// InvalidateHandle invalidates the handle passed - used on rename and delete
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

// logger handles go-nfs logs and reroutes them to rclone's logging system
type logger struct {
	level nfs.LogLevel
}

// logPrint intercepts go-nfs logs and calls rclone's log system instead
func (l *logger) logPrint(level fs.LogLevel, args ...interface{}) {
	fs.LogPrintf(level, "nfs", "%s", fmt.Sprint(args...))
}

// logPrintf intercepts go-nfs logs and calls rclone's log system instead
func (l *logger) logPrintf(level fs.LogLevel, format string, args ...interface{}) {
	fs.LogPrintf(level, "nfs", format, args...)
}

// Debug reroutes go-nfs Debug messages to Intercept
func (l *logger) Debug(args ...interface{}) {
	if l.level < nfs.DebugLevel {
		return
	}
	l.logPrint(fs.LogLevelDebug, args...)
}

// Debugf reroutes go-nfs Debugf messages to logPrintf
func (l *logger) Debugf(format string, args ...interface{}) {
	if l.level < nfs.DebugLevel {
		return
	}
	l.logPrintf(fs.LogLevelDebug, format, args...)
}

// Error reroutes go-nfs Error messages to Intercept
func (l *logger) Error(args ...interface{}) {
	if l.level < nfs.ErrorLevel {
		return
	}
	l.logPrint(fs.LogLevelError, args...)
}

// Errorf reroutes go-nfs Errorf messages to logPrintf
func (l *logger) Errorf(format string, args ...interface{}) {
	if l.level < nfs.ErrorLevel {
		return
	}
	l.logPrintf(fs.LogLevelError, format, args...)
}

// Fatal reroutes go-nfs Fatal messages to Intercept
func (l *logger) Fatal(args ...interface{}) {
	if l.level < nfs.FatalLevel {
		return
	}
	l.logPrint(fs.LogLevelError, args...)
}

// Fatalf reroutes go-nfs Fatalf messages to logPrintf
func (l *logger) Fatalf(format string, args ...interface{}) {
	if l.level < nfs.FatalLevel {
		return
	}
	l.logPrintf(fs.LogLevelError, format, args...)
}

// GetLevel returns the nfs.LogLevel
func (l *logger) GetLevel() nfs.LogLevel {
	return l.level
}

// Info reroutes go-nfs Info messages to Intercept
func (l *logger) Info(args ...interface{}) {
	if l.level < nfs.InfoLevel {
		return
	}
	l.logPrint(fs.LogLevelInfo, args...)
}

// Infof reroutes go-nfs Infof messages to logPrintf
func (l *logger) Infof(format string, args ...interface{}) {
	if l.level < nfs.InfoLevel {
		return
	}
	l.logPrintf(fs.LogLevelInfo, format, args...)
}

// Panic reroutes go-nfs Panic messages to Intercept
func (l *logger) Panic(args ...interface{}) {
	if l.level < nfs.PanicLevel {
		return
	}
	l.logPrint(fs.LogLevelError, args...)
}

// Panicf reroutes go-nfs Panicf messages to logPrintf
func (l *logger) Panicf(format string, args ...interface{}) {
	if l.level < nfs.PanicLevel {
		return
	}
	l.logPrintf(fs.LogLevelError, format, args...)
}

// ParseLevel parses the nfs.LogLevel
func (l *logger) ParseLevel(level string) (nfs.LogLevel, error) {
	return nfs.Log.ParseLevel(level)
}

// Print reroutes go-nfs Print messages to Intercept
func (l *logger) Print(args ...interface{}) {
	if l.level < nfs.InfoLevel {
		return
	}
	l.logPrint(fs.LogLevelInfo, args...)
}

// Printf reroutes go-nfs Printf messages to Intercept
func (l *logger) Printf(format string, args ...interface{}) {
	if l.level < nfs.InfoLevel {
		return
	}
	l.logPrintf(fs.LogLevelInfo, format, args...)
}

// SetLevel sets the nfs.LogLevel
func (l *logger) SetLevel(level nfs.LogLevel) {
	l.level = level
}

// Trace reroutes go-nfs Trace messages to Intercept
func (l *logger) Trace(args ...interface{}) {
	if l.level < nfs.DebugLevel {
		return
	}
	l.logPrint(fs.LogLevelDebug, args...)
}

// Tracef reroutes go-nfs Tracef messages to logPrintf
func (l *logger) Tracef(format string, args ...interface{}) {
	// FIXME BODGE ... the real fix is probably https://github.com/willscott/go-nfs/pull/28
	// This comes from `Log.Tracef("request: %v", w.req)` in conn.go
	// DEBUG : nfs: request: RPC #3285799202 (mount.Umnt)
	argsS := fmt.Sprint(args...)
	if strings.Contains(argsS, "mount.Umnt") {
		onUnmount()
	}
	if l.level < nfs.DebugLevel {
		return
	}
	l.logPrintf(fs.LogLevelDebug, format, args...)
}

// Warn reroutes go-nfs Warn messages to Intercept
func (l *logger) Warn(args ...interface{}) {
	if l.level < nfs.WarnLevel {
		return
	}
	l.logPrint(fs.LogLevelNotice, args...)
}

// Warnf reroutes go-nfs Warnf messages to logPrintf
func (l *logger) Warnf(format string, args ...interface{}) {
	if l.level < nfs.WarnLevel {
		return
	}
	l.logPrintf(fs.LogLevelNotice, format, args...)
}
