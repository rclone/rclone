//go:build !plan9

package sftp

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestShellEscapeUnix(t *testing.T) {
	for i, test := range []struct {
		unescaped, escaped string
	}{
		{"", ""},
		{"/this/is/harmless", "/this/is/harmless"},
		{"$(rm -rf /)", "\\$\\(rm\\ -rf\\ /\\)"},
		{"/test/\n", "/test/'\n'"},
		{":\"'", ":\\\"\\'"},
	} {
		got, err := quoteOrEscapeShellPath("unix", test.unescaped)
		assert.NoError(t, err)
		assert.Equal(t, test.escaped, got, fmt.Sprintf("Test %d unescaped = %q", i, test.unescaped))
	}
}

func TestShellEscapeCmd(t *testing.T) {
	for i, test := range []struct {
		unescaped, escaped string
		ok                 bool
	}{
		{"", "\"\"", true},
		{"c:/this/is/harmless", "\"c:/this/is/harmless\"", true},
		{"c:/test&notepad", "\"c:/test&notepad\"", true},
		{"c:/test\"&\"notepad", "", false},
	} {
		got, err := quoteOrEscapeShellPath("cmd", test.unescaped)
		if test.ok {
			assert.NoError(t, err)
			assert.Equal(t, test.escaped, got, fmt.Sprintf("Test %d unescaped = %q", i, test.unescaped))
		} else {
			assert.Error(t, err)
		}
	}
}

func TestShellEscapePowerShell(t *testing.T) {
	for i, test := range []struct {
		unescaped, escaped string
	}{
		{"", "''"},
		{"c:/this/is/harmless", "'c:/this/is/harmless'"},
		{"c:/test&notepad", "'c:/test&notepad'"},
		{"c:/test\"&\"notepad", "'c:/test\"&\"notepad'"},
		{"c:/test'&'notepad", "'c:/test''&''notepad'"},
	} {
		got, err := quoteOrEscapeShellPath("powershell", test.unescaped)
		assert.NoError(t, err)
		assert.Equal(t, test.escaped, got, fmt.Sprintf("Test %d unescaped = %q", i, test.unescaped))
	}
}

func TestRemotePathEncodesRemoteNames(t *testing.T) {
	f := &Fs{
		absRoot: "/srv/root",
		opt: Options{
			Enc: encoder.Display | encoder.EncodeColon,
		},
	}

	assert.Equal(t, "/srv/root/dir/file\uFF1Aname", f.remotePath("dir/file:name"))
	assert.Equal(t, "/srv/root", f.remotePath(""))
}

func TestRemoteShellPathEncodesRemoteNames(t *testing.T) {
	f := &Fs{
		absRoot: "/srv/root",
		opt: Options{
			Enc: encoder.Display | encoder.EncodeColon,
		},
	}

	assert.Equal(t, "/srv/root/dir/file\uFF1Aname", f.remoteShellPath("dir/file:name"))
}

func TestRemoteShellPathEncodesPathOverrideNames(t *testing.T) {
	f := &Fs{
		absRoot: "/srv/root",
		opt: Options{
			Enc:          encoder.Display | encoder.EncodeColon,
			PathOverride: "/shell/root",
		},
	}

	assert.Equal(t, "/shell/root/dir/file\uFF1Aname", f.remoteShellPath("dir/file:name"))

	f.opt.PathOverride = "@/volume"
	assert.Equal(t, "/volume/srv/root/dir/file\uFF1Aname", f.remoteShellPath("dir/file:name"))
}

func TestParseHash(t *testing.T) {
	for i, test := range []struct {
		sshOutput, checksum string
	}{
		{"8dbc7733dbd10d2efc5c0a0d8dad90f958581821  RELEASE.md\n", "8dbc7733dbd10d2efc5c0a0d8dad90f958581821"},
		{"03cfd743661f07975fa2f1220c5194cbaff48451  -\n", "03cfd743661f07975fa2f1220c5194cbaff48451"},
	} {
		got := parseHash([]byte(test.sshOutput))
		assert.Equal(t, test.checksum, got, fmt.Sprintf("Test %d sshOutput = %q", i, test.sshOutput))
	}
}

// fakePublicKey is a minimal ssh.PublicKey for tests. Marshal() is the only
// method exercised by the host-key code (FingerprintSHA256 sha256s it).
type fakePublicKey struct {
	keyType string
	data    []byte
}

func (k *fakePublicKey) Type() string                            { return k.keyType }
func (k *fakePublicKey) Marshal() []byte                         { return k.data }
func (k *fakePublicKey) Verify(_ []byte, _ *ssh.Signature) error { return nil }

// makeTestKeys returns n distinct marshalled ed25519 public keys.
func makeTestKeys(t *testing.T, n int) [][]byte {
	keys := make([][]byte, n)
	for i := range keys {
		pub, _, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)
		sshPub, err := ssh.NewPublicKey(pub)
		require.NoError(t, err)
		keys[i] = sshPub.Marshal()
	}
	return keys
}

// makeTestRSAKey returns a marshalled RSA public key.
func makeTestRSAKey(t *testing.T) []byte {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	sshPub, err := ssh.NewPublicKey(&priv.PublicKey)
	require.NoError(t, err)
	return sshPub.Marshal()
}

func TestParseHostKeysField(t *testing.T) {
	keys := makeTestKeys(t, 2)
	keyA, keyB := keys[0], keys[1]
	encA := base64.StdEncoding.EncodeToString(keyA)
	encB := base64.StdEncoding.EncodeToString(keyB)
	rsaKey := makeTestRSAKey(t)
	encRSA := base64.StdEncoding.EncodeToString(rsaKey)

	for _, test := range []struct {
		name    string
		input   fs.CommaSepList
		want    map[string][][]byte
		wantErr string
	}{
		{name: "Empty", input: fs.CommaSepList{}, want: map[string][][]byte{}},
		{name: "EmptyEntrySkipped", input: fs.CommaSepList{""}, want: map[string][][]byte{}},
		{name: "Single", input: fs.CommaSepList{"ssh-ed25519 " + encA}, want: map[string][][]byte{"ssh-ed25519": {keyA}}},
		{
			name:  "MultiAlgo",
			input: fs.CommaSepList{"ssh-ed25519 " + encA, "ssh-rsa " + encRSA},
			want:  map[string][][]byte{"ssh-ed25519": {keyA}, "ssh-rsa": {rsaKey}},
		},
		{
			name:  "MultiKeyPerAlgo",
			input: fs.CommaSepList{"ssh-ed25519 " + encA, "ssh-ed25519 " + encB},
			want:  map[string][][]byte{"ssh-ed25519": {keyA, keyB}},
		},
		{
			name:  "EntryLevelWhitespaceTrimmed",
			input: fs.CommaSepList{"  ssh-ed25519 " + encA + "  ", " ssh-rsa " + encRSA + "\t"},
			want:  map[string][][]byte{"ssh-ed25519": {keyA}, "ssh-rsa": {rsaKey}},
		},
		{
			name:  "DuplicatesDropped",
			input: fs.CommaSepList{"ssh-ed25519 " + encA, "ssh-ed25519 " + encA},
			want:  map[string][][]byte{"ssh-ed25519": {keyA}},
		},
		{name: "MalformedTooFewFields", input: fs.CommaSepList{"ssh-ed25519"}, wantErr: "expected"},
		{name: "MalformedTooManyFields", input: fs.CommaSepList{"ssh-ed25519 " + encA + " trailing"}, wantErr: "expected"},
		{name: "MalformedBase64", input: fs.CommaSepList{"ssh-ed25519 not-base64-!!!"}, wantErr: "base64"},
		{
			name:    "NotAPublicKey",
			input:   fs.CommaSepList{"ssh-ed25519 " + base64.StdEncoding.EncodeToString([]byte("junk"))},
			wantErr: "not a valid SSH public key",
		},
		{
			// The stated algorithm must be the key's own format name.
			name:    "AlgoMismatch",
			input:   fs.CommaSepList{"ssh-rsa " + encA},
			wantErr: `doesn't match the key's type "ssh-ed25519"`,
		},
		{
			// rsa-sha2-* are signature algorithms; an RSA key blob's
			// format is ssh-rsa, so hint at the correct spelling.
			name:    "RSASignatureAlgorithmHint",
			input:   fs.CommaSepList{"rsa-sha2-256 " + encRSA},
			wantErr: "use ssh-rsa",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseHostKeysField(test.input)
			if test.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}

	t.Run("Certificate", func(t *testing.T) {
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)
		signer, err := ssh.NewSignerFromKey(priv)
		require.NoError(t, err)
		cert := &ssh.Certificate{Key: signer.PublicKey(), CertType: ssh.HostCert, ValidBefore: ssh.CertTimeInfinity}
		require.NoError(t, cert.SignCert(rand.Reader, signer))
		entry := cert.Type() + " " + base64.StdEncoding.EncodeToString(cert.Marshal())
		_, err = parseHostKeysField(fs.CommaSepList{entry})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "certificate")
	})

	t.Run("TooManyEntries", func(t *testing.T) {
		var entries fs.CommaSepList
		for _, key := range makeTestKeys(t, maxHostKeys+1) {
			entries = append(entries, "ssh-ed25519 "+base64.StdEncoding.EncodeToString(key))
		}
		_, err := parseHostKeysField(entries)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "more than")
		// Trimming to the cap parses fine.
		_, err = parseHostKeysField(entries[:maxHostKeys])
		require.NoError(t, err)
	})
}

func TestFormatHostKeysFieldRoundTrip(t *testing.T) {
	// Pre-sort the ed25519 keys so the expected output can be written down.
	edKeys := makeTestKeys(t, 3)
	sort.Slice(edKeys, func(i, j int) bool { return bytes.Compare(edKeys[i], edKeys[j]) < 0 })
	rsaKey := makeTestRSAKey(t)
	canonical := map[string][][]byte{
		"ssh-ed25519": {edKeys[0], edKeys[1], edKeys[2]},
		"ssh-rsa":     {rsaKey},
	}
	formatted := formatHostKeysField(canonical)
	// Deterministic: algos sorted, keys within algo sorted by bytes.
	expected := fs.CommaSepList{
		"ssh-ed25519 " + base64.StdEncoding.EncodeToString(edKeys[0]),
		"ssh-ed25519 " + base64.StdEncoding.EncodeToString(edKeys[1]),
		"ssh-ed25519 " + base64.StdEncoding.EncodeToString(edKeys[2]),
		"ssh-rsa " + base64.StdEncoding.EncodeToString(rsaKey),
	}
	assert.Equal(t, expected, formatted)
	// The CommaSepList wire form joins entries with commas.
	assert.Equal(t, strings.Join(expected, ","), formatted.String())

	parsed, err := parseHostKeysField(formatted)
	require.NoError(t, err)
	assert.Equal(t, canonical, parsed)

	// Format is independent of input ordering.
	scrambled := map[string][][]byte{
		"ssh-rsa":     {rsaKey},
		"ssh-ed25519": {edKeys[2], edKeys[0], edKeys[1]},
	}
	assert.Equal(t, expected, formatHostKeysField(scrambled))
}

func TestPinnedHostKeyAlgorithms(t *testing.T) {
	for _, test := range []struct {
		name  string
		input map[string][][]byte
		want  []string
	}{
		{name: "Empty", input: map[string][][]byte{}, want: []string{}},
		{
			name:  "NonRSA",
			input: map[string][][]byte{"ssh-ed25519": {{0x01}}, "ecdsa-sha2-nistp256": {{0x02}}},
			want:  []string{"ecdsa-sha2-nistp256", "ssh-ed25519"},
		},
		{
			// "ssh-rsa" is a key format, not just a signature algorithm, so
			// it must expand to the rsa-sha2 signature algorithms too.
			name:  "RSAExpanded",
			input: map[string][][]byte{"ssh-rsa": {{0x01}}},
			want:  []string{"rsa-sha2-256", "rsa-sha2-512", "ssh-rsa"},
		},
		{
			name:  "RSAAndOthers",
			input: map[string][][]byte{"ssh-rsa": {{0x01}}, "ssh-ed25519": {{0x02}}},
			want:  []string{"rsa-sha2-256", "rsa-sha2-512", "ssh-ed25519", "ssh-rsa"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, pinnedHostKeyAlgorithms(test.input))
		})
	}
}

func TestHostKeyCallbackValidateMatch(t *testing.T) {
	keyBytes := []byte("trusted-key-marshalled-bytes")
	f := &Fs{
		hostKeys: map[string][][]byte{"ssh-ed25519": {keyBytes}},
	}
	cb := f.hostKeyCallback(nil)
	err := cb("example.com:22", nil, &fakePublicKey{keyType: "ssh-ed25519", data: keyBytes})
	assert.NoError(t, err)
}

func TestHostKeyCallbackValidateMismatch(t *testing.T) {
	pinned := []byte("pinned-key-bytes")
	offered := []byte("DIFFERENT-key-bytes")
	f := &Fs{
		hostKeys: map[string][][]byte{"ssh-ed25519": {pinned}},
	}
	cb := f.hostKeyCallback(nil)
	err := cb("example.com:22", nil, &fakePublicKey{keyType: "ssh-ed25519", data: offered})
	require.Error(t, err)
	// Mismatch message should include both fingerprints and a how-to hint.
	assert.Contains(t, err.Error(), "host key mismatch")
	assert.Contains(t, err.Error(), "--sftp-pin-host-key")
}

func TestHostKeyCallbackRejectsCertificate(t *testing.T) {
	f := &Fs{
		opt:      Options{PinHostKey: true},
		hostKeys: map[string][][]byte{},
	}
	var pending *pendingKey
	cb := f.hostKeyCallback(&pending)

	// Wrap a real Ed25519 key in an *ssh.Certificate so the type
	// assertion in the callback fires.
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	sshPub, err := ssh.NewPublicKey(pub)
	require.NoError(t, err)
	cert := &ssh.Certificate{Key: sshPub}

	err = cb("example.com:22", nil, cert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "certificate")
	assert.Nil(t, pending)
}

func TestHostKeyCallbackPinHostKeyStashes(t *testing.T) {
	mapper := configmap.Simple{}
	f := &Fs{
		opt:      Options{PinHostKey: true},
		hostKeys: map[string][][]byte{},
		m:        mapper,
	}
	keyBytes := []byte("fresh-key-bytes")
	var pending *pendingKey
	cb := f.hostKeyCallback(&pending)
	err := cb("example.com:22", nil, &fakePublicKey{keyType: "ssh-ed25519", data: keyBytes})
	assert.NoError(t, err)
	require.NotNil(t, pending, "PinHostKey should stash the offered key")
	assert.Equal(t, "ssh-ed25519", pending.algo)
	assert.Equal(t, keyBytes, pending.marshalled)
	// Neither durable nor in-memory state is touched in the callback: the
	// commit happens only after authentication succeeds.
	_, ok := mapper["host_keys"]
	assert.False(t, ok, "callback must not persist; that's commitHostKey's job")
	assert.Empty(t, f.hostKeys["ssh-ed25519"], "callback must not extend in-memory trust set pre-auth")
}

func TestHostKeyCallbackRefusesAtCap(t *testing.T) {
	// Populate host_keys at the cap, then attempt to PinHostKey a brand new
	// algorithm. The callback must refuse to prevent unbounded growth.
	f := &Fs{
		opt:      Options{PinHostKey: true},
		hostKeys: map[string][][]byte{},
	}
	for i := 0; i < maxHostKeys; i++ {
		algo := fmt.Sprintf("test-algo-%d", i)
		f.hostKeys[algo] = [][]byte{{byte(i)}}
	}
	var pending *pendingKey
	cb := f.hostKeyCallback(&pending)
	err := cb("example.com:22", nil, &fakePublicKey{keyType: "ssh-ed25519", data: []byte("new-key")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cap")
	// Nothing was stashed for persistence.
	assert.Nil(t, pending)
}

func TestHostKeyCallbackRefusesWhenNotPinning(t *testing.T) {
	// pin_host_key=false AND no entry pinned for this algo -> reject.
	f := &Fs{
		opt:      Options{PinHostKey: false},
		hostKeys: map[string][][]byte{},
	}
	cb := f.hostKeyCallback(nil)
	err := cb("example.com:22", nil, &fakePublicKey{keyType: "ssh-ed25519", data: []byte("x")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--sftp-pin-host-key")
}

func TestHostKeyCallbackNilPendingIsValidateOnly(t *testing.T) {
	// Even with pin_host_key set, a callback with no pending slot (the
	// shared validate-only config) must refuse an unpinned key rather
	// than accept it without anywhere to stash it.
	f := &Fs{
		opt:      Options{PinHostKey: true},
		hostKeys: map[string][][]byte{},
	}
	cb := f.hostKeyCallback(nil)
	err := cb("example.com:22", nil, &fakePublicKey{keyType: "ssh-ed25519", data: []byte("x")})
	require.Error(t, err)
}

func TestHostKeyCallbackPinHostKeyRace(t *testing.T) {
	mapper := configmap.Simple{}
	f := &Fs{
		opt:      Options{PinHostKey: true},
		hostKeys: map[string][][]byte{},
		m:        mapper,
	}
	keyBytes := []byte("race-key-bytes")

	// Each concurrent dial gets its own callback and pending key slot.
	var wg sync.WaitGroup
	const N = 16
	pendings := make([]*pendingKey, N)
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func(i int) {
			defer wg.Done()
			cb := f.hostKeyCallback(&pendings[i])
			err := cb("example.com:22", nil, &fakePublicKey{keyType: "ssh-ed25519", data: keyBytes})
			assert.NoError(t, err)
		}(i)
	}
	wg.Wait()
	for i := 0; i < N; i++ {
		require.NotNil(t, pendings[i], "dial %d should have stashed its own key", i)
		assert.Equal(t, keyBytes, pendings[i].marshalled)
	}
	// No mutation of f.hostKeys pre-auth, even under heavy parallel callbacks.
	assert.Empty(t, f.hostKeys["ssh-ed25519"])
}

func TestHostKeyCallbackPendingKeyIsPerConnection(t *testing.T) {
	// Two dials see different keys (e.g. the first connection failed
	// authentication and a MITM answered the retry). Each callback must
	// stash into its own slot so committing the successful connection's
	// key can never pin the abandoned connection's key.
	mapper := configmap.Simple{}
	f := &Fs{
		name:     "myremote",
		opt:      Options{PinHostKey: true},
		hostKeys: map[string][][]byte{},
		m:        mapper,
	}
	keyA := []byte("key-from-failed-connection")
	keyB := []byte("key-from-successful-connection")

	// Dial A: stashes key A, then authentication fails so it is abandoned.
	var pendingA *pendingKey
	require.NoError(t, f.hostKeyCallback(&pendingA)("example.com:22", nil, &fakePublicKey{keyType: "ssh-ed25519", data: keyA}))
	require.NotNil(t, pendingA)

	// Dial B: presents a different key, which must be stashed in B's own
	// slot, not silently accepted because A already stashed one.
	var pendingB *pendingKey
	require.NoError(t, f.hostKeyCallback(&pendingB)("example.com:22", nil, &fakePublicKey{keyType: "ssh-ed25519", data: keyB}))
	require.NotNil(t, pendingB)
	assert.Equal(t, keyB, pendingB.marshalled)

	// Only B authenticates, so only B's key is committed and pinned.
	f.commitHostKey(pendingB)
	assert.Equal(t, [][]byte{keyB}, f.hostKeys["ssh-ed25519"])
	assert.Equal(t, "ssh-ed25519 "+base64.StdEncoding.EncodeToString(keyB), mapper["host_keys"])
}

func TestNewFsHostKeysPrecedence(t *testing.T) {
	// Base config pointing at a port nothing listens on, so if NewFs
	// gets as far as connecting it fails with a dial error.
	newConfig := func() configmap.Simple {
		return configmap.Simple{
			"host": "127.0.0.1",
			"port": "1",
			"user": "testuser",
			"pass": obscure.MustObscure("testpass"),
		}
	}
	ctx, ci := fs.AddConfig(context.Background())
	ci.LowLevelRetries = 1

	t.Run("MalformedHostKeysRejectedWhenPinning", func(t *testing.T) {
		m := newConfig()
		m.Set("host_keys", "this is not a valid entry")
		_, err := NewFs(ctx, "test", "", m)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "host_keys")
	})

	t.Run("MalformedHostKeysIgnoredWithKnownHostsFile", func(t *testing.T) {
		// known_hosts_file takes precedence, so a bad host_keys value
		// must not stop the connection: the failure must be the dial
		// error, not a host_keys parse error.
		knownHosts := filepath.Join(t.TempDir(), "known_hosts")
		require.NoError(t, os.WriteFile(knownHosts, nil, 0o600))
		m := newConfig()
		m.Set("known_hosts_file", knownHosts)
		m.Set("host_keys", "this is not a valid entry")
		m.Set("pin_host_key", "true")
		_, err := NewFs(ctx, "test", "", m)
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "host_keys")
	})
}

func TestCommitHostKey(t *testing.T) {
	mapper := configmap.Simple{}
	f := &Fs{
		name:     "myremote",
		hostKeys: map[string][][]byte{},
		m:        mapper,
	}
	key1 := makeTestKeys(t, 1)[0]
	key2 := makeTestRSAKey(t)

	f.commitHostKey(&pendingKey{algo: "ssh-ed25519", marshalled: key1, fingerprint: "SHA256:fp1", hostname: "h"})
	f.commitHostKey(&pendingKey{algo: "ssh-rsa", marshalled: key2, fingerprint: "SHA256:fp2", hostname: "h"})

	stored, ok := mapper["host_keys"]
	require.True(t, ok, "commitHostKey must persist to the configmap")
	expected := "ssh-ed25519 " + base64.StdEncoding.EncodeToString(key1) + ",ssh-rsa " + base64.StdEncoding.EncodeToString(key2)
	assert.Equal(t, expected, stored)

	// commitHostKey must also extend the in-memory trust set so the validate
	// path on subsequent connections recognises the freshly-pinned keys.
	assert.Equal(t, [][]byte{key1}, f.hostKeys["ssh-ed25519"])
	assert.Equal(t, [][]byte{key2}, f.hostKeys["ssh-rsa"])

	// Second commit of the same key is a no-op (dedupe).
	f.commitHostKey(&pendingKey{algo: "ssh-ed25519", marshalled: key1, fingerprint: "SHA256:fp1", hostname: "h"})
	assert.Equal(t, expected, mapper["host_keys"])
	assert.Equal(t, [][]byte{key1}, f.hostKeys["ssh-ed25519"])
}

func TestCommitHostKeyRefusesToOverwriteMalformed(t *testing.T) {
	// host_keys parsed cleanly at NewFs time, then was hand-edited to
	// something garbage before commit ran. We must not overwrite that
	// value with just our new entry (which would destroy any other valid
	// entries it once contained).
	const garbage = "this is not a valid host_keys value"
	mapper := configmap.Simple{"host_keys": garbage}
	f := &Fs{
		name:     "myremote",
		hostKeys: map[string][][]byte{},
		m:        mapper,
	}
	key := []byte("would-have-been-pinned")
	f.commitHostKey(&pendingKey{algo: "ssh-ed25519", marshalled: key, fingerprint: "SHA256:fp", hostname: "h"})

	// Durable storage is preserved as-is.
	assert.Equal(t, garbage, mapper["host_keys"])
	// In-memory trust set is NOT extended — we refuse to pretend we pinned.
	assert.Empty(t, f.hostKeys["ssh-ed25519"])
}

func TestCommitHostKeyConcurrent(t *testing.T) {
	// Concurrent commits from parallel dials must not lose each
	// other's key in the read-modify-write of the stored value.
	mapper := configmap.Simple{}
	f := &Fs{
		name:     "myremote",
		hostKeys: map[string][][]byte{},
		m:        mapper,
	}
	const N = 8
	keys := makeTestKeys(t, N)
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func(i int) {
			defer wg.Done()
			f.commitHostKey(&pendingKey{
				algo:        "ssh-ed25519",
				marshalled:  keys[i],
				fingerprint: fmt.Sprintf("SHA256:fp%d", i),
				hostname:    "h",
			})
		}(i)
	}
	wg.Wait()
	assert.Len(t, f.hostKeys["ssh-ed25519"], N)
	stored, err := parseHostKeysField(fs.CommaSepList(strings.Split(mapper["host_keys"], ",")))
	require.NoError(t, err)
	assert.Len(t, stored["ssh-ed25519"], N)
}

func TestCommitHostKeyRefusesAtCap(t *testing.T) {
	// The stored host_keys value may have grown to the cap since the
	// callback checked it (e.g. another connection committed first), so
	// the cap must be re-checked against the re-read value.
	keys := makeTestKeys(t, maxHostKeys+1)
	entries := make(fs.CommaSepList, maxHostKeys)
	for i := range entries {
		entries[i] = "ssh-ed25519 " + base64.StdEncoding.EncodeToString(keys[i])
	}
	mapper := configmap.Simple{"host_keys": entries.String()}
	f := &Fs{
		name:     "myremote",
		hostKeys: map[string][][]byte{},
		m:        mapper,
	}
	f.commitHostKey(&pendingKey{algo: "ssh-ed25519", marshalled: keys[maxHostKeys], fingerprint: "SHA256:fp", hostname: "h"})

	// Stored value unchanged and the in-memory set not extended with the new key.
	assert.Equal(t, entries.String(), mapper["host_keys"])
	assert.Empty(t, f.hostKeys["ssh-ed25519"])
}

func TestParseUsage(t *testing.T) {
	for i, test := range []struct {
		sshOutput string
		usage     [3]int64
	}{
		{"Filesystem     1K-blocks     Used Available Use% Mounted on\n/dev/root       91283092 81111888  10154820  89% /", [3]int64{93473886208, 83058573312, 10398535680}},
		{"Filesystem     1K-blocks  Used Available Use% Mounted on\ntmpfs             818256  1636    816620   1% /run", [3]int64{837894144, 1675264, 836218880}},
		{"Filesystem   1024-blocks     Used Available Capacity iused      ifree %iused  Mounted on\n/dev/disk0s2   244277768 94454848 149566920    39%  997820 4293969459    0%   /", [3]int64{250140434432, 96721764352, 153156526080}},
	} {
		gotSpaceTotal, gotSpaceUsed, gotSpaceAvail := parseUsage([]byte(test.sshOutput))
		assert.Equal(t, test.usage, [3]int64{gotSpaceTotal, gotSpaceUsed, gotSpaceAvail}, fmt.Sprintf("Test %d sshOutput = %q", i, test.sshOutput))
	}
}

// internalTestHostKeyPinning exercises the full PinHostKey host-key flow
// against the live SFTP server by mirroring the running Fs's connection
// details into a fresh configmap, then driving NewFs through
// pin/validate/mismatch. The pinned key is written back into the configmap
// so the assertions can read it straight from there.
func (f *Fs) internalTestHostKeyPinning(t *testing.T) {
	ctx := context.Background()

	if len(f.opt.SSH) > 0 {
		t.Skip("host key pinning is bypassed when the ssh option is set")
	}

	// Mirror config from the running Fs into a fresh configmap. f.m.Get
	// returns effective values (including defaults), so the mirrored map is
	// self-contained and NewFs can be called directly with it.
	m := configmap.Simple{}
	for _, opt := range fs.MustFind("sftp").Options {
		// known_hosts_file must not be mirrored as it takes precedence
		// over pin_host_key and would disable the pinning under test.
		if opt.Name == "pin_host_key" || opt.Name == "host_keys" || opt.Name == "known_hosts_file" {
			continue
		}
		v, ok := f.m.Get(opt.Name)
		if !ok || v == "" {
			continue
		}
		m.Set(opt.Name, v)
	}
	m.Set("pin_host_key", "true")

	t.Run("FirstConnectPins", func(t *testing.T) {
		_, err := NewFs(ctx, "pinhostkey", "", m)
		if err != nil && strings.Contains(err.Error(), "SSH certificate") {
			t.Skipf("server presents an SSH certificate; pin_host_key cannot validate: %v", err)
		}
		require.NoError(t, err)
		require.Regexp(t, `^\S+ \S+`, m["host_keys"],
			"PinHostKey should have written host_keys back into the configmap")
	})

	t.Run("SecondConnectValidates", func(t *testing.T) {
		before := m["host_keys"]
		_, err := NewFs(ctx, "pinhostkey", "", m)
		require.NoError(t, err)
		assert.Equal(t, before, m["host_keys"],
			"validate path must not rewrite host_keys")
	})

	t.Run("MismatchRejected", func(t *testing.T) {
		// Substitute a different valid ed25519 key so the mismatch (not the
		// malformed-config) error path fires. This must run in validate-only
		// mode: with pin_host_key still set the negotiation stays open to
		// new algorithms and the server's key would be accepted and pinned
		// on first use rather than mismatching.
		m.Set("pin_host_key", "false")
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)
		sshPub, err := ssh.NewPublicKey(priv.Public())
		require.NoError(t, err)
		m.Set("host_keys", "ssh-ed25519 "+base64.StdEncoding.EncodeToString(sshPub.Marshal()))

		_, err = NewFs(ctx, "pinhostkey", "", m)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "host key mismatch")
	})
}

// InternalTest dispatches integration-only tests that need a live SFTP
// connection. fstests.Run invokes this after the standard test suite.
func (f *Fs) InternalTest(t *testing.T) {
	t.Run("HostKeyPinning", f.internalTestHostKeyPinning)
}

// Check interface
var _ fstests.InternalTester = (*Fs)(nil)
