package dlna

import (
	"crypto/md5"
	"encoding/xml"
	"fmt"
	"io"
	"log"

	"github.com/anacrolix/dms/soap"
	"github.com/anacrolix/dms/upnp"
)

func makeDeviceUUID(unique string) string {
	h := md5.New()
	if _, err := io.WriteString(h, unique); err != nil {
		log.Panicf("makeDeviceUUID write failed: %s", err)
	}
	buf := h.Sum(nil)
	return upnp.FormatUUID(buf)
}

func didlLite(chardata string) string {
	return `<DIDL-Lite` +
		` xmlns:dc="http://purl.org/dc/elements/1.1/"` +
		` xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/"` +
		` xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/"` +
		` xmlns:dlna="urn:schemas-dlna-org:metadata-1-0/">` +
		chardata +
		`</DIDL-Lite>`
}

func mustMarshalXML(value interface{}) []byte {
	ret, err := xml.MarshalIndent(value, "", "  ")
	if err != nil {
		log.Panicf("mustMarshalXML failed to marshal %v: %s", value, err)
	}
	return ret
}

// Marshal SOAP response arguments into a response XML snippet.
func marshalSOAPResponse(sa upnp.SoapAction, args map[string]string) []byte {
	soapArgs := make([]soap.Arg, 0, len(args))
	for argName, value := range args {
		soapArgs = append(soapArgs, soap.Arg{
			XMLName: xml.Name{Local: argName},
			Value:   value,
		})
	}
	return []byte(fmt.Sprintf(`<u:%[1]sResponse xmlns:u="%[2]s">%[3]s</u:%[1]sResponse>`,
		sa.Action, sa.ServiceURN.String(), mustMarshalXML(soapArgs)))
}
