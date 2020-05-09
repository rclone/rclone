// +build !plan9

package sftp

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/cmd/serve/proxy/proxyflags"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/lib/env"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfsflags"
	"golang.org/x/crypto/ssh"
)

// server contains everything to run the server
type server struct {
	f        fs.Fs
	opt      Options
	vfs      *vfs.VFS
	config   *ssh.ServerConfig
	listener net.Listener
	waitChan chan struct{} // for waiting on the listener to close
	proxy    *proxy.Proxy
}

func newServer(f fs.Fs, opt *Options) *server {
	s := &server{
		f:        f,
		opt:      *opt,
		waitChan: make(chan struct{}),
	}
	if proxyflags.Opt.AuthProxy != "" {
		s.proxy = proxy.New(&proxyflags.Opt)
	} else {
		s.vfs = vfs.New(f, &vfsflags.Opt)
	}
	return s
}

// getVFS gets the vfs from s or the proxy
func (s *server) getVFS(what string, sshConn *ssh.ServerConn) (VFS *vfs.VFS) {
	if s.proxy == nil {
		return s.vfs
	}
	if sshConn.Permissions == nil && sshConn.Permissions.Extensions == nil {
		fs.Infof(what, "SSH Permissions Extensions not found")
		return nil
	}
	key := sshConn.Permissions.Extensions["_vfsKey"]
	if key == "" {
		fs.Infof(what, "VFS key not found")
		return nil
	}
	VFS = s.proxy.Get(key)
	if VFS == nil {
		fs.Infof(what, "failed to read VFS from cache")
		return nil
	}
	return VFS
}

func (s *server) acceptConnections() {
	for {
		nConn, err := s.listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				return
			}
			fs.Errorf(nil, "Failed to accept incoming connection: %v", err)
			continue
		}
		what := describeConn(nConn)

		// Before use, a handshake must be performed on the incoming net.Conn.
		sshConn, chans, reqs, err := ssh.NewServerConn(nConn, s.config)
		if err != nil {
			fs.Errorf(what, "SSH login failed: %v", err)
			continue
		}

		fs.Infof(what, "SSH login from %s using %s", sshConn.User(), sshConn.ClientVersion())

		// Discard all global out-of-band Requests
		go ssh.DiscardRequests(reqs)

		c := &conn{
			what: what,
			vfs:  s.getVFS(what, sshConn),
		}
		if c.vfs == nil {
			fs.Infof(what, "Closing unauthenticated connection (couldn't find VFS)")
			_ = nConn.Close()
			continue
		}
		c.handlers = newVFSHandler(c.vfs)

		// Accept all channels
		go c.handleChannels(chans)
	}
}

// Based on example server code from golang.org/x/crypto/ssh and server_standalone
func (s *server) serve() (err error) {
	var authorizedKeysMap map[string]struct{}

	// ensure the user isn't trying to use conflicting flags
	if proxyflags.Opt.AuthProxy != "" && s.opt.AuthorizedKeys != "" && s.opt.AuthorizedKeys != DefaultOpt.AuthorizedKeys {
		return errors.New("--auth-proxy and --authorized-keys cannot be used at the same time")
	}

	// Load the authorized keys
	if s.opt.AuthorizedKeys != "" && proxyflags.Opt.AuthProxy == "" {
		authKeysFile := env.ShellExpand(s.opt.AuthorizedKeys)
		authorizedKeysMap, err = loadAuthorizedKeys(authKeysFile)
		// If user set the flag away from the default then report an error
		if err != nil && s.opt.AuthorizedKeys != DefaultOpt.AuthorizedKeys {
			return err
		}
		fs.Logf(nil, "Loaded %d authorized keys from %q", len(authorizedKeysMap), authKeysFile)
	}

	if !s.opt.NoAuth && len(authorizedKeysMap) == 0 && s.opt.User == "" && s.opt.Pass == "" && s.proxy == nil {
		return errors.New("no authorization found, use --user/--pass or --authorized-keys or --no-auth or --auth-proxy")
	}

	// An SSH server is represented by a ServerConfig, which holds
	// certificate details and handles authentication of ServerConns.
	s.config = &ssh.ServerConfig{
		ServerVersion: "SSH-2.0-" + fs.Config.UserAgent,
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			fs.Debugf(describeConn(c), "Password login attempt for %s", c.User())
			if s.proxy != nil {
				// query the proxy for the config
				_, vfsKey, err := s.proxy.Call(c.User(), string(pass), false)
				if err != nil {
					return nil, err
				}
				// just return the Key so we can get it back from the cache
				return &ssh.Permissions{
					Extensions: map[string]string{
						"_vfsKey": vfsKey,
					},
				}, nil
			} else if s.opt.User != "" && s.opt.Pass != "" {
				userOK := subtle.ConstantTimeCompare([]byte(c.User()), []byte(s.opt.User))
				passOK := subtle.ConstantTimeCompare(pass, []byte(s.opt.Pass))
				if (userOK & passOK) == 1 {
					return nil, nil
				}
			}
			return nil, fmt.Errorf("password rejected for %q", c.User())
		},
		PublicKeyCallback: func(c ssh.ConnMetadata, pubKey ssh.PublicKey) (*ssh.Permissions, error) {
			fs.Debugf(describeConn(c), "Public key login attempt for %s", c.User())
			if s.proxy != nil {
				//query the proxy for the config
				_, vfsKey, err := s.proxy.Call(
					c.User(),
					base64.StdEncoding.EncodeToString(pubKey.Marshal()),
					true,
				)
				if err != nil {
					return nil, err
				}
				// just return the Key so we can get it back from the cache
				return &ssh.Permissions{
					Extensions: map[string]string{
						"_vfsKey": vfsKey,
					},
				}, nil
			}
			if _, ok := authorizedKeysMap[string(pubKey.Marshal())]; ok {
				return &ssh.Permissions{
					// Record the public key used for authentication.
					Extensions: map[string]string{
						"pubkey-fp": ssh.FingerprintSHA256(pubKey),
					},
				}, nil
			}
			return nil, fmt.Errorf("unknown public key for %q", c.User())
		},
		AuthLogCallback: func(conn ssh.ConnMetadata, method string, err error) {
			status := "OK"
			if err != nil {
				status = err.Error()
			}
			fs.Debugf(describeConn(conn), "ssh auth %q from %q: %s", method, conn.ClientVersion(), status)
		},
		NoClientAuth: s.opt.NoAuth,
	}

	// Load the private key, from the cache if not explicitly configured
	keyPaths := s.opt.HostKeys
	cachePath := filepath.Join(config.CacheDir, "serve-sftp")
	if len(keyPaths) == 0 {
		keyPaths = []string{filepath.Join(cachePath, "id_rsa")}
	}
	for _, keyPath := range keyPaths {
		private, err := loadPrivateKey(keyPath)
		if err != nil && len(s.opt.HostKeys) == 0 {
			fs.Debugf(nil, "Failed to load %q: %v", keyPath, err)
			// If loading a cached key failed, make the keys and retry
			err = os.MkdirAll(cachePath, 0700)
			if err != nil {
				return errors.Wrap(err, "failed to create cache path")
			}
			const bits = 2048
			fs.Logf(nil, "Generating %d bit key pair at %q", bits, keyPath)
			err = makeSSHKeyPair(bits, keyPath+".pub", keyPath)
			if err != nil {
				return errors.Wrap(err, "failed to create SSH key pair")
			}
			// reload the new keys
			private, err = loadPrivateKey(keyPath)
		}
		if err != nil {
			return err
		}
		fs.Debugf(nil, "Loaded private key from %q", keyPath)

		s.config.AddHostKey(private)
	}

	// Once a ServerConfig has been configured, connections can be
	// accepted.
	s.listener, err = net.Listen("tcp", s.opt.ListenAddr)
	if err != nil {
		return errors.Wrap(err, "failed to listen for connection")
	}
	fs.Logf(nil, "SFTP server listening on %v\n", s.listener.Addr())

	go s.acceptConnections()

	return nil
}

// Addr returns the address the server is listening on
func (s *server) Addr() string {
	return s.listener.Addr().String()
}

// Serve runs the sftp server in the background.
//
// Use s.Close() and s.Wait() to shutdown server
func (s *server) Serve() error {
	err := s.serve()
	if err != nil {
		return err
	}
	return nil
}

// Wait blocks while the listener is open.
func (s *server) Wait() {
	<-s.waitChan
}

// Close shuts the running server down
func (s *server) Close() {
	err := s.listener.Close()
	if err != nil {
		fs.Errorf(nil, "Error on closing SFTP server: %v", err)
		return
	}
	close(s.waitChan)
}

func loadPrivateKey(keyPath string) (ssh.Signer, error) {
	privateBytes, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load private key")
	}
	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse private key")
	}
	return private, nil
}

// Public key authentication is done by comparing
// the public key of a received connection
// with the entries in the authorized_keys file.
func loadAuthorizedKeys(authorizedKeysPath string) (authorizedKeysMap map[string]struct{}, err error) {
	authorizedKeysBytes, err := ioutil.ReadFile(authorizedKeysPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load authorized keys")
	}
	authorizedKeysMap = make(map[string]struct{})
	for len(authorizedKeysBytes) > 0 {
		pubKey, _, _, rest, err := ssh.ParseAuthorizedKey(authorizedKeysBytes)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse authorized keys")
		}
		authorizedKeysMap[string(pubKey.Marshal())] = struct{}{}
		authorizedKeysBytes = bytes.TrimSpace(rest)
	}
	return authorizedKeysMap, nil
}

// makeSSHKeyPair make a pair of public and private keys for SSH access.
// Public key is encoded in the format for inclusion in an OpenSSH authorized_keys file.
// Private Key generated is PEM encoded
//
// Originally from: https://stackoverflow.com/a/34347463/164234
func makeSSHKeyPair(bits int, pubKeyPath, privateKeyPath string) (err error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return err
	}

	// generate and write private key as PEM
	privateKeyFile, err := os.OpenFile(privateKeyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer fs.CheckClose(privateKeyFile, &err)
	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	if err := pem.Encode(privateKeyFile, privateKeyPEM); err != nil {
		return err
	}

	// generate and write public key
	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(pubKeyPath, ssh.MarshalAuthorizedKey(pub), 0644)
}
