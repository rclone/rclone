# Torrent Backend for rclone

This backend provides a read-only interface for accessing BitTorrent files through rclone. It creates a virtual filesystem from torrent files, allowing you to browse and access their contents without manual torrent management.

## Features

- **Read-only Access**: Safe, read-only access to torrent contents
- **Dynamic Monitoring**: Automatically detects new and removed torrent files
- **Multiple Torrent Support**: Any torrent file residing within the root directory will be exposed at the target directory, also nested files.
- **Efficient Streaming**: Smart piece prioritization and read-ahead for media files
- **Bandwidth Control**: Configurable upload and download speed limits
- **Auto Cleanup**: Optional automatic removal of inactive torrents

## Installation

The torrent backend is included in the main rclone distribution. No additional installation is required.

## Configuration

Create a new torrent remote:

```bash
rclone config

# Choose 'n' for a new remote
# Choose 'torrent' as the type
```

### Required Configuration

| Option | Description |
|--------|-------------|
| `root_directory` | Local directory containing your .torrent files |

### Optional Configuration

| Option | Description | Default |
|--------|-------------|---------|
| `max_download_speed` | Maximum download speed (kBytes/s) | 0 (unlimited) |
| `max_upload_speed` | Maximum upload speed (kBytes/s) | 0 (unlimited) |
| `piece_read_ahead` | Number of pieces to read ahead | 5 |
| `cleanup_timeout` | Remove inactive torrents after X minutes | 0 (disabled) |
| `cache_dir` | Directory for downloaded data | System temp |

## Usage Examples

### Basic Operations

```bash
# Mount torrent contents
rclone mount remote: /mount/point

# List available content
rclone ls remote:

# Copy a file from a torrent
rclone copy remote:path/to/file /local/path

# Stream a media file
rclone mount remote: /mount/point
vlc /mount/point/movie.mp4
```

### Directory Structure

The backend accepts a directory structure like:

```
/root_directory/
├── movies/
│   ├── action/
│   │   └── movie.torrent
│   └── drama/
│       └── series.torrent
└── music/
    └── album.torrent
```

This creates a virtual filesystem:

```
/
├── movies/
│   ├── action/
│   │   └── movie_contents/
│   └── drama/
│       └── series_contents/
└── music/
    └── album_contents/
```

### Advanced Features

1. **Bandwidth Control**:
   ```bash
   rclone config set remote max_download_speed 1024
   rclone config set remote max_upload_speed 512
   ```

2. **Auto Cleanup**:
   ```bash
   # Remove torrents inactive for 30 minutes
   rclone config set remote cleanup_timeout 30
   ```

3. **Custom Cache Location**:
   ```bash
   rclone config set remote cache_dir /path/to/cache
   ```

## Implementation Details

- Uses anacrolix/torrent for BitTorrent functionality
- Lazy loading: Torrents are only loaded when accessed
- Smart piece prioritization for efficient streaming
- Memory-efficient design for handling multiple torrents
- Automatic piece management and read-ahead

## Limitations

- Read-only access (no write operations)
- Only supports .torrent files (no magnet links)
- Requires local storage for downloaded data
- Initial metadata download required before access

## Performance Tips

1. **Media Streaming**:
   - Increase `piece_read_ahead` for smoother playback
   - Use a local cache directory for better performance

2. **Multiple Access**:
   - The backend handles concurrent reads efficiently
   - Consider bandwidth limits with multiple users

3. **Resource Usage**:
   - Use cleanup_timeout to manage disk usage
   - Monitor cache directory size

## Contributing

Contributions are welcome! Please ensure:

1. Tests pass (`go test ./...`)
2. Code follows Go standards
3. Documentation is updated
4. New features include tests

## License

Same as rclone's main license.