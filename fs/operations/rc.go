package operations

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/rc"
)

func init() {
	rc.Add(rc.Call{
		Path:         "operations/list",
		AuthRequired: true,
		Fn:           rcList,
		Title:        "List the given remote and path in JSON format",
		Help: `This takes the following parameters

- fs - a remote name string e.g. "drive:"
- remote - a path within that remote e.g. "dir"
- opt - a dictionary of options to control the listing (optional)
    - recurse - If set recurse directories
    - noModTime - If set return modification time
    - showEncrypted -  If set show decrypted names
    - showOrigIDs - If set show the IDs for each item if known
    - showHash - If set return a dictionary of hashes

The result is

- list
    - This is an array of objects as described in the lsjson command

See the [lsjson command](/commands/rclone_lsjson/) for more information on the above and examples.
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
		Path:         "operations/about",
		AuthRequired: true,
		Fn:           rcAbout,
		Title:        "Return the space used on the remote",
		Help: `This takes the following parameters

- fs - a remote name string e.g. "drive:"

The result is as returned from rclone about --json

See the [about command](/commands/rclone_size/) command for more information on the above.
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
		return nil, errors.Errorf("%v doesn't support about", f)
	}
	u, err := doAbout(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "about call failed")
	}
	err = rc.Reshape(&out, u)
	if err != nil {
		return nil, errors.Wrap(err, "about Reshape failed")
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
			Help: `This takes the following parameters

- srcFs - a remote name string e.g. "drive:" for the source
- srcRemote - a path within that remote e.g. "file.txt" for the source
- dstFs - a remote name string e.g. "drive2:" for the destination
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
		{name: "copyurl", title: "Copy the URL to the object", help: "- url - string, URL to read from\n - autoFilename - boolean, set to true to retrieve destination file name from url"},
		{name: "uploadfile", title: "Upload file using multiform/form-data", help: "- each part in body represents a file to be uploaded", needsRequest: true},
		{name: "cleanup", title: "Remove trashed files in the remote or path", noRemote: true},
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
			Help: `This takes the following parameters

- fs - a remote name string e.g. "drive:"
` + remote + op.help + `
See the [` + op.name + ` command](/commands/rclone_` + op.name + `/) command for more information on the above.
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

		_, err = CopyURL(ctx, f, remote, url, autoFilename, noClobber)
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
					obj, err := Rcat(ctx, f, path.Join(remote, p.FileName()), p, time.Now())
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
	}
	panic("unknown rcSingleCommand type")
}

func init() {
	rc.Add(rc.Call{
		Path:         "operations/size",
		AuthRequired: true,
		Fn:           rcSize,
		Title:        "Count the number of bytes and files in remote",
		Help: `This takes the following parameters

- fs - a remote name string e.g. "drive:path/to/dir"

Returns

- count - number of files
- bytes - number of bytes in those files

See the [size command](/commands/rclone_size/) command for more information on the above.
`,
	})
}

// Size a directory
func rcSize(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	f, err := rc.GetFs(ctx, in)
	if err != nil {
		return nil, err
	}
	count, bytes, err := Count(ctx, f)
	if err != nil {
		return nil, err
	}
	out = make(rc.Params)
	out["count"] = count
	out["bytes"] = bytes
	return out, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "operations/publiclink",
		AuthRequired: true,
		Fn:           rcPublicLink,
		Title:        "Create or retrieve a public link to the given file or folder.",
		Help: `This takes the following parameters

- fs - a remote name string e.g. "drive:"
- remote - a path within that remote e.g. "dir"
- unlink - boolean - if set removes the link rather than adding it (optional)
- expire - string - the expiry time of the link e.g. "1d" (optional)

Returns

- url - URL of the resource

See the [link command](/commands/rclone_link/) command for more information on the above.
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
	if err != nil && !rc.IsErrParamNotFound(err) {
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
		Help: `This takes the following parameters

- fs - a remote name string e.g. "drive:"

This returns info about the remote passed in;

` + "```" + `
{
	// optional features and whether they are available or not
	"Features": {
		"About": true,
		"BucketBased": false,
		"CanHaveEmptyDirectories": true,
		"CaseInsensitive": false,
		"ChangeNotify": false,
		"CleanUp": false,
		"Copy": false,
		"DirCacheFlush": false,
		"DirMove": true,
		"DuplicateFiles": false,
		"GetTier": false,
		"ListR": false,
		"MergeDirs": false,
		"Move": true,
		"OpenWriterAt": true,
		"PublicLink": false,
		"Purge": true,
		"PutStream": true,
		"PutUnchecked": false,
		"ReadMimeType": false,
		"ServerSideAcrossConfigs": false,
		"SetTier": false,
		"SetWrapper": false,
		"UnWrap": false,
		"WrapFs": false,
		"WriteMimeType": false
	},
	// Names of hashes available
	"Hashes": [
		"MD5",
		"SHA-1",
		"DropboxHash",
		"QuickXorHash"
	],
	"Name": "local",	// Name as created
	"Precision": 1,		// Precision of timestamps in ns
	"Root": "/",		// Path as created
	"String": "Local file system at /" // how the remote will appear in logs
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
		return nil, errors.Wrap(err, "fsinfo Reshape failed")
	}
	return out, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "backend/command",
		AuthRequired: true,
		Fn:           rcBackend,
		Title:        "Runs a backend command.",
		Help: `This takes the following parameters

- command - a string with the command name
- fs - a remote name string e.g. "drive:"
- arg - a list of arguments for the backend command
- opt - a map of string to string of options

Returns

- result - result from the backend command

For example

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
		return nil, errors.Errorf("%v: doesn't support backend commands", f)
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
	result, err := doCommand(context.Background(), command, arg, opt)
	if err != nil {
		return nil, errors.Wrapf(err, "command %q failed", command)

	}
	out = make(rc.Params)
	out["result"] = result
	return out, nil
}
