# MediaVFS Quick Start Guide

## ✅ Compilation Complete!

Your rclone binary with mediavfs backend has been successfully compiled!

```
Binary: /home/user/rclone/rclone
Size: 102 MB
Version: rclone v1.73.0-DEV
Backend: mediavfs ✓ Available
```

---

## Quick Test

### 1. Verify Installation

```bash
cd /home/user/rclone

# Check version
./rclone version

# List all backends (mediavfs should be in the list)
./rclone config providers | grep mediavfs
```

### 2. Configure mediavfs

**Option A: Interactive Configuration**

```bash
./rclone config

# Then select:
# n) New remote
# name> mymedia
# Storage> mediavfs
# db_connection> postgres://user:password@localhost/dbname?sslmode=disable
# download_url> http://localhost:8080/gphotos/download
# table_name> media (or press Enter for default)
```

**Option B: Command Line Configuration**

```bash
./rclone config create mymedia mediavfs \
    db_connection "postgres://user:password@localhost/mediadb?sslmode=disable" \
    download_url "http://localhost:8080/gphotos/download"
```

**Option C: Environment Variables (No Config File)**

```bash
export RCLONE_CONFIG_MYMEDIA_TYPE=mediavfs
export RCLONE_CONFIG_MYMEDIA_DB_CONNECTION="postgres://user:pass@localhost/db?sslmode=disable"
export RCLONE_CONFIG_MYMEDIA_DOWNLOAD_URL="http://localhost:8080/download"

./rclone ls mymedia:
```

### 3. Test Your Setup

```bash
# List all configured remotes
./rclone listremotes

# List all users (top-level directories)
./rclone lsd mymedia:

# List files for a specific user
./rclone ls mymedia:username

# With verbose output (shows database queries and HTTP requests)
./rclone ls mymedia:username -vv

# Copy a file
./rclone copy mymedia:username/file.jpg ~/Downloads/

# Mount as filesystem
./rclone mount mymedia: /mnt/media --vfs-cache-mode full
```

---

## Prerequisites for mediavfs to Work

### 1. PostgreSQL Database

You need a PostgreSQL database with a table containing these columns:

```sql
CREATE TABLE media (
    media_key TEXT PRIMARY KEY,
    file_name TEXT NOT NULL,
    name TEXT,              -- Custom display name (optional)
    path TEXT,              -- Custom directory path (optional)
    user_name TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    utc_timestamp TIMESTAMP NOT NULL
    -- other columns are ignored
);
```

### 2. HTTP Download Server

You need an HTTP server that:

- Listens on the URL specified in `download_url`
- Accepts GET requests: `GET /download/{media_key}`
- Returns the file content (or redirects to it)
- **IMPORTANT**: Returns these headers for optimal performance:
  - `ETag: "unique-file-id"` (for file consistency checks)
  - `Accept-Ranges: bytes` (for range request support)
  - Honors `Range:` request header (returns 206 Partial Content)
  - Honors `If-Range:` header (conditional range requests)

**Example server response:**

```http
HTTP/1.1 200 OK
Content-Length: 1048576
Accept-Ranges: bytes
ETag: "abc123-v1"
Content-Type: image/jpeg

[file content]
```

---

## Example Usage Scenarios

### Scenario 1: Copy All Photos for a User

```bash
./rclone copy mymedia:alice/photos/ ~/Pictures/alice/ -P
```

### Scenario 2: Sync Folder (One-Way)

```bash
./rclone sync mymedia:bob/documents/ ~/Backups/bob/ --dry-run
# Remove --dry-run when ready
```

### Scenario 3: Mount for Video Streaming

```bash
mkdir /mnt/media
./rclone mount mymedia: /mnt/media \
    --vfs-cache-mode full \
    --vfs-read-ahead 128M \
    --buffer-size 64M

# Now play videos with seeking support
vlc /mnt/media/carol/videos/movie.mp4
```

### Scenario 4: Rename Files (Updates Database)

```bash
# Rename a file
./rclone moveto mymedia:dan/old.jpg mymedia:dan/new.jpg

# Move to subdirectory
./rclone moveto mymedia:dan/photo.jpg mymedia:dan/2024/photo.jpg
```

### Scenario 5: List Files Modified Recently

```bash
./rclone ls mymedia:eve/ --max-age 24h
```

---

## Troubleshooting

### Issue: "no remotes found"

```bash
# Check config file
cat ~/.config/rclone/rclone.conf

# Or create config interactively
./rclone config
```

### Issue: "connection refused" to PostgreSQL

```bash
# Test database connection
psql "postgres://user:password@localhost/dbname?sslmode=disable"

# Check if PostgreSQL is running
ps aux | grep postgres
```

### Issue: "HTTP error" when reading files

```bash
# Test download URL manually
curl -v http://localhost:8080/gphotos/download/your-media-key

# Check if download server is running
curl http://localhost:8080/health
```

### Issue: No files showing up

```bash
# Test with verbose logging
./rclone ls mymedia:username -vv

# This will show:
# - Database queries being executed
# - Number of rows returned
# - File paths being constructed
```

### Enable Debug Mode

```bash
# Full debug output
./rclone ls mymedia: -vv 2>&1 | less

# Save debug output to file
./rclone ls mymedia: -vv 2>&1 | tee debug.log
```

---

## Installation (Optional)

### Install Globally

```bash
# Copy to system binary directory
sudo cp rclone /usr/local/bin/
sudo chmod 755 /usr/local/bin/rclone

# Verify
rclone version
```

### Install to User Directory

```bash
# Copy to user bin
mkdir -p ~/bin
cp rclone ~/bin/
chmod 755 ~/bin/rclone

# Add to PATH (add to ~/.bashrc for persistence)
export PATH="$HOME/bin:$PATH"

# Verify
rclone version
```

---

## Performance Tips

### 1. Database Indexes

Add indexes for better query performance:

```sql
CREATE INDEX idx_media_user_name ON media(user_name);
CREATE INDEX idx_media_user_path ON media(user_name, path);
```

### 2. Connection Pooling

The backend automatically uses connection pooling with:
- Max 10 open connections
- Max 5 idle connections
- 1 hour connection lifetime

### 3. Parallel Transfers

Use multiple threads for faster operations:

```bash
./rclone copy mymedia:user/ ~/backup/ \
    --transfers 8 \
    --checkers 16
```

### 4. VFS Caching

For mount operations, use caching:

```bash
./rclone mount mymedia: /mnt/media \
    --vfs-cache-mode full \
    --dir-cache-time 5m \
    --vfs-cache-max-age 1h
```

---

## Next Steps

1. ✅ **Set up your PostgreSQL database** with the required schema
2. ✅ **Create your HTTP download server** with ETag and range support
3. ✅ **Configure rclone** with your database and download URL
4. ✅ **Test basic operations** (ls, copy, mount)
5. ✅ **Set up monitoring** (optional, use -vv for debug logs)

For more details, see:
- `/home/user/rclone/backend/mediavfs/README.md` - Full documentation
- `/home/user/rclone/backend/mediavfs/EXAMPLE_CONFIG.md` - Configuration examples
- `/home/user/rclone/backend/mediavfs/COMPILE_GUIDE.md` - Compilation guide

---

## Support

If you encounter issues:

1. Check verbose logs: `./rclone ls mymedia: -vv`
2. Verify database connection: `psql "your-connection-string"`
3. Test download server: `curl http://your-download-url/media-key`
4. Review documentation in `backend/mediavfs/`

Happy syncing! 🎉
