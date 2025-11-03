# Level3 Interactive Test Plan with MinIO

## Prerequisites ✅

- [x] 3 MinIO instances running (minioeven, minioodd, minioparity)
- [x] rclone.conf configured with S3 remotes
- [x] miniolevel3 configured with timeout_mode

## Test Environment

```ini
[minioeven]
type = s3
provider = Minio
endpoint = http://127.0.0.1:9001

[minioodd]
type = s3
provider = Minio
endpoint = http://127.0.0.1:9002

[minioparity]
type = s3
provider = Minio
endpoint = http://127.0.0.1:9003

[miniolevel3]
type = level3
even = minioeven:
odd = minioodd:
parity = minioparity:
timeout_mode = aggressive  # ADD THIS!
```

## Important: Add timeout_mode

First, update your config:

```bash
./rclone config update miniolevel3 timeout_mode aggressive
```

This will reduce degraded mode timeout from 2-5 minutes to 10-20 seconds.

---

## Test 1: Basic Functionality ✅

### 1.1 Create Bucket
```bash
./rclone mkdir miniolevel3:testbucket
```

**Expected**: Bucket created on all 3 MinIO instances

**Verify**:
```bash
./rclone lsd minioeven:
./rclone lsd minioodd:
./rclone lsd minioparity:
# All should show "testbucket"
```

### 1.2 Upload File
```bash
echo "Hello RAID3 World!" > /tmp/test.txt
./rclone copy /tmp/test.txt miniolevel3:testbucket/ -vv
```

**Expected Output**:
```
NOTICE: level3: Using aggressive timeout mode (retries=1, contimeout=5s, timeout=10s)
```

**Verify Particles**:
```bash
# Check even particle
./rclone cat minioeven:testbucket/test.txt | hexdump -C

# Check odd particle  
./rclone cat minioodd:testbucket/test.txt | hexdump -C

# Check parity particle
./rclone ls minioparity:testbucket/
# Should show: test.txt.parity-el or test.txt.parity-ol
```

### 1.3 Download File
```bash
./rclone cat miniolevel3:testbucket/test.txt
```

**Expected**: `Hello RAID3 World!`

### 1.4 List Files
```bash
./rclone ls miniolevel3:testbucket/
```

**Expected**: Only shows `test.txt` (parity files hidden)

---

## Test 2: Timeout Mode Verification 🕐

### 2.1 Test Standard Mode (Slow)
```bash
# Update to standard mode
./rclone config update miniolevel3 timeout_mode standard

# Stop one MinIO instance
docker stop minioodd

# Try to read (this will be SLOW - 2-5 minutes)
time ./rclone cat miniolevel3:testbucket/test.txt
```

**Expected**: 
- Takes 2-5 minutes to timeout
- Eventually fails or reconstructs (depending on implementation state)

### 2.2 Test Aggressive Mode (Fast)
```bash
# Update to aggressive mode
./rclone config update miniolevel3 timeout_mode aggressive

# Restart minioodd
docker start minioodd

# Wait for it to be ready
sleep 5

# Stop minioodd again
docker stop minioodd

# Try to read (should be much faster)
time ./rclone cat miniolevel3:testbucket/test.txt -vv
```

**Expected**:
- Takes 10-20 seconds to timeout
- Logs show: "Using aggressive timeout mode (retries=1, contimeout=5s, timeout=10s)"
- Should attempt reconstruction from even+parity

### 2.3 Test Balanced Mode (Medium)
```bash
# Update to balanced mode
./rclone config update miniolevel3 timeout_mode balanced

# With minioodd still stopped
time ./rclone cat miniolevel3:testbucket/test.txt -vv
```

**Expected**:
- Takes 30-60 seconds
- Logs show: "Using balanced timeout mode (retries=3, contimeout=15s, timeout=30s)"

**Restart minioodd for next tests**:
```bash
docker start minioodd
sleep 5
```

---

## Test 3: RAID 3 Degraded Mode 🔧

### 3.1 Test with Odd Backend Down
```bash
# Set back to aggressive mode
./rclone config update miniolevel3 timeout_mode aggressive

# Stop odd backend
docker stop minioodd

# Upload new file (should fail - writes require all backends)
echo "New file during degraded" > /tmp/degraded.txt
./rclone copy /tmp/degraded.txt miniolevel3:testbucket/ -vv
```

**Expected**: Upload fails (writes require all backends)

```bash
# Read existing file (should work via reconstruction)
./rclone cat miniolevel3:testbucket/test.txt -vv
```

**Expected**:
- Takes 10-20 seconds
- Logs show: "Reconstructed test.txt from even+parity (degraded mode)"
- Content matches original

**Restart minioodd**:
```bash
docker start minioodd
sleep 5
```

### 3.2 Test with Even Backend Down
```bash
# Stop even backend
docker stop minioeven

# Read existing file (should work via reconstruction)
./rclone cat miniolevel3:testbucket/test.txt -vv
```

**Expected**:
- Takes 10-20 seconds
- Logs show: "Reconstructed test.txt from odd+parity (degraded mode)"
- Content matches original

**Restart minioeven**:
```bash
docker start minioeven
sleep 5
```

### 3.3 Test with Parity Backend Down
```bash
# Stop parity backend
docker stop minioparity

# Read existing file (should work from even+odd)
./rclone cat miniolevel3:testbucket/test.txt -vv
```

**Expected**:
- Fast (no reconstruction needed)
- Content matches original

**Restart minioparity**:
```bash
docker start minioparity
sleep 5
```

---

## Test 4: Data Integrity 🔍

### 4.1 Upload Various File Sizes
```bash
# Even-length file (14 bytes)
echo "Hello, World!" > /tmp/even.txt
./rclone copy /tmp/even.txt miniolevel3:testbucket/

# Odd-length file (13 bytes)
echo "Hello World!" > /tmp/odd.txt
./rclone copy /tmp/odd.txt miniolevel3:testbucket/

# Single byte
echo -n "A" > /tmp/single.txt
./rclone copy /tmp/single.txt miniolevel3:testbucket/

# Empty file
touch /tmp/empty.txt
./rclone copy /tmp/empty.txt miniolevel3:testbucket/

# Large file (1 MB)
dd if=/dev/urandom of=/tmp/large.bin bs=1M count=1
./rclone copy /tmp/large.bin miniolevel3:testbucket/
```

### 4.2 Verify Parity Suffixes
```bash
./rclone ls minioparity:testbucket/
```

**Expected**:
```
even.txt.parity-el      # 14 bytes -> even length
odd.txt.parity-ol       # 13 bytes -> odd length
single.txt.parity-ol    # 1 byte -> odd length
empty.txt.parity-el     # 0 bytes -> even length
large.bin.parity-el     # 1MB -> even length
```

### 4.3 Verify MD5 Hashes
```bash
# Calculate MD5 of originals
md5sum /tmp/even.txt /tmp/odd.txt /tmp/single.txt /tmp/large.bin

# Download and calculate MD5
./rclone cat miniolevel3:testbucket/even.txt | md5sum
./rclone cat miniolevel3:testbucket/odd.txt | md5sum
./rclone cat miniolevel3:testbucket/single.txt | md5sum
./rclone cat miniolevel3:testbucket/large.bin | md5sum
```

**Expected**: All MD5 hashes match

---

## Test 5: Particle Inspection 🔬

### 5.1 Examine Particle Sizes
```bash
# For "Hello, World!" (14 bytes)
./rclone size minioeven:testbucket/even.txt
# Expected: 7 bytes

./rclone size minioodd:testbucket/even.txt
# Expected: 7 bytes

./rclone size minioparity:testbucket/even.txt.parity-el
# Expected: 7 bytes
```

### 5.2 Verify XOR Parity
```bash
# Download particles
./rclone cat minioeven:testbucket/even.txt > /tmp/even_particle.bin
./rclone cat minioodd:testbucket/even.txt > /tmp/odd_particle.bin
./rclone cat minioparity:testbucket/even.txt.parity-el > /tmp/parity_particle.bin

# View in hex
echo "Even particle:"
hexdump -C /tmp/even_particle.bin

echo "Odd particle:"
hexdump -C /tmp/odd_particle.bin

echo "Parity particle:"
hexdump -C /tmp/parity_particle.bin
```

**Manual Verification**:
- Parity[i] should equal Even[i] XOR Odd[i]
- For "Hello, World!":
  - Even: H, l, o, ,, W, r, d
  - Odd: e, l, (space), o, l, !
  - Parity: H^e, l^l, o^(space), ,^o, W^r, r^l, d^!

---

## Test 6: Operations Testing 🛠️

### 6.1 Move Operation
```bash
./rclone move miniolevel3:testbucket/single.txt miniolevel3:testbucket/renamed.txt -vv
```

**Expected**: All three particles moved

**Verify**:
```bash
./rclone ls miniolevel3:testbucket/
# Should show "renamed.txt", not "single.txt"

./rclone ls minioparity:testbucket/
# Should show "renamed.txt.parity-ol"
```

### 6.2 Delete Operation
```bash
./rclone delete miniolevel3:testbucket/renamed.txt -vv
```

**Expected**: All three particles deleted

**Verify**:
```bash
./rclone ls minioeven:testbucket/ | grep renamed
./rclone ls minioodd:testbucket/ | grep renamed
./rclone ls minioparity:testbucket/ | grep renamed
# All should return nothing
```

### 6.3 Sync Operation
```bash
# Create local directory with files
mkdir -p /tmp/sync_test
echo "File 1" > /tmp/sync_test/file1.txt
echo "File 2" > /tmp/sync_test/file2.txt
echo "File 3" > /tmp/sync_test/file3.txt

# Sync to level3
./rclone sync /tmp/sync_test/ miniolevel3:testbucket/sync/ -vv

# Verify
./rclone ls miniolevel3:testbucket/sync/
```

**Expected**: All 3 files uploaded with particles

---

## Test 7: Performance Testing ⚡

### 7.1 Upload Speed
```bash
# Create 10 MB file
dd if=/dev/urandom of=/tmp/10mb.bin bs=1M count=10

# Time upload
time ./rclone copy /tmp/10mb.bin miniolevel3:testbucket/ -vv
```

**Note**: 3x parallel writes (even, odd, parity)

### 7.2 Download Speed
```bash
# Time download
time ./rclone cat miniolevel3:testbucket/10mb.bin > /dev/null
```

**Note**: 2x parallel reads (even, odd) + merge

### 7.3 Degraded Mode Performance
```bash
# Stop one backend
docker stop minioodd

# Time degraded download
time ./rclone cat miniolevel3:testbucket/10mb.bin > /dev/null -vv
```

**Expected**: 
- 10-20 second timeout delay
- Then reconstruction from even+parity
- Logs show reconstruction message

**Restart**:
```bash
docker start minioodd
```

---

## Test 8: Error Handling 🚨

### 8.1 Two Backends Down (Should Fail)
```bash
# Stop two backends
docker stop minioodd minioparity

# Try to read
./rclone cat miniolevel3:testbucket/test.txt -vv
```

**Expected**: Fails with "insufficient particles" error

**Restart**:
```bash
docker start minioodd minioparity
sleep 5
```

### 8.2 Invalid Particle (Corruption Test)
```bash
# Corrupt odd particle
echo "CORRUPTED" | ./rclone rcat minioodd:testbucket/test.txt

# Try to read
./rclone cat miniolevel3:testbucket/test.txt -vv
```

**Expected**: Size validation error

**Fix**:
```bash
# Re-upload
./rclone copy /tmp/test.txt miniolevel3:testbucket/ --force
```

---

## Test 9: Cleanup 🧹

### 9.1 Delete Test Bucket
```bash
./rclone purge miniolevel3:testbucket -vv
```

**Expected**: Bucket and all particles removed from all 3 backends

**Verify**:
```bash
./rclone lsd minioeven:
./rclone lsd minioodd:
./rclone lsd minioparity:
# None should show testbucket
```

### 9.2 Stop MinIO Instances
```bash
docker stop minioeven minioodd minioparity
docker rm minioeven minioodd minioparity
```

### 9.3 Optional: Remove Data
```bash
# Only if you want to clean up completely
rm -rf ~/go/level3storage
```

---

## Expected Results Summary

| Test | Expected Result | Timeout (aggressive) |
|------|----------------|---------------------|
| Basic upload/download | ✅ Works | Instant |
| Standard mode degraded | ⚠️ Very slow | 2-5 minutes |
| Aggressive mode degraded | ✅ Fast | 10-20 seconds |
| Balanced mode degraded | ⚠️ Medium | 30-60 seconds |
| Reconstruction from even+parity | ✅ Works | 10-20s + reconstruction |
| Reconstruction from odd+parity | ✅ Works | 10-20s + reconstruction |
| Two backends down | ❌ Fails | 10-20s then error |
| Parity suffixes | ✅ Correct | N/A |
| MD5 integrity | ✅ Matches | N/A |
| XOR verification | ✅ Correct | N/A |

---

## Troubleshooting

### Issue: "connection refused"
**Solution**: Check MinIO containers are running:
```bash
docker ps | grep minio
```

### Issue: "SignatureDoesNotMatch"
**Solution**: Verify passwords in rclone.conf match Docker env vars

### Issue: Very slow even with aggressive mode
**Solution**: Check that timeout_mode is actually set:
```bash
./rclone config show miniolevel3
```

### Issue: Reconstruction not working
**Solution**: Verify parity files exist:
```bash
./rclone ls minioparity:testbucket/
```

---

## Quick Test Script

Save as `/tmp/test-level3.sh`:

```bash
#!/bin/bash
set -e

echo "=== Level3 MinIO Test Suite ==="

echo "1. Create bucket..."
./rclone mkdir miniolevel3:testbucket

echo "2. Upload test file..."
echo "Hello RAID3!" > /tmp/test.txt
./rclone copy /tmp/test.txt miniolevel3:testbucket/

echo "3. Verify download..."
./rclone cat miniolevel3:testbucket/test.txt

echo "4. Test degraded mode (stopping minioodd)..."
docker stop minioodd
sleep 2
echo "Reading with one backend down..."
time ./rclone cat miniolevel3:testbucket/test.txt -vv
docker start minioodd

echo "5. Cleanup..."
./rclone purge miniolevel3:testbucket

echo "=== All tests passed! ==="
```

Run with:
```bash
chmod +x /tmp/test-level3.sh
cd /Users/hfischer/go/src/rclone
/tmp/test-level3.sh
```

---

**Date**: November 1, 2025  
**Status**: Ready for interactive testing  
**Estimated time**: 30-45 minutes for full test suite

