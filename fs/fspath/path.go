// Package fspath contains routines for fspath manipulation
package fspath

import (
	"errors"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rclone/rclone/fs/driveletter"
)

const (
	configNameRe = `[\w_ -]+`
	remoteNameRe = `^(:?` + configNameRe + `):`
)

var (
	errInvalidCharacters = errors.New("config name contains invalid characters - may only contain 0-9, A-Z ,a-z ,_ , - and space")
	errCantBeEmpty       = errors.New("can't use empty string as a path")
	errCantStartWithDash = errors.New("config name starts with -")

	// urlMatcher is a pattern to match an rclone URL
	// note that this matches invalid remoteNames
	urlMatcher = regexp.MustCompile(`^(:?[^\\/:]*):(.*)$`)

	// configNameMatcher is a pattern to match an rclone config name
	configNameMatcher = regexp.MustCompile(`^` + configNameRe + `$`)

	// remoteNameMatcher is a pattern to match an rclone remote name
	remoteNameMatcher = regexp.MustCompile(remoteNameRe + `$`)
)

// CheckConfigName returns an error if configName is invalid
func CheckConfigName(configName string) error {
	if !configNameMatcher.MatchString(configName) {
		return errInvalidCharacters
	}
	// Reject configName, if it starts with -, complicates usage. (#4261)
	if strings.HasPrefix(configName, "-") {
		return errCantStartWithDash
	}
	return nil
}

// CheckRemoteName returns an error if remoteName is invalid
func CheckRemoteName(remoteName string) error {
	if !remoteNameMatcher.MatchString(remoteName) {
		return errInvalidCharacters
	}
	return nil
}

// Parse deconstructs a remote path into configName and fsPath
//
// If the path is a local path then configName will be returned as "".
//
// So "remote:path/to/dir" will return "remote", "path/to/dir"
// and "/path/to/local" will return ("", "/path/to/local")
//
// Note that this will turn \ into / in the fsPath on Windows
//
// An error may be returned if the remote name has invalid characters
// in it or if the path is empty.
func Parse(path string) (configName, fsPath string, err error) {
	if path == "" {
		return "", "", errCantBeEmpty
	}
	parts := urlMatcher.FindStringSubmatch(path)
	configName, fsPath = "", path
	if parts != nil && !driveletter.IsDriveLetter(parts[1]) {
		configName, fsPath = parts[1], parts[2]
		err = CheckRemoteName(configName + ":")
		if err != nil {
			return configName, fsPath, errInvalidCharacters
		}
	}
	// change native directory separators to / if there are any
	fsPath = filepath.ToSlash(fsPath)
	return configName, fsPath, nil
}

// Split splits a remote into a parent and a leaf
//
// if it returns leaf as an empty string then remote is a directory
//
// if it returns parent as an empty string then that means the current directory
//
// The returned values have the property that parent + leaf == remote
// (except under Windows where \ will be translated into /)
func Split(remote string) (parent string, leaf string, err error) {
	remoteName, remotePath, err := Parse(remote)
	if err != nil {
		return "", "", err
	}
	if remoteName != "" {
		remoteName += ":"
	}
	// Construct new remote name without last segment
	parent, leaf = path.Split(remotePath)
	return remoteName + parent, leaf, nil
}

// Make filePath absolute so it can't read above the root
func makeAbsolute(filePath string) string {
	leadingSlash := strings.HasPrefix(filePath, "/")
	filePath = path.Join("/", filePath)
	if !leadingSlash && strings.HasPrefix(filePath, "/") {
		filePath = filePath[1:]
	}
	return filePath
}

// JoinRootPath joins filePath onto remote
//
// If the remote has a leading "//" this is preserved to allow Windows
// network paths to be used as remotes.
//
// If filePath is empty then remote will be returned.
//
// If the path contains \ these will be converted to / on Windows.
func JoinRootPath(remote, filePath string) string {
	remote = filepath.ToSlash(remote)
	if filePath == "" {
		return remote
	}
	filePath = filepath.ToSlash(filePath)
	filePath = makeAbsolute(filePath)
	if strings.HasPrefix(remote, "//") {
		return "/" + path.Join(remote, filePath)
	}
	remoteName, remotePath, err := Parse(remote)
	if err != nil {
		// Couldn't parse so assume it is a path
		remoteName = ""
		remotePath = remote
	}
	remotePath = path.Join(remotePath, filePath)
	if remoteName != "" {
		remoteName += ":"
		// if have remote: then normalise the remotePath
		if remotePath == "." {
			remotePath = ""
		}
	}
	return remoteName + remotePath
}
