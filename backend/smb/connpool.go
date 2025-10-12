package smb

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	smb2 "github.com/cloudsoda/go-smb2"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fshttp"
	"golang.org/x/sync/errgroup"
)

// dial starts a client connection to the given SMB server. It is a
// convenience function that connects to the given network address,
// initiates the SMB handshake, and then sets up a Client.
//
// The context is only used for establishing the connection, not after.
func (f *Fs) dial(ctx context.Context, network, addr string) (*conn, error) {
	dialer := fshttp.NewDialer(ctx)
	tconn, err := dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	pass := ""
	if f.opt.Pass != "" {
		pass, err = obscure.Reveal(f.opt.Pass)
		if err != nil {
			return nil, err
		}
	}

	d := &smb2.Dialer{}
	if f.opt.UseKerberos {
		cl, err := NewKerberosFactory().GetClient(f.opt.KerberosCCache)
		if err != nil {
			return nil, err
		}

		spn := f.opt.SPN
		if spn == "" {
			spn = "cifs/" + f.opt.Host
		}

		d.Initiator = &smb2.Krb5Initiator{
			Client:    cl,
			TargetSPN: spn,
		}
	} else {
		d.Initiator = &smb2.NTLMInitiator{
			User:      f.opt.User,
			Password:  pass,
			Domain:    f.opt.Domain,
			TargetSPN: f.opt.SPN,
		}
	}

	session, err := d.DialConn(ctx, tconn, addr)
	if err != nil {
		return nil, err
	}

	return &conn{
		smbSession: session,
		conn:       &tconn,
	}, nil
}

// conn encapsulates a SMB client and corresponding SMB client
type conn struct {
	conn       *net.Conn
	smbSession *smb2.Session
	smbShare   *smb2.Share
	shareName  string
}

// Closes the connection
func (c *conn) close() (err error) {
	if c.smbShare != nil {
		err = c.smbShare.Umount()
	}
	sessionLogoffErr := c.smbSession.Logoff()
	if err != nil {
		return err
	}
	return sessionLogoffErr
}

// True if it's closed
func (c *conn) closed() bool {
	return c.smbSession.Echo() != nil
}

// Show that we are using a SMB session
//
// Call removeSession() when done
func (f *Fs) addSession() {
	f.sessions.Add(1)
}

// Show the SMB session is no longer in use
func (f *Fs) removeSession() {
	f.sessions.Add(-1)
}

// getSessions shows whether there are any sessions in use
func (f *Fs) getSessions() int32 {
	return f.sessions.Load()
}

// Open a new connection to the SMB server.
//
// The context is only used for establishing the connection, not after.
func (f *Fs) newConnection(ctx context.Context, share string) (c *conn, err error) {
	c, err = f.dial(ctx, "tcp", f.opt.Host+":"+f.opt.Port)
	if err != nil {
		return nil, fmt.Errorf("couldn't connect SMB: %w", err)
	}
	if share != "" {
		// mount the specified share as well if user requested
		err = c.mountShare(share)
		if err != nil {
			_ = c.smbSession.Logoff()
			return nil, fmt.Errorf("couldn't initialize SMB: %w", err)
		}
	}
	return c, nil
}

// Ensure the specified share is mounted or the session is unmounted
func (c *conn) mountShare(share string) (err error) {
	if c.shareName == share {
		return nil
	}
	if c.smbShare != nil {
		err = c.smbShare.Umount()
		c.smbShare = nil
	}
	if err != nil {
		return
	}
	if share != "" {
		c.smbShare, err = c.smbSession.Mount(share)
		if err != nil {
			return
		}
	}
	c.shareName = share
	return nil
}

// Get a SMB connection from the pool, or open a new one
func (f *Fs) getConnection(ctx context.Context, share string) (c *conn, err error) {
	accounting.LimitTPS(ctx)
	f.poolMu.Lock()
	for len(f.pool) > 0 {
		c = f.pool[0]
		f.pool = f.pool[1:]
		err = c.mountShare(share)
		if err == nil {
			break
		}
		fs.Debugf(f, "Discarding unusable SMB connection: %v", err)
		c = nil
	}
	f.poolMu.Unlock()
	if c != nil {
		return c, nil
	}
	err = f.pacer.Call(func() (bool, error) {
		c, err = f.newConnection(ctx, share)
		if err != nil {
			return true, err
		}
		return false, nil
	})
	return c, err
}

// Return a SMB connection to the pool
//
// It nils the pointed to connection out so it can't be reused
//
// if err is not nil then it checks the connection is alive using an
// ECHO request
func (f *Fs) putConnection(pc **conn, err error) {
	if pc == nil {
		return
	}
	c := *pc
	if c == nil {
		return
	}
	*pc = nil
	if err != nil {
		// If not a regular SMB error then check the connection
		if !(errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrExist) || errors.Is(err, os.ErrPermission)) {
			echoErr := c.smbSession.Echo()
			if echoErr != nil {
				fs.Debugf(f, "Connection failed, closing: %v", echoErr)
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

	g, _ := errgroup.WithContext(ctx)
	for i, c := range f.pool {
		g.Go(func() (err error) {
			if !c.closed() {
				err = c.close()
			}
			f.pool[i] = nil
			return err
		})
	}
	err = g.Wait()
	f.pool = nil
	return err
}
