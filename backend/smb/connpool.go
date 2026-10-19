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
func (f *Fs) dial(ctx context.Context, network, addr string) (c *conn, err error) {
	dialer := fshttp.NewDialer(ctx)
	tconn, err := dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}
	// Close tconn it if any of the remaining setup fails.
	defer func() {
		if err != nil {
			_ = tconn.Close()
		}
	}()

	pass := ""
	if f.opt.Pass != "" {
		pass, err = obscure.Reveal(f.opt.Pass)
		if err != nil {
			return nil, err
		}
	}

	d := &smb2.Dialer{}
	if f.opt.UseKerberos {
		cl, err := kerberosFactory.GetClient(f.opt.KerberosCCache)
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
			User:        f.opt.User,
			Password:    pass,
			Domain:      f.opt.Domain,
			Workstation: f.opt.Workstation,
			TargetSPN:   f.opt.SPN,
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

// connWaitTimeout is how long getConnection waits for a free slot when
// --max-connections is reached before giving up with an error.
const connWaitTimeout = 60 * time.Second

// errConnWoken is returned by getConnectionWake when it stopped waiting
// for a connection slot because its wake channel fired.
var errConnWoken = errors.New("smb: woken while waiting for a connection")

// notifyConnFree wakes one goroutine waiting for a connection slot
func (f *Fs) notifyConnFree() {
	select {
	case f.connFree <- struct{}{}:
	default:
	}
}

// connLimit returns the --max-connections limit, 0 meaning unlimited.
//
// It is read on every call so a change made with rc options/set
// applies to the running backend.
func (f *Fs) connLimit() int {
	if f.ci != nil {
		return f.ci.MaxConnections
	}
	return f.maxConns
}

// releaseSlot gives back the slot of a connection that was closed or
// never opened.
func (f *Fs) releaseSlot() {
	f.poolMu.Lock()
	f.live--
	f.poolMu.Unlock()
	f.notifyConnFree()
}

// Get a SMB connection from the pool, or open a new one
//
// When --max-connections is set, the total number of live SMB
// connections (pooled + in use) never exceeds it: callers wait for one
// to be returned instead of dialling another session. Servers such as
// Windows desktops only accept around 20 sessions per client.
//
// Callers must not hold a connection while asking for another one,
// or they can deadlock against each other once the limit is reached.
func (f *Fs) getConnection(ctx context.Context, share string) (c *conn, err error) {
	return f.getConnectionWake(ctx, share, nil)
}

// getConnectionWake is getConnection which also stops waiting for a
// free slot, returning errConnWoken, when wake fires. This lets a caller
// that has another way to get a connection (eg a filepool.Pool with a
// handle returned to it) stop waiting.
func (f *Fs) getConnectionWake(ctx context.Context, share string, wake <-chan struct{}) (c *conn, err error) {
	accounting.LimitTPS(ctx)
	var waitStart time.Time
	var timer *time.Timer
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	for {
		var discard []*conn
		f.poolMu.Lock()
		for len(f.pool) > 0 {
			c = f.pool[0]
			f.pool = f.pool[1:]
			err = c.mountShare(share)
			if err == nil {
				break
			}
			fs.Debugf(f, "Discarding unusable SMB connection: %v", err)
			discard = append(discard, c)
			f.live--
			c = nil
		}
		gotSlot := false
		limit := f.connLimit()
		if c == nil && (limit <= 0 || f.live < limit) {
			f.live++
			gotSlot = true
		}
		f.poolMu.Unlock()
		for _, d := range discard {
			go func(d *conn) { _ = d.close() }(d)
		}
		if c != nil {
			return c, nil
		}
		if gotSlot {
			break
		}
		if waitStart.IsZero() {
			waitStart = time.Now()
			fs.Debugf(f, "Waiting for a free SMB connection (limit %d)", limit)
		} else if time.Since(waitStart) > connWaitTimeout {
			return nil, fmt.Errorf("SMB connection limit (%d) reached: no connection became free in %v", limit, connWaitTimeout)
		}
		// Poll as well as waiting for a signal since the signal
		// wakes only one of several waiters.
		if timer == nil {
			timer = time.NewTimer(100 * time.Millisecond)
		} else {
			timer.Reset(100 * time.Millisecond)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-wake:
			return nil, errConnWoken
		case <-f.connFree:
		case <-timer.C:
		}
	}
	err = f.pacer.Call(func() (bool, error) {
		if f.dialConn != nil {
			c, err = f.dialConn(ctx, share)
		} else {
			c, err = f.newConnection(ctx, share)
		}
		if err != nil {
			return true, err
		}
		return false, nil
	})
	if err != nil {
		f.releaseSlot()
	}
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
				f.releaseSlot()
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
	f.notifyConnFree()
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
	f.live -= len(f.pool)
	f.pool = nil
	f.notifyConnFree()
	return err
}
