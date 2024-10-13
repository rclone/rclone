package gitannex

import (
	"fmt"
	"strings"

	"github.com/rclone/rclone/fs/fspath"
)

type layoutMode string

// All layout modes from git-annex-remote-rclone are supported.
const (
	layoutModeLower       layoutMode = "lower"
	layoutModeDirectory   layoutMode = "directory"
	layoutModeNodir       layoutMode = "nodir"
	layoutModeMixed       layoutMode = "mixed"
	layoutModeFrankencase layoutMode = "frankencase"
	layoutModeUnknown     layoutMode = ""
)

func allLayoutModes() []layoutMode {
	return []layoutMode{
		layoutModeLower,
		layoutModeDirectory,
		layoutModeNodir,
		layoutModeMixed,
		layoutModeFrankencase,
	}
}

func parseLayoutMode(mode string) layoutMode {
	for _, knownMode := range allLayoutModes() {
		if mode == string(knownMode) {
			return knownMode
		}
	}
	return layoutModeUnknown
}

type queryDirhashFunc func(msg string) (string, error)

func buildFsString(queryDirhash queryDirhashFunc, mode layoutMode, key, remoteName, prefix string) (string, error) {
	remoteName = strings.TrimSuffix(remoteName, ":") + ":"
	remoteString := fspath.JoinRootPath(remoteName, prefix)

	if mode == layoutModeNodir {
		return remoteString, nil
	}

	var dirhash string
	var err error
	switch mode {
	case layoutModeLower, layoutModeDirectory:
		dirhash, err = queryDirhash("DIRHASH-LOWER " + key)
	case layoutModeMixed, layoutModeFrankencase:
		dirhash, err = queryDirhash("DIRHASH " + key)
	default:
		panic("unreachable")
	}
	if err != nil {
		return "", fmt.Errorf("buildFsString failed to query dirhash: %w", err)
	}

	switch mode {
	case layoutModeLower:
		return fmt.Sprintf("%s/%s", remoteString, dirhash), nil
	case layoutModeDirectory:
		return fmt.Sprintf("%s/%s%s", remoteString, dirhash, key), nil
	case layoutModeMixed:
		return fmt.Sprintf("%s/%s", remoteString, dirhash), nil
	case layoutModeFrankencase:
		return fmt.Sprintf("%s/%s", remoteString, strings.ToLower(dirhash)), nil
	default:
		panic("unreachable")
	}
}
