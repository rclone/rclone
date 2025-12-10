# Step-by-Step Guide: Compiling rclone with mediavfs Backend

## Prerequisites

### 1. Go Programming Language
- **Required version**: Go 1.21 or later
- **Your version**: Go 1.24.7 ✅
- Check with: `go version`

### 2. Build Tools
- gcc/clang (for CGO if needed)
- make (optional, but helpful)
- git (for version control)

---

## Step 1: Verify Your Environment

```bash
# Check you're in the rclone directory
pwd
# Should output: /home/user/rclone

# Verify mediavfs backend files exist
ls -la backend/mediavfs/
# Should show: mediavfs.go, httpreader.go, README.md, EXAMPLE_CONFIG.md

# Check Go version
go version
# Should be Go 1.21 or later
```

---

## Step 2: Download Dependencies

The mediavfs backend requires the PostgreSQL driver. Let's ensure all dependencies are ready:

```bash
# Download and tidy all dependencies
go mod download

# This may take a few minutes as it downloads all rclone dependencies
```

---

## Step 3: Build rclone

### Option A: Quick Build (Recommended for testing)

```bash
# Build rclone binary (takes 2-5 minutes)
go build

# This creates an 'rclone' binary in the current directory
```

### Option B: Build with Optimizations (Smaller binary)

```bash
# Build with optimizations and strip debug info
go build -trimpath -ldflags="-s -w"

# Creates a smaller, optimized binary
```

### Option C: Build for Production

```bash
# Build with version info
go build -trimpath -ldflags="-s -w -X github.com/rclone/rclone/fs.Version=mediavfs-custom"
```

---

## Step 4: Verify the Build

```bash
# Check the binary was created
ls -lh rclone

# Should show a file around 80-120 MB

# Test the binary
./rclone version
```

---

## Step 5: Verify mediavfs Backend is Available

```bash
# List all available backends (should include mediavfs)
./rclone config providers | grep mediavfs

# Or check the full list
./rclone config providers

# Get detailed help for mediavfs
./rclone backend help mediavfs
```

---

## Step 6: Install rclone (Optional)

### Option A: Install to /usr/local/bin

```bash
sudo cp rclone /usr/local/bin/
sudo chmod 755 /usr/local/bin/rclone
```

### Option B: Install to ~/bin (user directory)

```bash
mkdir -p ~/bin
cp rclone ~/bin/
chmod 755 ~/bin/rclone

# Make sure ~/bin is in your PATH
echo 'export PATH="$HOME/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

### Option C: Use in place

```bash
# Just use ./rclone from the current directory
./rclone version
```

---

## Step 7: Configure mediavfs

Create your first mediavfs remote:

```bash
# Interactive configuration
./rclone config

# Or non-interactive
./rclone config create mymedia mediavfs \
    db_connection "postgres://user:password@localhost/mediadb?sslmode=disable" \
    download_url "http://localhost:8080/media/download"
```

---

## Step 8: Test mediavfs

```bash
# List available remotes
./rclone listremotes

# Test listing users (should query your database)
./rclone lsd mymedia:

# List files for a specific user
./rclone ls mymedia:username

# Test with verbose output to see debug info
./rclone ls mymedia:username -vv
```

---

## Troubleshooting

### Issue: "cannot find package"

```bash
# Run go mod tidy to fix dependencies
go mod tidy
go mod download
```

### Issue: Build fails with CGO errors

```bash
# Build without CGO
CGO_ENABLED=0 go build
```

### Issue: Binary is too large

```bash
# Build with size optimization
go build -ldflags="-s -w"

# Further compress with upx (if installed)
upx --best rclone
```

### Issue: "mediavfs not found in providers"

```bash
# Check if backend is registered
grep mediavfs backend/all/all.go

# Should show:
# _ "github.com/rclone/rclone/backend/mediavfs"

# If not present, check your branch
git status
```

---

## Quick Reference Commands

```bash
# Full build process
cd /home/user/rclone
go mod download
go build
./rclone version

# Configure mediavfs
./rclone config create mymedia mediavfs \
    db_connection "postgres://user:pass@localhost/db" \
    download_url "http://localhost/download"

# Test it
./rclone lsd mymedia:
./rclone ls mymedia:username -vv
```

---

## Build Time Expectations

- **First build**: 3-5 minutes (downloads dependencies)
- **Subsequent builds**: 1-2 minutes (uses cache)
- **Binary size**: ~80-120 MB (uncompressed)
- **Binary size**: ~30-40 MB (compressed with upx)

---

## Next Steps

After successful compilation:

1. ✅ Configure your PostgreSQL database connection
2. ✅ Set up your HTTP download server
3. ✅ Test with `rclone ls` and `rclone copy`
4. ✅ Try mounting with `rclone mount`

See `backend/mediavfs/README.md` for detailed usage instructions.
See `backend/mediavfs/EXAMPLE_CONFIG.md` for configuration examples.
