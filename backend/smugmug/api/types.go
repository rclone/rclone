// Package api contains SmugMug API response types.
package api

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Response is a generic SmugMug API response envelope.
type Response struct {
	Response struct {
		URI        string       `json:"Uri"`
		Album      *Album       `json:"Album"`
		AlbumImage []AlbumImage `json:"AlbumImage"`
		Image      []AlbumImage `json:"Image"`
		Node       *Node        `json:"Node"`
		Pages      Pages        `json:"Pages"`
	} `json:"Response"`
	Code    int    `json:"Code"`
	Message string `json:"Message"`
}

// Link is a SmugMug URI link.
type Link struct {
	URI string `json:"Uri"`
}

// UnmarshalJSON accepts both short URI strings and expanded link objects.
func (l *Link) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		l.URI = s
		return nil
	}
	type link Link
	return json.Unmarshal(b, (*link)(l))
}

// StringList is a SmugMug string or string-list field.
type StringList string

// UnmarshalJSON accepts either a string or a string array.
func (l *StringList) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*l = StringList(s)
		return nil
	}
	var ss []string
	if err := json.Unmarshal(b, &ss); err == nil {
		*l = StringList(strings.Join(ss, ","))
		return nil
	}
	return fmt.Errorf("unexpected string list value %s", string(b))
}

// Float is a SmugMug float field that may be encoded as a string.
type Float struct {
	value float64
	valid bool
}

// UnmarshalJSON accepts either a JSON number or numeric string.
func (f *Float) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}
	var n float64
	if err := json.Unmarshal(b, &n); err == nil {
		f.value = n
		f.valid = true
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		if strings.TrimSpace(s) == "" {
			return nil
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err != nil {
			return err
		}
		f.value = value
		f.valid = true
		return nil
	}
	return fmt.Errorf("unexpected float value %s", string(b))
}

// Ptr returns the float value as a pointer when present.
func (f Float) Ptr() *float64 {
	if !f.valid {
		return nil
	}
	value := f.value
	return &value
}

// Value returns the float value and whether it was present.
func (f Float) Value() (float64, bool) {
	return f.value, f.valid
}

// Album is a SmugMug album.
type Album struct {
	URI     string          `json:"Uri"`
	Name    string          `json:"Name"`
	URLName string          `json:"UrlName"`
	WebURI  string          `json:"WebUri"`
	Uris    map[string]Link `json:"Uris"`
}

// Node is a SmugMug folder or album node.
type Node struct {
	Name         string          `json:"Name"`
	URI          string          `json:"Uri"`
	NodeID       string          `json:"NodeID"`
	Type         string          `json:"Type"`
	URLName      string          `json:"UrlName"`
	URLPath      string          `json:"UrlPath"`
	WebURI       string          `json:"WebUri"`
	DateAdded    string          `json:"DateAdded"`
	DateModified string          `json:"DateModified"`
	Uris         map[string]Link `json:"Uris"`
}

// Pages describes paginated SmugMug responses.
type Pages struct {
	NextPage string `json:"NextPage"`
}

// AuthUserResponse is the authenticated user response.
type AuthUserResponse struct {
	Response struct {
		User User `json:"User"`
	} `json:"Response"`
}

// User is a SmugMug user.
type User struct {
	Name     string          `json:"Name"`
	NickName string          `json:"NickName"`
	URI      string          `json:"Uri"`
	WebURI   string          `json:"WebUri"`
	Uris     map[string]Link `json:"Uris"`
}

// NodeListResponse is a child node listing response.
type NodeListResponse struct {
	Response struct {
		Node  []Node `json:"Node"`
		Pages Pages  `json:"Pages"`
	} `json:"Response"`
}

// AlbumImage is a SmugMug album image.
type AlbumImage struct {
	URI          string          `json:"Uri"`
	FileName     string          `json:"FileName"`
	Title        string          `json:"Title"`
	Caption      string          `json:"Caption"`
	Keywords     StringList      `json:"Keywords"`
	Hidden       *bool           `json:"Hidden"`
	Latitude     Float           `json:"Latitude"`
	Longitude    Float           `json:"Longitude"`
	Altitude     Float           `json:"Altitude"`
	ArchivedURI  string          `json:"ArchivedUri"`
	ArchivedSize int64           `json:"ArchivedSize"`
	ArchivedMD5  string          `json:"ArchivedMD5"`
	OriginalSize int64           `json:"OriginalSize"`
	Size         int64           `json:"Size"`
	Date         string          `json:"Date"`
	LastUpdated  string          `json:"LastUpdated"`
	Format       string          `json:"Format"`
	MimeType     string          `json:"MimeType"`
	WebURI       string          `json:"WebUri"`
	Uris         map[string]Link `json:"Uris"`
}

// UploadResponse is the JSON response from upload.smugmug.com.
type UploadResponse struct {
	Stat    string `json:"stat"`
	Message string `json:"message"`
	Image   struct {
		ImageURI      string `json:"ImageUri"`
		AlbumImageURI string `json:"AlbumImageUri"`
		URL           string `json:"URL"`
	} `json:"Image"`
}
