---
title: "YouTube"
description: "Rclone docs for YouTube (ytfs)"
versionIntroduced: "v1.71"
---

# YouTube

The rclone YouTube backend (`ytfs:`) provides read-only access to YouTube channels,
playlists, and videos as a virtual filesystem.

**NB** This backend is **read-only**. Uploading, deleting, or modifying videos
is not supported. See the [Limitations](#limitations) section.

**NB** This backend requires `yt-dlp` to be installed on your system. Install it via:
```
pip install yt-dlp
```

## Configuration

The YouTube backend supports two configuration modes:

### Single URL Mode (Default)

The simplest setup requires no additional configuration. Rclone automatically discovers subscribed channels and playlists:

```
No remotes found, make a new one?
n) New remote
n/s/q> n

name> youtube

Type of storage to configure.
Storage> ytfs
```

### Manifest File Mode

For custom channel and playlist organization, use a JSON manifest file:

```
name> youtube_custom

Type of storage to configure.
Storage> ytfs

manifest_file> /path/to/manifest.json
```

### Options

- **manifest_file** (string): Path to a JSON manifest file defining channels and playlists. When set, this takes precedence over automatic discovery. Optional.

### Manifest JSON Format

Create a `manifest.json` file with the following structure:

```json
{
  "channels": [
    {
      "id": "UCddiUEpMJcEKBB68VHQnAow",
      "name": "My Channel"
    }
  ],
  "playlists": [
    {
      "id": "PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf",
      "name": "My Playlist"
    }
  ]
}
```

### Example rclone config with manifest

```ini
[youtube]
type = ytfs
manifest_file = /home/user/.config/rclone/yt_manifest.json
```

### Usage Examples

List channels from manifest:
```
rclone lsd youtube_custom:channels/
```

List videos in a channel:
```
rclone lsd "youtube_custom:channels/My Channel/"
```

List playlists:
```
rclone lsd youtube_custom:playlists/
```

Stream a video:
```
rclone cat "youtube_custom:channels/My Channel/Video Title — {VideoID}"
```

## Filesystem layout

```
ytfs:
  ├── channels/
  │   ├── {ChannelName1}/
  │   │   ├── videos/
  │   │   │   ├── {VideoTitle} — {VideoID}
  │   │   │   └── ...
  │   │   └── ...
  │   └── {ChannelName2}/
  │       └── ...
  └── playlists/
      ├── {PlaylistName1}/
      │   ├── {VideoTitle} — {VideoID}
      │   └── ...
      └── {PlaylistName2}/
          └── ...
```

The YouTube backend organizes content hierarchically:
- **channels/**: List of subscribed YouTube channels
- **playlists/**: List of user playlists
- **videos/**: Video files within each channel (accessible as directory entries)

Each video is accessed as a virtual file via `yt-dlp` and streamed on-demand.

## Reference

### Manifest JSON Schema

The manifest file uses a JSON structure to define custom channels and playlists. Both sections are optional.

**Root properties:**
- **channels** (array, optional): List of YouTube channels to expose
- **playlists** (array, optional): List of YouTube playlists to expose

**Channel object:**
- **id** (string, required): YouTube channel ID (e.g., `UCddiUEpMJcEKBB68VHQnAow`)
- **name** (string, required): Display name for the channel in the filesystem

**Playlist object:**
- **id** (string, required): YouTube playlist ID (e.g., `PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf`)
- **name** (string, required): Display name for the playlist in the filesystem

**Configuration note:** If both `url` and `manifest_file` are configured, `manifest_file` takes precedence. The manifest approach is recommended for reproducible configurations or when you need fine-grained control over exposed content.

## Limitations

### MVP Scope (Current)
- **Channels only**: Access to subscribed channels only; individual channel URLs not yet supported
- **Playlists only**: Organized collections of videos; search functionality not yet supported
- **No metadata**: Per-video metadata files (title, description, duration) not yet available
- **No thumbnails**: Video thumbnails not yet available
- **Streaming only**: Videos are streamed via `yt-dlp` on-demand; not downloaded or cached locally
- **Read-only**: No upload, delete, or modify support
- **No hash support**: Checksums are not available
- **Requires yt-dlp**: The `yt-dlp` tool must be installed

### Future Features (Planned)
- OAuth authentication for private videos and channels
- Per-video metadata files
- Video thumbnails
- Search functionality
- Individual channel/playlist URL support
