---
title: "SmugMug"
description: "Rclone docs for SmugMug backend"
versionIntroduced: "v1.76"
---

# SmugMug

This backend works with [SmugMug](https://www.smugmug.com/) in two modes:

- album mode uploads, lists, downloads, replaces, and deletes media in one
  configured album.
- library mode lists SmugMug folders and albums as a filesystem tree when
  `root_node` is configured.

## Configuration

Create a remote with:

```console
rclone config
```

Choose `smugmug`, then configure either `album_uri` or `root_node`.

For album mode, enter the album URI, album key, or a SmugMug web URL. The album
URI looks like `/api/v2/album/AbCdEf`; the key is the final `AbCdEf` part. For
web URLs, rclone resolves the URL to its parent album, so image URLs such as
`https://photos.example.com/2023/My-Album/i-AbCdEf/A` are accepted.

For library mode, set `root_node` to a node URI such as `/api/v2/node/AbCdEf`,
a node ID such as `AbCdEf`, or `root` for the authenticated user's root node.
In this mode folders and albums appear as directories. Uploads must target an
existing album path.

During setup, rclone opens SmugMug's OAuth authorization page and asks for the
six-digit verification code.

The bundled development API key is used by default. Advanced options allow you
to supply your own `api_key` and `api_secret`.

## Usage

Upload a single image:

```console
rclone copyto photo.jpg smug:photo.jpg
```

Upload a directory of images:

```console
rclone copy ./photos smug:
```

List the album:

```console
rclone ls smug:
```

List a library tree:

```console
rclone lsf smuglib:
rclone lsf smuglib:Projects
```

Upload into an album in library mode:

```console
rclone copyto photo.jpg smuglib:Projects/BlueMesa/photo.jpg
```

Create a SmugMug folder in library mode:

```console
rclone mkdir smuglib:Projects/NewFolder
```

Remove an empty SmugMug folder in library mode:

```console
rclone rmdir smuglib:Projects/NewFolder
```

Delete an uploaded image:

```console
rclone deletefile smug:photo.jpg
```

Get the SmugMug web URL for an image, folder, or album:

```console
rclone link smuglib:Projects/BlueMesa/photo.jpg
rclone link smuglib:Projects/BlueMesa
```

## Backend commands

Show the authenticated user and root node:

```console
rclone backend root smug:
```

List folders and albums:

```console
rclone backend list smug: Projects -o recursive=true
rclone backend list-albums smug: -o node=/api/v2/node/AbCdEf
rclone backend list-folders smug: Projects
```

Create folders and albums:

```console
rclone backend create-folder smug: BlueMesa -o path=Projects
rclone backend create-album smug: RiverLight -o path=Projects/BlueMesa -o privacy=Private
```

Creation commands return node and album URIs. Use the returned album URI as
`album_uri`, or browse it through a library-mode remote.

Copy or move images between album paths:

```console
rclone backend copy-image smuglib: Projects/BlueMesa/photo.jpg Projects/RiverLight/photo.jpg
rclone backend move-image smuglib: Projects/BlueMesa/photo.jpg Projects/RiverLight/photo.jpg
```

## Creating albums

Use `create-album` with a parent folder path:

```console
rclone backend create-album smug: RiverLight -o path=Projects
```

Or use the parent folder node URI directly:

```console
rclone backend create-album smug: RiverLight -o parent=/api/v2/node/AbCdEf
```

Set optional SmugMug fields with `-o` options:

```console
rclone backend create-album smug: RiverLight -o path=Projects -o privacy=Unlisted -o url_name=river-light
```

The command returns the new album's `node_uri`, `album_uri`, and `web_uri`.
Verify the album with:

```console
rclone backend list-albums smug:Projects
```

Then upload to the new album by using a library-mode remote:

```console
rclone copyto photo.jpg smuglib:Projects/RiverLight/photo.jpg
```

Or configure another album-mode remote with the returned `album_uri`.

## Copying and moving images

Use normal rclone commands to copy or move an image between albums in a
library-mode remote:

```console
rclone copyto smuglib:Projects/BlueMesa/photo.jpg smuglib:Projects/RiverLight/photo.jpg
rclone moveto smuglib:Projects/BlueMesa/photo.jpg smuglib:Projects/RiverLight/photo.jpg
```

The same operation is also available as backend commands:

```console
rclone backend copy-image smuglib: Projects/BlueMesa/photo.jpg Projects/RiverLight/photo.jpg
rclone backend move-image smuglib: Projects/BlueMesa/photo.jpg Projects/RiverLight/photo.jpg
```

SmugMug's exposed `CopyImage` API does not accept a destination album, so
cross-album copies stream through rclone and upload to the destination. Moves
delete the source image only after the destination upload succeeds.

## Metadata

SmugMug supports rclone metadata for image fields. Use `-M` to preserve
metadata during copies, or set fields on upload with `--metadata-set`:

```console
rclone copyto photo.jpg smuglib:Projects/BlueMesa/photo.jpg --metadata-set title="Cover"
rclone copyto photo.jpg smuglib:Projects/BlueMesa/photo.jpg --metadata-set keywords="travel,landscape"
```

Supported metadata keys are `title`, `caption`, `keywords`, `hidden`,
`latitude`, `longitude`, and `altitude`. Unsupported metadata keys are ignored
when writing.

## Limitations

SmugMug albums are flat media collections. In album mode, rclone represents
paths virtually by encoding slashes in uploaded file names. In library mode,
`mkdir` and `rmdir` manage SmugMug folders only. Use `rclone backend
create-album` when you need a new album.

SmugMug requires a `Content-MD5` header for uploads. If the source does not
provide an MD5 hash, rclone calculates one before upload and caches files larger
than `--smugmug-md5-memory-limit` on disk.

<!-- autogenerated options start - DO NOT EDIT - instead edit fs.RegInfo in backend/smugmug/smugmug.go and run make backenddocs to verify --> <!-- markdownlint-disable-line line-length -->
### Standard options

Here are the Standard options specific to smugmug (SmugMug).

#### --smugmug-album-uri

SmugMug album API URI, album key, or web URL to upload into.

Use values like `/api/v2/album/AbCdEf`, `AbCdEf`, or `https://photos.example.com/2023/My-Album/i-AbCdEf/A`.

Leave blank when using `root_node` library mode.

Properties:

- Config:      album_uri
- Env Var:     RCLONE_SMUGMUG_ALBUM_URI
- Type:        string
- Required:    false

#### --smugmug-root-node

SmugMug root node API URI or node ID for library mode.

When this is set, rclone presents SmugMug folders and albums as a filesystem tree. Use `root` or `authuser` for the authenticated user's root node.

Properties:

- Config:      root_node
- Env Var:     RCLONE_SMUGMUG_ROOT_NODE
- Type:        string
- Required:    false

### Advanced options

Here are the Advanced options specific to smugmug (SmugMug).

#### --smugmug-api-key

SmugMug API key.

Leave blank normally to use rclone's bundled development key.

Properties:

- Config:      api_key
- Env Var:     RCLONE_SMUGMUG_API_KEY
- Type:        string
- Default:     "2FHXLx2mJL8CKgKpSH2nNP995WSh3pDF"

#### --smugmug-api-secret

SmugMug API key secret.

Leave blank normally to use rclone's bundled development key.

**NB** Input to this must be obscured - see [rclone obscure](/commands/rclone_obscure/).

Properties:

- Config:      api_secret
- Env Var:     RCLONE_SMUGMUG_API_SECRET
- Type:        string
- Required:    false

#### --smugmug-access-token

OAuth access token.

This is normally set by `rclone config`.

Properties:

- Config:      access_token
- Env Var:     RCLONE_SMUGMUG_ACCESS_TOKEN
- Type:        string
- Required:    false

#### --smugmug-access-token-secret

OAuth access token secret.

This is normally set by `rclone config`.

**NB** Input to this must be obscured - see [rclone obscure](/commands/rclone_obscure/).

Properties:

- Config:      access_token_secret
- Env Var:     RCLONE_SMUGMUG_ACCESS_TOKEN_SECRET
- Type:        string
- Required:    false

#### --smugmug-md5-memory-limit

Files bigger than this will be cached on disk when rclone must calculate upload MD5.

Properties:

- Config:      md5_memory_limit
- Env Var:     RCLONE_SMUGMUG_MD5_MEMORY_LIMIT
- Type:        SizeSuffix
- Default:     32Mi

#### --smugmug-encoding

The encoding for the backend.

See the [encoding section in the overview](/overview/#encoding) for more info.

Properties:

- Config:      encoding
- Env Var:     RCLONE_SMUGMUG_ENCODING
- Type:        Encoding
- Default:     Slash,Question,Hash,Percent,BackSlash,Del,Ctl,InvalidUtf8,Dot

#### --smugmug-description

Description of the remote.

Properties:

- Config:      description
- Env Var:     RCLONE_SMUGMUG_DESCRIPTION
- Type:        string
- Required:    false

### Metadata

SmugMug image metadata is mapped to SmugMug image fields. Unsupported metadata keys are ignored when writing.

Here are the possible system metadata items for the smugmug backend.

| Name | Help | Type | Example | Read Only |
|------|------|------|---------|-----------|
| altitude | Image altitude in meters | float | 12.5 | N |
| caption | SmugMug image caption | string | Taken from the trail | N |
| hidden | Whether the image is hidden in SmugMug | bool | false | N |
| keywords | SmugMug image keywords | string | travel,landscape | N |
| latitude | Image latitude in decimal degrees | float | 35.681236 | N |
| longitude | Image longitude in decimal degrees | float | 139.767125 | N |
| title | SmugMug image title | string | Trip cover | N |

See the [metadata](/docs/#metadata) docs for more info.

## Backend commands

Here are the commands specific to the smugmug backend.

Run them with:

```console
rclone backend COMMAND remote:
```

The help below will explain what arguments each command takes.

See the [backend](/commands/rclone_backend/) command for more
info on how to pass options and arguments.

These can be run on a running backend using the rc command
[backend/command](/rc/#backend-command).

### root

Show the authenticated SmugMug user and root node.

```console
rclone backend root remote: [options] [<arguments>+]
```

This command shows the authenticated SmugMug account and the root node
URI used for library mode.

Usage examples:

```console
rclone backend root smug:
```

### list

List SmugMug folders and albums under a node or path.

```console
rclone backend list remote: [options] [<arguments>+]
```

This command lists SmugMug folder and album nodes.

By default it lists the configured root_node, or the authenticated user's root
node if root_node is not configured. Use -o node=/api/v2/node/abc123 to list a
specific node, or pass a path relative to the root node.

Usage examples:

```console
rclone backend list smug:
rclone backend list smug: Projects -o recursive=true
rclone backend list smug: -o node=/api/v2/node/abc123
```

Options:

- "node": Node URI or node ID to list.
- "path": Path relative to the root node to list.
- "recursive": Recursively list folders.

### list-folders

List SmugMug folders under a node or path.

```console
rclone backend list-folders remote: [options] [<arguments>+]
```

This is like the list command, but only returns folder nodes.

Usage example:

```console
rclone backend list-folders smug: Projects -o recursive=true
```

Options:

- "node": Node URI or node ID to list.
- "path": Path relative to the root node to list.
- "recursive": Recursively list folders.

### list-albums

List SmugMug albums under a node or path.

```console
rclone backend list-albums remote: [options] [<arguments>+]
```

This is like the list command, but only returns album nodes.

Usage example:

```console
rclone backend list-albums smug: Projects -o recursive=true
```

Options:

- "node": Node URI or node ID to list.
- "path": Path relative to the root node to list.
- "recursive": Recursively list folders.

### create-album

Create a SmugMug album under a folder node.

```console
rclone backend create-album remote: [options] [<arguments>+]
```

This command creates an album below a SmugMug folder node and returns the
new node URI, album URI, and web URL.

Pass the album name as the first argument or with -o name=. Pass the parent
folder with -o parent=/api/v2/node/abc123 or -o path=Projects. Privacy defaults to
Private if not supplied.

Usage examples:

```console
rclone backend create-album smug: "BlueMesa" -o path=Projects
rclone backend create-album smug: -o parent=/api/v2/node/abc123 -o name="New Album" -o privacy=Unlisted
```

Options:

- "name": Album display name.
- "parent": Parent folder node URI or node ID.
- "path": Parent folder path relative to the root node.
- "privacy": Album privacy: Private, Unlisted, or Public. Defaults to Private.
- "url_name": Album URL name.

### create-folder

Create a SmugMug folder under a folder node.

```console
rclone backend create-folder remote: [options] [<arguments>+]
```

This command creates a folder below a SmugMug folder node and returns the
new node URI and web URL.

Pass the folder name as the first argument or with -o name=. Pass the parent
folder with -o parent=/api/v2/node/abc123 or -o path=Projects. Privacy defaults to
Private if not supplied.

Usage examples:

```console
rclone backend create-folder smug: "BlueMesa" -o path=Projects
rclone backend create-folder smug: -o parent=/api/v2/node/abc123 -o name="BlueMesa"
```

Options:

- "name": Folder display name.
- "parent": Parent folder node URI or node ID.
- "path": Parent folder path relative to the root node.
- "privacy": Folder privacy: Private, Unlisted, or Public. Defaults to Private.
- "url_name": Folder URL name.

### copy-image

Copy a SmugMug image to another album path.

```console
rclone backend copy-image remote: [options] [<arguments>+]
```

This command copies one SmugMug image to another path. It can copy
between albums in library mode when the destination path is inside an existing
album.

SmugMug's exposed copy API does not accept a destination album, so this command
streams the image through rclone and uploads it to the destination.

Usage examples:

```console
rclone backend copy-image smuglib: Projects/BlueMesa/photo.jpg Projects/RiverLight/photo.jpg
rclone backend copy-image smuglib: -o src=Projects/BlueMesa/photo.jpg -o dst=Projects/RiverLight/photo.jpg
```

Options:

- "dst": Destination image path.
- "src": Source image path.

### move-image

Move a SmugMug image to another album path.

```console
rclone backend move-image remote: [options] [<arguments>+]
```

This command moves one SmugMug image to another path. It uploads the
image to the destination first, then removes the source image only after the
upload succeeds.

SmugMug's exposed copy API does not accept a destination album, so this command
streams the image through rclone and uploads it to the destination.

Usage examples:

```console
rclone backend move-image smuglib: Projects/BlueMesa/photo.jpg Projects/RiverLight/photo.jpg
rclone backend move-image smuglib: -o src=Projects/BlueMesa/photo.jpg -o dst=Projects/RiverLight/photo.jpg
```

Options:

- "dst": Destination image path.
- "src": Source image path.

<!-- autogenerated options stop -->
