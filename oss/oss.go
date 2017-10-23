package oss

import (
	"fmt"
	"io"
	"log"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/swift"
	"github.com/pkg/errors"
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "oss",
		Description: "Aliyun oss",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name: "env_auth",
			Help: "Get oss credentials from runtime (environment variables or EC2 meta data if no env vars). Only applies if access_id and access_key is blank.",
			Examples: []fs.OptionExample{
				{
					Value: "false",
					Help:  "Enter oss credentials in the next step",
				}, {
					Value: "true",
					Help:  "Get oss credentials from the environment (env vars or IAM)",
				},
			},
		}, {
			Name: "access_id",
			Help: "oss Access Key ID - leave blank for anonymous access or runtime credentials.",
		}, {
			Name: "access_key",
			Help: "oss Secret Access Key (password) - leave blank for anonymous access or runtime credentials.",
		}, {
			Name: "region",
			Help: "Region to connect to.",
			Examples: []fs.OptionExample{{
				Value: "oss-cn-hangzhou",
				Help:  "China east (hangzhou) The default endpoint - a good choice if you are unsure.",
			}, {
				Value: "oss-cn-shanghai",
				Help:  "China east (shanghai) Region\nNeeds location constraint cn-east-2.",
			}, {
				Value: "oss-cn-qingdao",
				Help:  "China north (qingdao) Region\nNeeds location constraint cn-north-1.",
			}, {
				Value: "oss-cn-beijing",
				Help:  "China north (beijing) Region\nNeeds location constraint cn-north-2.",
			}, {
				Value: "oss-cn-zhangjiakou",
				Help:  "China north (zhangjiakou) Region\nNeeds location constraint cn-north-3.",
			}, {
				Value: "oss-cn-shenzhen",
				Help:  "China south (shenzhen) Region\nNeeds location constraint cn-south-1.",
			}, {
				Value: "oss-cn-hongkong",
				Help:  "China (hongkong) Region\nNeeds location constraint cn-hongkong.",
			}, {
				Value: "oss-us-west-1",
				Help:  "west America (Silicon Valley)\nNeeds location constraint us-west-1.",
			}, {
				Value: "oss-us-east-1",
				Help:  "east America (Virginia) Region\nNeeds location constraint us-east-1.",
			}, {
				Value: "oss-ap-southeast-1",
				Help:  "Asia Pacific (Singapore) Region\nNeeds location constraint ap-southeast-1.",
			}, {
				Value: "oss-ap-southeast-2",
				Help:  "Asia Pacific (Sydney) Region\nNeeds location constraint ap-southeast-2.",
			}, {
				Value: "oss-ap-northeast-1",
				Help:  "Asia Pacific (Japanese)\nNeeds location constraint ap-northeast-1.",
			}, {
				Value: "oss-eu-central-1",
				Help:  "Central Europe (Frankfurt)\nNeeds location constraint eu-central-1.",
			}, {
				Value: "oss-me-east-1",
				Help:  "Middle East (Dubai) Region\nNeeds location constraint me-east-1.",
			}},
		}, {
			// 输入节点
			// oss endpoints: https://help.aliyun.com/document_detail/31837.html
			Name: "endpoint",
			Help: "Endpoint for OSS API.\nLeave blank if using OSS to use the default endpoint for the region.\nSpecify if using an OSS clone such as Ceph.",
		}, {
			Name: "location_constraint",
			Help: "Location constraint - must be set to match the Region. Used when creating buckets only.",
			Examples: []fs.OptionExample{{
				Value: "cn-hangzhou",
				Help:  "China east ( hangzhou) Region.",
			}, {
				Value: "cn-shanghai",
				Help:  "China east (shanghai) Region.",
			}, {
				Value: "cn-qingdao",
				Help:  "China north (qingdao) Region.",
			}, {
				Value: "cn-beijing",
				Help:  "China north (beijing) Region.",
			}, {
				Value: "cn-zhangjiakou",
				Help:  "China north (zhangjiakou) Region.",
			}, {
				Value: "cn-shenzhen",
				Help:  "China south (shenzhen) Region.",
			}, {
				Value: "cn-hongkong",
				Help:  "China (hongkong) Region.",
			}, {
				Value: "us-west-1",
				Help:  "west America (Silicon Valley).",
			}, {
				Value: "us-east-1",
				Help:  "east America (Virginia) Region.",
			}, {
				Value: "ap-southeast-1",
				Help:  "Asia Pacific (Singapore) Region.",
			}, {
				Value: "ap-southeast-2",
				Help:  "Asia Pacific (Sydney) Region.",
			}, {
				Value: "ap-northeast-1",
				Help:  "Asia Pacific (Japanese).",
			}, {
				Value: "eu-central-1",
				Help:  "Central Europe (Frankfurt).",
			}, {
				Value: "me-east-1",
				Help:  "Middle East (Dubai) Region.",
			}},
		}, {
			Name: "acl",
			Help: "Canned ACL used when creating buckets and/or storing objects in oss.",
			Examples: []fs.OptionExample{{
				Value: "private",
				Help:  "Owner gets FULL_CONTROL. No one else has access rights (default).",
			}, {
				Value: "public-read",
				Help:  "Owner gets FULL_CONTROL. The AllUsers group gets READ access.",
			}, {
				Value: "public-read-write",
				Help:  "Owner gets FULL_CONTROL. The AllUsers group gets READ and WRITE access.\nGranting this on a bucket is generally not recommended.",
			}, {
				Value: "default",
				Help:  "Owner gets FULL_CONTROL.the bucket is not recommended.",
			}},
		}, {
			Name: "storage_class",
			Help: "StorageClassType Bucket的存储类型",
			Examples: []fs.OptionExample{{
				Value: "Standard",
				Help:  "StorageStandard 标准存储模式",
			}, {
				Value: "Archive",
				Help:  "StorageArchive 归档存储模式",
			}, {
				Value: "IA",
				Help:  "StorageIA 低频存储模式",
			}},
		}},
	})
}

// Constants
const (
	metaMtime      = "Mtime"                // the meta key to store mtime in - eg X-Amz-Meta-Mtime
	listChunkSize  = 1000                   // number of items to read at once
	maxRetries     = 10                     // number of retries to make of operations
	maxSizeForCopy = 5 * 1024 * 1024 * 1024 // The maximum size of object we can COPY
)

// Globals
var (
	ossACL          = fs.StringP("oss-acl", "", "", "Canned ACL used when creating buckets and/or storing objects in OSS")
	ossStorageClass = fs.StringP("oss-storage-class", "", "", "Storage class to use when uploading OSS objects (Standard|Archive|IA)")
)

// Fs represents a remote oss server
type Fs struct {
	name               string       // the name of the remote
	root               string       // root of the bucket - ignore all objects above this
	features           *fs.Features // optional features
	c                  *oss.Client  // the connection to the oss server
	bucket             string       // the bucket we are working on
	bucketOKMu         sync.Mutex   // mutex to protect bucket OK
	bucketOK           bool         // true if we have created the bucket
	locationConstraint string       // location constraint of new buckets
	storageClass       string       // storage class
	acl                string       // bucket acl
}

// Object describes a oss object
type Object struct {
	// Will definitely have everything but meta which may be nil
	// List will read everything but meta & mimeType - to fill
	// that in you need to call metaData
	fs           *Fs                // what this object is part of
	remote       string             // The remote path
	meta         map[string]*string // The object metadata if known - may be nil
	key          string             `xml:"Key"`          // Object的Key
	mimeType     string             `xml:"Type"`         // Object Type
	size         int64              `xml:"Size"`         // Object的长度字节数
	etag         string             `xml:"ETag"`         // 标示Object的内容
	lastModified time.Time          `xml:"LastModified"` // Object最后修改时间
	storageClass string             `xml:"StorageClass"` // Object的存储类型
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	if f.root == "" {
		return f.bucket
	}
	return f.bucket + "/" + f.root
}

// String converts this Fs to a string
func (f *Fs) String() string {
	if f.root == "" {
		return fmt.Sprintf("oss bucket %s", f.bucket)
	}
	return fmt.Sprintf("oss bucket %s path %s", f.bucket, f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Pattern to match a oss path
// 验证路径格式
var matcher = regexp.MustCompile(`^([^/]*)(.*)$`)

// parseParse parses a oss 'url'
// 解析oss的路径
func ossParsePath(path string) (bucket, directory string, err error) {
	parts := matcher.FindStringSubmatch(path)
	if parts == nil {
		err = errors.Errorf("couldn't parse bucket out of oss path %q", path)
	} else {
		bucket, directory = parts[1], parts[2]
		directory = strings.Trim(directory, "/") //去除directory两边的“/”
	}
	return
}

// ossConnection makes a connection to oss
// 连接oss服务器
func ossConnection(endpoint string, accessID string, accessKey string) (*oss.Client, error) {
	client, err := oss.New(endpoint, accessID, accessKey)
	return client, err
}

func NewFs(name, root string) (fs.Fs, error) {
	bucket, directory, err := ossParsePath(root)
	if err != nil {
		return nil, err
	}
	endpoint := fs.ConfigFileGet(name, "endpoint")
	accessID := fs.ConfigFileGet(name, "access_id")
	accessKey := fs.ConfigFileGet(name, "access_key")
	c, err := ossConnection(endpoint, accessID, accessKey) //验证账户
	if err != nil {
		return nil, err
	}
	f := &Fs{
		name:               name, //remote的name
		c:                  c,
		bucket:             bucket,    //bucket的name
		root:               directory, //bucket里目录的路径，可以为空
		acl:                fs.ConfigFileGet(name, "acl"),
		locationConstraint: fs.ConfigFileGet(name, "location_constraint"),
		storageClass:       fs.ConfigFileGet(name, "storage_class"),
	}
	fmt.Println("newFs:", f.root)
	f.features = (&fs.Features{
		ReadMimeType:  true,
		WriteMimeType: true,
		BucketBased:   true,
	}).Fill(f)

	if *ossACL != "" {
		f.acl = *ossACL
	}
	if *ossStorageClass != "" {
		f.storageClass = *ossStorageClass
	}

	bucketObject, err := f.c.Bucket(bucket) //取存储空间（Bucket）的对象实例。
	if err != nil {
		log.Fatalf("Check Bucket Failed for %q: %v", bucket, err)
		return nil, err
	}
	if bucketObject == nil {
		log.Fatalf("Bucket is not existed for %q", bucket)
		return nil, nil
	}

	if f.root != "" {
		f.root += "/"
		//Check to see if the object exists
		bucket, _ := f.c.Bucket(f.bucket)
		pre := oss.Prefix(f.root)
		_, err := bucket.ListObjects(pre)
		if err == nil {
			f.root = path.Dir(directory)
			if f.root == "." {
				f.root = ""
			} else {
				f.root += "/"
			}
			// return an error with an fs which points to the parent
			return f, fs.ErrorIsFile
		}
	}

	return f, nil
}

// Return an Object from a path
// If it can't be found it returns the error ErrorObjectNotFound.
// 返回路径中的object，如果object不存在则返回ErrorObjectNotFound错误
func (f *Fs) newObjectWithInfo(remote string, info *oss.ObjectProperties) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	if info != nil {
		// Set info but not meta
		if &info.LastModified == nil {
			fs.Logf(o, "Failed to read last modified")
			o.lastModified = time.Now()
		} else {
			o.lastModified = info.LastModified
		}
		o.etag = info.ETag
		o.size = info.Size
	} else {
		err := o.metaData(f) // reads info and meta, returning an error
		if err != nil {
			return nil, err
		}
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(remote string) (fs.Object, error) {
	return f.newObjectWithInfo(remote, nil)
}

// listFn is called from list to handle an object.
type listFn func(remote string, object *oss.ObjectProperties, isDirectory bool) error

// list the objects into the function supplied
//
// dir is the starting directory, "" for root
//
// Set recurse to read sub directories
func (f *Fs) list(dir string, recurse bool, fn listFn) error {
	root := f.root
	if dir != "" {
		root += dir + "/"
	}
	delimiter := oss.Delimiter("")
	if !recurse {
		delimiter = oss.Delimiter("/")
	}
	pre := oss.Prefix(root)
	fmt.Println("pre:", root)
	bucket, _ := f.c.Bucket(f.bucket)
	for {
		listObjects, err := bucket.ListObjects(pre, delimiter)
		rootLength := len(f.root)
		if !recurse {
			for _, commonPrefix := range listObjects.CommonPrefixes {
				if commonPrefix == "" {
					fs.Logf(f, "Nil common prefix received")
					continue
				}
				remote := commonPrefix
				if !strings.HasPrefix(remote, f.root) {
					fs.Logf(f, "Odd name received %q", remote)
					continue
				}
				remote = remote[rootLength:]
				if strings.HasSuffix(remote, "/") {
					remote = remote[:len(remote)-1]
				}
				err = fn(remote, &oss.ObjectProperties{Key: remote}, true)
				if err != nil {
					return err
				}
			}
		}
		for _, object := range listObjects.Objects {
			key := object.Key
			if !strings.HasPrefix(key, f.root) {
				fs.Logf(f, "Odd name received %q", key)
				continue
			}
			if strings.HasSuffix(key, "/") {
				continue
			}
			remote := key[rootLength:]
			err = fn(remote, &object, false)
			if err != nil {
				return err
			}
		}
		if !listObjects.IsTruncated {
			break
		}
	}

	return nil
}

// Convert a list item into a DirEntry
func (f *Fs) itemToDirEntry(remote string, object *oss.ObjectProperties, isDirectory bool) (fs.DirEntry, error) {
	if isDirectory {
		size := int64(0)
		if &object.Size != nil {
			size = object.Size
		}
		d := fs.NewDir(remote, time.Time{}).SetSize(size)
		return d, nil
	}
	o, err := f.newObjectWithInfo(remote, object)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// listDir lists files and directories to out
// 显示目录和文件
func (f *Fs) listDir(dir string) (entries fs.DirEntries, err error) {
	// List the objects and directories
	err = f.list(dir, false, func(remote string, object *oss.ObjectProperties, isDirectory bool) error {
		if strings.HasSuffix(remote, "/") {
			return nil
		}
		entry, err := f.itemToDirEntry(remote, object, isDirectory)
		if err != nil {
			return err
		}
		if entry != nil {
			entries = append(entries, entry)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// listBuckets lists the buckets to out
// 显示bucket列表
func (f *Fs) listBuckets(dir string) (entries fs.DirEntries, err error) {
	if dir != "" {
		return nil, fs.ErrorListBucketRequired
	}
	listBucketsResult, err := f.c.ListBuckets(nil)
	if err != nil {
		return nil, err
	}
	for _, bucket := range listBucketsResult.Buckets {
		d := fs.NewDir(bucket.Name, bucket.CreationDate)
		entries = append(entries, d)
	}
	return entries, nil
}

// List the objects and directories in dir into entries.  The
// entries can be returned in any order but should be for a
// complete directory.
//
// dir should be "" to list the root, and should not have
// trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
func (f *Fs) List(dir string) (entries fs.DirEntries, err error) {
	if f.bucket == "" {
		return f.listBuckets(dir)
	}
	return f.listDir(dir)
}

// ListR lists the objects and directories of the Fs starting
// from dir recursively into out.
//
// dir should be "" to start from the root
//
// This should return ErrDirNotFound if the directory isn't
// found.
//
// It should call callback for each tranche of entries read.
// These need not be returned in any particular order.  If
// callback returns an error then the listing will stop
// immediately.
//
// Don't implement this unless you have a more efficient way
// of listing recursively that doing a directory traversal.
// 递归显示列表
func (f *Fs) ListR(dir string, callback fs.ListRCallback) (err error) {
	if f.bucket == "" {
		return fs.ErrorListBucketRequired
	}
	list := fs.NewListRHelper(callback)
	err = f.list(dir, true, func(remote string, object *oss.ObjectProperties, isDirectory bool) error {
		fmt.Println("***ListR-dir:", dir)
		entry, err := f.itemToDirEntry(remote, object, isDirectory)
		if err != nil {
			return err
		}
		return list.Add(entry)
	})
	if err != nil {
		return err
	}
	return list.Flush()
}

// Put in to the remote path with the modTime given of the given size
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
// Put the Object into the bucket
// 上传操作
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// Temporary Object under construction
	if f.root != "" {
		fs := &Object{
			fs: f,
			//在给复制到oss上的object设置信息时，如果f.root不为空（即上传到指定目录），则要在文件夹和object名字中间加上一个“/”
			remote: f.root + "/" + src.Remote(),
		}
		return fs, fs.Update(in, src, options...)
	} else {
		fs := &Object{
			fs:     f,
			remote: f.root + src.Remote(),
		}
		return fs, fs.Update(in, src, options...)
	}
}

// NB this can return incorrect results if called immediately after bucket deletion
// 如果在删除bucket时立即调用会报错
func (f *Fs) dirExists() (bool, error) {
	isBucketExist, err := oss.Client.IsBucketExist(*f.c, f.bucket)
	return isBucketExist, err
}

// Mkdir makes the directory (container, bucket)
// Shouldn't return an error if it already exists
// Mkdir creates the bucket if it doesn't exist
// 创建（bucket，目录），如果已经存在不会报错，不存在会创建
func (f *Fs) Mkdir(dir string) error {
	f.bucketOKMu.Lock()
	defer f.bucketOKMu.Unlock()
	if f.bucketOK {
		return nil
	}
	err := f.c.CreateBucket(f.bucket)
	if err != nil {
		if err.Error() == "BucketAlreadyOwnedByYou" {
			err = nil
		}
	}
	if err == nil {
		f.bucketOK = true
	}
	return err
}

// Rmdir removes the directory (container, bucket) if empty
// Return an error if it doesn't exist or isn't empty
// 删除目录（非空不能删除，并且会报错）
func (f *Fs) Rmdir(dir string) error {
	f.bucketOKMu.Lock()
	defer f.bucketOKMu.Unlock()
	if f.root != "" || dir != "" {
		return nil
	}
	// 删除bucket
	err := f.c.DeleteBucket(f.bucket)
	if err == nil {
		f.bucketOK = false
	}
	return err
}

// Precision of the remote
func (f *Fs) Precision() time.Duration {
	return time.Nanosecond
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() fs.HashSet {
	return fs.HashSet(fs.HashMD5)
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

func (o *Object) Remote() string {
	fmt.Println("Remote:", o.remote)
	return o.remote
}

var matchMd5 = regexp.MustCompile(`^[0-9a-f]{32}$`)

// Hash returns the Md5sum of an object returning a lowercase hex string
// 返回对象的Md5sum，为小写十六进制字符串的
func (o *Object) Hash(t fs.HashType) (string, error) {
	if t != fs.HashMD5 {
		return "", fs.ErrHashUnsupported
	}
	etag := strings.Trim(strings.ToLower(o.etag), `"`)
	// Check the etag is a valid md5sum
	if !matchMd5.MatchString(etag) {
		// fs.Debugf(o, "Invalid md5sum (probably multipart uploaded) - ignoring: %q", etag)
		return "", nil
	}
	return etag, nil
}

// Size returns the size of an object in bytes
// 返回object的大小
func (o *Object) Size() int64 {
	return o.size
}

// readMetaData gets the metadata if it hasn't already been fetched
// it also sets the info
// 获取object的metaData，并且可以设置object的metaData
func (o *Object) metaData(f *Fs) (err error) {
	if o.meta != nil {
		return nil
	}
	key := o.fs.root + o.remote
	archiveBucket, err := f.c.Bucket(o.fs.bucket)
	meta, erro := archiveBucket.GetObjectDetailedMeta(key)
	ContentLength, err := strconv.ParseInt(meta.Get("Content-Length"), 10, 64)
	o.size = ContentLength
	last := meta.Get("Last-Modified")
	lastModified, _ := time.Parse("Mon, 02 Jan 2006 15:04:05 MST", last)
	o.lastModified = lastModified
	return erro
}

// ModTime returns the modification time of the object
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
// 返回修改时间
func (o *Object) ModTime() time.Time {
	err := o.metaData(o.fs)
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return time.Now()
	}
	// read mtime out of metadata if available
	d, ok := o.meta[metaMtime]
	if !ok || d == nil {
		// fs.Debugf(o, "No metadata")
		return o.lastModified
	}
	modTime, err := swift.FloatStringToTime(*d)
	if err != nil {
		fs.Logf(o, "Failed to read mtime from object: %v", err)
		return o.lastModified
	}
	return modTime
}

// SetModTime sets the modification time of the local fs object
// 设置修改时间
func (o *Object) SetModTime(modTime time.Time) error {
	err := o.metaData(o.fs)
	if err != nil {
		return err
	}
	o.meta = make(map[string]*string)
	o.meta[metaMtime] = StringToPointer(swift.TimeToFloatString(modTime))
	if o.size >= maxSizeForCopy {
		fs.Debugf(o, "SetModTime is unsupported for objects bigger than %v bytes", fs.SizeSuffix(maxSizeForCopy))
		return nil
	}
	// Guess the content type
	mimeType := fs.MimeType(o)
	// Copy the object to itself to update the metadata
	key := o.fs.root + o.remote
	sourceKey := o.fs.bucket + "/" + key
	bucket, err := o.fs.c.Bucket(o.fs.bucket)
	_, erro := bucket.CopyObject(key, key, oss.ContentType(mimeType),
		oss.CopySource(o.fs.bucket, url.QueryEscape(sourceKey)),
		oss.MetadataDirective(oss.MetaReplace))
	return erro
}

// Storable raturns a boolean indicating if this object is storable
// object是否可储存
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
// 获取一个对象实例
func (o *Object) Open(options ...fs.OpenOption) (in io.ReadCloser, err error) {
	key := o.fs.root + o.remote
	bucket, _ := o.fs.c.Bucket(o.fs.bucket)
	resp, err := bucket.GetObject(key)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// Update the Object from in with modTime and size
// 使用metaData更新对象
func (o *Object) Update(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	err := o.fs.Mkdir("")
	if err != nil {
		return err
	}
	mimeType := fs.MimeType(src)
	bucket, err := o.fs.c.Bucket(o.fs.bucket)
	key := o.fs.root + o.remote
	erro := bucket.PutObject(key, in, oss.ContentType(mimeType))
	if erro != nil {
		return erro
	}
	// Read the metadata from the newly created object
	o.meta = nil // wipe old metadata
	erro = o.metaData(o.fs)
	return erro
}

// Remove an object
// 删除一个object
func (o *Object) Remove() error {
	bucket, _ := o.fs.c.Bucket(o.fs.bucket)
	key := o.fs.root + o.remote
	isObjectExist, _ := bucket.IsObjectExist(key)
	if isObjectExist {
		return nil
	} else {
		erro := bucket.DeleteObject(key)
		return erro
	}
}

// MimeType returns the content type of the Object if known, or "" if not
// MimeType of an Object if known, "" otherwise
// 返回object的类型
func (o *Object) MimeType() string {
	err := o.metaData(o.fs)
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return ""
	}
	return o.mimeType
}

// String returns a pointer to the string value passed in.
// util 把string转换成string的指针
func StringToPointer(v string) *string {
	return &v
}

// Check the interfaces are satisfied
var (
	_ fs.Fs        = &Fs{}
	_ fs.ListRer   = &Fs{}
	_ fs.Object    = &Object{}
	_ fs.MimeTyper = &Object{}
)
