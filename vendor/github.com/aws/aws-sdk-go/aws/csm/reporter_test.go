package csm_test

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/client/metadata"
	"github.com/aws/aws-sdk-go/aws/csm"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/aws/aws-sdk-go/private/protocol/jsonrpc"
)

func startUDPServer(done chan struct{}, fn func([]byte)) (string, error) {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return "", err
	}

	buf := make([]byte, 1024)
	i := 0
	go func() {
		defer conn.Close()
		for {
			i++
			select {
			case <-done:
				return
			default:
			}

			n, _, err := conn.ReadFromUDP(buf)
			fn(buf[:n])

			if err != nil {
				panic(err)
			}
		}
	}()

	return conn.LocalAddr().String(), nil
}

func TestReportingMetrics(t *testing.T) {
	reporter := csm.Get()
	if reporter == nil {
		t.Errorf("expected non-nil reporter")
	}

	sess := session.New()
	sess.Handlers.Clear()
	reporter.InjectHandlers(&sess.Handlers)

	md := metadata.ClientInfo{}
	op := &request.Operation{}
	r := request.New(*sess.Config, md, sess.Handlers, client.DefaultRetryer{NumMaxRetries: 0}, op, nil, nil)
	sess.Handlers.Complete.Run(r)
	m := <-csm.MetricsCh

	for k, v := range m {
		switch k {
		case "Timestamp":
			if _, ok := v.(float64); !ok {
				t.Errorf("expected a float value, but received %T", v)
			}
		case "Type":
			if e, a := "ApiCall", v.(string); e != a {
				t.Errorf("expected %q, but received %q", e, a)
			}
		}
	}
}

type mockService struct {
	*client.Client
}

type input struct{}
type output struct{}

func (s *mockService) Request(i input) *request.Request {
	op := &request.Operation{
		Name:       "foo",
		HTTPMethod: "POST",
		HTTPPath:   "/",
	}

	o := output{}
	req := s.NewRequest(op, &i, &o)
	return req
}

func BenchmarkWithCSM(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fmt.Sprintf("{}")))
	}))

	cfg := aws.Config{
		Endpoint: aws.String(server.URL),
	}

	sess := session.New(&cfg)
	r := csm.Get()

	r.InjectHandlers(&sess.Handlers)

	c := sess.ClientConfig("id", &cfg)

	svc := mockService{
		client.New(
			*c.Config,
			metadata.ClientInfo{
				ServiceName:   "service",
				ServiceID:     "id",
				SigningName:   "signing",
				SigningRegion: "region",
				Endpoint:      server.URL,
				APIVersion:    "0",
				JSONVersion:   "1.1",
				TargetPrefix:  "prefix",
			},
			c.Handlers,
		),
	}

	svc.Handlers.Sign.PushBackNamed(v4.SignRequestHandler)
	svc.Handlers.Build.PushBackNamed(jsonrpc.BuildHandler)
	svc.Handlers.Unmarshal.PushBackNamed(jsonrpc.UnmarshalHandler)
	svc.Handlers.UnmarshalMeta.PushBackNamed(jsonrpc.UnmarshalMetaHandler)
	svc.Handlers.UnmarshalError.PushBackNamed(jsonrpc.UnmarshalErrorHandler)

	for i := 0; i < b.N; i++ {
		req := svc.Request(input{})
		req.Send()
	}
}

func BenchmarkWithCSMNoUDPConnection(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fmt.Sprintf("{}")))
	}))

	cfg := aws.Config{
		Endpoint: aws.String(server.URL),
	}

	sess := session.New(&cfg)
	r := csm.Get()
	r.Pause()
	r.InjectHandlers(&sess.Handlers)
	defer r.Pause()

	c := sess.ClientConfig("id", &cfg)

	svc := mockService{
		client.New(
			*c.Config,
			metadata.ClientInfo{
				ServiceName:   "service",
				ServiceID:     "id",
				SigningName:   "signing",
				SigningRegion: "region",
				Endpoint:      server.URL,
				APIVersion:    "0",
				JSONVersion:   "1.1",
				TargetPrefix:  "prefix",
			},
			c.Handlers,
		),
	}

	svc.Handlers.Sign.PushBackNamed(v4.SignRequestHandler)
	svc.Handlers.Build.PushBackNamed(jsonrpc.BuildHandler)
	svc.Handlers.Unmarshal.PushBackNamed(jsonrpc.UnmarshalHandler)
	svc.Handlers.UnmarshalMeta.PushBackNamed(jsonrpc.UnmarshalMetaHandler)
	svc.Handlers.UnmarshalError.PushBackNamed(jsonrpc.UnmarshalErrorHandler)

	for i := 0; i < b.N; i++ {
		req := svc.Request(input{})
		req.Send()
	}
}

func BenchmarkWithoutCSM(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fmt.Sprintf("{}")))
	}))

	cfg := aws.Config{
		Endpoint: aws.String(server.URL),
	}
	sess := session.New(&cfg)
	c := sess.ClientConfig("id", &cfg)

	svc := mockService{
		client.New(
			*c.Config,
			metadata.ClientInfo{
				ServiceName:   "service",
				ServiceID:     "id",
				SigningName:   "signing",
				SigningRegion: "region",
				Endpoint:      server.URL,
				APIVersion:    "0",
				JSONVersion:   "1.1",
				TargetPrefix:  "prefix",
			},
			c.Handlers,
		),
	}

	svc.Handlers.Sign.PushBackNamed(v4.SignRequestHandler)
	svc.Handlers.Build.PushBackNamed(jsonrpc.BuildHandler)
	svc.Handlers.Unmarshal.PushBackNamed(jsonrpc.UnmarshalHandler)
	svc.Handlers.UnmarshalMeta.PushBackNamed(jsonrpc.UnmarshalMetaHandler)
	svc.Handlers.UnmarshalError.PushBackNamed(jsonrpc.UnmarshalErrorHandler)

	for i := 0; i < b.N; i++ {
		req := svc.Request(input{})
		req.Send()
	}
}
