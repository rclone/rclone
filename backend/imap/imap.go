// Package imap provides an interface to IMAP servers for email backup
package imap

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/env"
)

var (
	currentUser = env.CurrentUser()
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "imap",
		Description: "IMAP",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:      "host",
			Help:      "IMAP server host.\n\nE.g. \"imap.example.com\".",
			Required:  true,
			Sensitive: true,
		}, {
			Name:      "user",
			Help:      "IMAP username (usually email address).",
			Default:   currentUser,
			Sensitive: true,
		}, {
			Name:    "port",
			Help:    "IMAP server port.",
			Default: 993,
		}, {
			Name:       "pass",
			Help:       "IMAP password.",
			IsPassword: true,
		}, {
			Name: "tls",
			Help: `Use implicit TLS (IMAPS).

When using implicit TLS the client connects using TLS
right from the start. This is the default for port 993.`,
			Default: true,
		}, {
			Name: "starttls",
			Help: `Use STARTTLS.

When using STARTTLS the client connects in cleartext then
upgrades to TLS. Cannot be used with implicit TLS.`,
			Default: false,
		}, {
			Name:     "no_check_certificate",
			Help:     "Do not verify the TLS certificate of the server.",
			Default:  false,
			Advanced: true,
		}, {
			Name:    "idle_timeout",
			Default: fs.Duration(60 * time.Second),
			Help: `Max time before closing idle connections.

If no connections have been returned to the connection pool in the time
given, rclone will empty the connection pool.

Set to 0 to keep connections indefinitely.
`,
			Advanced: true,
		}, {
			Name:    "ask_password",
			Default: false,
			Help: `Allow asking for IMAP password when needed.

If this is set and no password is supplied then rclone will ask for a password.
`,
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			// IMAP mailbox names can have various restrictions
			Default: encoder.Display,
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	Host              string               `config:"host"`
	User              string               `config:"user"`
	Pass              string               `config:"pass"`
	Port              int                  `config:"port"`
	TLS               bool                 `config:"tls"`
	StartTLS          bool                 `config:"starttls"`
	SkipVerifyTLSCert bool                 `config:"no_check_certificate"`
	IdleTimeout       fs.Duration          `config:"idle_timeout"`
	AskPassword       bool                 `config:"ask_password"`
	Enc               encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote IMAP server
type Fs struct {
	name     string         // name of this remote
	root     string         // the mailbox path we are working on
	opt      Options        // parsed options
	ci       *fs.ConfigInfo // global config
	features *fs.Features   // optional features
	host     string
	user     string
	pass     string
	poolMu   sync.Mutex
	pool     []*imapclient.Client
	drain    *time.Timer // used to drain the pool when we stop using connections
}

// Object describes an IMAP message
type Object struct {
	fs       *Fs
	remote   string    // The remote path (mailbox/filename)
	uid      uint32    // IMAP UID
	size     int64     // Message size
	modTime  time.Time // Internal date
	mailbox  string    // Mailbox containing the message
	mimeType string    // Content type
}

// ------------------------------------------------------------

// Name of this fs
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String returns a description of the FS
func (f *Fs) String() string {
	return fmt.Sprintf("IMAP server %s", f.host)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Precision of the ModTimes in this Fs
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// Hashes are not supported
func (f *Fs) Hashes() hash.Set {
	return 0
}

// tlsConfig returns TLS configuration for the connection
func (f *Fs) tlsConfig() *tls.Config {
	return &tls.Config{
		ServerName:         f.opt.Host,
		InsecureSkipVerify: f.opt.SkipVerifyTLSCert,
	}
}

// getConnection gets an IMAP connection from the pool, or opens a new one
func (f *Fs) getConnection(ctx context.Context) (*imapclient.Client, error) {
	f.poolMu.Lock()
	if len(f.pool) > 0 {
		c := f.pool[0]
		f.pool = f.pool[1:]
		f.poolMu.Unlock()
		return c, nil
	}
	f.poolMu.Unlock()

	// Open a new connection
	return f.newConnection(ctx)
}

// newConnection creates a new IMAP connection
func (f *Fs) newConnection(ctx context.Context) (*imapclient.Client, error) {
	fs.Debugf(f, "Connecting to IMAP server %s:%d", f.opt.Host, f.opt.Port)

	addr := fmt.Sprintf("%s:%d", f.opt.Host, f.opt.Port)
	var c *imapclient.Client
	var err error

	if f.opt.StartTLS && !f.opt.TLS {
		// Use STARTTLS connection
		options := &imapclient.Options{
			TLSConfig: f.tlsConfig(),
		}
		c, err = imapclient.DialStartTLS(addr, options)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to IMAP server with STARTTLS: %w", err)
		}
	} else if f.opt.TLS {
		// Use implicit TLS connection
		options := &imapclient.Options{
			TLSConfig: f.tlsConfig(),
		}
		c, err = imapclient.DialTLS(addr, options)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to IMAP server with TLS: %w", err)
		}
	} else {
		// Use insecure connection
		c, err = imapclient.DialInsecure(addr, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to IMAP server: %w", err)
		}
	}

	// Login
	if err := c.Login(f.user, f.pass).Wait(); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("failed to login: %w", err)
	}

	return c, nil
}

// putConnection returns an IMAP connection to the pool
func (f *Fs) putConnection(pc **imapclient.Client, err error) {
	if pc == nil {
		return
	}
	c := *pc
	if c == nil {
		return
	}
	*pc = nil

	if err != nil {
		// Connection may be bad, close it
		_ = c.Close()
		return
	}

	f.poolMu.Lock()
	f.pool = append(f.pool, c)
	if f.opt.IdleTimeout > 0 {
		f.drain.Reset(time.Duration(f.opt.IdleTimeout))
	}
	f.poolMu.Unlock()
}

// drainPool closes all connections in the pool
func (f *Fs) drainPool(ctx context.Context) error {
	f.poolMu.Lock()
	defer f.poolMu.Unlock()
	if f.opt.IdleTimeout > 0 {
		f.drain.Stop()
	}
	if len(f.pool) != 0 {
		fs.Debugf(f, "closing %d unused connections", len(f.pool))
	}
	var lastErr error
	for i, c := range f.pool {
		if err := c.Logout().Wait(); err != nil {
			lastErr = err
		}
		_ = c.Close()
		f.pool[i] = nil
	}
	f.pool = nil
	return lastErr
}

// NewFs constructs an Fs from the path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	pass := ""
	if opt.AskPassword && opt.Pass == "" {
		pass = config.GetPassword("IMAP server password")
	} else {
		pass, err = obscure.Reveal(opt.Pass)
		if err != nil {
			return nil, fmt.Errorf("NewFs decrypt password: %w", err)
		}
	}

	user := opt.User
	if user == "" {
		user = currentUser
	}

	if opt.TLS && opt.StartTLS {
		return nil, errors.New("implicit TLS and STARTTLS are mutually exclusive")
	}

	ci := fs.GetConfig(ctx)
	f := &Fs{
		name: name,
		root: strings.Trim(root, "/"),
		opt:  *opt,
		ci:   ci,
		host: opt.Host,
		user: user,
		pass: pass,
	}

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		ReadMimeType:            true,
	}).Fill(ctx, f)

	// Set up the pool drainer
	if f.opt.IdleTimeout > 0 {
		f.drain = time.AfterFunc(time.Duration(opt.IdleTimeout), func() { _ = f.drainPool(ctx) })
	}

	// Make a connection to verify credentials
	c, err := f.getConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("NewFs: %w", err)
	}
	f.putConnection(&c, nil)

	// Check if root is a mailbox or a message
	if root != "" {
		// Check if it's a mailbox
		mailbox, msgName := splitPath(root)
		if msgName != "" {
			// It might be a file, check if parent mailbox exists
			c, err := f.getConnection(ctx)
			if err != nil {
				return nil, err
			}
			defer f.putConnection(&c, err)

			_, err = c.Select(mailbox, &imap.SelectOptions{ReadOnly: true}).Wait()
			if err == nil {
				// Parent mailbox exists, check if it's a file
				uid := parseUID(msgName)
				if uid > 0 {
					// Try to find the message
					seqSet := imap.UIDSet{}
					seqSet.AddNum(imap.UID(uid))
					fetchOptions := &imap.FetchOptions{
						Flags:        true,
						InternalDate: true,
						RFC822Size:   true,
					}
					msgs, err := c.Fetch(seqSet, fetchOptions).Collect()
					if err == nil && len(msgs) > 0 {
						// It's a file, return fs pointing to parent
						f.root = mailbox
						return f, fs.ErrorIsFile
					}
				}
			}
		}
	}

	return f, nil
}

// splitPath splits a path into mailbox and message name
func splitPath(p string) (mailbox, msgName string) {
	p = strings.Trim(p, "/")
	idx := strings.LastIndex(p, "/")
	if idx < 0 {
		return p, ""
	}
	return p[:idx], p[idx+1:]
}

// parseUID extracts UID from a filename
// Filename format: TIMESTAMP.UID.FLAGS or just UID
func parseUID(name string) uint32 {
	// Try to extract UID from maildir-like filename
	// Format: timestamp.uid.hostname,S=size;2,flags
	parts := strings.Split(name, ".")
	if len(parts) >= 2 {
		// Try the second part as UID
		if uid, err := strconv.ParseUint(parts[1], 10, 32); err == nil {
			return uint32(uid)
		}
	}
	// Try the whole name as UID
	if uid, err := strconv.ParseUint(name, 10, 32); err == nil {
		return uint32(uid)
	}
	// Try to extract UID from filename with R prefix (maildir format)
	re := regexp.MustCompile(`\.R([0-9a-f]+)\.`)
	matches := re.FindStringSubmatch(name)
	if len(matches) > 1 {
		// This is a hash, not directly usable as UID
		return 0
	}
	return 0
}

// makeFileName creates a filename from message info
// Uses semi-colon instead of colon for Windows compatibility
func makeFileName(uid uint32, size int64, timestamp time.Time, flags []imap.Flag) string {
	flagStr := ""
	for _, flag := range flags {
		switch flag {
		case imap.FlagSeen:
			flagStr += "S"
		case imap.FlagAnswered:
			flagStr += "R"
		case imap.FlagFlagged:
			flagStr += "F"
		case imap.FlagDeleted:
			flagStr += "T"
		case imap.FlagDraft:
			flagStr += "D"
		}
	}
	// Format: timestamp.Ruid.hostname,S=size;2,flags
	// Using semi-colon instead of colon for Windows compatibility
	return fmt.Sprintf("%d.R%d.rclone,S=%d;2,%s", timestamp.Unix(), uid, size, flagStr)
}

// List the objects and directories in dir into entries
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	c, err := f.getConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("List: %w", err)
	}
	defer f.putConnection(&c, err)

	mailboxPath := path.Join(f.root, dir)
	if mailboxPath == "" {
		mailboxPath = "*"
	}

	// List mailboxes (subdirectories)
	if dir == "" || !strings.Contains(mailboxPath, "/") {
		// List all mailboxes
		pattern := "*"
		if mailboxPath != "*" {
			pattern = mailboxPath + "/*"
		}
		listCmd := c.List("", pattern, nil)
		mailboxes, err := listCmd.Collect()
		if err != nil {
			return nil, fmt.Errorf("failed to list mailboxes: %w", err)
		}

		// Also list the root level
		if mailboxPath == "*" {
			rootList := c.List("", "*", nil)
			rootMailboxes, err := rootList.Collect()
			if err == nil {
				mailboxes = append(mailboxes, rootMailboxes...)
			}
		}

		seen := make(map[string]bool)
		for _, mb := range mailboxes {
			// Skip mailboxes that don't start with our root
			mbName := mb.Mailbox
			if f.root != "" && !strings.HasPrefix(mbName, f.root) {
				continue
			}
			// Get relative path
			relPath := strings.TrimPrefix(mbName, f.root)
			relPath = strings.TrimPrefix(relPath, "/")

			// Only show direct children
			if dir != "" {
				if !strings.HasPrefix(relPath, dir+"/") && relPath != dir {
					continue
				}
				relPath = strings.TrimPrefix(relPath, dir+"/")
			}

			if relPath == "" || strings.Contains(relPath, "/") {
				// Skip if it's the current dir or a nested path
				parts := strings.Split(relPath, "/")
				if len(parts) > 1 {
					relPath = parts[0]
				}
			}

			if relPath == "" || seen[relPath] {
				continue
			}
			seen[relPath] = true

			d := fs.NewDir(relPath, time.Time{})
			entries = append(entries, d)
		}
	}

	// List messages in the current mailbox
	currentMailbox := mailboxPath
	if currentMailbox == "*" {
		currentMailbox = "INBOX"
	}

	selectCmd, err := c.Select(currentMailbox, &imap.SelectOptions{ReadOnly: true}).Wait()
	if err != nil {
		// Mailbox doesn't exist, just return what we have
		if len(entries) > 0 {
			return entries, nil
		}
		return nil, fs.ErrorDirNotFound
	}

	if selectCmd.NumMessages == 0 {
		return entries, nil
	}

	// Fetch all messages
	seqSet := imap.SeqSet{}
	seqSet.AddRange(1, selectCmd.NumMessages)

	fetchOptions := &imap.FetchOptions{
		Flags:        true,
		InternalDate: true,
		RFC822Size:   true,
		UID:          true,
		Envelope:     true,
	}

	msgs, err := c.Fetch(seqSet, fetchOptions).Collect()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch messages: %w", err)
	}

	for _, msg := range msgs {
		fileName := makeFileName(uint32(msg.UID), int64(msg.RFC822Size), msg.InternalDate, msg.Flags)
		o := &Object{
			fs:      f,
			remote:  path.Join(dir, fileName),
			uid:     uint32(msg.UID),
			size:    int64(msg.RFC822Size),
			modTime: msg.InternalDate,
			mailbox: currentMailbox,
		}
		entries = append(entries, o)
	}

	return entries, nil
}

// NewObject finds the Object at remote
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	mailbox, msgName := splitPath(path.Join(f.root, remote))
	if msgName == "" {
		return nil, fs.ErrorIsDir
	}

	c, err := f.getConnection(ctx)
	if err != nil {
		return nil, err
	}
	defer f.putConnection(&c, err)

	_, err = c.Select(mailbox, &imap.SelectOptions{ReadOnly: true}).Wait()
	if err != nil {
		return nil, fs.ErrorObjectNotFound
	}

	// Try to parse UID from filename
	uid := parseUID(msgName)
	if uid == 0 {
		return nil, fs.ErrorObjectNotFound
	}

	seqSet := imap.UIDSet{}
	seqSet.AddNum(imap.UID(uid))

	fetchOptions := &imap.FetchOptions{
		Flags:        true,
		InternalDate: true,
		RFC822Size:   true,
		UID:          true,
	}

	msgs, err := c.Fetch(seqSet, fetchOptions).Collect()
	if err != nil || len(msgs) == 0 {
		return nil, fs.ErrorObjectNotFound
	}

	msg := msgs[0]
	return &Object{
		fs:      f,
		remote:  remote,
		uid:     uint32(msg.UID),
		size:    int64(msg.RFC822Size),
		modTime: msg.InternalDate,
		mailbox: mailbox,
	}, nil
}

// Put uploads a message to the mailbox
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	mailbox, _ := splitPath(path.Join(f.root, remote))
	if mailbox == "" {
		mailbox = "INBOX"
	}

	c, err := f.getConnection(ctx)
	if err != nil {
		return nil, err
	}
	defer f.putConnection(&c, err)

	// Read the message content
	data, err := io.ReadAll(in)
	if err != nil {
		return nil, fmt.Errorf("failed to read message: %w", err)
	}

	// Append the message
	appendOptions := &imap.AppendOptions{
		Flags: []imap.Flag{},
	}
	appendCmd := c.Append(mailbox, int64(len(data)), appendOptions)
	if _, err := appendCmd.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write message: %w", err)
	}
	if err := appendCmd.Close(); err != nil {
		return nil, fmt.Errorf("failed to close append: %w", err)
	}

	resp, err := appendCmd.Wait()
	if err != nil {
		return nil, fmt.Errorf("failed to append message: %w", err)
	}

	var uid uint32
	if resp.UID != 0 {
		uid = uint32(resp.UID)
	}

	return &Object{
		fs:      f,
		remote:  remote,
		uid:     uid,
		size:    int64(len(data)),
		modTime: src.ModTime(ctx),
		mailbox: mailbox,
	}, nil
}

// Mkdir creates the mailbox
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	mailboxPath := path.Join(f.root, dir)
	if mailboxPath == "" {
		return nil
	}

	c, err := f.getConnection(ctx)
	if err != nil {
		return err
	}
	defer f.putConnection(&c, err)

	err = c.Create(mailboxPath, nil).Wait()
	if err != nil {
		// Ignore "already exists" errors
		if !strings.Contains(err.Error(), "ALREADYEXISTS") &&
			!strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("failed to create mailbox: %w", err)
		}
	}
	return nil
}

// Rmdir deletes the mailbox if empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	mailboxPath := path.Join(f.root, dir)
	if mailboxPath == "" {
		return fs.ErrorDirNotFound
	}

	c, err := f.getConnection(ctx)
	if err != nil {
		return err
	}
	defer f.putConnection(&c, err)

	// Check if mailbox is empty
	selectCmd, err := c.Select(mailboxPath, &imap.SelectOptions{ReadOnly: true}).Wait()
	if err != nil {
		return fs.ErrorDirNotFound
	}

	if selectCmd.NumMessages > 0 {
		return fs.ErrorDirectoryNotEmpty
	}

	// Close the mailbox before deleting
	if err := c.Unselect().Wait(); err != nil {
		return fmt.Errorf("failed to unselect mailbox: %w", err)
	}

	err = c.Delete(mailboxPath).Wait()
	if err != nil {
		return fmt.Errorf("failed to delete mailbox: %w", err)
	}
	return nil
}

// Shutdown the backend
func (f *Fs) Shutdown(ctx context.Context) error {
	return f.drainPool(ctx)
}

// ------------------------------------------------------------

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// String version of o
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

// Hash returns the hash of an object
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.size
}

// ModTime returns the modification time of the object
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// SetModTime sets the modification time of the object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	// IMAP doesn't support modifying internal date
	return fs.ErrorCantSetModTime
}

// Storable returns whether this object is storable
func (o *Object) Storable() bool {
	return true
}

// Open opens the object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	c, err := o.fs.getConnection(ctx)
	if err != nil {
		return nil, err
	}

	_, err = c.Select(o.mailbox, &imap.SelectOptions{ReadOnly: true}).Wait()
	if err != nil {
		o.fs.putConnection(&c, err)
		return nil, fmt.Errorf("failed to select mailbox: %w", err)
	}

	seqSet := imap.UIDSet{}
	seqSet.AddNum(imap.UID(o.uid))

	fetchOptions := &imap.FetchOptions{
		BodySection: []*imap.FetchItemBodySection{{}},
	}

	msgs, err := c.Fetch(seqSet, fetchOptions).Collect()
	if err != nil || len(msgs) == 0 {
		o.fs.putConnection(&c, err)
		return nil, fs.ErrorObjectNotFound
	}

	msg := msgs[0]
	o.fs.putConnection(&c, nil)

	// Get the body section using the helper method
	bodySection := &imap.FetchItemBodySection{}
	body := msg.FindBodySection(bodySection)

	if body == nil {
		return nil, errors.New("no message body found")
	}

	return io.NopCloser(bytes.NewReader(body)), nil
}

// Update the object
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	// IMAP messages are immutable, so we need to delete and re-upload
	if err := o.Remove(ctx); err != nil {
		return err
	}

	newObj, err := o.fs.Put(ctx, in, src, options...)
	if err != nil {
		return err
	}

	// Update our fields from the new object
	newO := newObj.(*Object)
	o.uid = newO.uid
	o.size = newO.size
	o.modTime = newO.modTime
	return nil
}

// Remove deletes the object
func (o *Object) Remove(ctx context.Context) error {
	c, err := o.fs.getConnection(ctx)
	if err != nil {
		return err
	}
	defer o.fs.putConnection(&c, err)

	_, err = c.Select(o.mailbox, &imap.SelectOptions{ReadOnly: false}).Wait()
	if err != nil {
		return fmt.Errorf("failed to select mailbox: %w", err)
	}

	seqSet := imap.UIDSet{}
	seqSet.AddNum(imap.UID(o.uid))

	// Mark message as deleted
	storeFlags := &imap.StoreFlags{
		Op:     imap.StoreFlagsAdd,
		Flags:  []imap.Flag{imap.FlagDeleted},
		Silent: true,
	}
	storeCmd := c.Store(seqSet, storeFlags, nil)
	if err := storeCmd.Close(); err != nil {
		return fmt.Errorf("failed to mark message as deleted: %w", err)
	}

	// Expunge to permanently delete
	expungeCmd := c.Expunge()
	if err := expungeCmd.Close(); err != nil {
		return fmt.Errorf("failed to expunge: %w", err)
	}

	return nil
}

// MimeType returns the content type of the object
func (o *Object) MimeType(ctx context.Context) string {
	return "message/rfc822"
}

// Check the interfaces are satisfied
var (
	_ fs.Fs         = &Fs{}
	_ fs.Shutdowner = &Fs{}
	_ fs.Object     = &Object{}
	_ fs.MimeTyper  = &Object{}
)
