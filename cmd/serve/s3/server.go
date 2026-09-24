// Package s3 implements a fake s3 server for rclone
package s3

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"time"

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
	provider     *proxy.Provider
	faker        *gofakes3.GoFakeS3
	backend      *s3Backend
	handler      http.Handler
	ctx          context.Context // for global config
	etagHashType hash.Type
}

// Make a new S3 Server to serve the remote
func newServer(ctx context.Context, f fs.Fs, opt *Options, vfsOpt *vfscommon.Options, proxyOpt *proxy.Options) (s *Server, err error) {
	w := &Server{
		f:            f,
		ctx:          ctx,
		opt:          *opt,
		provider:     proxy.NewProvider(ctx, f, vfsOpt, proxyOpt),
		etagHashType: hash.None,
	}
	defer func() {
		if err != nil {
			if w.backend != nil {
				w.backend.stopReaper()
			}
			w.provider.Shutdown()
		}
	}()

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

	if len(opt.AuthKey) == 0 && !w.provider.IsProxy() {
		fs.Logf("serve s3", "No auth provided so allowing anonymous access")
	}

	authList, err := authlistResolver(opt.AuthKey)
	if err != nil {
		return nil, fmt.Errorf("parsing auth list failed: %q", err)
	}
	if w.provider.IsProxy() {
		// The proxy middleware authenticates every request itself so
		// gofakes3's own auth must be left empty.
		if len(authList) > 0 {
			fs.Logf("serve s3", "--auth-key is ignored when --auth-proxy is set - the proxy must supply the secret for each access key ID")
		}
		authList = nil
	}

	w.backend = newBackend(w)
	if w.opt.MultipartExpiry > 0 {
		w.backend.startReaper(time.Duration(w.opt.MultipartExpiry))
	}

	var newLogger logger
	w.faker = gofakes3.New(
		w.backend,
		gofakes3.WithHostBucket(!opt.ForcePathStyle),
		gofakes3.WithLogger(newLogger),
		gofakes3.WithRequestID(rand.Uint64()),
		gofakes3.WithoutVersioning(),
		gofakes3.WithV4Auth(authList),
		gofakes3.WithIntegrityCheck(true), // Check Content-MD5 if supplied
	)

	w.handler = w.faker.Server()

	if w.provider.IsProxy() {
		w.handler = proxyAuthMiddleware(w.handler, w)
	} else if len(opt.AuthKey) > 0 {
		w.faker.AddAuthKeys(authList)
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
	if w.provider.VFS() != nil {
		return w.provider.VFS(), nil
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

// auth authenticates the request via the auth proxy.
//
// The proxy maps the access key ID to a VFS and a secret access key
// and the request's signature is verified against that secret. If it
// fails against a cached secret the proxy is consulted again in case
// the secret has been rotated.
//
// The secret is only ever used here and is never registered with
// gofakes3, whose key store is shared by every instance in the
// process.
func (w *Server) auth(r *http.Request, accessKeyID string) (VFS *vfs.VFS, err error) {
	p := w.provider.Proxy()
	VFS, secret, err := p.CallAccessKey(accessKeyID, r.RemoteAddr, false)
	if err != nil {
		return nil, err
	}
	errCode := signature.V4SignVerifyWithSecret(r, secret)
	if errCode == signature.ErrNone {
		return VFS, nil
	}
	// Only a signature mismatch can be cured by a rotated secret, so
	// only then is the proxy worth consulting again - other failures
	// (expired request, bad date, missing headers) must not make
	// every bad request run the proxy.
	if signature.GetAPIError(errCode).Code == "SignatureDoesNotMatch" {
		VFS, secret, err = p.CallAccessKey(accessKeyID, r.RemoteAddr, true)
		if err != nil {
			return nil, err
		}
		errCode = signature.V4SignVerifyWithSecret(r, secret)
		if errCode == signature.ErrNone {
			return VFS, nil
		}
	}
	return nil, fmt.Errorf("signature verification failed: %s", signature.GetAPIError(errCode).Code)
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
	w.backend.stopReaper()
	err := w.server.Shutdown()
	w.provider.Shutdown()
	return err
}

// proxyAuthMiddleware authenticates each request via the auth proxy,
// storing the VFS it returns in the request context, and refuses
// requests the proxy does not accept.
func proxyAuthMiddleware(next http.Handler, ws *Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accessKey, errCode := parseAccessKeyID(r)
		if errCode != signature.ErrNone {
			fs.Infof(r.URL.Path, "%s: Auth failed: no access key ID in request", r.RemoteAddr)
			accessDenied(w)
			return
		}
		VFS, err := ws.auth(r, accessKey)
		if err != nil {
			fs.Infof(r.URL.Path, "%s: Auth failed: %v", r.RemoteAddr, err)
			accessDenied(w)
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), ctxKeyID, VFS))
		next.ServeHTTP(w, r)
	})
}

// accessDenied writes an S3 AccessDenied error response
func accessDenied(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>AccessDenied</Code><Message>Access Denied</Message></Error>`))
}

// parseAccessKeyID returns the access key ID from the request's
// Authorization header or presigned URL query parameters
func parseAccessKeyID(r *http.Request) (accessKey string, errCode signature.ErrorCode) {
	v4Auth := r.Header.Get("Authorization")
	if v4Auth == "" {
		// Presigned URLs carry the credential in the query string
		q := r.URL.Query()
		if q.Get("X-Amz-Signature") != "" {
			v4Auth = fmt.Sprintf("%s Credential=%s, SignedHeaders=%s, Signature=%s", q.Get("X-Amz-Algorithm"), q.Get("X-Amz-Credential"), q.Get("X-Amz-SignedHeaders"), q.Get("X-Amz-Signature"))
		}
	}
	req, errCode := signature.ParseSignV4(v4Auth)
	if errCode != signature.ErrNone {
		return "", errCode
	}
	return req.Credential.GetAccessKey(), signature.ErrNone
}
