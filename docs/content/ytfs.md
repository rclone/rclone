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

The YouTube backend can be configured with minimal setup:

```
No remotes found, make a new one?
n) New remote
n/s/q> n

name> youtube

Type of storage to configure.
Storage> ytfs
```

### Options

Currently the YouTube backend requires no additional configuration. It automatically discovers and lists subscribed channels and playlists for the authenticated user.

**Future options (not yet implemented):**
- OAuth authentication for accessing private videos and subscriptions

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
