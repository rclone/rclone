//go:build go1.21

package dlna

import (
	"net/http"

	"github.com/anacrolix/dms/upnp"
)

type mediaReceiverRegistrarService struct {
	*server
	upnp.Eventing
}

func (mrrs *mediaReceiverRegistrarService) Handle(action string, argsXML []byte, r *http.Request) (map[string]string, error) {
	switch action {
	case "IsAuthorized", "IsValidated":
		return map[string]string{
			"Result": "1",
		}, nil
	case "RegisterDevice":
		return map[string]string{
			"RegistrationRespMsg": mrrs.RootDeviceUUID,
		}, nil
	default:
		return nil, upnp.InvalidActionError
	}
}
