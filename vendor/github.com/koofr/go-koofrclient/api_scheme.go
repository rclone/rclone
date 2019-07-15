package koofrclient

import (
	"path"
)

type TokenRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type Token struct {
	Token string
}

type MountType string

const (
	MountDeviceType = "device"
	MountExportType = "export"
	MountImportType = "import"
)

type Mount struct {
	Id          string           `json:"id"`
	Name        string           `json:"name"`
	Type        MountType        `json:"type"`
	Origin      string           `json:"origin"`
	SpaceTotal  int64            `json:"spaceTotal"`
	SpaceUsed   int64            `json:"spaceUsed"`
	Online      bool             `json:"online"`
	Owner       MountUser        `json:"owner"`
	Users       []MountUser      `json:"users"`
	Groups      []MountGroup     `json:"groups"`
	Version     int              `json:"version"`
	Permissions MountPermissions `json:"permissions"`
	IsPrimary   bool             `json:"isPrimary"`
	IsShared    bool             `json:"isShared"`
}

type MountUser struct {
	Id          string           `json:"id"`
	Name        string           `json:"name"`
	Email       string           `json:"email"`
	Permissions MountPermissions `json:"permissions"`
}

type MountGroup struct {
	Id          string           `json:"id"`
	Name        string           `json:"name"`
	Permissions MountPermissions `json:"permissions"`
}

type MountPermissions struct {
	Read           bool `json:"READ"`
	Write          bool `json:"write"`
	Owner          bool `json:"OWNER"`
	Mount          bool `json:"MOUNT"`
	CreateReceiver bool `json:"CREATE_RECEIVER"`
	CreateLink     bool `json:"CREATE_LINK"`
	CreateAction   bool `json:"CREATE_ACTION"`
	Comment        bool `json:"COMMENT"`
}

type DeviceProvider string

const (
	StorageHubProvider  = "storagehub"
	StorageBlobProvider = "storageblob"
)

type Device struct {
	Id         string `json:"id"`
	ApiKey     string `json:"apiKey"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	SpaceTotal int64  `json:"spaceTotal"`
	SpaceUsed  int64  `json:"spaceUsed"`
	SpaceFree  int64  `json:"spaceFree"`
	Version    int    `json:"version"`
	Provider   struct {
		Name string      `json:"name"`
		Data interface{} `json:"data"`
	} `json:"provider"`
	ReadOnly    bool   `json:"readonly"`
	RootMountId string `json:"rootMountId"`
}

type DeviceCreate struct {
	Name         string         `json:"name"`
	ProviderName DeviceProvider `json:"providerName"`
}

type DeviceUpdate struct {
	Name string `json:"name"`
}

type FolderCreate struct {
	Name string `json:"name"`
}

type FileCopy struct {
	ToMountId string `json:"toMountId"`
	TPath     string `json:"toPath"`
	Modified  *int64 `json:"modified,omitempty"`
}

type FileMove struct {
	ToMountId string `json:"toMountId"`
	TPath     string `json:"toPath"`
}

type FileSpan struct {
	Start int64
	End   int64
}

type FileUpload struct {
	Name string `json:"name"`
}

type PutOptions struct {
	OverwriteIfModified        *int64
	OverwriteIfSize            *int64
	OverwriteIfHash            *string
	OverwriteIgnoreNonExisting bool
	NoRename                   bool
	ForceOverwrite             bool
	SetModified                *int64
}

type CopyOptions struct {
	SetModified *int64
}

type DeleteOptions struct {
	RemoveIfModified *int64
	RemoveIfSize     *int64
	RemoveIfHash     *string
	RemoveIfEmpty    bool
}

type FileInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Modified    int64  `json:"modified"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType"`
	Path        string `json:"path"`
	Hash        string `json:"hash"`
}

type FileTree struct {
	FileInfo
	Children []*FileTree `json:"children"`
}

func (tree *FileTree) Flatten() []FileInfo {
	trees := []*FileTree{tree}
	for i := 0; i < len(trees); i++ {
		tree := trees[i]
		for _, child := range tree.Children {
			child.Name = path.Join(tree.Name, child.Name)
			trees = append(trees, child)
		}
	}
	infos := make([]FileInfo, len(trees))
	for i, tree := range trees {
		infos[i] = tree.FileInfo
	}
	return infos
}

type User struct {
	Id        string `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
}

type Shared struct {
	Name        string    `json:name`
	Type        MountType `json:type`
	Modified    int64     `json:modified`
	Size        int64     `json:size`
	ContentType string    `json:contentType`
	Hash        string    `json:hash`
	Mount       Mount     `json:mount`
	Link        Link      `json:link`
	Receiver    Receiver  `json:receiver`
}

type Link struct {
	Id               string `json:id`
	Name             string `json:name`
	Path             string `json:path`
	Counter          int64  `json:counter`
	Url              string `json:url`
	ShortUrl         string `json:shortUrl`
	Hash             string `json:hash`
	Host             string `json:host`
	HasPassword      bool   `json:hasPassword`
	Password         string `json:password`
	ValidFrom        int64  `json:validFrom`
	ValidTo          int64  `json:validTo`
	PasswordRequired bool   `json:passwordRequired`
}

type Receiver struct {
	Id          string `json:id`
	Name        string `json:name`
	Path        string `json:path`
	Counter     int64  `json:counter`
	Url         string `json:url`
	ShortUrl    string `json:shortUrl`
	Hash        string `json:hash`
	Host        string `json:host`
	HasPassword bool   `json:hasPassword`
	Password    string `json:password`
	ValidFrom   int64  `json:validFrom`
	ValidTo     int64  `json:validTo`
	Alert       bool   `json:alert`
}
