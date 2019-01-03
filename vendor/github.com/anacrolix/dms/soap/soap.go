package soap

import (
	"encoding/xml"
)

const (
	EncodingStyle = "http://schemas.xmlsoap.org/soap/encoding/"
	EnvelopeNS    = "http://schemas.xmlsoap.org/soap/envelope/"
)

type Arg struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

type Action struct {
	XMLName xml.Name
	Args    []Arg
}

type Body struct {
	Action []byte `xml:",innerxml"`
}

type UPnPError struct {
	XMLName xml.Name `xml:"urn:schemas-upnp-org:control-1-0 UPnPError"`
	Code    uint     `xml:"errorCode"`
	Desc    string   `xml:"errorDescription"`
}

type FaultDetail struct {
	XMLName xml.Name `xml:"detail"`
	Data    interface{}
}

type Fault struct {
	XMLName     xml.Name    `xml:"http://schemas.xmlsoap.org/soap/envelope/ Fault"`
	FaultCode   string      `xml:"faultcode"`
	FaultString string      `xml:"faultstring"`
	Detail      FaultDetail `xml:"detail"`
}

func NewFault(s string, detail interface{}) *Fault {
	return &Fault{
		FaultCode:   EnvelopeNS + ":Client",
		FaultString: s,
		Detail: FaultDetail{
			Data: detail,
		},
	}
}

type Envelope struct {
	XMLName       xml.Name `xml:"http://schemas.xmlsoap.org/soap/envelope/ Envelope"`
	EncodingStyle string   `xml:"encodingStyle,attr"`
	Body          Body     `xml:"http://schemas.xmlsoap.org/soap/envelope/ Body"`
}

/* XML marshalling of nested namespaces is broken.

func NewEnvelope(action []byte) Envelope {
	return Envelope{
		EncodingStyle: EncodingStyle,
		Body:          Body{action},
	}
}
*/
