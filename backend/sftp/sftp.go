//go:build !plan9

// Package sftp provides a filesystem interface using github.com/pkg/sftp
package sftp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	iofs "io/fs"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/sftp"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/env"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/readers"
	sshagent "github.com/xanzy/ssh-agent"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

const (
	defaultShellType        = "unix"
	shellTypeNotSupported   = "none"
	hashCommandNotSupported = "none"
	minSleep                = 100 * time.Millisecond
	maxSleep                = 2 * time.Second
	decayConstant           = 2           // bigger for slower decay, exponential
	keepAliveInterval       = time.Minute // send keepalives every this long while running commands
)

var (
	currentUser          = env.CurrentUser()
	posixWinAbsPathRegex = regexp.MustCompile(`^/[a-zA-Z]\:($|/)`) // E.g. "/C:" or anything starting with "/C:/"
	unixShellEscapeRegex = regexp.MustCompile("[^A-Za-z0-9_.,:/\\@\u0080-\uFFFFFFFF\n-]")
)

func init() {
	fsi := &fs.RegInfo{
		Name:        "sftp",
		Description: "SSH/SFTP",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:      "host",
			Help:      "SSH host to connect to.\n\nE.g. \"example.com\".",
			Required:  true,
			Sensitive: true,
		}, {
			Name:      "user",
			Help:      "SSH username.",
			Default:   currentUser,
			Sensitive: true,
		}, {
			Name:    "port",
			Help:    "SSH port number.",
			Default: 22,
		}, {
			Name:       "pass",
			Help:       "SSH password, leave blank to use ssh-agent.",
			IsPassword: true,
		}, {
			Name: "key_pem",
			Help: `Raw PEM-encoded private key.

Note that this should be on a single line with line endings replaced with '\n', eg

    key_pem = -----BEGIN RSA PRIVATE KEY-----\nMaMbaIXtE\n0gAMbMbaSsd\nMbaass\n-----END RSA PRIVATE KEY-----

This will generate the single line correctly:

    awk '{printf "%s\\n", $0}' < ~/.ssh/id_rsa

If specified, it will override the key_file parameter.`,
			Sensitive: true,
		}, {
			Name: "key_file",
			Help: "Path to PEM-encoded private key file.\n\nLeave blank or set key-use-agent to use ssh-agent." + env.ShellExpandHelp,
		}, {
			Name: "key_file_pass",
			Help: `The passphrase to decrypt the PEM-encoded private key file.

Only PEM encrypted key files (old OpenSSH format) are supported. Encrypted keys
in the new OpenSSH format can't be used.`,
			IsPassword: true,
			Sensitive:  true,
		}, {
			Name: "pubkey_file",
			Help: `Optional path to public key file.

Set this if you have a signed certificate you want to use for authentication.` + env.ShellExpandHelp,
		}, {
			Name: "known_hosts_file",
			Help: `Optional path to known_hosts file.

Set this value to enable server host key validation.` + env.ShellExpandHelp,
			Advanced: true,
			Examples: []fs.OptionExample{{
				Value: "~/.ssh/known_hosts",
				Help:  "Use OpenSSH's known_hosts file.",
			}},
		}, {
			Name: "key_use_agent",
			Help: `When set forces the usage of the ssh-agent.

When key-file is also set, the ".pub" file of the specified key-file is read and only the associated key is
requested from the ssh-agent. This allows to avoid ` + "`Too many authentication failures for *username*`" + ` errors
when the ssh-agent contains many keys.`,
			Default: false,
		}, {
			Name: "use_insecure_cipher",
			Help: `Enable the use of insecure ciphers and key exchange methods.

This enables the use of the following insecure ciphers and key exchange methods:

- aes128-cbc
- aes192-cbc
- aes256-cbc
- 3des-cbc
- diffie-hellman-group-exchange-sha256
- diffie-hellman-group-exchange-sha1

Those algorithms are insecure and may allow plaintext data to be recovered by an attacker.

This must be false if you use either ciphers or key_exchange advanced options.
`,
			Default: false,
			Examples: []fs.OptionExample{
				{
					Value: "false",
					Help:  "Use default Cipher list.",
				}, {
					Value: "true",
					Help:  "Enables the use of the aes128-cbc cipher and diffie-hellman-group-exchange-sha256, diffie-hellman-group-exchange-sha1 key exchange.",
				},
			},
		}, {
			Name:    "disable_hashcheck",
			Default: false,
			Help:    "Disable the execution of SSH commands to determine if remote file hashing is available.\n\nLeave blank or set to false to enable hashing (recommended), set to true to disable hashing.",
		}, {
			Name:    "ask_password",
			Default: false,
			Help: `Allow asking for SFTP password when needed.

If this is set and no password is supplied then rclone will:
- ask for a password
- not contact the ssh agent
`,
			Advanced: true,
		}, {
			Name:    "path_override",
			Default: "",
			Help: `Override path used by SSH shell commands.

This allows checksum calculation when SFTP and SSH paths are
different. This issue affects among others Synology NAS boxes.

E.g. if shared folders can be found in directories representing volumes:

    rclone sync /home/local/directory remote:/directory --sftp-path-override /volume2/directory

E.g. if home directory can be found in a shared folder called "home":

    rclone sync /home/local/directory remote:/home/directory --sftp-path-override /volume1/homes/USER/directory
	
To specify only the path to the SFTP remote's root, and allow rclone to add any relative subpaths automatically (including unwrapping/decrypting remotes as necessary), add the '@' character to the beginning of the path.

E.g. the first example above could be rewritten as:

	rclone sync /home/local/directory remote:/directory --sftp-path-override @/volume2
	
Note that when using this method with Synology "home" folders, the full "/homes/USER" path should be specified instead of "/home".

E.g. the second example above should be rewritten as:

	rclone sync /home/local/directory remote:/homes/USER/directory --sftp-path-override @/volume1`,
			Advanced: true,
		}, {
			Name:     "set_modtime",
			Default:  true,
			Help:     "Set the modified time on the remote if set.",
			Advanced: true,
		}, {
			Name:     "shell_type",
			Default:  "",
			Help:     "The type of SSH shell on remote server, if any.\n\nLeave blank for autodetect.",
			Advanced: true,
			Examples: []fs.OptionExample{
				{
					Value: shellTypeNotSupported,
					Help:  "No shell access",
				}, {
					Value: "unix",
					Help:  "Unix shell",
				}, {
					Value: "powershell",
					Help:  "PowerShell",
				}, {
					Value: "cmd",
					Help:  "Windows Command Prompt",
				},
			},
		}, {
			Name:     "md5sum_command",
			Default:  "",
			Help:     "The command used to read md5 hashes.\n\nLeave blank for autodetect.",
			Advanced: true,
		}, {
			Name:     "sha1sum_command",
			Default:  "",
			Help:     "The command used to read sha1 hashes.\n\nLeave blank for autodetect.",
			Advanced: true,
		}, {
			Name:     "skip_links",
			Default:  false,
			Help:     "Set to skip any symlinks and any other non regular files.",
			Advanced: true,
		}, {
			Name:     "subsystem",
			Default:  "sftp",
			Help:     "Specifies the SSH2 subsystem on the remote host.",
			Advanced: true,
		}, {
			Name:    "server_command",
			Default: "",
			Help: `Specifies the path or command to run a sftp server on the remote host.

The subsystem option is ignored when server_command is defined.

If adding server_command to the configuration file please note that 
it should not be enclosed in quotes, since that will make rclone fail.

A working example is:

    [remote_name]
    type = sftp
    server_command = sudo /usr/libexec/openssh/sftp-server`,
			Advanced: true,
		}, {
			Name:    "use_fstat",
			Default: false,
			Help: `If set use fstat instead of stat.

Some servers limit the amount of open files and calling Stat after opening
the file will throw an error from the server. Setting this flag will call
Fstat instead of Stat which is called on an already open file handle.

It has been found that this helps with IBM Sterling SFTP servers which have
"extractability" level set to 1 which means only 1 file can be opened at
any given time.
`,
			Advanced: true,
		}, {
			Name:    "disable_concurrent_reads",
			Default: false,
			Help: `If set don't use concurrent reads.

Normally concurrent reads are safe to use and not using them will
degrade performance, so this option is disabled by default.

Some servers limit the amount number of times a file can be
downloaded. Using concurrent reads can trigger this limit, so if you
have a server which returns

    Failed to copy: file does not exist

Then you may need to enable this flag.

If concurrent reads are disabled, the use_fstat option is ignored.
`,
			Advanced: true,
		}, {
			Name:    "disable_concurrent_writes",
			Default: false,
			Help: `If set don't use concurrent writes.

Normally rclone uses concurrent writes to upload files. This improves
the performance greatly, especially for distant servers.

This option disables concurrent writes should that be necessary.
`,
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
			Name: "chunk_size",
			Help: `Upload and download chunk size.

This controls the maximum size of payload in SFTP protocol packets.
The RFC limits this to 32768 bytes (32k), which is the default. However,
a lot of servers support larger sizes, typically limited to a maximum
total package size of 256k, and setting it larger will increase transfer
speed dramatically on high latency links. This includes OpenSSH, and,
for example, using the value of 255k works well, leaving plenty of room
for overhead while still being within a total packet size of 256k.

Make sure to test thoroughly before using a value higher than 32k,
and only use it if you always connect to the same server or after
sufficiently broad testing. If you get errors such as
"failed to send packet payload: EOF", lots of "connection lost",
or "corrupted on transfer", when copying a larger file, try lowering
the value. The server run by [rclone serve sftp](/commands/rclone_serve_sftp)
sends packets with standard 32k maximum payload so you must not
set a different chunk_size when downloading files, but it accepts
packets up to the 256k total size, so for uploads the chunk_size
can be set as for the OpenSSH example above.
`,
			Default:  32 * fs.Kibi,
			Advanced: true,
		}, {
			Name: "concurrency",
			Help: `The maximum number of outstanding requests for one file

This controls the maximum number of outstanding requests for one file.
Increasing it will increase throughput on high latency links at the
cost of using more memory.
`,
			Default:  64,
			Advanced: true,
		}, {
			Name: "connections",
			Help: strings.ReplaceAll(`Maximum number of SFTP simultaneous connections, 0 for unlimited.

Note that setting this is very likely to cause deadlocks so it should
be used with care.

If you are doing a sync or copy then make sure connections is one more
than the sum of |--transfers| and |--checkers|.

If you use |--check-first| then it just needs to be one more than the
maximum of |--checkers| and |--transfers|.

So for |connections 3| you'd use |--checkers 2 --transfers 2
--check-first| or |--checkers 1 --transfers 1|.

`, "|", "`"),
			Default:  0,
			Advanced: true,
		}, {
			Name:    "set_env",
			Default: fs.SpaceSepList{},
			Help: `Environment variables to pass to sftp and commands

Set environment variables in the form:

    VAR=value

to be passed to the sftp client and to any commands run (eg md5sum).

Pass multiple variables space separated, eg

    VAR1=value VAR2=value

and pass variables with spaces in quotes, eg

    "VAR3=value with space" "VAR4=value with space" VAR5=nospacehere

`,
			Advanced: true,
		}, {
			Name:    "ciphers",
			Default: fs.SpaceSepList{},
			Help: `Space separated list of ciphers to be used for session encryption, ordered by preference.

At least one must match with server configuration. This can be checked for example using ssh -Q cipher.

This must not be set if use_insecure_cipher is true.

Example:

    aes128-ctr aes192-ctr aes256-ctr aes128-gcm@openssh.com aes256-gcm@openssh.com
`,
			Advanced: true,
		}, {
			Name:    "key_exchange",
			Default: fs.SpaceSepList{},
			Help: `Space separated list of key exchange algorithms, ordered by preference.

At least one must match with server configuration. This can be checked for example using ssh -Q kex.

This must not be set if use_insecure_cipher is true.

Example:

    sntrup761x25519-sha512@openssh.com curve25519-sha256 curve25519-sha256@libssh.org ecdh-sha2-nistp256
`,
			Advanced: true,
		}, {
			Name:    "macs",
			Default: fs.SpaceSepList{},
			Help: `Space separated list of MACs (message authentication code) algorithms, ordered by preference.

At least one must match with server configuration. This can be checked for example using ssh -Q mac.

Example:

    umac-64-etm@openssh.com umac-128-etm@openssh.com hmac-sha2-256-etm@openssh.com
`,
			Advanced: true,
		}, {
			Name:    "host_key_algorithms",
			Default: fs.SpaceSepList{},
			Help: `Space separated list of host key algorithms, ordered by preference.

At least one must match with server configuration. This can be checked for example using ssh -Q HostKeyAlgorithms.

Note: This can affect the outcome of key negotiation with the server even if server host key validation is not enabled.

Example:

    ssh-ed25519 ssh-rsa ssh-dss
`,
			Advanced: true,
		}, {
			Name:    "ssh",
			Default: fs.SpaceSepList{},
			Help: `Path and arguments to external ssh binary.

Normally rclone will use its internal ssh library to connect to the
SFTP server. However it does not implement all possible ssh options so
it may be desirable to use an external ssh binary.

Rclone ignores all the internal config if you use this option and
expects you to configure the ssh binary with the user/host/port and
any other options you need.

**Important** The ssh command must log in without asking for a
password so needs to be configured with keys or certificates.

Rclone will run the command supplied either with the additional
arguments "-s sftp" to access the SFTP subsystem or with commands such
as "md5sum /path/to/file" appended to read checksums.

Any arguments with spaces in should be surrounded by "double quotes".

An example setting might be:

    ssh -o ServerAliveInterval=20 user@example.com

Note that when using an external ssh binary rclone makes a new ssh
connection for every hash it calculates.
`,
		}, {
			Name:    "socks_proxy",
			Default: "",
			Help: `Socks 5 proxy host.
	
Supports the format user:pass@host:port, user@host:port, host:port.

Example:

	myUser:myPass@localhost:9005
	`,
			Advanced: true,
		}, {
			Name:    "copy_is_hardlink",
			Default: false,
			Help: `Set to enable server side copies using hardlinks.

The SFTP protocol does not define a copy command so normally server
side copies are not allowed with the sftp backend.

However the SFTP protocol does support hardlinking, and if you enable
this flag then the sftp backend will support server side copies. These
will be implemented by doing a hardlink from the source to the
destination.

Not all sftp servers support this.

Note that hardlinking two files together will use no additional space
as the source and the destination will be the same file.

This feature may be useful backups made with --copy-dest.`,
			Advanced: true,
		}},
	}
	fs.Register(fsi)
}

// Options defines the configuration for this backend
type Options struct {
	Host                    string          `config:"host"`
	User                    string          `config:"user"`
	Port                    string          `config:"port"`
	Pass                    string          `config:"pass"`
	KeyPem                  string          `config:"key_pem"`
	KeyFile                 string          `config:"key_file"`
	KeyFilePass             string          `config:"key_file_pass"`
	PubKeyFile              string          `config:"pubkey_file"`
	KnownHostsFile          string          `config:"known_hosts_file"`
	KeyUseAgent             bool            `config:"key_use_agent"`
	UseInsecureCipher       bool            `config:"use_insecure_cipher"`
	DisableHashCheck        bool            `config:"disable_hashcheck"`
	AskPassword             bool            `config:"ask_password"`
	PathOverride            string          `config:"path_override"`
	SetModTime              bool            `config:"set_modtime"`
	ShellType               string          `config:"shell_type"`
	Md5sumCommand           string          `config:"md5sum_command"`
	Sha1sumCommand          string          `config:"sha1sum_command"`
	SkipLinks               bool            `config:"skip_links"`
	Subsystem               string          `config:"subsystem"`
	ServerCommand           string          `config:"server_command"`
	UseFstat                bool            `config:"use_fstat"`
	DisableConcurrentReads  bool            `config:"disable_concurrent_reads"`
	DisableConcurrentWrites bool            `config:"disable_concurrent_writes"`
	IdleTimeout             fs.Duration     `config:"idle_timeout"`
	ChunkSize               fs.SizeSuffix   `config:"chunk_size"`
	Concurrency             int             `config:"concurrency"`
	Connections             int             `config:"connections"`
	SetEnv                  fs.SpaceSepList `config:"set_env"`
	Ciphers                 fs.SpaceSepList `config:"ciphers"`
	KeyExchange             fs.SpaceSepList `config:"key_exchange"`
	MACs                    fs.SpaceSepList `config:"macs"`
	HostKeyAlgorithms       fs.SpaceSepList `config:"host_key_algorithms"`
	SSH                     fs.SpaceSepList `config:"ssh"`
	SocksProxy              string          `config:"socks_proxy"`
	CopyIsHardlink          bool            `config:"copy_is_hardlink"`
}

// Fs stores the interface to the remote SFTP files
type Fs struct {
	name         string
	root         string
	absRoot      string
	shellRoot    string
	shellType    string
	opt          Options          // parsed options
	ci           *fs.ConfigInfo   // global config
	m            configmap.Mapper // config
	features     *fs.Features     // optional features
	config       *ssh.ClientConfig
	url          string
	mkdirLock    *stringLock
	cachedHashes *hash.Set
	poolMu       sync.Mutex
	pool         []*conn
	drain        *time.Timer // used to drain the pool when we stop using the connections
	pacer        *fs.Pacer   // pacer for operations
	savedpswd    string
	sessions     atomic.Int32 // count in use sessions
	tokens       *pacer.TokenDispenser
}

// Object is a remote SFTP file that has been stat'd (so it exists, but is not necessarily open for reading)
type Object struct {
	fs      *Fs
	remote  string
	size    int64       // size of the object
	modTime uint32      // modification time of the object as unix time
	mode    os.FileMode // mode bits from the file
	md5sum  *string     // Cached MD5 checksum
	sha1sum *string     // Cached SHA1 checksum
}

// conn encapsulates an ssh client and corresponding sftp client
type conn struct {
	sshClient  sshClient
	sftpClient *sftp.Client
	err        chan error
}

// Wait for connection to close
func (c *conn) wait() {
	c.err <- c.sshClient.Wait()
}

// Send keepalives every interval over the ssh connection until done is closed
func (c *conn) sendKeepAlives(interval time.Duration) (done chan struct{}) {
	done = make(chan struct{})
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				c.sshClient.SendKeepAlive()
			case <-done:
				return
			}
		}
	}()
	return done
}

// Closes the connection
func (c *conn) close() error {
	sftpErr := c.sftpClient.Close()
	sshErr := c.sshClient.Close()
	if sftpErr != nil {
		return sftpErr
	}
	return sshErr
}

// Returns an error if closed
func (c *conn) closed() error {
	select {
	case err := <-c.err:
		return err
	default:
	}
	return nil
}

// Show that we are using an ssh session
//
// Call removeSession() when done
func (f *Fs) addSession() {
	f.sessions.Add(1)
}

// Show the ssh session is no longer in use
func (f *Fs) removeSession() {
	f.sessions.Add(-1)
}

// getSessions shows whether there are any sessions in use
func (f *Fs) getSessions() int32 {
	return f.sessions.Load()
}

// Open a new connection to the SFTP server.
func (f *Fs) sftpConnection(ctx context.Context) (c *conn, err error) {
	// Rate limit rate of new connections
	c = &conn{
		err: make(chan error, 1),
	}
	if len(f.opt.SSH) == 0 {
		c.sshClient, err = f.newSSHClientInternal(ctx, "tcp", f.opt.Host+":"+f.opt.Port, f.config)
	} else {
		c.sshClient, err = f.newSSHClientExternal()
	}
	if err != nil {
		return nil, fmt.Errorf("couldn't connect SSH: %w", err)
	}
	c.sftpClient, err = f.newSftpClient(c.sshClient)
	if err != nil {
		_ = c.sshClient.Close()
		return nil, fmt.Errorf("couldn't initialise SFTP: %w", err)
	}
	go c.wait()
	return c, nil
}

// Set any environment variables on the ssh.Session
func (f *Fs) setEnv(s sshSession) error {
	for _, env := range f.opt.SetEnv {
		equal := strings.IndexRune(env, '=')
		if equal < 0 {
			return fmt.Errorf("no = found in env var %q", env)
		}
		// fs.Debugf(f, "Setting env %q = %q", env[:equal], env[equal+1:])
		err := s.Setenv(env[:equal], env[equal+1:])
		if err != nil {
			return fmt.Errorf("failed to set env var %q: %w", env[:equal], err)
		}
	}
	return nil
}

// Creates a new SFTP client on conn, using the specified subsystem
// or sftp server, and zero or more option functions
func (f *Fs) newSftpClient(client sshClient, opts ...sftp.ClientOption) (*sftp.Client, error) {
	s, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	err = f.setEnv(s)
	if err != nil {
		return nil, err
	}
	pw, err := s.StdinPipe()
	if err != nil {
		return nil, err
	}
	pr, err := s.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if f.opt.ServerCommand != "" {
		if err := s.Start(f.opt.ServerCommand); err != nil {
			return nil, err
		}
	} else {
		if err := s.RequestSubsystem(f.opt.Subsystem); err != nil {
			return nil, err
		}
	}
	opts = opts[:len(opts):len(opts)] // make sure we don't overwrite the callers opts
	opts = append(opts,
		sftp.UseFstat(f.opt.UseFstat),
		sftp.UseConcurrentReads(!f.opt.DisableConcurrentReads),
		sftp.UseConcurrentWrites(!f.opt.DisableConcurrentWrites),
		sftp.MaxPacketUnchecked(int(f.opt.ChunkSize)),
		sftp.MaxConcurrentRequestsPerFile(f.opt.Concurrency),
	)
	return sftp.NewClientPipe(pr, pw, opts...)
}

// Get an SFTP connection from the pool, or open a new one
func (f *Fs) getSftpConnection(ctx context.Context) (c *conn, err error) {
	accounting.LimitTPS(ctx)
	if f.opt.Connections > 0 {
		f.tokens.Get()
	}
	f.poolMu.Lock()
	for len(f.pool) > 0 {
		c = f.pool[0]
		f.pool = f.pool[1:]
		err := c.closed()
		if err == nil {
			break
		}
		fs.Errorf(f, "Discarding closed SSH connection: %v", err)
		c = nil
	}
	f.poolMu.Unlock()
	if c != nil {
		return c, nil
	}
	err = f.pacer.Call(func() (bool, error) {
		c, err = f.sftpConnection(ctx)
		if err != nil {
			return true, err
		}
		return false, nil
	})
	if f.opt.Connections > 0 && c == nil {
		f.tokens.Put()
	}
	return c, err
}

// Return an SFTP connection to the pool
//
// It nils the pointed to connection out so it can't be reused
//
// if err is not nil then it checks the connection is alive using a
// Getwd request
func (f *Fs) putSftpConnection(pc **conn, err error) {
	if f.opt.Connections > 0 {
		defer f.tokens.Put()
	}
	c := *pc
	if !c.sshClient.CanReuse() {
		return
	}
	*pc = nil
	if err != nil {
		// work out if this is an expected error
		isRegularError := false
		var statusErr *sftp.StatusError
		var pathErr *os.PathError
		switch {
		case errors.Is(err, os.ErrNotExist):
			isRegularError = true
		case errors.As(err, &statusErr):
			isRegularError = true
		case errors.As(err, &pathErr):
			isRegularError = true
		}
		// If not a regular SFTP error code then check the connection
		if !isRegularError {
			_, nopErr := c.sftpClient.Getwd()
			if nopErr != nil {
				fs.Debugf(f, "Connection failed, closing: %v", nopErr)
				_ = c.close()
				return
			}
			fs.Debugf(f, "Connection OK after error: %v", err)
		}
	}
	f.poolMu.Lock()
	f.pool = append(f.pool, c)
	if f.opt.IdleTimeout > 0 {
		f.drain.Reset(time.Duration(f.opt.IdleTimeout)) // nudge on the pool emptying timer
	}
	f.poolMu.Unlock()
}

// Drain the pool of any connections
func (f *Fs) drainPool(ctx context.Context) (err error) {
	f.poolMu.Lock()
	defer f.poolMu.Unlock()
	if sessions := f.getSessions(); sessions != 0 {
		fs.Debugf(f, "Not closing %d unused connections as %d sessions active", len(f.pool), sessions)
		if f.opt.IdleTimeout > 0 {
			f.drain.Reset(time.Duration(f.opt.IdleTimeout)) // nudge on the pool emptying timer
		}
		return nil
	}
	if f.opt.IdleTimeout > 0 {
		f.drain.Stop()
	}
	if len(f.pool) != 0 {
		fs.Debugf(f, "Closing %d unused connections", len(f.pool))
	}
	for i, c := range f.pool {
		if cErr := c.closed(); cErr == nil {
			cErr = c.close()
			if cErr != nil {
				fs.Debugf(f, "Ignoring error closing connection: %v", cErr)
			}
		}
		f.pool[i] = nil
	}
	f.pool = nil
	return nil
}

// NewFs creates a new Fs object from the name and root. It connects to
// the host specified in the config file.
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// This will hold the Fs object.  We need to create it here
	// so we can refer to it in the SSH callback, but it's populated
	// in NewFsWithConnection
	f := &Fs{
		ci: fs.GetConfig(ctx),
	}
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	if len(opt.SSH) != 0 && ((opt.User != currentUser && opt.User != "") || opt.Host != "" || (opt.Port != "22" && opt.Port != "")) {
		fs.Logf(name, "--sftp-ssh is in use - ignoring user/host/port from config - set in the parameters to --sftp-ssh (remove them from the config to silence this warning)")
	}
	f.tokens = pacer.NewTokenDispenser(opt.Connections)

	if opt.User == "" {
		opt.User = currentUser
	}
	if opt.Port == "" {
		opt.Port = "22"
	}

	sshConfig := &ssh.ClientConfig{
		User:            opt.User,
		Auth:            []ssh.AuthMethod{},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         f.ci.ConnectTimeout,
		ClientVersion:   "SSH-2.0-" + f.ci.UserAgent,
	}

	if len(opt.HostKeyAlgorithms) != 0 {
		sshConfig.HostKeyAlgorithms = []string(opt.HostKeyAlgorithms)
	}

	if opt.KnownHostsFile != "" {
		hostcallback, err := knownhosts.New(env.ShellExpand(opt.KnownHostsFile))
		if err != nil {
			return nil, fmt.Errorf("couldn't parse known_hosts_file: %w", err)
		}
		sshConfig.HostKeyCallback = hostcallback
	}

	if opt.UseInsecureCipher && (opt.Ciphers != nil || opt.KeyExchange != nil) {
		return nil, fmt.Errorf("use_insecure_cipher must be false if ciphers or key_exchange are set in advanced configuration")
	}

	sshConfig.Config.SetDefaults()
	if opt.UseInsecureCipher {
		sshConfig.Config.Ciphers = append(sshConfig.Config.Ciphers, "aes128-cbc", "aes192-cbc", "aes256-cbc", "3des-cbc")
		sshConfig.Config.KeyExchanges = append(sshConfig.Config.KeyExchanges, "diffie-hellman-group-exchange-sha1", "diffie-hellman-group-exchange-sha256")
	} else {
		if opt.Ciphers != nil {
			sshConfig.Config.Ciphers = opt.Ciphers
		}
		if opt.KeyExchange != nil {
			sshConfig.Config.KeyExchanges = opt.KeyExchange
		}
	}

	if opt.MACs != nil {
		sshConfig.Config.MACs = opt.MACs
	}

	keyFile := env.ShellExpand(opt.KeyFile)
	pubkeyFile := env.ShellExpand(opt.PubKeyFile)
	//keyPem := env.ShellExpand(opt.KeyPem)
	// Add ssh agent-auth if no password or file or key PEM specified
	if (len(opt.SSH) == 0 && opt.Pass == "" && keyFile == "" && !opt.AskPassword && opt.KeyPem == "") || opt.KeyUseAgent {
		sshAgentClient, _, err := sshagent.New()
		if err != nil {
			return nil, fmt.Errorf("couldn't connect to ssh-agent: %w", err)
		}
		signers, err := sshAgentClient.Signers()
		if err != nil {
			return nil, fmt.Errorf("couldn't read ssh agent signers: %w", err)
		}
		if keyFile != "" {
			// If `opt.KeyUseAgent` is false, then it's expected that `opt.KeyFile` contains the private key
			// and `${opt.KeyFile}.pub` contains the public key.
			//
			// If `opt.KeyUseAgent` is true, then it's expected that `opt.KeyFile` contains the public key.
			// This is how it works with openssh; the `IdentityFile` in openssh config points to the public key.
			// It's not necessary to specify the public key explicitly when using ssh-agent, since openssh and rclone
			// will try all the keys they find in the ssh-agent until they find one that works. But just like
			// `IdentityFile` is used in openssh config to limit the search to one specific key, so does
			// `opt.KeyFile` in rclone config limit the search to that specific key.
			//
			// However, previous versions of rclone would always expect to find the public key in
			// `${opt.KeyFile}.pub` even if `opt.KeyUseAgent` was true. So for the sake of backward compatibility
			// we still first attempt to read the public key from `${opt.KeyFile}.pub`. But if it fails with
			// an `fs.ErrNotExist` then we also try to read the public key from `opt.KeyFile`.
			pubBytes, err := os.ReadFile(keyFile + ".pub")
			if err != nil {
				if errors.Is(err, iofs.ErrNotExist) && opt.KeyUseAgent {
					pubBytes, err = os.ReadFile(keyFile)
					if err != nil {
						return nil, fmt.Errorf("failed to read public key file: %w", err)
					}
				} else {
					return nil, fmt.Errorf("failed to read public key file: %w", err)
				}
			}

			pub, _, _, _, err := ssh.ParseAuthorizedKey(pubBytes)
			if err != nil {
				return nil, fmt.Errorf("failed to parse public key file: %w", err)
			}
			pubM := pub.Marshal()
			found := false
			for _, s := range signers {
				if bytes.Equal(pubM, s.PublicKey().Marshal()) {
					sshConfig.Auth = append(sshConfig.Auth, ssh.PublicKeys(s))
					found = true
					break
				}
			}
			if !found {
				return nil, errors.New("private key not found in the ssh-agent")
			}
		} else {
			sshConfig.Auth = append(sshConfig.Auth, ssh.PublicKeys(signers...))
		}
	}

	// Load key file as a private key, if specified. This is only needed when not using an ssh agent.
	if (keyFile != "" && !opt.KeyUseAgent) || opt.KeyPem != "" {
		var key []byte
		if opt.KeyPem == "" {
			key, err = os.ReadFile(keyFile)
			if err != nil {
				return nil, fmt.Errorf("failed to read private key file: %w", err)
			}
		} else {
			// wrap in quotes because the config is a coming as a literal without them.
			opt.KeyPem, err = strconv.Unquote("\"" + opt.KeyPem + "\"")
			if err != nil {
				return nil, fmt.Errorf("pem key not formatted properly: %w", err)
			}
			key = []byte(opt.KeyPem)
		}
		clearpass := ""
		if opt.KeyFilePass != "" {
			clearpass, err = obscure.Reveal(opt.KeyFilePass)
			if err != nil {
				return nil, err
			}
		}
		var signer ssh.Signer
		if clearpass == "" {
			signer, err = ssh.ParsePrivateKey(key)
		} else {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(clearpass))
		}
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key file: %w", err)
		}

		// If a public key has been specified then use that
		if pubkeyFile != "" {
			certfile, err := os.ReadFile(pubkeyFile)
			if err != nil {
				return nil, fmt.Errorf("unable to read cert file: %w", err)
			}

			pk, _, _, _, err := ssh.ParseAuthorizedKey(certfile)
			if err != nil {
				return nil, fmt.Errorf("unable to parse cert file: %w", err)
			}

			// And the signer for this, which includes the private key signer
			// This is what we'll pass to the ssh client.
			// Normally the ssh client will use the public key built
			// into the private key, but we need to tell it to use the user
			// specified public key cert.  This signer is specific to the
			// cert and will include the private key signer.  Now ssh
			// knows everything it needs.
			cert, ok := pk.(*ssh.Certificate)
			if !ok {
				return nil, errors.New("public key file is not a certificate file: " + pubkeyFile)
			}
			pubsigner, err := ssh.NewCertSigner(cert, signer)
			if err != nil {
				return nil, fmt.Errorf("error generating cert signer: %w", err)
			}
			sshConfig.Auth = append(sshConfig.Auth, ssh.PublicKeys(pubsigner))
		} else {
			sshConfig.Auth = append(sshConfig.Auth, ssh.PublicKeys(signer))
		}
	}

	// Auth from password if specified
	if opt.Pass != "" {
		clearpass, err := obscure.Reveal(opt.Pass)
		if err != nil {
			return nil, err
		}
		sshConfig.Auth = append(sshConfig.Auth,
			ssh.Password(clearpass),
			ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
				return f.keyboardInteractiveReponse(user, instruction, questions, echos, clearpass)
			}),
		)
	}

	// Config for password if none was defined and we're allowed to
	// We don't ask now; we ask if the ssh connection succeeds
	if opt.Pass == "" && opt.AskPassword {
		sshConfig.Auth = append(sshConfig.Auth,
			ssh.PasswordCallback(f.getPass),
			ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
				pass, _ := f.getPass()
				return f.keyboardInteractiveReponse(user, instruction, questions, echos, pass)
			}),
		)
	}

	return NewFsWithConnection(ctx, f, name, root, m, opt, sshConfig)
}

// Do the keyboard interactive challenge
//
// Just send the password back for all questions
func (f *Fs) keyboardInteractiveReponse(user, instruction string, questions []string, echos []bool, pass string) ([]string, error) {
	fs.Debugf(f, "Keyboard interactive auth requested")
	answers := make([]string, len(questions))
	for i := range answers {
		answers[i] = pass
	}
	return answers, nil
}

// If we're in password mode and ssh connection succeeds then this
// callback is called.  First time around we ask the user, and then
// save it so on reconnection we give back the previous string.
// This removes the ability to let the user correct a mistaken entry,
// but means that reconnects are transparent.
// We'll reuse config.Pass for this, 'cos we know it's not been
// specified.
func (f *Fs) getPass() (string, error) {
	for f.savedpswd == "" {
		_, _ = fmt.Fprint(os.Stderr, "Enter SFTP password: ")
		f.savedpswd = config.ReadPassword()
	}
	return f.savedpswd, nil
}

// NewFsWithConnection creates a new Fs object from the name and root and an ssh.ClientConfig. It connects to
// the host specified in the ssh.ClientConfig
func NewFsWithConnection(ctx context.Context, f *Fs, name string, root string, m configmap.Mapper, opt *Options, sshConfig *ssh.ClientConfig) (fs.Fs, error) {
	// Populate the Filesystem Object
	f.name = name
	f.root = root
	f.absRoot = root
	f.shellRoot = root
	f.opt = *opt
	f.m = m
	f.config = sshConfig
	f.url = "sftp://" + opt.User + "@" + opt.Host + ":" + opt.Port + "/" + root
	f.mkdirLock = newStringLock()
	f.pacer = fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant)))
	f.savedpswd = ""
	// set the pool drainer timer going
	if f.opt.IdleTimeout > 0 {
		f.drain = time.AfterFunc(time.Duration(f.opt.IdleTimeout), func() { _ = f.drainPool(ctx) })
	}

	f.features = (&fs.Features{
		CanHaveEmptyDirectories:  true,
		SlowHash:                 true,
		PartialUploads:           true,
		DirModTimeUpdatesOnWrite: true, // indicate writing files to a directory updates its modtime
	}).Fill(ctx, f)
	if !opt.CopyIsHardlink {
		// Disable server side copy unless --sftp-copy-is-hardlink is set
		f.features.Copy = nil
	}
	// Make a connection and pool it to return errors early
	c, err := f.getSftpConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("NewFs: %w", err)
	}
	// Check remote shell type, try to auto-detect if not configured and save to config for later
	if f.opt.ShellType != "" {
		f.shellType = f.opt.ShellType
		fs.Debugf(f, "Shell type %q from config", f.shellType)
	} else {
		session, err := c.sshClient.NewSession()
		if err != nil {
			f.shellType = shellTypeNotSupported
			fs.Debugf(f, "Failed to get shell session for shell type detection command: %v", err)
		} else {
			var stdout, stderr bytes.Buffer
			session.SetStdout(&stdout)
			session.SetStderr(&stderr)
			shellCmd := "echo ${ShellId}%ComSpec%"
			fs.Debugf(f, "Running shell type detection remote command: %s", shellCmd)
			err = session.Run(shellCmd)
			_ = session.Close()
			f.shellType = defaultShellType
			if err != nil {
				fs.Debugf(f, "Remote command failed: %v (stdout=%v) (stderr=%v)", err, bytes.TrimSpace(stdout.Bytes()), bytes.TrimSpace(stderr.Bytes()))
			} else {
				outBytes := stdout.Bytes()
				fs.Debugf(f, "Remote command result: %s", outBytes)
				outString := string(bytes.TrimSpace(stdout.Bytes()))
				if outString != "" {
					if strings.HasPrefix(outString, "Microsoft.PowerShell") { // PowerShell: "Microsoft.PowerShell%ComSpec%"
						f.shellType = "powershell"
					} else if !strings.HasSuffix(outString, "%ComSpec%") { // Command Prompt: "${ShellId}C:\WINDOWS\system32\cmd.exe"
						// Additional positive test, to avoid misdetection on unpredicted Unix shell variants
						s := strings.ToLower(outString)
						if strings.Contains(s, ".exe") || strings.Contains(s, ".com") {
							f.shellType = "cmd"
						}
					} // POSIX-based Unix shell: "%ComSpec%"
				} // fish Unix shell: ""
			}
		}
		// Save permanently in config to avoid the extra work next time
		fs.Debugf(f, "Shell type %q detected (set option shell_type to override)", f.shellType)
		f.m.Set("shell_type", f.shellType)
	}
	// Ensure we have absolute path to root
	// It appears that WS FTP doesn't like relative paths,
	// and the openssh sftp tool also uses absolute paths.
	if !path.IsAbs(f.root) {
		// Trying RealPath first, to perform proper server-side canonicalize.
		// It may fail (SSH_FX_FAILURE reported on WS FTP) and will then resort
		// to simple path join with current directory from Getwd (which can work
		// on WS FTP, even though it is also based on RealPath).
		absRoot, err := c.sftpClient.RealPath(f.root)
		if err != nil {
			fs.Debugf(f, "Failed to resolve path using RealPath: %v", err)
			cwd, err := c.sftpClient.Getwd()
			if err != nil {
				fs.Debugf(f, "Failed to to read current directory - using relative paths: %v", err)
			} else {
				f.absRoot = path.Join(cwd, f.root)
				fs.Debugf(f, "Relative path joined with current directory to get absolute path %q", f.absRoot)
			}
		} else {
			f.absRoot = absRoot
			fs.Debugf(f, "Relative path resolved to %q", f.absRoot)
		}
	}
	f.putSftpConnection(&c, err)
	if root != "" && !strings.HasSuffix(root, "/") {
		// Check to see if the root is actually an existing file,
		// and if so change the filesystem root to its parent directory.
		oldAbsRoot := f.absRoot
		remote := path.Base(root)
		f.root = path.Dir(root)
		f.absRoot = path.Dir(f.absRoot)
		if f.root == "." {
			f.root = ""
		}
		_, err = f.NewObject(ctx, remote)
		if err != nil {
			if err != fs.ErrorObjectNotFound && err != fs.ErrorIsDir {
				return nil, err
			}
			// File doesn't exist so keep the old f
			f.root = root
			f.absRoot = oldAbsRoot
			err = nil
		} else {
			// File exists so change fs to point to the parent and return it with an error
			err = fs.ErrorIsFile
		}
	} else {
		err = nil
	}
	fs.Debugf(f, "Using root directory %q", f.absRoot)
	return f, err
}

// Name returns the configured name of the file system
func (f *Fs) Name() string {
	return f.name
}

// Root returns the root for the filesystem
func (f *Fs) Root() string {
	return f.root
}

// String returns the URL for the filesystem
func (f *Fs) String() string {
	return f.url
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Precision is the remote sftp file system's modtime precision, which we have no way of knowing. We estimate at 1s
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// NewObject creates a new remote sftp file object
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	err := o.stat(ctx)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// dirExists returns true,nil if the directory exists, false, nil if
// it doesn't or false, err
func (f *Fs) dirExists(ctx context.Context, dir string) (bool, error) {
	if dir == "" {
		dir = "."
	}
	c, err := f.getSftpConnection(ctx)
	if err != nil {
		return false, fmt.Errorf("dirExists: %w", err)
	}
	info, err := c.sftpClient.Stat(dir)
	f.putSftpConnection(&c, err)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("dirExists stat failed: %w", err)
	}
	if !info.IsDir() {
		return false, fs.ErrorIsFile
	}
	return true, nil
}

// List the objects and directories in dir into entries.  The
// entries can be returned in any order but should be for a
// complete directory.
//
// dir should be "" to list the root, and should not have
// trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	root := path.Join(f.absRoot, dir)
	sftpDir := root
	if sftpDir == "" {
		sftpDir = "."
	}
	c, err := f.getSftpConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("List: %w", err)
	}
	infos, err := c.sftpClient.ReadDir(sftpDir)
	f.putSftpConnection(&c, err)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fs.ErrorDirNotFound
		}
		return nil, fmt.Errorf("error listing %q: %w", dir, err)
	}
	for _, info := range infos {
		remote := path.Join(dir, info.Name())
		// If file is a symlink (not a regular file is the best cross platform test we can do), do a stat to
		// pick up the size and type of the destination, instead of the size and type of the symlink.
		if !info.Mode().IsRegular() && !info.IsDir() {
			if f.opt.SkipLinks {
				// skip non regular file if SkipLinks is set
				continue
			}
			oldInfo := info
			info, err = f.stat(ctx, remote)
			if err != nil {
				if !os.IsNotExist(err) {
					fs.Errorf(remote, "stat of non-regular file failed: %v", err)
				}
				info = oldInfo
			}
		}
		if info.IsDir() {
			d := fs.NewDir(remote, info.ModTime())
			entries = append(entries, d)
		} else {
			o := &Object{
				fs:     f,
				remote: remote,
			}
			o.setMetadata(info)
			entries = append(entries, o)
		}
	}
	return entries, nil
}

// Put data from <in> into a new remote sftp file object described by <src.Remote()> and <src.ModTime(ctx)>
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	err := f.mkParentDir(ctx, src.Remote())
	if err != nil {
		return nil, fmt.Errorf("Put mkParentDir failed: %w", err)
	}
	// Temporary object under construction
	o := &Object{
		fs:     f,
		remote: src.Remote(),
	}
	err = o.Update(ctx, in, src, options...)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// mkParentDir makes the parent of remote if necessary and any
// directories above that
func (f *Fs) mkParentDir(ctx context.Context, remote string) error {
	parent := path.Dir(remote)
	return f.mkdir(ctx, path.Join(f.absRoot, parent))
}

// mkdir makes the directory and parents using native paths
func (f *Fs) mkdir(ctx context.Context, dirPath string) error {
	f.mkdirLock.Lock(dirPath)
	defer f.mkdirLock.Unlock(dirPath)
	if dirPath == "." || dirPath == "/" {
		return nil
	}
	ok, err := f.dirExists(ctx, dirPath)
	if err != nil {
		return fmt.Errorf("mkdir dirExists failed: %w", err)
	}
	if ok {
		return nil
	}
	parent := path.Dir(dirPath)
	err = f.mkdir(ctx, parent)
	if err != nil {
		return err
	}
	c, err := f.getSftpConnection(ctx)
	if err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	err = c.sftpClient.Mkdir(dirPath)
	f.putSftpConnection(&c, err)
	if err != nil {
		if os.IsExist(err) {
			fs.Debugf(f, "directory %q exists after Mkdir is attempted", dirPath)
			return nil
		}
		return fmt.Errorf("mkdir %q failed: %w", dirPath, err)
	}
	return nil
}

// Mkdir makes the root directory of the Fs object
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	root := path.Join(f.absRoot, dir)
	return f.mkdir(ctx, root)
}

// DirSetModTime sets the directory modtime for dir
func (f *Fs) DirSetModTime(ctx context.Context, dir string, modTime time.Time) error {
	o := Object{
		fs:     f,
		remote: dir,
	}
	return o.SetModTime(ctx, modTime)
}

// Rmdir removes the root directory of the Fs object
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	// Check to see if directory is empty as some servers will
	// delete recursively with RemoveDirectory
	entries, err := f.List(ctx, dir)
	if err != nil {
		return fmt.Errorf("Rmdir: %w", err)
	}
	if len(entries) != 0 {
		return fs.ErrorDirectoryNotEmpty
	}
	// Remove the directory
	root := path.Join(f.absRoot, dir)
	c, err := f.getSftpConnection(ctx)
	if err != nil {
		return fmt.Errorf("Rmdir: %w", err)
	}
	err = c.sftpClient.RemoveDirectory(root)
	f.putSftpConnection(&c, err)
	return err
}

// Move renames a remote sftp file object
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}
	err := f.mkParentDir(ctx, remote)
	if err != nil {
		return nil, fmt.Errorf("Move mkParentDir failed: %w", err)
	}
	c, err := f.getSftpConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("Move: %w", err)
	}
	srcPath, dstPath := srcObj.path(), path.Join(f.absRoot, remote)
	if _, ok := c.sftpClient.HasExtension("posix-rename@openssh.com"); ok {
		err = c.sftpClient.PosixRename(srcPath, dstPath)
	} else {
		// If haven't got PosixRename then remove source first before renaming
		err = c.sftpClient.Remove(dstPath)
		if err != nil && !errors.Is(err, iofs.ErrNotExist) {
			fs.Errorf(f, "Move: Failed to remove existing file %q: %v", dstPath, err)
		}
		err = c.sftpClient.Rename(srcPath, dstPath)
	}
	f.putSftpConnection(&c, err)
	if err != nil {
		return nil, fmt.Errorf("Move Rename failed: %w", err)
	}
	dstObj, err := f.NewObject(ctx, remote)
	if err != nil {
		return nil, fmt.Errorf("Move NewObject failed: %w", err)
	}
	return dstObj, nil
}

// Copy server side copies a remote sftp file object using hardlinks
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	if !f.opt.CopyIsHardlink {
		return nil, fs.ErrorCantCopy
	}
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	err := f.mkParentDir(ctx, remote)
	if err != nil {
		return nil, fmt.Errorf("Copy mkParentDir failed: %w", err)
	}
	c, err := f.getSftpConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("Copy: %w", err)
	}
	srcPath, dstPath := srcObj.path(), path.Join(f.absRoot, remote)
	err = c.sftpClient.Link(srcPath, dstPath)
	f.putSftpConnection(&c, err)
	if err != nil {
		if sftpErr, ok := err.(*sftp.StatusError); ok {
			if sftpErr.FxCode() == sftp.ErrSSHFxOpUnsupported {
				// Remote doesn't support Link
				return nil, fs.ErrorCantCopy
			}
		}
		return nil, fmt.Errorf("Copy failed: %w", err)
	}
	dstObj, err := f.NewObject(ctx, remote)
	if err != nil {
		return nil, fmt.Errorf("Copy NewObject failed: %w", err)
	}
	return dstObj, nil
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server-side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}
	srcPath := path.Join(srcFs.absRoot, srcRemote)
	dstPath := path.Join(f.absRoot, dstRemote)

	// Check if destination exists
	ok, err := f.dirExists(ctx, dstPath)
	if err != nil {
		return fmt.Errorf("DirMove dirExists dst failed: %w", err)
	}
	if ok {
		return fs.ErrorDirExists
	}

	// Make sure the parent directory exists
	err = f.mkdir(ctx, path.Dir(dstPath))
	if err != nil {
		return fmt.Errorf("DirMove mkParentDir dst failed: %w", err)
	}

	// Do the move
	c, err := f.getSftpConnection(ctx)
	if err != nil {
		return fmt.Errorf("DirMove: %w", err)
	}
	err = c.sftpClient.Rename(
		srcPath,
		dstPath,
	)
	f.putSftpConnection(&c, err)
	if err != nil {
		return fmt.Errorf("DirMove Rename(%q,%q) failed: %w", srcPath, dstPath, err)
	}
	return nil
}

// run runds cmd on the remote end returning standard output
func (f *Fs) run(ctx context.Context, cmd string) ([]byte, error) {
	f.addSession() // Show session in use
	defer f.removeSession()

	c, err := f.getSftpConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("run: get SFTP connection: %w", err)
	}
	defer f.putSftpConnection(&c, err)

	// Send keepalives while the connection is open
	defer close(c.sendKeepAlives(keepAliveInterval))

	session, err := c.sshClient.NewSession()
	if err != nil {
		return nil, fmt.Errorf("run: get SFTP session: %w", err)
	}
	err = f.setEnv(session)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = session.Close()
	}()

	var stdout, stderr bytes.Buffer
	session.SetStdout(&stdout)
	session.SetStderr(&stderr)

	fs.Debugf(f, "Running remote command: %s", cmd)
	err = session.Run(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to run %q: %s: %w", cmd, bytes.TrimSpace(stderr.Bytes()), err)
	}
	fs.Debugf(f, "Remote command result: %s", bytes.TrimSpace(stdout.Bytes()))

	return stdout.Bytes(), nil
}

// Hashes returns the supported hash types of the filesystem
func (f *Fs) Hashes() hash.Set {
	ctx := context.TODO()

	if f.cachedHashes != nil {
		return *f.cachedHashes
	}

	hashSet := hash.NewHashSet()
	f.cachedHashes = &hashSet

	if f.opt.DisableHashCheck || f.shellType == shellTypeNotSupported {
		return hashSet
	}

	// look for a hash command which works
	checkHash := func(hashType hash.Type, commands []struct{ hashFile, hashEmpty string }, expected string, hashCommand *string, changed *bool) bool {
		if *hashCommand == hashCommandNotSupported {
			return false
		}
		if *hashCommand != "" {
			return true
		}
		fs.Debugf(f, "Checking default %v hash commands", hashType)
		*changed = true
		for _, command := range commands {
			output, err := f.run(ctx, command.hashEmpty)
			if err != nil {
				fs.Debugf(f, "Hash command skipped: %v", err)
				continue
			}
			output = bytes.TrimSpace(output)
			if parseHash(output) == expected {
				*hashCommand = command.hashFile
				fs.Debugf(f, "Hash command accepted")
				return true
			}
			fs.Debugf(f, "Hash command skipped: Wrong output")
		}
		*hashCommand = hashCommandNotSupported
		return false
	}

	changed := false
	md5Commands := []struct {
		hashFile, hashEmpty string
	}{
		{"md5sum", "md5sum"},
		{"md5 -r", "md5 -r"},
		{"rclone md5sum", "rclone md5sum"},
	}
	sha1Commands := []struct {
		hashFile, hashEmpty string
	}{
		{"sha1sum", "sha1sum"},
		{"sha1 -r", "sha1 -r"},
		{"rclone sha1sum", "rclone sha1sum"},
	}
	if f.shellType == "powershell" {
		md5Commands = append(md5Commands, struct {
			hashFile, hashEmpty string
		}{
			"&{param($Path);Get-FileHash -Algorithm MD5 -LiteralPath $Path -ErrorAction Stop|Select-Object -First 1 -ExpandProperty Hash|ForEach-Object{\"$($_.ToLower())  ${Path}\"}}",
			"Get-FileHash -Algorithm MD5 -InputStream ([System.IO.MemoryStream]::new()) -ErrorAction Stop|Select-Object -First 1 -ExpandProperty Hash|ForEach-Object{$_.ToLower()}",
		})

		sha1Commands = append(sha1Commands, struct {
			hashFile, hashEmpty string
		}{
			"&{param($Path);Get-FileHash -Algorithm SHA1 -LiteralPath $Path -ErrorAction Stop|Select-Object -First 1 -ExpandProperty Hash|ForEach-Object{\"$($_.ToLower())  ${Path}\"}}",
			"Get-FileHash -Algorithm SHA1 -InputStream ([System.IO.MemoryStream]::new()) -ErrorAction Stop|Select-Object -First 1 -ExpandProperty Hash|ForEach-Object{$_.ToLower()}",
		})
	}

	md5Works := checkHash(hash.MD5, md5Commands, "d41d8cd98f00b204e9800998ecf8427e", &f.opt.Md5sumCommand, &changed)
	sha1Works := checkHash(hash.SHA1, sha1Commands, "da39a3ee5e6b4b0d3255bfef95601890afd80709", &f.opt.Sha1sumCommand, &changed)

	if changed {
		// Save permanently in config to avoid the extra work next time
		fs.Debugf(f, "Setting hash command for %v to %q (set sha1sum_command to override)", hash.MD5, f.opt.Md5sumCommand)
		f.m.Set("md5sum_command", f.opt.Md5sumCommand)
		fs.Debugf(f, "Setting hash command for %v to %q (set md5sum_command to override)", hash.SHA1, f.opt.Sha1sumCommand)
		f.m.Set("sha1sum_command", f.opt.Sha1sumCommand)
	}

	if sha1Works {
		hashSet.Add(hash.SHA1)
	}
	if md5Works {
		hashSet.Add(hash.MD5)
	}

	return hashSet
}

// About gets usage stats
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	// If server implements the vendor-specific VFS statistics extension prefer that
	// (OpenSSH implements it on using syscall.Statfs on Linux and API function GetDiskFreeSpace on Windows)
	c, err := f.getSftpConnection(ctx)
	if err != nil {
		return nil, err
	}
	var vfsStats *sftp.StatVFS
	if _, found := c.sftpClient.HasExtension("statvfs@openssh.com"); found {
		fs.Debugf(f, "Server has VFS statistics extension")
		aboutPath := f.absRoot
		if aboutPath == "" {
			aboutPath = "/"
		}
		fs.Debugf(f, "About path %q", aboutPath)
		vfsStats, err = c.sftpClient.StatVFS(aboutPath)
	}
	f.putSftpConnection(&c, err) // Return to pool asap, if running shell command below it will be reused
	if vfsStats != nil {
		total := vfsStats.TotalSpace()
		free := vfsStats.FreeSpace()
		used := total - free
		return &fs.Usage{
			Total: fs.NewUsageValue(int64(total)),
			Used:  fs.NewUsageValue(int64(used)),
			Free:  fs.NewUsageValue(int64(free)),
		}, nil
	} else if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		fs.Debugf(f, "Failed to retrieve VFS statistics, trying shell command instead: %v", err)
	} else {
		fs.Debugf(f, "Server does not have the VFS statistics extension, trying shell command instead")
	}

	// Fall back to shell command method if possible
	if f.shellType == shellTypeNotSupported || f.shellType == "cmd" {
		fs.Debugf(f, "About shell command is not available for shell type %q (set option shell_type to override)", f.shellType)
		return nil, fmt.Errorf("not supported with shell type %q", f.shellType)
	}
	aboutShellPath := f.remoteShellPath("")
	if aboutShellPath == "" {
		aboutShellPath = "/"
	}
	fs.Debugf(f, "About path %q", aboutShellPath)
	aboutShellPathArg, err := f.quoteOrEscapeShellPath(aboutShellPath)
	if err != nil {
		return nil, err
	}
	// PowerShell
	if f.shellType == "powershell" {
		shellCmd := "Get-Item " + aboutShellPathArg + " -ErrorAction Stop|Select-Object -First 1 -ExpandProperty PSDrive|ForEach-Object{\"$($_.Used) $($_.Free)\"}"
		fs.Debugf(f, "About using shell command for shell type %q", f.shellType)
		stdout, err := f.run(ctx, shellCmd)
		if err != nil {
			fs.Debugf(f, "About shell command for shell type %q failed (set option shell_type to override): %v", f.shellType, err)
			return nil, fmt.Errorf("powershell command failed: %w", err)
		}
		split := strings.Fields(string(stdout))
		usage := &fs.Usage{}
		if len(split) == 2 {
			usedValue, usedErr := strconv.ParseInt(split[0], 10, 64)
			if usedErr == nil {
				usage.Used = fs.NewUsageValue(usedValue)
			}
			freeValue, freeErr := strconv.ParseInt(split[1], 10, 64)
			if freeErr == nil {
				usage.Free = fs.NewUsageValue(freeValue)
				if usedErr == nil {
					usage.Total = fs.NewUsageValue(usedValue + freeValue)
				}
			}
		}
		return usage, nil
	}
	// Unix/default shell
	shellCmd := "df -k " + aboutShellPathArg
	fs.Debugf(f, "About using shell command for shell type %q", f.shellType)
	stdout, err := f.run(ctx, shellCmd)
	if err != nil {
		fs.Debugf(f, "About shell command for shell type %q failed (set option shell_type to override): %v", f.shellType, err)
		return nil, fmt.Errorf("your remote may not have the required df utility: %w", err)
	}
	usageTotal, usageUsed, usageAvail := parseUsage(stdout)
	usage := &fs.Usage{}
	if usageTotal >= 0 {
		usage.Total = fs.NewUsageValue(usageTotal)
	}
	if usageUsed >= 0 {
		usage.Used = fs.NewUsageValue(usageUsed)
	}
	if usageAvail >= 0 {
		usage.Free = fs.NewUsageValue(usageAvail)
	}
	return usage, nil
}

// Shutdown the backend, closing any background tasks and any
// cached connections.
func (f *Fs) Shutdown(ctx context.Context) error {
	return f.drainPool(ctx)
}

// Fs is the filesystem this remote sftp file object is located within
func (o *Object) Fs() fs.Info {
	return o.fs
}

// String returns the URL to the remote SFTP file
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote the name of the remote SFTP file, relative to the fs root
func (o *Object) Remote() string {
	return o.remote
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *Object) Hash(ctx context.Context, r hash.Type) (string, error) {
	if o.fs.opt.DisableHashCheck {
		return "", nil
	}
	_ = o.fs.Hashes()

	var hashCmd string
	if r == hash.MD5 {
		if o.md5sum != nil {
			return *o.md5sum, nil
		}
		hashCmd = o.fs.opt.Md5sumCommand
	} else if r == hash.SHA1 {
		if o.sha1sum != nil {
			return *o.sha1sum, nil
		}
		hashCmd = o.fs.opt.Sha1sumCommand
	} else {
		return "", hash.ErrUnsupported
	}
	if hashCmd == "" || hashCmd == hashCommandNotSupported {
		return "", hash.ErrUnsupported
	}

	shellPathArg, err := o.fs.quoteOrEscapeShellPath(o.shellPath())
	if err != nil {
		return "", fmt.Errorf("failed to calculate %v hash: %w", r, err)
	}
	outBytes, err := o.fs.run(ctx, hashCmd+" "+shellPathArg)
	if err != nil {
		return "", fmt.Errorf("failed to calculate %v hash: %w", r, err)
	}
	hashString := parseHash(outBytes)
	fs.Debugf(o, "Parsed hash: %s", hashString)
	if r == hash.MD5 {
		o.md5sum = &hashString
	} else if r == hash.SHA1 {
		o.sha1sum = &hashString
	}
	return hashString, nil
}

// quoteOrEscapeShellPath makes path a valid string argument in configured shell
// and also ensures it cannot cause unintended behavior.
func quoteOrEscapeShellPath(shellType string, shellPath string) (string, error) {
	// PowerShell
	if shellType == "powershell" {
		return "'" + strings.ReplaceAll(shellPath, "'", "''") + "'", nil
	}
	// Windows Command Prompt
	if shellType == "cmd" {
		if strings.Contains(shellPath, "\"") {
			return "", fmt.Errorf("path is not valid in shell type %s: %s", shellType, shellPath)
		}
		return "\"" + shellPath + "\"", nil
	}
	// Unix shell
	safe := unixShellEscapeRegex.ReplaceAllString(shellPath, `\$0`)
	return strings.ReplaceAll(safe, "\n", "'\n'"), nil
}

// quoteOrEscapeShellPath makes path a valid string argument in configured shell
func (f *Fs) quoteOrEscapeShellPath(shellPath string) (string, error) {
	return quoteOrEscapeShellPath(f.shellType, shellPath)
}

// remotePath returns the native SFTP path of the file or directory at the remote given
func (f *Fs) remotePath(remote string) string {
	return path.Join(f.absRoot, remote)
}

// remoteShellPath returns the SSH shell path of the file or directory at the remote given
func (f *Fs) remoteShellPath(remote string) string {
	if f.opt.PathOverride != "" {
		shellPath := path.Join(f.opt.PathOverride, remote)
		if f.opt.PathOverride[0] == '@' {
			shellPath = path.Join(strings.TrimPrefix(f.opt.PathOverride, "@"), f.absRoot, remote)
		}
		fs.Debugf(f, "Shell path redirected to %q with option path_override", shellPath)
		return shellPath
	}
	shellPath := path.Join(f.absRoot, remote)
	if f.shellType == "powershell" || f.shellType == "cmd" {
		// If remote shell is powershell or cmd, then server is probably Windows.
		// The sftp package converts everything to POSIX paths: Forward slashes, and
		// absolute paths starts with a slash. An absolute path on a Windows server will
		// then look like this "/C:/Windows/System32". We must remove the "/" prefix
		// to make this a valid path for shell commands. In case of PowerShell there is a
		// possibility that it is a Unix server, with PowerShell Core shell, but assuming
		// root folders with names such as "C:" are rare, we just take this risk,
		// and option path_override can always be used to work around corner cases.
		if posixWinAbsPathRegex.MatchString(shellPath) {
			shellPath = strings.TrimPrefix(shellPath, "/")
			fs.Debugf(f, "Shell path adjusted to %q (set option path_override to override)", shellPath)
			return shellPath
		}
	}
	fs.Debugf(f, "Shell path %q", shellPath)
	return shellPath
}

// Converts a byte array from the SSH session returned by
// an invocation of md5sum/sha1sum to a hash string
// as expected by the rest of this application
func parseHash(bytes []byte) string {
	// For strings with backslash *sum writes a leading \
	// https://unix.stackexchange.com/q/313733/94054
	return strings.ToLower(strings.Split(strings.TrimLeft(string(bytes), "\\"), " ")[0]) // Split at hash / filename separator / all convert to lowercase
}

// Parses the byte array output from the SSH session
// returned by an invocation of df into
// the disk size, used space, and available space on the disk, in that order.
// Only works when `df` has output info on only one disk
func parseUsage(bytes []byte) (spaceTotal int64, spaceUsed int64, spaceAvail int64) {
	spaceTotal, spaceUsed, spaceAvail = -1, -1, -1
	lines := strings.Split(string(bytes), "\n")
	if len(lines) < 2 {
		return
	}
	split := strings.Fields(lines[1])
	if len(split) < 6 {
		return
	}
	spaceTotal, err := strconv.ParseInt(split[1], 10, 64)
	if err != nil {
		spaceTotal = -1
	}
	spaceUsed, err = strconv.ParseInt(split[2], 10, 64)
	if err != nil {
		spaceUsed = -1
	}
	spaceAvail, err = strconv.ParseInt(split[3], 10, 64)
	if err != nil {
		spaceAvail = -1
	}
	return spaceTotal * 1024, spaceUsed * 1024, spaceAvail * 1024
}

// Size returns the size in bytes of the remote sftp file
func (o *Object) Size() int64 {
	return o.size
}

// ModTime returns the modification time of the remote sftp file
func (o *Object) ModTime(ctx context.Context) time.Time {
	return time.Unix(int64(o.modTime), 0)
}

// path returns the native SFTP path of the object
func (o *Object) path() string {
	return o.fs.remotePath(o.remote)
}

// shellPath returns the SSH shell path of the object
func (o *Object) shellPath() string {
	return o.fs.remoteShellPath(o.remote)
}

// setMetadata updates the info in the object from the stat result passed in
func (o *Object) setMetadata(info os.FileInfo) {
	o.modTime = info.Sys().(*sftp.FileStat).Mtime
	o.size = info.Size()
	o.mode = info.Mode()
}

// statRemote stats the file or directory at the remote given
func (f *Fs) stat(ctx context.Context, remote string) (info os.FileInfo, err error) {
	absPath := remote
	if !strings.HasPrefix(remote, "/") {
		absPath = path.Join(f.absRoot, remote)
	}
	c, err := f.getSftpConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("stat: %w", err)
	}
	info, err = c.sftpClient.Stat(absPath)
	f.putSftpConnection(&c, err)
	return info, err
}

// stat updates the info in the Object
func (o *Object) stat(ctx context.Context) error {
	info, err := o.fs.stat(ctx, o.remote)
	if err != nil {
		if os.IsNotExist(err) {
			return fs.ErrorObjectNotFound
		}
		return fmt.Errorf("stat failed: %w", err)
	}
	if info.IsDir() {
		return fs.ErrorIsDir
	}
	o.setMetadata(info)
	return nil
}

// SetModTime sets the modification and access time to the specified time
//
// it also updates the info field
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	if !o.fs.opt.SetModTime {
		return nil
	}
	c, err := o.fs.getSftpConnection(ctx)
	if err != nil {
		return fmt.Errorf("SetModTime: %w", err)
	}
	err = c.sftpClient.Chtimes(o.path(), modTime, modTime)
	o.fs.putSftpConnection(&c, err)
	if err != nil {
		return fmt.Errorf("SetModTime failed: %w", err)
	}
	err = o.stat(ctx)
	if err != nil && err != fs.ErrorIsDir {
		return fmt.Errorf("SetModTime stat failed: %w", err)
	}
	return nil
}

// Storable returns whether the remote sftp file is a regular file (not a directory, symbolic link, block device, character device, named pipe, etc.)
func (o *Object) Storable() bool {
	return o.mode.IsRegular()
}

// objectReader represents a file open for reading on the SFTP server
type objectReader struct {
	f          *Fs
	sftpFile   *sftp.File
	pipeReader *io.PipeReader
	done       chan struct{}
}

func (f *Fs) newObjectReader(sftpFile *sftp.File) *objectReader {
	pipeReader, pipeWriter := io.Pipe()
	file := &objectReader{
		f:          f,
		sftpFile:   sftpFile,
		pipeReader: pipeReader,
		done:       make(chan struct{}),
	}
	// Show connection in use
	f.addSession()

	go func() {
		// Use sftpFile.WriteTo to pump data so that it gets a
		// chance to build the window up.
		_, err := sftpFile.WriteTo(pipeWriter)
		// Close the pipeWriter so the pipeReader fails with
		// the same error or EOF if err == nil
		_ = pipeWriter.CloseWithError(err)
		// signal that we've finished
		close(file.done)
	}()

	return file
}

// Read from a remote sftp file object reader
func (file *objectReader) Read(p []byte) (n int, err error) {
	n, err = file.pipeReader.Read(p)
	return n, err
}

// Close a reader of a remote sftp file
func (file *objectReader) Close() (err error) {
	// Close the sftpFile - this will likely cause the WriteTo to error
	err = file.sftpFile.Close()
	// Close the pipeReader so writes to the pipeWriter fail
	_ = file.pipeReader.Close()
	// Wait for the background process to finish
	<-file.done
	// Show connection no longer in use
	file.f.removeSession()
	return err
}

// Open a remote sftp file object for reading. Seek is supported
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	var offset, limit int64 = 0, -1
	for _, option := range options {
		switch x := option.(type) {
		case *fs.SeekOption:
			offset = x.Offset
		case *fs.RangeOption:
			offset, limit = x.Decode(o.Size())
		default:
			if option.Mandatory() {
				fs.Logf(o, "Unsupported mandatory option: %v", option)
			}
		}
	}
	c, err := o.fs.getSftpConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("Open: %w", err)
	}
	sftpFile, err := c.sftpClient.Open(o.path())
	o.fs.putSftpConnection(&c, err)
	if err != nil {
		return nil, fmt.Errorf("Open failed: %w", err)
	}
	if offset > 0 {
		off, err := sftpFile.Seek(offset, io.SeekStart)
		if err != nil || off != offset {
			return nil, fmt.Errorf("Open Seek failed: %w", err)
		}
	}
	in = readers.NewLimitedReadCloser(o.fs.newObjectReader(sftpFile), limit)
	return in, nil
}

type sizeReader struct {
	io.Reader
	size int64
}

// Size returns the expected size of the stream
//
// It is used in sftpFile.ReadFrom as a hint to work out the
// concurrency needed
func (sr *sizeReader) Size() int64 {
	return sr.size
}

// Update a remote sftp file using the data <in> and ModTime from <src>
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	o.fs.addSession() // Show session in use
	defer o.fs.removeSession()
	// Clear the hash cache since we are about to update the object
	o.md5sum = nil
	o.sha1sum = nil
	c, err := o.fs.getSftpConnection(ctx)
	if err != nil {
		return fmt.Errorf("Update: %w", err)
	}
	// Hang on to the connection for the whole upload so it doesn't get reused while we are uploading
	file, err := c.sftpClient.OpenFile(o.path(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		o.fs.putSftpConnection(&c, err)
		return fmt.Errorf("Update Create failed: %w", err)
	}
	// remove the file if upload failed
	remove := func() {
		c, removeErr := o.fs.getSftpConnection(ctx)
		if removeErr != nil {
			fs.Debugf(src, "Failed to open new SSH connection for delete: %v", removeErr)
			return
		}
		removeErr = c.sftpClient.Remove(o.path())
		o.fs.putSftpConnection(&c, removeErr)
		if removeErr != nil {
			fs.Debugf(src, "Failed to remove: %v", removeErr)
		} else {
			fs.Debugf(src, "Removed after failed upload: %v", err)
		}
	}
	_, err = file.ReadFrom(&sizeReader{Reader: in, size: src.Size()})
	if err != nil {
		o.fs.putSftpConnection(&c, err)
		remove()
		return fmt.Errorf("Update ReadFrom failed: %w", err)
	}
	err = file.Close()
	if err != nil {
		o.fs.putSftpConnection(&c, err)
		remove()
		return fmt.Errorf("Update Close failed: %w", err)
	}
	// Release connection only when upload has finished so we don't upload multiple files on the same connection
	o.fs.putSftpConnection(&c, err)

	// Set the mod time - this stats the object if o.fs.opt.SetModTime == true
	err = o.SetModTime(ctx, src.ModTime(ctx))
	if err != nil {
		return fmt.Errorf("Update SetModTime failed: %w", err)
	}

	// Stat the file after the upload to read its stats back if o.fs.opt.SetModTime == false
	if !o.fs.opt.SetModTime {
		err = o.stat(ctx)
		if err == fs.ErrorObjectNotFound {
			// In the specific case of o.fs.opt.SetModTime == false
			// if the object wasn't found then don't return an error
			fs.Debugf(o, "Not found after upload with set_modtime=false so returning best guess")
			o.modTime = uint32(src.ModTime(ctx).Unix())
			o.size = src.Size()
			o.mode = os.FileMode(0666) // regular file
		} else if err != nil {
			return fmt.Errorf("Update stat failed: %w", err)
		}
	}

	return nil
}

// Remove a remote sftp file object
func (o *Object) Remove(ctx context.Context) error {
	c, err := o.fs.getSftpConnection(ctx)
	if err != nil {
		return fmt.Errorf("Remove: %w", err)
	}
	err = c.sftpClient.Remove(o.path())
	o.fs.putSftpConnection(&c, err)
	return err
}

// Check the interfaces are satisfied
var (
	_ fs.Fs             = &Fs{}
	_ fs.PutStreamer    = &Fs{}
	_ fs.Mover          = &Fs{}
	_ fs.Copier         = &Fs{}
	_ fs.DirMover       = &Fs{}
	_ fs.DirSetModTimer = &Fs{}
	_ fs.Abouter        = &Fs{}
	_ fs.Shutdowner     = &Fs{}
	_ fs.Object         = &Object{}
)
