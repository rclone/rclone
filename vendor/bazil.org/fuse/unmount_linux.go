package fuse

import (
	"bytes"
	"errors"
	"os/exec"
)

func unmount(dir string) error {
	cmd := exec.Command("fusermount", "-u", dir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if len(output) > 0 {
			output = bytes.TrimRight(output, "\n")
			msg := err.Error() + ": " + string(output)
			err = errors.New(msg)
		}
		return err
	}
	return nil
}
