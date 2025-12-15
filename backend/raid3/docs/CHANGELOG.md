# Changelog

## [v1.1.0] - 2025-11-02

### 🚨 Critical Fixes

#### Fixed: Update Corruption (CRITICAL)
- **Issue**: Update operations could corrupt files when backend unavailable
- **Symptom**: `ERROR: invalid particle sizes: even=11, odd=14`
- **Impact**: Files became unreadable, data loss risk
- **Fix**: Added pre-flight health check before Update
- **Result**: Update now fails safely, original file preserved

#### Fixed: Move Creating Degraded Files
- **Issue**: Move operations created degraded files on retry
- **Symptom**: Moved file missing one particle
- **Impact**: Immediate degraded state, performance hit
- **Fix**: Added pre-flight health check before Move
- **Result**: Move now fails in degraded mode, file stays at original location

#### Fixed: Put Creating Degraded Files on Retry
- **Issue**: Put succeeded partially on retry attempts
- **Symptom**: New files created with missing particles
- **Impact**: Every new file required reconstruction
- **Fix**: Added pre-flight health check before Put
- **Result**: Put now fails immediately in degraded mode

### ✅ New Features

#### Pre-flight Health Check
- Added `checkAllBackendsAvailable()` function
- Tests all 3 backends before write operations
- 5-second timeout (parallel checks)
- Clear error messages
- Overhead: +0.2 seconds (acceptable)

#### Strict Write Enforcement
- Put/Update/Move blocked in degraded mode
- Matches hardware RAID 3 behavior
- Prevents corruption from rclone's retry logic
- Error: "write blocked in degraded mode (RAID 3 policy)"

#### Update Particle Size Validation
- Validates particle sizes after Update
- Detects corruption if it occurs
- Returns detailed error message
- Defense-in-depth safety measure

### ⚡ Performance Impact

| Operation | Before Fix | After Fix | Change |
|-----------|------------|-----------|--------|
| Put (normal) | ~1s | ~1.2s | +0.2s (health check) |
| Put (degraded) | Variable (retry) | ~5s (fail fast) | Faster, safer |
| Update (normal) | ~1s | ~1.2s | +0.2s (health check) |
| Update (degraded) | 🚨 Corruption | ~5s (fail safe) | **Fixed!** |
| Move (normal) | ~1s | ~1.2s | +0.2s (health check) |
| Move (degraded) | Created degraded | ~5s (fail safe) | **Fixed!** |

---

## [v1.0.0] - 2025-11-01

### Initial Release

#### Core Features
- ✅ RAID 3 byte-level striping
- ✅ XOR parity calculation
- ✅ Three-backend architecture (even, odd, parity)
- ✅ Degraded mode reads (2 of 3 backends)
- ✅ Automatic parity reconstruction
- ✅ Heal background uploads
- ✅ S3/MinIO support with timeout modes
- ✅ Comprehensive test suite

#### Known Issues (Fixed in v1.1.0):
- ⚠️ Update could corrupt files in degraded mode
- ⚠️ Move could create degraded files on retry
- ⚠️ Put could create degraded files on retry

---

**Note**: This changelog tracks major changes. For detailed implementation notes, see the documentation files in the `docs/` directory.
