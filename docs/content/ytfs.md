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

## Emby Server Integration

ytfs can integrate with Emby media servers by mounting the YouTube filesystem on a path accessible to Emby, enabling automatic discovery and playback of YouTube videos.

### Setup Example

1. Create a mount point on your server:
```bash
mkdir -p /mnt/youtube
```

2. Configure ytfs with a manifest file listing your channels and playlists:
```ini
[youtube]
type = ytfs
manifest_file = /home/user/.config/rclone/yt_manifest.json
auto_reload = true
reload_debounce_ms = 500
```

3. Mount ytfs using rclone mount:
```bash
rclone mount youtube: /mnt/youtube --allow-other --vfs-cache-mode writes
```

4. Add `/mnt/youtube` as a library in Emby:
   - Go to Settings > Libraries > Add Library
   - Select Movies or TV Shows
   - Point to `/mnt/youtube/channels` or `/mnt/youtube/playlists`
   - Enable "Refresh library on startup"

### Metadata & Auto-Discovery

ytfs provides automatic metadata generation for Emby:

**Available metadata files per video:**
- `{VideoID}.nfo` - KODI-compatible metadata (title, description, duration)
- `{VideoID}-thumb.jpg` - Video thumbnail (auto-fetched from YouTube)
- `{VideoID}.srt` - Subtitle file (auto-downloaded via yt-dlp)
- `{VideoID}.chapters.xml` - Chapter markers (auto-generated based on video duration)

**Metadata Caching:**
- Cache TTL: 24 hours
- Concurrent requests to the same metadata are blocked until the first fetch completes
- Expired cache entries are automatically discarded
- Cache stores up to several hundred entries without size limits (suitable for personal libraries)

**File Watching & Hot Reload:**
- Manifest file changes are automatically detected and reloaded
- Debounce window (default 500ms) prevents reload storms when modifying the manifest
- Disable auto-reload by setting `auto_reload = false` if you manage reloads manually

### Example Manifest for Emby

```json
{
  "channels": [
    {
      "id": "UCddiUEpMJcEKBB68VHQnAow",
      "name": "Educational Content"
    },
    {
      "id": "UCxyz",
      "name": "Tech Reviews"
    }
  ],
  "playlists": [
    {
      "id": "PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf",
      "name": "Favorites"
    }
  ]
}
```

### Kids Content Filtering

To restrict Emby to kid-friendly channels only:

1. Create a separate manifest with only kid-approved channels
2. Mount with a dedicated ytfs instance:
```bash
rclone mount youtube_kids: /mnt/youtube_kids
```

3. Add this library separately in Emby with parental controls

### Performance Considerations

- **First metadata fetch**: ~1-2 seconds per video (downloads from YouTube)
- **Subsequent metadata requests**: ~10-50ms (from cache)
- **Library scanning**: Automatic on first access; cached entries persist across restarts
- **Concurrent users**: Metadata requests are serialized per video ID to avoid redundant downloads

### Troubleshooting

**Missing metadata files:**
- Ensure `yt-dlp` is installed: `pip install yt-dlp`
- Check that YouTube hasn't restricted the video
- Verify network connectivity for thumbnail downloads

**Slow first library scan:**
- Normal - metadata is being fetched for all videos
- Subsequent scans will be much faster (using cache)
- Configure Emby to scan in the background

## Limitations

### MVP Scope (Current)
- **Channels only**: Access to subscribed channels only; individual channel URLs not yet supported
- **Playlists only**: Organized collections of videos; search functionality not yet supported
- **Streaming only**: Videos are streamed via `yt-dlp` on-demand; not downloaded or cached locally
- **Read-only**: No upload, delete, or modify support
- **No hash support**: Checksums are not available
- **Requires yt-dlp**: The `yt-dlp` tool must be installed for metadata and subtitle extraction

### Future Features (Planned)
- OAuth authentication for private videos and channels
- Search functionality
- Individual channel/playlist URL support
- Enhanced subtitle format support (VTT, ASS/SSA)
- Video description indexing for Emby search
