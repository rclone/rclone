package http

import (
	"net"
	"net/http"
	"reflect"
	"testing"

	"golang.org/x/net/nettest"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetOptions(t *testing.T) {
	tests := []struct {
		name string
		want Options
	}{
		{name: "basic", want: defaultServerOptions},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetOptions(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetOptions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMount(t *testing.T) {
	type args struct {
		pattern string
		h       http.Handler
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{name: "basic", args: args{
			pattern: "/",
			h:       http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {}),
		}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr {
				require.Error(t, Mount(tt.args.pattern, tt.args.h))
			} else {
				require.NoError(t, Mount(tt.args.pattern, tt.args.h))
			}
			assert.NotNil(t, defaultServer)
			assert.True(t, defaultServer.baseRouter.Match(chi.NewRouteContext(), "GET", tt.args.pattern), "Failed to match route after registering")
		})
		if err := Shutdown(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNewServer(t *testing.T) {
	type args struct {
		listeners    []net.Listener
		tlsListeners []net.Listener
		opt          Options
	}
	listener, err := nettest.NewLocalListener("tcp")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{name: "default http", args: args{
			listeners:    []net.Listener{listener},
			tlsListeners: []net.Listener{},
			opt:          defaultServerOptions,
		}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewServer(tt.args.listeners, tt.args.tlsListeners, tt.args.opt)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewServer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			s, ok := got.(*server)
			require.True(t, ok, "NewServer returned unexpected type")
			if len(tt.args.listeners) > 0 {
				assert.Equal(t, listener.Addr(), s.addrs[0])
			} else {
				assert.Empty(t, s.addrs)
			}
			if len(tt.args.tlsListeners) > 0 {
				assert.Equal(t, listener.Addr(), s.tlsAddrs[0])
			} else {
				assert.Empty(t, s.tlsAddrs)
			}
			if tt.args.opt.BaseURL != "" {
				assert.NotSame(t, s.baseRouter, s.httpServer.Handler, "should have wrapped baseRouter")
			} else {
				assert.Same(t, s.baseRouter, s.httpServer.Handler, "should be baseRouter")
			}
			if useSSL(tt.args.opt) {
				assert.NotNil(t, s.httpServer.TLSConfig, "missing SSL config")
			} else {
				assert.Nil(t, s.httpServer.TLSConfig, "unexpectedly has SSL config")
			}
		})
	}
}

func TestRestart(t *testing.T) {
	tests := []struct {
		name    string
		started bool
		wantErr bool
	}{
		{name: "started", started: true, wantErr: false},
		{name: "stopped", started: false, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.started {
				require.NoError(t, Restart()) // Call it twice basically
			} else {
				require.NoError(t, Shutdown())
			}
			current := defaultServer
			if err := Restart(); (err != nil) != tt.wantErr {
				t.Errorf("Restart() error = %v, wantErr %v", err, tt.wantErr)
			}
			assert.NotNil(t, defaultServer, "failed to start default server")
			assert.NotSame(t, current, defaultServer, "same server instance as before restart")
		})
	}
}

func TestRoute(t *testing.T) {
	type args struct {
		pattern string
		fn      func(r chi.Router)
	}
	tests := []struct {
		name string
		args args
		test func(t *testing.T, r chi.Router)
	}{
		{
			name: "basic",
			args: args{
				pattern: "/basic",
				fn:      func(r chi.Router) {},
			},
			test: func(t *testing.T, r chi.Router) {
				require.Len(t, r.Routes(), 1)
				assert.Equal(t, r.Routes()[0].Pattern, "/basic/*")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, Restart())
			_, err := Route(tt.args.pattern, tt.args.fn)
			require.NoError(t, err)
			tt.test(t, defaultServer.baseRouter)
		})

		if err := Shutdown(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSetOptions(t *testing.T) {
	type args struct {
		opt Options
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "basic",
			args: args{opt: Options{
				ListenAddr:         "127.0.0.1:9999",
				BaseURL:            "/basic",
				ServerReadTimeout:  1,
				ServerWriteTimeout: 1,
				MaxHeaderBytes:     1,
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetOptions(tt.args.opt)
			require.Equal(t, tt.args.opt, defaultServerOptions)
			require.NoError(t, Restart())
			if useSSL(tt.args.opt) {
				assert.Equal(t, tt.args.opt.ListenAddr, defaultServer.tlsAddrs[0].String())
			} else {
				assert.Equal(t, tt.args.opt.ListenAddr, defaultServer.addrs[0].String())
			}
			assert.Equal(t, tt.args.opt.ServerReadTimeout, defaultServer.httpServer.ReadTimeout)
			assert.Equal(t, tt.args.opt.ServerWriteTimeout, defaultServer.httpServer.WriteTimeout)
			assert.Equal(t, tt.args.opt.MaxHeaderBytes, defaultServer.httpServer.MaxHeaderBytes)
			if tt.args.opt.BaseURL != "" && tt.args.opt.BaseURL != "/" {
				assert.NotSame(t, defaultServer.httpServer.Handler, defaultServer.baseRouter, "BaseURL ignored")
			}
		})
		SetOptions(DefaultOpt)
	}
}

func TestShutdown(t *testing.T) {
	tests := []struct {
		name    string
		started bool
		wantErr bool
	}{
		{name: "started", started: true, wantErr: false},
		{name: "stopped", started: false, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.started {
				require.NoError(t, Restart())
			} else {
				require.NoError(t, Shutdown()) // Call it twice basically
			}
			if err := Shutdown(); (err != nil) != tt.wantErr {
				t.Errorf("Shutdown() error = %v, wantErr %v", err, tt.wantErr)
			}
			assert.Nil(t, defaultServer, "default server not deleted")
		})
	}
}

func TestURL(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{name: "basic", want: "http://127.0.0.1:8080/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, Restart())
			if got := URL(); got != tt.want {
				t.Errorf("URL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_server_Mount(t *testing.T) {
	type args struct {
		pattern string
		h       http.Handler
	}
	tests := []struct {
		name string
		args args
		opt  Options
	}{
		{name: "basic", args: args{
			pattern: "/",
			h:       http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {}),
		}, opt: defaultServerOptions},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listener, err := nettest.NewLocalListener("tcp")
			require.NoError(t, err)
			s, err2 := NewServer([]net.Listener{listener}, []net.Listener{}, tt.opt)
			require.NoError(t, err2)
			s.Mount(tt.args.pattern, tt.args.h)
			srv, ok := s.(*server)
			require.True(t, ok)
			assert.NotNil(t, srv)
			assert.True(t, srv.baseRouter.Match(chi.NewRouteContext(), "GET", tt.args.pattern), "Failed to Match() route after registering")
		})
	}
}

func Test_server_Route(t *testing.T) {
	type args struct {
		pattern string
		fn      func(r chi.Router)
	}
	tests := []struct {
		name string
		args args
		opt  Options
		test func(t *testing.T, r chi.Router)
	}{
		{
			name: "basic",
			args: args{
				pattern: "/basic",
				fn: func(r chi.Router) {

				},
			},
			test: func(t *testing.T, r chi.Router) {
				require.Len(t, r.Routes(), 1)
				assert.Equal(t, r.Routes()[0].Pattern, "/basic/*")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listener, err := nettest.NewLocalListener("tcp")
			require.NoError(t, err)
			s, err2 := NewServer([]net.Listener{listener}, []net.Listener{}, tt.opt)
			require.NoError(t, err2)
			s.Route(tt.args.pattern, tt.args.fn)
			srv, ok := s.(*server)
			require.True(t, ok)
			assert.NotNil(t, srv)
			tt.test(t, srv.baseRouter)
		})
	}
}

func Test_server_Shutdown(t *testing.T) {
	tests := []struct {
		name    string
		opt     Options
		wantErr bool
	}{
		{
			name:    "basic",
			opt:     defaultServerOptions,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listener, err := nettest.NewLocalListener("tcp")
			require.NoError(t, err)
			s, err2 := NewServer([]net.Listener{listener}, []net.Listener{}, tt.opt)
			require.NoError(t, err2)
			srv, ok := s.(*server)
			require.True(t, ok)
			if err := s.Shutdown(); (err != nil) != tt.wantErr {
				t.Errorf("Shutdown() error = %v, wantErr %v", err, tt.wantErr)
			}
			assert.EqualError(t, srv.httpServer.Serve(listener), http.ErrServerClosed.Error())
		})
	}
}

func Test_start(t *testing.T) {
	tests := []struct {
		name    string
		opt     Options
		wantErr bool
	}{
		{
			name:    "basic",
			opt:     defaultServerOptions,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetOptions(tt.opt)
			if err := start(); (err != nil) != tt.wantErr {
				t.Errorf("start() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			s := defaultServer
			if useSSL(tt.opt) {
				assert.Equal(t, tt.opt.ListenAddr, s.tlsAddrs[0].String())
			} else {
				assert.Equal(t, tt.opt.ListenAddr, s.addrs[0].String())
			}
			/* accessing s.httpServer.* can't be done synchronously and is a race condition
			assert.Equal(t, tt.opt.ServerReadTimeout, defaultServer.httpServer.ReadTimeout)
			assert.Equal(t, tt.opt.ServerWriteTimeout, defaultServer.httpServer.WriteTimeout)
			assert.Equal(t, tt.opt.MaxHeaderBytes, defaultServer.httpServer.MaxHeaderBytes)
			if tt.opt.BaseURL != "" && tt.opt.BaseURL != "/" {
				assert.NotSame(t, s.baseRouter, s.httpServer.Handler, "should have wrapped baseRouter")
			} else {
				assert.Same(t, s.baseRouter, s.httpServer.Handler, "should be baseRouter")
			}
			if useSSL(tt.opt) {
				require.NotNil(t, s.httpServer.TLSConfig, "missing SSL config")
				assert.NotEmpty(t, s.httpServer.TLSConfig.Certificates, "missing SSL config")
			} else if s.httpServer.TLSConfig != nil {
				assert.Empty(t, s.httpServer.TLSConfig.Certificates, "unexpectedly has SSL config")
			}
			*/
		})
	}
}

func Test_useSSL(t *testing.T) {
	type args struct {
		opt Options
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "basic",
			args: args{opt: Options{
				SslCert:  "",
				SslKey:   "",
				ClientCA: "",
			}},
			want: false,
		},
		{
			name: "basic",
			args: args{opt: Options{
				SslCert:  "",
				SslKey:   "test",
				ClientCA: "",
			}},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := useSSL(tt.args.opt); got != tt.want {
				t.Errorf("useSSL() = %v, want %v", got, tt.want)
			}
		})
	}
}
