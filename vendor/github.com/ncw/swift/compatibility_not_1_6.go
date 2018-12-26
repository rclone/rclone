// +build !go1.6

package swift

import (
	"net/http"
	"time"
)

const IS_AT_LEAST_GO_16 = false

func SetExpectContinueTimeout(tr *http.Transport, t time.Duration)          {}
func AddExpectAndTransferEncoding(req *http.Request, hasContentLength bool) {}
