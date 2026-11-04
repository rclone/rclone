---
title: "Tarkle"
description: "Rclone docs for Tarkle"
versionIntroduced: "v1.76"
---

# Tarkle

[Tarkle](https://tarkle.com/) is a file sharing and client portal service. This
backend talks to the Tarkle public files API (`/api/v1`) with a workspace API
token.

Large files are uploaded as parallel multipart uploads straight to the storage
provider using short-lived signed URLs, so file data never goes through the API
host. Downloads work the same way and support range requests.

## Configuration

Create an API token with the scopes `files:read` and `files:write`. A token acts
as the person who created it and can do no more than that person's role allows.
Every token works for every workspace its creator can access, so you must say
which workspace this remote uses: set `workspace` to the workspace key (8
characters; rclone sends it as the `X-Workspace-Key` header on every API call
and never to the storage URLs). If you leave it blank, rclone stops and lists
the workspace keys and names to choose from (it never picks one for you, even
if there is only one).

Here is a run through of `rclone config` to make a remote called `remote`.

```console
rclone config
```

Then follow through the interactive setup:

```text
No remotes found, make a new one?
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n

Enter name for new remote.
name> remote

Option Storage.
Type of storage to configure.
Choose a number from below, or type in your own value.
XX / Tarkle
   \ (tarkle)
Storage> tarkle

Option token.
API token.
Enter a value.
token> YOUR_API_TOKEN

Option workspace.
Workspace key (8 characters, as listed by GET /workspaces).
Enter a value. Press Enter to leave empty.
workspace> ab12cd34

Edit advanced config?
y) Yes
n) No (default)
y/n> n

Configuration complete.
Options:
- type: tarkle
- token: YOUR_API_TOKEN
- workspace: ab12cd34
Keep this "remote" remote?
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d> y
```

Once configured you can then use `rclone` like this (replace `remote` with the
name you gave your remote):

List directories and files in the top level of your Tarkle library

```console
rclone lsf remote:
```

To copy a local directory to a Tarkle directory called backup

```console
rclone copy /home/source remote:backup
```

### Modification times and hashes

Tarkle stores the modification time rclone sends when a file is uploaded (the
`modified_at` of the file, to the millisecond; rclone uses a precision of 1
second) and `rclone touch` / `--refresh-times` can change it without uploading
the file again. A file that was uploaded by another client without a time shows
the time it was last updated on Tarkle. Folders have no modification time.

Tarkle supports SHA-256. rclone sends the SHA-256 of the source when it finishes
an upload if the source can give it; for files up to 100 MiB Tarkle reads the
stored file and compares it, and rejects the upload if they differ (rclone then
uploads the file again). For larger files Tarkle only stores the value it was
given, which is the uploader's claim and not verified by Tarkle. Files uploaded
by other clients without a SHA-256 have none, and rclone then compares them by
size and modification time.

### Uploads, duplicates and versions

Rclone must know the size of a file before it starts uploading it. Streams of
unknown size (`rclone rcat`) are spooled to disk first by rclone.

The server decides how a file is cut into parts (the part size and part count)
so there is no `chunk_size` option. Up to `--tarkle-upload-concurrency` parts of
one file are in memory and uploading at the same time, each as large as the part
size the server chose, so memory use per transferred file is about the part size
times the concurrency. Files that rclone decides to copy with several threads
(over `--multi-thread-cutoff`) use the same parts.

If a part fails only that part is sent again (with a fresh signed URL if the
old one expired). If the server finds parts missing when the upload is
finished, rclone reads just those parts from the source again.

Rclone overwrites a file with the same name. On plans where Tarkle keeps
versions, the new content becomes a new version of the existing file. Otherwise
the old file is replaced after the new one is uploaded.

Empty files are created in a single call without an upload.

### Deleting

Deleting a file or folder moves it to the Tarkle bin, where it stays for as long
as the plan's bin retention allows. `rclone purge` moves a folder with all its
contents to the bin. `rclone rmdir` refuses to remove a folder that is not empty.

If a file is part of an active transfer, deleting it fails and tells you so. Use
`--tarkle-delete-transfers` to delete those transfers too. Feedback boards and
paid transfers with purchases can never be deleted through the API.

### Restricted filename characters

In addition to the [default restricted characters set](/overview/#restricted-characters)
the following characters are also replaced:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| \         | 0x5C  | ＼           |

File names can also not start or end with the following characters. These only
get replaced if they are the first or last character in the name:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| SP        | 0x20  | ␠           |

Invalid UTF-8 bytes will also be [replaced](/overview/#invalid-utf8),
as they can't be used in JSON strings.

Names are case insensitive and a file and a folder in the same folder can't
share a name.

<!-- autogenerated options start - DO NOT EDIT - instead edit fs.RegInfo in backend/tarkle/tarkle.go and run make backenddocs to verify --> <!-- markdownlint-disable-line line-length -->
### Standard options

Here are the Standard options specific to tarkle (Tarkle).

#### --tarkle-token

API token.

Create one in the Tarkle workspace settings with the scopes files:read
and files:write. A token acts as the person who created it.

**NB** Input to this must be obscured - see [rclone obscure](/commands/rclone_obscure/).

Properties:

- Config:      token
- Env Var:     RCLONE_TARKLE_TOKEN
- Type:        string
- Required:    true

#### --tarkle-workspace

Workspace key (8 characters, as listed by GET /workspaces).

An account-wide token works for every workspace its creator can access
so this is required: it is sent as X-Workspace-Key on every call (a workspace id is not accepted).
If it is blank, rclone lists the workspace keys and names to choose from.

Properties:

- Config:      workspace
- Env Var:     RCLONE_TARKLE_WORKSPACE
- Type:        string
- Required:    false

### Advanced options

Here are the Advanced options specific to tarkle (Tarkle).

#### --tarkle-url

URL of the Tarkle app (the API is served from <url>/api/v1).

Properties:

- Config:      url
- Env Var:     RCLONE_TARKLE_URL
- Type:        string
- Default:     "https://app.tarkle.com"

#### --tarkle-list-chunk

Number of items to list in each call (1-1000).

Properties:

- Config:      list_chunk
- Env Var:     RCLONE_TARKLE_LIST_CHUNK
- Type:        int
- Default:     1000

#### --tarkle-upload-concurrency

Number of parts of one file uploaded at the same time.

Parts are buffered in memory: up to the part size chosen by the server
(see the docs) times this many per transferred file.

Properties:

- Config:      upload_concurrency
- Env Var:     RCLONE_TARKLE_UPLOAD_CONCURRENCY
- Type:        int
- Default:     4

#### --tarkle-delete-transfers

Also delete the transfers that contain a file or folder being deleted.

Without this, deleting an item that is in an active transfer fails with
a message listing the transfers. Feedback boards and paid transfers
with purchases can never be deleted through the API.

Properties:

- Config:      delete_transfers
- Env Var:     RCLONE_TARKLE_DELETE_TRANSFERS
- Type:        bool
- Default:     false

#### --tarkle-disable-listr

Don't use the recursive listing (GET /files?recursive=true).

Properties:

- Config:      disable_listr
- Env Var:     RCLONE_TARKLE_DISABLE_LISTR
- Type:        bool
- Default:     false

#### --tarkle-encoding

The encoding for the backend.

See the [encoding section in the overview](/overview/#encoding) for more info.

Properties:

- Config:      encoding
- Env Var:     RCLONE_TARKLE_ENCODING
- Type:        Encoding
- Default:     Slash,BackSlash,Del,Ctl,LeftSpace,RightSpace,InvalidUtf8,Dot

#### --tarkle-description

Description of the remote.

Properties:

- Config:      description
- Env Var:     RCLONE_TARKLE_DESCRIPTION
- Type:        string
- Required:    false

### Metadata

Tarkle reports modification and creation times. Modification time is set through the upload and `rclone touch`; creation time is read only.

Here are the possible system metadata items for the tarkle backend.

| Name | Help | Type | Example | Read Only |
|------|------|------|---------|-----------|
| btime | Time of file birth (creation on Tarkle) | RFC 3339 | 2006-01-02T15:04:05.999999999Z07:00 | N |
| mtime | Time of last modification (the uploader's own time if it was set) | RFC 3339 | 2006-01-02T15:04:05.999999999Z07:00 | N |

See the [metadata](/docs/#metadata) docs for more info.

<!-- autogenerated options stop -->

## Limitations

Tarkle only supports filenames up to 255 bytes in length, where filenames are
encoded in UTF8, and paths up to 50 folders deep.

The directory listing of a file is by name lookup in its folder, so `rclone
cat` and similar on a single file in a folder with very many entries lists that
folder first.
