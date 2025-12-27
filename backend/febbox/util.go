// util.go - Utility functions for Febbox backend
package feb_box

import (
    "crypto/md5"
    "encoding/hex"
    "fmt"
    "net/http"
    "net/url"
    "path/filepath"
    "regexp"
    "strings"
    "time"
)

// ParseTime parses Febbox timestamp strings
func ParseTime(timeStr string) (time.Time, error) {
    formats := []string{
        time.RFC3339,
        "2006-01-02 15:04:05",
        "2006-01-02T15:04:05Z",
        "2006-01-02T15:04:05.000Z",
        time.RFC1123,
        "Jan 2,2006 15:04", // Febbox format
        "2006-01-02",
    }

    for _, format := range formats {
        t, err := time.Parse(format, timeStr)
        if err == nil {
            return t, nil
        }
    }

    return time.Time{}, fmt.Errorf("unable to parse time string: %s", timeStr)
}

// SanitizePath cleans up file paths
func SanitizePath(path string) string {
    path = strings.Trim(path, "/")
    re := regexp.MustCompile(`/{2,}`)
    path = re.ReplaceAllString(path, "/")
    path = filepath.Clean(path)
    return path
}

// In util.go, add this function for proper URL encoding
func EncodeFidsJSON(fid int64) string {
    jsonStr := fmt.Sprintf(`["%d"]`, fid)
    return url.QueryEscape(jsonStr)
}

// ExtractShareKey extracts share key from Febbox URL
func ExtractShareKey(febboxURL string) (string, error) {
    parsedURL, err := url.Parse(febboxURL)
    if err != nil {
        return "", err
    }

    if strings.Contains(parsedURL.Host, "febbox.com") {
        pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
        if len(pathParts) >= 2 && pathParts[0] == "share" {
            return pathParts[1], nil
        }

        query := parsedURL.Query()
        if shareKey := query.Get("share_key"); shareKey != "" {
            return shareKey, nil
        }
    }

    return "", fmt.Errorf("no share key found in URL")
}

// ParseCookieString parses cookie string into []*http.Cookie
func ParseCookieString(cookieStr string) []*http.Cookie {
    var cookies []*http.Cookie
    parts := strings.Split(cookieStr, ";")
    
    for _, part := range parts {
        part = strings.TrimSpace(part)
        if part == "" {
            continue
        }
        
        keyValue := strings.SplitN(part, "=", 2)
        if len(keyValue) != 2 {
            continue
        }
        
        name := strings.TrimSpace(keyValue[0])
        value := strings.TrimSpace(keyValue[1])
        
        if name == "" || value == "" {
            continue
        }
        
        cookies = append(cookies, &http.Cookie{
            Name:  name,
            Value: value,
        })
    }
    
    return cookies
}

// GenerateFileID creates a unique file ID
func GenerateFileID(filename string, parentID string) string {
    data := fmt.Sprintf("%s|%s|%d", filename, parentID, time.Now().UnixNano())
    hash := md5.Sum([]byte(data))
    return hex.EncodeToString(hash[:])
}

// IsValidCookie checks if cookie format looks valid
func IsValidCookie(cookie string) bool {
    if len(cookie) < 10 {
        return false
    }

    validChars := regexp.MustCompile(`^[A-Za-z0-9\-_=\.]+$`)
    return validChars.MatchString(cookie)
}

// BuildURL constructs API URLs with parameters
func BuildURL(baseURL string, endpoint string, params map[string]string) string {
    if len(params) == 0 {
        return baseURL + endpoint
    }

    var queryParts []string
    for key, value := range params {
        encodedValue := url.QueryEscape(value)
        queryParts = append(queryParts, fmt.Sprintf("%s=%s", key, encodedValue))
    }

    return baseURL + endpoint + "?" + strings.Join(queryParts, "&")
}

// ParseFileSize parses human-readable file size to bytes
func ParseFileSize(sizeStr string) (int64, error) {
    var size int64
    var unit string

    n, err := fmt.Sscanf(sizeStr, "%d%s", &size, &unit)
    if err != nil || n != 2 {
        n, err := fmt.Sscanf(sizeStr, "%d", &size)
        if err != nil || n != 1 {
            return 0, fmt.Errorf("invalid size format: %s", sizeStr)
        }
        return size, nil
    }

    unit = strings.ToUpper(unit)
    switch unit {
    case "B":
        return size, nil
    case "KB", "K":
        return size * 1024, nil
    case "MB", "M":
        return size * 1024 * 1024, nil
    case "GB", "G":
        return size * 1024 * 1024 * 1024, nil
    case "TB", "T":
        return size * 1024 * 1024 * 1024 * 1024, nil
    default:
        return 0, fmt.Errorf("unknown unit: %s", unit)
    }
}

// FormatFileSize formats bytes to human-readable string
func FormatFileSize(bytes int64) string {
    const unit = 1024
    if bytes < unit {
        return fmt.Sprintf("%d B", bytes)
    }

    div, exp := int64(unit), 0
    for n := bytes / unit; n >= unit; n /= unit {
        div *= unit
        exp++
    }

    units := []string{"KB", "MB", "GB", "TB"}
    return fmt.Sprintf("%.2f %s", float64(bytes)/float64(div), units[exp])
}

// SplitPath splits a path into directory and filename components
func SplitPath(fullPath string) (dir, file string) {
    fullPath = strings.Trim(fullPath, "/")
    if fullPath == "" {
        return "", ""
    }

    lastSlash := strings.LastIndex(fullPath, "/")
    if lastSlash == -1 {
        return "", fullPath
    }

    return fullPath[:lastSlash], fullPath[lastSlash+1:]
}

// JoinPath joins path components
func JoinPath(parts ...string) string {
    var cleanedParts []string
    for _, part := range parts {
        part = strings.Trim(part, "/")
        if part != "" {
            cleanedParts = append(cleanedParts, part)
        }
    }
    return strings.Join(cleanedParts, "/")
}

// IsVideoFile checks if filename is a video file
func IsVideoFile(filename string) bool {
    videoExts := map[string]bool{
        ".mp4": true, ".avi": true, ".mkv": true, ".mov": true,
        ".wmv": true, ".flv": true, ".webm": true, ".m4v": true,
        ".mpg": true, ".mpeg": true, ".3gp": true, ".ts": true,
    }

    ext := strings.ToLower(filepath.Ext(filename))
    return videoExts[ext]
}

// IsSubtitleFile checks if filename is a subtitle file
func IsSubtitleFile(filename string) bool {
    subExts := map[string]bool{
        ".srt": true, ".sub": true, ".ass": true, ".ssa": true,
        ".vtt": true, ".sbv": true,
    }

    ext := strings.ToLower(filepath.Ext(filename))
    return subExts[ext]
}