package fs

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"io"
	"log"
	"os"
	"testing"
)

func setupLogging() {
	globalConfig = &ConfigInfo{
		LogLevel: LogLevelDebug,
	}
}

func captureLogging(print func()) string {
	// keep backup of the real stdout
	old := log.Default().Writer()
	r, w, _ := os.Pipe()
	log.Default().SetOutput(w)

	print()

	outC := make(chan string)

	// send stdout to channel
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.String()
	}()

	// restore
	w.Close()
	log.Default().SetOutput(old)
	return <-outC
}

const Value = "MY_VALUE"
const Section = "s3"

func TestConfigEnvVarsNotLoggingSensitiveValues(t *testing.T) {
	setupLogging()

	fsInfo := &RegInfo{
		Options: []Option{
			{Name: "access_key_id", IsPassword: true},
			{Name: "region"},
		},
	}

	cev := configEnvVars{Section, fsInfo}

	for _, option := range fsInfo.Options {
		err := os.Setenv(ConfigToEnv(Section, option.Name), Value)
		assert.NoError(t, err)

		logMessage := captureLogging(func() {
			cev.Get(option.Name)
		})

		if option.IsPassword {
			assert.NotContains(t, logMessage, Value)
		} else {
			assert.Contains(t, logMessage, Value)
		}
	}
}
