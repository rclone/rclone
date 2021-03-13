// Implement config options reading and writing
//
// This is done here rather than in fs/fs.go so we don't cause a circular dependency

package rcshare

import (
	"context"
	"log"
	"math/rand"
	"path"
	"sync"
	"time"

	"github.com/rclone/rclone/fs/rc"
)

// SharedLink descripts a disposable shared file link of rclone server
type SharedLink struct {
	SharedName string
	Token      string
	Fs         string
	Remote     string
	Expire     time.Time
	Unlimited  bool
}

var (
	sharedLinkTableMutex = sync.Mutex{}
	sharedLinkTable      = map[string]SharedLink{}
	letters              = []rune("1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
)

// AddSharedLink Add a shared link
func AddSharedLink(link SharedLink) {
	sharedLinkTableMutex.Lock()
	sharedLinkTable[path.Join(link.Token, link.SharedName)] = link
	sharedLinkTableMutex.Unlock()
}

// GetSharedLink Get Shared link
func GetSharedLink(token string, sharedName string) (SharedLink, bool) {
	sharedLinkTableMutex.Lock()
	v, ok := sharedLinkTable[path.Join(token, sharedName)]
	sharedLinkTableMutex.Unlock()
	return v, ok
}

// DeleteSharedLink Delete Shared link
func DeleteSharedLink(token string, sharedName string) {
	sharedLinkTableMutex.Lock()
	delete(sharedLinkTable, path.Join(token, sharedName))
	sharedLinkTableMutex.Unlock()
}

// generate random sequence, length is n
func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func init() {
	rc.Add(rc.Call{
		Path:         "share/create",
		AuthRequired: true,
		Fn:           rcShareCreate,
		Title:        "Create shared file link of rclone server",
		Help: `This takes the following parameters

- fs - a remote name string eg "drive:"
- remote - a path within that remote eg "path/to/file.txt"
- token - access token. if not provided, auto-generate it. eg "yf62HDvd78" (optional)
- sharedName - shared file name. If not provided, extract it from remote. eg "file.txt" (optional)
- expire - the expiry time of the link. Not provided mean unlimited. Accept ms|s|m|h|d|w|M|y suffixes. Defaults to second if not provided eg "1d" (optional)

Returns

- sharedLink - URL of the resource. eg "share/links/yf62HDvd78/file.txt"

Other users can directly download the shared file by ` + "`GET`" + ` method.

eg: ` + "`curl http://localhost:5572/share/links/yf62HDvd78/file.txt -o file.txt`" + `

`,
	})
}

// Make a share link
func rcShareCreate(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	f, remote, err := rc.GetFsAndRemote(ctx, in)
	oth, _ := in.GetString("remote")
	log.Println(oth)
	if err != nil {
		return nil, err
	}
	// check if file object is exist
	_, err = f.NewObject(ctx, remote)
	if err != nil {
		return nil, err
	}
	link := SharedLink{}
	link.Fs, _ = in.GetString("fs")
	link.Remote, _ = in.GetString("remote")
	token, err := in.GetString("token")
	if err != nil {
		if !rc.IsErrParamNotFound(err) {
			return nil, err
		}
		token = randSeq(20)
	}
	link.Token = token
	sharedName, err := in.GetString("sharedName")
	if err != nil {
		if !rc.IsErrParamNotFound(err) {
			return nil, err
		}
		sharedName = path.Base(remote)
	}
	link.SharedName = sharedName
	expireDuration, err := in.GetDuration("expire")
	link.Unlimited = false
	switch {
	case err == nil:
		link.Expire = time.Now().Add(expireDuration)
	case !rc.IsErrParamNotFound(err):
		return nil, err
	default:
		link.Expire = time.Time{}
		link.Unlimited = true
	}
	// log.Printf(link.expire.UTC().String())
	AddSharedLink(link)
	out = make(rc.Params)
	out["sharedLink"] = path.Join("share", "links", link.Token, link.SharedName)
	return out, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "share/list",
		AuthRequired: true,
		Fn:           rcShareList,
		Title:        "List shared file link of rclone server",
		Help: `Parameters - None

Returns

` + "```jsonc" + `
{
	"sharedLinks": [
		{
			"expire": "2018-11-09T15:53:09.000Z", // expired date, UTC format. if not exist, mean unlimited.
			"fs": "drive:", // a remote name string of the shared remote object
			"remote": "a/b/file.txt", // a path within that remote for the shared remote object
			"sharedName": "file.txt",
			"token": "yf62HDvd78"
		},
		{
			"fs": "drive2:",
			"remote": "foo/bar.pdf",
			"sharedName": "file2.pdf",
			"token": "iIbHBwF5sT6zTVOlgEvD"
		}
	]
}
` + "```\n",
	})
}

// List shared file links
func rcShareList(ctx context.Context, in rc.Params) (out rc.Params, err error) {

	sharedLinkTableMutex.Lock()
	list := make([]rc.Params, 0, len(sharedLinkTable))

	for _, v := range sharedLinkTable {
		item := make(rc.Params)
		item["sharedName"] = v.SharedName
		item["token"] = v.Token
		item["fs"] = v.Fs
		item["remote"] = v.Remote
		if !v.Unlimited {
			item["expire"] = v.Expire.Local().Format(time.RFC3339)
		}

		list = append(list, item)
	}
	sharedLinkTableMutex.Unlock()
	out = make(rc.Params)
	out["sharedLinks"] = list
	return out, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "share/delete",
		AuthRequired: true,
		Fn:           rcShareDelete,
		Title:        "Deprecate shared file link of rclone server",
		Help: `This takes the following parameters

Pick one of two

- fs - string, // a remote name string eg "drive:"
- remote - string // a path within that remote eg "path/to/file.txt"

Note that it may delete more than one shared links

or 

- token - string, // access token. eg "yf62HDvd78"
- sharedName - string // shared file name. eg "file.txt"

Note that it delete exactly one shared link or do nothing

Returns - None
`,
	})
}

// Delete shared file link
func rcShareDelete(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	_, _, err = rc.GetFsAndRemote(ctx, in)
	// provided fs and remote
	if err == nil {
		fs, _ := in.GetString("fs")
		remote, _ := in.GetString("remote")
		sharedLinkTableMutex.Lock()
		for k, v := range sharedLinkTable {
			if v.Fs == fs && v.Remote == remote {
				delete(sharedLinkTable, k)
			}
		}
		sharedLinkTableMutex.Unlock()
		return nil, nil
	}
	if !rc.IsErrParamNotFound(err) {
		return nil, err
	}
	// provided sharedName and token
	sharedName, err := in.GetString("sharedName")
	if err != nil {
		return nil, err
	}
	token, err := in.GetString("token")
	if err != nil {
		return nil, err
	}
	DeleteSharedLink(token, sharedName)
	return nil, nil
}
