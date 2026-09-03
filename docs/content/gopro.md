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

### No paid subscription required

This backend works against a free GoPro account with no active GoPro
subscription and no storage entitlement of its own - checked against one
directly: authentication, listing, downloading, uploading and deleting all
worked identically to a paying account. The OAuth grant does come back with
a narrower `scope` on such an account (missing `media_library_beta` among
others), but nothing this backend calls appears to be gated behind it.

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
their own camera-original recording, and aren't included by default -
see [`--gopro-include-edits`](#gopro-include-edits) to opt in.

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

`Copy` and `DirMove` remain unimplemented - see "Limitations" below.

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

"source" (the default) downloads the original camera file. Any other
value is matched against the label or quality of the renditions GoPro
offers for that media item (for example "1080p", or a proxy label such
as "high_res_proxy_mp4"); if no match is found the backend falls back
to the first file offered.

Properties:

- Config:      download_variation
- Env Var:     RCLONE_GOPRO_DOWNLOAD_VARIATION
- Type:        string
- Default:     "source"

#### --gopro-include-edits

Include Highlights and user-made Edits in listings.

Off by default: these "MultiClipEdit"/"Edit" media are composed from
other clips rather than being their own camera-original recording, so
they're often a redundant rendering of content the library already
has natively, and they behave differently enough to be worth opting
into deliberately rather than getting by surprise:

- file_size is always null for these - this backend reports their
  size as unknown (like a chaptered video or burst photo set) rather
  than skipping them, so [--gopro-read-size](#gopro-read-size) is
  needed for an exact size (e.g. for rclone mount).
- Their own file_extension is that of an internal Edit Decision List
  (typically "json"), not of what's actually downloaded - GoPro
  serves the rendered video (the "baked_source" rendition) for these,
  not the EDL, and this backend's Content-Type follows the filename's
  own extension (usually ".mp4") to match what's actually served.

Properties:

- Config:      include_edits
- Env Var:     RCLONE_GOPRO_INCLUDE_EDITS
- Type:        bool
- Default:     false

#### --gopro-always-add-id

Always name files "name {id}.ext" instead of just "name.ext".

Without this, a file only gets its GoPro media ID appended to its name
when it collides with another file of the same name in the same
listing - but GoPro cameras recycle filenames constantly, so whether a
given file collides depends on what else happens to exist at listing
time, which can change from one run to the next with no change to the
file itself. That makes a plain name unstable: a file uploaded earlier
under "GX010123.MP4" can silently become "GX010123 {id}.MP4" the
moment a second "GX010123.MP4" turns up elsewhere in the library, and
rclone has no way to know the two names refer to the same file -
"sync" would delete and re-transfer it under the new name, and "copy"
would leave a stale duplicate behind under the old one, forever.

On by default because that failure mode is silent and only shows up
intermittently, on whichever run happens to introduce a colliding
name - a normal-looking successful sync/copy today doesn't mean this
won't bite on some future run. Turn it off only if you want clean
names and are confident this library won't hit a same-name collision
(a small, static library of hand-picked clips, say), or don't mind the
occasional renamed-and-re-transferred file.

Properties:

- Config:      always_add_id
- Env Var:     RCLONE_GOPRO_ALWAYS_ADD_ID
- Type:        bool
- Default:     true

#### --gopro-verify-size

Verify a file's size with a live request before relying on it.

file_size from the search API can be wrong - confirmed live, a few KB
larger than the size actually served, specifically for a video whose
"source" rendition had been moved to colder S3 storage. A wrong size
here does more than look odd: it fails rclone's post-transfer
integrity check (discarding an otherwise-complete download and
restarting it), can corrupt rclone's multi-thread downloader (which
divides a file into ranged chunks using this size before the transfer
starts), and makes "sync" re-download an already-correct file on every
single run, since the sizes would never match.

GoPro's API exposes no storage class or anything else that reliably
predicts which files are affected - checked live, colder storage alone
isn't enough, most files there still report their size correctly. The
one thing a live account probe did find in common with the one
affected file, out of hundreds checked, is that GoPro had reprocessed
it after upload (a non-null reprocessed_at). That's a single data
point, not a proven rule, but it's free to check - the same listing
already fetches it - so it's the default: cheaper than checking every
file, safer than checking none.

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

Read the exact size of a chaptered video or burst photo set item.

file_size from the search API is only the total across every item of a
chaptered video or burst photo set, not any single one, so an
individual item's size is normally left unknown (-1) rather than
guessed at by dividing it evenly. Set this if you need an exact size
for one of these, for example for rclone mount. This does one extra
request per file, on top of whatever --gopro-verify-size already costs
for that file - so listing a large library with lots of chaptered/burst
items will be slower still.

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
