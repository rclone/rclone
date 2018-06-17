// Copyright 2013 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package remote_api

// This file provides the client for connecting remotely to a user's production
// application.

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"

	"google.golang.org/appengine/internal"
	pb "google.golang.org/appengine/internal/remote_api"
)

// Client is a connection to the production APIs for an application.
type Client struct {
	hc    *http.Client
	url   string
	appID string
}

// NewClient returns a client for the given host. All communication will
// be performed over SSL unless the host is localhost.
func NewClient(host string, client *http.Client) (*Client, error) {
	// Add an appcfg header to outgoing requests.
	wrapClient := new(http.Client)
	*wrapClient = *client
	t := client.Transport
	if t == nil {
		t = http.DefaultTransport
	}
	wrapClient.Transport = &headerAddingRoundTripper{t}

	url := url.URL{
		Scheme: "https",
		Host:   host,
		Path:   "/_ah/remote_api",
	}
	if host == "localhost" || strings.HasPrefix(host, "localhost:") {
		url.Scheme = "http"
	}
	u := url.String()
	appID, err := getAppID(wrapClient, u)
	if err != nil {
		return nil, fmt.Errorf("unable to contact server: %v", err)
	}
	return &Client{
		hc:    wrapClient,
		url:   u,
		appID: appID,
	}, nil
}

// NewContext returns a copy of parent that will cause App Engine API
// calls to be sent to the client's remote host.
func (c *Client) NewContext(parent context.Context) context.Context {
	ctx := internal.WithCallOverride(parent, c.call)
	ctx = internal.WithLogOverride(ctx, c.logf)
	ctx = internal.WithAppIDOverride(ctx, c.appID)
	return ctx
}

// NewRemoteContext returns a context that gives access to the production
// APIs for the application at the given host. All communication will be
// performed over SSL unless the host is localhost.
func NewRemoteContext(host string, client *http.Client) (context.Context, error) {
	c, err := NewClient(host, client)
	if err != nil {
		return nil, err
	}
	return c.NewContext(context.Background()), nil
}

var logLevels = map[int64]string{
	0: "DEBUG",
	1: "INFO",
	2: "WARNING",
	3: "ERROR",
	4: "CRITICAL",
}

func (c *Client) logf(level int64, format string, args ...interface{}) {
	log.Printf(logLevels[level]+": "+format, args...)
}

func (c *Client) call(ctx context.Context, service, method string, in, out proto.Message) error {
	req, err := proto.Marshal(in)
	if err != nil {
		return fmt.Errorf("error marshalling request: %v", err)
	}

	remReq := &pb.Request{
		ServiceName: proto.String(service),
		Method:      proto.String(method),
		Request:     req,
		// NOTE(djd): RequestId is unused in the server.
	}

	req, err = proto.Marshal(remReq)
	if err != nil {
		return fmt.Errorf("proto.Marshal: %v", err)
	}

	// TODO(djd): Respect ctx.Deadline()?
	resp, err := c.hc.Post(c.url, "application/octet-stream", bytes.NewReader(req))
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad response %d; body: %q", resp.StatusCode, body)
	}
	if err != nil {
		return fmt.Errorf("failed reading response: %v", err)
	}
	remResp := &pb.Response{}
	if err := proto.Unmarshal(body, remResp); err != nil {
		return fmt.Errorf("error unmarshalling response: %v", err)
	}

	if ae := remResp.GetApplicationError(); ae != nil {
		return &internal.APIError{
			Code:    ae.GetCode(),
			Detail:  ae.GetDetail(),
			Service: service,
		}
	}

	if remResp.Response == nil {
		return fmt.Errorf("unexpected response: %s", proto.MarshalTextString(remResp))
	}

	return proto.Unmarshal(remResp.Response, out)
}

// This is a forgiving regexp designed to parse the app ID from YAML.
var appIDRE = regexp.MustCompile(`app_id["']?\s*:\s*['"]?([-a-z0-9.:~]+)`)

func getAppID(client *http.Client, url string) (string, error) {
	// Generate a pseudo-random token for handshaking.
	token := strconv.Itoa(rand.New(rand.NewSource(time.Now().UnixNano())).Int())

	resp, err := client.Get(fmt.Sprintf("%s?rtok=%s", url, token))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad response %d; body: %q", resp.StatusCode, body)
	}
	if err != nil {
		return "", fmt.Errorf("failed reading response: %v", err)
	}

	// Check the token is present in response.
	if !bytes.Contains(body, []byte(token)) {
		return "", fmt.Errorf("token not found: want %q; body %q", token, body)
	}

	match := appIDRE.FindSubmatch(body)
	if match == nil {
		return "", fmt.Errorf("app ID not found: body %q", body)
	}

	return string(match[1]), nil
}

type headerAddingRoundTripper struct {
	Wrapped http.RoundTripper
}

func (t *headerAddingRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set("X-Appcfg-Api-Version", "1")
	return t.Wrapped.RoundTrip(r)
}
