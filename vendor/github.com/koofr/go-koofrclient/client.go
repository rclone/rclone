package koofrclient

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/koofr/go-httpclient"
)

type KoofrClient struct {
	*httpclient.HTTPClient
	token  string
	userID string
}

func NewKoofrClient(baseUrl string, disableSecurity bool) *KoofrClient {
	var httpClient *httpclient.HTTPClient

	if disableSecurity {
		httpClient = httpclient.Insecure()
	} else {
		httpClient = httpclient.New()
	}

	return NewKoofrClientWithHTTPClient(baseUrl, httpClient)
}

func NewKoofrClientWithHTTPClient(baseUrl string, httpClient *httpclient.HTTPClient) *KoofrClient {
	apiBaseUrl, _ := url.Parse(baseUrl)

	httpClient.BaseURL = apiBaseUrl

	client:= &KoofrClient{
		HTTPClient: httpClient,
		token:      "",
		userID:     "",
	}

	client.SetUserAgent("go koofrclient")
	return client
}

func (c *KoofrClient) SetUserAgent(ua string) {
	c.Headers.Set("User-Agent", ua)
}

func (c *KoofrClient) SetToken(token string) {
	c.token = token
	c.HTTPClient.Headers.Set("Authorization", fmt.Sprintf("Token token=%s", token))
}

func (c *KoofrClient) GetToken() string {
	return c.token
}

func (c *KoofrClient) SetUserID(userID string) {
	c.userID = userID
}

func (c *KoofrClient) GetUserID() string {
	return c.userID
}

func (c *KoofrClient) Authenticate(email string, password string) (err error) {
	var tokenResponse Token

	tokenRequest := TokenRequest{
		Email:    email,
		Password: password,
	}

	request := httpclient.RequestData{
		Method:         "POST",
		Path:           "/token",
		Headers:        make(http.Header),
		ExpectedStatus: []int{http.StatusOK},
		ReqEncoding:    httpclient.EncodingJSON,
		ReqValue:       tokenRequest,
		RespEncoding:   httpclient.EncodingJSON,
		RespValue:      &tokenResponse,
	}

	res, err := c.Request(&request)

	if err != nil {
		return
	}

	c.SetToken(tokenResponse.Token)
	c.SetUserID(res.Header.Get("X-User-ID"))

	return
}
