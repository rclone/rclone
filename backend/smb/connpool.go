package smb

import (
	"context"
	"fmt"
	"net"
	"time"

	smb2 "github.com/cloudsoda/go-smb2"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fshttp"
)

// dial starts a client connection to the given SMB server. It is a
// convenience function that connects to the given network address,
// initiates the SMB handshake, and then sets up a Client.
func (f *Fs) dial(ctx context.Context, network, addr string) (*conn, error) {
	dialer := fshttp.NewDialer(ctx)
	tconn, err := dialer.Dial(network, addr)
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

	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:      f.opt.User,
			Password:  pass,
			Domain:    f.opt.Domain,
			TargetSPN: f.opt.SPN,
		},
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
	var nopErr error
	if c.smbShare != nil {
		// stat the current directory
		_, nopErr = c.smbShare.Stat(".")
	} else {
		// list the shares
		_, nopErr = c.smbSession.ListSharenames()
	}
	return nopErr != nil
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
func (f *Fs) newConnection(ctx context.Context, share string) (c *conn, err error) {
	// As we are pooling these connections we need to decouple
	// them from the current context
	bgCtx := context.Background()

	c, err = f.dial(bgCtx, "tcp", f.opt.Host+":"+f.opt.Port)
	if err != nil {
		return nil, fmt.Errorf("couldn't connect SMB: %w", err)
	}
	if share != "" {
		// mount the specified share as well if user requested
		c.smbShare, err = c.smbSession.Mount(share)
		if err != nil {
			_ = c.smbSession.Logoff()
			return nil, fmt.Errorf("couldn't initialize SMB: %w", err)
		}
		c.smbShare = c.smbShare.WithContext(bgCtx)
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
func (f *Fs) putConnection(pc **conn) {
	c := *pc
	*pc = nil

	var nopErr error
	if c.smbShare != nil {
		// stat the current directory
		_, nopErr = c.smbShare.Stat(".")
	} else {
		// list the shares
		_, nopErr = c.smbSession.ListSharenames()
	}
	if nopErr != nil {
		fs.Debugf(f, "Connection failed, closing: %v", nopErr)
		_ = c.close()
		return
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
		if !c.closed() {
			cErr := c.close()
			if cErr != nil {
				err = cErr
			}
		}
		f.pool[i] = nil
	}
	f.pool = nil
	return err
}
