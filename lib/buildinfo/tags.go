package buildinfo

import (
	"sort"
	"strings"
)

// Tags contains slice of build tags
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
