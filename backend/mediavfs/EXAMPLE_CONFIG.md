# MediaVFS Configuration Examples

## Basic Configuration

Create a basic mediavfs remote:

```bash
rclone config create mymedia mediavfs \
    db_connection "postgres://user:password@localhost/mediadb?sslmode=disable" \
    download_url "http://localhost:8080/media/download"
```

## Advanced Configuration

### With SSL/TLS for PostgreSQL

```bash
rclone config create securemedia mediavfs \
    db_connection "postgres://user:password@localhost/mediadb?sslmode=require" \
    download_url "http://localhost:8080/media/download"
```

### With Custom Table Name

```bash
rclone config create custommedia mediavfs \
    db_connection "postgres://user:password@localhost/mediadb?sslmode=disable" \
    download_url "http://localhost:8080/media/download" \
    table_name "my_media_table"
```

### Using Environment Variables

```bash
export RCLONE_CONFIG_MYMEDIA_TYPE=mediavfs
export RCLONE_CONFIG_MYMEDIA_DB_CONNECTION="postgres://user:password@localhost/mediadb?sslmode=disable"
export RCLONE_CONFIG_MYMEDIA_DOWNLOAD_URL="http://localhost:8080/media/download"

rclone ls mymedia:
```

## PostgreSQL Connection String Examples

### Local Connection

```
postgres://username:password@localhost/database_name?sslmode=disable
```

### Remote Connection with SSL

```
postgres://username:password@db.example.com:5432/database_name?sslmode=require
```

### With Connection Pool Settings

```
postgres://username:password@localhost/database_name?sslmode=disable&pool_max_conns=10
```

### Using Socket Connection

```
postgres://username:password@/database_name?host=/var/run/postgresql&sslmode=disable
```

## HTTP Download Server Requirements

Your HTTP download server should implement the following:

### 1. Basic Endpoint

```
GET /media/download/{media_key}
```

This should redirect or proxy to the actual file location.

### 2. Required Headers

The server MUST return these headers:

```http
HTTP/1.1 200 OK
Content-Length: 1048576
Accept-Ranges: bytes
ETag: "unique-file-identifier-v1"
```

### 3. Range Request Support

The server MUST support range requests:

```http
Request:
GET /media/download/abc123
Range: bytes=1024-2047

Response:
HTTP/1.1 206 Partial Content
Content-Range: bytes 1024-2047/1048576
Content-Length: 1024
Accept-Ranges: bytes
ETag: "unique-file-identifier-v1"
```

### 4. If-Range Support

The server SHOULD honor If-Range for conditional requests:

```http
Request:
GET /media/download/abc123
Range: bytes=1024-
If-Range: "unique-file-identifier-v1"

Response (if ETag matches):
HTTP/1.1 206 Partial Content
Content-Range: bytes 1024-1048575/1048576
ETag: "unique-file-identifier-v1"

Response (if ETag doesn't match):
HTTP/1.1 200 OK
Content-Length: 1048576
ETag: "unique-file-identifier-v2"
```

### Example Server Implementation (Go)

```go
func handleDownload(w http.ResponseWriter, r *http.Request) {
    mediaKey := chi.URLParam(r, "mediaKey")

    // Get actual file URL from database or cache
    actualURL := getActualFileURL(mediaKey)
    fileSize := getFileSize(mediaKey)
    etag := generateETag(mediaKey)

    // Set headers
    w.Header().Set("Accept-Ranges", "bytes")
    w.Header().Set("ETag", etag)

    // Handle range requests
    rangeHeader := r.Header.Get("Range")
    ifRange := r.Header.Get("If-Range")

    if rangeHeader != "" {
        // Check If-Range if present
        if ifRange != "" && ifRange != etag {
            // ETag mismatch, return full file
            http.Redirect(w, r, actualURL, http.StatusFound)
            return
        }

        // Parse and handle range
        // ... implementation details ...
    }

    // Redirect to actual file
    http.Redirect(w, r, actualURL, http.StatusFound)
}
```

### Example Server Implementation (Python/Flask)

```python
from flask import Flask, request, redirect, make_response

@app.route('/media/download/<media_key>')
def download_media(media_key):
    # Get actual file URL from database
    actual_url = get_actual_file_url(media_key)
    file_size = get_file_size(media_key)
    etag = generate_etag(media_key)

    # Check for range request
    range_header = request.headers.get('Range')
    if_range = request.headers.get('If-Range')

    response = redirect(actual_url, code=302)
    response.headers['Accept-Ranges'] = 'bytes'
    response.headers['ETag'] = etag

    if range_header and if_range:
        # Validate If-Range
        if if_range != etag:
            # Return full file on ETag mismatch
            response.status_code = 200

    return response
```

## Usage Examples

### Copy Files

```bash
# Copy a specific file
rclone copy mymedia:john/photo.jpg ~/Downloads/

# Copy entire user directory
rclone copy mymedia:john/ ~/Downloads/john/

# Sync with auto-delete
rclone sync mymedia:john/photos/ ~/Photos/john/
```

### Move/Rename Files

```bash
# Rename a file
rclone moveto mymedia:john/old.jpg mymedia:john/new.jpg

# Move to subdirectory
rclone moveto mymedia:john/photo.jpg mymedia:john/2024/photo.jpg

# Move multiple files
rclone move mymedia:john/temp/ mymedia:john/archive/
```

### Mount as Filesystem

```bash
# Basic mount
rclone mount mymedia: /mnt/media

# Mount with caching for better performance
rclone mount mymedia: /mnt/media --vfs-cache-mode full

# Mount with specific user
rclone mount mymedia:john /mnt/john --vfs-cache-mode writes

# Mount as read-only
rclone mount mymedia: /mnt/media --read-only
```

### Video Streaming

```bash
# Mount with optimizations for video streaming
rclone mount mymedia: /mnt/media \
    --vfs-cache-mode full \
    --vfs-read-ahead 128M \
    --buffer-size 64M \
    --vfs-read-chunk-size 32M

# Now you can play videos with seeking support
vlc /mnt/media/john/videos/movie.mp4
```

### Advanced Operations

```bash
# Check what will be copied (dry run)
rclone copy mymedia:john/ ~/backup/ --dry-run -v

# Copy only files modified in last 24 hours
rclone copy mymedia:john/ ~/backup/ --max-age 24h

# Copy files larger than 10MB
rclone copy mymedia:john/ ~/backup/ --min-size 10M

# Parallel transfers for speed
rclone copy mymedia:john/ ~/backup/ --transfers 8 --checkers 16
```

## Troubleshooting

### Enable Debug Logging

```bash
rclone ls mymedia: -vv
```

This will show:
- Database queries
- HTTP requests and responses
- ETag tracking
- Range request handling
- Retry attempts

### Test Database Connection

```bash
rclone backend version mymedia:
```

### Test HTTP Download

```bash
curl -v http://localhost:8080/media/download/your-media-key
```

Verify that the response includes:
- `Accept-Ranges: bytes` header
- `ETag` header
- Proper redirect or file content

### Test Range Requests

```bash
curl -v -H "Range: bytes=0-1023" http://localhost:8080/media/download/your-media-key
```

Should return `206 Partial Content` with `Content-Range` header.

### Test If-Range Support

```bash
# Get ETag first
ETAG=$(curl -sI http://localhost:8080/media/download/your-media-key | grep -i etag | cut -d' ' -f2)

# Use If-Range
curl -v -H "Range: bytes=1024-" -H "If-Range: $ETAG" http://localhost:8080/media/download/your-media-key
```

Should return `206 Partial Content` if ETag matches.
