# rclone Development Log

## Working State
**Session:** 2 | **Date:** 2026-07-24 | **Branch:** feat/ytfs

### Active Task
Fix PR #9657 CI failure and prepare for merge
- [x] Step 1: Identify CI failure (go.mod missing fsnotify dependency)
- [x] Step 2: Run go mod tidy to resolve deps
- [x] Step 3: Verify all tests pass locally (-race flag)
- [x] Step 4: Amend last commit with go.mod fix
- [x] Step 5: Force push to feat/ytfs branch
- [ ] Step 6: Wait for CI to pass and merge PR

### Key Files (current shape)
**`backend/ytfs/ytfs.go`** (914 lines)
Full Fs implementation with read-only enforcement, metadata caching, file watcher for hot reload, concurrent deduplication. Thread-safe with RWMutex for manifest swaps.

**`backend/ytfs/ytfs_test.go`** (4671 lines, 77.0% coverage)
Comprehensive test suite: 174 test functions covering metadata caching TTL/expiry, concurrent requests, file watcher debounce, XML escaping, context cancellation, stress tests (200 goroutines).

**`backend/ytfs/metadata/cache.go`** (285 lines)
Cache manager with 32 methods: TTL tracking, concurrent deduplication, size limits (2MB thumbnails, 10MB subtitles).

**`backend/ytfs/metadata/types.go`** (272 lines)
Type definitions: CacheConfig, VideoMetadata, NFOMovie, ThumbnailManifest, SubtitlesManifest, ChaptersManifest.

**`go.mod`** (FIXED)
Added `github.com/fsnotify/fsnotify v1.7.0` for file watcher implementation (was missing, caused CI failure).

### Decisions (active)
- **D-1 (sanitize char)**: Use `∕` U+2215 DIVISION SLASH for raw `/` replacement
- **D-2 (cache strategy)**: TTL-based (24h default, 7d max) with concurrent deduplication
- **D-3 (hot reload)**: File watcher with 500ms debounce, RWMutex for thread safety
- **D-4 (Emby metadata)**: NFO + thumbnails + SRT + chapters, lazy-loaded on first access

### Next Steps
1. Wait for GitHub CI to run with fixed go.mod
2. Merge PR #9657 into master when CI passes
3. Monitor that metadata caching feature works in real Emby setups

### Blockers
- None currently; CI should pass with go.mod fix

### Watch Out
- fsnotify is critical for hot reload — ensure it's correctly imported in ytfs.go
- Don't commit .agentic/ or docs/agents/ directories (already in .gitignore)

---

## Milestones
- [x] Core ytfs backend with channels/playlists/videos
- [x] JSON manifest support with hot reload
- [x] Metadata caching and Emby integration  
- [x] Comprehensive test coverage (77%)
- [x] Complete documentation with use cases
- [x] Fix CI build failure (go.mod)
- [ ] Merge PR #9657 to master

## Session Archive

### Session 1 -- 2026-07-24: Implement ytfs with metadata and Emby support
**What we did:** Implemented complete YouTube filesystem backend with three-phase rollout: base backend (82.6% coverage), JSON manifest (86% coverage), metadata caching + hot reload + Emby integration (77% coverage). 3 commits, comprehensive test suite, full documentation.
**Files:** backend/ytfs/*, docs/content/ytfs.md, DEVLOG.md
**Decisions:** Slash sanitization with U+2215; metadata TTL-based caching; file watcher for hot reload; NFO/thumbnail/SRT/chapters support for Emby; read-only invariant enforcement

## Mistakes & Lessons
### 2026-07-24 - Command injection vulnerability in Object.Open()
**What happened:** Initial implementation used `sh -c "yt-dlp ... " + videoID` which allowed command injection
**Root cause:** Shell string concatenation without escaping
**How we fixed it:** Changed to exec.CommandContext with discrete args: `exec.CommandContext(ctx, "yt-dlp", "-f", "best", "-o", "-", videoID)`
**Lesson:** Always use exec with discrete arguments, never concatenate shell commands

### 2026-07-24 - Missing go.mod dependency caused CI failure
**What happened:** PR showed DIRTY status with ACTION_REQUIRED on CI check, blocking merge
**Root cause:** Added fsnotify for file watcher but didn't run `go mod tidy` before committing
**How we fixed it:** Ran go mod tidy, amended last commit with go.mod/go.sum changes, force pushed
**Lesson:** Always run `go mod tidy` after adding new imports; CI catches build issues before merge

## Technical Debt & Future Ideas
- OAuth2 support for private videos (YouTube Data API v3 alternative)
- Cache persistence to disk (~/.cache/ytfs/) for metadata longevity
- Search support (searches/ directory for search results)
- Combine storage layer for writable uploads alongside read-only YouTube content
