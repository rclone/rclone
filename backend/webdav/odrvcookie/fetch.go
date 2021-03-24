// Package odrvcookie can fetch authentication cookies for a sharepoint webdav endpoint
package odrvcookie

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"html/template"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fshttp"
	"golang.org/x/net/publicsuffix"
)

// CookieAuth hold the authentication information
// These are username and password as well as the authentication endpoint
type CookieAuth struct {
	user     string
	pass     string
	endpoint string
}

// CookieResponse contains the requested cookies
type CookieResponse struct {
	RtFa    http.Cookie
	FedAuth http.Cookie
}

// SharepointSuccessResponse holds a response from a successful microsoft login
type SharepointSuccessResponse struct {
	XMLName xml.Name            `xml:"Envelope"`
	Body    SuccessResponseBody `xml:"Body"`
}

// SuccessResponseBody is the body of a successful response, it holds the token
type SuccessResponseBody struct {
	XMLName xml.Name
	Type    string    `xml:"RequestSecurityTokenResponse>TokenType"`
	Created time.Time `xml:"RequestSecurityTokenResponse>Lifetime>Created"`
	Expires time.Time `xml:"RequestSecurityTokenResponse>Lifetime>Expires"`
	Token   string    `xml:"RequestSecurityTokenResponse>RequestedSecurityToken>BinarySecurityToken"`
}

// SharepointError holds an error response microsoft login
type SharepointError struct {
	XMLName xml.Name          `xml:"Envelope"`
	Body    ErrorResponseBody `xml:"Body"`
}

func (e *SharepointError) Error() string {
	return fmt.Sprintf("%s: %s (%s)", e.Body.FaultCode, e.Body.Reason, e.Body.Detail)
}

// ErrorResponseBody contains the body of an erroneous response
type ErrorResponseBody struct {
	XMLName   xml.Name
	FaultCode string `xml:"Fault>Code>Subcode>Value"`
	Reason    string `xml:"Fault>Reason>Text"`
	Detail    string `xml:"Fault>Detail>error>internalerror>text"`
}

// reqString is a template that gets populated with the user data in order to retrieve a "BinarySecurityToken"
const reqString = `<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
xmlns:a="http://www.w3.org/2005/08/addressing"
xmlns:u="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd">
<s:Header>
<a:Action s:mustUnderstand="1">http://schemas.xmlsoap.org/ws/2005/02/trust/RST/Issue</a:Action>
<a:ReplyTo>
<a:Address>http://www.w3.org/2005/08/addressing/anonymous</a:Address>
</a:ReplyTo>
<a:To s:mustUnderstand="1">https://login.microsoftonline.com/extSTS.srf</a:To>
<o:Security s:mustUnderstand="1"
 xmlns:o="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd">
<o:UsernameToken>
  <o:Username>{{ .Username }}</o:Username>
  <o:Password>{{ .Password }}</o:Password>
</o:UsernameToken>
</o:Security>
</s:Header>
<s:Body>
<t:RequestSecurityToken xmlns:t="http://schemas.xmlsoap.org/ws/2005/02/trust">
<wsp:AppliesTo xmlns:wsp="http://schemas.xmlsoap.org/ws/2004/09/policy">
  <a:EndpointReference>
    <a:Address>{{ .Address }}</a:Address>
  </a:EndpointReference>
</wsp:AppliesTo>
<t:KeyType>http://schemas.xmlsoap.org/ws/2005/05/identity/NoProofKey</t:KeyType>
<t:RequestType>http://schemas.xmlsoap.org/ws/2005/02/trust/Issue</t:RequestType>
<t:TokenType>urn:oasis:names:tc:SAML:1.0:assertion</t:TokenType>
</t:RequestSecurityToken>
</s:Body>
</s:Envelope>`

// New creates a new CookieAuth struct
func New(pUser, pPass, pEndpoint string) CookieAuth {
	retStruct := CookieAuth{
		user:     pUser,
		pass:     pPass,
		endpoint: pEndpoint,
	}

	return retStruct
}

// Cookies creates a CookieResponse. It fetches the auth token and then
// retrieves the Cookies
func (ca *CookieAuth) Cookies(ctx context.Context) (*CookieResponse, error) {
	tokenResp, err := ca.getSPToken(ctx)
	if err != nil {
		return nil, err
	}
	return ca.getSPCookie(tokenResp)
}

func (ca *CookieAuth) getSPCookie(conf *SharepointSuccessResponse) (*CookieResponse, error) {
	spRoot, err := url.Parse(ca.endpoint)
	if err != nil {
		return nil, errors.Wrap(err, "Error while constructing endpoint URL")
	}

	u, err := url.Parse(spRoot.Scheme + "://" + spRoot.Host + "/_forms/default.aspx?wa=wsignin1.0")
	if err != nil {
		return nil, errors.Wrap(err, "Error while constructing login URL")
	}

	// To authenticate with davfs or anything else we need two cookies (rtFa and FedAuth)
	// In order to get them we use the token we got earlier and a cookieJar
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Jar: jar,
	}

	// Send the previously acquired Token as a Post parameter
	if _, err = client.Post(u.String(), "text/xml", strings.NewReader(conf.Body.Token)); err != nil {
		return nil, errors.Wrap(err, "Error while grabbing cookies from endpoint: %v")
	}

	cookieResponse := CookieResponse{}
	for _, cookie := range jar.Cookies(u) {
		if (cookie.Name == "rtFa") || (cookie.Name == "FedAuth") {
			switch cookie.Name {
			case "rtFa":
				cookieResponse.RtFa = *cookie
			case "FedAuth":
				cookieResponse.FedAuth = *cookie
			}
		}
	}
	return &cookieResponse, nil
}

func (ca *CookieAuth) getSPToken(ctx context.Context) (conf *SharepointSuccessResponse, err error) {
	reqData := map[string]interface{}{
		"Username": ca.user,
		"Password": ca.pass,
		"Address":  ca.endpoint,
	}

	t := template.Must(template.New("authXML").Parse(reqString))

	buf := &bytes.Buffer{}
	if err := t.Execute(buf, reqData); err != nil {
		return nil, errors.Wrap(err, "Error while filling auth token template")
	}

	// Create and execute the first request which returns an auth token for the sharepoint service
	// With this token we can authenticate on the login page and save the returned cookies
	req, err := http.NewRequestWithContext(ctx, "POST", "https://login.microsoftonline.com/extSTS.srf", buf)
	if err != nil {
		return nil, err
	}

	client := fshttp.NewClient(ctx)
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "Error while logging in to endpoint")
	}
	defer fs.CheckClose(resp.Body, &err)

	respBuf := bytes.Buffer{}
	_, err = respBuf.ReadFrom(resp.Body)
	if err != nil {
		return nil, err
	}
	s := respBuf.Bytes()

	conf = &SharepointSuccessResponse{}
	err = xml.Unmarshal(s, conf)
	if conf.Body.Token == "" {
		// xml Unmarshal won't fail if the response doesn't contain a token
		// However, the token will be empty
		sErr := &SharepointError{}

		errSErr := xml.Unmarshal(s, sErr)
		if errSErr == nil {
			return nil, sErr
		}
	}

	if err != nil {
		return nil, errors.Wrap(err, "Error while reading endpoint response")
	}
	return
}
