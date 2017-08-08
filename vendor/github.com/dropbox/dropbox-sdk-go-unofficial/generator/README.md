# Dropbox Go SDK Generator

This directory contains the [Stone](https://github.com/dropbox/stone) code generators
used to programmatically generate the [Dropbox Go SDK](https://github.com/dropbox/dropbox-sdk-go).

## Requirements

  * While not a hard requirement, this repo currently assumes `python3` in the path.
  * Assumes you have already installed [Stone](https://github.com/dropbox/stone)
  * Requires [goimports](https://godoc.org/golang.org/x/tools/cmd/goimports) to fix up imports in the auto-generated code

## Basic Setup

  . Clone this repo
  . Run `git submodule init` followed by `git submodule update`
  . Run `./generate-sdk.sh` to generate code under `../dropbox`

## Generated Code

### Basic Types

Here is how Stone [basic types](https://github.com/dropbox/stone/blob/master/doc/lang_ref.rst#basic-types) map to Go types:

Stone Type | Go Type
---------- | -------
Int32/Int64/UInt32/UInt64 | int32/int64/uint32/uint64
Float32/Float64 | float32/float64
Boolean | bool
String | string
Timestamp | time.Time
Void | struct{}

### Structs

Stone [structs](https://github.com/dropbox/stone/blob/master/doc/lang_ref.rst#struct) are represented as Go [structs](https://gobyexample.com/structs) in a relatively straight-forward manner. Each struct member is exported and also gets assigned the correct json tag. The latter is used for serializing requests and deserializing responses. Non-primitive types are represented as pointers to the corresponding type.

```
struct Account
    "The amount of detail revealed about an account depends on the user
    being queried and the user making the query."

    account_id AccountId
        "The user's unique Dropbox ID."
    name Name
        "Details of a user's name."
```

```go
// The amount of detail revealed about an account depends on the user being
// queried and the user making the query.
type Account struct {
	// The user's unique Dropbox ID.
	AccountId string `json:"account_id"`
	// Details of a user's name.
	Name *Name `json:"name"`
}
```

#### Inheritance

Stone supports [struct inheritance](https://github.com/dropbox/stone/blob/master/doc/lang_ref.rst#inheritance). In Go, we support this via [embedding](https://golang.org/doc/effective_go.html#embedding)

```
struct BasicAccount extends Account
    "Basic information about any account."

    is_teammate Boolean
        "Whether this user is a teammate of the current user. If this account
        is the current user's account, then this will be :val:`true`."
```

```go
// Basic information about any account.
type BasicAccount struct {
	Account
	// Whether this user is a teammate of the current user. If this account is
	// the current user's account, then this will be `True`.
	IsTeammate bool `json:"is_teammate"`
```

### Unions

Stone https://github.com/dropbox/stone/blob/master/doc/lang_ref.rst#union[unions] are bit more complex as Go doesn't have native support for union types (tagged or otherwise). We declare a union as a Go struct with all the possible fields as pointer types, and then use the tag value to populate the correct field during deserialization. This necessitates the use of an intermediate wrapper struct for the deserialization to work correctly, see below for a concrete example.

```
union SpaceAllocation
    "Space is allocated differently based on the type of account."

    individual IndividualSpaceAllocation
        "The user's space allocation applies only to their individual account."
    team TeamSpaceAllocation
        "The user shares space with other members of their team."
```

```go
// Space is allocated differently based on the type of account.
type SpaceAllocation struct {
	dropbox.Tagged
	// The user's space allocation applies only to their individual account.
	Individual *IndividualSpaceAllocation `json:"individual,omitempty"`
	// The user shares space with other members of their team.
	Team *TeamSpaceAllocation `json:"team,omitempty"`
}

// Valid tag values for `SpaceAllocation`
const (
	SpaceAllocation_Individual = "individual"
	SpaceAllocation_Team       = "team"
	SpaceAllocation_Other      = "other"
)

func (u *SpaceAllocation) UnmarshalJSON(body []byte) error {
	type wrap struct {
		dropbox.Tagged
		// The user's space allocation applies only to their individual account.
		Individual json.RawMessage `json:"individual,omitempty"`
		// The user shares space with other members of their team.
		Team json.RawMessage `json:"team,omitempty"`
	}
	var w wrap
	if err := json.Unmarshal(body, &w); err != nil {
		return err
	}
	u.Tag = w.Tag
	switch u.Tag {
	case "individual":
		if err := json.Unmarshal(body, &u.Individual); err != nil {
			return err
		}

	case "team":
		if err := json.Unmarshal(body, &u.Team); err != nil {
			return err
		}

	}
	return nil
}
```

### Struct with Enumerated Subtypes

Per the https://github.com/dropbox/stone/blob/master/doc/lang_ref.rst#struct-polymorphism[spec], structs with enumerated subtypes are a mechanism of inheritance:

> If a struct enumerates its subtypes, an instance of any subtype will satisfy the type constraint. This is useful when wanting to discriminate amongst types that are part of the same hierarchy while simultaneously being able to avoid discriminating when accessing common fields.

To represent structs with enumerated subtypes in Go, we use a combination of Go interface types and unions as implemented above. Considering the following:

```
struct Metadata
    union
        file FileMetadata
        folder FolderMetadata
        deleted DeletedMetadata  # Used by list_folder* and search

    name String
    path_lower String?
    path_display String?
    parent_shared_folder_id common.SharedFolderId?
    
struct FileMetadata extends Metadata
    id Id
    client_modified common.DropboxTimestamp
    ...
```

In this case, `FileMetadata`, `FolderMetadata` etc are subtypes of `Metadata`. Specifically, any subtype can be used where a parent type is expected. Thus, if `list_folder` returns a list of `Metadata`s, we should be able to parse and "upcast" to one of the enumerated subtypes.

First, we define structs to represent the base and enumerated types as we did for inherited structs above:

```go
type Metadata struct {
	Name string `json:"name"`
	PathLower string `json:"path_lower,omitempty"`
	PathDisplay string `json:"path_display,omitempty"`
	ParentSharedFolderId string `json:"parent_shared_folder_id,omitempty"`
}

type FileMetadata struct {
	Metadata
	Id string `json:"id"`
	ClientModified time.Time `json:"client_modified"`
	...
}
```

Next, we define an interface type with a dummy method and ensure that both the base and the subtypes implement the interface:

```go
type IsMetadata interface {
	IsMetadata()
}

func (u *Metadata) IsMetadata() {} // Subtypes get this for free due to embedding
```

At this point, types or methods that accept/return a struct with enumerated subtypes can use the interface type instead. For instance:

```go
func GetMetadata(arg *GetMetadataArg) (res IsMetadata, err error) {...}

type ListFolderResult struct {
	// The files and (direct) subfolders in the folder.
	Entries []IsMetadata `json:"entries"`
	...
}
```

Finally, to actually deserialize a bag of bytes into the appropriate type or subtype, we use a trick similar to how we handle unions above.

```go
type metadataUnion struct {
	dropbox.Tagged
	File    *FileMetadata    `json:"file,omitempty"`
	Folder  *FolderMetadata  `json:"folder,omitempty"`
	Deleted *DeletedMetadata `json:"deleted,omitempty"`
}

func (u *metadataUnion) UnmarshalJSON(body []byte) error {...}

func (dbx *apiImpl) GetMetadata(arg *GetMetadataArg) (res IsMetadata, err error) {
   	...
   	var tmp metadataUnion
	err = json.Unmarshal(body, &tmp)
	if err != nil {
		return
	}
	switch tmp.Tag {
	case "file":
		res = tmp.File
	case "folder":
		res = tmp.Folder
	case "deleted":
		res = tmp.Deleted
	}
}
```
