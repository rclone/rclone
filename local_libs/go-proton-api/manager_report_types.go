package proton

import (
	"encoding/json"
	"fmt"

	"github.com/ProtonMail/gluon/rfc822"
)

type ClientType int

const (
	ClientTypeEmail ClientType = iota + 1
	ClientTypeVPN
	ClientTypeCalendar
	ClientTypeDrive
)

type ReportBugReq struct {
	OS        string
	OSVersion string

	Browser           string
	BrowserVersion    string
	BrowserExtensions string

	Resolution  string
	DisplayMode string

	Client        string
	ClientVersion string
	ClientType    ClientType

	Title       string
	Description string

	Username string
	Email    string

	Country string
	ISP     string
}

func (req ReportBugReq) toFormData() map[string]string {
	b, err := json.Marshal(req)
	if err != nil {
		panic(err)
	}

	var raw map[string]any

	if err := json.Unmarshal(b, &raw); err != nil {
		panic(err)
	}

	res := make(map[string]string)

	for key := range raw {
		if val := fmt.Sprint(raw[key]); val != "" {
			res[key] = val
		}
	}

	return res
}

type ReportBugAttachment struct {
	Name     string
	Filename string
	MIMEType rfc822.MIMEType
	Body     []byte
}
