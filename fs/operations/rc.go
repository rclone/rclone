package operations

import (
	"context"
	"strings"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/rc"
	"github.com/pkg/errors"
)

func init() {
	rc.Add(rc.Call{
		Path:         "operations/list",
		AuthRequired: true,
		Fn:           rcList,
		Title:        "List the given remote and path in JSON format",
		Help: `This takes the following parameters

- fs - a remote name string eg "drive:"
- remote - a path within that remote eg "dir"
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
	f, remote, err := rc.GetFsAndRemote(in)
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

- fs - a remote name string eg "drive:"

The result is as returned from rclone about --json

See the [about command](/commands/rclone_size/) command for more information on the above.
`,
	})
}

// About the remote
func rcAbout(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	f, err := rc.GetFs(in)
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

- srcFs - a remote name string eg "drive:" for the source
- srcRemote - a path within that remote eg "file.txt" for the source
- dstFs - a remote name string eg "drive2:" for the destination
- dstRemote - a path within that remote eg "file2.txt" for the destination
`,
		})
	}
}

// Copy a file
func rcMoveOrCopyFile(ctx context.Context, in rc.Params, cp bool) (out rc.Params, err error) {
	srcFs, srcRemote, err := rc.GetFsAndRemoteNamed(in, "srcFs", "srcRemote")
	if err != nil {
		return nil, err
	}
	dstFs, dstRemote, err := rc.GetFsAndRemoteNamed(in, "dstFs", "dstRemote")
	if err != nil {
		return nil, err
	}
	return nil, moveOrCopyFile(ctx, dstFs, srcFs, dstRemote, srcRemote, cp)
}

func init() {
	for _, op := range []struct {
		name     string
		title    string
		help     string
		noRemote bool
	}{
		{name: "mkdir", title: "Make a destination directory or container"},
		{name: "rmdir", title: "Remove an empty directory or container"},
		{name: "purge", title: "Remove a directory or container and all of its contents"},
		{name: "rmdirs", title: "Remove all the empty directories in the path", help: "- leaveRoot - boolean, set to true not to delete the root\n"},
		{name: "delete", title: "Remove files in the path", noRemote: true},
		{name: "deletefile", title: "Remove the single file pointed to"},
		{name: "copyurl", title: "Copy the URL to the object", help: "- url - string, URL to read from\n"},
		{name: "cleanup", title: "Remove trashed files in the remote or path", noRemote: true},
	} {
		op := op
		remote := "- remote - a path within that remote eg \"dir\"\n"
		if op.noRemote {
			remote = ""
		}
		rc.Add(rc.Call{
			Path:         "operations/" + op.name,
			AuthRequired: true,
			Fn: func(ctx context.Context, in rc.Params) (rc.Params, error) {
				return rcSingleCommand(ctx, in, op.name, op.noRemote)
			},
			Title: op.title,
			Help: `This takes the following parameters

- fs - a remote name string eg "drive:"
` + remote + op.help + `
See the [` + op.name + ` command](/commands/rclone_` + op.name + `/) command for more information on the above.
`,
		})
	}
}

// Run a single command, eg Mkdir
func rcSingleCommand(ctx context.Context, in rc.Params, name string, noRemote bool) (out rc.Params, err error) {
	var (
		f      fs.Fs
		remote string
	)
	if noRemote {
		f, err = rc.GetFs(in)
	} else {
		f, remote, err = rc.GetFsAndRemote(in)
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
		_, err = CopyURL(ctx, f, remote, url)
		return nil, err
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

- fs - a remote name string eg "drive:path/to/dir"

Returns

- count - number of files
- bytes - number of bytes in those files

See the [size command](/commands/rclone_size/) command for more information on the above.
`,
	})
}

// Size a directory
func rcSize(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	f, err := rc.GetFs(in)
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

- fs - a remote name string eg "drive:"
- remote - a path within that remote eg "dir"

Returns

- url - URL of the resource

See the [link command](/commands/rclone_link/) command for more information on the above.
`,
	})
}

// Make a public link
func rcPublicLink(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	f, remote, err := rc.GetFsAndRemote(in)
	if err != nil {
		return nil, err
	}
	url, err := PublicLink(ctx, f, remote)
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

- fs - a remote name string eg "drive:"

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
	f, err := rc.GetFs(in)
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
