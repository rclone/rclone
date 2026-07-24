# rclone Development Log

## Working State
**Session:** 1 | **Date:** 2026-07-24 | **Branch:** feat/ytfs

### Active Task
YouTube read-only filesystem backend via yt-dlp
- [x] Step 1: Create feature branch and scaffold backend structure
- [x] Step 2: Implement sanitizeName/unsanitizeName helpers (using `∕` U+2215)
- [x] Step 3: Wire up basic Fs interface (stub List/NewObject, read-only ops)
- [ ] Step 4: Implement yt-dlp metadata extraction (api/api.go)
- [ ] Step 5: Implement directory tree (channels, videos, playlists)
- [ ] Step 6: Implement streaming/playback integration
- [ ] Step 7: Add integration tests
- [ ] Step 8: Register backend in all.go, docs, navbar

### Key Files (current shape)
**`backend/ytfs/ytfs.go`** (NEW, ~150 lines)
Core Fs implementation: NewFs, sanitizeName/unsanitizeName helpers, read-only ops (Put/Mkdir/Rmdir return ErrorPermissionDenied). Stubs for List/NewObject.

**`backend/ytfs/ytfs_test.go`** (NEW, ~75 lines)
Tests for sanitizeName round-trip: "Video / Title" ↔ "Video ∕ Title", covering edge cases. All passing.

**`backend/ytfs/api/api.go`** (NEW, ~80 lines)
yt-dlp wrapper: Channel/Video/Playlist types and Client methods (stubs). Includes extractJSON helper for calling yt-dlp -J.

### Decisions (active)
- **D-1 (sanitize char)**: Use `∕` U+2215 DIVISION SLASH for raw `/` replacement (consistent with gmailfs/gcalfs).
- **D-2 (substring only)**: Sanitize user strings (titles, names) only before splicing into paths.
- **MVP scope**: channels, videos, playlists; yt-dlp API; VFS-based caching (rclone built-in).

### Next Steps
1. Flesh out api/api.go with yt-dlp calls (GetChannelInfo, GetVideoInfo, GetPlaylistInfo).
2. Implement directory listing pattern (channels/ → /channels/{channel-id}/ → videos/playlists).
3. Add Object type and Open() for streaming support.
4. Write integration tests (mocked yt-dlp, real tree navigation).
5. Register backend in `backend/all.go` and test harness.
6. Add documentation (docs/content/ytfs.md, etc.).

### Blockers
- None yet; yt-dlp needs to be installed in dev env for real tests.

### Watch Out
- Slash handling is critical — any title with "/" must roundtrip cleanly through sanitize/unsanitize.
- Cache key parity (learned from Sprint 3): ensure any cached channel/video data uses sanitized names as keys.
- Read-only invariant: all write ops must return ErrorPermissionDenied.

---

## Milestones
- [ ] Backend compiles and tests pass
- [ ] Directory tree structure (channels/videos/playlists) working
- [ ] Streaming/Open() implementation
- [ ] Integration tests
- [ ] Documentation & registration
- [ ] Ready for upstream PR

## Mistakes & Lessons
(None yet — fresh start.)

## Technical Debt & Future Ideas
- Consider OAuth2 support (YouTube Data API v3) as alternative to yt-dlp.
- Cache persistence to disk (e.g., ~/.cache/ytfs/) for metadata.
- Search support (searches/ directory for search results).
