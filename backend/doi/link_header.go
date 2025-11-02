package doi

import (
	"regexp"
	"strings"
)

var linkRegex = regexp.MustCompile(`^<(.+)>$`)
var valueRegex = regexp.MustCompile(`^"(.+)"$`)

// headerLink represents a link as presented in HTTP headers
// MDN Reference: https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Link
type headerLink struct {
	Href   string
	Rel    string
	Type   string
	Extras map[string]string
}

func parseLinkHeader(header string) (links []headerLink) {
	for link := range strings.SplitSeq(header, ",") {
		link = strings.TrimSpace(link)
		parsed := parseLink(link)
		if parsed != nil {
			links = append(links, *parsed)
		}
	}
	return links
}

func parseLink(link string) (parsedLink *headerLink) {
	var parts []string
	for part := range strings.SplitSeq(link, ";") {
		parts = append(parts, strings.TrimSpace(part))
	}

	match := linkRegex.FindStringSubmatch(parts[0])
	if match == nil {
		return nil
	}

	result := &headerLink{
		Href:   match[1],
		Extras: map[string]string{},
	}

	for _, keyValue := range parts[1:] {
		parsed := parseKeyValue(keyValue)
		if parsed != nil {
			key, value := parsed[0], parsed[1]
			switch strings.ToLower(key) {
			case "rel":
				result.Rel = value
			case "type":
				result.Type = value
			default:
				result.Extras[key] = value
			}
		}
	}
	return result
}

func parseKeyValue(keyValue string) []string {
	parts := strings.SplitN(keyValue, "=", 2)
	if parts[0] == "" || len(parts) < 2 {
		return nil
	}
	match := valueRegex.FindStringSubmatch(parts[1])
	if match != nil {
		parts[1] = match[1]
		return parts
	}
	return parts
}
