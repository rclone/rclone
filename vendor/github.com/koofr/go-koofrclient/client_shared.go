package koofrclient

import (
	"net/http"

	"github.com/koofr/go-httpclient"
)

func (c *KoofrClient) Shared() (shared []Shared, err error) {
	d := &struct {
		Files *[]Shared
	}{&shared}

	request := httpclient.RequestData{
		Method:         "GET",
		Path:           "/api/v2/shared",
		ExpectedStatus: []int{http.StatusOK},
		RespEncoding:   httpclient.EncodingJSON,
		RespValue:      &d,
	}

	_, err = c.Request(&request)

	return
}
