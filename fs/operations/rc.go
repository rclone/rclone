package operations

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/lib/diskusage"
)

func init() {
	rc.Add(rc.Call{
		Path:         "operations/list",
		AuthRequired: true,
		Fn:           rcList,
		Title:        "List the given remote and path in JSON format",
		Help: `This takes the following parameters:

- fs - a remote name string e.g. "drive:"
- remote - a path within that remote e.g. "dir"
- opt - a dictionary of options to control the listing (optional)
    - recurse - If set recurse directories
    - noModTime - If set return modification time
    - showEncrypted -  If set show decrypted names
    - showOrigIDs - If set show the IDs for each item if known
    - showHash - If set return a dictionary of hashes
    - noMimeType - If set don't show mime types
    - dirsOnly - If set only show directories
    - filesOnly - If set only show files
    - metadata - If set return metadata of objects also
    - hashTypes - array of strings of hash types to show if showHash set

Returns:

- list
    - This is an array of objects as described in the lsjson command

See the [lsjson](/commands/rclone_lsjson/) command for more information on the above and examples.
`,
	})
}

// List the directory
func rcList(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	f, remote, err := rc.GetFsAndRemote(ctx, in)
	if err != nil {
		return nil, err
	}
	var opt ListJSONOpt
	err = in.GetStruct("opt", &opt)
	if rc.NotErrParamNotFound(err) {
		return nil, err
	}
	var list = []*ListJSONItem{}
	err = ListJSON(ctx, f, remote, &opt, func(item *ListJSONItem) error {
		list = append(list, item)
		return nil
	})
	if err != nil {
		return nil, err
	}
	out = make(rc.Params)
	out["list"] = list
	return out, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "operations/stat",
		AuthRequired: true,
		Fn:           rcStat,
		Title:        "Give information about the supplied file or directory",
		Help: `This takes the following parameters

- fs - a remote name string eg "drive:"
- remote - a path within that remote eg "dir"
- opt - a dictionary of options to control the listing (optional)
    - see operations/list for the options

The result is

- item - an object as described in the lsjson command. Will be null if not found.

Note that if you are only interested in files then it is much more
efficient to set the filesOnly flag in the options.

See the [lsjson](/commands/rclone_lsjson/) command for more information on the above and examples.
`,
	})
}

// List the directory
func rcStat(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	f, remote, err := rc.GetFsAndRemote(ctx, in)
	if err != nil {
		return nil, err
	}
	var opt ListJSONOpt
	err = in.GetStruct("opt", &opt)
	if rc.NotErrParamNotFound(err) {
		return nil, err
	}
	item, err := StatJSON(ctx, f, remote, &opt)
	if err != nil {
		return nil, err
	}
	out = make(rc.Params)
	out["item"] = item
	return out, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "operations/about",
		AuthRequired: true,
		Fn:           rcAbout,
		Title:        "Return the space used on the remote",
		Help: `This takes the following parameters:

- fs - a remote name string e.g. "drive:"

The result is as returned from rclone about --json

See the [about](/commands/rclone_about/) command for more information on the above.
`,
	})
}

// About the remote
func rcAbout(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	f, err := rc.GetFs(ctx, in)
	if err != nil {
		return nil, err
	}
	doAbout := f.Features().About
	if doAbout == nil {
		return nil, fmt.Errorf("%v doesn't support about", f)
	}
	u, err := doAbout(ctx)
	if err != nil {
		return nil, fmt.Errorf("about call failed: %w", err)
	}
	err = rc.Reshape(&out, u)
	if err != nil {
		return nil, fmt.Errorf("about Reshape failed: %w", err)
	}
	return out, nil
}

func init() {
	for _, copy := range []bool{false, true} {
		copy := copy
		name := "Move"
		if copy {
			name = "Copy"
		}
		rc.Add(rc.Call{
			Path:         "operations/" + strings.ToLower(name) + "file",
			AuthRequired: true,
			Fn: func(ctx context.Context, in rc.Params) (rc.Params, error) {
				return rcMoveOrCopyFile(ctx, in, copy)
			},
			Title: name + " a file from source remote to destination remote",
			Help: `This takes the following parameters:

- srcFs - a remote name string e.g. "drive:" for the source, "/" for local filesystem
- srcRemote - a path within that remote e.g. "file.txt" for the source
- dstFs - a remote name string e.g. "drive2:" for the destination, "/" for local filesystem
- dstRemote - a path within that remote e.g. "file2.txt" for the destination
`,
		})
	}
}

// Copy a file
func rcMoveOrCopyFile(ctx context.Context, in rc.Params, cp bool) (out rc.Params, err error) {
	srcFs, srcRemote, err := rc.GetFsAndRemoteNamed(ctx, in, "srcFs", "srcRemote")
	if err != nil {
		return nil, err
	}
	dstFs, dstRemote, err := rc.GetFsAndRemoteNamed(ctx, in, "dstFs", "dstRemote")
	if err != nil {
		return nil, err
	}
	return nil, moveOrCopyFile(ctx, dstFs, srcFs, dstRemote, srcRemote, cp)
}

func init() {
	for _, op := range []struct {
		name         string
		title        string
		help         string
		noRemote     bool
		needsRequest bool
	}{
		{name: "mkdir", title: "Make a destination directory or container"},
		{name: "rmdir", title: "Remove an empty directory or container"},
		{name: "purge", title: "Remove a directory or container and all of its contents"},
		{name: "rmdirs", title: "Remove all the empty directories in the path", help: "- leaveRoot - boolean, set to true not to delete the root\n"},
		{name: "delete", title: "Remove files in the path", noRemote: true},
		{name: "deletefile", title: "Remove the single file pointed to"},
		{name: "copyurl", title: "Copy the URL to the object", help: "- url - string, URL to read from\n - autoFilename - boolean, set to true to retrieve destination file name from url\n"},
		{name: "uploadfile", title: "Upload file using multiform/form-data", help: "- each part in body represents a file to be uploaded\n", needsRequest: true},
		{name: "cleanup", title: "Remove trashed files in the remote or path", noRemote: true},
		{name: "settier", title: "Changes storage tier or class on all files in the path", noRemote: true},
		{name: "settierfile", title: "Changes storage tier or class on the single file pointed to"},
	} {
		op := op
		remote := "- remote - a path within that remote e.g. \"dir\"\n"
		if op.noRemote {
			remote = ""
		}
		rc.Add(rc.Call{
			Path:         "operations/" + op.name,
			AuthRequired: true,
			NeedsRequest: op.needsRequest,
			Fn: func(ctx context.Context, in rc.Params) (rc.Params, error) {
				return rcSingleCommand(ctx, in, op.name, op.noRemote)
			},
			Title: op.title,
			Help: `This takes the following parameters:

- fs - a remote name string e.g. "drive:"
` + remote + op.help + `
See the [` + op.name + `](/commands/rclone_` + op.name + `/) command for more information on the above.
`,
		})
	}
}

// Run a single command, e.g. Mkdir
func rcSingleCommand(ctx context.Context, in rc.Params, name string, noRemote bool) (out rc.Params, err error) {
	var (
		f      fs.Fs
		remote string
	)
	if noRemote {
		f, err = rc.GetFs(ctx, in)
	} else {
		f, remote, err = rc.GetFsAndRemote(ctx, in)
	}
	if err != nil {
		return nil, err
	}
	switch name {
	case "mkdir":
		return nil, Mkdir(ctx, f, remote)
	case "rmdir":
		return nil, Rmdir(ctx, f, remote)
	case "purge":
		return nil, Purge(ctx, f, remote)
	case "rmdirs":
		leaveRoot, err := in.GetBool("leaveRoot")
		if rc.NotErrParamNotFound(err) {
			return nil, err
		}
		return nil, Rmdirs(ctx, f, remote, leaveRoot)
	case "delete":
		return nil, Delete(ctx, f)
	case "deletefile":
		o, err := f.NewObject(ctx, remote)
		if err != nil {
			return nil, err
		}
		return nil, DeleteFile(ctx, o)
	case "copyurl":
		url, err := in.GetString("url")
		if err != nil {
			return nil, err
		}
		autoFilename, _ := in.GetBool("autoFilename")
		noClobber, _ := in.GetBool("noClobber")
		headerFilename, _ := in.GetBool("headerFilename")

		_, err = CopyURL(ctx, f, remote, url, autoFilename, headerFilename, noClobber)
		return nil, err
	case "uploadfile":

		var request *http.Request
		request, err := in.GetHTTPRequest()

		if err != nil {
			return nil, err
		}

		contentType := request.Header.Get("Content-Type")
		mediaType, params, err := mime.ParseMediaType(contentType)
		if err != nil {
			return nil, err
		}

		if strings.HasPrefix(mediaType, "multipart/") {
			mr := multipart.NewReader(request.Body, params["boundary"])
			for {
				p, err := mr.NextPart()
				if err == io.EOF {
					return nil, nil
				}
				if err != nil {
					return nil, err
				}
				if p.FileName() != "" {
					obj, err := Rcat(ctx, f, path.Join(remote, p.FileName()), p, time.Now(), nil)
					if err != nil {
						return nil, err
					}
					fs.Debugf(obj, "Upload Succeeded")
				}
			}
		}
		return nil, nil
	case "cleanup":
		return nil, CleanUp(ctx, f)
	case "settier":
		if !f.Features().SetTier {
			return nil, fmt.Errorf("remote %s does not support settier", f.Name())
		}
		tier, err := in.GetString("tier")
		if err != nil {
			return nil, err
		}
		return nil, SetTier(ctx, f, tier)
	case "settierfile":
		if !f.Features().SetTier {
			return nil, fmt.Errorf("remote %s does not support settier", f.Name())
		}
		tier, err := in.GetString("tier")
		if err != nil {
			return nil, err
		}
		o, err := f.NewObject(ctx, remote)
		if err != nil {
			return nil, err
		}
		return nil, SetTierFile(ctx, o, tier)
	}
	panic("unknown rcSingleCommand type")
}

func init() {
	rc.Add(rc.Call{
		Path:         "operations/size",
		AuthRequired: true,
		Fn:           rcSize,
		Title:        "Count the number of bytes and files in remote",
		Help: `This takes the following parameters:

- fs - a remote name string e.g. "drive:path/to/dir"

Returns:

- count - number of files
- bytes - number of bytes in those files

See the [size](/commands/rclone_size/) command for more information on the above.
`,
	})
}

// Size a directory
func rcSize(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	f, err := rc.GetFs(ctx, in)
	if err != nil {
		return nil, err
	}
	count, bytes, sizeless, err := Count(ctx, f)
	if err != nil {
		return nil, err
	}
	out = make(rc.Params)
	out["count"] = count
	out["bytes"] = bytes
	out["sizeless"] = sizeless
	return out, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "operations/publiclink",
		AuthRequired: true,
		Fn:           rcPublicLink,
		Title:        "Create or retrieve a public link to the given file or folder.",
		Help: `This takes the following parameters:

- fs - a remote name string e.g. "drive:"
- remote - a path within that remote e.g. "dir"
- unlink - boolean - if set removes the link rather than adding it (optional)
- expire - string - the expiry time of the link e.g. "1d" (optional)

Returns:

- url - URL of the resource

See the [link](/commands/rclone_link/) command for more information on the above.
`,
	})
}

// Make a public link
func rcPublicLink(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	f, remote, err := rc.GetFsAndRemote(ctx, in)
	if err != nil {
		return nil, err
	}
	unlink, _ := in.GetBool("unlink")
	expire, err := in.GetDuration("expire")
	if rc.IsErrParamNotFound(err) {
		expire = time.Duration(fs.DurationOff)
	} else if err != nil {
		return nil, err
	}
	url, err := PublicLink(ctx, f, remote, fs.Duration(expire), unlink)
	if err != nil {
		return nil, err
	}
	out = make(rc.Params)
	out["url"] = url
	return out, nil
}

func init() {
	rc.Add(rc.Call{
		Path:  "operations/fsinfo",
		Fn:    rcFsInfo,
		Title: "Return information about the remote",
		Help: `This takes the following parameters:

- fs - a remote name string e.g. "drive:"

This returns info about the remote passed in;

` + "```" + `
{
        // optional features and whether they are available or not
        "Features": {
                "About": true,
                "BucketBased": false,
                "BucketBasedRootOK": false,
                "CanHaveEmptyDirectories": true,
                "CaseInsensitive": false,
                "ChangeNotify": false,
                "CleanUp": false,
                "Command": true,
                "Copy": false,
                "DirCacheFlush": false,
                "DirMove": true,
                "Disconnect": false,
                "DuplicateFiles": false,
                "GetTier": false,
                "IsLocal": true,
                "ListR": false,
                "MergeDirs": false,
                "MetadataInfo": true,
                "Move": true,
                "OpenWriterAt": true,
                "PublicLink": false,
                "Purge": true,
                "PutStream": true,
                "PutUnchecked": false,
                "ReadMetadata": true,
                "ReadMimeType": false,
                "ServerSideAcrossConfigs": false,
                "SetTier": false,
                "SetWrapper": false,
                "Shutdown": false,
                "SlowHash": true,
                "SlowModTime": false,
                "UnWrap": false,
                "UserInfo": false,
                "UserMetadata": true,
                "WrapFs": false,
                "WriteMetadata": true,
                "WriteMimeType": false
        },
        // Names of hashes available
        "Hashes": [
                "md5",
                "sha1",
                "whirlpool",
                "crc32",
                "sha256",
                "dropbox",
                "mailru",
                "quickxor"
        ],
        "Name": "local",        // Name as created
        "Precision": 1,         // Precision of timestamps in ns
        "Root": "/",            // Path as created
        "String": "Local file system at /", // how the remote will appear in logs
        // Information about the system metadata for this backend
        "MetadataInfo": {
                "System": {
                        "atime": {
                                "Help": "Time of last access",
                                "Type": "RFC 3339",
                                "Example": "2006-01-02T15:04:05.999999999Z07:00"
                        },
                        "btime": {
                                "Help": "Time of file birth (creation)",
                                "Type": "RFC 3339",
                                "Example": "2006-01-02T15:04:05.999999999Z07:00"
                        },
                        "gid": {
                                "Help": "Group ID of owner",
                                "Type": "decimal number",
                                "Example": "500"
                        },
                        "mode": {
                                "Help": "File type and mode",
                                "Type": "octal, unix style",
                                "Example": "0100664"
                        },
                        "mtime": {
                                "Help": "Time of last modification",
                                "Type": "RFC 3339",
                                "Example": "2006-01-02T15:04:05.999999999Z07:00"
                        },
                        "rdev": {
                                "Help": "Device ID (if special file)",
                                "Type": "hexadecimal",
                                "Example": "1abc"
                        },
                        "uid": {
                                "Help": "User ID of owner",
                                "Type": "decimal number",
                                "Example": "500"
                        }
                },
                "Help": "Textual help string\n"
        }
}
` + "```" + `

This command does not have a command line equivalent so use this instead:

    rclone rc --loopback operations/fsinfo fs=remote:

`,
	})
}

// Fsinfo the remote
func rcFsInfo(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	f, err := rc.GetFs(ctx, in)
	if err != nil {
		return nil, err
	}
	info := GetFsInfo(f)
	err = rc.Reshape(&out, info)
	if err != nil {
		return nil, fmt.Errorf("fsinfo Reshape failed: %w", err)
	}
	return out, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "backend/command",
		AuthRequired: true,
		Fn:           rcBackend,
		Title:        "Runs a backend command.",
		Help: `This takes the following parameters:

- command - a string with the command name
- fs - a remote name string e.g. "drive:"
- arg - a list of arguments for the backend command
- opt - a map of string to string of options

Returns:

- result - result from the backend command

Example:

    rclone rc backend/command command=noop fs=. -o echo=yes -o blue -a path1 -a path2

Returns

` + "```" + `
{
	"result": {
		"arg": [
			"path1",
			"path2"
		],
		"name": "noop",
		"opt": {
			"blue": "",
			"echo": "yes"
		}
	}
}
` + "```" + `

Note that this is the direct equivalent of using this "backend"
command:

    rclone backend noop . -o echo=yes -o blue path1 path2

Note that arguments must be preceded by the "-a" flag

See the [backend](/commands/rclone_backend/) command for more information.
`,
	})
}

// Make a public link
func rcBackend(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	f, err := rc.GetFs(ctx, in)
	if err != nil {
		return nil, err
	}
	doCommand := f.Features().Command
	if doCommand == nil {
		return nil, fmt.Errorf("%v: doesn't support backend commands", f)
	}
	command, err := in.GetString("command")
	if err != nil {
		return nil, err
	}
	var opt = map[string]string{}
	err = in.GetStructMissingOK("opt", &opt)
	if err != nil {
		return nil, err
	}
	var arg = []string{}
	err = in.GetStructMissingOK("arg", &arg)
	if err != nil {
		return nil, err
	}
	result, err := doCommand(ctx, command, arg, opt)
	if err != nil {
		return nil, fmt.Errorf("command %q failed: %w", command, err)

	}
	out = make(rc.Params)
	out["result"] = result
	return out, nil
}

// This should really be in fs/rc/internal.go but can't go there due
// to a circular dependency on config.
func init() {
	rc.Add(rc.Call{
		Path:  "core/du",
		Fn:    rcDu,
		Title: "Returns disk usage of a locally attached disk.",
		Help: `
This returns the disk usage for the local directory passed in as dir.

If the directory is not passed in, it defaults to the directory
pointed to by --cache-dir.

- dir - string (optional)

Returns:

` + "```" + `
{
	"dir": "/",
	"info": {
		"Available": 361769115648,
		"Free": 361785892864,
		"Total": 982141468672
	}
}
` + "```" + `
`,
	})
}

// Terminates app
func rcDu(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	dir, err := in.GetString("dir")
	if rc.IsErrParamNotFound(err) {
		dir = config.GetCacheDir()

	} else if err != nil {
		return nil, err
	}
	info, err := diskusage.New(dir)
	if err != nil {
		return nil, err
	}
	out = rc.Params{
		"dir":  dir,
		"info": info,
	}
	return out, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "operations/check",
		AuthRequired: true,
		Fn:           rcCheck,
		Title:        "check the source and destination are the same",
		Help: `Checks the files in the source and destination match.  It compares
sizes and hashes and logs a report of files that don't
match.  It doesn't alter the source or destination.

This takes the following parameters:

- srcFs - a remote name string e.g. "drive:" for the source, "/" for local filesystem
- dstFs - a remote name string e.g. "drive2:" for the destination, "/" for local filesystem
- download - check by downloading rather than with hash
- checkFileHash - treat checkFileFs:checkFileRemote as a SUM file with hashes of given type
- checkFileFs - treat checkFileFs:checkFileRemote as a SUM file with hashes of given type
- checkFileRemote - treat checkFileFs:checkFileRemote as a SUM file with hashes of given type
- oneWay -  check one way only, source files must exist on remote
- combined - make a combined report of changes (default false)
- missingOnSrc - report all files missing from the source (default true)
- missingOnDst - report all files missing from the destination (default true)
- match - report all matching files (default false)
- differ - report all non-matching files (default true)
- error - report all files with errors (hashing or reading) (default true)

If you supply the download flag, it will download the data from
both remotes and check them against each other on the fly.  This can
be useful for remotes that don't support hashes or if you really want
to check all the data.

If you supply the size-only global flag, it will only compare the sizes not
the hashes as well.  Use this for a quick check.

If you supply the checkFileHash option with a valid hash name, the
checkFileFs:checkFileRemote must point to a text file in the SUM
format. This treats the checksum file as the source and dstFs as the
destination. Note that srcFs is not used and should not be supplied in
this case.

Returns:

- success - true if no error, false otherwise
- status - textual summary of check, OK or text string
- hashType - hash used in check, may be missing
- combined - array of strings of combined report of changes
- missingOnSrc - array of strings of all files missing from the source
- missingOnDst - array of strings of all files missing from the destination
- match - array of strings of all matching files
- differ - array of strings of all non-matching files
- error - array of strings of all files with errors (hashing or reading)

`,
	})
}

// Writer which writes into the slice provided
type stringWriter struct {
	out *[]string
}

// Write writes len(p) bytes from p to the underlying data stream. It returns
// the number of bytes written from p (0 <= n <= len(p)) and any error
// encountered that caused the write to stop early. Write must return a non-nil
// error if it returns n < len(p). Write must not modify the slice data,
// even temporarily.
//
// Implementations must not retain p.
func (s stringWriter) Write(p []byte) (n int, err error) {
	result := string(p)
	result = strings.TrimSuffix(result, "\n")
	*s.out = append(*s.out, result)
	return len(p), nil
}

// Check two directories
func rcCheck(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	srcFs, err := rc.GetFsNamed(ctx, in, "srcFs")
	if err != nil && !rc.IsErrParamNotFound(err) {
		return nil, err
	}

	dstFs, err := rc.GetFsNamed(ctx, in, "dstFs")
	if err != nil {
		return nil, err
	}

	checkFileFs, checkFileRemote, err := rc.GetFsAndRemoteNamed(ctx, in, "checkFileFs", "checkFileRemote")
	if err != nil && !rc.IsErrParamNotFound(err) {
		return nil, err
	}

	checkFileHash, err := in.GetString("checkFileHash")
	if err != nil && !rc.IsErrParamNotFound(err) {
		return nil, err
	}

	checkFileSet := 0
	if checkFileHash != "" {
		checkFileSet++
	}
	if checkFileFs != nil {
		checkFileSet++
	}
	if checkFileRemote != "" {
		checkFileSet++
	}
	if checkFileSet > 0 && checkFileSet < 3 {
		return nil, fmt.Errorf("need all of checkFileFs, checkFileRemote, checkFileHash to be set together")
	}

	var checkFileHashType hash.Type
	if checkFileHash != "" {
		if err := checkFileHashType.Set(checkFileHash); err != nil {
			return nil, err
		}
		if srcFs != nil {
			return nil, rc.NewErrParamInvalid(errors.New("only supply dstFs when using checkFileHash"))
		}
	} else if srcFs == nil {
		return nil, rc.NewErrParamInvalid(errors.New("need srcFs parameter when not using checkFileHash"))
	}

	oneway, _ := in.GetBool("oneway")
	download, _ := in.GetBool("download")

	opt := &CheckOpt{
		Fsrc:   srcFs,
		Fdst:   dstFs,
		OneWay: oneway,
	}

	out = rc.Params{}

	getOutput := func(name string, Default bool) io.Writer {
		active, err := in.GetBool(name)
		if err != nil {
			active = Default
		}
		if !active {
			return nil
		}
		result := []string{}
		out[name] = &result
		return stringWriter{&result}
	}

	opt.Combined = getOutput("combined", false)
	opt.MissingOnSrc = getOutput("missingOnSrc", true)
	opt.MissingOnDst = getOutput("missingOnDst", true)
	opt.Match = getOutput("match", false)
	opt.Differ = getOutput("differ", true)
	opt.Error = getOutput("error", true)

	if checkFileHash != "" {
		out["hashType"] = checkFileHashType.String()
		err = CheckSum(ctx, dstFs, checkFileFs, checkFileRemote, checkFileHashType, opt, download)
	} else {
		if download {
			err = CheckDownload(ctx, opt)
		} else {
			out["hashType"] = srcFs.Hashes().Overlap(dstFs.Hashes()).GetOne().String()
			err = Check(ctx, opt)
		}
	}
	if err != nil {
		out["status"] = err.Error()
		out["success"] = false
	} else {
		out["status"] = "OK"
		out["success"] = true
	}
	return out, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "operations/hashsum",
		AuthRequired: true,
		Fn:           rcHashsum,
		Title:        "Produces a hashsum file for all the objects in the path.",
		Help: `Produces a hash file for all the objects in the path using the hash
named.  The output is in the same format as the standard
md5sum/sha1sum tool.

This takes the following parameters:

- fs - a remote name string e.g. "drive:" for the source, "/" for local filesystem
    - this can point to a file and just that file will be returned in the listing.
- hashType - type of hash to be used
- download - check by downloading rather than with hash (boolean)
- base64 - output the hashes in base64 rather than hex (boolean)

If you supply the download flag, it will download the data from the
remote and create the hash on the fly. This can be useful for remotes
that don't support the given hash or if you really want to check all
the data.

Note that if you wish to supply a checkfile to check hashes against
the current files then you should use operations/check instead of
operations/hashsum.

Returns:

- hashsum - array of strings of the hashes
- hashType - type of hash used

Example:

    $ rclone rc --loopback operations/hashsum fs=bin hashType=MD5 download=true base64=true
    {
        "hashType": "md5",
        "hashsum": [
            "WTSVLpuiXyJO_kGzJerRLg==  backend-versions.sh",
            "v1b_OlWCJO9LtNq3EIKkNQ==  bisect-go-rclone.sh",
            "VHbmHzHh4taXzgag8BAIKQ==  bisect-rclone.sh",
        ]
    }

See the [hashsum](/commands/rclone_hashsum/) command for more information on the above.
`,
	})
}

// Hashsum a directory
func rcHashsum(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	ctx, f, err := rc.GetFsNamedFileOK(ctx, in, "fs")
	if err != nil {
		return nil, err
	}

	download, _ := in.GetBool("download")
	base64, _ := in.GetBool("base64")
	hashType, err := in.GetString("hashType")
	if err != nil {
		return nil, fmt.Errorf("%s\n%w", hash.HelpString(0), err)
	}
	var ht hash.Type
	err = ht.Set(hashType)
	if err != nil {
		return nil, fmt.Errorf("%s\n%w", hash.HelpString(0), err)
	}

	hashes := []string{}
	err = HashLister(ctx, ht, base64, download, f, stringWriter{&hashes})
	out = rc.Params{
		"hashType": ht.String(),
		"hashsum":  hashes,
	}
	return out, err
}
