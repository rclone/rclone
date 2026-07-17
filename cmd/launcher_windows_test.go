//go:build windows

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasExplorerParent(t *testing.T) {
	parentProcessIDs := map[uint32]uint32{100: 50}
	assert.True(t, hasExplorerParent(100, parentProcessIDs, map[uint32]string{50: "Explorer.EXE"}))
	assert.False(t, hasExplorerParent(100, parentProcessIDs, map[uint32]string{50: "cmd.exe"}))
	assert.False(t, hasExplorerParent(200, parentProcessIDs, map[uint32]string{50: "explorer.exe"}))
}
