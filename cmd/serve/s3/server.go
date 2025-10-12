// Package s3 implements a fake s3 server for rclone
package s3

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rclone/gofakes3"
	"github.com/rclone/gofakes3/signature"
	"github.com/rclone/rclone/cmd/serve/proxy"
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

// Server is a s3.FileSystem interface
type Server struct {
	server       *httplib.Server
	opt          Options
	f            fs.Fs
	_vfs         *vfs.VFS // don't use directly, use getVFS
	faker        *gofakes3.GoFakeS3
	handler      http.Handler
	proxy        *proxy.Proxy
	ctx          context.Context // for global config
	s3Secret     string
	etagHashType hash.Type
}

// Make a new S3 Server to serve the remote
func newServer(ctx context.Context, f fs.Fs, opt *Options, vfsOpt *vfscommon.Options, proxyOpt *proxy.Options) (s *Server, err error) {
	w := &Server{
		f:            f,
		ctx:          ctx,
		opt:          *opt,
		etagHashType: hash.None,
	}

	if w.opt.EtagHash == "auto" {
		w.etagHashType = f.Hashes().GetOne()
	} else if w.opt.EtagHash != "" {
		err := w.etagHashType.Set(w.opt.EtagHash)
		if err != nil {
			return nil, err
		}
	}
	if w.etagHashType != hash.None {
		fs.Debugf(f, "Using hash %v for ETag", w.etagHashType)
	}

	if len(opt.AuthKey) == 0 {
		fs.Logf("serve s3", "No auth provided so allowing anonymous access")
	} else {
		w.s3Secret = getAuthSecret(opt.AuthKey)
	}

	var newLogger logger
	w.faker = gofakes3.New(
		newBackend(w),
		gofakes3.WithHostBucket(!opt.ForcePathStyle),
		gofakes3.WithLogger(newLogger),
		gofakes3.WithRequestID(rand.Uint64()),
		gofakes3.WithoutVersioning(),
		gofakes3.WithV4Auth(authlistResolver(opt.AuthKey)),
		gofakes3.WithIntegrityCheck(true), // Check Content-MD5 if supplied
	)

	w.handler = w.faker.Server()

	if proxy.Opt.AuthProxy != "" {
		w.proxy = proxy.New(ctx, proxyOpt, vfsOpt)
		// proxy auth middleware
		w.handler = proxyAuthMiddleware(w.handler, w)
		w.handler = authPairMiddleware(w.handler, w)
	} else {
		w._vfs = vfs.New(f, vfsOpt)

		if len(opt.AuthKey) > 0 {
			w.faker.AddAuthKeys(authlistResolver(opt.AuthKey))
		}
	}

	w.server, err = httplib.NewServer(ctx,
		httplib.WithConfig(opt.HTTP),
		httplib.WithAuth(opt.Auth),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to init server: %w", err)
	}

	router := w.server.Router()
	w.Bind(router)

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
func (w *Server) auth(accessKeyID string) (value any, err error) {
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

// Serve serves the s3 server until the server is shutdown
func (w *Server) Serve() error {
	w.server.Serve()
	fs.Logf(w.f, "Starting s3 server on %s", w.server.URLs())
	w.server.Wait()
	return nil
}

// Addr returns the first address of the server
func (w *Server) Addr() net.Addr {
	return w.server.Addr()
}

// Shutdown the server
func (w *Server) Shutdown() error {
	return w.server.Shutdown()
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
