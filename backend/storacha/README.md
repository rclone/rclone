# Storacha Backend for rclone

This backend implements directory tree management with proper CID propagation for Storacha/IPFS.

## Setup

### 1. Install Node.js Dependencies

```bash
cd backend/storacha
npm install
```

This will install:
- `@storacha/client` - Storacha web3.storage client
- `@ipld/dag-pb` - IPLD DAG-PB codec for directory structures
- `@ipld/car` - CAR file format support
- `ipfs-unixfs` - UnixFS data structures
- `multiformats` - CID and multihash support

### 2. Configure rclone

```bash
rclone config
```

Choose:
- `n` for new remote
- Name: `mystoracha`
- Storage: `storacha`
- `space_did`: Your Storacha space DID (e.g., `did:key:...`)
- `email`: Your email for authentication (optional if already logged in)

### 3. Usage

```bash
# List files
rclone ls mystoracha:

# Upload a file
rclone copy localfile.txt mystoracha:path/to/file.txt

# Create a directory
rclone mkdir mystoracha:newdir

# Copy within same remote (server-side, instant!)
rclone copy mystoracha:file1.txt mystoracha:file2.txt

# Sync directories
rclone sync local/dir mystoracha:remote/dir
```

## How It Works

### Directory Tree Structure

Storacha uses content-addressed storage where:
- Files are stored with immutable CIDs
- Directories are DAG-PB nodes containing links to children
- When you add/modify a file, ALL parent directories get new CIDs

Example:
```
Root (CID_R1)
  └─ folder1 (CID_F1)
       └─ folder2 (CID_F2)
            └─ file.txt (CID_file)

After adding newfile.txt:
Root (CID_R2)  ← NEW!
  └─ folder1 (CID_F1_new)  ← NEW!
       └─ folder2 (CID_F2_new)  ← NEW!
            ├─ file.txt (CID_file)  ← Same
            └─ newfile.txt (CID_newfile)  ← New
```

### CID Propagation

When uploading `a/b/c/file.txt`:

1. **Upload file** → Get CID_file
2. **Walk down**: Root → a/ → b/ → c/
3. **Walk back up**, creating new directories:
   - New c/ with file.txt link → CID_c_new
   - New b/ with c/ link → CID_b_new  
   - New a/ with b/ link → CID_a_new
   - New Root with a/ link → CID_root_new
4. **Update space** to point to CID_root_new

**Efficiency**: Only directory metadata (~few KB) is uploaded, never file contents again!

### Server-Side Copy

Copying within the same remote is instant:
- Source file already has a CID
- Just update directory tree to add new link
- No data transfer needed!

## Implementation Details

### Key Methods (storacha-bridge.mjs)

- `init()` - Initialize client and get root CID
- `updatePath()` - Core method for CID propagation
- `mkdir()` - Create directories with tree updates
- `setSpaceRoot()` - Persist new root CID
- `getCIDForPath()` - Query CIDs by path
- `upload()` - Upload file content
- `copy()` - Server-side copy (instant!)
- `list()` - List directory contents from DAG
- `download()` - Retrieve file content

### Metadata Storage

- Local cache: `~/.config/rclone/storacha-meta/<space-did>.json`
- Stores path → CID mappings for quick lookups
- Also stores `_rootCID` for the space

### Caching

The Go backend caches:
- `rootCID` - Current space root
- `cidCache` - Path to CID mappings
- Thread-safe with `sync.RWMutex`

## Troubleshooting

### "Node.js not found"
Install Node.js 18 or higher: https://nodejs.org/

### "Space not found"
1. Login to Storacha: `w3 login <email>`
2. Create a space: `w3 space create`
3. Copy the space DID for rclone config

### "Directory not found" errors
The directory tree is being built on-demand. If parents don't exist, they're created automatically during `updatePath()`.

### Performance tips
- Initial setup creates empty root directory
- All operations are incremental (only modified paths updated)
- Directory blocks are small (~KB) so tree updates are fast
- File content never re-uploaded during moves/copies

## Architecture

```
┌─────────────┐
│   rclone    │
│  (Go code)  │
└──────┬──────┘
       │ JSON-RPC over stdin/stdout
       │
┌──────▼──────┐
│  Node.js    │
│   Bridge    │
│ (this file) │
└──────┬──────┘
       │ API calls
       │
┌──────▼──────┐
│  Storacha   │
│    /IPFS    │
└─────────────┘
```

## Development

To test the bridge standalone:

```bash
echo '{"id":1,"method":"init","params":{"spaceDID":"did:key:..."}}' | node storacha-bridge.mjs
```

To see debug output:
```bash
DEBUG=* rclone ls mystoracha: -vv
```

## References

- [IPFS MFS (Mutable File System)](https://github.com/ipfs/boxo/tree/main/mfs)
- [UnixFS Spec](https://github.com/ipfs/specs/blob/main/UNIXFS.md)
- [DAG-PB Spec](https://ipld.io/specs/codecs/dag-pb/spec/)
- [Storacha Docs](https://docs.web3.storage/)
