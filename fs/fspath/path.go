// Package fspath contains routines for fspath manipulation
package fspath

import (
	"errors"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/driveletter"
)

const (
	configNameRe = `[\w_. -]+`
	remoteNameRe = `^(:?` + configNameRe + `)`
)

var (
	errInvalidCharacters = errors.New("config name contains invalid characters - may only contain `0-9`, `A-Z`, `a-z`, `_`, `-`, `.` and space")
	errCantBeEmpty       = errors.New("can't use empty string as a path")
	errCantStartWithDash = errors.New("config name starts with `-`")
	errBadConfigParam    = errors.New("config parameters may only contain `0-9`, `A-Z`, `a-z` and `_`")
	errEmptyConfigParam  = errors.New("config parameters can't be empty")
	errConfigNameEmpty   = errors.New("config name can't be empty")
	errConfigName        = errors.New("config name needs a trailing `:`")
	errParam             = errors.New("config parameter must end with `,` or `:`")
	errValue             = errors.New("unquoted config value must end with `,` or `:`")
	errQuotedValue       = errors.New("unterminated quoted config value")
	errAfterQuote        = errors.New("expecting `:` or `,` or another quote after a quote")
	errSyntax            = errors.New("syntax error in config string")

	// configNameMatcher is a pattern to match an rclone config name
	configNameMatcher = regexp.MustCompile(`^` + configNameRe + `$`)

	// remoteNameMatcher is a pattern to match an rclone remote name at the start of a config
	remoteNameMatcher = regexp.MustCompile(`^` + remoteNameRe + `(:$|,)`)
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

// checkRemoteName returns an error if remoteName is invalid
func checkRemoteName(remoteName string) error {
	if remoteName == ":" || remoteName == "::" {
		return errConfigNameEmpty
	}
	if !remoteNameMatcher.MatchString(remoteName) {
		return errInvalidCharacters
	}
	return nil
}

// Return true if c is a valid character for a config parameter
func isConfigParam(c rune) bool {
	return ((c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '_')
}

// Parsed is returned from Parse with the results of the connection string decomposition
//
// If Name is "" then it is a local path in Path
//
// Note that ConfigString + ":" + Path is equal to the input of Parse except that Path may have had
// \ converted to /
type Parsed struct {
	Name         string           // Just the name of the config: "remote" or ":backend"
	ConfigString string           // The whole config string: "remote:" or ":backend,value=6:"
	Path         string           // The file system path, may be empty
	Config       configmap.Simple // key/value config parsed out of ConfigString may be nil
}

// Parse deconstructs a path into a Parsed structure
//
// If the path is a local path then parsed.Name will be returned as "".
//
// So "remote:path/to/dir" will return Parsed{Name:"remote", Path:"path/to/dir"},
// and "/path/to/local" will return Parsed{Name:"", Path:"/path/to/local"}
//
// Note that this will turn \ into / in the fsPath on Windows
//
// An error may be returned if the remote name has invalid characters or the
// parameters are invalid or the path is empty.
func Parse(path string) (parsed Parsed, err error) {
	parsed.Path = filepath.ToSlash(path)
	if path == "" {
		return parsed, errCantBeEmpty
	}
	// If path has no `:` in, it must be a local path
	if !strings.ContainsRune(path, ':') {
		return parsed, nil
	}
	// States for parser
	const (
		stateConfigName = uint8(iota)
		stateParam
		stateValue
		stateQuotedValue
		stateAfterQuote
		stateDone
	)
	var (
		state   = stateConfigName // current state of parser
		i       int               // position in path
		prev    int               // previous position in path
		c       rune              // current rune under consideration
		quote   rune              // kind of quote to end this quoted string
		param   string            // current parameter value
		doubled bool              // set if had doubled quotes
	)
loop:
	for i, c = range path {
		// Example Parse
		// remote,param=value,param2="qvalue":/path/to/file
		switch state {
		// Parses "remote,"
		case stateConfigName:
			if i == 0 && c == ':' {
				continue
			} else if c == '/' || c == '\\' {
				// `:` or `,` not before a path separator must be a local path,
				// except if the path started with `:` in which case it was intended
				// to be an on the fly remote so return an error.
				if path[0] == ':' {
					return parsed, errInvalidCharacters
				}
				return parsed, nil
			} else if c == ':' || c == ',' {
				parsed.Name = path[:i]
				err := checkRemoteName(parsed.Name + ":")
				if err != nil {
					return parsed, err
				}
				prev = i + 1
				if c == ':' {
					// If we parsed a drive letter, must be a local path
					if driveletter.IsDriveLetter(parsed.Name) {
						parsed.Name = ""
						return parsed, nil
					}
					state = stateDone
					break loop
				}
				state = stateParam
				parsed.Config = make(configmap.Simple)
			}
		// Parses param= and param2=
		case stateParam:
			if c == ':' || c == ',' || c == '=' {
				param = path[prev:i]
				if len(param) == 0 {
					return parsed, errEmptyConfigParam
				}
				prev = i + 1
				if c == '=' {
					state = stateValue
					break
				}
				parsed.Config[param] = "true"
				if c == ':' {
					state = stateDone
					break loop
				}
				state = stateParam
			} else if !isConfigParam(c) {
				return parsed, errBadConfigParam
			}
		// Parses value
		case stateValue:
			if c == '\'' || c == '"' {
				if i == prev {
					quote = c
					state = stateQuotedValue
					prev = i + 1
					doubled = false
					break
				}
			} else if c == ':' || c == ',' {
				value := path[prev:i]
				prev = i + 1
				parsed.Config[param] = value
				if c == ':' {
					state = stateDone
					break loop
				}
				state = stateParam
			}
		// Parses "qvalue"
		case stateQuotedValue:
			if c == quote {
				state = stateAfterQuote
			}
		// Parses : or , or quote after "qvalue"
		case stateAfterQuote:
			if c == ':' || c == ',' {
				value := path[prev : i-1]
				// replace any doubled quotes if there were any
				if doubled {
					value = strings.ReplaceAll(value, string(quote)+string(quote), string(quote))
				}
				prev = i + 1
				parsed.Config[param] = value
				if c == ':' {
					state = stateDone
					break loop
				} else {
					state = stateParam
				}
			} else if c == quote {
				// Here is a doubled quote to indicate a literal quote
				state = stateQuotedValue
				doubled = true
			} else {
				return parsed, errAfterQuote
			}
		}

	}

	// Depending on which state we were in when we fell off the
	// end of the state machine we can return a sensible error.
	switch state {
	default:
		return parsed, errSyntax
	case stateConfigName:
		return parsed, errConfigName
	case stateParam:
		return parsed, errParam
	case stateValue:
		return parsed, errValue
	case stateQuotedValue:
		return parsed, errQuotedValue
	case stateAfterQuote:
		return parsed, errAfterQuote
	case stateDone:
		break
	}

	parsed.ConfigString = path[:i]
	parsed.Path = path[i+1:]

	// change native directory separators to / if there are any
	parsed.Path = filepath.ToSlash(parsed.Path)
	return parsed, nil
}

// SplitFs splits a remote a remoteName and an remotePath.
//
// SplitFs("remote:path/to/file") -> ("remote:", "path/to/file")
// SplitFs("/to/file") -> ("", "/to/file")
//
// If it returns remoteName as "" then remotePath is a local path
//
// The returned values have the property that remoteName + remotePath ==
// remote (except under Windows where \ will be translated into /)
func SplitFs(remote string) (remoteName string, remotePath string, err error) {
	parsed, err := Parse(remote)
	if err != nil {
		return "", "", err
	}
	remoteName, remotePath = parsed.ConfigString, parsed.Path
	if remoteName != "" {
		remoteName += ":"
	}
	return remoteName, remotePath, nil
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
	remoteName, remotePath, err := SplitFs(remote)
	if err != nil {
		return "", "", err
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
	parsed, err := Parse(remote)
	remoteName, remotePath := parsed.ConfigString, parsed.Path
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
