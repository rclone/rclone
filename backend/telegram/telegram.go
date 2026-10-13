// Package telegram implements a Telegram MTProto remote.
package telegram

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gotd/td/session"
	gotd "github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/encoder"
	"golang.org/x/term"
)

const (
	historyPageSize = 100
	connectTimeout  = 30 * time.Second
)

// Options defines the configuration for this backend.
type Options struct {
	APIID         int                  `config:"api_id"`
	APIHash       string               `config:"api_hash"`
	Phone         string               `config:"phone"`
	AuthCode      string               `config:"auth_code"`
	TwoFAPassword string               `config:"twofa_password"`
	ChatID        string               `config:"chat_id"`
	Encoding      encoder.MultiEncoder `config:"encoding"`
}

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "telegram",
		Description: "Telegram (MTProto)",
		NewFs:       NewFs,
		Options: []fs.Option{
			{
				Name:     "api_id",
				Help:     "Telegram API ID from https://my.telegram.org.",
				Required: true,
			}, {
				Name:       "api_hash",
				Help:       "Telegram API hash from https://my.telegram.org.",
				Required:   true,
				IsPassword: true,
			}, {
				Name: "phone",
				Help: "Phone number for the first sign-in, in international format. If empty, it is requested on an interactive terminal.",
			}, {
				Name:       "auth_code",
				Help:       "One-time sign-in code sent by Telegram. If empty, it is requested on an interactive terminal.",
				IsPassword: true,
				Advanced:   true,
			}, {
				Name:       "twofa_password",
				Help:       "Telegram two-step verification password, if enabled. If empty, it is requested securely on an interactive terminal.",
				IsPassword: true,
				Advanced:   true,
			}, {
				Name:    "chat_id",
				Help:    "Chat to use: 'me' for Saved Messages or a public username such as '@my_channel'.",
				Default: "me",
			}, {
				Name:       "session_string",
				Help:       "Saved Telegram authorization session.",
				IsPassword: true,
				Advanced:   true,
			}, {
				Name:     "encoding",
				Help:     "The encoding for file names.",
				Default:  encoder.Base | encoder.EncodeBackSlash | encoder.EncodeInvalidUtf8 | encoder.EncodeRightSpace,
				Advanced: true,
			},
		},
	})
}

// Fs represents a flat file system backed by messages in one Telegram chat.
type Fs struct {
	name     string
	root     string
	opt      Options
	features *fs.Features
	client   *gotd.Client
	tg       *tg.Client
	peer     tg.InputPeerClass
	cancel   context.CancelFunc
	done     chan struct{}

	mu     sync.RWMutex
	runErr error
}

// Object describes one document message in Telegram.
type Object struct {
	fs            *Fs
	remote        string
	size          int64
	modTime       time.Time
	messageID     int
	documentID    int64
	accessHash    int64
	fileReference []byte
}

type configSessionStorage struct {
	mapper configmap.Mapper
}

func (s *configSessionStorage) LoadSession(_ context.Context) ([]byte, error) {
	value, ok := s.mapper.Get("session_string")
	if !ok || value == "" {
		return nil, session.ErrNotFound
	}
	value, err := obscure.Reveal(value)
	if err != nil {
		return nil, fmt.Errorf("telegram: reveal stored session: %w", err)
	}
	return []byte(value), nil
}

func (s *configSessionStorage) StoreSession(_ context.Context, data []byte) error {
	value, err := obscure.Obscure(string(data))
	if err != nil {
		return fmt.Errorf("telegram: obscure stored session: %w", err)
	}
	s.mapper.Set("session_string", value)
	return nil
}

func revealConfigSecret(name, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	revealed, err := obscure.Reveal(value)
	if err != nil {
		return "", fmt.Errorf("telegram: reveal %s: %w", name, err)
	}
	return revealed, nil
}

type userAuthenticator struct {
	phone       string
	code        string
	password    string
	interactive bool
}

func (a userAuthenticator) Phone(context.Context) (string, error) {
	phone := strings.TrimSpace(a.phone)
	if phone == "" {
		if !a.interactive {
			return "", errors.New("telegram: phone is required for the first sign-in; configure phone or use an interactive terminal")
		}
		phone = strings.TrimSpace(config.ReadLine("Telegram phone number (international format): "))
	}
	if phone == "" {
		return "", errors.New("telegram: phone is required for the first sign-in")
	}
	return phone, nil
}

func (a userAuthenticator) Password(context.Context) (string, error) {
	password := a.password
	if password == "" {
		if !a.interactive {
			return "", errors.New("telegram: twofa_password is required; configure it or use an interactive terminal")
		}
		password = config.GetPassword("Telegram two-step verification password")
	}
	if password == "" {
		return "", errors.New("telegram: twofa_password is required")
	}
	return password, nil
}

func (a userAuthenticator) Code(context.Context, *tg.AuthSentCode) (string, error) {
	code := strings.TrimSpace(a.code)
	if code == "" {
		if !a.interactive {
			return "", errors.New("telegram: auth_code is required for the first sign-in; configure it or use an interactive terminal")
		}
		code = strings.TrimSpace(config.ReadLine("Telegram sign-in code: "))
	}
	if code == "" {
		return "", errors.New("telegram: auth_code is required for the first sign-in")
	}
	return code, nil
}

func (userAuthenticator) AcceptTermsOfService(context.Context, tg.HelpTermsOfService) error {
	return errors.New("telegram: accepting updated terms of service must be done in the Telegram app")
}

func (userAuthenticator) SignUp(context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, errors.New("telegram: account sign-up is not supported")
}

// NewFs creates a Telegram file system.
func NewFs(ctx context.Context, name, root string, mapper configmap.Mapper) (fs.Fs, error) {
	var opt Options
	if err := configstruct.Set(mapper, &opt); err != nil {
		return nil, err
	}
	var err error
	if opt.APIHash, err = revealConfigSecret("api_hash", opt.APIHash); err != nil {
		return nil, err
	}
	if opt.AuthCode, err = revealConfigSecret("auth_code", opt.AuthCode); err != nil {
		return nil, err
	}
	if opt.TwoFAPassword, err = revealConfigSecret("twofa_password", opt.TwoFAPassword); err != nil {
		return nil, err
	}
	if opt.APIID <= 0 {
		return nil, errors.New("telegram: api_id must be a positive integer")
	}
	if opt.APIHash == "" {
		return nil, errors.New("telegram: api_hash is required")
	}

	f := &Fs{name: name, root: root, opt: opt}
	if err := f.connect(ctx, mapper); err != nil {
		return nil, fmt.Errorf("telegram: connect: %w", err)
	}
	f.features = (&fs.Features{CanHaveEmptyDirectories: false}).Fill(ctx, f)

	// Telegram has no directory hierarchy; a non-empty root can only name a file.
	if root != "" {
		if strings.Contains(root, "/") {
			_ = f.Shutdown(context.Background())
			return nil, fs.ErrorDirNotFound
		}
		_, err := f.NewObject(ctx, root)
		if err == nil {
			f.root = ""
			return f, fs.ErrorIsFile
		}
		_ = f.Shutdown(context.Background())
		if errors.Is(err, fs.ErrorObjectNotFound) {
			return nil, fs.ErrorDirNotFound
		}
		return nil, err
	}
	return f, nil
}

func (f *Fs) connect(ctx context.Context, mapper configmap.Mapper) error {
	storage := &configSessionStorage{mapper: mapper}
	apiHash := f.opt.APIHash
	f.client = gotd.NewClient(f.opt.APIID, apiHash, gotd.Options{SessionStorage: storage})
	f.opt.APIHash = ""
	f.tg = tg.NewClient(f.client)
	clearAuthInputs := func() {
		mapper.Set("auth_code", "")
		mapper.Set("twofa_password", "")
		f.opt.AuthCode = ""
		f.opt.TwoFAPassword = ""
	}

	clientCtx, cancel := context.WithCancel(ctx)
	f.cancel = cancel
	f.done = make(chan struct{})
	ready := make(chan error, 1)
	var readyOnce sync.Once
	signalReady := func(err error) {
		readyOnce.Do(func() { ready <- err })
	}

	go func() {
		err := f.client.Run(clientCtx, func(runCtx context.Context) error {
			status, err := f.client.Auth().Status(runCtx)
			if err != nil {
				signalReady(err)
				return err
			}
			if !status.Authorized {
				flow := auth.NewFlow(userAuthenticator{
					phone:       f.opt.Phone,
					code:        f.opt.AuthCode,
					password:    f.opt.TwoFAPassword,
					interactive: term.IsTerminal(int(os.Stdin.Fd())),
				}, auth.SendCodeOptions{})
				err = f.client.Auth().IfNecessary(runCtx, flow)
				clearAuthInputs()
				if err != nil {
					signalReady(err)
					return err
				}
			} else {
				// Remove stale one-time credentials from the saved remote config.
				clearAuthInputs()
			}

			peer, err := f.resolvePeer(runCtx, f.opt.ChatID)
			if err != nil {
				signalReady(err)
				return err
			}
			f.peer = peer
			signalReady(nil)
			<-runCtx.Done()
			return nil
		})
		f.mu.Lock()
		f.runErr = err
		f.mu.Unlock()
		close(f.done)
		signalReady(err)
	}()

	timer := time.NewTimer(connectTimeout)
	defer timer.Stop()
	select {
	case err := <-ready:
		if err != nil {
			cancel()
			f.waitForClient()
			return err
		}
		if err = f.clientError(); err != nil {
			cancel()
			f.waitForClient()
			return err
		}
		return nil
	case <-f.done:
		return f.clientRunError()
	case <-ctx.Done():
		cancel()
		f.waitForClient()
		return ctx.Err()
	case <-timer.C:
		cancel()
		f.waitForClient()
		return errors.New("timed out waiting for Telegram sign-in")
	}
}

func (f *Fs) waitForClient() {
	if f.done != nil {
		<-f.done
	}
}

func (f *Fs) clientRunError() error {
	f.mu.RLock()
	err := f.runErr
	f.mu.RUnlock()
	if err == nil {
		return errors.New("telegram: client stopped unexpectedly")
	}
	return fmt.Errorf("telegram: client stopped: %w", err)
}

func (f *Fs) clientError() error {
	select {
	case <-f.done:
		return f.clientRunError()
	default:
		return nil
	}
}

func (f *Fs) resolvePeer(ctx context.Context, chatID string) (tg.InputPeerClass, error) {
	username, err := normalizeChatTarget(chatID)
	if err != nil {
		return nil, err
	}
	if username == "" {
		return &tg.InputPeerSelf{}, nil
	}

	resolved, err := f.tg.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{Username: username})
	if err != nil {
		return nil, err
	}
	switch peer := resolved.Peer.(type) {
	case *tg.PeerUser:
		for _, item := range resolved.Users {
			if user, ok := item.(*tg.User); ok && user.ID == peer.UserID {
				return &tg.InputPeerUser{UserID: user.ID, AccessHash: user.AccessHash}, nil
			}
		}
	case *tg.PeerChannel:
		for _, item := range resolved.Chats {
			if channel, ok := item.(*tg.Channel); ok && channel.ID == peer.ChannelID {
				return &tg.InputPeerChannel{ChannelID: channel.ID, AccessHash: channel.AccessHash}, nil
			}
		}
	case *tg.PeerChat:
		return &tg.InputPeerChat{ChatID: peer.ChatID}, nil
	}
	return nil, fmt.Errorf("telegram: username %q did not resolve to a supported chat", chatID)
}

func normalizeChatTarget(chatID string) (string, error) {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" || chatID == "me" {
		return "", nil
	}
	if _, err := strconv.ParseInt(chatID, 10, 64); err == nil {
		return "", errors.New("telegram: numeric chat IDs are not supported; use 'me' or a public username")
	}
	username := strings.TrimPrefix(chatID, "@")
	if username == "" || strings.HasPrefix(username, "@") || strings.ContainsAny(username, "/\\ \t\r\n") {
		return "", fmt.Errorf("telegram: invalid chat username %q", chatID)
	}
	return username, nil
}

type historyGetter interface {
	MessagesGetHistory(context.Context, *tg.MessagesGetHistoryRequest) (tg.MessagesMessagesClass, error)
}

func collectMessages(ctx context.Context, peer tg.InputPeerClass, getter historyGetter) ([]*tg.Message, error) {
	var messages []*tg.Message
	seenIDs := make(map[int]struct{})
	offsetID := 0
	for {
		result, err := getter.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer:     peer,
			OffsetID: offsetID,
			Limit:    historyPageSize,
		})
		if err != nil {
			return nil, err
		}
		modified, ok := result.AsModified()
		if !ok {
			return nil, fmt.Errorf("telegram: unexpected history response %T", result)
		}
		page := modified.GetMessages()
		if len(page) == 0 {
			break
		}

		oldestID := 0
		for _, item := range page {
			message, ok := item.(*tg.Message)
			if !ok {
				continue
			}
			if _, exists := seenIDs[message.ID]; exists {
				continue
			}
			seenIDs[message.ID] = struct{}{}
			messages = append(messages, message)
			if oldestID == 0 || message.ID < oldestID {
				oldestID = message.ID
			}
		}
		if len(page) < historyPageSize || oldestID == 0 || oldestID == offsetID {
			break
		}
		offsetID = oldestID
	}
	return messages, nil
}

func (f *Fs) listMessages(ctx context.Context) ([]*tg.Message, error) {
	if err := f.clientError(); err != nil {
		return nil, err
	}
	messages, err := collectMessages(ctx, f.peer, f.tg)
	if err != nil {
		return nil, fmt.Errorf("telegram: list history: %w", err)
	}
	return messages, nil
}

func (f *Fs) objectFromMessage(message *tg.Message) (*Object, bool) {
	if message == nil || message.ID <= 0 {
		return nil, false
	}
	media, ok := message.Media.(*tg.MessageMediaDocument)
	if !ok {
		return nil, false
	}
	document, ok := media.Document.AsNotEmpty()
	if !ok {
		return nil, false
	}
	fileName := documentFileName(document.GetAttributes())
	if fileName == "" {
		fileName = fmt.Sprintf("file-%d", message.ID)
	}
	remote := f.opt.Encoding.FromStandardName(fileName)
	modTime := time.Time{}
	if message.Date > 0 {
		modTime = time.Unix(int64(message.Date), 0)
	}
	return &Object{
		fs:            f,
		remote:        remote,
		size:          document.GetSize(),
		modTime:       modTime,
		messageID:     message.ID,
		documentID:    document.GetID(),
		accessHash:    document.GetAccessHash(),
		fileReference: append([]byte(nil), document.GetFileReference()...),
	}, true
}

func documentFileName(attributes []tg.DocumentAttributeClass) string {
	for _, attribute := range attributes {
		if name, ok := attribute.(*tg.DocumentAttributeFilename); ok && name.FileName != "" {
			return name.FileName
		}
	}
	return ""
}

func (f *Fs) entriesFromMessages(messages []*tg.Message) fs.DirEntries {
	entries := make(fs.DirEntries, 0, len(messages))
	seenNames := make(map[string]struct{})
	for _, message := range messages {
		object, ok := f.objectFromMessage(message)
		if !ok {
			continue
		}
		// History is newest first, so the newest message wins when names collide.
		if _, exists := seenNames[object.remote]; exists {
			continue
		}
		seenNames[object.remote] = struct{}{}
		entries = append(entries, object)
	}
	return entries
}

// List returns all file messages in the root of the configured chat.
func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	if dir != "" {
		return nil, fs.ErrorDirNotFound
	}
	messages, err := f.listMessages(ctx)
	if err != nil {
		return nil, err
	}
	return f.entriesFromMessages(messages), nil
}

// NewObject finds a file by its rclone path.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	if remote == "" || strings.Contains(remote, "/") {
		return nil, fs.ErrorObjectNotFound
	}
	entries, err := f.List(ctx, "")
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.Remote() == remote {
			object, ok := entry.(*Object)
			if !ok {
				return nil, fmt.Errorf("telegram: unexpected object type %T", entry)
			}
			return object, nil
		}
	}
	return nil, fs.ErrorObjectNotFound
}

// Put uploads a document, replacing the current message for the same remote when it exists.
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	existing, err := f.NewObject(ctx, src.Remote())
	if err == nil {
		object, ok := existing.(*Object)
		if !ok {
			return nil, fmt.Errorf("telegram: unexpected object type %T", existing)
		}
		if err := object.Update(ctx, in, src, options...); err != nil {
			return nil, err
		}
		return object, nil
	}
	if !errors.Is(err, fs.ErrorObjectNotFound) {
		return nil, err
	}
	object := &Object{fs: f, remote: src.Remote()}
	if err := object.Update(ctx, in, src, options...); err != nil {
		return nil, err
	}
	return object, nil
}

// Mkdir accepts only the already-existing chat root; Telegram has no folders.
func (f *Fs) Mkdir(_ context.Context, dir string) error {
	if dir == "" {
		return nil
	}
	return fs.ErrorDirNotFound
}

// Rmdir refuses to remove the chat root because that would not be a directory operation.
func (f *Fs) Rmdir(_ context.Context, dir string) error {
	if dir == "" {
		return errors.New("telegram: the chat root cannot be removed")
	}
	return fs.ErrorDirNotFound
}

// Name returns the configured remote name.
func (f *Fs) Name() string { return f.name }

// Root returns the root path, which is empty for this flat backend.
func (f *Fs) Root() string { return f.root }

// String describes the remote.
func (f *Fs) String() string { return fmt.Sprintf("Telegram chat root '%s'", f.root) }

// Precision reports that Telegram message dates cannot represent or preserve file modification times.
func (f *Fs) Precision() time.Duration { return fs.ModTimeNotSupported }

// Hashes returns the supported hash types.
func (f *Fs) Hashes() hash.Set { return hash.Set(hash.None) }

// Features returns optional features of this backend.
func (f *Fs) Features() *fs.Features { return f.features }

// Shutdown cancels and waits for the MTProto client.
func (f *Fs) Shutdown(ctx context.Context) error {
	if f.cancel == nil || f.done == nil {
		return nil
	}
	f.cancel()
	select {
	case <-f.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

var _ fs.Shutdowner = (*Fs)(nil)

func (f *Fs) fetchMessage(ctx context.Context, messageID int) (*tg.Message, error) {
	if err := f.clientError(); err != nil {
		return nil, err
	}
	var result tg.MessagesMessagesClass
	var err error
	if channel, ok := f.peer.(*tg.InputPeerChannel); ok {
		result, err = f.tg.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
			Channel: &tg.InputChannel{ChannelID: channel.ChannelID, AccessHash: channel.AccessHash},
			ID:      []tg.InputMessageClass{&tg.InputMessageID{ID: messageID}},
		})
	} else {
		result, err = f.tg.MessagesGetMessages(ctx, []tg.InputMessageClass{&tg.InputMessageID{ID: messageID}})
	}
	if err != nil {
		return nil, err
	}
	modified, ok := result.AsModified()
	if !ok {
		return nil, fmt.Errorf("telegram: unexpected message response %T", result)
	}
	for _, item := range modified.GetMessages() {
		if message, ok := item.(*tg.Message); ok && message.ID == messageID {
			return message, nil
		}
	}
	return nil, fs.ErrorObjectNotFound
}

func (f *Fs) deleteMessage(ctx context.Context, messageID int) error {
	if messageID <= 0 {
		return errors.New("telegram: refusing to delete a message without a valid ID")
	}
	if err := f.clientError(); err != nil {
		return err
	}
	if channel, ok := f.peer.(*tg.InputPeerChannel); ok {
		_, err := f.tg.ChannelsDeleteMessages(ctx, &tg.ChannelsDeleteMessagesRequest{
			Channel: &tg.InputChannel{ChannelID: channel.ChannelID, AccessHash: channel.AccessHash},
			ID:      []int{messageID},
		})
		return err
	}
	_, err := f.tg.MessagesDeleteMessages(ctx, &tg.MessagesDeleteMessagesRequest{
		ID:     []int{messageID},
		Revoke: true,
	})
	return err
}

func messageFromUpdate(update tg.UpdateClass) *tg.Message {
	switch update := update.(type) {
	case *tg.UpdateNewMessage:
		message, _ := update.Message.(*tg.Message)
		return message
	case *tg.UpdateNewChannelMessage:
		message, _ := update.Message.(*tg.Message)
		return message
	default:
		return nil
	}
}

func messageFromUpdates(updates tg.UpdatesClass) (*tg.Message, int) {
	if short, ok := updates.(*tg.UpdateShortSentMessage); ok {
		return nil, short.ID
	}
	if container, ok := updates.(interface{ GetUpdates() []tg.UpdateClass }); ok {
		for _, update := range container.GetUpdates() {
			if message := messageFromUpdate(update); message != nil {
				return message, message.ID
			}
		}
	}
	return nil, 0
}

func (f *Fs) sentObject(ctx context.Context, updates tg.UpdatesClass, remote string) (*Object, error) {
	message, messageID := messageFromUpdates(updates)
	if message == nil && messageID > 0 {
		var err error
		message, err = f.fetchMessage(ctx, messageID)
		if err != nil {
			return nil, fmt.Errorf("telegram: upload succeeded but its message could not be read: %w", err)
		}
	}
	object, ok := f.objectFromMessage(message)
	if !ok {
		return nil, errors.New("telegram: upload succeeded but the response contained no document message")
	}
	object.remote = remote
	return object, nil
}

// Update uploads a replacement message and removes the previous message after the upload succeeds.
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, _ ...fs.OpenOption) error {
	if err := o.fs.clientError(); err != nil {
		return err
	}
	remote := src.Remote()
	if remote == "" || strings.Contains(remote, "/") {
		return fmt.Errorf("telegram: only flat file names are supported, got %q", remote)
	}
	if src.Size() < 0 {
		return errors.New("telegram: uploads require a known file size")
	}
	name := o.fs.opt.Encoding.ToStandardName(remote)
	uploadedFile, err := uploader.NewUploader(o.fs.tg).Upload(ctx, uploader.NewUpload(name, in, src.Size()))
	if err != nil {
		return fmt.Errorf("telegram: upload document: %w", err)
	}
	updates, err := o.fs.tg.MessagesSendMedia(ctx, &tg.MessagesSendMediaRequest{
		Peer:    o.fs.peer,
		Media:   &tg.InputMediaUploadedDocument{File: uploadedFile, Attributes: []tg.DocumentAttributeClass{&tg.DocumentAttributeFilename{FileName: name}}},
		Message: "",
	})
	if err != nil {
		return fmt.Errorf("telegram: send document: %w", err)
	}
	newObject, err := o.fs.sentObject(ctx, updates, remote)
	if err != nil {
		return err
	}
	if o.messageID > 0 && o.messageID != newObject.messageID {
		if err := o.fs.deleteMessage(ctx, o.messageID); err != nil {
			return fmt.Errorf("telegram: replacement was uploaded as message %d, but old message %d could not be deleted: %w", newObject.messageID, o.messageID, err)
		}
	}
	*o = *newObject
	return nil
}

// Fs returns the file system this object is part of.
func (o *Object) Fs() fs.Info { return o.fs }

// String returns the object name.
func (o *Object) String() string { return o.remote }

// Remote returns the path of the object.
func (o *Object) Remote() string { return o.remote }

// ModTime returns the message creation time.
func (o *Object) ModTime(context.Context) time.Time { return o.modTime }

// Size returns the size of the document.
func (o *Object) Size() int64 { return o.size }

// Storable reports that this object can be stored.
func (o *Object) Storable() bool { return true }

// Hash returns the requested hash or an unsupported-hash error.
func (o *Object) Hash(context.Context, hash.Type) (string, error) { return "", hash.ErrUnsupported }

// SetModTime reports that Telegram message timestamps cannot be changed.
func (o *Object) SetModTime(context.Context, time.Time) error { return fs.ErrorCantSetModTime }

type rangeWriter struct {
	writer    io.Writer
	skip      int64
	remaining int64
}

func (w *rangeWriter) Write(p []byte) (int, error) {
	originalLen := len(p)
	if w.skip > 0 {
		skipped := min(int64(len(p)), w.skip)
		p = p[skipped:]
		w.skip -= skipped
	}
	if w.remaining == 0 {
		return originalLen, nil
	}
	if w.remaining >= 0 && int64(len(p)) > w.remaining {
		p = p[:w.remaining]
	}
	if len(p) == 0 {
		return originalLen, nil
	}
	n, err := w.writer.Write(p)
	if err != nil {
		return originalLen - len(p) + n, err
	}
	if n != len(p) {
		return originalLen - len(p) + n, io.ErrShortWrite
	}
	if w.remaining >= 0 {
		w.remaining -= int64(n)
	}
	return originalLen, nil
}

type cancelPipeReader struct {
	*io.PipeReader
	cancel context.CancelFunc
}

func (r *cancelPipeReader) Close() error {
	r.cancel()
	return r.PipeReader.Close()
}

// Open downloads the document message, refreshing its file reference first.
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	message, err := o.fs.fetchMessage(ctx, o.messageID)
	if err != nil {
		return nil, fmt.Errorf("telegram: refresh document reference: %w", err)
	}
	fresh, ok := o.fs.objectFromMessage(message)
	if !ok {
		return nil, fs.ErrorObjectNotFound
	}
	location := &tg.InputDocumentFileLocation{
		ID:            fresh.documentID,
		AccessHash:    fresh.accessHash,
		FileReference: fresh.fileReference,
	}

	fs.FixRangeOption(options, fresh.size)
	offset, limit := int64(0), int64(-1)
	for _, option := range options {
		switch option := option.(type) {
		case *fs.RangeOption:
			offset, limit = option.Decode(fresh.size)
		case *fs.SeekOption:
			offset, limit = option.Offset, -1
		}
	}
	if offset < 0 {
		offset = 0
	}
	if limit == 0 || (fresh.size >= 0 && offset >= fresh.size) {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}
	if fresh.size >= 0 && limit >= 0 && offset+limit > fresh.size {
		limit = fresh.size - offset
	}

	reader, writer := io.Pipe()
	downloadCtx, cancel := context.WithCancel(ctx)
	go func() {
		target := io.Writer(writer)
		if offset != 0 || limit >= 0 {
			target = &rangeWriter{writer: writer, skip: offset, remaining: limit}
		}
		_, streamErr := downloader.NewDownloader().Download(o.fs.tg, location).Stream(downloadCtx, target)
		if streamErr != nil {
			_ = writer.CloseWithError(streamErr)
		} else {
			_ = writer.Close()
		}
	}()
	return &cancelPipeReader{PipeReader: reader, cancel: cancel}, nil
}

// Remove deletes the Telegram message containing this document.
func (o *Object) Remove(ctx context.Context) error {
	return o.fs.deleteMessage(ctx, o.messageID)
}
