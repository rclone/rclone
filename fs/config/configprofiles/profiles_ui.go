// Package profiles handles presets for config
package profiles

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rclone/rclone/fs/config"
)

// ShowProfiles prints an overview of every profile in the config file.
func ShowProfiles() {
	profiles := GetProfileList()
	fmt.Printf("%-20s %s\n", "Name", "Type")
	fmt.Printf("%-20s %s\n", "====", "====")
	for _, profile := range profiles {
		fmt.Printf("%-20s %s\n", profile, "profile")
	}
}

// GetProfileList returns the user-facing names of all profile
// sections (i.e. without the "profile:" prefix), sorted.
func GetProfileList() []string {
	sections := config.LoadedData().GetSectionList()
	profiles := []string{}
	for _, s := range sections {
		if !config.IsProfileSection(s) {
			continue
		}
		profiles = append(profiles, strings.TrimPrefix(s, config.ProfileSectionPrefix))
	}
	sort.Strings(profiles)
	return profiles
}
