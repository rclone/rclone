// Package s3 implements a fake s3 server for rclone
package s3

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rclone/gofakes3"
	"github.com/rclone/gofakes3/signature"
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/cmd/serve/proxy/proxyflags"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	httplib "github.com/rclone/rclone/lib/http"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
)

type ctxKey int

const (
	ctxKeyID ctxKey = iota
)

// Options contains options for the http Server
type Options struct {
	//TODO add more options
	pathBucketMode bool
	hashName       string
	hashType       hash.Type
	authPair       []string
	noCleanup      bool
	Auth           httplib.AuthConfig
	HTTP           httplib.Config
}

// Server is a s3.FileSystem interface
type Server struct {
	server   *httplib.Server
	f        fs.Fs
	_vfs     *vfs.VFS // don't use directly, use getVFS
	faker    *gofakes3.GoFakeS3
	handler  http.Handler
	proxy    *proxy.Proxy
	ctx      context.Context // for global config
	s3Secret string
}

// Make a new S3 Server to serve the remote
func newServer(ctx context.Context, f fs.Fs, opt *Options) (s *Server, err error) {
	w := &Server{
		f:   f,
		ctx: ctx,
	}

	if len(opt.authPair) == 0 {
		fs.Logf("serve s3", "No auth provided so allowing anonymous access")
	} else {
		w.s3Secret = getAuthSecret(opt.authPair)
	}

	var newLogger logger
	w.faker = gofakes3.New(
		newBackend(w, opt),
		gofakes3.WithHostBucket(!opt.pathBucketMode),
		gofakes3.WithLogger(newLogger),
		gofakes3.WithRequestID(rand.Uint64()),
		gofakes3.WithoutVersioning(),
		gofakes3.WithV4Auth(authlistResolver(opt.authPair)),
		gofakes3.WithIntegrityCheck(true), // Check Content-MD5 if supplied
	)

	w.handler = http.NewServeMux()
	w.handler = w.faker.Server()

	if proxyflags.Opt.AuthProxy != "" {
		w.proxy = proxy.New(ctx, &proxyflags.Opt)
		// proxy auth middleware
		w.handler = proxyAuthMiddleware(w.handler, w)
		w.handler = authPairMiddleware(w.handler, w)
	} else {
		w._vfs = vfs.New(f, &vfscommon.Opt)

		if len(opt.authPair) > 0 {
			w.faker.AddAuthKeys(authlistResolver(opt.authPair))
		}
	}

	w.server, err = httplib.NewServer(ctx,
		httplib.WithConfig(opt.HTTP),
		httplib.WithAuth(opt.Auth),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to init server: %w", err)
	}

	return w, nil
}

func (w *Server) getVFS(ctx context.Context) (VFS *vfs.VFS, err error) {
	if w._vfs != nil {
		return w._vfs, nil
	}

	value := ctx.Value(ctxKeyID)
	if value == nil {
		return nil, errors.New("no VFS found in context")
	}

	VFS, ok := value.(*vfs.VFS)
	if !ok {
		return nil, fmt.Errorf("context value is not VFS: %#v", value)
	}
	return VFS, nil
}

// auth does proxy authorization
func (w *Server) auth(accessKeyID string) (value interface{}, err error) {
	VFS, _, err := w.proxy.Call(stringToMd5Hash(accessKeyID), accessKeyID, false)
	if err != nil {
		return nil, err
	}
	return VFS, err
}

// Bind register the handler to http.Router
func (w *Server) Bind(router chi.Router) {
	router.Handle("/*", w.handler)
}

// Serve serves the s3 server
func (w *Server) Serve() error {
	w.server.Serve()
	fs.Logf(w.f, "Starting s3 server on %s", w.server.URLs())
	return nil
}

func authPairMiddleware(next http.Handler, ws *Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accessKey, _ := parseAccessKeyID(r)
		// set the auth pair
		authPair := map[string]string{
			accessKey: ws.s3Secret,
		}
		ws.faker.AddAuthKeys(authPair)
		next.ServeHTTP(w, r)
	})
}

func proxyAuthMiddleware(next http.Handler, ws *Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accessKey, _ := parseAccessKeyID(r)
		value, err := ws.auth(accessKey)
		if err != nil {
			fs.Infof(r.URL.Path, "%s: Auth failed: %v", r.RemoteAddr, err)
		}
		if value != nil {
			r = r.WithContext(context.WithValue(r.Context(), ctxKeyID, value))
		}

		next.ServeHTTP(w, r)
	})
}

func parseAccessKeyID(r *http.Request) (accessKey string, error signature.ErrorCode) {
	v4Auth := r.Header.Get("Authorization")
	req, err := signature.ParseSignV4(v4Auth)
	if err != signature.ErrNone {
		return "", err
	}

	return req.Credential.GetAccessKey(), signature.ErrNone
}

func stringToMd5Hash(s string) string {
	hasher := md5.New()
	hasher.Write([]byte(s))
	return hex.EncodeToString(hasher.Sum(nil))
}

func getAuthSecret(authPair []string) string {
	if len(authPair) == 0 {
		return ""
	}

	splited := strings.Split(authPair[0], ",")
	if len(splited) != 2 {
		return ""
	}

	secret := strings.TrimSpace(splited[1])
	return secret
}
