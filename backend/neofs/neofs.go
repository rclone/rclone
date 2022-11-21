// Package neofs provides an interface to NeoFS object storage.
package neofs

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nspcc-dev/neofs-sdk-go/container"
	"github.com/nspcc-dev/neofs-sdk-go/container/acl"
	cid "github.com/nspcc-dev/neofs-sdk-go/container/id"
	"github.com/nspcc-dev/neofs-sdk-go/netmap"
	resolver "github.com/nspcc-dev/neofs-sdk-go/ns"
	"github.com/nspcc-dev/neofs-sdk-go/object"
	oid "github.com/nspcc-dev/neofs-sdk-go/object/id"
	"github.com/nspcc-dev/neofs-sdk-go/pool"
	"github.com/nspcc-dev/neofs-sdk-go/user"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rclone/rclone/lib/bucket"
)

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "neofs",
		Description: "NeoFS - distributed, decentralized object storage",
		NewFs:       NewFs,
		Options: []fs.Option{
			{
				Name:     "neofs_endpoint",
				Help:     "Endpoints to connect to NeoFS node.",
				Required: true,
				Examples: []fs.OptionExample{
					{
						Value: "grpcs://st1.t5.fs.neo.org:8082",
						Help:  "One Testnet endpoint.",
					},
					{
						Value: "grpcs://st1.storage.fs.neo.org:8082",
						Help:  "One Mainnet endpoint.",
					},
					{
						Value: "s01.neofs.devenv:8080",
						Help:  "One devenv endpoint.",
					},
					{
						Value: "s01.neofs.devenv:8080 s02.neofs.devenv:8080",
						Help:  "Multiple endpoints to form pool.",
					},
					{
						Value: "s01.neofs.devenv:8080,1 s02.neofs.devenv:8080,2",
						Help:  "Multiple endpoints with priority (less value is higher priority). Until s01 is healthy all request will be send to it.",
					},
					{
						Value: "s01.neofs.devenv:8080,1,1 s02.neofs.devenv:8080,2,1 s03.neofs.devenv:8080,2,9",
						Help:  "Multiple endpoints with priority and weights. After s01 is unhealthy requests will be send to s02 and s03 in proportions 10% and 90% respectively.",
					},
				},
			},
			{
				Name:    "neofs_dial_timeout",
				Default: fs.Duration(10 * time.Second),
				Help:    "NeoFS connection timeout.",
			},
			{
				Name:    "neofs_healthcheck_timeout",
				Default: fs.Duration(10 * time.Second),
				Help:    "NeoFS request timeout.",
			},
			{
				Name:    "neofs_rebalance_interval",
				Default: fs.Duration(30 * time.Second),
				Help:    "NeoFS rebalance connections interval.",
			},
			{
				Name:    "neofs_session_expiration_duration",
				Default: 100,
				Help:    "NeoFS session expiration duration.",
			},
			{
				Name: "rpc_endpoint",
				Help: "Endpoint to connect to NeoFS morph rpc node to resolve container names.",
				Examples: []fs.OptionExample{
					{
						Value: "https://rpc1.morph.t5.fs.neo.org:51331",
						Help:  "Testnet morph rpc.",
					},
					{
						Value: "https://rpc1.morph.fs.neo.org:40341",
						Help:  "Mainnet morph rpc.",
					},
					{
						Value: "http://morph-chain.neofs.devenv:30333",
						Help:  "Devenv morph rpc.",
					},
				},
			},
			{
				Name:     "wallet",
				Help:     "Path to wallet.",
				Required: true,
			},
			{
				Name: "address",
				Help: "Address of wallet account.",
			},
			{
				Name: "pass",
				Help: "Password to decrypt wallet.",
			},
			{
				Name:    "placement_policy",
				Default: "REP 3",
				Help:    "Placement policy for new containers.",
				Examples: []fs.OptionExample{
					{
						Value: "REP 3",
						Help:  "Container will have 3 replicas.",
					},
					{
						Value: "REP 2 IN X CBF 3 SELECT 2 FROM F AS X FILTER Deployed EQ NSPCC AS F",
						Help:  "Container will have 2 replicas with backup factor 3 and nodes hosted by NSPCC.",
					},
				},
			},
			{
				Name:    "basic_acl",
				Default: "private",
				Help:    "Basic ACL for new containers.",
				Examples: []fs.OptionExample{
					{
						Value: "public-read-write",
						Help:  "Public container, anyone can read and write.",
					},
					{
						Value: "0x0fffffff",
						Help:  "Public container, anyone can read and write. Also extended ACL is allowed.",
					},
					{
						Value: "private",
						Help:  "Private container, only owner has access to it.",
					},
				},
			},
		},
	})
}

// Options defines the configuration for this backend
type Options struct {
	NeofsEndpoint                  string      `config:"neofs_endpoint"`
	NeofsDialTimeout               fs.Duration `config:"neofs_dial_timeout"`
	NeofsHealthcheckTimeout        fs.Duration `config:"neofs_healthcheck_timeout"`
	NeofsRebalanceInterval         fs.Duration `config:"neofs_rebalance_interval"`
	NeofsSessionExpirationDuration uint64      `config:"neofs_session_expiration_duration"`
	RPCEndpoint                    string      `config:"rpc_endpoint"`
	Wallet                         string      `config:"wallet"`
	Address                        string      `config:"address"`
	Password                       string      `config:"password"`
	PlacementPolicy                string      `config:"placement_policy"`
	BasicACLStr                    string      `config:"basic_acl"`
	BasicACL                       acl.Basic   `config:"-"`
}

// Fs represents a remote NeoFS server.
type Fs struct {
	name          string // the name of the remote
	root          string // root of the bucket - ignore all objects above this
	opt           *Options
	ctx           context.Context
	pool          *pool.Pool
	owner         *user.ID
	rootContainer string
	rootDirectory string
	features      *fs.Features
	resolver      *resolver.NNS
}

type searchFilter struct {
	Header    string
	Value     string
	MatchType object.SearchMatchType
}

// Object describes a NeoFS object.
type Object struct {
	*object.Object
	fs          *Fs
	remote      string
	filePath    string
	contentType string
	timestamp   time.Time
}

// Shutdown the backend, closing any background tasks and any cached connections.
func (f *Fs) Shutdown(_ context.Context) error {
	f.pool.Close()
	return nil
}

// PutStream uploads to the remote path with the modTime given of indeterminate size.
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// NewFs creates a new Fs object from the name and root. It connects to
// the host specified in the config file.
func NewFs(ctx context.Context, name string, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	err = opt.BasicACL.DecodeString(opt.BasicACLStr)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse basic acl: %w", err)
	}

	acc, err := getAccount(opt)
	if err != nil {
		return nil, err
	}

	p, err := createPool(ctx, acc.PrivateKey(), opt)
	if err != nil {
		return nil, err
	}

	nnsResolver, err := createNNSResolver(opt)
	if err != nil {
		return nil, err
	}

	var owner user.ID
	user.IDFromKey(&owner, acc.PrivateKey().PrivateKey.PublicKey)

	f := &Fs{
		name:     name,
		opt:      opt,
		ctx:      ctx,
		pool:     p,
		owner:    &owner,
		resolver: nnsResolver,
	}

	f.setRoot(root)

	f.features = (&fs.Features{
		DuplicateFiles:    true,
		ReadMimeType:      true,
		WriteMimeType:     true,
		BucketBased:       true,
		BucketBasedRootOK: true,
	}).Fill(ctx, f)

	if f.rootContainer != "" && f.rootDirectory != "" && !strings.HasSuffix(root, "/") {
		// Check to see if the (container,directory) is actually an existing file
		oldRoot := f.root
		newRoot, leaf := path.Split(oldRoot)
		f.setRoot(newRoot)
		_, err := f.NewObject(ctx, leaf)
		if err != nil {
			// File doesn't exist or is a directory so return old f
			f.setRoot(oldRoot)
			return f, nil
		}
		// return an error with a fs which points to the parent
		return f, fs.ErrorIsFile
	}

	return f, nil
}

func newObject(f *Fs, obj object.Object, container string) *Object {
	// we should not include rootDirectory into remote name
	prefix := f.rootDirectory
	if prefix != "" {
		prefix += "/"
	}

	objID, _ := obj.ID()

	objInfo := &Object{
		Object:   &obj,
		fs:       f,
		filePath: objID.EncodeToString(),
	}

	for _, attr := range obj.Attributes() {
		if attr.Key() == object.AttributeFilePath {
			objInfo.filePath = attr.Value()
		}
		if attr.Key() == object.AttributeContentType {
			objInfo.contentType = attr.Value()
		}
		if attr.Key() == object.AttributeTimestamp {
			value, err := strconv.ParseInt(attr.Value(), 10, 64)
			if err != nil {
				continue
			}
			objInfo.timestamp = time.Unix(value, 0)
		}
	}

	objInfo.remote = objInfo.filePath
	if strings.Contains(objInfo.remote, prefix) {
		objInfo.remote = objInfo.remote[len(prefix):]
		if container != "" && f.rootContainer == "" {
			// we should add container name to remote name if root is empty
			objInfo.remote = path.Join(container, objInfo.remote)
		}
	}

	return objInfo
}

// MimeType of an Object if known, "" otherwise.
func (o *Object) MimeType(_ context.Context) string {
	return o.contentType
}

// Return a string version.
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// ID returns the ID of the Object as string.
func (o *Object) ID() string {
	return o.ObjectID().EncodeToString()
}

// ObjectID returns the id of Object.
func (o *Object) ObjectID() oid.ID {
	objID, _ := o.Object.ID()
	return objID
}

// Remote returns the remote path.
func (o *Object) Remote() string {
	return o.remote
}

// ModTime returns the modification time of the object.
func (o *Object) ModTime(_ context.Context) time.Time {
	return o.timestamp
}

// Size returns the size of the file.
func (o *Object) Size() int64 {
	return int64(o.PayloadSize())
}

// Fs returns the parent Fs.
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Hash returns SHA256 of an object returning a lowercase hex string.
func (o *Object) Hash(_ context.Context, ty hash.Type) (string, error) {
	if ty == hash.SHA256 {
		payloadCheckSum, _ := o.PayloadChecksum()
		return hex.EncodeToString(payloadCheckSum.Value()), nil
	}
	return "", nil
}

// Storable says whether this object can be stored.
func (o *Object) Storable() bool {
	return true
}

// SetModTime returns error since NeoFS doesn't support mod time changing.
func (o *Object) SetModTime(_ context.Context, _ time.Time) error {
	return fs.ErrorCantSetModTime
}

// BuffCloser is wrapper to load files from neofs.
type BuffCloser struct {
	io.Reader
}

// Close implements io.ReadCloser method.
func (bc *BuffCloser) Close() error {
	return nil
}

// Open opens the object for read.
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	var isRange bool
	offset, length := uint64(0), o.PayloadSize()
	fs.FixRangeOption(options, int64(o.PayloadSize()))
	for _, option := range options {
		switch option := option.(type) {
		case *fs.RangeOption:
			isRange = true
			offset = uint64(option.Start)
			if option.End < 0 {
				option.End = int64(o.PayloadSize()) + option.End
			}
			length = uint64(option.End - option.Start + 1)
		case *fs.SeekOption:
			isRange = true
			offset = uint64(option.Offset)
			length = o.PayloadSize() - uint64(option.Offset)
		default:
			if option.Mandatory() {
				fs.Logf(o, "Unsupported mandatory option: %v", option)
			}
		}
	}

	cnrID, _ := o.ContainerID()
	addr := newAddress(cnrID, o.ObjectID())

	if isRange {
		var prm pool.PrmObjectRange
		prm.SetAddress(addr)
		prm.SetOffset(offset)
		prm.SetLength(length)

		rangeReader, err := o.fs.pool.ObjectRange(ctx, prm)
		if err != nil {
			return nil, err
		}
		return &rangeReader, nil
	}

	var prm pool.PrmObjectGet
	prm.SetAddress(addr)

	// we cannot use ObjectRange in this case because it panics if zero length range is requested
	res, err := o.fs.pool.GetObject(ctx, prm)
	if err != nil {
		return nil, fmt.Errorf("couldn't get object %s: %w", addr, err)
	}

	return res.Payload, nil
}

// Name of the remote (as passed into NewFs).
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs).
func (f *Fs) Root() string {
	return f.root
}

// String converts this Fs to a string.
func (f *Fs) String() string {
	if f.rootContainer == "" {
		return fmt.Sprintf("NeoFS root")
	}
	if f.rootDirectory == "" {
		return fmt.Sprintf("NeoFS container %s", f.rootContainer)
	}
	return fmt.Sprintf("NeoFS container %s path %s", f.rootContainer, f.rootDirectory)
}

// Precision of the ModTimes in this Fs.
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// Hashes returns the supported hash types of the filesystem.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.SHA256)
}

// Features returns the optional features of this Fs.
func (f *Fs) Features() *fs.Features {
	return f.features
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
func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	containerStr, containerPath := bucket.Split(path.Join(f.root, dir))

	if containerStr == "" {
		if containerPath != "" {
			return nil, fs.ErrorListBucketRequired
		}
		return f.listContainers(ctx)
	}

	return f.listEntries(ctx, containerStr, containerPath, dir, false)
}

// ListR lists the objects and directories of the Fs starting
// from dir recursively into out.
//
// dir should be "" to start from the root, and should not
// have trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
//
// It should call callback for each tranche of entries read.
// These need not be returned in any particular order.  If
// callback returns an error then the listing will stop
// immediately.
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) error {
	containerStr, containerPath := bucket.Split(path.Join(f.root, dir))

	list := walk.NewListRHelper(callback)

	if containerStr == "" {
		if containerPath != "" {
			return fs.ErrorListBucketRequired
		}
		containers, err := f.listContainers(ctx)
		if err != nil {
			return err
		}
		for _, containerDir := range containers {
			if err = f.listR(ctx, list, containerDir.Remote(), containerPath, dir); err != nil {
				return err
			}
		}
		return list.Flush()
	}

	if err := f.listR(ctx, list, containerStr, containerPath, dir); err != nil {
		return err
	}
	return list.Flush()
}

func (f *Fs) listR(ctx context.Context, list *walk.ListRHelper, containerStr, containerPath, dir string) error {
	entries, err := f.listEntries(ctx, containerStr, containerPath, dir, true)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err = list.Add(entry); err != nil {
			return err
		}
	}

	return nil
}

// Put the Object into the container.
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	containerStr, containerPath := bucket.Split(filepath.Join(f.root, src.Remote()))

	if mimeTyper, ok := src.(fs.MimeTyper); ok {
		mimeType := mimeTyper.MimeType(ctx)
		if len(mimeType) != 0 {
			options = append(options, &fs.HTTPOption{
				Key:   object.AttributeContentType,
				Value: mimeType,
			})
		}
	}

	cnrID, err := f.parseContainer(ctx, containerStr)
	if err != nil {
		return nil, fs.ErrorDirNotFound
	}

	if len(containerPath) == 0 {
		return nil, fs.ErrorObjectNotFound
	}

	ids, err := f.findObjectsFilePath(ctx, cnrID, containerPath)
	if err != nil {
		return nil, err
	}

	headers := map[string]string{object.AttributeFilePath: containerPath}

	for _, option := range options {
		key, value := option.Header()
		lowerKey := strings.ToLower(key)
		switch lowerKey {
		case "":
			// ignore
		case "content-type":
			headers[object.AttributeContentType] = value
		case "timestamp":
			headers[object.AttributeTimestamp] = value
		default:
			if value != "" {
				headers[key] = value
			}
		}
	}

	if headers[object.AttributeTimestamp] == "" {
		headers[object.AttributeTimestamp] = strconv.FormatInt(src.ModTime(ctx).UTC().Unix(), 10)
	}

	objHeader := formObject(f.owner, cnrID, headers)

	var prmPut pool.PrmObjectPut
	prmPut.SetHeader(*objHeader)
	prmPut.SetPayload(in)

	objID, err := f.pool.PutObject(ctx, prmPut)
	if err != nil {
		return nil, err
	}

	var prmHead pool.PrmObjectHead
	prmHead.SetAddress(newAddress(cnrID, objID))

	obj, err := f.pool.HeadObject(ctx, prmHead)
	if err != nil {
		return nil, err
	}

	for _, id := range ids {
		var prmDelete pool.PrmObjectDelete
		prmDelete.SetAddress(newAddress(cnrID, id))

		_ = f.pool.DeleteObject(ctx, prmDelete)
	}

	return newObject(f, obj, ""), nil
}

// Update the Object with the modTime and size.
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	obj, err := o.fs.Put(ctx, in, src, options...)
	if err != nil {
		return err
	}

	objInfo, ok := obj.(*Object)
	if !ok {
		return fmt.Errorf("invalid object type")
	}

	if err = o.Remove(ctx); err != nil {
		fs.Logf(o, "couldn't remove old file after update '%s': %s", o.ObjectID(), err.Error())
	}

	o.filePath = objInfo.filePath
	o.remote = objInfo.remote
	o.timestamp = objInfo.timestamp
	o.Object = objInfo.Object

	return nil
}

// Remove an object.
func (o *Object) Remove(ctx context.Context) error {
	cnrID, _ := o.ContainerID()
	var prm pool.PrmObjectDelete
	prm.SetAddress(newAddress(cnrID, o.ObjectID()))
	return o.fs.pool.DeleteObject(ctx, prm)
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	containerStr, containerPath := bucket.Split(filepath.Join(f.root, remote))

	cnrID, err := f.parseContainer(ctx, containerStr)
	if err != nil {
		return nil, fs.ErrorDirNotFound
	}

	if len(containerPath) == 0 {
		return nil, fs.ErrorObjectNotFound
	}

	ids, err := f.findObjectsFilePath(ctx, cnrID, containerPath)
	if err != nil {
		return nil, err
	}

	if len(ids) == 0 {
		return nil, fs.ErrorObjectNotFound
	}

	var prm pool.PrmObjectHead
	prm.SetAddress(newAddress(cnrID, ids[0]))

	obj, err := f.pool.HeadObject(ctx, prm)
	if err != nil {
		return nil, fmt.Errorf("head object: %w", err)
	}

	return newObject(f, obj, ""), nil
}

// Mkdir creates the container if it doesn't exist.
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	containerStr, _ := bucket.Split(path.Join(f.root, dir))
	if containerStr == "" {
		return nil
	}

	if _, err := f.parseContainer(ctx, containerStr); err == nil {
		return nil
	}

	var policy netmap.PlacementPolicy
	if err := policy.DecodeString(f.opt.PlacementPolicy); err != nil {
		return fmt.Errorf("parse placement policy: %w", err)
	}

	var cnr container.Container
	cnr.Init()
	cnr.SetPlacementPolicy(policy)
	cnr.SetOwner(*f.owner)
	cnr.SetBasicACL(f.opt.BasicACL)

	container.SetCreationTime(&cnr, time.Now())

	container.SetName(&cnr, containerStr)
	var domain container.Domain
	domain.SetName(containerStr)
	container.WriteDomain(&cnr, domain)

	var prm pool.PrmContainerPut
	prm.SetContainer(cnr)

	if _, err := f.pool.PutContainer(ctx, prm); err != nil {
		return fmt.Errorf("put container: %w", err)
	}

	return nil
}

// Rmdir removes the container if empty.
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	containerStr, containerPath := bucket.Split(path.Join(f.root, dir))
	if containerStr == "" || containerPath != "" {
		return nil
	}

	cnrID, err := f.parseContainer(ctx, containerStr)
	if err != nil {
		return fs.ErrorDirNotFound
	}

	isEmpty, err := f.isContainerEmpty(ctx, cnrID)
	if err != nil {
		return err
	}
	if !isEmpty {
		return fs.ErrorDirectoryNotEmpty
	}

	var prm pool.PrmContainerDelete
	prm.SetContainerID(cnrID)

	if err = f.pool.DeleteContainer(ctx, prm); err != nil {
		return fmt.Errorf("couldn't delete container %s '%s': %w", cnrID, containerStr, err)
	}

	return nil
}

// Purge all files in the directory specified.
func (f *Fs) Purge(ctx context.Context, dir string) error {
	containerStr, containerPath := bucket.Split(path.Join(f.root, dir))

	cnrID, err := f.parseContainer(ctx, containerStr)
	if err != nil {
		return nil
	}

	if err = f.deleteByPrefix(ctx, cnrID, containerPath); err != nil {
		return fmt.Errorf("delete by prefix: %w", err)
	}

	return nil
}

// setRoot changes the root of the Fs
func (f *Fs) setRoot(root string) {
	f.root = strings.Trim(root, "/")
	f.rootContainer, f.rootDirectory = bucket.Split(f.root)
}

func (f *Fs) parseContainer(ctx context.Context, containerName string) (cid.ID, error) {
	var err error
	var cnrID cid.ID
	if err = cnrID.DecodeString(containerName); err == nil {
		return cnrID, nil
	}

	if f.resolver != nil {
		if cnrID, err = f.resolver.ResolveContainerName(containerName); err == nil {
			return cnrID, nil
		}
	} else {
		if dirEntries, err := f.listContainers(ctx); err == nil {
			for _, dirEntry := range dirEntries {
				if dirEntry.Remote() == containerName {
					if ider, ok := dirEntry.(fs.IDer); ok {
						return cnrID, cnrID.DecodeString(ider.ID())
					}
				}
			}
		}
	}

	return cid.ID{}, fmt.Errorf("couldn't resolve container '%s'", containerName)
}

func (f *Fs) listEntries(ctx context.Context, containerStr, containerPath, directory string, recursive bool) (fs.DirEntries, error) {
	cnrID, err := f.parseContainer(ctx, containerStr)
	if err != nil {
		return nil, fs.ErrorDirNotFound
	}

	ids, err := f.findObjectsPrefix(ctx, cnrID, containerPath)
	if err != nil {
		return nil, err
	}

	res := make([]fs.DirEntry, 0, len(ids))

	dirs := make(map[string]*fs.Dir)
	objs := make(map[string]*Object)

	for _, id := range ids {
		var prm pool.PrmObjectHead
		prm.SetAddress(newAddress(cnrID, id))
		obj, err := f.pool.HeadObject(ctx, prm)
		if err != nil {
			return nil, err
		}

		objInf := newObject(f, obj, containerStr)

		if !recursive {
			withoutPath := strings.TrimPrefix(objInf.filePath, containerPath)
			trimPrefixSlash := strings.TrimPrefix(withoutPath, "/")
			// emulate directories
			if index := strings.Index(trimPrefixSlash, "/"); index >= 0 {
				dir := fs.NewDir(filepath.Join(directory, trimPrefixSlash[:index]), time.Time{})
				dir.SetID(filepath.Join(containerPath, dir.Remote()))
				dirs[dir.ID()] = dir
				continue
			}
		}

		if o, ok := objs[objInf.remote]; !ok || o.timestamp.Before(objInf.timestamp) {
			objs[objInf.remote] = objInf
		}
	}

	for _, dir := range dirs {
		res = append(res, dir)
	}

	for _, obj := range objs {
		res = append(res, obj)
	}

	return res, nil
}

func (f *Fs) listContainers(ctx context.Context) (fs.DirEntries, error) {
	var prmList pool.PrmContainerList
	prmList.SetOwnerID(*f.owner)
	containers, err := f.pool.ListContainers(ctx, prmList)
	if err != nil {
		return nil, err
	}

	res := make([]fs.DirEntry, len(containers))

	for i, containerID := range containers {
		var prm pool.PrmContainerGet
		prm.SetContainerID(containerID)

		cnr, err := f.pool.GetContainer(ctx, prm)
		if err != nil {
			return nil, fmt.Errorf("couldn't get container '%s': %w", containerID, err)
		}

		res[i] = newDir(containerID, cnr)
	}

	return res, nil
}

func (f *Fs) findObjectsFilePath(ctx context.Context, cnrID cid.ID, filePath string) ([]oid.ID, error) {
	return f.findObjects(ctx, cnrID, searchFilter{
		Header:    object.AttributeFilePath,
		Value:     filePath,
		MatchType: object.MatchStringEqual,
	})
}

func (f *Fs) findObjectsPrefix(ctx context.Context, cnrID cid.ID, prefix string) ([]oid.ID, error) {
	return f.findObjects(ctx, cnrID, searchFilter{
		Header:    object.AttributeFilePath,
		Value:     prefix,
		MatchType: object.MatchCommonPrefix,
	})
}

func (f *Fs) findObjects(ctx context.Context, cnrID cid.ID, filters ...searchFilter) ([]oid.ID, error) {
	sf := object.NewSearchFilters()
	sf.AddRootFilter()

	for _, filter := range filters {
		sf.AddFilter(filter.Header, filter.Value, filter.MatchType)
	}

	return f.searchObjects(ctx, cnrID, sf)
}

func (f *Fs) deleteByPrefix(ctx context.Context, cnrID cid.ID, prefix string) error {
	filters := object.NewSearchFilters()
	filters.AddRootFilter()
	filters.AddFilter(object.AttributeFilePath, prefix, object.MatchCommonPrefix)

	var prmSearch pool.PrmObjectSearch
	prmSearch.SetContainerID(cnrID)
	prmSearch.SetFilters(filters)

	res, err := f.pool.SearchObjects(ctx, prmSearch)
	if err != nil {
		return fmt.Errorf("init searching using client: %w", err)
	}
	defer res.Close()

	var (
		inErr     error
		found     bool
		prmDelete pool.PrmObjectDelete
		addr      oid.Address
	)

	addr.SetContainer(cnrID)

	err = res.Iterate(func(id oid.ID) bool {
		found = true

		addr.SetObject(id)
		prmDelete.SetAddress(addr)

		if err = f.pool.DeleteObject(ctx, prmDelete); err != nil {
			inErr = fmt.Errorf("delete object: %w", err)
			return true
		}

		return false
	})
	if err == nil {
		err = inErr
	}
	if err != nil {
		return fmt.Errorf("iterate objects: %w", err)
	}

	if !found {
		return fs.ErrorDirNotFound
	}

	return nil
}

func (f *Fs) isContainerEmpty(ctx context.Context, cnrID cid.ID) (bool, error) {
	filters := object.NewSearchFilters()
	filters.AddRootFilter()

	var prm pool.PrmObjectSearch
	prm.SetContainerID(cnrID)
	prm.SetFilters(filters)

	res, err := f.pool.SearchObjects(ctx, prm)
	if err != nil {
		return false, fmt.Errorf("init searching using client: %w", err)
	}

	defer res.Close()

	isEmpty := true
	err = res.Iterate(func(id oid.ID) bool {
		isEmpty = false
		return true
	})
	if err != nil {
		return false, fmt.Errorf("iterate objects: %w", err)
	}

	return isEmpty, nil
}

func (f *Fs) searchObjects(ctx context.Context, cnrID cid.ID, filters object.SearchFilters) ([]oid.ID, error) {
	var prm pool.PrmObjectSearch
	prm.SetContainerID(cnrID)
	prm.SetFilters(filters)

	res, err := f.pool.SearchObjects(ctx, prm)
	if err != nil {
		return nil, fmt.Errorf("init searching using client: %w", err)
	}

	defer res.Close()

	var buf []oid.ID

	err = res.Iterate(func(id oid.ID) bool {
		buf = append(buf, id)
		return false
	})
	if err != nil {
		return nil, fmt.Errorf("iterate objects: %w", err)
	}

	return buf, nil
}

// Check the interfaces are satisfied
var (
	_ fs.Fs          = &Fs{}
	_ fs.ListRer     = &Fs{}
	_ fs.Purger      = &Fs{}
	_ fs.PutStreamer = &Fs{}
	_ fs.Shutdowner  = &Fs{}
	_ fs.Object      = &Object{}
	_ fs.MimeTyper   = &Object{}
)
