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
rclone <-> gphotos_mobile backend <-> Google Photos Mobile API (protobuf)
                |
                +-> SQLite cache (~/.gpmc/<email>/storage.db)
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
- **Cache location**: `~/.gpmc/<email>/storage.db` (configurable)

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
rclone config create mygphotos gphotos_mobile auth_data="YOUR_AUTH_DATA_STRING"
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
   \ (gphotos_mobile)
[snip]
Storage> gphotos_mobile

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
- type: gphotos_mobile
- auth_data: XXXX
Keep this "mygphotos" remote?
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d> y
```

### Environment variable setup (no config file)

```console
export RCLONE_GPHOTOS_MOBILE_AUTH_DATA="your_auth_data_string"
export RCLONE_CONFIG=""
rclone ls :gphotos_mobile:media/
```

On Windows (PowerShell):

```powershell
$env:RCLONE_GPHOTOS_MOBILE_AUTH_DATA = "your_auth_data_string"
$env:RCLONE_CONFIG = ""
.\rclone.exe ls :gphotos_mobile:media/
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

<!-- autogenerated options start - DO NOT EDIT - instead edit fs.RegInfo in backend/gphotosmobile/gphotosmobile.go and run make backenddocs to verify --> <!-- markdownlint-disable-line line-length -->
<!-- autogenerated options stop -->

## Mount

When using `rclone mount`, you **must** use `--vfs-cache-mode full`.

Google Photos download URLs do not support HTTP Range requests (partial
reads). The backend handles this by downloading each file once to a local
temp file and sharing it across all readers. This means:

- **First access** to a file downloads it fully (buffered in background)
- **Seeking** within an already-opened file is instant
- **Multiple opens** of the same file share one download
- Temp files are cleaned up 30 seconds after the last reader closes

For video playback, the file needs to buffer enough data before playback
starts. Larger files take longer to start but once buffered, seeking is
instant.

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

Google Photos download URLs do not support HTTP Range headers. The backend
works around this by downloading entire files to local temp files and
serving reads from there. This means:

- First access to a large file may be slow (must download fully)
- Additional disk space is used for temp files during downloads
- Streaming video from mount requires buffering time

### No album support (yet)

The backend currently only exposes the flat `media/` directory. Album
listing and creation are not yet implemented.

### Duplicate filenames

If multiple items share the same filename, duplicates get a suffix with
the first 8 characters of the media key. For example, two files named
`photo.jpg` become `photo.jpg` and `photo_AF1QipO2.jpg`.

The deduplication only applies within a single directory listing.
`NewObject()` lookups by filename return the first match.

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

| Feature | gphotos (REST API) | gphotos_mobile (Mobile API) |
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
and re-syncing: `rm ~/.gpmc/<email>/storage.db`
