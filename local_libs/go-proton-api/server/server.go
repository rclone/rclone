package server

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/bradenaw/juniper/xslices"
	"github.com/gin-gonic/gin"
	"github.com/rclone/go-proton-api"
	"github.com/rclone/go-proton-api/server/backend"
)

type AuthCacher interface {
	GetAuthInfo(username string) (proton.AuthInfo, bool)
	SetAuthInfo(username string, info proton.AuthInfo)
	GetAuth(username string) (proton.Auth, bool)
	SetAuth(username string, auth proton.Auth)
}

// StatusHook is a function that can be used to modify the response code of a call.
type StatusHook func(*http.Request) (int, bool)

type Server struct {
	// r is the gin router.
	r *gin.Engine

	// s is the underlying server.
	s *httptest.Server

	// b is the server backend, which manages accounts, messages, attachments, etc.
	b *backend.Backend

	// callWatchers records callWatchers received by the server.
	callWatchers     []callWatcher
	callWatchersLock sync.RWMutex

	// statusHooks are hooks that can be used to modify the response code of a call.
	statusHooks     []StatusHook
	statusHooksLock sync.RWMutex

	// domain is the test server domain.
	domain string

	// minAppVersion is the minimum app version that the server will accept.
	minAppVersion *semver.Version

	// proxyOrigin is the URL of the origin server when the server is a proxy.
	proxyOrigin string

	// proxyTransport is the transport to use when the server is a proxy.
	proxyTransport *http.Transport

	// authCacher can optionally be set to cache proxied auth calls.
	authCacher AuthCacher

	// offline is whether to pretend the server is offline and return 5xx errors.
	offline bool

	// rateLimit is the rate limiter for the server.
	rateLimit *rateLimiter
}

func New(opts ...Option) *Server {
	builder := newServerBuilder()

	for _, opt := range opts {
		opt.config(builder)
	}

	return builder.build()
}

// GetHostURL returns the API root to make calls to.
func (s *Server) GetHostURL() string {
	return s.s.URL
}

// GetProxyURL returns the API root to make calls to which should be proxied.
func (s *Server) GetProxyURL() string {
	return s.s.URL + "/proxy"
}

// GetDomain returns the domain of the server (e.g. "proton.local").
func (s *Server) GetDomain() string {
	return s.domain
}

// AddCallWatcher adds a call watcher to the server.
func (s *Server) AddCallWatcher(fn func(Call), paths ...string) {
	s.callWatchersLock.Lock()
	defer s.callWatchersLock.Unlock()

	s.callWatchers = append(s.callWatchers, newCallWatcher(fn, paths...))
}

// AddStatusHook adds a status hook to the server.
func (s *Server) AddStatusHook(fn StatusHook) {
	s.statusHooksLock.Lock()
	defer s.statusHooksLock.Unlock()

	s.statusHooks = append(s.statusHooks, fn)
}

// CreateUser creates a new server user with the given username and password.
// A single address will be created for the user, derived from the username and the server's domain.
func (s *Server) CreateUser(username string, password []byte) (string, string, error) {
	userID, err := s.b.CreateUser(username, password)
	if err != nil {
		return "", "", err
	}

	addrID, err := s.b.CreateAddress(userID, username+"@"+s.domain, password, true, proton.AddressStatusEnabled, proton.AddressTypeOriginal)
	if err != nil {
		return "", "", err
	}

	return userID, addrID, nil
}

func (s *Server) RemoveUser(userID string) error {
	return s.b.RemoveUser(userID)
}

func (s *Server) RefreshUser(userID string, refresh proton.RefreshFlag) error {
	return s.b.RefreshUser(userID, refresh)
}

func (s *Server) GetUserKeyIDs(userID string) ([]string, error) {
	user, err := s.b.GetUser(userID)
	if err != nil {
		return nil, err
	}

	return xslices.Map(user.Keys, func(key proton.Key) string {
		return key.ID
	}), nil
}

func (s *Server) CreateUserKey(userID string, password []byte) error {
	return s.b.CreateUserKey(userID, password)
}

func (s *Server) RemoveUserKey(userID, keyID string) error {
	return s.b.RemoveUserKey(userID, keyID)
}

func (s *Server) CreateAddress(userID, email string, password []byte) (string, error) {
	return s.b.CreateAddress(userID, email, password, true, proton.AddressStatusEnabled, proton.AddressTypeOriginal)
}

func (s *Server) CreateAddressAsUpdate(userID, email string, password []byte) (string, error) {
	return s.b.CreateAddressAsUpdate(userID, email, password, true, proton.AddressStatusEnabled, proton.AddressTypeOriginal)
}

func (s *Server) ChangeAddressType(userID, addrId string, addrType proton.AddressType) error {
	return s.b.ChangeAddressType(userID, addrId, addrType)
}

func (s *Server) RemoveAddress(userID, addrID string) error {
	return s.b.RemoveAddress(userID, addrID)
}

func (s *Server) CreateAddressKey(userID, addrID string, password []byte) error {
	return s.b.CreateAddressKey(userID, addrID, password)
}

func (s *Server) RemoveAddressKey(userID, addrID, keyID string) error {
	return s.b.RemoveAddressKey(userID, addrID, keyID)
}

func (s *Server) CreateLabel(userID, name, parentID string, labelType proton.LabelType) (string, error) {
	label, err := s.b.CreateLabel(userID, name, parentID, labelType)
	if err != nil {
		return "", err
	}

	return label.ID, nil
}

func (s *Server) GetLabels(userID string) ([]proton.Label, error) {
	return s.b.GetLabels(userID)
}

func (s *Server) LabelMessage(userID, msgID, labelID string) error {
	return s.b.LabelMessages(userID, labelID, msgID)
}

func (s *Server) UnlabelMessage(userID, msgID, labelID string) error {
	return s.b.UnlabelMessages(userID, labelID, msgID)
}

func (s *Server) AddAddressCreatedEvent(userID, addrID string) error {
	return s.b.AddAddressCreatedUpdate(userID, addrID)
}

func (s *Server) AddLabelCreatedEvent(userID, labelID string) error {
	return s.b.AddLabelCreatedUpdate(userID, labelID)
}

func (s *Server) AddMessageCreatedEvent(userID, messageID string) error {
	return s.b.AddMessageCreatedUpdate(userID, messageID)
}

// SetMaxUpdatesPerEvent
func (s *Server) SetMaxUpdatesPerEvent(max int) {
	s.b.SetMaxUpdatesPerEvent(max)
}

func (s *Server) SetAuthLife(authLife time.Duration) {
	s.b.SetAuthLife(authLife)
}

func (s *Server) SetMinAppVersion(minAppVersion *semver.Version) {
	s.minAppVersion = minAppVersion
}

func (s *Server) SetOffline(offline bool) {
	s.offline = offline
}

func (s *Server) RevokeUser(userID string) error {
	sessions, err := s.b.GetSessions(userID)
	if err != nil {
		return err
	}

	for _, session := range sessions {
		if err := s.b.DeleteSession(userID, session.UID); err != nil {
			return err
		}
	}

	return nil
}

func (s *Server) Close() {
	s.proxyTransport.CloseIdleConnections()
	s.s.Close()
}
