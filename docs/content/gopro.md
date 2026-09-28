---
title: "GoPro Media Library"
description: "Rclone docs for GoPro Media Library"
versionIntroduced: "v1.76"
---

# GoPro Media Library

[GoPro Media Library](https://gopro.com/media-library/) is GoPro's cloud
storage for photos and video. GoPro does not publish an API for it: this
backend is built by reverse engineering the gopro.com web app,
cross-checked against several community clients
([dustin/gopro-plus](https://github.com/dustin/gopro-plus),
[mvisonneau/gpcd](https://github.com/mvisonneau/gpcd),
[aricha/GoProcure](https://github.com/aricha/GoProcure),
[itsankoff/gopro-plus](https://github.com/itsankoff/gopro-plus)) and
verified against a live account. **GoPro can change or remove this API at
any time without notice** - if this backend suddenly stops working, that is
likely why.

Paths are specified as `remote:path`. The layout is virtual (see
[Directory layout](#directory-layout) below) rather than a container/path
scheme, so most paths will just be `remote:media/all` or similar.

## Configuration

Here is an example of making a remote for GoPro Media Library.

First run:

```console
rclone config
```

This will guide you through an interactive setup process:

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
XX / GoPro Media Library
   \ (gopro)
Storage> gopro

Option user.
GoPro account email.
Enter a value. Press Enter to leave empty.
user> you@example.com

Option pass.
GoPro account password.
Enter a value. Press Enter to leave empty.
y) Yes, type in my own password
g) Generate random password
n) No, leave this optional password blank
y/g/n> y
Enter the password:
password:
Confirm the password:
password:

Edit advanced config?
y/n> n

Keep this "remote" remote?
y/e/d> y
```

This runs a standard OAuth2 password grant against GoPro's own token
endpoint and stores a refresh token, so it does not need to be repeated -
`rclone` will renew the access token automatically. If GoPro rejects that
refresh outright (confirmed live: a stored token can end up blacklisted
server-side, not just expired), this backend automatically falls back to
running the same password grant again using the stored username/password,
the same recovery `rclone config reconnect` performs manually, so a normal
command self-heals instead of failing until someone runs that by hand. If
your account can't complete the password grant at all (for example because
it requires interactive 2FA that this flow doesn't support), see
[`--gopro-access-token`](#gopro-access-token) for a fallback - static
tokens have no refresh or recovery of any kind, so expect to paste in a new
one by hand periodically.

Once configured you can use it like any other remote:

```console
rclone lsd remote:media
rclone copy remote:media/by-year/2026 /path/to/backup
rclone mount remote:media/all /mnt/gopro
```

## Directory layout

GoPro Media Library has no folders of its own - it's a flat, ID-keyed library.
This backend presents a virtual directory tree over it, in the same style
as the [Google Photos](/googlephotos/#layout) backend:

```text
media/
├── all/                       every ready media item, flat
├── by-year/YYYY/
├── by-month/YYYY/YYYY-MM/
└── by-day/YYYY/YYYY-MM-DD/
upload/                        files rclone has uploaded this run
```

`by-year`/`by-month`/`by-day` are filtered by `captured_at`, in UTC - not
the timezone the camera recorded in, since the API gives no way to filter
on that. `media/all` and the date-filtered views all show the same
underlying items; nothing needs to be uploaded more than once to appear in
more than one of them.

None of these directories can be created, renamed or removed - `rclone
mkdir`/`rmdir` on anything under `media/` fails, since they already exist
whenever there is a matching pattern and there's nothing real underneath to
delete. Only `upload/` accepts new files, and only `upload/` supports
`mkdir`/`rmdir` for organising them into subdirectories.

### Duplicate filenames

GoPro cameras recycle filenames constantly (`GX010123.MP4` turns up over
and over across different recording sessions). Whenever a directory listing
contains two items with the same name, both are renamed to
`name {id}.ext`, where `id` is the item's GoPro media ID. This mirrors how
the Google Photos backend handles the same problem.

By default ([`--gopro-always-add-id`](#gopro-always-add-id)) every file
gets this treatment, not just ones actually colliding in a given listing.
Whether a particular file collides depends on what else happens to exist
in the library at listing time, which isn't stable from one run to the
next - a file uploaded today as `GX010123.MP4` can silently become
`GX010123 {id}.MP4` the moment some unrelated second `GX010123.MP4` turns
up elsewhere, with nothing about the original file itself having changed.
rclone has no way to know the old and new names are the same file, so
`sync` would delete and re-transfer it, and `copy` would leave a stale
duplicate behind under the old name, forever. Always including the ID
makes a file's name a stable function of the file itself, not of whatever
else happens to be in the library that day. Turn this off for cleaner
names if the library is small/static enough that a same-name collision is
not a realistic concern.

### Chaptered videos and burst photos

A single library entry can be made of several files: GoPro splits a long
continuous recording into numbered chapters, and a burst photo shoot is
stored as one entry with dozens (sometimes 100+) of numbered frames. This
backend exposes each one as a separate file, named `name-N.ext` (for
example `GX010294-1.MP4`, `GX010294-2.MP4`).

Because the size reported by the API for a multi-item entry is the *total*
across every item, not any one item's size, `rclone size` and `rclone
ls`/`lsl` show `-1` (unknown) for these files by default rather than a
guess - an inaccurate guess would make rclone's own transfer integrity
check fail on every download. Set
[`--gopro-read-size`](#gopro-read-size) if you need exact sizes for these
(for example for `rclone mount`), at the cost of one extra request per
file.

### Highlights and Edits

GoPro-generated Highlight reels and user-made Edits (`MultiClipEdit`
and `Edit` media types) are composed from other clips rather than being
their own camera-original recording. They're included by default,
matching what GoPro's own web/app library shows - see
[`--gopro-include-edits`](#gopro-include-edits) to exclude them.

These behave differently enough from ordinary media to be worth calling
out even once included: their own `file_extension` is that of an
internal Edit Decision List (typically `json`), not of what actually
gets downloaded - GoPro serves the rendered video (a `baked_source`
rendition) for these, never the EDL, and this backend's reported
Content-Type follows the filename's own extension (usually `.mp4`)
rather than `file_extension`, to match what's actually served. Their
`file_size` is always null, reported the same way as the multi-item
files above (`-1`, unknown, resolved on demand via
[`--gopro-read-size`](#gopro-read-size)) rather than skipped.
Auto-generated Highlights often have no filename at all; these are
listed as `{id}.mp4`.

### Size verification

The `file_size` GoPro's API reports for a media item can be wrong - seen
live, a few KB larger than the size its "source" rendition actually
serves. This isn't just cosmetic: `rclone copy`'s multi-thread downloader
divides a file into ranged chunks using the size known *before* the
download starts, so a too-large size makes it request a chunk that runs
past the real end of the file and fails the whole transfer (`failed to
write chunk: expected ... but wrote ...`); and a sync that only ever saw
the stale size would re-download an already-correct file on every single
run, forever, since the sizes would never match.

[`--gopro-verify-size`](#gopro-verify-size) controls which files get
checked against a live response before relying on their size, correcting
and logging a `NOTICE`-level warning whenever one is wrong (the file is
still downloaded either way - the size actually served is trustworthy).
By default this only checks files GoPro has reprocessed since upload,
the one thing a live account probe found in common with the one affected
file out of hundreds checked - colder storage alone isn't enough, most
archived files still report correctly. Set it to `always` for the
strongest guarantee at the cost of one extra request per file, or `off`
to skip the check entirely and trust `file_size` as-is. Either way this
is on top of what [`--gopro-read-size`](#gopro-read-size) already costs
for a multi-item file.

## Modification times and hashes

GoPro Media Library reports a `captured_at` timestamp for every item,
which this backend uses as the modification time. Confirmed live, this
isn't actually fixed at upload time the way most backends' equivalent is
- `PUT /media/{id}` can change it - so `SetModTime` is implemented
(`rclone touch` and similar work). This changes GoPro's own record of
when the medium was captured, not just a local label, so treat it
accordingly.

That said, this backend still reports its modtime precision as
unsupported, deliberately: `rclone sync`/`copy` don't use modification
time to decide what needs transferring here, so an ordinary sync run
never calls `SetModTime` as a side effect and won't silently rewrite
capture dates just because a local file's timestamp doesn't exactly
match. It's only invoked when something asks for it directly - `rclone
touch`, or a `Move` across a `by-year`/`by-month`/`by-day` boundary (see
"Renaming and moving files" below).

There is no supported hash algorithm, so `--checksum` cannot be used;
`rclone sync` falls back to comparing size alone, which for the
multi-item files described above means it's unavailable unless
`--gopro-read-size` is set.

## Which file gets downloaded

`_embedded.files[]` in GoPro's API is a transcoded proxy, not the camera
original - confirmed on a real account, where it resolved to a 1080p
rendition of a video actually shot in 4K. By default
(`--gopro-download-variation source`) this backend instead downloads the
rendition labelled `"source"`, which is the true original. Set
`--gopro-download-variation` to something else (for example `1080p`, or a
proxy label like `high_res_proxy_mp4`) to download a transcoded rendition
instead, if you want smaller/faster transfers and don't need the original.

## Renaming and moving files

`Move` is implemented (`rclone moveto`, and `rclone move`/`sync` for
files that already exist at the destination under a different name) -
confirmed live, a medium's filename isn't fixed after upload the way it
is on most backends: `PUT /media/{id}` renames it in place.

Moving within `media/all`, or within the same `by-year`/`by-month`/`by-day`
bucket, only renames the file. Moving across a `by-year`, `by-month` or
`by-day` boundary (for example `media/by-day/2026/2026-08-28/x.mp4` to
`media/by-day/2026/2026-08-29/x.mp4`) also changes `captured_at` to match
the destination date, since those directories are views computed from it
- this is the one way a move can actually reposition a file between them,
not just cosmetic, so treat it with the same care as
[`SetModTime`](#modification-times-and-hashes) above. A multi-item file
(a chaptered video or burst photo set item) can't be moved individually -
the API renames the whole medium, not one chapter or frame of it.

GoPro stores the name as both the filename and the title shown in its
app, so a move that keeps the name (only changing the date) leaves both
untouched. A file can't be renamed to a name with nothing before its
extension.

`Copy` and `DirMove` remain unimplemented - see "Limitations" below.

## Link sharing

`PublicLink` is implemented (`rclone link remote:path`), creating a
public share that needs no authentication to view or download from -
confirmed live with a fresh upload and a plain unauthenticated request
against the returned URL. GoPro calls the underlying object a
"collection" internally; this backend always creates one holding just
the single file being shared, and the link is
`https://gopro.com/v/{collection-id}`.

The link's title defaults to the file's own name (with this backend's
own `{id}` disambiguation suffix stripped, since that's never meant to
be shown outside this backend) - set
[`--gopro-link-title`](#gopro-link-title) for a custom one; `rclone
link` itself has no way to pass a one-off title per call, so this
applies for the remote's lifetime, not just the next link created.
[`--gopro-link-allow-download`](#gopro-link-allow-download) controls
whether the link also allows downloading the original file and sharing
any GPS data embedded in it - off by default; GoPro's API has one
field for both, confirmed against its own web UI, so they can't be set
independently.

`--expire` and `--unlink` are not supported and are silently ignored,
per this command's own documented behaviour for backends that can't:
GoPro's collections API exposes no expiry field, so links don't expire,
and a medium can be referenced by any number of independent shares with
no way to look up which ones from the medium's own record, so there's
no single reliable "the" link to remove on request.

## Deleting files

By default, `rclone delete`/`rclone rmdir`/removing a file during
`rclone sync` doesn't actually make GoPro forget about it - confirmed
live, GoPro moves it to what its own web/app UI calls "Recently
Deleted" instead. It's recoverable there for up to 60 days, and still
counts against your storage quota, even though every listing and
`NewObject` lookup this backend does already correctly treats it as
gone. Set [`--gopro-use-trash=false`](#gopro-use-trash) to skip that
and delete permanently instead - confirmed live, this still takes
GoPro roughly a minute to actually process in the background, not
instant, but it's gone for good once it does, unlike the 60-day
recoverable default.

## Uploading

Files uploaded to `upload/` are always sent in chunks (there's no
single-shot upload endpoint), read from the source in order but PUT to
GoPro concurrently - `--gopro-upload-concurrency` chunks in flight at
once, `--gopro-upload-chunk-size` bytes each. GoPro's chunk upload
accepts parts in any order, so this is safe; it mainly speeds up large
single-file uploads, since each chunk is a separate HTTP round trip.
`--gopro-upload-chunk-size` can't go below 5Mi: GoPro's upload endpoint
is S3-backed and enforces that as the minimum part size (except for the
last part of a file). `PutStream` isn't supported - the protocol needs
the file size before the first chunk is requested.

Chunk buffers come from rclone's shared memory pool rather than being
allocated per chunk, so an upload's buffer memory is subject to
[`--max-buffer-memory`](/docs/#max-buffer-memory) and
[`--use-mmap`](/docs/#use-mmap) like any other chunked-upload backend's.

<!-- autogenerated options start - DO NOT EDIT - instead edit fs.RegInfo in backend/gopro/gopro.go and run make backenddocs to verify --> <!-- markdownlint-disable-line line-length -->
### Standard options

Here are the Standard options specific to gopro (GoPro Media Library).

#### --gopro-user

GoPro account email.

Leave blank if using access_token instead.

Properties:

- Config:      user
- Env Var:     RCLONE_GOPRO_USER
- Type:        string
- Required:    false

#### --gopro-pass

GoPro account password.

Leave blank if using access_token instead.

**NB** Input to this must be obscured - see [rclone obscure](/commands/rclone_obscure/).

Properties:

- Config:      pass
- Env Var:     RCLONE_GOPRO_PASS
- Type:        string
- Required:    false

### Advanced options

Here are the Advanced options specific to gopro (GoPro Media Library).

#### --gopro-access-token

Static bearer token, as an alternative to user/pass.

Copy the value of the gp_access_token cookie from a browser session
logged into gopro.com/media-library. This does not refresh, so it
will stop working (typically within a few hours) and need pasting in
again - prefer user/pass unless your account can't complete that
flow.

Properties:

- Config:      access_token
- Env Var:     RCLONE_GOPRO_ACCESS_TOKEN
- Type:        string
- Required:    false

#### --gopro-download-variation

Which rendition to download.

"source" (the default) downloads the original camera file, or the
rendered video for Highlights and Edits. Any other value is matched
against the label or quality of the renditions GoPro offers (for
example "1080p" or "high_res_proxy_mp4"), falling back to the first
file offered if nothing matches.

Properties:

- Config:      download_variation
- Env Var:     RCLONE_GOPRO_DOWNLOAD_VARIATION
- Type:        string
- Default:     "source"

#### --gopro-include-edits

Include Highlights and user-made Edits in listings.

These "MultiClipEdit"/"Edit" media are rendered from other clips.
GoPro reports no size for them, so their size is unknown unless
[--gopro-read-size](#gopro-read-size) is set (e.g. for rclone mount).
The rendered video is downloaded and they are listed as ".mp4" -
unnamed auto-generated Highlights as "{id}.mp4".

Turn this off to list only camera originals.

Properties:

- Config:      include_edits
- Env Var:     RCLONE_GOPRO_INCLUDE_EDITS
- Type:        bool
- Default:     true

#### --gopro-include-processing

Include media GoPro hasn't finished processing yet.

By default only media in the "ready" state is listed. This adds the
"uploading", "registered", "transcoding" and "stabilizing" states.
Media in these states is often downloadable already, but items with no
file size yet are still skipped.

Properties:

- Config:      include_processing
- Env Var:     RCLONE_GOPRO_INCLUDE_PROCESSING
- Type:        bool
- Default:     false

#### --gopro-include-failed

Include media stuck in a "failure" or "unknown" state.

Such media may have no usable content. This is mainly useful to find
and remove stuck items.

Properties:

- Config:      include_failed
- Env Var:     RCLONE_GOPRO_INCLUDE_FAILED
- Type:        bool
- Default:     false

#### --gopro-show-all

List everything in the library, bypassing all filters.

This ignores [--gopro-include-edits](#gopro-include-edits),
[--gopro-include-processing](#gopro-include-processing) and
[--gopro-include-failed](#gopro-include-failed), and also lists
"export" media (internal renders GoPro's own app never shows). It can
surface media this backend doesn't know how to handle, so use it for
troubleshooting rather than normal browsing.

It has no effect with [--gopro-trashed-only](#gopro-trashed-only),
which always lists everything.

Properties:

- Config:      show_all
- Env Var:     RCLONE_GOPRO_SHOW_ALL
- Type:        bool
- Default:     false

#### --gopro-show-empty-dirs

Show every media/by-year, by-month and by-day directory.

By default only years, months and days with media in them are listed.
A path under an unlisted day can still be used as a move destination
either way - this only changes what is listed.

Properties:

- Config:      show_empty_dirs
- Env Var:     RCLONE_GOPRO_SHOW_EMPTY_DIRS
- Type:        bool
- Default:     false

#### --gopro-start-year

Year to start media/by-year, by-month and by-day listings from.

0 (the default) uses the year of the earliest media in the library.
Set it together with [--gopro-show-empty-dirs](#gopro-show-empty-dirs)
to list earlier years.

Properties:

- Config:      start_year
- Env Var:     RCLONE_GOPRO_START_YEAR
- Type:        int
- Default:     0

#### --gopro-link-allow-download

Allow downloading the original file from a public share link.

This is the "Allow Download" toggle in GoPro's web app. GoPro ties it
to sharing any GPS data embedded in the file, so enabling it shares
that location data with recipients too.

Properties:

- Config:      link_allow_download
- Env Var:     RCLONE_GOPRO_LINK_ALLOW_DOWNLOAD
- Type:        bool
- Default:     false

#### --gopro-link-title

Title for public share links.

Defaults to the file's name without its {id} suffix. As "rclone link"
can't pass a title, this applies to every link created.

Properties:

- Config:      link_title
- Env Var:     RCLONE_GOPRO_LINK_TITLE
- Type:        string
- Required:    false

#### --gopro-use-trash

Send deleted files to GoPro's trash instead of deleting permanently.

Trashed media shows as "Recently Deleted" in GoPro's app, can be
restored for up to 60 days (see "rclone backend restore" and
[--gopro-trashed-only](#gopro-trashed-only)) and still counts against
the storage quota. Media from GoPro cameras doesn't count against any
quota, so there's nothing to gain by skipping the trash for it.

Properties:

- Config:      use_trash
- Env Var:     RCLONE_GOPRO_USE_TRASH
- Type:        bool
- Default:     true

#### --gopro-trashed-only

Only show media in GoPro's trash.

With this set, every listing under media/ shows "Recently Deleted"
instead of the active library, including items the other filters would
hide. Deleting a file here removes it permanently, regardless of
[--gopro-use-trash](#gopro-use-trash). Use "rclone backend restore" to
move it back to the library.

To view the trash next to the normal library, override this per
command, e.g. "gopro,trashed_only=true:media/all".

Properties:

- Config:      trashed_only
- Env Var:     RCLONE_GOPRO_TRASHED_ONLY
- Type:        bool
- Default:     false

#### --gopro-always-add-id

Always add the media ID to file names, as "name {id}.ext".

GoPro cameras reuse file names, so names that collide within a listing
always get the ID. Without this option whether a file collides can
change from one run to the next, renaming it - sync then deletes and
re-transfers it. Only turn this off for a library without duplicate
names.

Properties:

- Config:      always_add_id
- Env Var:     RCLONE_GOPRO_ALWAYS_ADD_ID
- Type:        bool
- Default:     true

#### --gopro-verify-size

Verify file sizes with a HEAD request before relying on them.

The size GoPro reports can be wrong, which fails rclone's integrity
check and makes sync re-transfer the file on every run. The only files
seen affected had been reprocessed by GoPro after upload, so by default
only those are checked.

Properties:

- Config:      verify_size
- Env Var:     RCLONE_GOPRO_VERIFY_SIZE
- Type:        string
- Default:     "reprocessed"
- Examples:
  - "reprocessed"
    - Verify only files GoPro has reprocessed since upload
  - "always"
    - Verify every file - safest, one extra request per file
  - "off"
    - Never verify - fastest, trusts file_size from the API as-is

#### --gopro-read-size

Read the exact size of chaptered videos, burst photos and edits.

GoPro only reports the total size of a chaptered video or burst photo
set, and none for Highlights and Edits, so their size is unknown by
default. Set this if you need exact sizes, e.g. for rclone mount. This
costs one extra request per file.

Properties:

- Config:      read_size
- Env Var:     RCLONE_GOPRO_READ_SIZE
- Type:        bool
- Default:     false

#### --gopro-upload-chunk-size

Chunk size for uploads to the upload/ directory.

Must be at least 5Mi: GoPro's upload endpoint is S3-backed and rejects
anything smaller for every part but the last.

Properties:

- Config:      upload_chunk_size
- Env Var:     RCLONE_GOPRO_UPLOAD_CHUNK_SIZE
- Type:        SizeSuffix
- Default:     6Mi

#### --gopro-upload-concurrency

Concurrency for multipart uploads.

GoPro's chunk upload protocol accepts parts in any order, so chunks of
a single file are PUT concurrently once read. Note that chunks are
buffered in memory, so total memory use can be up to
upload_chunk_size * upload_concurrency.

Properties:

- Config:      upload_concurrency
- Env Var:     RCLONE_GOPRO_UPLOAD_CONCURRENCY
- Type:        int
- Default:     4

#### --gopro-encoding

The encoding for the backend.

See the [encoding section in the overview](/overview/#encoding) for more info.

Properties:

- Config:      encoding
- Env Var:     RCLONE_GOPRO_ENCODING
- Type:        Encoding
- Default:     Slash,CrLf,InvalidUtf8,Dot

#### --gopro-description

Description of the remote.

Properties:

- Config:      description
- Env Var:     RCLONE_GOPRO_DESCRIPTION
- Type:        string
- Required:    false

## Backend commands

Here are the commands specific to the gopro backend.

Run them with:

```console
rclone backend COMMAND remote:
```

The help below will explain what arguments each command takes.

See the [backend](/commands/rclone_backend/) command for more
info on how to pass options and arguments.

These can be run on a running backend using the rc command
[backend/command](/rc/#backend-command).

### restore

Restore media from GoPro's trash

```console
rclone backend restore remote: [options] [<arguments>+]
```

This restores media from GoPro's trash to the active library,
whether or not --gopro-trashed-only is set.

With no arguments, it restores everything in the trash:

    rclone backend restore gopro:

Otherwise each argument names one medium to restore, either by its id
or by its "name {id}.ext" file name as listed with --gopro-trashed-only:

    rclone backend restore gopro: 6a99f18a239bf36f4c2377cf "photo {6a99f18a239bf36f4c2377cf}.jpg"

With --dry-run, it only logs what would be restored.

GoPro restores asynchronously and reports no failures, so a restored
item can take a while to reappear in the library, and occasionally
doesn't at all. Check with --gopro-trashed-only if one is missing.

<!-- autogenerated options stop -->

## Limitations

- No `ListR`: the directory tree above is several overlapping views of the
  same flat media list, so a full recursive listing wouldn't be any faster
  than rclone's default directory-by-directory walk.
- `Copy` and `DirMove` aren't implemented: GoPro Media Library has no
  server-side copy operation to build `Copy` on, and `DirMove` has nothing
  real to rename - every directory in the tree above is synthetic. `Move`
  is implemented; see "Renaming and moving files" below.
- Only `Video` and `Burst` media have been confirmed to use the two
  chapter/burst addressing schemes described above; other multi-item types
  (`TimeLapse`, `Continuous`, ...) may follow either one.
- Albums, moments, livestreams and sidecar files (GPMF/GPS telemetry, RAW
  `.GPR` companions from RAW+JPEG capture, an Edit's own Edit Decision
  List, etc.) aren't exposed by this backend - only the main rendition of
  each item is. Highlights and Edits themselves can be included with
  [`--gopro-include-edits`](#gopro-include-edits).
- This is an unofficial, reverse-engineered API. Use it with the
  expectation that GoPro could change or remove it at any time.
