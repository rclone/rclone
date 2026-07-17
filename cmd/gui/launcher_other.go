//go:build !windows

package gui

type desktopLauncher struct{}

func newDesktopLauncher(bool) (*desktopLauncher, bool, error) {
	return nil, false, nil
}

func (l *desktopLauncher) publishURL(string) {}

func (l *desktopLauncher) Close() {}

func showLauncherError(error) {}
