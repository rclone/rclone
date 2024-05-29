//go:build !plan9 && !js

package azurefiles

import (
	"context"
	"math/rand"
	"strings"
	"testing"

	"github.com/rclone/rclone/fstest/fstests"
	"github.com/stretchr/testify/assert"
)

func (f *Fs) InternalTest(t *testing.T) {
	t.Run("Authentication", f.InternalTestAuth)
}

var _ fstests.InternalTester = (*Fs)(nil)

func (f *Fs) InternalTestAuth(t *testing.T) {
	t.Skip("skipping since this requires authentication credentials which are not part of repo")
	shareName := "test-rclone-oct-2023"
	testCases := []struct {
		name    string
		options *Options
	}{
		{
			name: "ConnectionString",
			options: &Options{
				ShareName:        shareName,
				ConnectionString: "",
			},
		},
		{
			name: "AccountAndKey",
			options: &Options{
				ShareName: shareName,
				Account:   "",
				Key:       "",
			}},
		{
			name: "SASUrl",
			options: &Options{
				ShareName: shareName,
				SASURL:    "",
			}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fs, err := newFsFromOptions(context.TODO(), "TestAzureFiles", "", tc.options)
			assert.NoError(t, err)
			dirName := randomString(10)
			assert.NoError(t, fs.Mkdir(context.TODO(), dirName))
		})
	}
}

const chars = "abcdefghijklmnopqrstuvwzyxABCDEFGHIJKLMNOPQRSTUVWZYX"

func randomString(charCount int) string {
	strBldr := strings.Builder{}
	for i := 0; i < charCount; i++ {
		randPos := rand.Int63n(52)
		strBldr.WriteByte(chars[randPos])
	}
	return strBldr.String()
}
