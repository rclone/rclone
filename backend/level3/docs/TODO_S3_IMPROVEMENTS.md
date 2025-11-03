# Level3 S3 Improvements - Action Plan

## Summary of Current Status

✅ **Working**:
- Local filesystem backends (fully functional, all tests pass)
- RAID 3 byte splitting and XOR parity
- Degraded mode reconstruction (even+parity or odd+parity)
- All automated tests pass

❌ **Broken**:
- S3/MinIO degraded mode (hangs indefinitely when backend unavailable)

## Tomorrow's Tasks (Priority Order)

### HIGH PRIORITY - Document Current State

1. **Update TESTING.md**
   - [ ] Add section on S3 limitations
   - [ ] Document that S3 degraded mode doesn't currently work
   - [ ] Provide `--low-level-retries 1` workaround
   - [ ] Note 10-15 second expected delay with workaround
   - [ ] Link to S3_TIMEOUT_RESEARCH.md for details

2. **Update README.md**
   - [ ] Add "Limitations" section
   - [ ] Clear statement about S3 support status
   - [ ] Recommend local/file backends for production

3. **Add Runtime Warning**
   - [ ] Detect when backends are S3-based
   - [ ] Log warning on first access: "S3 backends detected - degraded mode may be slow"
   - [ ] Suggest `--low-level-retries 1` in warning

### MEDIUM PRIORITY - Test Workaround

4. **Verify `--low-level-retries 1` Solution**
   - [ ] Start 3 MinIO instances
   - [ ] Upload test file with all 3 running
   - [ ] Stop one MinIO instance
   - [ ] Test: `rclone cat miniolevel3:file --low-level-retries 1 --contimeout 5s`
   - [ ] Measure actual timeout duration
   - [ ] Verify reconstruction succeeds
   - [ ] Document actual timings

5. **Update Config Examples**
   - [ ] Add S3-optimized rclone config example
   - [ ] Show recommended flags for S3 usage
   - [ ] Add to TESTING.md MinIO section

### LOW PRIORITY - Future Improvements

6. **Design Phase 2 (probe_timeout option)**
   - [ ] Sketch out implementation approach
   - [ ] Identify where to inject timeout context
   - [ ] Research AWS SDK context cancellation behavior
   - [ ] Document in S3_TIMEOUT_RESEARCH.md

7. **Consider Upstream Contribution**
   - [ ] Draft issue for rclone GitHub
   - [ ] Propose health check layer for virtual backends
   - [ ] Share research findings with rclone maintainers

## Decision Points for Tomorrow

### Question 1: Scope for Initial Release

**Option A**: Release as "local backends only"
- ✅ Clear expectations
- ✅ No broken functionality
- ❌ Limits usefulness

**Option B**: Release with documented S3 limitations
- ✅ More useful immediately
- ✅ S3 works with workaround
- ⚠️ Need clear docs to avoid user frustration

**Recommendation**: Option B with very clear warnings

### Question 2: How Much Effort on S3?

**Option A**: Document and move on
- Time: 1-2 hours
- Users can use workaround
- Focus on other features

**Option B**: Implement Phase 2 (probe_timeout)
- Time: 1-2 days
- Better S3 support (10-15s failover)
- Still not as good as commercial solutions

**Option C**: Implement Phase 3 (health checking)
- Time: 1-2 weeks
- Production-grade S3 support
- Major undertaking

**Recommendation**: Start with Option A, consider Option B if S3 use cases are important

### Question 3: Testing Priority

What to test tomorrow:

**Must test**:
- [ ] S3 with `--low-level-retries 1` actually works
- [ ] Document actual timeout values observed

**Nice to test**:
- [ ] Real AWS S3 (not just MinIO)
- [ ] Different S3 providers (Wasabi, Backblaze B2, etc.)
- [ ] Large files in degraded mode

**Can skip for now**:
- Stress testing with many failures
- Auto-recovery scenarios
- Performance benchmarks

## Success Criteria for Tomorrow

**Minimum**:
- [ ] Clear documentation of S3 limitations
- [ ] Workaround tested and documented
- [ ] Warning logs implemented
- [ ] User expectations properly set

**Stretch Goals**:
- [ ] Tested with real AWS S3
- [ ] Phase 2 design document complete
- [ ] Upstream issue drafted

## Notes / Reminders

- The RAID 3 logic itself is solid - this is purely an S3 timeout issue
- Union backend has the same problem, so this isn't our bug
- Commercial solutions use health checking - that's the "real" fix
- Local backends work great, which proves the concept

## Files to Review Tomorrow

1. `/Users/hfischer/go/src/rclone/backend/level3/S3_TIMEOUT_RESEARCH.md` - Research findings
2. `/Users/hfischer/go/src/rclone/backend/level3/TESTING.md` - Update with S3 section
3. `/Users/hfischer/go/src/rclone/backend/level3/README.md` - Add limitations
4. `/Users/hfischer/go/src/rclone/backend/level3/level3.go` - Add warning logs

## Questions to Answer

1. Do we want to support S3 in v1, or wait until we have health checking?
2. Is 10-15 second degraded failover acceptable for S3 use cases?
3. Should we recommend level3 for S3, or only for local storage?
4. Do we need to test with other S3 providers besides MinIO?

---

**Prepared**: October 31, 2025  
**Status**: Ready for tomorrow's session  
**Estimated time**: 2-4 hours for documentation + testing

