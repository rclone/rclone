// Package api has type definitions for sugarsync
//
// Converted from the API docs with help from https://www.onlinetool.io/xmltogo/
package api

import (
	"encoding/xml"
	"time"
)

// AppAuthorization is used to request a refresh token
//
// The token is returned in the Location: field
type AppAuthorization struct {
	XMLName          xml.Name `xml:"appAuthorization"`
	Username         string   `xml:"username"`
	Password         string   `xml:"password"`
	Application      string   `xml:"application"`
	AccessKeyID      string   `xml:"accessKeyId"`
	PrivateAccessKey string   `xml:"privateAccessKey"`
}

// TokenAuthRequest is the request to get Authorization
type TokenAuthRequest struct {
	XMLName          xml.Name `xml:"tokenAuthRequest"`
	AccessKeyID      string   `xml:"accessKeyId"`
	PrivateAccessKey string   `xml:"privateAccessKey"`
	RefreshToken     string   `xml:"refreshToken"`
}

// Authorization is returned from the TokenAuthRequest
type Authorization struct {
	XMLName    xml.Name  `xml:"authorization"`
	Expiration time.Time `xml:"expiration"`
	User       string    `xml:"user"`
}

// File represents a single file
type File struct {
	Name            string    `xml:"displayName"`
	Ref             string    `xml:"ref"`
	DsID            string    `xml:"dsid"`
	TimeCreated     time.Time `xml:"timeCreated"`
	Parent          string    `xml:"parent"`
	Size            int64     `xml:"size"`
	LastModified    time.Time `xml:"lastModified"`
	MediaType       string    `xml:"mediaType"`
	PresentOnServer bool      `xml:"presentOnServer"`
	FileData        string    `xml:"fileData"`
	Versions        string    `xml:"versions"`
	PublicLink      PublicLink
}

// Collection represents
// - Workspace Collection
// - Sync Folders collection
// - Folder
type Collection struct {
	Type        string    `xml:"type,attr"`
	Name        string    `xml:"displayName"`
	Ref         string    `xml:"ref"` // only for Folder
	DsID        string    `xml:"dsid"`
	TimeCreated time.Time `xml:"timeCreated"`
	Parent      string    `xml:"parent"`
	Collections string    `xml:"collections"`
	Files       string    `xml:"files"`
	Contents    string    `xml:"contents"`
	// Sharing     bool      `xml:"sharing>enabled,attr"`
}

// CollectionContents is the result of a list call
type CollectionContents struct {
	//XMLName     xml.Name     `xml:"collectionContents"`
	Start       int          `xml:"start,attr"`
	HasMore     bool         `xml:"hasMore,attr"`
	End         int          `xml:"end,attr"`
	Collections []Collection `xml:"collection"`
	Files       []File       `xml:"file"`
}

// User is returned from the /user call
type User struct {
	XMLName  xml.Name `xml:"user"`
	Username string   `xml:"username"`
	Nickname string   `xml:"nickname"`
	Quota    struct {
		Limit int64 `xml:"limit"`
		Usage int64 `xml:"usage"`
	} `xml:"quota"`
	Workspaces            string `xml:"workspaces"`
	SyncFolders           string `xml:"syncfolders"`
	Deleted               string `xml:"deleted"`
	MagicBriefcase        string `xml:"magicBriefcase"`
	WebArchive            string `xml:"webArchive"`
	MobilePhotos          string `xml:"mobilePhotos"`
	Albums                string `xml:"albums"`
	RecentActivities      string `xml:"recentActivities"`
	ReceivedShares        string `xml:"receivedShares"`
	PublicLinks           string `xml:"publicLinks"`
	MaximumPublicLinkSize int    `xml:"maximumPublicLinkSize"`
}

// CreateFolder is posted to a folder URL to create a folder
type CreateFolder struct {
	XMLName xml.Name `xml:"folder"`
	Name    string   `xml:"displayName"`
}

// MoveFolder is posted to a folder URL to move a folder
type MoveFolder struct {
	XMLName xml.Name `xml:"folder"`
	Name    string   `xml:"displayName"`
	Parent  string   `xml:"parent"`
}

// CreateSyncFolder is posted to the root folder URL to create a sync folder
type CreateSyncFolder struct {
	XMLName xml.Name `xml:"syncFolder"`
	Name    string   `xml:"displayName"`
}

// CreateFile is posted to a folder URL to create a file
type CreateFile struct {
	XMLName   xml.Name `xml:"file"`
	Name      string   `xml:"displayName"`
	MediaType string   `xml:"mediaType"`
}

// MoveFile is posted to a file URL to create a file
type MoveFile struct {
	XMLName xml.Name `xml:"file"`
	Name    string   `xml:"displayName"`
	Parent  string   `xml:"parent"`
}

// CopyFile copies a file from source
type CopyFile struct {
	XMLName xml.Name `xml:"fileCopy"`
	Source  string   `xml:"source,attr"`
	Name    string   `xml:"displayName"`
}

// PublicLink is the URL and enabled flag for a public link
type PublicLink struct {
	XMLName xml.Name `xml:"publicLink"`
	URL     string   `xml:",chardata"`
	Enabled bool     `xml:"enabled,attr"`
}

// SetPublicLink can be used to enable the file for sharing
type SetPublicLink struct {
	XMLName    xml.Name `xml:"file"`
	PublicLink PublicLink
}

// SetLastModified sets the modified time for a file
type SetLastModified struct {
	XMLName      xml.Name  `xml:"file"`
	LastModified time.Time `xml:"lastModified"`
}
