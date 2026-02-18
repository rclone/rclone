---
title: "Google Photos Mobile"
description: "Rclone docs for Google Photos Mobile API backend"
versionIntroduced: "v1.74"
---

# {{< icon "fa fa-images" >}} Google Photos Mobile

The rclone backend for [Google Photos](https://www.google.com/photos/about/)
using the **reverse-engineered mobile API** provides full, unrestricted access
to your Google Photos library -- including downloading original quality photos
and videos, uploading, and trashing items.

This is an alternative to the official [Google Photos](/googlephotos/) backend
which uses the REST API and has severe limitations (can only download files it
uploaded, compressed video downloads, stripped EXIF data, etc.).

**This backend has no such limitations.** It uses the same API that the
Android Google Photos app uses, giving you:

- Download **any** photo or video at **original quality**
- Full EXIF metadata preserved
- Upload files at original quality
- Move items to trash
- Full library listing with metadata (location, camera info, timestamps, etc.)
- Mount as a local drive with FUSE

## How it works

This backend does **not** use the official Google Photos REST API. Instead, it
uses the internal mobile API that the Google Photos Android app communicates
with. This requires Android device credentials obtained from the Google
Photos app (see [Prerequisites](#prerequisites)).

### Architecture

```text
rclone <-> gphotosmobile backend <-> Google Photos Mobile API (protobuf)
                |
                +-> SQLite cache (<cache-dir>/gphotosmobile/<remote>.db)
```

1. **Authentication**: Uses Android device tokens (obtained via `gms_auth`)
   to get OAuth bearer tokens from `android.googleapis.com/auth`
2. **Library sync**: On first run, downloads the full media library index
   via a protobuf streaming API. This is cached in a local SQLite database.
   Subsequent runs do fast incremental delta syncs.
3. **File listing**: Served from the local SQLite cache. No API calls needed.
4. **Downloads**: Gets a time-limited download URL from the API, then
   streams the file. Each file is downloaded once to a local temp file and
   shared across multiple readers (for mount/seek support).
5. **Uploads**: Calculates SHA1 hash, checks for duplicates on the server,
   then uploads via a resumable upload flow (get token -> upload bytes -> commit).
6. **Protobuf encoding**: Uses raw protobuf wire format encoding/decoding
   (no compiled .proto files) to match the mobile app's request format.

### Cache behavior

The library index is cached locally in SQLite for performance:

- **First run**: Full sync downloads all items (~2 minutes for 30K items)
- **Subsequent runs**: Incremental delta sync (~1 second)
- **Sync throttle**: At most one sync every 30 seconds (persisted across
  rclone invocations)
- **After uploads/deletes**: Immediate sync to reflect changes
- **Cache location**: `<cache-dir>/gphotosmobile/<remote-name>.db`
  (where `<cache-dir>` is rclone's `--cache-dir`, typically
  `~/.cache/rclone` on Linux, `%LocalAppData%\rclone` on Windows)
- **Cache is regenerable**: If the database is deleted, rclone will
  re-download the full library index on the next run. No data is lost.

To force a full re-sync, delete the cache database file.

## Prerequisites

You need Android device credentials to use this backend. These are obtained
from the Google Photos Android app using one of the methods below. You only
need to do this once -- the credentials do not expire under normal use.

### Getting auth_data via ReVanced (no root required)

This is the recommended method. It uses a patched Google Photos app
(ReVanced) and captures the authentication string via ADB logcat.

1. Install **GmsCore** (microG) on your Android device or emulator:
   [https://github.com/ReVanced/GmsCore/releases](https://github.com/ReVanced/GmsCore/releases)

2. Install **Google Photos ReVanced** (patched APK):
   [https://github.com/j-hc/revanced-magisk-module/releases](https://github.com/j-hc/revanced-magisk-module/releases)
   (or patch it yourself with ReVanced Manager)

3. Connect the device to your PC via **ADB**.

4. Open a terminal on your PC and run:

   **Windows:**

   ```cmd
   adb logcat | FINDSTR "auth%2Fphotos.native"
   ```

   **Linux/macOS:**

   ```shell
   adb logcat | grep "auth%2Fphotos.native"
   ```

5. If you are already using ReVanced, remove your Google Account from
   GmsCore first, then re-add it.

6. Open **Google Photos ReVanced** on the device and log into your account.

7. One or more identical log lines should appear in the terminal.

8. Copy the text starting from `androidId=` to the end of the line.
   This is your `auth_data` string.

**Note:** The `auth_data` credentials do not expire under normal use, but
Google may revoke them if it detects unusual activity. Each credential is
tied to a specific device ID.

## Configuration

### Quick setup (one command)

```console
rclone config create mygphotos gphotosmobile auth_data="YOUR_AUTH_DATA_STRING"
```

### Interactive setup

```console
rclone config
```

```text
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n

Enter name for new remote.
name> mygphotos

Option Storage.
Type of storage to configure.
Choose a number from below, or type in your own value.
[snip]
XX / Google Photos (Mobile API - full access)
   \ (gphotosmobile)
[snip]
Storage> gphotosmobile

Option auth_data.
Google Photos mobile API auth data.
Enter a value.
auth_data> androidId=XXXX&app=com.google.android.apps.photos&...

Edit advanced config?
y) Yes
n) No (default)
y/n> n

Configuration complete.
Options:
- type: gphotosmobile
- auth_data: XXXX
Keep this "mygphotos" remote?
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d> y
```

### Environment variable setup (no config file)

```console
export RCLONE_GPHOTOSMOBILE_AUTH_DATA="your_auth_data_string"
export RCLONE_CONFIG=""
rclone ls :gphotosmobile:media/
```

On Windows (PowerShell):

```powershell
$env:RCLONE_GPHOTOSMOBILE_AUTH_DATA = "your_auth_data_string"
$env:RCLONE_CONFIG = ""
.\rclone.exe ls :gphotosmobile:media/
```

## Usage

### Layout

The backend presents a virtual filesystem:

```text
/
  media/
    file1.jpg
    file2.mp4
    ...
```

All photos and videos appear as a flat list under `media/`. Files are
named using their original filename from Google Photos.

### List all files

```console
rclone ls mygphotos:media/
```

### List directories

```console
rclone lsd mygphotos:
```

### Copy a file from Google Photos

```console
rclone copy mygphotos:media/photo.jpg /local/path/
```

### Copy a file to Google Photos

```console
rclone copy /local/path/photo.jpg mygphotos:media/
```

### Sync a local directory to Google Photos

```console
rclone sync /local/photos/ mygphotos:media/ --progress
```

### Mount as a drive

On Windows (requires [WinFsp](https://winfsp.dev/) and build with
`-tags cmount`):

```console
rclone mount mygphotos: G: --vfs-cache-mode full
```

On Linux/macOS:

```console
rclone mount mygphotos: /mnt/gphotos --vfs-cache-mode full
```

**Important**: `--vfs-cache-mode full` is required for mount. See the
[Mount](#mount) section for details.

### Delete a file (move to trash)

```console
rclone delete mygphotos:media/unwanted.jpg
```

This moves the file to Google Photos trash. It does **not** permanently
delete it.

### Metadata

The backend supports reading rich metadata from Google Photos items and
writing the description (caption).

#### Read metadata

```console
rclone lsjson mygphotos:media/photo.jpg --metadata
```

This returns all available metadata fields: description, media type,
dimensions, location (GPS coordinates and name), camera info (make,
model, aperture, shutter speed, ISO, focal length), timestamps, and
flags (favorite, archived, origin).

#### Set description

```console
rclone copyto mygphotos:media/photo.jpg mygphotos:media/photo.jpg --metadata-set description="Sunset at the beach"
```

Or clear it:

```console
rclone copyto mygphotos:media/photo.jpg mygphotos:media/photo.jpg --metadata-set description=""
```

The description is the caption text shown under the media item in the
Google Photos app and web UI.

<!-- autogenerated options start - DO NOT EDIT - instead edit fs.RegInfo in backend/gphotosmobile/gphotosmobile.go and run make backenddocs to verify --> <!-- markdownlint-disable-line line-length -->
### Standard options

Here are the Standard options specific to gphotosmobile (Google Photos (Mobile API - full access)).

#### --gphotosmobile-auth-data

Google Photos mobile API auth data.

This is the auth string containing your Android device credentials
for Google Photos. It starts with 'androidId='.

See the documentation for instructions on how to obtain it
using Google Photos ReVanced and ADB logcat.

Properties:

- Config:      auth_data
- Env Var:     RCLONE_GPHOTOSMOBILE_AUTH_DATA
- Type:        string
- Required:    true

### Advanced options

Here are the Advanced options specific to gphotosmobile (Google Photos (Mobile API - full access)).

#### --gphotosmobile-cache-db-path

Path to SQLite cache database.

The media library index is cached locally in SQLite for fast listing.
The default location is inside rclone's cache directory
(see --cache-dir) at gphotosmobile/<remote-name>.db.

The initial sync downloads the full library index and may take a few
minutes for large libraries. Subsequent runs use fast incremental sync.
If this file is deleted, rclone will re-sync the full library on next run.

Properties:

- Config:      cache_db_path
- Env Var:     RCLONE_GPHOTOSMOBILE_CACHE_DB_PATH
- Type:        string
- Required:    false

#### --gphotosmobile-device-model

Device model to report to Google Photos.

This is included in the user agent and upload metadata.
Leave empty for default (Pixel 9a).

Properties:

- Config:      device_model
- Env Var:     RCLONE_GPHOTOSMOBILE_DEVICE_MODEL
- Type:        string
- Required:    false

#### --gphotosmobile-device-make

Device manufacturer to report to Google Photos.

This is included in upload metadata.
Leave empty for default (Google).

Properties:

- Config:      device_make
- Env Var:     RCLONE_GPHOTOSMOBILE_DEVICE_MAKE
- Type:        string
- Required:    false

#### --gphotosmobile-apk-version

Google Photos APK version code to impersonate.

This is sent in the user agent and protobuf requests.
Update if Google blocks older versions.

Properties:

- Config:      apk_version
- Env Var:     RCLONE_GPHOTOSMOBILE_APK_VERSION
- Type:        int64
- Default:     49029607

#### --gphotosmobile-android-api

Android API level to report.

35 corresponds to Android 15, which ships with the default
device model (Pixel 9a). Included in upload metadata.

Properties:

- Config:      android_api
- Env Var:     RCLONE_GPHOTOSMOBILE_ANDROID_API
- Type:        int64
- Default:     35

#### --gphotosmobile-download-cache

Enable download cache for opened files.

Google Photos download URLs do not support HTTP Range requests.
When enabled, each opened file is downloaded once to a local temp
file and shared across concurrent readers, with instant seeking.
This is useful for rclone mount where the VFS layer needs to seek.

When disabled (the default), Open() returns the raw HTTP stream
directly with no temp files. Use --vfs-cache-mode full for mount.

Properties:

- Config:      download_cache
- Env Var:     RCLONE_GPHOTOSMOBILE_DOWNLOAD_CACHE
- Type:        bool
- Default:     false

#### --gphotosmobile-encoding

The encoding for the backend.

See the [encoding section in the overview](/overview/#encoding) for more info.

Properties:

- Config:      encoding
- Env Var:     RCLONE_GPHOTOSMOBILE_ENCODING
- Type:        Encoding
- Default:     Slash,CrLf,InvalidUtf8,Dot

#### --gphotosmobile-description

Description of the remote.

Properties:

- Config:      description
- Env Var:     RCLONE_GPHOTOSMOBILE_DESCRIPTION
- Type:        string
- Required:    false

### Metadata

Metadata is returned for all media items. The "description" (caption) can be written; all other fields are read-only.

Here are the possible system metadata items for the gphotosmobile backend.

| Name | Help | Type | Example | Read Only |
|------|------|------|---------|-----------|
| aperture | Aperture f-number from EXIF. | float | 1.8 | **Y** |
| camera-make | Camera manufacturer from EXIF. | string | Google | **Y** |
| camera-model | Camera model from EXIF. | string | Pixel 9a | **Y** |
| description | Caption/description of the media item in Google Photos. | string | Sunset at the beach | N |
| duration | Duration in milliseconds (videos only). | int | 15000 | **Y** |
| focal-length | Focal length in mm from EXIF. | float | 4.38 | **Y** |
| height | Height in pixels. | int | 3024 | **Y** |
| is-archived | Whether the item is archived. | bool | false | **Y** |
| is-favorite | Whether the item is marked as favorite. | bool | true | **Y** |
| iso | ISO sensitivity from EXIF. | int | 100 | **Y** |
| latitude | GPS latitude of the media item. | float | 37.7749 | **Y** |
| location-name | Reverse-geocoded location name. | string | San Francisco, CA | **Y** |
| longitude | GPS longitude of the media item. | float | -122.4194 | **Y** |
| media-type | Media type: photo or video. | string | photo | **Y** |
| origin | Origin of the item: self, partner, or shared. | string | self | **Y** |
| shutter-speed | Shutter speed in seconds from EXIF. | float | 0.001 | **Y** |
| taken-at | Time the photo/video was taken (UTC). | RFC 3339 | 2006-01-02T15:04:05Z | **Y** |
| uploaded-at | Time the item was uploaded to Google Photos (UTC). | RFC 3339 | 2006-01-02T15:04:05Z | **Y** |
| width | Width in pixels. | int | 4032 | **Y** |

See the [metadata](/docs/#metadata) docs for more info.

## Backend commands

Here are the commands specific to the gphotosmobile backend.

Run them with:

```console
rclone backend COMMAND remote:
```

The help below will explain what arguments each command takes.

See the [backend](/commands/rclone_backend/) command for more
info on how to pass options and arguments.

These can be run on a running backend using the rc command
[backend/command](/rc/#backend-command).

### set-description

Set the description (caption) of a media item.

```console
rclone backend set-description remote: [options] [<arguments>+]
```

Set the description (caption) of a media item in Google Photos.
The description is the text shown under the item in the Google Photos app.

Usage:

    rclone backend set-description remote:media/photo.jpg -o description="Sunset at the beach"

To clear the description:

    rclone backend set-description remote:media/photo.jpg -o description=""


Options:

- "description": The description text to set (empty to clear)

<!-- autogenerated options stop -->

## Download cache

Google Photos download URLs do not support HTTP Range requests (partial
reads). By default the backend streams downloads directly with no temp
files, which works well for `rclone copy` and `rclone sync`.

For `rclone mount` or other use cases that need seeking, you can enable
the optional **download cache**:

```console
rclone mount mygphotos: /mnt/gphotos \
    --vfs-cache-mode full \
    --gphotosmobile-download-cache=true
```

or in your rclone config:

```ini
[mygphotos]
type = gphotosmobile
auth_data = ...
download_cache = true
```

When enabled, each opened file is downloaded once to a local temp file
in the background and shared across concurrent readers. This gives you:

- **Seeking** within an already-opened file is instant (data is on disk)
- **Multiple opens** of the same file share one download
- Temp files are cleaned up 30 seconds after the last reader closes

## Mount

When using `rclone mount`, you **must** use `--vfs-cache-mode full`.
You may also want to enable `download_cache` for better seeking
performance, though the VFS cache layer can handle it on its own.

Recommended mount flags:

```console
rclone mount mygphotos: /mnt/gphotos \
    --vfs-cache-mode full \
    --vfs-read-chunk-size 64M \
    --buffer-size 64M \
    -v
```

## Limitations

### No Range request support

Google Photos download URLs do not support HTTP Range headers. The
backend streams each download from byte 0. For `rclone copy`/`rclone sync`
this is fine since files are read sequentially.

For `rclone mount`, use `--vfs-cache-mode full` so the VFS layer caches
files locally. You can optionally enable `download_cache = true` for
additional seeking support at the backend level. See the
[Download cache](#download-cache) section.

### Uploads are media files only

Google Photos only accepts image and video files. Uploading non-media
files (e.g. `.txt`, `.pdf`, `.zip`) will fail at the commit stage.
Supported formats include JPEG, PNG, GIF, WebP, HEIC, MP4, MOV, and
other common photo/video formats.

### No album support (yet)

The backend currently only exposes the flat `media/` directory. Album
listing and creation are not yet implemented.

### Duplicate filenames

If multiple items share the same filename, **all** of them get a suffix
with their dedup key (a URL-safe base64-encoded SHA1 hash of the file
content). For example, two files both named `photo.jpg` become
`photo_2VRSgGLVhhnUtc6WW3tL1kYSrKE.jpg` and
`photo_f3TGtgToY86xuq3_IqUp10ZtBnU.jpg`. This ensures filenames are
stable and deterministic regardless of database ordering between syncs.

Files with unique names are not affected.

### Deletion is trash only

The `Remove()` operation moves items to Google Photos trash. The Google
Photos mobile API does not support permanent deletion. Items in trash are
automatically deleted by Google after 60 days.

Some items may not have a `dedup_key` in the cache, which is required for
the trash operation. These items cannot be deleted via rclone.

### Modification times

The modification time shown is the media creation timestamp from Google
Photos (typically EXIF capture time or upload time). This is **read-only**
and cannot be changed via rclone.

### File sizes

File sizes come from the library sync metadata. In rare cases, some items
may have a size of 0 in the API response. These will show as 0 bytes in
listings but download correctly.

### Authentication

The `auth_data` uses Android device credentials (a master token). This
has some implications:

- The token does **not** expire normally
- Google may revoke it if it detects unusual activity
- Each token is tied to a specific Android device ID
- The token grants **full access** to the Google account's Photos library

### Upload deduplication

Google Photos deduplicates by file content (SHA1 hash). If you upload a
file that already exists in the library, the backend detects this and
skips the upload. The existing media key is reused.

### Cache sync delay

The local SQLite cache syncs with the server at most every 30 seconds.
Changes made outside of rclone (e.g. via the Google Photos app) may take
up to 30 seconds to appear. Uploads and deletes made through rclone
trigger an immediate sync.

## Comparison with Google Photos (REST API) backend

| Feature | gphotos (REST API) | gphotosmobile (Mobile API) |
|---------|--------------------|-----------------------------|
| Download any photo | Only rclone-uploaded (since March 2025) | All photos |
| Download quality | Compressed | Original |
| Video quality | Heavily compressed | Original |
| EXIF data | Stripped (no location) | Preserved |
| Upload | Yes | Yes |
| Delete | Only from rclone-created albums | Any item (trash) |
| Albums | Yes | Not yet |
| Auth method | OAuth (browser) | Android device token |
| Requires root Android | No | No (ReVanced method) |
| Official API | Yes | No (reverse-engineered) |
| Rate limits | Google API quotas | No known limits |
| Mount support | Limited (no sizes by default) | Full (with vfs-cache-mode full) |

## Troubleshooting

### "auth token refresh failed"

Your `auth_data` may be invalid or revoked. Re-do the
[credential extraction](#getting-auth_data-via-revanced-no-root-required)
to get a new token.

### First run is very slow

The initial library sync downloads the index for all items. For a library
with 30,000+ items this takes about 2 minutes. Subsequent runs use
incremental sync and are fast (~1 second).

### Files not showing up after upload

The cache syncs at most every 30 seconds. Wait and run the listing again,
or the upload should have triggered an immediate sync.

### Mount: video playback stutters

The backend must download the entire file before seeking works. Use
`--vfs-cache-mode full` and wait for the file to buffer. You can monitor
progress in the rclone log output with `-v`.

### "no dedup key available for trash operation"

The item doesn't have a dedup_key in the cache. This can happen for items
that were synced with incomplete metadata. Try deleting the cache database
and re-syncing. Delete the cache database file at
`<cache-dir>/gphotosmobile/<remote-name>.db` (see `rclone help flags`
for `--cache-dir` location).
