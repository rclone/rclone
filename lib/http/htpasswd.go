package http

import (
	"bufio"
	"crypto/md5"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base64"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"golang.org/x/crypto/bcrypt"
)

// md5CryptAlphabet is the base64 variant used by crypt(3)
const md5CryptAlphabet = "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// md5Crypt returns the MD5-crypt hash of password as used by "$1$"
// and Apache "$apr1$" entries, in the form magic+salt+"$"+hash.
//
// Unlike crypt(3) the salt isn't truncated to 8 bytes, for
// compatibility with entries rclone accepted previously.
func md5Crypt(password, salt, magic string) string {
	alt := md5.Sum([]byte(password + salt + password))
	h := md5.New()
	_, _ = io.WriteString(h, password+magic+salt)
	for n := len(password); n > 0; n -= md5.Size {
		_, _ = h.Write(alt[:min(n, md5.Size)])
	}
	// The algorithm adds a NUL or the first byte of the password
	// for each bit of the password length
	for n := len(password); n > 0; n >>= 1 {
		if n&1 != 0 {
			_, _ = h.Write([]byte{0})
		} else {
			_, _ = io.WriteString(h, password[:1])
		}
	}
	sum := h.Sum(nil)
	for i := range 1000 {
		h.Reset()
		if i&1 != 0 {
			_, _ = io.WriteString(h, password)
		} else {
			_, _ = h.Write(sum)
		}
		if i%3 != 0 {
			_, _ = io.WriteString(h, salt)
		}
		if i%7 != 0 {
			_, _ = io.WriteString(h, password)
		}
		if i&1 != 0 {
			_, _ = h.Write(sum)
		} else {
			_, _ = io.WriteString(h, password)
		}
		sum = h.Sum(sum[:0])
	}
	out := []byte(magic + salt + "$")
	encode := func(v uint32, n int) {
		for range n {
			out = append(out, md5CryptAlphabet[v&0x3f])
			v >>= 6
		}
	}
	for _, i := range [][3]int{{0, 6, 12}, {1, 7, 13}, {2, 8, 14}, {3, 9, 15}, {4, 10, 5}} {
		encode(uint32(sum[i[0]])<<16|uint32(sum[i[1]])<<8|uint32(sum[i[2]]), 4)
	}
	encode(uint32(sum[11]), 2)
	return string(out)
}

// parseMD5Crypt returns the magic and salt of an MD5-crypt hash or
// ok=false if secret isn't one.
func parseMD5Crypt(secret string) (magic, salt string, ok bool) {
	for _, magic = range []string{"$1$", "$apr1$"} {
		if rest, found := strings.CutPrefix(secret, magic); found {
			salt, _, ok = strings.Cut(rest, "$")
			return magic, salt, ok
		}
	}
	return "", "", false
}

// isBcrypt returns true if secret is a bcrypt hash
func isBcrypt(secret string) bool {
	for _, prefix := range []string{"$2a$", "$2b$", "$2x$", "$2y$"} {
		if strings.HasPrefix(secret, prefix) {
			return true
		}
	}
	return false
}

// supportedHash returns true if secret is in a format checkHash
// can check.
func supportedHash(secret string) bool {
	if _, _, ok := parseMD5Crypt(secret); ok {
		return true
	}
	return strings.HasPrefix(secret, "{SHA}") || isBcrypt(secret)
}

// checkHash returns true if password matches the hashed secret.
//
// It returns false if secret isn't in a supported format.
func checkHash(secret, password string) bool {
	switch {
	case strings.HasPrefix(secret, "{SHA}"):
		sum := sha1.Sum([]byte(password))
		return subtle.ConstantTimeCompare([]byte(secret[len("{SHA}"):]), []byte(base64.StdEncoding.EncodeToString(sum[:]))) == 1
	case isBcrypt(secret):
		return bcrypt.CompareHashAndPassword([]byte(secret), []byte(password)) == nil
	}
	if magic, salt, ok := parseMD5Crypt(secret); ok {
		return subtle.ConstantTimeCompare([]byte(secret), []byte(md5Crypt(password, salt, magic))) == 1
	}
	return false
}

// parseHtpasswd reads user:hash lines from in, returning a map of
// user to hash. Blank lines and lines starting with # are ignored and
// malformed lines are logged and skipped.
func parseHtpasswd(name string, in io.Reader) (map[string]string, error) {
	users := make(map[string]string)
	scanner := bufio.NewScanner(in)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		user, secret, ok := strings.Cut(line, ":")
		// Some tools write extra fields after the hash
		secret, _, _ = strings.Cut(secret, ":")
		user, secret = strings.TrimSpace(user), strings.TrimSpace(secret)
		if !ok || user == "" || secret == "" {
			// Don't log the line as it may contain a password
			fs.Errorf(name, "Ignoring malformed htpasswd line %d", lineNumber)
			continue
		}
		if !supportedHash(secret) {
			fs.Errorf(name, "Password for htpasswd user %q is in an unsupported format - use bcrypt, MD5 or SHA1", user)
		}
		users[user] = secret
	}
	return users, scanner.Err()
}

// htpasswdFile checks passwords against an htpasswd file, reloading
// it whenever it changes.
type htpasswdFile struct {
	path    string
	mu      sync.Mutex
	loaded  bool
	modTime time.Time
	size    int64
	users   map[string]string // user => hashed password
}

// load reads the users from the file. On error it leaves no users and
// the load is retried on the next lookup, as fixing errors such as
// permissions needn't change the file's size or modification time.
//
// Call with mu held.
func (h *htpasswdFile) load(info os.FileInfo) {
	h.loaded, h.users = false, nil
	f, err := os.Open(h.path)
	if err != nil {
		fs.Errorf(nil, "Failed to read htpasswd file: %v", err)
		return
	}
	defer func() { _ = f.Close() }()
	users, err := parseHtpasswd(h.path, f)
	if err != nil {
		fs.Errorf(nil, "Failed to read htpasswd file %q: %v", h.path, err)
		return
	}
	h.loaded, h.modTime, h.size = true, info.ModTime(), info.Size()
	h.users = users
}

// lookup returns the hashed password for user, or "" if there isn't
// one.
func (h *htpasswdFile) lookup(user string) string {
	info, err := os.Stat(h.path)
	h.mu.Lock()
	defer h.mu.Unlock()
	if err != nil {
		fs.Errorf(nil, "Failed to read htpasswd file: %v", err)
		h.loaded, h.users = false, nil
		return ""
	}
	if !h.loaded || !info.ModTime().Equal(h.modTime) || info.Size() != h.size {
		h.load(info)
	}
	return h.users[user]
}

// check returns true if user and password match an entry in the file.
func (h *htpasswdFile) check(user, password string) bool {
	return checkHash(h.lookup(user), password)
}
