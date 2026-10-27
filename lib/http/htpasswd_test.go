package http

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMD5Crypt(t *testing.T) {
	// Expected values made with "openssl passwd"
	for _, test := range []struct {
		password, salt, magic, want string
	}{
		{"password", "saltsalt", "$1$", "$1$saltsalt$qjXMvbEw8oaL.CzflDtaK/"},
		{"", "ab", "$1$", "$1$ab$rn6aQS/o7141mj179E/zA."},
		{"md5", "s7fogein", "$apr1$", "$apr1$s7fogein$IK9ItbnGM14ct0bY4Uyik1"},
		{"averyveryverylongpasswordthatislongerthan32bytes", "12345678", "$apr1$", "$apr1$12345678$AzbvZTnrGfP3.51TG02ZD1"},
		{"pässwörd", "x£y", "$1$", "$1$x£y$DQwRxj42VvbTcvJrXRM8k1"},
	} {
		assert.Equal(t, test.want, md5Crypt(test.password, test.salt, test.magic), test.password)
	}
}

// TestCheckHashGolden checks checkHash against hashes made by
// htpasswd, openssl, mkpasswd and Python bcrypt with
// testdata/make-htpasswd-vectors.sh. The expected results were
// recorded from github.com/abbot/go-http-auth v0.4.0 which rclone
// used previously.
func TestCheckHashGolden(t *testing.T) {
	f, err := os.Open("testdata/htpasswd-golden.tsv")
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	n := 0
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), "\t")
		require.Len(t, fields, 4, scanner.Text())
		label, secret, password, want := fields[0], fields[1], fields[2], fields[3] == "true"
		assert.Equal(t, want, checkHash(secret, password), "%s %q %q", label, secret, password)
		n++
	}
	require.NoError(t, scanner.Err())
	assert.Greater(t, n, 1000)
}

func TestCheckHash(t *testing.T) {
	for _, test := range []struct {
		secret, password string
		want             bool
	}{
		{"{SHA}2PRZAyDhNDqRW2OUFwZQqPNdaSY=", "sha", true},
		{"{SHA}2PRZAyDhNDqRW2OUFwZQqPNdaSY=", "wrong", false},
		{"$apr1$s7fogein$IK9ItbnGM14ct0bY4Uyik1", "md5", true},
		{"$apr1$s7fogein$IK9ItbnGM14ct0bY4Uyik1", "wrong", false},
		{"$1$saltsalt$qjXMvbEw8oaL.CzflDtaK/", "password", true},
		{"$2y$10$K/b3mVXUA6X857TOTYIL9.Lbaeg9oBjMQwUX5NefpVUCcYP0Z5KY2", "bcrypt", true},
		{"$2y$10$K/b3mVXUA6X857TOTYIL9.Lbaeg9oBjMQwUX5NefpVUCcYP0Z5KY2", "wrong", false},
		// Unsupported formats never match
		{"plain", "plain", false},
		{"abJnggxhB/yWI", "des", false},
		{"", "", false},
	} {
		assert.Equal(t, test.want, checkHash(test.secret, test.password), "%q %q", test.secret, test.password)
	}
}

func TestParseHtpasswd(t *testing.T) {
	in := strings.NewReader(`# comment
user1:{SHA}2PRZAyDhNDqRW2OUFwZQqPNdaSY=

  user2 : $apr1$s7fogein$IK9ItbnGM14ct0bY4Uyik1  
user3:$1$saltsalt$qjXMvbEw8oaL.CzflDtaK/:extra:fields
malformed
:nouser
nopass:
user1:{SHA}overridden=
`)
	users, err := parseHtpasswd("test", in)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"user1": "{SHA}overridden=",
		"user2": "$apr1$s7fogein$IK9ItbnGM14ct0bY4Uyik1",
		"user3": "$1$saltsalt$qjXMvbEw8oaL.CzflDtaK/",
	}, users)
}

func TestHtpasswdFileUnsupported(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".htpasswd")
	require.NoError(t, os.WriteFile(path, []byte("des:abJnggxhB/yWI\nplain:plain\nmd5:$apr1$s7fogein$IK9ItbnGM14ct0bY4Uyik1\n"), 0600))
	h := &htpasswdFile{path: path}
	assert.False(t, h.check("des", "des"))
	assert.False(t, h.check("plain", "plain"))
	assert.True(t, h.check("md5", "md5"))
	assert.False(t, h.check("nosuchuser", "md5"))
}

// The users must follow changes to the htpasswd file
func TestHtpasswdFileReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".htpasswd")
	require.NoError(t, os.WriteFile(path, []byte("sha:{SHA}2PRZAyDhNDqRW2OUFwZQqPNdaSY=\n"), 0600))
	h := &htpasswdFile{path: path}
	assert.True(t, h.check("sha", "sha"))

	require.NoError(t, os.WriteFile(path, []byte("bcrypt:$2y$10$K/b3mVXUA6X857TOTYIL9.Lbaeg9oBjMQwUX5NefpVUCcYP0Z5KY2\n"), 0600))
	future := time.Now().Add(time.Hour)
	require.NoError(t, os.Chtimes(path, future, future))
	assert.False(t, h.check("sha", "sha"))
	assert.True(t, h.check("bcrypt", "bcrypt"))

	// A missing file lets nobody in
	require.NoError(t, os.Remove(path))
	assert.False(t, h.check("bcrypt", "bcrypt"))
}

// A file which can't be read must be retried even if it doesn't change
func TestHtpasswdFileUnreadable(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("can't make file unreadable")
	}
	path := filepath.Join(t.TempDir(), ".htpasswd")
	require.NoError(t, os.WriteFile(path, []byte("sha:{SHA}2PRZAyDhNDqRW2OUFwZQqPNdaSY=\n"), 0000))
	h := &htpasswdFile{path: path}
	assert.False(t, h.check("sha", "sha"))

	// chmod changes neither the size nor the modification time
	require.NoError(t, os.Chmod(path, 0600))
	assert.True(t, h.check("sha", "sha"))
}
