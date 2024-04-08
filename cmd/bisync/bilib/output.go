// Package bilib provides common stuff for bisync and bisync_test
package bilib

import (
	"bytes"
	"log"

	"github.com/rclone/rclone/fs"
	"github.com/sirupsen/logrus"
)

// CaptureOutput runs a function capturing its output.
func CaptureOutput(fun func()) []byte {
	logSave := log.Writer()
	logrusSave := logrus.StandardLogger().Writer()
	defer func() {
		err := logrusSave.Close()
		if err != nil {
			fs.Errorf(nil, "error closing logrusSave: %v", err)
		}
	}()
	buf := &bytes.Buffer{}
	log.SetOutput(buf)
	logrus.SetOutput(buf)
	fun()
	log.SetOutput(logSave)
	logrus.SetOutput(logrusSave)
	return buf.Bytes()
}
