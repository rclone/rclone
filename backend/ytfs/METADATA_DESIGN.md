# YTFS Metadata Design for Emby Compatibility

**Version:** 1.0  
**Date:** 2024-07-24  
**Status:** Design Document  

## Overview

This document defines the metadata storage and caching architecture for YTFS (YouTube filesystem) backend to provide Emby media server compatibility. The design enables Emby to discover, display, and manage YouTube videos with proper metadata (thumbnails, descriptions, chapter markers, captions, etc.) without storing full video files.

## Architecture Goals

1. **Emby Compatibility** — Generate standard NFO (XML metadata) files that Emby understands
2. **Performance** — Cache metadata and thumbnails locally to minimize API calls
3. **Scalability** — Support large channel/playlist hierarchies
4. **Maintainability** — Structured, extensible format for future additions
5. **Storage Efficiency** — Configurable TTL-based cache expiry to manage disk usage

---

## 1. Directory Structure

### 1.1 Virtual Filesystem Structure (What Emby Sees)

YTFS presents YouTube content through a virtual filesystem that maps to standard media library paths:

```
ytfs:/
├── channels/
│   ├── Channel_Name_1/
│   │   ├── video_id_1.mp4          (virtual, streamed on-demand)
│   │   ├── video_id_1.nfo          (Emby metadata XML)
│   │   ├── video_id_1-thumb.jpg    (cached thumbnail)
│   │   ├── video_id_1.srt          (cached captions)
│   │   ├── video_id_1.chapters.xml (cached chapter markers)
│   │   │
│   │   └── video_id_2.mp4
│   │       ├── video_id_2.nfo
│   │       ├── video_id_2-thumb.jpg
│   │       └── ...
│   │
│   └── Channel_Name_2/
│       ├── video_id_3.mp4
│       └── ...
│
└── playlists/
    ├── Playlist_Name_1/
    │   ├── video_id_4.mp4
    │   ├── video_id_4.nfo
    │   └── ...
    │
    └── Playlist_Name_2/
        └── ...
```

### 1.2 Local Metadata Storage Structure (Backend Cache)

Metadata is cached locally in the user's cache directory:

```
~/.cache/ytfs/
├── config.json                    (cache settings, TTL, version)
│
├── metadata/
│   ├── videos/                    (per-video metadata)
│   │   ├── dQw4w9WgXcQ/           (videoID subdirectory)
│   │   │   ├── info.json          (full video metadata from yt-dlp)
│   │   │   ├── nfo.xml            (Emby NFO file)
│   │   │   ├── description.txt    (plain text description)
│   │   │   ├── stats.json         (view count, like count, etc.)
│   │   │   └── captions/
│   │   │       ├── en.srt         (English captions)
│   │   │       ├── de.srt         (German captions)
│   │   │       └── auto.srt       (auto-generated captions)
│   │   │
│   │   └── jNgzTYEM21Q/
│   │       ├── info.json
│   │       ├── nfo.xml
│   │       └── ...
│   │
│   ├── channels/                  (per-channel metadata)
│   │   ├── UCxxxxxx/              (channelID subdirectory)
│   │   │   ├── info.json          (channel info from yt-dlp)
│   │   │   ├── description.txt    (channel description)
│   │   │   └── banner.jpg         (cached channel banner)
│   │   │
│   │   └── UCyyyyyy/
│   │       └── ...
│   │
│   └── playlists/                 (per-playlist metadata)
│       ├── PLxxxxxx/
│       │   ├── info.json
│       │   └── description.txt
│       │
│       └── PLyyyyyy/
│           └── ...
│
├── thumbnails/                    (cached images)
│   ├── videos/
│   │   ├── dQw4w9WgXcQ.jpg        (standard resolution)
│   │   ├── dQw4w9WgXcQ_hq.jpg     (high quality)
│   │   ├── dQw4w9WgXcQ_maxres.jpg (maximum resolution)
│   │   └── jNgzTYEM21Q.jpg
│   │
│   ├── channels/
│   │   ├── UCxxxxxx.jpg           (channel avatar)
│   │   └── UCyyyyyy.jpg
│   │
│   └── playlists/
│       ├── PLxxxxxx.jpg
│       └── PLyyyyyy.jpg
│
├── subtitles/                     (cached caption files)
│   ├── dQw4w9WgXcQ_en.srt         (English)
│   ├── dQw4w9WgXcQ_de.srt         (German)
│   ├── dQw4w9WgXcQ_auto.srt       (auto-generated)
│   └── ...
│
├── chapters/                      (cached chapter markers)
│   ├── dQw4w9WgXcQ.xml            (chapter metadata)
│   └── jNgzTYEM21Q.xml
│
└── expiry/                        (TTL tracking)
    ├── videos.json                (video metadata expiry times)
    ├── thumbnails.json            (thumbnail expiry times)
    ├── subtitles.json             (subtitle expiry times)
    └── channels.json              (channel metadata expiry times)
```

---

## 2. NFO (Emby Metadata XML Format)

### 2.1 NFO File Specification

The NFO file is an XML document that Emby reads to populate video metadata. One NFO file per video, named `{videoID}.nfo`.

### 2.2 Complete NFO Schema

```xml
<?xml version="1.0" encoding="utf-8"?>
<movie>
  <!-- Identification -->
  <title>Video Title Here</title>
  <originaltitle>Video Title Here</originaltitle>
  <sorttitle>video title here</sorttitle>
  <uniqueid type="youtube">dQw4w9WgXcQ</uniqueid>
  <uniqueid type="imdb">tt0000000</uniqueid>  <!-- Optional if available -->
  
  <!-- Core Metadata -->
  <plot>
    Full video description text goes here. Can span multiple lines.
    Supports HTML entities for special characters.
  </plot>
  <tagline>Short tagline or summary</tagline>
  <runtime>300</runtime>  <!-- Duration in seconds -->
  
  <!-- Dates -->
  <aired>2024-01-15</aired>  <!-- Upload date in YYYY-MM-DD format -->
  <year>2024</year>  <!-- Year extracted from upload date -->
  <dateadded>2024-07-24T10:30:00Z</dateadded>  <!-- When metadata was cached -->
  
  <!-- People -->
  <actor>
    <name>Channel Name / Uploader</name>
    <role>Uploader</role>
    <thumb>https://yt-cdn.example.com/avatar.jpg</thumb>
  </actor>
  
  <director>Channel Name</director>  <!-- Channel acts as director -->
  
  <!-- Content Classification -->
  <contentrating>TV-G</contentrating>  <!-- Default safe rating for YouTube -->
  <rating default="true">7.5</rating>  <!-- Calculated from views/likes if available -->
  
  <!-- Genres & Tags -->
  <genre>Documentary</genre>
  <genre>Educational</genre>
  
  <tag>youtube</tag>
  <tag>channel-name</tag>
  <tag>video-category</tag>  <!-- Music, Gaming, Education, etc. -->
  
  <!-- Engagement Metrics -->
  <likes>1250</likes>  <!-- Custom tag for like count -->
  <views>45000</views>  <!-- Custom tag for view count -->
  <comments>320</comments>  <!-- Custom tag for comment count -->
  
  <!-- Media & Related Links -->
  <thumb type="poster" aspect="16:9" preview="thumb://dQw4w9WgXcQ">
    https://i.ytimg.com/vi/dQw4w9WgXcQ/maxresdefault.jpg
  </thumb>
  
  <fanart>
    <thumb url="https://i.ytimg.com/vi/dQw4w9WgXcQ/maxresdefault.jpg" />
  </fanart>
  
  <url>https://www.youtube.com/watch?v=dQw4w9WgXcQ</url>
  
  <!-- Optional: Chapter Markers (if available) -->
  <chapters>
    <chapter>
      <name>Introduction</name>
      <position>0</position>  <!-- Start time in seconds -->
    </chapter>
    <chapter>
      <name>Main Content</name>
      <position>45</position>
    </chapter>
    <chapter>
      <name>Conclusion</name>
      <position>250</position>
    </chapter>
  </chapters>
  
  <!-- Optional: Credits -->
  <credits>
    <editor>Auto-generated by YTFS</editor>
  </credits>
  
  <!-- Emby-Specific Fields -->
  <musicbrainzalbumid/>  <!-- Unused for YouTube -->
  <collectionnumber/>
  <imdbid/>  <!-- YouTube ID is used instead -->
  
  <!-- Thumbnail References (alt format) -->
  <art>
    <poster url="thumb://dQw4w9WgXcQ" />
    <banner url="fanart://dQw4w9WgXcQ" />
  </art>
</movie>
```

### 2.3 NFO Generation Rules

#### Title Handling
- Use `<title>` for the video title as provided by YouTube
- Sanitize special XML characters: `&`, `<`, `>`, `"`, `'`
- Keep sorttitle lowercase for proper alphabetical ordering

#### Description Processing
- Extract first 2000 characters of video description
- Preserve line breaks as-is
- Escape XML entities: `&` → `&amp;`, `<` → `&lt;`, etc.
- Truncate with ellipsis if longer than 2000 chars

#### Date Formatting
- Upload date format from yt-dlp: `YYYYMMDD` → convert to `YYYY-MM-DD`
- If unavailable, use current date as fallback
- Year extracted from upload date (first 4 digits)

#### Rating Calculation
- Formula: `(likes / (likes + dislikes)) * 10` if both available
- Fallback: `7.0` if no engagement data
- Range: 0-10 (Emby standard)

#### Genre Mapping
- Extract from yt-dlp's `categories` field
- Common YouTube categories:
  - Music → "Music"
  - Gaming → "Gaming"
  - Education → "Documentary", "Educational"
  - Entertainment → "Comedy"
  - How-To → "How-To"
  - Default → "Web Video"

---

## 3. Cache Structure & Management

### 3.1 Cache Configuration File

**Location:** `~/.cache/ytfs/config.json`

```json
{
  "version": "1.0",
  "cacheDir": "~/.cache/ytfs",
  "ttl": {
    "videoMetadata": 604800,     // 7 days in seconds
    "thumbnails": 2592000,        // 30 days
    "subtitles": 2592000,         // 30 days
    "chapters": 604800,           // 7 days
    "channelInfo": 1209600,       // 14 days
    "playlistInfo": 604800        // 7 days
  },
  "thumbnailQuality": {
    "default": "medium",          // medium (320x180) or high (480x360)
    "maxResolution": true         // Also download maxresdefault if available
  },
  "subtitles": {
    "autoGenerated": true,        // Include auto-generated captions
    "preferredLanguages": ["en", "de", "fr"],
    "maxLanguages": 5             // Limit number of subtitle files
  },
  "chapters": {
    "enabled": true,
    "cacheFormat": "xml"          // xml or json
  },
  "cleanupPolicy": {
    "enabled": true,
    "cleanupAge": 2592000,        // 30 days: clean files older than this
    "maxCacheSize": 10737418240   // 10GB: max cache size
  }
}
```

### 3.2 Per-Video Metadata Cache

**Location:** `~/.cache/ytfs/metadata/videos/{videoID}/info.json`

Full metadata dump from yt-dlp, stored for offline reference:

```json
{
  "id": "dQw4w9WgXcQ",
  "title": "Rick Astley - Never Gonna Give You Up",
  "description": "Music video for Never Gonna Give You Up by Rick Astley...",
  "duration": 213,
  "upload_date": "20090101",
  "uploader": "Rick Astley",
  "uploader_id": "RickAstleyVEVO",
  "uploader_url": "https://www.youtube.com/@RickAstleyVEVO",
  "view_count": 1230000000,
  "like_count": 11000000,
  "comment_count": 3200000,
  "age_restricted": false,
  "categories": ["Music"],
  "tags": ["rick", "astley", "never", "gonna"],
  "thumbnail": "https://i.ytimg.com/vi/dQw4w9WgXcQ/maxresdefault.jpg",
  "ext": "mp4",
  "format_id": "18",
  "format_note": "medium",
  "acodec": "aac",
  "vcodec": "h264",
  "abr": 128,
  "vbr": 640,
  "fps": 25,
  "resolution": "640x360",
  "aspect_ratio": 1.78,
  "webpage_url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
  "_ytdl_fetch_timestamp": 1721779200
}
```

### 3.3 Thumbnail Cache Strategy

**Location:** `~/.cache/ytfs/thumbnails/videos/{videoID}.jpg`

#### Thumbnail Resolution Hierarchy

YouTube provides multiple thumbnail resolutions. YTFS downloads in this order:

1. **maxresdefault.jpg** (1280x720) — Best quality when available
2. **sddefault.jpg** (640x480) — SD fallback
3. **hqdefault.jpg** (480x360) — High quality
4. **mqdefault.jpg** (320x180) — Medium quality
5. **default.jpg** (120x90) — Fallback

**Strategy:**
- Download maxres if available (config option)
- Always include medium resolution for fast loading
- Store multiple resolutions with suffixes: `{videoID}.jpg`, `{videoID}_hq.jpg`, `{videoID}_maxres.jpg`
- Emby auto-scales, so provide best available

#### Metadata for Thumbnails

**Location:** `~/.cache/ytfs/metadata/videos/{videoID}/thumb_manifest.json`

```json
{
  "videoId": "dQw4w9WgXcQ",
  "downloaded": "2024-07-24T10:30:00Z",
  "expiresAt": "2024-08-23T10:30:00Z",
  "sizes": {
    "maxres": {
      "url": "https://i.ytimg.com/vi/dQw4w9WgXcQ/maxresdefault.jpg",
      "width": 1280,
      "height": 720,
      "cached": true,
      "filename": "dQw4w9WgXcQ_maxres.jpg"
    },
    "hq": {
      "url": "https://i.ytimg.com/vi/dQw4w9WgXcQ/hqdefault.jpg",
      "width": 480,
      "height": 360,
      "cached": true,
      "filename": "dQw4w9WgXcQ_hq.jpg"
    },
    "mq": {
      "url": "https://i.ytimg.com/vi/dQw4w9WgXcQ/mqdefault.jpg",
      "width": 320,
      "height": 180,
      "cached": true,
      "filename": "dQw4w9WgXcQ.jpg"
    }
  },
  "primaryThumb": "dQw4w9WgXcQ.jpg"
}
```

### 3.4 Subtitle/Caption Cache

**Location:** `~/.cache/ytfs/subtitles/{videoID}_{lang}.srt`

#### Format: SubRip (.srt)

```srt
1
00:00:00,500 --> 00:00:07,000
Rick Astley - Never Gonna Give You Up

2
00:00:08,000 --> 00:00:15,000
(Upbeat music plays)

3
00:00:15,500 --> 00:00:22,000
We're no strangers to love
You know the rules, and so do I

...
```

#### Subtitle Manifest

**Location:** `~/.cache/ytfs/metadata/videos/{videoID}/subtitles_manifest.json`

```json
{
  "videoId": "dQw4w9WgXcQ",
  "lastFetched": "2024-07-24T10:30:00Z",
  "expiresAt": "2024-08-23T10:30:00Z",
  "subtitles": {
    "en": {
      "name": "English",
      "auto_generated": false,
      "cached": true,
      "filename": "dQw4w9WgXcQ_en.srt"
    },
    "de": {
      "name": "Deutsch",
      "auto_generated": false,
      "cached": true,
      "filename": "dQw4w9WgXcQ_de.srt"
    },
    "en-US": {
      "name": "English (Auto-generated)",
      "auto_generated": true,
      "cached": true,
      "filename": "dQw4w9WgXcQ_en-US_auto.srt"
    }
  }
}
```

### 3.5 Chapter Markers Cache

**Location:** `~/.cache/ytfs/chapters/{videoID}.xml`

If YouTube provides chapter information (via description timestamps or explicit chapters):

```xml
<?xml version="1.0" encoding="utf-8"?>
<chapters>
  <chapter>
    <id>chapter1</id>
    <name>Introduction</name>
    <startMs>0</startMs>
    <endMs>45000</endMs>
  </chapter>
  <chapter>
    <id>chapter2</id>
    <name>Main Performance</name>
    <startMs>45000</startMs>
    <endMs>250000</endMs>
  </chapter>
  <chapter>
    <id>chapter3</id>
    <name>Outro</name>
    <startMs>250000</startMs>
    <endMs>213000</endMs>
  </chapter>
</chapters>
```

**Alternative JSON format:**

```json
{
  "videoId": "dQw4w9WgXcQ",
  "chapters": [
    {
      "id": "chapter1",
      "name": "Introduction",
      "startSeconds": 0,
      "endSeconds": 45
    },
    {
      "id": "chapter2",
      "name": "Main Performance",
      "startSeconds": 45,
      "endSeconds": 250
    }
  ]
}
```

---

## 4. TTL-Based Cache Expiry

### 4.1 Expiry Tracking

**Location:** `~/.cache/ytfs/expiry/videos.json`

Tracks when each cached item should expire:

```json
{
  "dQw4w9WgXcQ": {
    "metadata": {
      "createdAt": "2024-07-17T10:30:00Z",
      "expiresAt": "2024-07-24T10:30:00Z",
      "ttlSeconds": 604800
    },
    "thumbnail": {
      "createdAt": "2024-07-17T10:30:00Z",
      "expiresAt": "2024-08-16T10:30:00Z",
      "ttlSeconds": 2592000
    },
    "subtitles": {
      "createdAt": "2024-07-17T10:30:00Z",
      "expiresAt": "2024-08-16T10:30:00Z",
      "ttlSeconds": 2592000
    },
    "chapters": {
      "createdAt": "2024-07-17T10:30:00Z",
      "expiresAt": "2024-07-24T10:30:00Z",
      "ttlSeconds": 604800
    }
  },
  "jNgzTYEM21Q": {
    "metadata": {
      "createdAt": "2024-07-20T15:45:00Z",
      "expiresAt": "2024-07-27T15:45:00Z",
      "ttlSeconds": 604800
    }
  }
}
```

### 4.2 Cache Invalidation Strategy

**Lazy Expiry:**
- Check expiry time when reading cached file
- If expired, delete and fetch fresh data from yt-dlp
- No active background cleanup (saves resources)

**Eager Cleanup (Optional):**
- Run periodic cleanup task (daily or on-demand)
- Remove files older than configured TTL
- Respect max cache size limit (e.g., 10GB)
- Priority: oldest first, unless marked for retention

### 4.3 Expiry Management API

```go
// Check if cached metadata is valid
func (c *Cache) IsValid(ctx context.Context, videoID string, dataType string) bool {
    expiry, ok := c.expiryData[videoID][dataType]
    if !ok {
        return false
    }
    return time.Now().Before(expiry.ExpiresAt)
}

// Mark item for refresh (set expiry to now)
func (c *Cache) Invalidate(ctx context.Context, videoID string, dataType string) error {
    // Implementation forces re-fetch on next read
}

// Cleanup expired entries
func (c *Cache) Cleanup(ctx context.Context, maxAge time.Duration) error {
    // Remove files older than maxAge
    // Stop if cache size exceeds limit
}

// Get remaining TTL
func (c *Cache) TTLRemaining(videoID string, dataType string) time.Duration {
    expiry, ok := c.expiryData[videoID][dataType]
    if !ok {
        return 0
    }
    return time.Until(expiry.ExpiresAt)
}
```

---

## 5. Metadata Types & Formats

### 5.1 Metadata Categories

| Category | Type | Source | TTL | Cache |
|----------|------|--------|-----|-------|
| Video Info | JSON (yt-dlp dump) | yt-dlp | 7 days | `info.json` |
| Thumbnail | JPEG/PNG | YouTube CDN | 30 days | Multiple resolutions |
| Subtitles | SRT | yt-dlp extract | 30 days | Per-language files |
| Chapters | XML/JSON | yt-dlp/parse | 7 days | Single file |
| Channel Info | JSON (yt-dlp) | yt-dlp | 14 days | `info.json` |
| Description | Plain text | yt-dlp | 7 days | `.txt` file |
| Statistics | JSON | yt-dlp | 7 days | `stats.json` |

### 5.2 Detailed Metadata Fields Cached

#### Video Metadata Fields (from yt-dlp)

**Essential (required for Emby):**
- `id` — Unique video ID
- `title` — Video title
- `description` — Full description
- `duration` — Duration in seconds
- `upload_date` — Date in YYYYMMDD format
- `webpage_url` — YouTube URL

**Engagement (for ratings/stats):**
- `view_count` — Total views
- `like_count` — Total likes
- `comment_count` — Total comments
- `average_rating` — If available

**Media Info (for playback):**
- `ext` — Container format (usually mp4)
- `vcodec` — Video codec (h264, vp9, etc.)
- `acodec` — Audio codec (aac, opus, etc.)
- `fps` — Frames per second
- `resolution` — Video resolution

**Creator Info:**
- `uploader` — Channel name
- `uploader_id` — Channel ID
- `uploader_url` — Channel URL

**Content Classification:**
- `age_restricted` — Is 18+
- `categories` — YouTube video categories
- `tags` — Video tags

#### Channel Metadata Fields

**Essential:**
- `id` — Channel ID
- `title` — Channel name
- `description` — Channel description
- `subscriber_count` — Total subscribers (if public)
- `channel_follower_count` — Follower count

**Visual:**
- `thumbnails` — Channel avatar URLs
- `banner_url` — Channel banner URL

#### Playlist Metadata Fields

**Essential:**
- `id` — Playlist ID
- `title` — Playlist name
- `description` — Playlist description
- `playlist_count` — Number of videos
- `url` — Playlist URL

**Visual:**
- `thumbnails` — Playlist thumbnail (from first video)

---

## 6. Implementation Considerations

### 6.1 Go Package Structure

```
backend/ytfs/
├── ytfs.go                    (main filesystem)
├── metadata/
│   ├── cache.go              (cache management)
│   ├── nfo.go                (NFO generation)
│   ├── store.go              (file storage)
│   └── expiry.go             (TTL tracking)
├── api/
│   ├── api.go                (yt-dlp wrapper)
│   └── client.go             (HTTP client for thumbnails)
└── metadata_test.go          (tests)
```

### 6.2 Cache Directory Resolution

**Priority order:**
1. Config file override: `YTFS_CACHE_DIR` env var
2. XDG standard: `$XDG_CACHE_HOME/ytfs` (Linux)
3. macOS: `~/Library/Caches/ytfs`
4. Windows: `%APPDATA%\ytfs\cache`
5. Fallback: `~/.cache/ytfs`

### 6.3 Concurrent Access Handling

- Use file locks for expiry tracking to prevent corruption
- NFO generation is read-only after creation
- Thumbnail downloads should be deduplicated (prevent multiple concurrent downloads of same file)

### 6.4 Error Handling Strategy

**For missing metadata:**
- Thumbnails: Use placeholder image from Emby
- Subtitles: Don't add to NFO if not available
- Chapters: Optional, omit from NFO if unavailable
- Description: Use short fallback if empty

**For cache corruption:**
- Invalid JSON: Delete and re-fetch
- Corrupted thumbnail: Delete and re-download
- Expired data: Silently re-fetch (TTL check)

### 6.5 Performance Optimizations

1. **Batch metadata fetching** — Query 50 videos at once when possible
2. **Parallel downloads** — Download thumbnails/subtitles concurrently (limit to 3 concurrent)
3. **Incremental updates** — Only update changed fields, preserve cached data
4. **Lazy loading** — Fetch subtitles/chapters on-demand, not by default

---

## 7. Configuration Schema

### 7.1 YTFS Backend Options Extension

Add to existing YTFS options in `ytfs.go`:

```go
type Options struct {
    URL              string `config:"url"`
    UseOAuth         bool   `config:"use_oauth"`
    ManifestFile     string `config:"manifest_file"`
    
    // New metadata options
    CacheDir         string        `config:"cache_dir"`           // Override cache location
    MetadataTTL      int           `config:"metadata_ttl"`        // Default: 604800 (7 days)
    ThumbnailTTL     int           `config:"thumbnail_ttl"`       // Default: 2592000 (30 days)
    SubtitleTTL      int           `config:"subtitle_ttl"`        // Default: 2592000 (30 days)
    CacheThumbnails  bool          `config:"cache_thumbnails"`   // Default: true
    CacheSubtitles   bool          `config:"cache_subtitles"`    // Default: true
    CacheChapters    bool          `config:"cache_chapters"`     // Default: true
    MaxCacheSize     int64         `config:"max_cache_size"`     // Default: 10GB
    HighQualityThumb bool          `config:"high_quality_thumb"` // Default: true
}
```

---

## 8. Example Workflow

### 8.1 User Adds Channel to Emby

1. User configures YTFS backend: `ytfs://channel=UCxxxxxx`
2. YTFS scans channel, lists videos
3. For each video:
   - Call `yt-dlp` to get metadata → cache in `metadata/videos/{id}/info.json`
   - Generate `{id}.nfo` using schema
   - Download thumbnail → `thumbnails/videos/{id}.jpg`
   - Download subtitles → `subtitles/{id}_{lang}.srt`
   - Extract chapters if available → `chapters/{id}.xml`
   - Track expiry times in `expiry/videos.json`
4. Emby scans folder, reads all `.nfo` files
5. Emby displays videos with metadata, thumbnails, and captions

### 8.2 Subsequent Access (7 days later)

1. User accesses same channel in Emby
2. YTFS checks `expiry/videos.json`:
   - Metadata expired → Re-fetch via yt-dlp, update NFO
   - Thumbnails valid → Use cached file (30-day TTL)
   - Subtitles valid → Use cached files
3. Emby uses updated metadata
4. Cleanup task (if enabled) removes files older than 30 days

---

## 9. Future Extensions

1. **Chapter Editing** — Allow users to manually add/edit chapters via NFO
2. **Custom Ratings** — User-provided ratings stored in separate JSON
3. **Collections** — Group videos into Emby collections via playlist manifests
4. **Watched Tracking** — Store watched status per video
5. **Search Indexing** — Full-text search of descriptions and titles
6. **Live Channel Updates** — Auto-refresh channel feeds on interval
7. **Streaming Optimization** — Adaptive bitrate selection based on network

---

## 10. File Format Validation

### 10.1 NFO Validation Rules

Before writing NFO file, validate:
- [ ] XML is well-formed (can be parsed)
- [ ] Required tags present: `<title>`, `<plot>`, `<runtime>`, `<aired>`
- [ ] No duplicate unique IDs
- [ ] Dates in correct format (YYYY-MM-DD)
- [ ] XML special characters properly escaped
- [ ] File size < 1MB (safety limit)

### 10.2 Thumbnail Validation

Before caching thumbnail:
- [ ] File is valid JPEG or PNG
- [ ] Dimensions >= 120x90 (minimum)
- [ ] File size < 5MB
- [ ] HTTP 200 response

### 10.3 Subtitle Validation

Before caching subtitles:
- [ ] Valid SubRip format (SRT)
- [ ] Timecodes in ascending order
- [ ] No duplicate entries
- [ ] File size < 10MB

---

## 11. Emby Integration Points

### 11.1 Supported Emby Features

- **Metadata Display** — NFO fields render correctly in Emby UI
- **Thumbnails** — Multiple resolutions for responsive display
- **Captions** — In-player subtitle selection
- **Chapters** — Chapter markers in progress bar
- **Ratings** — User ratings and like counts displayed
- **Search** — Emby search indexes title and description

### 11.2 NFO Best Practices for Emby

1. Always include `<uniqueid>` with type="youtube"
2. Use `<runtime>` in seconds (Emby requires this)
3. Use `<aired>` in YYYY-MM-DD format
4. Thumbnail URLs should be direct HTTPS links
5. Keep plot description under 3000 characters
6. Use standard genres (Emby has predefined set)
7. Runtime should never be 0 or negative

---

## 12. Security & Privacy

### 12.1 Cache Directory Permissions

- Cache dir: `700` (drwx------)
- NFO files: `600` (-rw-------)
- Thumbnails: `600` (-rw-------)
- Config: `600` (-rw-------)

### 12.2 Data Retention

- No persistent user authentication stored
- yt-dlp cookies optional (can be ephemeral)
- Metadata only from public YouTube data
- Cache can be safely deleted anytime (regenerates on access)

### 12.3 Rate Limiting

- Respect YouTube's rate limits via yt-dlp
- Batch requests where possible
- Implement exponential backoff for 429 responses
- Cache TTL reduces API pressure

---

## Appendix A: Configuration Examples

### A.1 Minimal Configuration

```json
{
  "version": "1.0",
  "cacheDir": "~/.cache/ytfs"
}
```

Uses all defaults (7-day metadata TTL, cache thumbnails, enable subtitles).

### A.2 High-Performance Configuration (Large Library)

```json
{
  "version": "1.0",
  "cacheDir": "/mnt/cache/ytfs",
  "ttl": {
    "videoMetadata": 1209600,    // 14 days
    "thumbnails": 5184000,       // 60 days
    "subtitles": 5184000,        // 60 days
    "chapters": 1209600          // 14 days
  },
  "thumbnailQuality": {
    "default": "high",
    "maxResolution": true
  },
  "subtitles": {
    "autoGenerated": false,      // Skip auto-generated to save space
    "preferredLanguages": ["en"],
    "maxLanguages": 3
  },
  "cleanupPolicy": {
    "enabled": true,
    "cleanupAge": 5184000,       // 60 days
    "maxCacheSize": 107374182400 // 100GB
  }
}
```

### A.3 Minimal Cache Configuration (Storage-Constrained)

```json
{
  "version": "1.0",
  "cacheDir": "~/.cache/ytfs",
  "ttl": {
    "videoMetadata": 259200,     // 3 days
    "thumbnails": 259200,        // 3 days
    "subtitles": 86400           // 1 day
  },
  "thumbnailQuality": {
    "default": "medium",
    "maxResolution": false
  },
  "subtitles": {
    "autoGenerated": false,
    "preferredLanguages": ["en"],
    "maxLanguages": 1
  },
  "cleanupPolicy": {
    "enabled": true,
    "cleanupAge": 259200,        // 3 days
    "maxCacheSize": 1073741824   // 1GB
  }
}
```

---

## Appendix B: yt-dlp Metadata Extraction Fields

Complete list of fields used from yt-dlp output:

```
id                          # Video ID
title                       # Video title
description                 # Full description
duration                    # Duration in seconds
upload_date                 # Date YYYYMMDD
uploader                    # Channel name
uploader_id                 # Channel ID
uploader_url                # Channel URL
view_count                  # View count
like_count                  # Like count
comment_count               # Comment count
age_restricted              # Boolean 18+
categories                  # List of categories
tags                        # List of tags
thumbnail                   # Thumbnail URL
subtitles                   # Dict of subtitle tracks by language
automatic_captions          # Dict of auto-generated captions
chapters                    # List of chapter info (if available)
ext                         # File extension
vcodec                      # Video codec
acodec                      # Audio codec
fps                         # Frames per second
resolution                  # Resolution WIDTHxHEIGHT
webpage_url                 # YouTube URL
```

---

## Document Revisions

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2024-07-24 | Initial design document |

