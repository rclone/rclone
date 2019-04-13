// +build windows

package open

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	// "syscall"
)

var (
	cmd      = "url.dll,FileProtocolHandler"
	runDll32 = filepath.Join(os.Getenv("SYSTEMROOT"), "System32", "rundll32.exe")
)

func cleaninput(input string) string {
	r := strings.NewReplacer("&", "^&")
	return r.Replace(input)
}

func open(input string) *exec.Cmd {
	cmd := exec.Command(runDll32, cmd, input)
	//cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}

func openWith(input string, appName string) *exec.Cmd {
	cmd := exec.Command("cmd", "/C", "start", "", appName, cleaninput(input))
	//cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}
