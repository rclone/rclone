package upnpav

import (
	"encoding/xml"
)

const (
	NoSuchObjectErrorCode = 701
)

type Resource struct {
	XMLName      xml.Name `xml:"res"`
	ProtocolInfo string   `xml:"protocolInfo,attr"`
	URL          string   `xml:",chardata"`
	Size         uint64   `xml:"size,attr,omitempty"`
	Bitrate      uint     `xml:"bitrate,attr,omitempty"`
	Duration     string   `xml:"duration,attr,omitempty"`
	Resolution   string   `xml:"resolution,attr,omitempty"`
}

type Container struct {
	Object
	XMLName    xml.Name `xml:"container"`
	ChildCount int      `xml:"childCount,attr"`
}

type Item struct {
	Object
	XMLName xml.Name `xml:"item"`
	Res     []Resource
}

type Object struct {
	ID          string `xml:"id,attr"`
	ParentID    string `xml:"parentID,attr"`
	Restricted  int    `xml:"restricted,attr"` // indicates whether the object is modifiable
	Class       string `xml:"upnp:class"`
	Icon        string `xml:"upnp:icon,omitempty"`
	Title       string `xml:"dc:title"`
	Artist      string `xml:"upnp:artist,omitempty"`
	Album       string `xml:"upnp:album,omitempty"`
	Genre       string `xml:"upnp:genre,omitempty"`
	AlbumArtURI string `xml:"upnp:albumArtURI,omitempty"`
	Searchable  int    `xml:"searchable,attr"`
}
