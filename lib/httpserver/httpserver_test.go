package httpserver_test

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"testing"

	"github.com/ncw/rclone/lib/httpserver"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestGetRequest(t *testing.T) {
	err := httpserver.Register("TestRemote/Notify", func(params map[string]string, writer io.Writer, reader io.Reader) error {
		return httpserver.WriteJSONMap(writer, map[string]interface{}{"message": "get ok"})
	})
	require.NoError(t, err)

	m, err := getRequest("TestRemote/Notify")
	require.NoError(t, err)
	require.Equal(t, "get ok", m["message"])
}

func TestPostRequest(t *testing.T) {
	err := httpserver.Register("TestRemote/Notify2", func(params map[string]string, writer io.Writer, reader io.Reader) error {
		m := make(map[string]interface{})
		data, err := ioutil.ReadAll(reader)
		if err != nil {
			return err
		}
		if len(data) > 0 {
			err := json.Unmarshal(data, &m)
			if err != nil {
				return err
			}
			return httpserver.WriteJSONMap(writer, m)
		}
		return httpserver.WriteJSONMap(writer, map[string]interface{}{"message": "not ok"})
	})
	require.NoError(t, err)

	m, err := postRequest("TestRemote/Notify2", "{\"message\": \"post ok\"}")
	require.NoError(t, err)
	require.Equal(t, "post ok", m["message"])

	m, err = getRequest("TestRemote/Notify")
	require.NoError(t, err)
	require.Equal(t, "get ok", m["message"])
}

func TestSupports(t *testing.T) {
	url := "http://localhost:8083/"
	log.Printf("Requesting: %v", url)
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer func() {
		_ = resp.Body.Close()
	}()
	sr := &httpserver.SupportsResponse{}
	err = json.NewDecoder(resp.Body).Decode(&sr)
	require.NoError(t, err)
	require.ElementsMatch(t, sr.Routes, []string{"/", "/TestRemote/Notify", "/TestRemote/Notify2"})
}

func TestOverrideIgnoredHandler(t *testing.T) {
	err := httpserver.Register("TestRemote/Override", func(params map[string]string, writer io.Writer, reader io.Reader) error {
		return httpserver.WriteJSONMap(writer, map[string]interface{}{"message": "override1"})
	})
	require.NoError(t, err)

	err = httpserver.Register("TestRemote/Override", func(params map[string]string, writer io.Writer, reader io.Reader) error {
		return httpserver.WriteJSONMap(writer, map[string]interface{}{"message": "override2"})
	})
	require.NoError(t, err)

	m, err := getRequest("TestRemote/Override")
	require.NoError(t, err)
	require.Equal(t, "override1", m["message"])
}

func TestParams(t *testing.T) {
	err := httpserver.Register("TestRemote/Params", func(params map[string]string, writer io.Writer, reader io.Reader) error {
		m, ok := params["message"]
		if !ok {
			return errors.New("message param doesn't exist")
		}

		return httpserver.WriteJSONMap(writer, map[string]interface{}{"message": m})
	})
	require.NoError(t, err)

	m, err := getRequest("TestRemote/Params?message=params")
	require.NoError(t, err)
	require.Equal(t, "params", m["message"])
}

func TestError(t *testing.T) {
	err := httpserver.Register("TestRemote/Error", func(params map[string]string, writer io.Writer, reader io.Reader) error {
		return errors.New("expected error")
	})
	require.NoError(t, err)

	url := "http://localhost:8083/TestRemote/Error"
	log.Printf("Requesting: %v", url)
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer func() {
		_ = resp.Body.Close()
	}()
	data := make(map[string]string)
	err = json.NewDecoder(resp.Body).Decode(&data)

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.Equal(t, "expected error", data["message"])
	require.Equal(t, "error", data["status"])
}

func getRequest(name string) (map[string]string, error) {
	m := make(map[string]string)
	url := "http://localhost:8083/" + name
	log.Printf("Requesting: %v", url)
	resp, err := http.Get(url)
	if err != nil {
		return m, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	err = json.NewDecoder(resp.Body).Decode(&m)
	return m, err
}

func postRequest(name string, data string) (map[string]string, error) {
	m := make(map[string]string)
	url := "http://localhost:8083/" + name
	log.Printf("Requesting: %v", url)
	resp, err := http.Post(url, "application/json", strings.NewReader(data))
	if err != nil {
		return m, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	err = json.NewDecoder(resp.Body).Decode(&m)
	return m, err
}
