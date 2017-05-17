package azure

import (
	"github.com/Azure/azure-sdk-for-go/storage"
	"github.com/ncw/rclone/fs"
	"time"
	"fmt"
	"encoding/base64"
	"encoding/hex"
	"io"
	"os"
	"path"
)

const (
	listChunkSize = 5000 // number of items to read at once
)

// Fs represents a local filesystem rooted at root
type Fs struct {
	name      string // the name of the remote
	account   string // name of the storage Account
	container string // name of the Storage Account Container
	root      string
	features  *fs.Features // optional features
	bc        *storage.BlobStorageClient
	cc        *storage.Container
}

type Object struct {
	fs     *Fs
	remote string
	blob   *storage.Blob
}

// Register with Fs
func init() {
	fsi := &fs.RegInfo{
		Name:        "azure",
		Description: "Azure Blob Storage",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name: "azure_account",
			Help: "Azure Storage Account Name",
		}, {
			Name: "azure_account_key",
			Help: "Azure Storage Account Key",
		}, {
			Name: "azure_container",
			Help: "Azure Storage Account Blob Container",
		}},
	}
	fs.Register(fsi)
}

//func azureParseUri(uri string) (account, container, root string, err error) {
//	//https://hl37iyhcj646wshrd0.blob.core.windows.net/shared
//	parts := matcher.FindStringSubmatch(uri)
//	if parts == nil {
//		err = errors.Errorf("couldn't parse account / continer out of azure path %q", uri)
//	} else {
//		account, container, root = parts[1], parts[2], parts[3]
//		root = strings.Trim(root, "/")
//	}
//	return
//}

func azureConnection(name, account, accountKey, container string) (*storage.BlobStorageClient, *storage.Container, error) {
	client, err := storage.NewClient(account, accountKey, storage.DefaultBaseURL, "2016-05-31", true)
	if err != nil {
		return nil, nil, err
	}
	tmp_bc := client.GetBlobService()
	bc := &tmp_bc
	tmp_cc := bc.GetContainerReference(container)
	cc := &tmp_cc
	return bc, cc, nil
}

func sl(path string) string {
	if path[len(path)-1:] != "/" {
		return path + "/"
	} else {
		return path
	}
}

func unsl(path string) string {
	if path[len(path)-1:] == "/" {
		return path[:len(path)-1]
	} else {
		return path
	}
}


func NewFs(name, root string) (fs.Fs, error) {
	account := fs.ConfigFileGet(name, "azure_account", os.Getenv("AZURE_ACCOUNT"))
	accountKey := fs.ConfigFileGet(name, "azure_account_key", os.Getenv("AZURE_ACCOUNT_KEY"))
	container := fs.ConfigFileGet(name, "azure_container", os.Getenv("AZURE_CONTAINER"))
	bc, cc, err := azureConnection(name, account, accountKey, container)
	if err != nil {
		return nil, err
	}
	f := &Fs{
		name:      name,
		account:   account,
		container: container,
		root:      root,
		bc:        bc,
		cc:        cc,
	}
	if f.root != "" {
		f.root = sl(f.root)
		_, err := bc.GetBlobProperties(container, root)
		if err == nil {
			// exists !
			f.root = path.Dir(root)
			if f.root == "." {
				f.root = ""
			} else {
				f.root += "/"
			}
			return f, fs.ErrorIsFile
		}
	}
	f.features = (&fs.Features{}).Fill(f)
	return f, nil
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String converts this Fs to a string
func (f *Fs) String() string {
	return fmt.Sprintf("Azure Blob Account %s container %s, directory %s", f.account, f.container, f.root)
}

// Precision of the remote
func (f *Fs) Precision() time.Duration {
	return time.Millisecond
}

func (f *Fs) Copy(src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	err := f.bc.CopyBlob(f.container, f.root + remote, f.bc.GetBlobURL(f.container, srcObj.blob.Name))
	if err != nil {
		return nil, err
	}
	return f.NewObject(remote)
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() fs.HashSet {
	return fs.HashSet(fs.HashMD5)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

type visitFunc func(remote string, blob *storage.Blob, isDirectory bool) error

func listInnerRecurse(f *Fs, out *fs.ListOpts, dir string, level int, visitor visitFunc) error {
	dirWithRoot := f.root
	if dir != "" {
		dirWithRoot += dir + "/"
	}

	maxresults := uint(listChunkSize)
	delimiter := "/"
	if level == fs.MaxLevel {
		return fs.ErrorLevelNotSupported
	}

	marker := ""
	for {
		resp, err := f.cc.ListBlobs(storage.ListBlobsParameters{
			Prefix:     dirWithRoot,
			Delimiter:  delimiter,
			Marker:     marker,
			Include:    "metadata",
			MaxResults: maxresults,
			Timeout:    100,
		})
		if err != nil {
			return err
		}
		rootLength := len(f.root)
		for _, blob := range resp.Blobs {
			err := visitor(blob.Name[rootLength:], &blob, false)
			if err != nil {
				return err
			}
		}
		for _, blobPrefix := range resp.BlobPrefixes {
			strippedDir := unsl(blobPrefix[rootLength:])
			err := visitor(strippedDir, nil, true)
			if err != nil {
				return err
			}
			if err == nil && level < (*out).Level() {
				err := listInnerRecurse(f, out, strippedDir, level+1, visitor)
				if err != nil {
					return err
				}
			}
		}
		if resp.NextMarker != "" {
			marker = resp.NextMarker
		} else {
			break
		}
	}
	return nil
}

// List lists files and directories to out
func (f *Fs) List(out fs.ListOpts, dir string) {
	defer out.Finished()

	// List the objects and directories
	listInnerRecurse(f, &out, dir, 1, func(remote string, blob *storage.Blob, isDirectory bool) error {
		if isDirectory {
			dir := &fs.Dir{
				Name:  remote,
				Bytes: int64(0),
				Count: 0,
			}
			if out.AddDir(dir) {
				return fs.ErrorListAborted
			}
		} else {
			newBlob := blob
			o, err := f.newObjectWithInfo(remote, newBlob)
			if err != nil {
				return err
			}
			if out.Add(o) {
				return fs.ErrorListAborted
			}
		}
		return nil
	})
	return
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(remote string) (fs.Object, error) {
	return f.newObjectWithInfo(remote, nil)
}

func copyBlob(blob *storage.Blob) *storage.Blob {
	var tmp storage.Blob = storage.Blob{}
	tmp.Name = blob.Name
	tmp.Properties.LastModified = blob.Properties.LastModified
	tmp.Properties.Etag = blob.Properties.Etag
	tmp.Properties.ContentMD5 = blob.Properties.ContentMD5
	tmp.Properties.ContentLength = blob.Properties.ContentLength
	tmp.Properties.ContentType = blob.Properties.ContentType
	tmp.Properties.ContentEncoding = blob.Properties.ContentEncoding
	tmp.Properties.CacheControl = blob.Properties.CacheControl
	tmp.Properties.ContentLanguage = blob.Properties.ContentLanguage
	tmp.Properties.BlobType = blob.Properties.BlobType
	tmp.Properties.SequenceNumber = blob.Properties.SequenceNumber
	tmp.Properties.CopyID = blob.Properties.CopyID
	tmp.Properties.CopyStatus = blob.Properties.CopyStatus
	tmp.Properties.CopySource = blob.Properties.CopySource
	tmp.Properties.CopyProgress = blob.Properties.CopyProgress
	tmp.Properties.CopyCompletionTime = blob.Properties.CopyCompletionTime
	tmp.Properties.CopyStatusDescription = blob.Properties.CopyStatusDescription
	tmp.Properties.LeaseStatus = blob.Properties.LeaseStatus
	tmp.Properties.LeaseState = blob.Properties.LeaseState
	for k,v := range blob.Metadata {
		tmp.Metadata[k] = v
	}
	return &tmp
}

//If it can't be found it returns the error ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(remote string, blob *storage.Blob) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	if blob != nil {
		o.blob = copyBlob(blob)
	} else {
		err := o.readMetaData() // reads info and meta, returning an error
		if err != nil {
			return nil, err
		}
	}
	return o, nil
}

// Put the Object into the bucket
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo) (fs.Object, error) {
	// Temporary Object under construction
	fso := &Object{
		fs:     f,
		remote: src.Remote(),
	}
	return fso, fso.Update(in, src)
}

// Mkdir creates the bucket if it doesn't exist
func (f *Fs) Mkdir(dir string) error {
	return nil
}

// Rmdir deletes the bucket if the fs is at the root
// Returns an error if it isn't empty
func (f *Fs) Rmdir(dir string) error {
	return nil
}

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Return a string version
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// Hash returns the Md5sum of an object returning a lowercase hex string
func (o *Object) Hash(t fs.HashType) (string, error) {
	if t != fs.HashMD5 {
		return "", fs.ErrHashUnsupported
	}
	dc, err := base64.StdEncoding.DecodeString(o.blob.Properties.ContentMD5)
	if err != nil {
		fs.Logf(o, "Cannot decode string: %s", err)
		return "", err
	}
	return hex.EncodeToString(dc), nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.blob.Properties.ContentLength
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *Object) readMetaData() (err error) {
	if o.blob != nil {
		return nil
	}
	meta, err := o.fs.bc.GetBlobMetadata(o.fs.container, o.fs.root + o.remote)
	if err != nil {
		return err
	}
	props, err := o.fs.bc.GetBlobProperties(o.fs.container, o.fs.root + o.remote)
	if err != nil {
		return err
	}
	o.blob = copyBlob(&storage.Blob{Name: o.remote, Properties: *props, Metadata: meta})
	return nil
}

func (o *Object) ModTime() time.Time {
	err := o.readMetaData()
	t, _ := time.Parse(time.RFC1123, o.blob.Properties.LastModified)
	if err != nil {
		fs.Logf(o, "Failed to read LastModified: %v", err)
		return time.Now()
	}
	return t
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(modTime time.Time) error {
	return nil
}

// Storable raturns a boolean indicating if this object is storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(options ...fs.OpenOption) (in io.ReadCloser, err error) {
	var readRange *string = nil
	for _, option := range options {
		switch option.(type) {
		case *fs.RangeOption, *fs.SeekOption:
			_, value := option.Header()
			readRange = &value
		default:
			if option.Mandatory() {
				fs.Logf(o, "Unsupported mandatory option: %v", option)
			}
		}
	}
	if readRange != nil {
		return o.fs.bc.GetBlobRange(o.fs.container, o.fs.root + o.remote, *readRange, map[string]string{})
	} else {
		return o.fs.bc.GetBlob(o.fs.container, o.fs.root + o.remote)
	}
}

// Update the Object from in with modTime and size
func (o *Object) Update(in io.Reader, src fs.ObjectInfo) error {
	size := src.Size()
	if size <= 64 * 1000 * 1000 {
		err := o.fs.bc.CreateBlockBlobFromReader(o.fs.container, o.fs.root + o.remote, uint64(size), in, map[string]string{})
		if err != nil {
			return err
		}
	} else {
		// create block, put block, put block list
		return fs.ErrorCantCopy
	}


	// Read the metadata from the newly created object
	o.blob = nil // wipe old metadata
	err := o.readMetaData()
	return err
}

// Remove an object
func (o *Object) Remove() error {
	return o.fs.bc.DeleteBlob(o.fs.container, o.fs.root + o.remote, map[string]string{})
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType() string {
	err := o.readMetaData()
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return ""
	}
	return o.blob.Properties.ContentType
}

// Check the interfaces are satisfied
var (
	_ fs.Fs        = &Fs{}
	_ fs.Copier    = &Fs{}
	_ fs.Object    = &Object{}
	_ fs.MimeTyper = &Object{}
)
