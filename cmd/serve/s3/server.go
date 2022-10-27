// Package s3 implements a fake s3 server for rclone
package s3

import (
	"context"
	"math/rand"
	"net/http"

	"github.com/Mikubill/gofakes3"
	"github.com/go-chi/chi/v5"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/http/auth"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfsflags"
)

// Options contains options for the http Server
type Options struct {
	//TODO add more options
	hostBucketMode bool
	hashName       string
	hashType       hash.Type
	authPair       []string
	noCleanup      bool
}

// Server is a s3.FileSystem interface
type Server struct {
	f       fs.Fs
	vfs     *vfs.VFS
	faker   *gofakes3.GoFakeS3
	handler http.Handler
	ctx     context.Context // for global config
}

// Make a new S3 Server to serve the remote
func newServer(ctx context.Context, f fs.Fs, opt *Options) *Server {
	w := &Server{
		f:   f,
		ctx: ctx,
		vfs: vfs.New(f, &vfsflags.Opt),
	}

	var newLogger logger
	w.faker = gofakes3.New(
		newBackend(w.vfs, opt),
		gofakes3.WithHostBucket(opt.hostBucketMode),
		gofakes3.WithLogger(newLogger),
		gofakes3.WithRequestID(rand.Uint64()),
		gofakes3.WithoutVersioning(),
		gofakes3.WithV4Auth(authlistResolver(opt.authPair)),
	)

	w.handler = w.faker.Server()
	// router.Route("/*", w.handler)
	// w.Server = httplib.NewServer(w.handler, &httpflags.Opt)
	return w
}

// Bind register the handler to http.Router
func (w *Server) Bind(router chi.Router) {
	if m := auth.Auth(auth.Opt); m != nil {
		router.Use(m)
	}

	router.Handle("/*", w.handler)
}
