//go:build windows

package gui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/Microsoft/go-winio"
	"github.com/rclone/rclone/lib/atexit"
	"golang.org/x/sys/windows"
)

const desktopLauncherTimeout = 10 * time.Second

type desktopLauncher struct {
	mutex        windows.Handle
	listener     net.Listener
	done         chan struct{}
	urlReady     chan struct{}
	url          string
	urlMu        sync.RWMutex
	urlOnce      sync.Once
	closeOnce    sync.Once
	acceptWG     sync.WaitGroup
	handlerWG    sync.WaitGroup
	atexitHandle atexit.FnHandle
}

func newDesktopLauncher(enabled bool) (*desktopLauncher, bool, error) {
	if !enabled {
		return nil, false, nil
	}
	sid, sessionID, err := desktopLauncherIdentity()
	if err != nil {
		return nil, false, fmt.Errorf("failed to identify Windows user session: %w", err)
	}
	return startDesktopLauncher(
		fmt.Sprintf(`Local\rclone-gui-%s-%d`, sid, sessionID),
		fmt.Sprintf(`\\.\pipe\rclone-gui-%s-%d`, sid, sessionID),
		fmt.Sprintf("D:P(A;;GA;;;%s)", sid),
	)
}

func desktopLauncherIdentity() (string, uint32, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", 0, err
	}
	var sessionID uint32
	if err := windows.ProcessIdToSessionId(uint32(os.Getpid()), &sessionID); err != nil {
		return "", 0, err
	}
	return user.User.Sid.String(), sessionID, nil
}

func startDesktopLauncher(mutexName, pipeName, securityDescriptor string) (*desktopLauncher, bool, error) {
	name, err := windows.UTF16PtrFromString(mutexName)
	if err != nil {
		return nil, false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), desktopLauncherTimeout)
	defer cancel()
	for {
		mutex, mutexErr := windows.CreateMutex(nil, false, name)
		if mutexErr == nil {
			launcher, err := listenDesktopLauncher(mutex, pipeName, securityDescriptor)
			return launcher, false, err
		}
		if !errors.Is(mutexErr, windows.ERROR_ALREADY_EXISTS) {
			return nil, false, mutexErr
		}
		_ = windows.CloseHandle(mutex)
		connected, err := requestDesktopOpen(ctx, pipeName)
		if connected {
			return nil, true, err
		}
		if err := ctx.Err(); err != nil {
			return nil, false, fmt.Errorf("timed out waiting for the running GUI: %w", err)
		}
		select {
		case <-ctx.Done():
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func listenDesktopLauncher(mutex windows.Handle, pipeName, securityDescriptor string) (*desktopLauncher, error) {
	listener, err := winio.ListenPipe(pipeName, &winio.PipeConfig{SecurityDescriptor: securityDescriptor})
	if err != nil {
		_ = windows.CloseHandle(mutex)
		return nil, err
	}
	launcher := &desktopLauncher{
		mutex:    mutex,
		listener: listener,
		done:     make(chan struct{}),
		urlReady: make(chan struct{}),
	}
	launcher.atexitHandle = atexit.Register(launcher.Close)
	launcher.acceptWG.Add(1)
	go launcher.serve()
	return launcher, nil
}

func requestDesktopOpen(ctx context.Context, pipeName string) (bool, error) {
	conn, err := winio.DialPipeContext(ctx, pipeName)
	if err != nil {
		return false, err
	}
	defer func() { _ = conn.Close() }()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if _, err := fmt.Fprintln(conn, "open"); err != nil {
		return true, err
	}
	response, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return true, err
	}
	response = strings.TrimSpace(response)
	if response == "ok" {
		return true, nil
	}
	if message, found := strings.CutPrefix(response, "error "); found {
		return true, errors.New(message)
	}
	return true, fmt.Errorf("unexpected response from running GUI: %q", response)
}

func (l *desktopLauncher) serve() {
	defer l.acceptWG.Done()
	for {
		conn, err := l.listener.Accept()
		if err != nil {
			select {
			case <-l.done:
				return
			default:
				return
			}
		}
		l.handlerWG.Add(1)
		go l.handle(conn)
	}
}

func (l *desktopLauncher) handle(conn net.Conn) {
	defer l.handlerWG.Done()
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(desktopLauncherTimeout))
	command, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil || strings.TrimSpace(command) != "open" {
		return
	}
	select {
	case <-l.urlReady:
	case <-l.done:
		_, _ = fmt.Fprintln(conn, "error GUI is shutting down")
		return
	}
	l.urlMu.RLock()
	loginURL := l.url
	l.urlMu.RUnlock()
	if err := openBrowser(loginURL); err != nil {
		message := strings.NewReplacer("\r", " ", "\n", " ").Replace(err.Error())
		_, _ = fmt.Fprintln(conn, "error "+message)
		return
	}
	_, _ = fmt.Fprintln(conn, "ok")
}

func (l *desktopLauncher) publishURL(loginURL string) {
	l.urlMu.Lock()
	l.url = loginURL
	l.urlMu.Unlock()
	l.urlOnce.Do(func() { close(l.urlReady) })
}

func (l *desktopLauncher) Close() {
	l.closeOnce.Do(func() {
		atexit.Unregister(l.atexitHandle)
		close(l.done)
		_ = l.listener.Close()
		l.acceptWG.Wait()
		l.handlerWG.Wait()
		_ = windows.CloseHandle(l.mutex)
	})
}

func showLauncherError(err error) {
	text, textErr := windows.UTF16PtrFromString("Rclone could not start the GUI:\n\n" + err.Error())
	title, titleErr := windows.UTF16PtrFromString("Rclone GUI")
	if textErr != nil || titleErr != nil {
		return
	}
	messageBox := windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW")
	_, _, _ = messageBox.Call(
		0,
		uintptr(unsafe.Pointer(text)),
		uintptr(unsafe.Pointer(title)),
		0x00000010,
	)
}
