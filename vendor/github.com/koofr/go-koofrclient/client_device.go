package koofrclient

import (
	"github.com/koofr/go-httpclient"
	"net/http"
)

func (c *KoofrClient) Devices() (devices []Device, err error) {
	d := &struct {
		Devices *[]Device
	}{&devices}

	request := httpclient.RequestData{
		Method:         "GET",
		Path:           "/api/v2/devices",
		ExpectedStatus: []int{http.StatusOK},
		RespEncoding:   httpclient.EncodingJSON,
		RespValue:      &d,
	}

	_, err = c.Request(&request)

	return
}

func (c *KoofrClient) DevicesCreate(name string, provider DeviceProvider) (device Device, err error) {
	deviceCreate := DeviceCreate{name, provider}

	request := httpclient.RequestData{
		Method:         "POST",
		Path:           "/api/v2/devices",
		ExpectedStatus: []int{http.StatusCreated},
		ReqEncoding:    httpclient.EncodingJSON,
		ReqValue:       deviceCreate,
		RespEncoding:   httpclient.EncodingJSON,
		RespValue:      &device,
	}

	_, err = c.Request(&request)

	return
}

func (c *KoofrClient) DevicesDetails(deviceId string) (device Device, err error) {
	request := httpclient.RequestData{
		Method:         "GET",
		Path:           "/api/v2/devices/" + deviceId,
		ExpectedStatus: []int{http.StatusOK},
		RespEncoding:   httpclient.EncodingJSON,
		RespValue:      &device,
	}

	_, err = c.Request(&request)

	return
}

func (c *KoofrClient) DevicesUpdate(deviceId string, deviceUpdate DeviceUpdate) (err error) {
	request := httpclient.RequestData{
		Method:         "PUT",
		Path:           "/api/v2/devices/" + deviceId,
		ExpectedStatus: []int{http.StatusNoContent},
		ReqEncoding:    httpclient.EncodingJSON,
		ReqValue:       deviceUpdate,
		RespConsume:    true,
	}

	_, err = c.Request(&request)

	return
}

func (c *KoofrClient) DevicesDelete(deviceId string) (err error) {
	request := httpclient.RequestData{
		Method:         "DELETE",
		Path:           "/api/v2/devices/" + deviceId,
		ExpectedStatus: []int{http.StatusNoContent},
		RespConsume:    true,
	}

	_, err = c.Request(&request)

	return
}
