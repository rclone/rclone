//go:build !plan9

package sftp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"strings"

	"github.com/pkg/sftp"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/terminal"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"golang.org/x/crypto/ssh"
)

func describeConn(c interface {
	RemoteAddr() net.Addr
	LocalAddr() net.Addr
}) string {
	return fmt.Sprintf("serve sftp %s->%s", c.RemoteAddr(), c.LocalAddr())
}

// Return the exit status of the command
type exitStatus struct {
	RC uint32
}

// The incoming exec command
type execCommand struct {
	Command string
}

var shellUnEscapeRegex = regexp.MustCompile(`\\(.)`)

// Unescape a string that was escaped by rclone
func shellUnEscape(str string) string {
	str = strings.ReplaceAll(str, "'\n'", "\n")
	str = shellUnEscapeRegex.ReplaceAllString(str, `$1`)
	return str
}

// Info about the current connection
type conn struct {
	vfs      *vfs.VFS
	handlers sftp.Handlers
	what     string
}

// execCommand implements an extremely limited number of commands to
// interoperate with the rclone sftp backend
func (c *conn) execCommand(ctx context.Context, out io.Writer, command string) (err error) {
	binary, args := command, ""
	space := strings.Index(command, " ")
	if space >= 0 {
		binary = command[:space]
		args = strings.TrimLeft(command[space+1:], " ")
	}
	args = shellUnEscape(args)
	fs.Debugf(c.what, "exec command: binary = %q, args = %q", binary, args)
	switch binary {
	case "df":
		about := c.vfs.Fs().Features().About
		if about == nil {
			return errors.New("df not supported")
		}
		usage, err := about(ctx)
		if err != nil {
			return fmt.Errorf("about failed: %w", err)
		}
		total, used, free := int64(-1), int64(-1), int64(-1)
		if usage.Total != nil {
			total = *usage.Total / 1024
		}
		if usage.Used != nil {
			used = *usage.Used / 1024
		}
		if usage.Free != nil {
			free = *usage.Free / 1024
		}
		perc := int64(0)
		if total > 0 && used >= 0 {
			perc = (100 * used) / total
		}
		_, err = fmt.Fprintf(out, `		Filesystem                   1K-blocks      Used Available Use%% Mounted on
/dev/root %d %d  %d  %d%% /
`, total, used, free, perc)
		if err != nil {
			return fmt.Errorf("send output failed: %w", err)
		}
	case "md5sum", "sha1sum":
		ht := hash.MD5
		if binary == "sha1sum" {
			ht = hash.SHA1
		}
		if !c.vfs.Fs().Hashes().Contains(ht) {
			return fmt.Errorf("%v hash not supported", ht)
		}
		var hashSum string
		if args == "" {
			// empty hash for no input
			if ht == hash.MD5 {
				hashSum = "d41d8cd98f00b204e9800998ecf8427e"
			} else {
				hashSum = "da39a3ee5e6b4b0d3255bfef95601890afd80709"
			}
			args = "-"
		} else {
			node, err := c.vfs.Stat(args)
			if err != nil {
				return fmt.Errorf("hash failed finding file %q: %w", args, err)
			}
			if node.IsDir() {
				return errors.New("can't hash directory")
			}
			o, ok := node.DirEntry().(fs.ObjectInfo)
			if !ok {
				fs.Debugf(args, "File uploading - reading hash from VFS cache")
				in, err := node.Open(os.O_RDONLY)
				if err != nil {
					return fmt.Errorf("hash vfs open failed: %w", err)
				}
				defer func() {
					_ = in.Close()
				}()
				h, err := hash.NewMultiHasherTypes(hash.NewHashSet(ht))
				if err != nil {
					return fmt.Errorf("hash vfs create multi-hasher failed: %w", err)
				}
				_, err = io.Copy(h, in)
				if err != nil {
					return fmt.Errorf("hash vfs copy failed: %w", err)
				}
				hashSum = h.Sums()[ht]
			} else {
				hashSum, err = o.Hash(ctx, ht)
				if err != nil {
					return fmt.Errorf("hash failed: %w", err)
				}
			}
		}
		_, err = fmt.Fprintf(out, "%s  %s\n", hashSum, args)
		if err != nil {
			return fmt.Errorf("send output failed: %w", err)
		}
	case "echo":
		// Special cases for legacy rclone command detection.
		// Before rclone v1.49.0 the sftp backend used "echo 'abc' | md5sum" when
		// detecting hash support, but was then changed to instead just execute
		// md5sum/sha1sum (without arguments), which is handled above. The following
		// code is therefore only necessary to support rclone versions older than
		// v1.49.0 using a sftp remote connected to a rclone serve sftp instance
		// running a newer version of rclone (e.g. latest).
		switch args {
		case "'abc' | md5sum":
			if c.vfs.Fs().Hashes().Contains(hash.MD5) {
				_, err = fmt.Fprintf(out, "0bee89b07a248e27c83fc3d5951213c1  -\n")
				if err != nil {
					return fmt.Errorf("send output failed: %w", err)
				}
			} else {
				return errors.New("md5 hash not supported")
			}
		case "'abc' | sha1sum":
			if c.vfs.Fs().Hashes().Contains(hash.SHA1) {
				_, err = fmt.Fprintf(out, "03cfd743661f07975fa2f1220c5194cbaff48451  -\n")
				if err != nil {
					return fmt.Errorf("send output failed: %w", err)
				}
			} else {
				return errors.New("sha1 hash not supported")
			}
		default:
			_, err = fmt.Fprintf(out, "%s\n", args)
			if err != nil {
				return fmt.Errorf("send output failed: %w", err)
			}
		}
	default:
		return fmt.Errorf("%q not implemented", command)
	}
	return nil
}

// handle a new incoming channel request
func (c *conn) handleChannel(newChannel ssh.NewChannel) {
	fs.Debugf(c.what, "Incoming channel: %s\n", newChannel.ChannelType())
	if newChannel.ChannelType() != "session" {
		err := newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
		fs.Debugf(c.what, "Unknown channel type: %s\n", newChannel.ChannelType())
		if err != nil {
			fs.Errorf(c.what, "Failed to reject unknown channel: %v", err)
		}
		return
	}
	channel, requests, err := newChannel.Accept()
	if err != nil {
		fs.Errorf(c.what, "could not accept channel: %v", err)
		return
	}
	defer func() {
		err := channel.Close()
		if err != nil && err != io.EOF {
			fs.Debugf(c.what, "Failed to close channel: %v", err)
		}
	}()
	fs.Debugf(c.what, "Channel accepted\n")

	isSFTP := make(chan bool, 1)
	var command execCommand

	// Handle out-of-band requests
	go func(in <-chan *ssh.Request) {
		for req := range in {
			fs.Debugf(c.what, "Request: %v\n", req.Type)
			ok := false
			var subSystemIsSFTP bool
			var reply []byte
			switch req.Type {
			case "subsystem":
				fs.Debugf(c.what, "Subsystem: %s\n", req.Payload[4:])
				if string(req.Payload[4:]) == "sftp" {
					ok = true
					subSystemIsSFTP = true
				}
			case "exec":
				err := ssh.Unmarshal(req.Payload, &command)
				if err != nil {
					fs.Errorf(c.what, "ignoring bad exec command: %v", err)
				} else {
					ok = true
					subSystemIsSFTP = false
				}
			}
			fs.Debugf(c.what, " - accepted: %v\n", ok)
			err = req.Reply(ok, reply)
			if err != nil {
				fs.Errorf(c.what, "Failed to Reply to request: %v", err)
				return
			}
			if ok {
				// Wake up main routine after we have responded
				isSFTP <- subSystemIsSFTP
			}
		}
	}(requests)

	// Wait for either subsystem "sftp" or "exec" request
	if <-isSFTP {
		if err := serveChannel(channel, c.handlers, c.what); err != nil {
			fs.Errorf(c.what, "Failed to serve SFTP: %v", err)
		}
	} else {
		var rc = uint32(0)
		err := c.execCommand(context.TODO(), channel, command.Command)
		if err != nil {
			rc = 1
			_, errPrint := fmt.Fprintf(channel.Stderr(), "%v\n", err)
			if errPrint != nil {
				fs.Errorf(c.what, "Failed to write to stderr: %v", errPrint)
			}
			fs.Debugf(c.what, "command %q failed with error: %v", command.Command, err)
		}
		_, err = channel.SendRequest("exit-status", false, ssh.Marshal(exitStatus{RC: rc}))
		if err != nil {
			fs.Errorf(c.what, "Failed to send exit status: %v", err)
		}
	}
}

// Service the incoming Channel channel in go routine
func (c *conn) handleChannels(chans <-chan ssh.NewChannel) {
	for newChannel := range chans {
		go c.handleChannel(newChannel)
	}
}

func serveChannel(rwc io.ReadWriteCloser, h sftp.Handlers, what string) error {
	fs.Debugf(what, "Starting SFTP server")
	server := sftp.NewRequestServer(rwc, h)
	defer func() {
		err := server.Close()
		if err != nil && err != io.EOF {
			fs.Debugf(what, "Failed to close server: %v", err)
		}
	}()
	err := server.Serve()
	if err != nil && err != io.EOF {
		return fmt.Errorf("completed with error: %w", err)
	}
	fs.Debugf(what, "exited session")
	return nil
}

func serveStdio(f fs.Fs) error {
	if terminal.IsTerminal(int(os.Stdout.Fd())) {
		return errors.New("refusing to run SFTP server directly on a terminal. Please let sshd start rclone, by connecting with sftp or sshfs")
	}
	sshChannel := &stdioChannel{
		stdin:  os.Stdin,
		stdout: os.Stdout,
	}
	handlers := newVFSHandler(vfs.New(f, &vfscommon.Opt))
	return serveChannel(sshChannel, handlers, "stdio")
}

type stdioChannel struct {
	stdin  *os.File
	stdout *os.File
}

func (c *stdioChannel) Read(data []byte) (int, error) {
	return c.stdin.Read(data)
}

func (c *stdioChannel) Write(data []byte) (int, error) {
	return c.stdout.Write(data)
}

func (c *stdioChannel) Close() error {
	err1 := c.stdin.Close()
	err2 := c.stdout.Close()
	if err1 != nil {
		return err1
	}
	return err2
}
