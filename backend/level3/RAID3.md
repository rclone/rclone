# Level3 Backend - RAID 3 Implementation

## Overview

The `level3` backend implements RAID 3 storage with byte-level striping across three remotes:
- **Even remote**: Stores bytes at even indices (0, 2, 4, 6, ...)
- **Odd remote**: Stores bytes at odd indices (1, 3, 5, 7, ...)
- **Parity remote**: Stores XOR parity of even and odd bytes

## Data Distribution

### Even-Length Files (e.g., 14 bytes)

```
Original: [A, B, C, D, E, F, G, H, I, J, K, L, M, N]
           ↓ Split ↓
Even:     [A, C, E, G, I, K, M]      → 7 bytes
Odd:      [B, D, F, H, J, L, N]      → 7 bytes  
Parity:   [A^B, C^D, E^F, G^H, I^J, K^L, M^N] → 7 bytes (.parity-el)
```

### Odd-Length Files (e.g., 11 bytes)

```
Original: [A, B, C, D, E, F, G, H, I, J, K]
           ↓ Split ↓
Even:     [A, C, E, G, I, K]          → 6 bytes
Odd:      [B, D, F, H, J]             → 5 bytes
Parity:   [A^B, C^D, E^F, G^H, I^J, K] → 6 bytes (.parity-ol)
          └─────── XOR pairs ────┘  └─ last byte (no partner)
```

## Parity Calculation

### XOR Parity Logic

- For each pair: `parity[i] = even[i] XOR odd[i]`
- For odd-length files, last parity byte = last even byte (no XOR partner)
- Parity size always equals even particle size

### Filename Suffixes

Parity files are named with suffixes to indicate original data length:
- `.parity-el` - Even-length original data
- `.parity-ol` - Odd-length original data

This information is essential for future reconstruction from parity.

## Configuration

```ini
[raid3]
type = level3
even = /path/to/backend1     # Even-indexed bytes
odd = /path/to/backend2      # Odd-indexed bytes
parity = /path/to/backend3   # XOR parity
```

Example with cloud storage:

```ini
[raid3]
type = level3
even = s3:bucket1/data
odd = gdrive:backup/data
parity = dropbox:parity
```

## Example: "Hello, World!" (14 bytes)

```bash
Original data (hex):
48 65 6c 6c 6f 2c 20 57 6f 72 6c 64 21 0a
H  e  l  l  o  ,     W  o  r  l  d  !  \n

Even (7 bytes):
48 6c 6f 20 6f 6c 21
H  l  o     o  l  !

Odd (7 bytes):
65 6c 2c 57 72 64 0a
e  l  ,  W  r  d  \n

Parity (7 bytes) - saved as "test.txt.parity-el":
2d 00 43 77 1d 08 2b
│  │  │  │  │  │  └─ 0x21 XOR 0x0A = 0x2B
│  │  │  │  │  └──── 0x6C XOR 0x64 = 0x08
│  │  │  │  └─────── 0x6F XOR 0x72 = 0x1D
│  │  │  └────────── 0x20 XOR 0x57 = 0x77
│  │  └───────────── 0x6F XOR 0x2C = 0x43
│  └──────────────── 0x6C XOR 0x6C = 0x00
└─────────────────── 0x48 XOR 0x65 = 0x2D
```

## Verification

Upload a file:
```bash
echo "Hello, World!" > test.txt
rclone copy test.txt raid3:

# Verify particles
ls -l /path/to/backend1/test.txt         # 7 bytes (even)
ls -l /path/to/backend2/test.txt         # 7 bytes (odd)
ls -l /path/to/backend3/test.txt.parity-el  # 7 bytes (parity)

# Download and verify
rclone copy raid3:test.txt ./downloaded.txt
diff test.txt downloaded.txt  # Should be identical
```

## Operations

### Upload
- Splits data into even/odd bytes
- Calculates XOR parity
- Uploads to all three backends in parallel
- Parity file gets appropriate suffix (.parity-el or .parity-ol)

### Download
- Reads even and odd particles
- Validates sizes
- Merges bytes back to original
- Parity is ignored during download (used only for future reconstruction)

### Delete
- Removes all three particles (even, odd, parity)
- Searches for both .parity-el and .parity-ol suffixes

### List
- Shows union of files from even and odd backends
- Filters out parity files (hidden from user)
- Shows original (reconstructed) file sizes

## Future: Reconstruction from Parity

**When even particle is lost:**
```
For byte i in reconstructed:
  if i is even:
    data[i] = parity[i/2] XOR odd[i/2]
  else:
    data[i] = odd[i/2]
```

**When odd particle is lost:**
```
For byte i in reconstructed:
  if i is even:
    data[i] = even[i/2]
  else:
    data[i] = parity[i/2] XOR even[i/2]
```

**When parity is lost:**
- No problem - can still reconstruct from even+odd
- Can regenerate parity from even+odd

## Benefits of RAID 3

- **Future fault tolerance**: Will recover from single backend failure (to be implemented)
- **Parity storage**: Only ~50% overhead compared to full duplication
- **Byte-level granularity**: More thorough than block-level RAID

## Current Limitations

- Parity reconstruction not yet implemented (files need both even and odd particles)
- Cannot recover from failure of even or odd backend yet
- Memory buffering of entire files
- Cannot move files within same RAID 3 backend (rclone overlap detection)

## Testing

Comprehensive testing included:
- ✅ Unit tests for split/merge
- ✅ Unit tests for XOR parity calculation  
- ✅ Unit tests for parity reconstruction logic
- ✅ End-to-end tests with even and odd length files
- ✅ MD5 hash verification
- ✅ Particle size validation
- ✅ Deletion of all three particles

