package sugarsync

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"

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
