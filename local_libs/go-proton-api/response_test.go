package proton_test

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/rclone/go-proton-api"
	"github.com/rclone/go-proton-api/server"
	"github.com/stretchr/testify/require"
)

func TestNetError_DropOnWrite(t *testing.T) {
	l, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	dropListener := proton.NewListener(l, proton.NewDropConn)

	// Use a custom listener that drops all writes.
	dropListener.SetCanWrite(false)

	// Simulate a server that refuses to write.
	s := server.New(server.WithListener(dropListener))
	defer s.Close()

	m := proton.New(proton.WithHostURL(s.GetHostURL()))
	defer m.Close()

	// This should fail with a URL error.
	pingErr := m.Ping(context.Background())

	if urlErr := new(url.Error); !errors.As(pingErr, &urlErr) {
		t.Fatalf("expected a url.Error, got %T: %v", pingErr, pingErr)
	}
}

func TestAPIError_DeserializeWithoutDetails(t *testing.T) {
	errJson := `
{
	"Status": 400,
	"Code": 1000,
	"Error": "Foo Bar"
}
`
	var err proton.APIError

	require.NoError(t, json.Unmarshal([]byte(errJson), &err))
	require.Nil(t, err.Details)
}

func TestAPIError_DeserializeWithoutDetailsValue(t *testing.T) {
	errJson := `
{
	"Status": 400,
	"Code": 1000,
	"Error": "Foo Bar",
	"Details": 20
}
`
	var err proton.APIError

	require.NoError(t, json.Unmarshal([]byte(errJson), &err))
	require.NotNil(t, err.Details)
	require.Equal(t, `20`, err.DetailsToString())
}

func TestAPIError_DeserializeWithDetailsObject(t *testing.T) {
	errJson := `
{
	"Status": 400,
	"Code": 1000,
	"Error": "Foo Bar",
	"Details": {
		"object2": {
			"v": 20
		},
		"foo": "bar"
	}
}
`
	var err proton.APIError

	require.NoError(t, json.Unmarshal([]byte(errJson), &err))
	require.NotNil(t, err.Details)
	require.Equal(t, `{"foo":"bar","object2":{"v":20}}`, err.DetailsToString())
}

func TestAPIError_DeserializeWithDetailsArray(t *testing.T) {
	errJson := `
{
	"Status": 400,
	"Code": 1000,
	"Error": "Foo Bar",
	"Details": [
		{
			"object2": {
				"v": 20
			},
			"foo": "bar"
		},
		499,
		"hello"
	]
}
`
	var err proton.APIError

	require.NoError(t, json.Unmarshal([]byte(errJson), &err))
	require.NotNil(t, err.Details)
	require.Equal(t, `[{"foo":"bar","object2":{"v":20}},499,"hello"]`, err.DetailsToString())
}

func TestNetError_RouteInErrorMessage(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer s.Close()

	m := proton.New(proton.WithHostURL(s.URL))
	defer m.Close()

	pingErr := m.Quark(context.Background(), "test/ping")

	require.Error(t, pingErr)
	require.Contains(t, pingErr.Error(), "GET")
	require.Contains(t, pingErr.Error(), "/test/ping")
}
