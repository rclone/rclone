package telegramdrive

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
	"github.com/PhateValleyman/rclone/fs"
	"github.com/PhateValleyman/rclone/fs/config/configmap"
	"github.com/PhateValleyman/rclone/fs/config/configstruct"
	"github.com/PhateValleyman/rclone/fs/hash"
)

// Options definuje parametry pro rclone.conf
type Options struct {
	AppID    int    `config:"api_id"`
	AppHash  string `config:"api_hash"`
	Phone    string `config:"phone"`
	AuthCode string `config:"auth_code"`
	Password string `config:"twofa_password"`
	Session  string `config:"session_string"`
	ChatID   string `config:"chat_id"`
}

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "telegramdrive",
		Description: "Telegram Drive (Nativní MTProto)",
		NewFs:       NewFs,
		Options: []fs.Option{
			{Name: "api_id", Required: true, Help: "Telegram API ID"},
			{Name: "api_hash", Required: true, Help: "Telegram API Hash"},
			{Name: "phone", Help: "Telefonní číslo ve formátu +420123456789"},
			{Name: "auth_code", IsPassword: true, Help: "Přihlašovací kód z Telegramu (pouze při prvním přihlášení)"},
			{Name: "twofa_password", IsPassword: true, Advanced: true, Help: "Heslo Telegram 2FA (pouze pokud je zapnuté)"},
			{Name: "chat_id", Default: "me", Help: "Cíl pro soubory: 'me' nebo username chatu (např. '@my_channel')"},
			{Name: "session_string", Advanced: true, Help: "Uložený session string pro automatické přihlášení"},
		},
	})
}

// Fs reprezentuje souborový systém Telegramu (kořen nebo složku)
type Fs struct {
	name     string
	root     string
	opt      Options
	features *fs.Features
	client   *telegram.Client
	tg       *tg.Client
	cancel   context.CancelFunc
	peer     tg.InputPeerClass
}

// Object reprezentuje jeden konkrétní soubor na Telegramu
type Object struct {
	fs         *Fs
	remote     string
	size       int64
	modTime    time.Time
	messageID  int // ID zprávy v Telegramu
	documentID int64
	accessHash int64
}

// configSessionStorage implementuje session.Storage pro rclone config
type configSessionStorage struct {
	m    configmap.Mapper
	name string
}

func (s *configSessionStorage) LoadSession(ctx context.Context) ([]byte, error) {
	val, ok := s.m.Get("session_string")
	if !ok || val == "" {
		return nil, session.ErrNotFound
	}
	return []byte(val), nil
}

func (s *configSessionStorage) StoreSession(ctx context.Context, data []byte) error {
	s.m.Set("session_string", string(data))
	return nil
}

// terminalAuth implementuje auth.UserAuthenticator pro interaktivní zadání kódu
type terminalAuth struct {
	phone    string
	authCode string
	password string
}

func (a terminalAuth) Phone(_ context.Context) (string, error) {
	return a.phone, nil
}

func (a terminalAuth) Password(_ context.Context) (string, error) {
	return a.password, nil
}

func (a terminalAuth) Code(_ context.Context, _ *tg.AuthSentCode) (string, error) {
	return a.authCode, nil
}

func (a terminalAuth) AcceptTermsOfService(_ context.Context, tos tg.HelpTermsOfService) error {
	return nil
}

func (a terminalAuth) SignUp(_ context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, fmt.Errorf("signup not supported via rclone")
}

// NewFs inicializuje spojení
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	var opt Options
	if err := configstruct.Set(m, &opt); err != nil {
		return nil, err
	}

	f := &Fs{
		name: name,
		root: root,
		opt:  opt,
	}

	// Inicializace klienta a přihlášení
	err := f.connect(ctx, m)
	if err != nil {
		return nil, fmt.Errorf("chyba při připojování k Telegramu: %w", err)
	}

	f.features = (&fs.Features{}).Fill(ctx, f)
	return f, nil
}

// connect vytvoří MTProto klienta a provede autentizaci
func (f *Fs) connect(ctx context.Context, m configmap.Mapper) error {
	storage := &configSessionStorage{m: m, name: f.name}

	f.client = telegram.NewClient(f.opt.AppID, f.opt.AppHash, telegram.Options{
		SessionStorage: storage,
	})
	f.tg = tg.NewClient(f.client)

	// Spuštění klienta v pozadí
	clientCtx, cancel := context.WithCancel(context.Background())
	f.cancel = cancel

	errChan := make(chan error, 1)
	go func() {
		errChan <- f.client.Run(clientCtx, func(ctx context.Context) error {
			status, err := f.client.Auth().Status(ctx)
			if err != nil {
				return err
			}
			if !status.Authorized {
				if f.opt.Phone == "" {
					return fmt.Errorf("phone number is required for first login")
				}
				flow := auth.NewFlow(terminalAuth{
					phone:    f.opt.Phone,
					authCode: f.opt.AuthCode,
					password: f.opt.Password,
				}, auth.SendCodeOptions{})
				if err := f.client.Auth().IfNecessary(ctx, flow); err != nil {
					return err
				}
			}

			// Resolvování PeerID
			peer, err := f.resolvePeer(ctx, f.opt.ChatID)
			if err != nil {
				return fmt.Errorf("chyba při resolvování chatu: %w", err)
			}
			f.peer = peer

			// Signalizujeme, že jsme připojeni a autorizováni
			errChan <- nil
			<-ctx.Done()
			return nil
		})
	}()

	// Čekáme na výsledek autentizace nebo chybu při startu
	select {
	case err := <-errChan:
		if err != nil {
			cancel()
			return err
		}
	case <-time.After(30 * time.Second):
		cancel()
		return fmt.Errorf("timeout při připojování k Telegramu")
	case <-ctx.Done():
		cancel()
		return ctx.Err()
	}

	return nil
}

// resolvePeer převede chat_id na tg.InputPeerClass
func (f *Fs) resolvePeer(ctx context.Context, chatID string) (tg.InputPeerClass, error) {
	if chatID == "me" || chatID == "" {
		return &tg.InputPeerSelf{}, nil
	}

	if id, err := strconv.ParseInt(chatID, 10, 64); err == nil {
		if id < 0 {
			id = -id
			if id >= 1000000000 {
				id -= 1000000000000
			}
			return nil, fmt.Errorf("numeric chat IDs require a channel access hash; use a username or me")
		}
		return nil, fmt.Errorf("numeric chat IDs require a user access hash; use a username or me")
	}

	username := strings.TrimPrefix(chatID, "@")
	resolved, err := f.tg.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
		Username: username,
	})
	if err != nil {
		return nil, err
	}
	switch peer := resolved.Peer.(type) {
	case *tg.PeerUser:
		for _, user := range resolved.Users {
			if user, ok := user.(*tg.User); ok && user.ID == peer.UserID {
				return &tg.InputPeerUser{UserID: peer.UserID, AccessHash: user.AccessHash}, nil
			}
		}
	case *tg.PeerChannel:
		for _, chat := range resolved.Chats {
			if chat, ok := chat.(*tg.Channel); ok && chat.ID == peer.ChannelID {
				return &tg.InputPeerChannel{ChannelID: peer.ChannelID, AccessHash: chat.AccessHash}, nil
			}
		}
	}
	return nil, fmt.Errorf("username %q did not resolve to a supported peer", chatID)
}

// ==========================================
// Metody pro Fs (Manipulace se složkami)
// ==========================================

func (f *Fs) Name() string             { return f.name }
func (f *Fs) Root() string             { return f.root }
func (f *Fs) String() string           { return fmt.Sprintf("TelegramDrive root '%s'", f.root) }
func (f *Fs) Precision() time.Duration { return time.Second }
func (f *Fs) Hashes() hash.Set         { return hash.Set(hash.None) }
func (f *Fs) Features() *fs.Features   { return f.features }

// List zobrazí obsah složky (ZOBRAZENÍ SOUBORŮ)
func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	if dir != "" {
		return nil, fs.ErrorDirNotFound
	}

	var entries fs.DirEntries

	// TODO: Implementovat stránkování historie
	history, err := f.tg.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
		Peer:  f.peer,
		Limit: 100,
	})
	if err != nil {
		return nil, fmt.Errorf("chyba při získávání historie: %w", err)
	}

	messages, ok := history.AsModified()
	if !ok {
		return nil, fmt.Errorf("neočekávaný formát historie")
	}

	for _, msg := range messages.GetMessages() {
		m, ok := msg.(*tg.Message)
		if !ok {
			continue
		}

		// Hledáme dokumenty (soubory)
		if media, ok := m.Media.(*tg.MessageMediaDocument); ok {
			if document, ok := media.Document.AsNotEmpty(); ok {
				// Získání názvu souboru z atributů
				fileName := "unknown"
				for _, attr := range document.GetAttributes() {
					if nameAttr, ok := attr.(*tg.DocumentAttributeFilename); ok {
						fileName = nameAttr.FileName
						break
					}
				}

				o := &Object{
					fs:         f,
					remote:     fileName,
					size:       document.GetSize(),
					modTime:    time.Unix(int64(m.Date), 0),
					messageID:  m.ID,
					documentID: document.GetID(),
					accessHash: document.GetAccessHash(),
				}
				entries = append(entries, o)
			}
		}
	}

	return entries, nil
}

// Shutdown ukončí běžícího klienta
func (f *Fs) Shutdown(ctx context.Context) error {
	if f.cancel != nil {
		f.cancel()
	}
	return nil
}

var _ fs.Shutdowner = (*Fs)(nil)

// NewObject najde existující soubor podle cesty
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	// Prohledáme historii pro konkrétní soubor
	entries, err := f.List(ctx, "")
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.Remote() == remote {
			return entry.(fs.Object), nil
		}
	}
	return nil, fs.ErrorObjectNotFound
}

// Put nahraje nový soubor (NAHRÁVÁNÍ)
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: src.Remote(),
		size:   src.Size(),
	}
	err := o.Update(ctx, in, src, options...)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// Mkdir a Rmdir jsou povinné, i když Telegram nemá skutečné složky
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	return nil
}

func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return nil
}

// ==========================================
// Metody pro Object (Manipulace se souborem)
// ==========================================

func (o *Object) Fs() fs.Info                           { return o.fs }
func (o *Object) String() string                        { return o.remote }
func (o *Object) Remote() string                        { return o.remote }
func (o *Object) ModTime(ctx context.Context) time.Time { return o.modTime }
func (o *Object) Size() int64                           { return o.size }
func (o *Object) Storable() bool                        { return true }
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// SetModTime Telegram neumožňuje měnit datum starých zpráv, takže vracíme chybu nebo ignorujeme
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return fs.ErrorCantSetModTime
}

// Open stáhne soubor z Telegramu (ČTENÍ SOUBORU)
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	loc := &tg.InputDocumentFileLocation{
		ID:            o.documentID,
		AccessHash:    o.accessHash,
		FileReference: nil, // TODO: Získat z dokumentu pokud je vyžadováno
	}

	d := downloader.NewDownloader()
	r, w := io.Pipe()

	go func() {
		_, err := d.Download(o.fs.tg, loc).Stream(ctx, w)
		w.CloseWithError(err)
	}()

	return r, nil
}

// Update přehraje existující soubor (NAHRÁVÁNÍ SOUBORU / PŘEPSÁNÍ)
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	u := uploader.NewUploader(o.fs.tg)

	// Nahrání souboru jako dokumentu
	f, err := u.Upload(ctx, uploader.NewUpload(o.remote, in, src.Size()))
	if err != nil {
		return fmt.Errorf("chyba při nahrávání: %w", err)
	}

	// Odeslání zprávy s dokumentem
	msg, err := o.fs.tg.MessagesSendMedia(ctx, &tg.MessagesSendMediaRequest{
		Peer: o.fs.peer,
		Media: &tg.InputMediaUploadedDocument{
			File: f,
			Attributes: []tg.DocumentAttributeClass{
				&tg.DocumentAttributeFilename{FileName: o.remote},
			},
		},
		Message: src.Remote(),
	})
	if err != nil {
		return fmt.Errorf("chyba při odesílání zprávy: %w", err)
	}

	// Aktualizace ID zprávy
	if updates, ok := msg.(*tg.Updates); ok {
		for _, update := range updates.GetUpdates() {
			if u, ok := update.(*tg.UpdateNewMessage); ok {
				if m, ok := u.Message.(*tg.Message); ok {
					o.messageID = m.ID
				}
			}
		}
	}

	o.size = src.Size()
	o.modTime = src.ModTime(ctx)
	return nil
}

// Remove smaže soubor (MAZÁNÍ SOUBORŮ)
func (o *Object) Remove(ctx context.Context) error {
	_, err := o.fs.tg.MessagesDeleteMessages(ctx, &tg.MessagesDeleteMessagesRequest{
		ID:     []int{o.messageID},
		Revoke: true,
	})
	return err
}
