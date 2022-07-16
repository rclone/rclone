// Package discord provides an interface to Discord's chat channels.
// I'm not saying anything wrong!
package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/lib/rest"
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "discord",
		Description: "Discord",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "auth_token",
			Help:     "Authentication token for your Discord bot. Follow instructions at https://rclone.org/discord/ for this field.",
			Required: true,
		}, {
			Name:     "chunks_channel",
			Help:     "Internal ID of the channels to upload attached files. Split by space to spread to multiple channels.",
			Required: true,
		}, {
			Name:     "journal_channel",
			Help:     "Internal ID of the channels to upload metadata. It is preferred to have many channel as possible to list up files faster.",
			Required: true,
		}, {
			Name:     "list_timeout",
			Help:     "Timeout for listing up metadata messages. 0 for no timeout.",
			Advanced: true,
		}, {
			Name:     "quick_delete",
			Help:     "Enabling this will make it not to delete messages with attached files. Metadata message is always deleted.",
			Advanced: true,
			Default:  false,
		}, {
			Name:     "chunk_message",
			Help:     "Message for posting chunk message.",
			Advanced: true,
			Default:  "Hello! This is a chunk message posted by rclone.",
		}, {
			Name:     "journal_message",
			Help:     "Message for posting chunk message.",
			Advanced: true,
			Default:  "Hello! This is a metadata message posted by rclone.",
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			// We'll store metadata in JSON format, so the same restriction apply here
			Default: encoder.Display | encoder.EncodeInvalidUtf8,
		},
		}})
}

const (
	overallUploadLimit int64 = 8 * 1024 * 1024
	chunkSize          int64 = overallUploadLimit
)

// Options defines the configuration for this backend
type Options struct {
	AuthToken      string               `config:"auth_token"`
	ChunksChannel  string               `config:"chunks_channel"`
	JournalChannel string               `config:"journal_channel"`
	ListTimeout    fs.Duration          `config:"list_timeout"`
	QuickDelete    bool                 `config:"quick_delete"`
	ChunksMessage  string               `config:"chunks_message"`
	JournalMessage string               `config:"journal_message"`
	Enc            encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a Discord remote
type Fs struct {
	name            string       // name of this remote
	root            string       // the path we are working on if any
	opt             Options      // parsed config options
	features        *fs.Features // optional features
	bot             *discordgo.Session
	chunkChannels   []discordgo.Channel
	journalChannels []discordgo.Channel
	srv             *rest.Client
	ctx             context.Context
	entryCache      map[string]*Object
	entryLock       sync.RWMutex
}

// Object describes a file at Discord channels
type Object struct {
	fs                *Fs       // reference to Fs
	remote            string    // the remote path
	modTime           time.Time // last modified time
	metadataChannelID string
	messageID         string
	meta              *JournalMetadata
	metaMu            sync.RWMutex
}

// JournalMetadata is the content of metadata to be stored on journal channel
type JournalMetadata struct {
	FileName  string   `json:"filename"`
	Size      int64    `json:"size"`
	ModTime   string   `json:"modtime"`
	Md5       string   `json:"md5"`
	Sha1      string   `json:"sha1"`
	Sha256    string   `json:"sha256"`
	Urls      []string `json:"urls"`
	ChunkSize int64    `json:"chunksize"`
	// use this to massive-delete message without inferring others
	MessageIDs map[string][]string `json:"messageids"`
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
	root := f.root
	gid := f.chunkChannels[0].GuildID
	if root == "/" || root == "" {
		return fmt.Sprintf("Discord server (%s) root", gid)
	}
	return fmt.Sprintf("Discord server (%s) path %s", gid, root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Hashes returns type of hashes that are supposed to be written
func (f *Fs) Hashes() hash.Set {
	return hash.NewHashSet(hash.MD5, hash.SHA1, hash.SHA256)
}

// Precision returns the precision of mtime that the server responds
func (f *Fs) Precision() time.Duration {
	return time.Nanosecond
}

// NewFs constructs an Fs from the path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	f := &Fs{
		name: name,
		opt:  *opt,
		ctx:  ctx,
		root: root,
		srv:  rest.NewClient(fshttp.NewClient(ctx)),
	}
	f.bot, err = discordgo.New("Bot " + opt.AuthToken)
	if err != nil {
		return nil, err
	}
	f.features = (&fs.Features{
		BucketBased:       true,
		BucketBasedRootOK: true, // there are no limit for path as long as json accepts it
	}).Fill(ctx, f)

	f.chunkChannels, err = fetchChannels(f.bot, opt.ChunksChannel)
	if err != nil {
		return nil, err
	}
	f.journalChannels, err = fetchChannels(f.bot, opt.JournalChannel)
	if err != nil {
		return nil, err
	}
	f.entryCache = make(map[string]*Object)

	// test if the root exists as a file
	_, err = f.NewObject(ctx, "/")
	if err == nil {
		f.root = betterPathDir(root)
		return f, fs.ErrorIsFile
	}
	return f, nil
}

// Shutdown releases all resources held by this backend
func (f *Fs) Shutdown(ctx context.Context) error {
	return f.bot.Close()
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// ModTime is the last modified time (read-only)
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// Size is the file length
func (o *Object) Size() int64 {
	return o.meta.Size
}

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Hash returns the hash value presented in the metadata
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	if ty == hash.MD5 {
		return o.meta.Md5, nil
	}
	if ty == hash.SHA1 {
		return o.meta.Sha1, nil
	}
	if ty == hash.SHA256 {
		return o.meta.Sha256, nil
	}
	return "", hash.ErrUnsupported
}

// Storable returns if this object is storable
func (o *Object) Storable() bool {
	return true
}

// SetModTime sets modTime on a particular file
func (o *Object) SetModTime(ctx context.Context, t time.Time) (err error) {
	o.modTime = t
	o.meta.ModTime = t.Format(time.RFC3339Nano)
	return o.amendMetadata(*o.meta, true)
}

// List files and directories in a directory
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	grandparent := strings.TrimRight(path.Join(f.root, dir), "/") + "/"
	if grandparent == "/" {
		grandparent = ""
	}

	entryChan := make(chan fs.DirEntry)
	go func() {
		err = f.streamAllObjects(entryChan)
	}()
	for ent := range entryChan {
		obj, ok := ent.(*Object)
		if ok && strings.HasPrefix(obj.remote, grandparent) {
			path := trimPathPrefix(obj.remote, grandparent)
			newRemote := trimPathPrefix(obj.remote, f.root)
			if !strings.Contains(path, "/") && newRemote != "" {
				obj.remote = newRemote
				entries = append(entries, obj)
				f.insertObjectCache(obj)
			}
		}
		dire, ok := ent.(*fs.Dir)
		if ok && strings.HasPrefix(dire.Remote(), grandparent) {
			path := trimPathPrefix(dire.Remote(), grandparent)
			newRemote := trimPathPrefix(dire.Remote(), f.root)
			if !strings.Contains(path, "/") && newRemote != "" {
				dire.SetRemote(newRemote)
				entries = append(entries, dire)
			}
		}
	}

	return entries, err
}

// Mkdir is not supported
func (f *Fs) Mkdir(ctx context.Context, dir string) (err error) {
	return nil
}

// Rmdir as well. directories are based
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (ret fs.Object, err error) {
	if obj := f.searchObjectCache(remote); obj != nil {
		return obj, nil
	}

	grandparent := strings.TrimRight(path.Join(f.root, remote), "/")

	entryChan := make(chan fs.DirEntry)
	go func() {
		err = f.streamAllObjects(entryChan)
	}()
	for ent := range entryChan {
		obj, ok := ent.(*Object)
		if ok && obj.remote == grandparent {
			obj.remote = trimPathPrefix(obj.remote, f.root)
			f.insertObjectCache(obj)
			return obj, nil
		}
	}

	if err == nil {
		err = fs.ErrorObjectNotFound
	}
	return nil, err
}

// Put uploads a file
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o, err := f.NewObject(ctx, src.Remote())
	if err != nil {
		o = &Object{
			fs:      f,
			remote:  src.Remote(),
			modTime: src.ModTime(ctx),
			// Update() will fill missing fields
		}
	} else if o2, ok := o.(*Object); ok {
		o2.modTime = src.ModTime(ctx)
	}

	err = o.Update(ctx, in, src, options...)
	return o, err
}

// PutUnchecked puts in to the remote path with the modTime given of the given size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
//
// May create duplicates or return errors if src already
// exists.
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: src.Remote(),
		// Update() will fill missing fields
	}

	return o, o.Update(ctx, in, src, options...)
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// Put and its relatives are already aware of size == -1
	return f.Put(ctx, in, src, options...)
}

// Move src to this remote using server-side move operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantMove
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	return f.copyOrMove(src, remote, false)
}

// Copy src to this remote using server-side copy operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	return f.copyOrMove(src, remote, true)
}

func (f *Fs) copyOrMove(src fs.Object, remote string, copy bool) (_ fs.Object, err error) {
	cant := fs.ErrorCantMove
	if copy {
		cant = fs.ErrorCantCopy
	}
	dstPath := path.Join(f.root, remote)
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy/move - not same remote type")
		return nil, cant
	}
	srcPath := path.Join(srcObj.fs.root, srcObj.remote)

	if dstPath == srcPath {
		// amending metadata doesn't make sense
		fs.Debugf(src, "Can't copy/move - the source and destination files cannot be the same!")
		return nil, cant
	}

	// copy source and overwrite some values
	ret := *srcObj //nolint:govet
	if copy {
		// prevent amendMetadata from deleting original message
		ret.messageID = ""
		ret.metadataChannelID = ""
	}
	ret.metaMu = sync.RWMutex{}
	newMeta := *ret.meta
	newMeta.FileName = dstPath
	ret.remote = remote
	return &ret, ret.amendMetadata(newMeta, !copy)
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
//
// Don't implement this unless you have a more efficient way
// of listing recursively than doing a directory traversal.
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) (err error) {
	var entries fs.DirEntries
	grandparent := strings.TrimRight(path.Join(f.root, dir), "/") + "/"
	if grandparent == "/" {
		grandparent = ""
	}

	entryChan := make(chan fs.DirEntry)
	go func() {
		err = f.streamAllObjects(entryChan)
	}()
	for ent := range entryChan {
		obj, ok := ent.(*Object)
		if ok && strings.HasPrefix(obj.remote, grandparent) && obj.remote != f.root {
			obj.remote = trimPathPrefix(obj.remote, f.root)
			entries = append(entries, obj)
			f.insertObjectCache(obj)
		}
		dire, ok := ent.(*fs.Dir)
		if ok && strings.HasPrefix(dire.Remote(), grandparent) && dire.Remote() != f.root {
			dire.SetRemote(trimPathPrefix(dire.Remote(), f.root))
			entries = append(entries, dire)
		}
	}

	if err == nil {
		err = callback(entries)
	}
	return err
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	var openOptions []fs.OpenOption
	var offset, limit int64 = 0, -1

	for _, option := range options {
		switch opt := option.(type) {
		case *fs.SeekOption:
			offset = opt.Offset
		case *fs.RangeOption:
			offset, limit = opt.Decode(o.meta.Size)
		default:
			// pass Options on to the wrapped open, if appropriate
			openOptions = append(openOptions, option)
		}
	}

	if offset < 0 {
		return nil, errors.New("invalid offset")
	}
	if limit < 0 {
		limit = o.meta.Size - offset
	}

	return o.newLinearReader(ctx, offset, limit, openOptions)
}

// Update the Object from in with modTime and size
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	if o.meta == nil {
		o.meta = &JournalMetadata{
			FileName: o.fs.opt.Enc.FromStandardPath(path.Join(o.fs.root, o.remote)),
		}
	}
	o.meta.Size = src.Size()
	// TODO: vary by server tier
	o.meta.ChunkSize = chunkSize
	o.modTime = src.ModTime(ctx)
	o.meta.ModTime = o.modTime.Format(time.RFC3339Nano)
	// we're always overwriting it, removing previous data
	if !o.fs.opt.QuickDelete {
		o.massiveDeleteChunkMessages() //nolint:errcheck
	}
	o.metaMu.Lock()
	o.meta.Urls = nil
	o.meta.MessageIDs = make(map[string][]string)
	o.metaMu.Unlock()

	mher, err := hash.NewMultiHasherTypes(o.fs.Hashes())
	in = io.TeeReader(in, mher)
	c := newChunkingReader(in, chunkSize, o.meta.Size)

	defer func() {
		if err != nil {
			// cleanup
			o.Remove(ctx) //nolint:errcheck
		}
	}()

	var message *discordgo.Message
	for !c.done {
		newChannelID := randomPick(o.fs.chunkChannels).ID
		buf := make([]byte, chunkSize)

		// fully read to allow retry
		var r int
		if r, err = io.ReadFull(c, buf); err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
			return err
		}
		if r == 0 {
			// do not upload empty file, hopefully c.done == true
			continue
		}
		buf = buf[:r]
		err = retry(func() (err error) {
			message, err = o.fs.bot.ChannelMessageSendComplex(newChannelID, &discordgo.MessageSend{
				Content: o.fs.opt.ChunksMessage,
				Files: []*discordgo.File{{
					Name:   random.String(32),
					Reader: bytes.NewReader(buf),
				}},
			})
			return
		}, 10)
		if err != nil {
			return err
		}
		// refill read limit
		c.chunkLimit = c.chunkSize

		// add chunk into metadata
		o.metaMu.Lock()
		appendMessageMap(o.meta.MessageIDs, newChannelID, message.ID)
		for _, atc := range message.Attachments {
			o.meta.Urls = append(o.meta.Urls, atc.URL)
		}
		o.metaMu.Unlock()
	}

	if o.meta.Size < 0 {
		// fill in number of bytes read reported by chunkingReader
		o.meta.Size = c.readCount
	}

	o.meta.Sha1, _ = mher.SumString(hash.SHA1, false)
	o.meta.Sha256, _ = mher.SumString(hash.SHA256, false)
	o.meta.Md5, _ = mher.SumString(hash.MD5, false)
	// amend or create metadata with new journal
	err = o.amendMetadata(*o.meta, true)
	if err == nil {
		o.fs.insertObjectCache(o)
	}
	return err
}

// Remove an object
func (o *Object) Remove(ctx context.Context) (err error) {
	// remove metadata first
	if o.metadataChannelID != "" && o.messageID != "" {
		err = o.fs.bot.ChannelMessageDelete(o.metadataChannelID, o.messageID)
	}
	if err != nil {
		return
	}
	// delete all involving messages
	if o.fs.opt.QuickDelete {
		return nil
	}
	return o.massiveDeleteChunkMessages()
}

// String converts this Fs to a string
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// find all files/directories within journal channels
func (f *Fs) streamAllObjects(entryChan chan fs.DirEntry) (err error) {
	defer close(entryChan)
	errorChan := make(chan error)
	messageChan := make(chan *discordgo.Message)
	timeoutChan := make(<-chan time.Time)
	crawlCount := 0
	for _, ch := range f.journalChannels {
		go crawlMessages(f, ch.ID, messageChan, errorChan)
		crawlCount++
	}

	if f.opt.ListTimeout != 0 {
		timeoutChan = time.After(time.Duration(f.opt.ListTimeout))
	}

	knownDirs := map[string]interface{}{
		"": nil,
	}

	completeCount := 0
outer:
	for completeCount < crawlCount {
		select {
		case res := <-messageChan:
			mes, err := f.buildObjectFromMessage(res)
			if err != nil {
				fs.LogPrintf(fs.LogLevelError, f, "message %s in channel %s has a problem: %v. skipping", res.ID, res.ChannelID, err)
				continue outer
			}
			if mes == nil {
				continue outer
			}
			entryChan <- mes

			// populate children directories (code taken from IA)
			child := strings.TrimRight(betterPathDir(mes.remote), "/")
			for {
				if _, ok := knownDirs[child]; ok {
					break
				}
				// directory
				entryChan <- fs.NewDir(child, time.Unix(0, 0))

				knownDirs[child] = nil
				child = strings.TrimRight(betterPathDir(child), "/")
			}

		case err := <-errorChan:
			completeCount++
			if err != nil {
				return err
			}
		case <-timeoutChan:
			break outer
		}
	}

	return nil
}

func (f *Fs) buildObjectFromMessage(m *discordgo.Message) (*Object, error) {
	meta, err := loadMetadata(f, m)
	if err != nil {
		return nil, err
	}
	t, err := time.Parse(time.RFC3339Nano, meta.ModTime)
	if err != nil {
		return nil, err
	}

	return &Object{
		fs:                f,
		remote:            f.opt.Enc.ToStandardPath(meta.FileName),
		modTime:           t,
		metadataChannelID: m.ChannelID,
		messageID:         m.ID,
		meta:              meta,
	}, nil
}

func (f *Fs) insertObjectCache(o *Object) {
	f.entryLock.Lock()
	defer f.entryLock.Unlock()
	f.entryCache[o.remote] = o
}

func (f *Fs) searchObjectCache(remote string) *Object {
	f.entryLock.RLock()
	defer f.entryLock.RUnlock()
	ret, ok := f.entryCache[remote]
	if !ok {
		ret = nil
	}
	return ret
}

func betterPathDir(p string) string {
	d := path.Dir(p)
	if d == "." {
		return ""
	}
	return d
}

func betterPathClean(p string) string {
	d := path.Clean(p)
	if d == "." {
		return ""
	}
	return d
}

func trimPathPrefix(s, prefix string) string {
	// we need to clean the paths to make tests pass!
	s, prefix = betterPathClean(s), betterPathClean(prefix)
	if s == prefix || s == prefix+"/" {
		return ""
	}
	return strings.TrimPrefix(s, strings.TrimRight(prefix, "/")+"/")
}

var splreg = regexp.MustCompile(`\s+`) // please compile!

func fetchChannels(session *discordgo.Session, s string) (entries []discordgo.Channel, err error) {
	for _, chid := range splreg.Split(s, -1) {
		channel, err := session.Channel(chid)
		if err != nil {
			return entries, err
		}
		entries = append(entries, *channel)
	}
	return entries, nil
}

func loadMetadata(f *Fs, message *discordgo.Message) (jm *JournalMetadata, err error) {
	atc := message.Attachments
	if len(atc) != 1 {
		return nil, fmt.Errorf("message %s has incorrect number of attachments. (%d != 1)", message.ID, len(atc))
	}
	url := atc[0].URL
	opts := rest.Opts{
		Method:  "GET",
		Path:    "",
		RootURL: url,
	}

	var resp *http.Response
	err = retry(func() (err error) {
		resp, err = f.srv.CallJSON(f.ctx, &opts, nil, &jm)
		return
	}, 10)
	if err == nil && resp.StatusCode >= 400 {
		err = fmt.Errorf("status code %d", resp.StatusCode)
	}
	if err != nil {
		return nil, err
	}
	return jm, nil
}

func crawlMessages(f *Fs, channelID string, messageChan chan *discordgo.Message, finOrErrorChan chan error) {
	beforeID := ""
	for {
		messages, err := f.bot.ChannelMessages(channelID, 100, beforeID, "", "")
		if err != nil {
			finOrErrorChan <- err
			return
		}
		for _, m := range messages {
			messageChan <- m
			// the last ID is needed, do that here
			beforeID = m.ID
		}
		if len(messages) < 100 {
			break
		}
	}
	finOrErrorChan <- nil
}

func (o *Object) amendMetadata(jm JournalMetadata, deleteOld bool) error {
	bot := o.fs.bot
	f := o.fs
	newChannelID := randomPick(f.journalChannels).ID
	encoded, err := json.Marshal(jm)
	if err != nil {
		return err
	}
	var message *discordgo.Message
	err = retry(func() (err error) {
		message, err = o.fs.bot.ChannelMessageSendComplex(newChannelID, &discordgo.MessageSend{
			Content: o.fs.opt.JournalMessage,
			Files: []*discordgo.File{{
				Name:   "file-metadata.json",
				Reader: bytes.NewReader(encoded),
			}},
		})
		return
	}, 10)
	if err != nil {
		return err
	}
	if o.metadataChannelID != "" && o.messageID != "" && deleteOld {
		err := bot.ChannelMessageDelete(o.metadataChannelID, o.messageID)
		if err != nil {
			fs.LogPrintf(fs.LogLevelError, f, "failed to delete old message for amending: %v", err)
		}
	}
	o.metadataChannelID = newChannelID
	o.messageID = message.ID
	o.meta = &jm
	return nil
}

func (o *Object) massiveDeleteChunkMessages() (err error) {
	if o.meta == nil {
		return nil
	}
	if o.meta.MessageIDs == nil {
		return nil
	}
	o.metaMu.RLock()
	defer o.metaMu.RUnlock()
	after := func(sl []string, length int) []string {
		slen := len(sl)
		if slen < length {
			return []string{}
		}
		return sl[length:]
	}
	for ch, mes := range o.meta.MessageIDs {
		for len(mes) > 0 {
			erra := o.fs.bot.ChannelMessagesBulkDelete(ch, mes)
			if erra != nil && err == nil {
				err = erra
			}
			mes = after(mes, 100)
		}
	}
	return
}

func randomPick(s []discordgo.Channel) discordgo.Channel {
	return s[rand.Intn(len(s))]
}

func appendMessageMap(mmm map[string][]string, channel, message string) {
	if value, ok := mmm[channel]; ok {
		mmm[channel] = append(value, message)
	} else {
		mmm[channel] = []string{message}
	}
}

func retry(fn func() error, count int) (err error) {
	attempt := 0
	for attempt < count {
		err = fn()
		if err == nil {
			return
		}
		attempt++
	}
	return
}

// stripped down version of chunker.chunkingReader for its need

type chunkingReader struct {
	baseReader   io.Reader
	sizeTotal    int64
	sizeLeft     int64
	readCount    int64
	chunkSize    int64
	chunkLimit   int64
	chunkNo      int
	err          error
	done         bool
	expectSingle bool
	smallHead    []byte
}

func newChunkingReader(base io.Reader, chunkSize, totalSize int64) *chunkingReader {
	c := &chunkingReader{
		baseReader: base,
		chunkSize:  chunkSize,
		sizeTotal:  totalSize,
	}
	c.chunkLimit = c.chunkSize
	c.sizeLeft = c.sizeTotal
	c.expectSingle = c.sizeTotal >= 0 && c.sizeTotal <= c.chunkSize
	return c
}

// Note: unlike chunker's case, it doesn't skip by hash
func (c *chunkingReader) Read(buf []byte) (bytesRead int, err error) {
	if c.chunkLimit <= 0 {
		// Chunk complete - switch to next one.
		// Note #1:
		// We might not get here because some remotes (e.g. box multi-uploader)
		// read the specified size exactly and skip the concluding EOF Read.
		// Then a check in the put loop will kick in.
		// Note #2:
		// The crypt backend after receiving EOF here will call Read again
		// and we must insist on returning EOF, so we postpone refilling
		// chunkLimit to the main loop.
		return 0, io.EOF
	}
	if int64(len(buf)) > c.chunkLimit {
		buf = buf[0:c.chunkLimit]
	}
	bytesRead, err = c.baseReader.Read(buf)
	if err != nil && err != io.EOF {
		c.err = err
		c.done = true
		return
	}
	c.accountBytes(int64(bytesRead))
	if c.chunkNo == 0 && c.expectSingle && bytesRead > 0 {
		c.smallHead = append(c.smallHead, buf[:bytesRead]...)
	}
	if bytesRead == 0 && c.sizeLeft == 0 {
		err = io.EOF // Force EOF when no data left.
	}
	if err == io.EOF {
		c.done = true
	}
	return
}

func (c *chunkingReader) accountBytes(bytesRead int64) {
	c.readCount += bytesRead
	c.chunkLimit -= bytesRead
	if c.sizeLeft != -1 {
		c.sizeLeft -= bytesRead
	}
}

func (c *chunkingReader) Close() error { return nil }

// taken from chunker's linearReader

// linearReader opens and reads file chunks sequentially, without read-ahead
type linearReader struct {
	ctx     context.Context
	chunks  []string
	options []fs.OpenOption
	limit   int64
	count   int64
	pos     int
	reader  io.ReadCloser
	err     error

	fs *Fs
	o  *Object
}

func (o *Object) newLinearReader(ctx context.Context, offset, limit int64, options []fs.OpenOption) (io.ReadCloser, error) {
	r := &linearReader{
		ctx:     ctx,
		chunks:  o.meta.Urls,
		options: options,
		limit:   limit,
		fs:      o.fs,
		o:       o,
	}

	// skip to chunk for given offset
	err := io.EOF
	for offset >= 0 && err != nil {
		offset, err = r.nextChunk(offset)
	}
	if err == nil || err == io.EOF {
		r.err = err
		return r, nil
	}
	return nil, err
}

func (r *linearReader) nextChunk(offset int64) (int64, error) {
	if r.err != nil {
		return -1, r.err
	}
	if r.pos >= len(r.chunks) || r.limit <= 0 || offset < 0 {
		return -1, io.EOF
	}

	chunk := r.chunks[r.pos]
	count := int64min(r.o.meta.ChunkSize, r.o.meta.Size-r.o.meta.ChunkSize*int64(r.pos))
	r.pos++

	if offset >= count {
		return offset - count, io.EOF
	}
	count -= offset
	if r.limit < count {
		count = r.limit
	}

	if err := r.Close(); err != nil {
		return -1, err
	}

	opts := rest.Opts{
		Method:  "GET",
		Path:    "",
		RootURL: chunk,
		Options: r.options,
	}
	resp, err := r.fs.srv.Call(r.ctx, &opts)
	if err != nil {
		return -1, err
	}

	r.reader = resp.Body
	// cdn.discordapp.com doesn't support Range header so we'll mimick it here
	err = dummyRead(r.reader, offset)
	// abuse chunkingReader here to limit length of Reader; io.LimitReader isn't ReadCloser
	r.reader = newChunkingReader(r.reader, count, count)
	r.count = count
	return offset, err
}

func (r *linearReader) Read(p []byte) (n int, err error) {
	if r.err != nil {
		return 0, r.err
	}
	if r.limit <= 0 {
		r.err = io.EOF
		return 0, io.EOF
	}

	for r.count <= 0 {
		// current chunk has been read completely or its size is zero
		off, err := r.nextChunk(0)
		if off < 0 {
			r.err = err
			return 0, err
		}
	}

	n, err = r.reader.Read(p)
	if err == nil || err == io.EOF {
		r.count -= int64(n)
		r.limit -= int64(n)
		if r.limit > 0 {
			err = nil // more data to read
		}
	}
	r.err = err
	return
}

func (r *linearReader) Close() (err error) {
	if r.reader != nil {
		err = r.reader.Close()
		r.reader = nil
	}
	return
}

// dummyRead advances in by specified count of bytes
func dummyRead(in io.Reader, size int64) error {
	const bufLen = 1048576 // 1 MiB
	buf := make([]byte, bufLen)
	for size > 0 {
		n := size
		if n > bufLen {
			n = bufLen
		}
		if _, err := io.ReadFull(in, buf[0:n]); err != nil {
			return err
		}
		size -= n
	}
	return nil
}

func int64min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

var (
	_ fs.Fs             = &Fs{}
	_ fs.Copier         = &Fs{}
	_ fs.Mover          = &Fs{}
	_ fs.ListRer        = &Fs{}
	_ fs.PutUncheckeder = &Fs{}
	_ fs.PutStreamer    = &Fs{}
	_ fs.Shutdowner     = &Fs{}
	_ fs.Object         = &Object{}
	_ io.ReadCloser     = &chunkingReader{}
	_ io.ReadCloser     = &linearReader{}
)
