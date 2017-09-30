package utils

import (
	"bytes"
	"os/exec"
)

// GetCommit returns git HEAD (short)
func GetCommit() string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return ""
	}
	return string(out.Bytes()[:7])
}
