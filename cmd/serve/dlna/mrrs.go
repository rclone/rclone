package dlna

import (
	"net/http"

	"github.com/anacrolix/dms/upnp"
)

type mediaReceiverRegistrarService struct {
	*server
	upnp.Eventing
}

func (mrrs *mediaReceiverRegistrarService) Handle(action string, argsXML []byte, r *http.Request) ([]soapArg, error) {
	switch action {
	case "IsAuthorized", "IsValidated":
		return soapArgs(
			"Result", "1",
		), nil
	case "RegisterDevice":
		return soapArgs(
			"RegistrationRespMsg", mrrs.RootDeviceUUID,
		), nil
	default:
		return nil, upnp.InvalidActionError
	}
}
