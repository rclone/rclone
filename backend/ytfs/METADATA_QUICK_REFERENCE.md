# YTFS Metadata Quick Reference

**For developers implementing the metadata caching system.**

## File Locations

| Purpose | Path | Format |
|---------|------|--------|
| NFO (Emby metadata) | `{virtual_path}/{videoID}.nfo` | XML |
| Thumbnail (cached) | `~/.cache/ytfs/thumbnails/videos/{videoID}.jpg` | JPEG/PNG |
| Subtitles (cached) | `~/.cache/ytfs/subtitles/{videoID}_{lang}.srt` | SRT |
| Chapter markers | `~/.cache/ytfs/chapters/{videoID}.xml` | XML/JSON |
| Video info dump | `~/.cache/ytfs/metadata/videos/{videoID}/info.json` | JSON |
| Cache config | `~/.cache/ytfs/config.json` | JSON |
| Expiry tracker | `~/.cache/ytfs/expiry/videos.json` | JSON |

## NFO Required Fields (Emby)

Minimum fields to include in every `.nfo` file:

```xml
<title>Video Title</title>
<plot>Description text</plot>
<runtime>300</runtime>                    <!-- Seconds, required -->
<aired>2024-07-24</aired>                 <!-- YYYY-MM-DD, required -->
<uniqueid type="youtube">videoID</uniqueid>
```

## NFO Optional Fields (Recommended)

```xml
<year>2024</year>
<director>Channel Name</director>
<actor>...</actor>
<genre>Music</genre>
<thumb>...</thumb>
<views>1000000</views>
<likes>50000</likes>
```

## TTL Defaults

| Data Type | TTL | Rationale |
|-----------|-----|-----------|
| Video Metadata | 7 days | Changes infrequently (views/likes update slowly) |
| Thumbnails | 30 days | Static on YouTube |
| Subtitles | 30 days | Static (YouTube rarely updates) |
| Chapters | 7 days | May be edited by uploader |
| Channel Info | 14 days | Slower change frequency |
| Playlist Info | 7 days | Videos added/removed frequently |

## Cache Directory Structure (Abbreviated)

```
~/.cache/ytfs/
├── config.json
├── metadata/
│   ├── videos/{videoID}/
│   │   ├── info.json
│   │   ├── nfo.xml (if generated)
│   │   ├── description.txt
│   │   └── subtitles_manifest.json
│   ├── channels/{channelID}/info.json
│   └── playlists/{playlistID}/info.json
├── thumbnails/videos/{videoID}.jpg
├── subtitles/{videoID}_{lang}.srt
├── chapters/{videoID}.xml
└── expiry/
    ├── videos.json
    ├── thumbnails.json
    └── subtitles.json
```

## Implementation Checklist

### Phase 1: Cache Infrastructure
- [ ] Implement `Cache` interface (see `metadata/cache.go`)
- [ ] Implement cache directory initialization
- [ ] Implement config loading/saving
- [ ] Implement file I/O for JSON, XML, text

### Phase 2: Metadata Operations
- [ ] Implement video metadata caching (get/set)
- [ ] Implement channel metadata caching
- [ ] Implement playlist metadata caching
- [ ] Implement TTL tracking

### Phase 3: Media Files
- [ ] Implement thumbnail caching (multiple resolutions)
- [ ] Implement subtitle caching (SRT format)
- [ ] Implement chapter marker caching (XML/JSON)

### Phase 4: NFO Generation
- [ ] Implement NFO schema struct (use `types.NFOMovie`)
- [ ] Implement `GenerateNFO()` from `VideoMetadata`
- [ ] Implement XML serialization
- [ ] Validate NFO before writing

### Phase 5: Cleanup & Maintenance
- [ ] Implement expiry checking (`IsValid()`)
- [ ] Implement cleanup by TTL
- [ ] Implement cleanup by max cache size
- [ ] Implement cache statistics

### Phase 6: Integration
- [ ] Wire cache into YTFS filesystem (`Fs` struct)
- [ ] Generate NFO files when videos are listed
- [ ] Provide thumbnail paths to Emby
- [ ] Provide subtitle paths to Emby

## Code Examples

### Initializing Cache

```go
func initializeYTFSCache(ctx context.Context) (metadata.Cache, error) {
    cacheDir := expandHome("~/.cache/ytfs")
    
    cache := metadata.NewFileSystemCache(cacheDir)
    err := cache.Initialize(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to initialize cache: %w", err)
    }
    
    return cache, nil
}
```

### Generating NFO for a Video

```go
func generateVideoNFO(ctx context.Context, videoID string, 
    cache metadata.Cache, manager *metadata.Manager) error {
    
    // Load video metadata from cache (or fetch fresh)
    video, err := cache.GetVideoMetadata(ctx, videoID)
    if err != nil {
        // Video not cached, would need to fetch via yt-dlp
        return err
    }
    
    // Generate NFO
    nfo, err := manager.GenerateNFO(ctx, video)
    if err != nil {
        return fmt.Errorf("failed to generate NFO: %w", err)
    }
    
    // Save NFO
    err = cache.SetNFO(ctx, videoID, nfo)
    if err != nil {
        return fmt.Errorf("failed to save NFO: %w", err)
    }
    
    return nil
}
```

### Checking Cache Validity

```go
func getVideoMetadataOrFetch(ctx context.Context, videoID string,
    cache metadata.Cache, client *api.Client) (*metadata.VideoMetadata, error) {
    
    // Check if cached data is still valid
    if cache.IsValid(ctx, videoID, "metadata") {
        return cache.GetVideoMetadata(ctx, videoID)
    }
    
    // Cache expired or not found, fetch fresh
    video, err := client.GetVideoInfo(ctx, "https://youtube.com/watch?v="+videoID)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch video info: %w", err)
    }
    
    // Save to cache
    err = cache.SetVideoMetadata(ctx, videoID, &metadata.VideoMetadata{
        ID: video.ID,
        Title: video.Title,
        // ... populate all fields
    })
    
    return video, nil
}
```

### Caching Thumbnails

```go
func cacheThumbnail(ctx context.Context, videoID, url string,
    cache metadata.Cache, httpClient *http.Client) error {
    
    // Download thumbnail
    resp, err := httpClient.Get(url)
    if err != nil {
        return fmt.Errorf("failed to download thumbnail: %w", err)
    }
    defer resp.Body.Close()
    
    data, err := io.ReadAll(resp.Body)
    if err != nil {
        return fmt.Errorf("failed to read thumbnail: %w", err)
    }
    
    // Validate (size, format)
    if len(data) > 5*1024*1024 { // 5MB max
        return fmt.Errorf("thumbnail too large: %d bytes", len(data))
    }
    
    // Save to cache
    return cache.SetThumbnail(ctx, videoID, data, "maxres")
}
```

## Emby Path Mapping

When Emby scans the YTFS mount, it looks for NFO files alongside video files:

```
/mnt/ytfs/channels/Channel_Name/
├── video_id_1.mp4          (virtual video stream)
├── video_id_1.nfo          (read from ~/.cache/ytfs/metadata/videos/...)
├── video_id_1-thumb.jpg    (symlink or copy from ~/.cache/ytfs/thumbnails/...)
└── video_id_1.srt          (symlink from ~/.cache/ytfs/subtitles/...)
```

**Strategy:** Either:
1. **Symlink** — Cache files in `~/.cache/ytfs/`, create symlinks in virtual path
2. **Copy on Access** — Detect path requests and serve from cache
3. **Virtual Paths** — Modify YTFS to return cache paths directly

## Error Handling

All cache operations should handle these errors:

```go
if err == metadata.ErrExpired {
    // Re-fetch data
} else if err == metadata.ErrNotFound {
    // First-time access, fetch fresh
} else if err == metadata.ErrInvalidFormat {
    // Corrupted cache, delete and re-fetch
} else if err != nil {
    // Other errors (disk full, permission denied, etc.)
}
```

## Performance Tips

1. **Batch operations** — Fetch metadata for 50+ videos at once
2. **Lazy loading** — Don't generate NFO until Emby requests file list
3. **Parallel downloads** — Download thumbnails concurrently (limit to 3)
4. **Incremental updates** — Only update fields that changed since last cache
5. **Memory mapping** — For large subtitle files, use mmap instead of reading fully

## Testing Checklist

- [ ] NFO XML is well-formed (can parse with xml.Unmarshal)
- [ ] All required fields present in NFO
- [ ] Dates in correct format (YYYY-MM-DD)
- [ ] Special XML characters properly escaped
- [ ] Thumbnails valid image format and size
- [ ] Subtitles valid SRT format
- [ ] Expiry tracking accurate
- [ ] Cache cleanup respects TTL and size limits
- [ ] Emby can read and display NFO files
- [ ] Video metadata correctly reflects YouTube data

## Configuration Example

For developer testing:

```json
{
  "version": "1.0",
  "cacheDir": "/tmp/ytfs_test",
  "ttl": {
    "videoMetadata": 60,
    "thumbnails": 300,
    "subtitles": 300,
    "chapters": 60
  },
  "thumbnailQuality": {
    "default": "medium",
    "maxResolution": false
  },
  "subtitles": {
    "autoGenerated": true,
    "preferredLanguages": ["en"],
    "maxLanguages": 3
  },
  "cleanupPolicy": {
    "enabled": false,
    "cleanupAge": 3600,
    "maxCacheSize": 52428800
  }
}
```

Set short TTLs (60 seconds) for testing to avoid waiting for expiry.

## Related Files

- `metadata/types.go` — Type definitions
- `metadata/cache.go` — Cache interface and manager
- `METADATA_DESIGN.md` — Full design document (specs, schemas, examples)
- `api/api.go` — yt-dlp client (data source)

## Emby Documentation References

- **NFO Format** — Emby uses standard XBMC NFO format
- **Metadata Scrapers** — Emby reads NFO files from media folders
- **Thumbnail Support** — Emby displays `*-thumb.jpg` or `poster.jpg`
- **Subtitle Support** — Emby reads `.srt`, `.sub`, `.vtt` files

## Future Enhancements

1. **Database Backend** — SQLite for faster queries on large libraries
2. **Streaming Cache** — Cache video segments as they're watched
3. **Collection Support** — Group playlists into Emby collections
4. **User Ratings** — Store user-provided ratings separately from YouTube
5. **Watch History** — Track which videos have been watched
6. **Search Index** — Full-text search of descriptions
7. **Watched Status** — Sync with Emby's watched tracking
