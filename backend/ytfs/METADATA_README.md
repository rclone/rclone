# YTFS Metadata & Emby Integration

This directory contains the design and implementation framework for YouTube video metadata caching and Emby media server compatibility in the rclone YTFS backend.

## Documentation Overview

### 1. **METADATA_DESIGN.md** (28 KB)
The complete specification document covering:
- Virtual filesystem structure for Emby
- NFO (Emby metadata XML) format with full schema
- Cache directory structure and organization
- TTL-based expiry mechanism with cleanup policies
- Metadata types and field definitions
- yt-dlp integration points
- Configuration schemas
- Example workflows and use cases
- Emby integration requirements
- Security and privacy considerations

**Read this for:** Understanding the complete architecture, specifications, and design rationale.

### 2. **METADATA_QUICK_REFERENCE.md** (9.4 KB)
Developer-focused quick reference containing:
- File location summary table
- NFO required/optional fields
- TTL defaults and rationale
- Implementation checklist (6 phases)
- Code examples for common operations
- Emby path mapping strategies
- Error handling guide
- Performance optimization tips
- Testing checklist
- Configuration examples
- Related files and references
- Future enhancements

**Read this for:** Getting started with implementation, code examples, and quick lookups.

### 3. **metadata/types.go** (8.1 KB)
Go type definitions providing:
- `CacheConfig` — Global cache configuration structure
- `VideoMetadata` — Video information from yt-dlp
- `NFOMovie` — Emby NFO XML representation
- `ThumbnailManifest` — Thumbnail tracking
- `SubtitlesManifest` — Subtitle tracking
- `ChaptersManifest` — Chapter marker tracking
- `ChannelMetadata` — Channel information
- `PlaylistMetadata` — Playlist information
- `CacheStats` — Cache usage statistics

**Use this for:** Type-safe implementation, marshaling/unmarshaling JSON and XML.

### 4. **metadata/cache.go** (7.8 KB)
Cache interface and manager implementation providing:
- `Cache` interface — Contract for all cache operations
- `Manager` — High-level cache operations
- `GenerateNFO()` — Convert video metadata to Emby NFO
- Helper functions for data transformation
- Error types for cache operations

**Use this for:** Understanding cache operations, implementing concrete cache backend.

## Directory Structure

```
backend/ytfs/
├── METADATA_README.md          (this file)
├── METADATA_DESIGN.md          (28 KB, full specification)
├── METADATA_QUICK_REFERENCE.md (9.4 KB, developer guide)
├── metadata/
│   ├── types.go               (Go type definitions)
│   ├── cache.go               (Cache interface & manager)
│   ├── store.go               (file operations — TODO)
│   └── expiry.go              (TTL management — TODO)
├── ytfs.go                     (existing: main filesystem)
├── ytfs_test.go                (existing: tests)
├── patterns.go                 (existing: URL routing)
└── api/
    ├── api.go                  (existing: yt-dlp wrapper)
    └── api_test.go             (existing: tests)
```

## Quick Start

### For Understanding the Architecture
1. Read **METADATA_DESIGN.md** sections 1-3 (directory structure and NFO format)
2. Review cache directory structure (section 3)
3. Look at example workflow (section 8)

### For Implementation
1. Review **METADATA_QUICK_REFERENCE.md** implementation checklist
2. Use **types.go** for data structure definitions
3. Reference **cache.go** for interface contracts
4. Follow code examples in quick reference
5. Implement in 6 phases as outlined

### For Testing
1. Review testing checklist in quick reference
2. Use configuration examples for test environment
3. Validate NFO files against schema
4. Test cache expiry with short TTLs (60 seconds)

## Key Design Decisions

### 1. Dual Storage Strategy
- **Virtual paths:** YTFS presents files at `channels/Channel_Name/video_id.mp4`, `video_id.nfo`, etc.
- **Physical cache:** Metadata stored in `~/.cache/ytfs/` organized by type and ID
- **Strategy:** Either symlink or virtual resolution layer connects them

### 2. NFO as Primary Metadata
- Single `.nfo` XML file per video follows Emby standard
- Generated from yt-dlp JSON dump
- Supports all Emby metadata fields: ratings, genres, thumbnails, captions, chapters
- Escapes XML properly and validates before writing

### 3. TTL-Based Expiry (Not Active Cleanup)
- Lazy expiry: Check when reading, re-fetch if expired
- Optional eager cleanup: Remove files older than TTL or if cache size exceeded
- Default TTLs: 7 days metadata, 30 days thumbnails
- Per-item tracking in `expiry/videos.json`

### 4. yt-dlp as Single Source of Truth
- All video/channel/playlist metadata from yt-dlp JSON output
- Subtitles extracted via yt-dlp's caption support
- Chapters parsed from description or explicit markers
- Thumbnails downloaded directly from YouTube CDN

### 5. Hierarchical Cache Organization
- Organized by type: `metadata/`, `thumbnails/`, `subtitles/`, `chapters/`
- Further organized by entity type: `videos/`, `channels/`, `playlists/`
- Per-entity organization by ID: `{videoID}/`, `{channelID}/`, etc.
- Manifest files track related files and metadata

## Implementation Roadmap

### Phase 1: Foundation (1 week)
- Implement `Cache` interface with filesystem backend
- Create cache directory and config initialization
- Implement TTL tracking and expiry checking

### Phase 2: Metadata (1 week)
- Implement video/channel/playlist metadata get/set
- Wire yt-dlp API client for fresh data
- Implement metadata validation

### Phase 3: NFO Generation (3-4 days)
- Implement `GenerateNFO()` with full schema
- Add XML serialization and validation
- Test with Emby to ensure compatibility

### Phase 4: Media Files (1 week)
- Implement thumbnail caching (multi-resolution)
- Implement subtitle caching (SRT format)
- Implement chapter marker caching

### Phase 5: Integration (3-4 days)
- Wire cache into YTFS filesystem (`Fs` struct)
- Auto-generate NFO when videos are listed
- Provide paths for thumbnails/subtitles to Emby

### Phase 6: Polish (3-4 days)
- Implement cleanup and cache statistics
- Add configuration options
- Comprehensive testing and documentation

**Total estimated time:** 4-5 weeks for full implementation

## Configuration

Default configuration at `~/.cache/ytfs/config.json`:

```json
{
  "version": "1.0",
  "cacheDir": "~/.cache/ytfs",
  "ttl": {
    "videoMetadata": 604800,
    "thumbnails": 2592000,
    "subtitles": 2592000,
    "chapters": 604800,
    "channelInfo": 1209600,
    "playlistInfo": 604800
  },
  "thumbnailQuality": {
    "default": "medium",
    "maxResolution": true
  },
  "subtitles": {
    "autoGenerated": true,
    "preferredLanguages": ["en", "de", "fr"],
    "maxLanguages": 5
  },
  "chapters": {
    "enabled": true,
    "cacheFormat": "xml"
  },
  "cleanupPolicy": {
    "enabled": true,
    "cleanupAge": 2592000,
    "maxCacheSize": 10737418240
  }
}
```

## Emby Integration

### What Emby Sees
When you mount YTFS with rclone:
```
/mnt/ytfs/
├── channels/
│   ├── Channel_A/
│   │   ├── vid_001.mp4              (virtual video stream)
│   │   ├── vid_001.nfo              (metadata XML)
│   │   ├── vid_001-thumb.jpg        (thumbnail)
│   │   └── vid_001.srt              (captions)
│   └── Channel_B/
│       └── ...
└── playlists/
    └── ...
```

### Emby Scanning Process
1. Emby scans folder for `.mp4`/`.mkv` files
2. Emby looks for `{filename}.nfo` alongside each video
3. Emby reads metadata from NFO file
4. Emby looks for `{filename}-thumb.jpg` for thumbnail
5. Emby looks for `{filename}.srt` for subtitles
6. Emby displays all metadata in library

## Testing with Emby

1. **Mock Setup:**
   ```bash
   mkdir -p /tmp/ytfs_test
   # Configure YTFS with cache_dir=/tmp/ytfs_test
   ```

2. **Generate Sample NFO:**
   - Use `GenerateNFO()` with test video metadata
   - Write to `/tmp/ytfs_test/metadata/videos/{id}/nfo.xml`

3. **Verify NFO:** 
   - Parse with xml.Unmarshal (Go)
   - Validate required fields present
   - Check date formats

4. **Emby Scan:**
   - Add YTFS mount to Emby library
   - Force library refresh
   - Verify videos appear with correct metadata

## Error Handling

All cache operations can return:
- `ErrExpired` — Cache entry too old, needs re-fetch
- `ErrNotFound` — Entry not in cache
- `ErrInvalidFormat` — Corrupted cache file
- Standard Go errors — Disk I/O, permissions, etc.

Implement graceful fallback for missing metadata:
- Missing thumbnail → Emby default
- Missing subtitles → Don't add to NFO
- Missing chapters → Omit from NFO
- Empty description → Use fallback text

## Security Considerations

1. **Cache Permissions:** 700 on directory, 600 on files
2. **No Secrets:** Cache only public YouTube metadata
3. **Expiry:** Old cache invalidated after TTL
4. **Cleanup:** Can safely delete entire cache directory
5. **Integrity:** Validate file formats before using

## Performance Targets

- **NFO Generation:** < 100ms per video
- **Thumbnail Caching:** < 2 seconds per image
- **Subtitle Parsing:** < 500ms per file
- **Cache Hit Rate:** > 95% for repeated accesses
- **Disk Usage:** < 10GB for 1000 videos with all media

## Debugging Tips

1. **Enable verbose logging** in yt-dlp calls
2. **Check cache file permissions** if access denied
3. **Validate NFO XML** before blaming Emby
4. **Test expiry** with short TTLs (60 seconds)
5. **Monitor cache size** to ensure cleanup works
6. **Review config** for typos or invalid values

## Related Files

- `ytfs.go` — Main YTFS filesystem (integrate cache here)
- `api/api.go` — yt-dlp client (data source for cache)
- `patterns.go` — URL routing patterns (may need updates for metadata paths)

## References

- **Emby Metadata Format:** XBMC NFO standard, compatible with Kodi
- **yt-dlp Documentation:** https://github.com/yt-dlp/yt-dlp
- **YouTube API:** Limited; yt-dlp scraping is the practical approach
- **SubRip Format:** RFC 2016 for `.srt` files

## Contributing

When implementing this design:

1. Follow Go idioms and project conventions
2. Use provided types and interfaces
3. Add comprehensive tests (80%+ coverage)
4. Update DEVLOG.md with progress
5. Document any deviations from design
6. Keep implementation modular (swappable backends)

## Questions & Issues

Refer to:
1. **Full specification:** METADATA_DESIGN.md (sections 1-7)
2. **Implementation guide:** METADATA_QUICK_REFERENCE.md
3. **Code examples:** metadata/cache.go and metadata/types.go
4. **Error handling:** Quick reference error handling section

---

**Document Version:** 1.0  
**Last Updated:** 2024-07-24  
**Status:** Ready for Implementation
