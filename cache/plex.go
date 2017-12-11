// +build !plan9,go1.7

package cache

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"sync"

	"github.com/ncw/rclone/fs"
)

const (
	// defPlexLoginURL is the default URL for Plex login
	defPlexLoginURL = "https://plex.tv/users/sign_in.json"
)

// plexConnector is managing the cache integration with Plex
type plexConnector struct {
	url      *url.URL
	username string
	password string
	token    string
	f        *Fs
	mu       sync.Mutex
}

// newPlexConnector connects to a Plex server and generates a token
func newPlexConnector(f *Fs, plexURL, username, password string) (*plexConnector, error) {
	u, err := url.ParseRequestURI(strings.TrimRight(plexURL, "/"))
	if err != nil {
		return nil, err
	}

	pc := &plexConnector{
		f:        f,
		url:      u,
		username: username,
		password: password,
		token:    "",
	}

	return pc, nil
}

// newPlexConnector connects to a Plex server and generates a token
func newPlexConnectorWithToken(f *Fs, plexURL, token string) (*plexConnector, error) {
	u, err := url.ParseRequestURI(strings.TrimRight(plexURL, "/"))
	if err != nil {
		return nil, err
	}

	pc := &plexConnector{
		f:     f,
		url:   u,
		token: token,
	}

	return pc, nil
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
		return fmt.Errorf("failed to obtain token: %v", err)
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
		fs.ConfigFileSet(p.f.Name(), "plex_token", p.token)
		fs.SaveConfig()
		fs.Infof(p.f.Name(), "Connected to Plex server: %v", p.url.String())
	}

	return nil
}

// isConnected checks if this rclone is authenticated to Plex
func (p *plexConnector) isConnected() bool {
	return p.token != ""
}

// isConfigured checks if this rclone is configured to use a Plex server
func (p *plexConnector) isConfigured() bool {
	return p.url != nil
}

func (p *plexConnector) isPlaying(co *Object) bool {
	isPlaying := false
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/status/sessions", p.url.String()), nil)
	if err != nil {
		return false
	}
	p.fillDefaultHeaders(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	var data map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return false
	}
	sizeGen, ok := get(data, "MediaContainer", "size")
	if !ok {
		return false
	}
	size, ok := sizeGen.(float64)
	if !ok || size < float64(1) {
		return false
	}
	videosGen, ok := get(data, "MediaContainer", "Video")
	if !ok {
		fs.Errorf("plex", "empty videos: %v", data)
		return false
	}
	videos, ok := videosGen.([]interface{})
	if !ok || len(videos) < 1 {
		fs.Errorf("plex", "empty videos: %v", data)
		return false
	}
	for _, v := range videos {
		keyGen, ok := get(v, "key")
		if !ok {
			fs.Errorf("plex", "failed to find: key")
			continue
		}
		key, ok := keyGen.(string)
		if !ok {
			fs.Errorf("plex", "failed to understand: key")
			continue
		}
		req, err := http.NewRequest("GET", fmt.Sprintf("%s%s", p.url.String(), key), nil)
		if err != nil {
			return false
		}
		p.fillDefaultHeaders(req)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false
		}
		var data map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&data)
		if err != nil {
			return false
		}

		remote := co.Remote()
		if cr, yes := co.CacheFs.isWrappedByCrypt(); yes {
			remote, err = cr.DecryptFileName(co.Remote())
			if err != nil {
				fs.Errorf("plex", "can not decrypt wrapped file: %v", err)
				continue
			}
		}
		fpGen, ok := get(data, "MediaContainer", "Metadata", 0, "Media", 0, "Part", 0, "file")
		if !ok {
			fs.Errorf("plex", "failed to understand: %v", data)
			continue
		}
		fp, ok := fpGen.(string)
		if !ok {
			fs.Errorf("plex", "failed to understand: %v", fp)
			continue
		}
		if strings.Contains(fp, remote) {
			isPlaying = true
			break
		}
	}

	return isPlaying
}

func (p *plexConnector) isPlayingAsync(co *Object, response chan bool) {
	time.Sleep(time.Second) // FIXME random guess here
	res := p.isPlaying(co)
	response <- res
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
