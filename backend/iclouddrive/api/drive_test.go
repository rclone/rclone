package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestZoneFromDriveID(t *testing.T) {
	for _, test := range []struct {
		name string
		id   string
		want string
	}{
		{"app container", "FOLDER::iCloud.md.obsidian::documents#o2v", "iCloud.md.obsidian"},
		{"another app container", "FOLDER::com.apple.Pages::documents#7qt", "com.apple.Pages"},
		{"ordinary folder", "FOLDER::com.apple.CloudDocs::B847FE2D", "com.apple.CloudDocs"},
		{"file in a container", "FILE::iCloud.md.obsidian::abc123", "iCloud.md.obsidian"},
		{"no zone", "root", defaultZone},
		{"empty", "", defaultZone},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, ZoneFromDriveID(test.id))
		})
	}
}

func TestZoneFromDriveIDRoundTrip(t *testing.T) {
	id := ConstructDriveID("abc123", "iCloud.md.obsidian", "FILE")
	assert.Equal(t, "FILE::iCloud.md.obsidian::abc123", id)
	assert.Equal(t, "iCloud.md.obsidian", ZoneFromDriveID(id))
}
