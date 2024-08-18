package serve

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/stretchr/testify/assert"
)

func TestObjectBadMethod(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("BADMETHOD", "http://example.com/aFile", nil)
	o := mockobject.New("aFile")
	Object(w, r, o)
	resp := w.Result()
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "Method Not Allowed\n", string(body))
}

func TestObjectHEAD(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("HEAD", "http://example.com/aFile", nil)
	o := mockobject.New("aFile").WithContent([]byte("hello"), mockobject.SeekModeNone)
	_ = o.SetModTime(context.Background(), time.Date(2023, 9, 20, 12, 11, 15, 0, time.FixedZone("", 4*60*60))) // UTC+4
	Object(w, r, o)
	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "5", resp.Header.Get("Content-Length"))
	assert.Equal(t, "bytes", resp.Header.Get("Accept-Ranges"))
	assert.Equal(t, "Wed, 20 Sep 2023 08:11:15 GMT", resp.Header.Get("Last-Modified"))
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "", string(body))
}

func TestObjectGET(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://example.com/aFile", nil)
	o := mockobject.New("aFile").WithContent([]byte("hello"), mockobject.SeekModeNone)
	_ = o.SetModTime(context.Background(), time.Date(2023, 9, 20, 12, 11, 15, 0, time.FixedZone("", 2*60*60))) // UTC+2
	Object(w, r, o)
	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "5", resp.Header.Get("Content-Length"))
	assert.Equal(t, "bytes", resp.Header.Get("Accept-Ranges"))
	assert.Equal(t, "Wed, 20 Sep 2023 10:11:15 GMT", resp.Header.Get("Last-Modified"))
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "hello", string(body))
}

func TestObjectRange(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://example.com/aFile", nil)
	r.Header.Add("Range", "bytes=3-5")
	o := mockobject.New("aFile").WithContent([]byte("0123456789"), mockobject.SeekModeNone)
	Object(w, r, o)
	resp := w.Result()
	assert.Equal(t, http.StatusPartialContent, resp.StatusCode)
	assert.Equal(t, "3", resp.Header.Get("Content-Length"))
	assert.Equal(t, "bytes", resp.Header.Get("Accept-Ranges"))
	assert.Equal(t, "bytes 3-5/10", resp.Header.Get("Content-Range"))
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "345", string(body))
}

func TestObjectBadRange(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://example.com/aFile", nil)
	r.Header.Add("Range", "xxxbytes=3-5")
	o := mockobject.New("aFile").WithContent([]byte("0123456789"), mockobject.SeekModeNone)
	Object(w, r, o)
	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
		assert.Equal(t, "10", contentLength)
	}
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "Bad Request\n", string(body))
}
