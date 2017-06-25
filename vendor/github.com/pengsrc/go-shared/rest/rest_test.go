package rest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBuildURL(t *testing.T) {
	testURL := AddQueryParameters(
		"http://api.test.com",
		map[string]string{
			"test":  "1",
			"test2": "2",
		},
	)
	assert.Equal(t, "http://api.test.com?test=1&test2=2", testURL)
}

func TestBuildRequest(t *testing.T) {
	request := Request{
		Method:  Get,
		BaseURL: "http://api.test.com",
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"Authorization": "Bearer APK_KEY",
		},
		QueryParams: map[string]string{
			"test":  "1",
			"test2": "2",
		},
	}
	req, err := BuildRequestObject(&request)
	assert.NoError(t, err)
	assert.NotNil(t, req)
}

func TestBuildResponse(t *testing.T) {
	fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/not+json")
		fmt.Fprintln(w, "{\"message\": \"success\"}")
	}))
	defer fakeServer.Close()

	request := Request{
		Method:  Get,
		BaseURL: fakeServer.URL,
	}
	req, err := BuildRequestObject(&request)
	assert.NoError(t, err)

	res, err := MakeRequest(req)
	assert.NoError(t, err)

	response, err := BuildResponse(res)
	assert.NoError(t, err)
	err = response.ParseJSON()
	assert.Error(t, err)

	assert.Equal(t, 200, response.StatusCode)
	assert.NotEqual(t, 0, len(response.Body))
	assert.NotEqual(t, 0, len(response.Headers))
	assert.Nil(t, response.JSON)
}

func TestRest(t *testing.T) {
	fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, "{\"message\": \"success\"}")
	}))
	defer fakeServer.Close()

	request := Request{
		Method:  Get,
		BaseURL: fakeServer.URL + "/test_endpoint",
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"Authorization": "Bearer APK_KEY",
		},
		QueryParams: map[string]string{
			"test":  "1",
			"test2": "2",
		},
	}
	response, err := API(&request)
	assert.NoError(t, err)
	err = response.ParseJSON()
	assert.NoError(t, err)

	assert.Equal(t, 200, response.StatusCode)
	assert.NotEqual(t, 0, len(response.Body))
	assert.NotEqual(t, 0, len(response.Headers))
	assert.Equal(t, "success", response.JSON.Path("message").Data().(string))
}

func TestDefaultContentType(t *testing.T) {
	request := Request{
		Method:  Get,
		BaseURL: "http://localhost",
		Body:    []byte(`{"hello": "world"}`),
	}
	req, err := BuildRequestObject(&request)
	assert.NoError(t, err)
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
}

func TestCustomContentType(t *testing.T) {
	request := Request{
		Method:  Get,
		BaseURL: "http://localhost",
		Headers: map[string]string{"Content-Type": "custom"},
		Body:    []byte("Hello World"),
	}
	res, err := BuildRequestObject(&request)
	assert.NoError(t, err)
	assert.Equal(t, "custom", res.Header.Get("Content-Type"))
}

func TestCustomHTTPClient(t *testing.T) {
	fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Millisecond * 20)
		fmt.Fprintln(w, "{\"message\": \"success\"}")
	}))
	defer fakeServer.Close()

	request := Request{
		Method:  Get,
		BaseURL: fakeServer.URL + "/test_endpoint",
	}
	customClient := &Client{&http.Client{Timeout: time.Millisecond * 10}}
	_, err := customClient.API(&request)
	assert.True(t, strings.Contains(err.Error(), "Client.Timeout exceeded while awaiting headers"))
}
