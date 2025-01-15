//go:build !plan9 && !js

package cache

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	cache "github.com/patrickmn/go-cache"
	"github.com/rclone/rclone/fs"
	"golang.org/x/net/websocket"
)

const (
	// defPlexLoginURL is the default URL for Plex login
	defPlexLoginURL        = "https://plex.tv/users/sign_in.json"
	defPlexNotificationURL = "%s/:/websockets/notifications?X-Plex-Token=%s"
)

// PlaySessionStateNotification is part of the API response of Plex
type PlaySessionStateNotification struct {
	SessionKey       string `json:"sessionKey"`
	GUID             string `json:"guid"`
	Key              string `json:"key"`
	ViewOffset       int64  `json:"viewOffset"`
	State            string `json:"state"`
	TranscodeSession string `json:"transcodeSession"`
}

// NotificationContainer is part of the API response of Plex
type NotificationContainer struct {
	Type             string                         `json:"type"`
	Size             int                            `json:"size"`
	PlaySessionState []PlaySessionStateNotification `json:"PlaySessionStateNotification"`
}

// PlexNotification is part of the API response of Plex
type PlexNotification struct {
	Container NotificationContainer `json:"NotificationContainer"`
}

// plexConnector is managing the cache integration with Plex
type plexConnector struct {
	url        *url.URL
	username   string
	password   string
	token      string
	insecure   bool
	f          *Fs
	mu         sync.Mutex
	running    bool
	runningMu  sync.Mutex
	stateCache *cache.Cache
	saveToken  func(string)
}

// newPlexConnector connects to a Plex server and generates a token
func newPlexConnector(f *Fs, plexURL, username, password string, insecure bool, saveToken func(string)) (*plexConnector, error) {
	u, err := url.ParseRequestURI(strings.TrimRight(plexURL, "/"))
	if err != nil {
		return nil, err
	}

	pc := &plexConnector{
		f:          f,
		url:        u,
		username:   username,
		password:   password,
		token:      "",
		insecure:   insecure,
		stateCache: cache.New(time.Hour, time.Minute),
		saveToken:  saveToken,
	}

	return pc, nil
}

// newPlexConnector connects to a Plex server and generates a token
func newPlexConnectorWithToken(f *Fs, plexURL, token string, insecure bool) (*plexConnector, error) {
	u, err := url.ParseRequestURI(strings.TrimRight(plexURL, "/"))
	if err != nil {
		return nil, err
	}

	pc := &plexConnector{
		f:          f,
		url:        u,
		token:      token,
		insecure:   insecure,
		stateCache: cache.New(time.Hour, time.Minute),
	}
	pc.listenWebsocket()

	return pc, nil
}

func (p *plexConnector) closeWebsocket() {
	p.runningMu.Lock()
	defer p.runningMu.Unlock()
	fs.Infof("plex", "stopped Plex watcher")
	p.running = false
}

func (p *plexConnector) websocketDial() (*websocket.Conn, error) {
	u := strings.TrimRight(strings.Replace(strings.Replace(
		p.url.String(), "http://", "ws://", 1), "https://", "wss://", 1), "/")
	url := fmt.Sprintf(defPlexNotificationURL, u, p.token)

	config, err := websocket.NewConfig(url, "http://localhost")
	if err != nil {
		return nil, err
	}
	if p.insecure {
		config.TlsConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return websocket.DialConfig(config)
}

func (p *plexConnector) listenWebsocket() {
	p.runningMu.Lock()
	defer p.runningMu.Unlock()

	conn, err := p.websocketDial()
	if err != nil {
		fs.Errorf("plex", "%v", err)
		return
	}

	p.running = true
	go func() {
		for {
			if !p.isConnected() {
				break
			}

			notif := &PlexNotification{}
			err := websocket.JSON.Receive(conn, notif)
			if err != nil {
				fs.Debugf("plex", "%v", err)
				p.closeWebsocket()
				break
			}
			// we're only interested in play events
			if notif.Container.Type == "playing" {
				// we loop through each of them
				for _, v := range notif.Container.PlaySessionState {
					// event type of playing
					if v.State == "playing" {
						// if it's not cached get the details and cache them
						if _, found := p.stateCache.Get(v.Key); !found {
							req, err := http.NewRequest("GET", fmt.Sprintf("%s%s", p.url.String(), v.Key), nil)
							if err != nil {
								continue
							}
							p.fillDefaultHeaders(req)
							resp, err := http.DefaultClient.Do(req)
							if err != nil {
								continue
							}
							var data []byte
							data, err = io.ReadAll(resp.Body)
							if err != nil {
								continue
							}
							p.stateCache.Set(v.Key, data, cache.DefaultExpiration)
						}
					} else if v.State == "stopped" {
						p.stateCache.Delete(v.Key)
					}
				}
			}
		}
	}()
}

// fillDefaultHeaders will add common headers to requests
func (p *plexConnector) fillDefaultHeaders(req *http.Request) {
	req.Header.Add("X-Plex-Client-Identifier", fmt.Sprintf("rclone (%v)", p.f.String()))
	req.Header.Add("X-Plex-Product", fmt.Sprintf("rclone (%v)", p.f.Name()))
	req.Header.Add("X-Plex-Version", fs.Version)
	req.Header.Add("Accept", "application/json")
	if p.token != "" {
		req.Header.Add("X-Plex-Token", p.token)
	}
}

// authenticate will generate a token based on a username/password
func (p *plexConnector) authenticate() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	form := url.Values{}
	form.Set("user[login]", p.username)
	form.Add("user[password]", p.password)
	req, err := http.NewRequest("POST", defPlexLoginURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	p.fillDefaultHeaders(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	var data map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return fmt.Errorf("failed to obtain token: %w", err)
	}
	tokenGen, ok := get(data, "user", "authToken")
	if !ok {
		return fmt.Errorf("failed to obtain token: %v", data)
	}
	token, ok := tokenGen.(string)
	if !ok {
		return fmt.Errorf("failed to obtain token: %v", data)
	}
	p.token = token
	if p.token != "" {
		if p.saveToken != nil {
			p.saveToken(p.token)
		}
		fs.Infof(p.f.Name(), "Connected to Plex server: %v", p.url.String())
	}
	p.listenWebsocket()

	return nil
}

// isConnected checks if this rclone is authenticated to Plex
func (p *plexConnector) isConnected() bool {
	p.runningMu.Lock()
	defer p.runningMu.Unlock()
	return p.running
}

// isConfigured checks if this rclone is configured to use a Plex server
func (p *plexConnector) isConfigured() bool {
	return p.url != nil
}

func (p *plexConnector) isPlaying(co *Object) bool {
	var err error
	if !p.isConnected() {
		p.listenWebsocket()
	}

	remote := co.Remote()
	if cr, yes := p.f.isWrappedByCrypt(); yes {
		remote, err = cr.DecryptFileName(co.Remote())
		if err != nil {
			fs.Debugf("plex", "can not decrypt wrapped file: %v", err)
			return false
		}
	}

	isPlaying := false
	for _, v := range p.stateCache.Items() {
		if bytes.Contains(v.Object.([]byte), []byte(remote)) {
			isPlaying = true
			break
		}
	}

	return isPlaying
}

// adapted from: https://stackoverflow.com/a/28878037 (credit)
func get(m interface{}, path ...interface{}) (interface{}, bool) {
	for _, p := range path {
		switch idx := p.(type) {
		case string:
			if mm, ok := m.(map[string]interface{}); ok {
				if val, found := mm[idx]; found {
					m = val
					continue
				}
			}
			return nil, false
		case int:
			if mm, ok := m.([]interface{}); ok {
				if len(mm) > idx {
					m = mm[idx]
					continue
				}
			}
			return nil, false
		}
	}
	return m, true
}
