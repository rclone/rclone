package buildinfo

import (
	"sort"
	"strings"
)

// Tags contains slice of build tags.
// The `cmount` tag is added by cmd/cmount/mount.go only if build is static.
// The `noselfupdate` tag is added by cmd/selfupdate/noselfupdate.go
// Other tags including `cgo` are detected in this package.
var Tags []string

// GetLinkingAndTags tells how the rclone executable was linked
// and returns space separated build tags or the string "none".
func GetLinkingAndTags() (linking, tagString string) {
	linking = "static"
	tagList := []string{}
	for _, tag := range Tags {
		if tag == "cgo" {
			linking = "dynamic"
		} else {
			tagList = append(tagList, tag)
		}
	}
	if len(tagList) > 0 {
		sort.Strings(tagList)
		tagString = strings.Join(tagList, " ")
	} else {
		tagString = "none"
	}
	return
}
