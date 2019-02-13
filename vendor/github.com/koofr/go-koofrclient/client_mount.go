package koofrclient

import (
	"github.com/koofr/go-httpclient"
	"net/http"
)

func (c *KoofrClient) Mounts() (mounts []Mount, err error) {
	d := &struct {
		Mounts *[]Mount
	}{&mounts}

	request := httpclient.RequestData{
		Method:         "GET",
		Path:           "/api/v2/mounts",
		ExpectedStatus: []int{http.StatusOK},
		RespEncoding:   httpclient.EncodingJSON,
		RespValue:      &d,
	}

	_, err = c.Request(&request)

	return
}

func (c *KoofrClient) MountsDetails(mountId string) (mount Mount, err error) {
	request := httpclient.RequestData{
		Method:         "GET",
		Path:           "/api/v2/mounts/" + mountId,
		ExpectedStatus: []int{http.StatusOK},
		RespEncoding:   httpclient.EncodingJSON,
		RespValue:      &mount,
	}

	_, err = c.Request(&request)

	return
}
