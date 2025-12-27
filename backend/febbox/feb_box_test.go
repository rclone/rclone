// feb_box_test.go
package feb_box

import (
    "testing"
    "time"

    "github.com/rclone/rclone/fs"
    "github.com/stretchr/testify/assert"
)

func TestRegistration(t *testing.T) {
    var backend *fs.RegInfo
    for _, b := range fs.Registry {
        if b.Name == "febbox" {
            backend = b
            break
        }
    }
    
    assert.NotNil(t, backend, "febbox backend should be registered")
    assert.Equal(t, "febbox", backend.Name)
    assert.Equal(t, "Febbox Cloud Storage", backend.Description)
}

func TestGetMimeType(t *testing.T) {
    tests := []struct {
        ext      string
        expected string
    }{
        {"mp4", "video/mp4"},
        {"mkv", "video/x-matroska"},
        {"m3u8", "application/x-mpegURL"},
        {"mp3", "audio/mpeg"},
        {"jpg", "image/jpeg"},
        {"png", "image/png"},
        {"pdf", "application/octet-stream"},
        {"unknown", "application/octet-stream"},
    }

    for _, tt := range tests {
        t.Run(tt.ext, func(t *testing.T) {
            result := getMimeType(tt.ext)
            assert.Equal(t, tt.expected, result)
        })
    }
}

func TestObjectInterface(t *testing.T) {
    fsObj := &Fs{
        name: "test",
        root: "/",
        opt: Options{
            Cookies:   "test-cookies",  // Changed from Cookie to Cookies
            ShareKey: "test-share-key",
        },
        shareKey: "test-share-key",
    }

    obj := &Object{
        fs:      fsObj,
        remote:  "test.mp4",
        fid:     12345,
        name:    "test.mp4",
        size:    1024 * 1024 * 100,
        modTime: time.Now(),
        isDir:   false,
        mimeType: "video/mp4",
    }

    assert.Equal(t, "test.mp4", obj.Remote())
    assert.Equal(t, int64(104857600), obj.Size())
    assert.Equal(t, "video/mp4", obj.mimeType)
    assert.False(t, obj.isDir)
    assert.True(t, obj.Storable())
    assert.Equal(t, fsObj, obj.Fs())
}

func TestParseCookieString(t *testing.T) {
    tests := []struct {
        name      string
        cookieStr string
        expected  int // number of cookies
    }{
        {
            name:      "multiple cookies",
            cookieStr: "PHPSESSID=abc; ui=def; cf_clearance=ghi",
            expected:  3,
        },
        {
            name:      "single cookie",
            cookieStr: "ui=abc123",
            expected:  1,
        },
        {
            name:      "empty cookies",
            cookieStr: "",
            expected:  0,
        },
        {
            name:      "malformed cookie",
            cookieStr: "ui=abc; broken; name=value",
            expected:  2, // Should skip the broken one
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cookies := ParseCookieString(tt.cookieStr)
            assert.Equal(t, tt.expected, len(cookies))
        })
    }
}

func TestParseCookieStringValues(t *testing.T) {
    cookieStr := "PHPSESSID=abc123; ui=eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9; cf_clearance=def456"
    cookies := ParseCookieString(cookieStr)
    
    assert.Equal(t, 3, len(cookies))
    
    // Check each cookie
    foundPHPSESSID := false
    foundUI := false
    foundCF := false
    
    for _, cookie := range cookies {
        switch cookie.Name {
        case "PHPSESSID":
            assert.Equal(t, "abc123", cookie.Value)
            foundPHPSESSID = true
        case "ui":
            assert.Equal(t, "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9", cookie.Value)
            foundUI = true
        case "cf_clearance":
            assert.Equal(t, "def456", cookie.Value)
            foundCF = true
        }
    }
    
    assert.True(t, foundPHPSESSID, "Should have PHPSESSID cookie")
    assert.True(t, foundUI, "Should have UI cookie")
    assert.True(t, foundCF, "Should have cf_clearance cookie")
}