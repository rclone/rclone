package upnp

import (
	"encoding/xml"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
)

var serviceURNRegexp *regexp.Regexp = regexp.MustCompile(`^urn:schemas-upnp-org:service:(\w+):(\d+)$`)

type ServiceURN struct {
	Type    string
	Version uint64
}

func (me ServiceURN) String() string {
	return fmt.Sprintf("urn:schemas-upnp-org:service:%s:%d", me.Type, me.Version)
}

func ParseServiceType(s string) (ret ServiceURN, err error) {
	matches := serviceURNRegexp.FindStringSubmatch(s)
	if matches == nil {
		err = errors.New(s)
		return
	}
	if len(matches) != 3 {
		log.Panicf("Invalid serviceURNRegexp ?")
	}
	ret.Type = matches[1]
	ret.Version, err = strconv.ParseUint(matches[2], 0, 0)
	return
}

type SoapAction struct {
	ServiceURN
	Action string
}

func ParseActionHTTPHeader(s string) (ret SoapAction, err error) {
	if s[0] != '"' || s[len(s)-1] != '"' {
		return
	}
	s = s[1 : len(s)-1]
	hashIndex := strings.LastIndex(s, "#")
	if hashIndex == -1 {
		return
	}
	ret.Action = s[hashIndex+1:]
	ret.ServiceURN, err = ParseServiceType(s[:hashIndex])
	return
}

type SpecVersion struct {
	Major int `xml:"major"`
	Minor int `xml:"minor"`
}

type Icon struct {
	Mimetype string `xml:"mimetype"`
	Width    int    `xml:"width"`
	Height   int    `xml:"height"`
	Depth    int    `xml:"depth"`
	URL      string `xml:"url"`
}

type Service struct {
	XMLName     xml.Name `xml:"service"`
	ServiceType string   `xml:"serviceType"`
	ServiceId   string   `xml:"serviceId"`
	SCPDURL     string
	ControlURL  string `xml:"controlURL"`
	EventSubURL string `xml:"eventSubURL"`
}

type Device struct {
	DeviceType   string `xml:"deviceType"`
	FriendlyName string `xml:"friendlyName"`
	Manufacturer string `xml:"manufacturer"`
	ModelName    string `xml:"modelName"`
	UDN          string
	IconList     []Icon    `xml:"iconList>icon"`
	ServiceList  []Service `xml:"serviceList>service"`
}

type DeviceDesc struct {
	XMLName     xml.Name    `xml:"urn:schemas-upnp-org:device-1-0 root"`
	SpecVersion SpecVersion `xml:"specVersion"`
	Device      Device      `xml:"device"`
}

type Error struct {
	XMLName xml.Name `xml:"urn:schemas-upnp-org:control-1-0 UPnPError"`
	Code    uint     `xml:"errorCode"`
	Desc    string   `xml:"errorDescription"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("%d %s", e.Code, e.Desc)
}

const (
	InvalidActionErrorCode        = 401
	ActionFailedErrorCode         = 501
	ArgumentValueInvalidErrorCode = 600
)

var (
	InvalidActionError        = Errorf(401, "Invalid Action")
	ArgumentValueInvalidError = Errorf(600, "The argument value is invalid")
)

// Errorf creates an UPNP error from the given code and description
func Errorf(code uint, tpl string, args ...interface{}) *Error {
	return &Error{Code: code, Desc: fmt.Sprintf(tpl, args...)}
}

// ConvertError converts any error to an UPNP error
func ConvertError(err error) *Error {
	if err == nil {
		return nil
	}
	if e, ok := err.(*Error); ok {
		return e
	}
	return Errorf(ActionFailedErrorCode, err.Error())
}

type Action struct {
	Name      string
	Arguments []Argument
}

type Argument struct {
	Name            string
	Direction       string
	RelatedStateVar string
}

type SCPD struct {
	XMLName           xml.Name        `xml:"urn:schemas-upnp-org:service-1-0 scpd"`
	SpecVersion       SpecVersion     `xml:"specVersion"`
	ActionList        []Action        `xml:"actionList>action"`
	ServiceStateTable []StateVariable `xml:"serviceStateTable>stateVariable"`
}

type StateVariable struct {
	SendEvents    string    `xml:"sendEvents,attr"`
	Name          string    `xml:"name"`
	DataType      string    `xml:"dataType"`
	AllowedValues *[]string `xml:"allowedValueList>allowedValue,omitempty"`
}

func FormatUUID(buf []byte) string {
	return fmt.Sprintf("uuid:%x-%x-%x-%x-%x", buf[:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16])
}
