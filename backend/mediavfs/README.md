# Media VFS Backend

The Media VFS (mediavfs) backend provides a virtual filesystem interface to a PostgreSQL media database.

## Features

- **User-based directory structure**: Files are automatically organized by username (e.g., `mount/username/file.ext`)
- **Custom paths and names**: Support for storing custom file names and directory structures in the database
- **Read-only with move/rename**: Files can be read, moved, and renamed, but not written, uploaded, or deleted
- **HTTP redirect support**: File content is fetched via HTTP from a local server that redirects to the actual download URL
- **Range request support**: Supports partial file reads and resume functionality via HTTP range requests

## Configuration

To configure the mediavfs backend, you need:

1. **Database Connection String** (`db_connection`): PostgreSQL connection string
2. **Download URL** (`download_url`): Base URL for the HTTP server that provides file downloads
3. **Table Name** (`table_name`): Optional, name of the media table (defaults to "media")

### Example Configuration

```bash
rclone config create mymedia mediavfs \
    db_connection "postgres://user:password@localhost/mediadb?sslmode=disable" \
    download_url "http://localhost/gphotos/download" \
    table_name "media"
```

Or interactively:

```bash
rclone config
```

Then choose:
- `n` for new remote
- Name: `mymedia`
- Storage: `mediavfs`
- Fill in the required fields

## Database Schema Requirements

The backend expects a PostgreSQL table with the following columns:

- `media_key` (string): Unique identifier used to construct the download URL
- `file_name` (string): Original filename
- `name` (string, nullable): Custom display name (if set, used instead of file_name)
- `path` (string, nullable): Custom path within the user's directory
- `user_name` (string): Username for organizing files
- `size_bytes` (integer): File size in bytes
- `utc_timestamp` (timestamp): File modification time

Other columns in the schema are ignored by the backend.

## Usage

### List files by user

```bash
# List all users
rclone ls mymedia:

# List files for a specific user
rclone ls mymedia:john

# List files in a subdirectory
rclone ls mymedia:john/photos
```

### Copy files

```bash
# Copy a file from mediavfs to local storage
rclone copy mymedia:john/photo.jpg /local/destination/

# Copy all files from a user
rclone copy mymedia:john/ /local/destination/
```

### Move/Rename files

```bash
# Rename a file (updates the 'name' column in the database)
rclone moveto mymedia:john/oldname.jpg mymedia:john/newname.jpg

# Move a file to a different directory (updates the 'path' column)
rclone moveto mymedia:john/photo.jpg mymedia:john/photos/photo.jpg
```

**Note**: Files cannot be moved between different users. Attempting to do so will result in an error.

### Mount as a filesystem

```bash
# Mount the mediavfs as a local filesystem
rclone mount mymedia: /mnt/media --vfs-cache-mode full

# Now you can browse files
ls /mnt/media/
ls /mnt/media/john/
```

## How It Works

### File Structure

The virtual filesystem is organized as:
```
/
├── user1/
│   ├── file1.jpg
│   └── photos/
│       └── file2.jpg
├── user2/
│   ├── video.mp4
│   └── docs/
│       └── document.pdf
```

### File Reading

When you read a file:
1. The backend queries the database to get the `media_key`
2. It constructs a URL: `{download_url}/{media_key}`
3. An HTTP GET request is made to this URL
4. The local server redirects to the actual file location (e.g., on a CDN)
5. The file content is streamed back through rclone

### Range Requests and ETag Support

The backend features an intelligent HTTP client with advanced capabilities:

**ETag Support:**
- Automatically caches ETags from initial requests
- Uses `If-Range` headers with ETags to ensure file consistency
- Detects file changes on the server via ETag mismatches
- Prevents corrupted downloads when files are modified mid-stream

**Range Request Handling:**
- Full support for HTTP range requests (206 Partial Content)
- Automatic detection of server range support via `Accept-Ranges` header
- Falls back to full downloads with byte skipping if ranges aren't supported
- Enables efficient seeking in large files without downloading the entire file

**Automatic Resume:**
- Automatically retries on network errors (up to 3 attempts)
- Resumes from the exact byte position on connection failures
- Uses range requests to continue downloads seamlessly
- Resets retry counter on successful reads for long-running transfers

**Performance Optimizations:**
- Different reader implementations based on access patterns:
  - `seekableHTTPReader`: For random access and seeking (e.g., video playback)
  - `httpReader`: For streaming with range support (e.g., resume downloads)
  - `optimizedHTTPReader`: For simple full-file reads (e.g., copy operations)
- Minimal overhead for common operations
- Efficient connection reuse via HTTP client pooling

### Move/Rename Operations

When you move or rename a file:
1. The backend checks that source and destination users match
2. It parses the new path and filename from the destination
3. It updates the `name` and `path` columns in the database
4. The `media_key` remains unchanged (the actual file isn't moved)

## Limitations

- **Read-only**: You cannot upload new files or modify existing file content
- **No deletion**: Files cannot be deleted through rclone
- **Same-user moves only**: Files can only be moved/renamed within the same user's directory
- **No directory creation**: Directories are virtual and derived from the `path` column
- **No hash support**: File hashes are not supported (returns `hash.ErrUnsupported`)

## Use Cases

This backend is ideal for:
- Providing rclone access to media files stored in a database
- Creating a virtual filesystem view of database-backed file storage
- Enabling standard file operations on database-managed content
- Integrating with applications that store file metadata in PostgreSQL

## Technical Details

### HTTP Client Implementation

The backend uses a sophisticated HTTP client implementation that adapts to different use cases:

1. **ETag Tracking**: On the first request, the client stores the ETag from the server. On subsequent range requests, it includes `If-Range: <etag>` to ensure the file hasn't changed. If the ETag doesn't match, the server returns 200 OK with the full new file instead of 206 Partial Content.

2. **Automatic Recovery**: If a network error occurs during reading, the client automatically attempts to resume from the last successful position using a range request. It retries up to 3 times before giving up.

3. **Smart Reader Selection**: The `Open()` method analyzes the requested options and selects the optimal reader:
   - If seeking is needed (`fs.SeekOption`), uses `seekableHTTPReader` with full seek support
   - If range reading is needed (`fs.RangeOption`), uses `httpReader` with ETag tracking
   - Otherwise, uses `optimizedHTTPReader` for simple streaming

4. **Connection Pooling**: Uses rclone's standard HTTP client with connection pooling for efficient reuse of TCP connections.

### Example HTTP Flow

```
Initial Request:
  GET /gphotos/download/abc123 HTTP/1.1

  Response:
  HTTP/1.1 200 OK
  ETag: "abc123-v1"
  Accept-Ranges: bytes
  Content-Length: 1048576

Range Request with ETag:
  GET /gphotos/download/abc123 HTTP/1.1
  Range: bytes=524288-
  If-Range: "abc123-v1"

  Response (if file unchanged):
  HTTP/1.1 206 Partial Content
  ETag: "abc123-v1"
  Content-Range: bytes 524288-1048575/1048576

  Response (if file changed):
  HTTP/1.1 200 OK
  ETag: "abc123-v2"
  Content-Length: 1048576
```

## Example: Complete Setup

1. **Setup PostgreSQL with media data**
2. **Create HTTP redirect server** (already implemented per requirements)
   - Ensure server returns proper `ETag` headers for file consistency
   - Ensure server supports `Range` requests and returns `Accept-Ranges: bytes`
   - Ensure server honors `If-Range` header for conditional range requests
3. **Configure rclone**:
   ```bash
   rclone config create gphotos mediavfs \
       db_connection "postgres://user:pass@localhost/photos?sslmode=disable" \
       download_url "http://localhost/gphotos/download"
   ```
4. **Use it**:
   ```bash
   rclone ls gphotos:
   rclone copy gphotos:alice/vacation.jpg ~/Pictures/
   rclone moveto gphotos:alice/photo.jpg gphotos:alice/2024/photo.jpg

   # Mount for video streaming (automatic seeking support)
   rclone mount gphotos: /mnt/media --vfs-cache-mode full
   ```
