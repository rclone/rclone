// Package api has type definitions for webdav
package api

import (
	"encoding/xml"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
)

const (
	// Wed, 27 Sep 2017 14:28:34 GMT
	timeFormat = time.RFC1123
	// The same as time.RFC1123 with optional leading zeros on the date
	// see https://github.com/rclone/rclone/issues/2574
	noZerosRFC1123 = "Mon, _2 Jan 2006 15:04:05 MST"
)

// Multistatus contains responses returned from an HTTP 207 return code
type Multistatus struct {
	Responses []Response `xml:"response"`
}

// Response contains an Href the response it about and its properties
type Response struct {
	Href  string `xml:"href"`
	Props Prop   `xml:"propstat"`
}

// Prop is the properties of a response
//
// This is a lazy way of decoding the multiple <s:propstat> in the
// response.
//
// The response might look like this.
//
// <d:response>
//
//	<d:href>/remote.php/webdav/Nextcloud%20Manual.pdf</d:href>
//	<d:propstat>
//	  <d:prop>
//	    <d:getlastmodified>Tue, 19 Dec 2017 22:02:36 GMT</d:getlastmodified>
//	    <d:getcontentlength>4143665</d:getcontentlength>
//	    <d:resourcetype/>
//	    <d:getetag>"048d7be4437ff7deeae94db50ff3e209"</d:getetag>
//	    <d:getcontenttype>application/pdf</d:getcontenttype>
//	  </d:prop>
//	  <d:status>HTTP/1.1 200 OK</d:status>
//	</d:propstat>
//	<d:propstat>
//	  <d:prop>
//	    <d:quota-used-bytes/>
//	    <d:quota-available-bytes/>
//	  </d:prop>
//	  <d:status>HTTP/1.1 404 Not Found</d:status>
//	</d:propstat>
//
// </d:response>
//
// So we elide the array of <d:propstat> and within that the array of
// <d:prop> into one struct.
//
// Note that status collects all the status values for which we just
// check the first is OK.
type Prop struct {
	Status       []string  `xml:"DAV: status"`
	Name         string    `xml:"DAV: prop>displayname,omitempty"`
	Type         *xml.Name `xml:"DAV: prop>resourcetype>collection,omitempty"`
	IsCollection *string   `xml:"DAV: prop>iscollection,omitempty"` // this is a Microsoft extension see #2716
	Size         int64     `xml:"DAV: prop>getcontentlength,omitempty"`
	Modified     Time      `xml:"DAV: prop>getlastmodified,omitempty"`
	Checksums    []string  `xml:"prop>checksums>checksum,omitempty"`
	Permissions  string    `xml:"prop>permissions,omitempty"`
	MESha1Hex    *string   `xml:"ME: prop>sha1hex,omitempty"` // Fastmail-specific sha1 checksum
}

// Parse a status of the form "HTTP/1.1 200 OK" or "HTTP/1.1 200"
var parseStatus = regexp.MustCompile(`^HTTP/[0-9.]+\s+(\d+)`)

// StatusOK examines the Status and returns an OK flag
func (p *Prop) StatusOK() bool {
	// Assume OK if no statuses received
	if len(p.Status) == 0 {
		return true
	}
	match := parseStatus.FindStringSubmatch(p.Status[0])
	if len(match) < 2 {
		return false
	}
	code, err := strconv.Atoi(match[1])
	if err != nil {
		return false
	}
	if code >= 200 && code < 300 {
		return true
	}
	return false
}

// Hashes returns a map of all checksums - may be nil
func (p *Prop) Hashes() (hashes map[hash.Type]string) {
	if len(p.Checksums) > 0 {
		hashes = make(map[hash.Type]string)
		for _, checksums := range p.Checksums {
			checksums = strings.ToLower(checksums)
			for _, checksum := range strings.Split(checksums, " ") {
				switch {
				case strings.HasPrefix(checksum, "sha1:"):
					hashes[hash.SHA1] = checksum[5:]
				case strings.HasPrefix(checksum, "md5:"):
					hashes[hash.MD5] = checksum[4:]
				}
			}
		}
		return hashes
	} else if p.MESha1Hex != nil {
		hashes = make(map[hash.Type]string)
		hashes[hash.SHA1] = *p.MESha1Hex
		return hashes
	}
	return nil
}

// PropValue is a tagged name and value
type PropValue struct {
	XMLName xml.Name `xml:""`
	Value   string   `xml:",chardata"`
}

// Error is used to describe webdav errors
//
// <d:error xmlns:d="DAV:" xmlns:s="http://sabredav.org/ns">
//
//	<s:exception>Sabre\DAV\Exception\NotFound</s:exception>
//	<s:message>File with name Photo could not be located</s:message>
//
// </d:error>
type Error struct {
	Exception  string `xml:"exception,omitempty"`
	Message    string `xml:"message,omitempty"`
	Status     string
	StatusCode int
}

// Error returns a string for the error and satisfies the error interface
func (e *Error) Error() string {
	var out []string
	if e.Message != "" {
		out = append(out, e.Message)
	}
	if e.Exception != "" {
		out = append(out, e.Exception)
	}
	if e.Status != "" {
		out = append(out, e.Status)
	}
	if len(out) == 0 {
		return "Webdav Error"
	}
	return strings.Join(out, ": ")
}

// Time represents date and time information for the
// webdav API marshalling to and from timeFormat
type Time time.Time

// MarshalXML turns a Time into XML
func (t *Time) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	timeString := (*time.Time)(t).Format(timeFormat)
	return e.EncodeElement(timeString, start)
}

// Possible time formats to parse the time with
var timeFormats = []string{
	timeFormat,     // Wed, 27 Sep 2017 14:28:34 GMT (as per RFC)
	time.RFC1123Z,  // Fri, 05 Jan 2018 14:14:38 +0000 (as used by mydrive.ch)
	time.UnixDate,  // Wed May 17 15:31:58 UTC 2017 (as used in an internal server)
	noZerosRFC1123, // Fri, 7 Sep 2018 08:49:58 GMT (as used by server in #2574)
	time.RFC3339,   // Wed, 31 Oct 2018 13:57:11 CET (as used by komfortcloud.de)
}

var oneTimeError sync.Once

// UnmarshalXML turns XML into a Time
func (t *Time) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var v string
	err := d.DecodeElement(&v, &start)
	if err != nil {
		return err
	}

	// If time is missing then return the epoch
	if v == "" {
		*t = Time(time.Unix(0, 0))
		return nil
	}

	// Parse the time format in multiple possible ways
	var newT time.Time
	for _, timeFormat := range timeFormats {
		newT, err = time.Parse(timeFormat, v)
		if err == nil {
			*t = Time(newT)
			break
		}
	}
	if err != nil {
		oneTimeError.Do(func() {
			fs.Errorf(nil, "Failed to parse time %q - using the epoch", v)
		})
		// Return the epoch instead
		*t = Time(time.Unix(0, 0))
		// ignore error
		err = nil
	}
	return err
}

// Quota is used to read the bytes used and available
//
// <d:multistatus xmlns:d="DAV:" xmlns:s="http://sabredav.org/ns" xmlns:oc="http://owncloud.org/ns" xmlns:nc="http://nextcloud.org/ns">
//
//	<d:response>
//	 <d:href>/remote.php/webdav/</d:href>
//	 <d:propstat>
//	  <d:prop>
//	   <d:quota-available-bytes>-3</d:quota-available-bytes>
//	   <d:quota-used-bytes>376461895</d:quota-used-bytes>
//	  </d:prop>
//	  <d:status>HTTP/1.1 200 OK</d:status>
//	 </d:propstat>
//	</d:response>
//
// </d:multistatus>
type Quota struct {
	Available string `xml:"DAV: response>propstat>prop>quota-available-bytes"`
	Used      string `xml:"DAV: response>propstat>prop>quota-used-bytes"`
}
