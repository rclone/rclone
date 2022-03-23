package sugarsync

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
)

func TestErrorHandler(t *testing.T) {
	for _, test := range []struct {
		name   string
		body   string
		code   int
		status string
		want   string
	}{
		{
			name:   "empty",
			body:   "",
			code:   500,
			status: "internal error",
			want:   `HTTP error 500 (internal error) returned body: ""`,
		},
		{
			name:   "unknown",
			body:   "<h1>unknown</h1>",
			code:   500,
			status: "internal error",
			want:   `HTTP error 500 (internal error) returned body: "<h1>unknown</h1>"`,
		},
		{
			name:   "blank",
			body:   "Nothing here <h3></h3>",
			code:   500,
			status: "internal error",
			want:   `HTTP error 500 (internal error) returned body: "Nothing here <h3></h3>"`,
		},
		{
			name:   "real",
			body:   "<h1>an error</h1>\n<h3>Can not move sync folder.</h3>\n<p>more stuff</p>",
			code:   500,
			status: "internal error",
			want:   `HTTP error 500 (internal error): Can not move sync folder.`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			resp := http.Response{
				Body:       ioutil.NopCloser(bytes.NewBufferString(test.body)),
				StatusCode: test.code,
				Status:     test.status,
			}
			got := errorHandler(&resp)
			assert.Equal(t, test.want, got.Error())
		})
	}
}

// TODO: Test more than just "description" states
func TestRiConfig(t *testing.T) {
	const (
		descriptionCompleteState = "description_complete"
		descriptionState         = "description"
		newDescription           = "New description"
	)
	states := []fstest.ConfigStateTestFixture{
		{
			Name:        "description state",
			Mapper:      configmap.Simple{},
			Input:       fs.ConfigIn{State: descriptionState},
			ExpectState: descriptionCompleteState,
		},
		{
			Name:            "description complete",
			Mapper:          configmap.Simple{},
			Input:           fs.ConfigIn{State: descriptionCompleteState, Result: newDescription},
			ExpectMapper:    configmap.Simple{fs.ConfigDescription: newDescription},
			ExpectNilOutput: true,
		},
	}
	fstest.AssertConfigStates(t, states, riConfig)
}
