// Package upnpav provides utilities for DLNA server.
package upnpav

import (
	"encoding/xml"
	"time"
)

const (
	// NoSuchObjectErrorCode : The specified ObjectID is invalid.
	NoSuchObjectErrorCode = 701
)

// Resource description
type Resource struct {
	XMLName      xml.Name `xml:"res"`
	ProtocolInfo string   `xml:"protocolInfo,attr"`
	URL          string   `xml:",chardata"`
	Size         uint64   `xml:"size,attr,omitempty"`
	Bitrate      uint     `xml:"bitrate,attr,omitempty"`
	Duration     string   `xml:"duration,attr,omitempty"`
	Resolution   string   `xml:"resolution,attr,omitempty"`
}

// Container description
type Container struct {
	Object
	XMLName    xml.Name `xml:"container"`
	ChildCount *int     `xml:"childCount,attr"`
}

// Item description
type Item struct {
	Object
	XMLName  xml.Name `xml:"item"`
	Res      []Resource
	InnerXML string `xml:",innerxml"`
}

// Object description
type Object struct {
	ID          string    `xml:"id,attr"`
	ParentID    string    `xml:"parentID,attr"`
	Restricted  int       `xml:"restricted,attr"` // indicates whether the object is modifiable
	Class       string    `xml:"upnp:class"`
	Icon        string    `xml:"upnp:icon,omitempty"`
	Title       string    `xml:"dc:title"`
	Date        Timestamp `xml:"dc:date"`
	Artist      string    `xml:"upnp:artist,omitempty"`
	Album       string    `xml:"upnp:album,omitempty"`
	Genre       string    `xml:"upnp:genre,omitempty"`
	AlbumArtURI string    `xml:"upnp:albumArtURI,omitempty"`
	Searchable  int       `xml:"searchable,attr"`
}

// Timestamp wraps time.Time for formatting purposes
type Timestamp struct {
	time.Time
}

// MarshalXML formats the Timestamp per DIDL-Lite spec
func (t Timestamp) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	return e.EncodeElement(t.Format("2006-01-02"), start)
}
