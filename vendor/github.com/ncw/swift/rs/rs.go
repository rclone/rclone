package rs

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/ncw/swift"
)

// RsConnection is a RackSpace specific wrapper to the core swift library which
// exposes the RackSpace CDN commands via the CDN Management URL interface.
type RsConnection struct {
	swift.Connection
	cdnUrl string
}

// manage is similar to the swift storage method, but uses the CDN Management URL for CDN specific calls.
func (c *RsConnection) manage(p swift.RequestOpts) (resp *http.Response, headers swift.Headers, err error) {
	p.OnReAuth = func() (string, error) {
		if c.cdnUrl == "" {
			c.cdnUrl = c.Auth.CdnUrl()
		}
		if c.cdnUrl == "" {
			return "", errors.New("The X-CDN-Management-Url does not exist on the authenticated platform")
		}
		return c.cdnUrl, nil
	}
	if c.Authenticated() {
		_, err = p.OnReAuth()
		if err != nil {
			return nil, nil, err
		}
	}
	return c.Connection.Call(c.cdnUrl, p)
}

// ContainerCDNEnable enables a container for public CDN usage.
//
// Change the default TTL of 259200 seconds (72 hours) by passing in an integer value.
//
// This method can be called again to change the TTL.
func (c *RsConnection) ContainerCDNEnable(container string, ttl int) (swift.Headers, error) {
	h := swift.Headers{"X-CDN-Enabled": "true"}
	if ttl > 0 {
		h["X-TTL"] = strconv.Itoa(ttl)
	}

	_, headers, err := c.manage(swift.RequestOpts{
		Container:  container,
		Operation:  "PUT",
		ErrorMap:   swift.ContainerErrorMap,
		NoResponse: true,
		Headers:    h,
	})
	return headers, err
}

// ContainerCDNDisable disables CDN access to a container.
func (c *RsConnection) ContainerCDNDisable(container string) error {
	h := swift.Headers{"X-CDN-Enabled": "false"}

	_, _, err := c.manage(swift.RequestOpts{
		Container:  container,
		Operation:  "PUT",
		ErrorMap:   swift.ContainerErrorMap,
		NoResponse: true,
		Headers:    h,
	})
	return err
}

// ContainerCDNMeta returns the CDN metadata for a container.
func (c *RsConnection) ContainerCDNMeta(container string) (swift.Headers, error) {
	_, headers, err := c.manage(swift.RequestOpts{
		Container:  container,
		Operation:  "HEAD",
		ErrorMap:   swift.ContainerErrorMap,
		NoResponse: true,
		Headers:    swift.Headers{},
	})
	return headers, err
}
