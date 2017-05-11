// See swift_test.go for requirements to run this test.
package rs_test

import (
	"os"
	"testing"

	"github.com/ncw/swift/rs"
)

var (
	c rs.RsConnection
)

const (
	CONTAINER    = "GoSwiftUnitTest"
	OBJECT       = "test_object"
	CONTENTS     = "12345"
	CONTENT_SIZE = int64(len(CONTENTS))
	CONTENT_MD5  = "827ccb0eea8a706c4c34a16891f84e7b"
)

// Test functions are run in order - this one must be first!
func TestAuthenticate(t *testing.T) {
	UserName := os.Getenv("SWIFT_API_USER")
	ApiKey := os.Getenv("SWIFT_API_KEY")
	AuthUrl := os.Getenv("SWIFT_AUTH_URL")
	if UserName == "" || ApiKey == "" || AuthUrl == "" {
		t.Fatal("SWIFT_API_USER, SWIFT_API_KEY and SWIFT_AUTH_URL not all set")
	}
	c = rs.RsConnection{}
	c.UserName = UserName
	c.ApiKey = ApiKey
	c.AuthUrl = AuthUrl
	err := c.Authenticate()
	if err != nil {
		t.Fatal("Auth failed", err)
	}
	if !c.Authenticated() {
		t.Fatal("Not authenticated")
	}
}

// Setup
func TestContainerCreate(t *testing.T) {
	err := c.ContainerCreate(CONTAINER, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCDNEnable(t *testing.T) {
	headers, err := c.ContainerCDNEnable(CONTAINER, 0)
	if err != nil {
		t.Error(err)
	}
	if _, ok := headers["X-Cdn-Uri"]; !ok {
		t.Error("Failed to enable CDN for container")
	}
}

func TestOnReAuth(t *testing.T) {
	c2 := rs.RsConnection{}
	c2.UserName = c.UserName
	c2.ApiKey = c.ApiKey
	c2.AuthUrl = c.AuthUrl
	_, err := c2.ContainerCDNEnable(CONTAINER, 0)
	if err != nil {
		t.Fatalf("Failed to reauthenticate: %v", err)
	}
}

func TestCDNMeta(t *testing.T) {
	headers, err := c.ContainerCDNMeta(CONTAINER)
	if err != nil {
		t.Error(err)
	}
	if _, ok := headers["X-Cdn-Uri"]; !ok {
		t.Error("CDN is not enabled")
	}
}

func TestCDNDisable(t *testing.T) {
	err := c.ContainerCDNDisable(CONTAINER) // files stick in CDN until TTL expires
	if err != nil {
		t.Error(err)
	}
}

// Teardown
func TestContainerDelete(t *testing.T) {
	err := c.ContainerDelete(CONTAINER)
	if err != nil {
		t.Fatal(err)
	}
}
