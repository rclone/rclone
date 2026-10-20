package oauthutil

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAuthServerHandleAuth checks that only a callback carrying the
// random state may complete the flow, since the callback is an
// unauthenticated request any web page in the user's browser can make.
func TestAuthServerHandleAuth(t *testing.T) {
	for _, test := range []struct {
		name       string
		query      string
		wantStatus int
		wantOK     bool
	}{
		{"CorrectState", "?code=CODE&state=correct-state", http.StatusOK, true},
		{"WrongState", "?code=CODE&state=wrong-state", http.StatusBadRequest, false},
		{"BlankState", "?code=CODE&state=", http.StatusBadRequest, false},
		{"MissingState", "?code=CODE", http.StatusBadRequest, false},
		{"MissingCode", "?state=correct-state", http.StatusBadRequest, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := newAuthServer(&Options{}, "", "correct-state", "")
			w := httptest.NewRecorder()
			s.handleAuth(w, httptest.NewRequest("GET", "/"+test.query, nil))
			assert.Equal(t, test.wantStatus, w.Code)
			result := <-s.result
			assert.Equal(t, test.wantOK, result.OK)
			if test.wantOK {
				assert.Equal(t, "CODE", result.Code)
				assert.Equal(t, "CODE", result.Form.Get("code"))
			} else {
				assert.Equal(t, "", result.Code)
			}
		})
	}
}
