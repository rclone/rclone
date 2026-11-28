package s3

import (
	"net/http"
	"slices"
	"strings"

	"github.com/rclone/gofakes3"
)

type writeConditionsKey struct{}

// writeConditions contains the validated destination ETags of a request.
type writeConditions struct {
	match, none []string
	err         error
}

// parseWriteConditions parses If-Match and If-None-Match. It returns nil
// when neither header is present.
func parseWriteConditions(h http.Header) *writeConditions {
	var c writeConditions
	for _, field := range []struct {
		name string
		tags *[]string
	}{{"If-Match", &c.match}, {"If-None-Match", &c.none}} {
		if values := h.Values(field.name); len(values) > 0 {
			tags := splitEntityTags(strings.Join(values, ","))
			if len(tags) == 0 {
				return &writeConditions{err: gofakes3.ErrInvalidArgument}
			}
			*field.tags = tags
		}
	}
	if c.match == nil && c.none == nil {
		return nil
	}
	return &c
}

// needsETag reports whether validation requires hashing an existing object.
func (c *writeConditions) needsETag() bool {
	return len(c.match) > 0 && c.match[0] != "*" || len(c.none) > 0 && c.none[0] != "*"
}

// check validates the conditions against the destination. etag is the quoted
// strong ETag when needsETag is true and the destination exists.
func (c *writeConditions) check(exists bool, etag string) error {
	if len(c.match) > 0 {
		if !exists {
			return gofakes3.ErrNoSuchKey
		}
		if c.match[0] != "*" && !slices.Contains(c.match, etag) {
			return gofakes3.ErrPreconditionFailed
		}
	}
	if exists {
		for _, tag := range c.none {
			if tag == "*" || strings.TrimPrefix(tag, "W/") == etag {
				return gofakes3.ErrPreconditionFailed
			}
		}
	}
	return nil
}

// splitEntityTags accepts quoted lists and single unquoted tags used by S3 clients.
func splitEntityTags(value string) []string {
	value = strings.Trim(value, " \t")
	if value == "*" {
		return []string{"*"}
	}
	if value != "" && value[0] != '"' && !strings.HasPrefix(value, "W/") {
		for i := range len(value) {
			c := value[i]
			if c <= 0x20 || c >= 0x7f || c == '"' || c == ',' || c == '*' {
				return nil
			}
		}
		return []string{`"` + value + `"`}
	}
	var tags []string
	for value != "" {
		end := 0
		if strings.HasPrefix(value, "W/") {
			end = 2
		}
		if len(value) <= end || value[end] != '"' {
			return nil
		}
		end++
		for end < len(value) && value[end] != '"' {
			if value[end] < 0x21 || value[end] == 0x7f {
				return nil
			}
			end++
		}
		if end == len(value) {
			return nil
		}
		end++
		tags = append(tags, value[:end])
		value = strings.TrimLeft(value[end:], " \t")
		if value == "" {
			return tags
		}
		if value[0] != ',' {
			return nil
		}
		value = strings.TrimLeft(value[1:], " \t")
		if value == "" {
			return nil
		}
	}
	return tags
}
