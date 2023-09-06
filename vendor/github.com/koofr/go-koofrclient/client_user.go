package koofrclient

import (
	"github.com/koofr/go-httpclient"
	"net/http"
)

func (c *KoofrClient) UserInfo() (user User, err error) {
	request := httpclient.RequestData{
		Method:         "GET",
		Path:           "/api/v2/user",
		ExpectedStatus: []int{http.StatusOK},
		RespEncoding:   httpclient.EncodingJSON,
		RespValue:      &user,
	}

	_, err = c.Request(&request)

	return
}
