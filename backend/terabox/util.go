package terabox

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/rest"
)

// NewRequest init new request params
func NewRequest(method, path string) *rest.Opts {
	if strings.HasPrefix(path, "https://") {
		return &rest.Opts{Method: method, RootURL: path, Parameters: url.Values{}}
	}
	return &rest.Opts{Method: method, Path: path, Parameters: url.Values{}}
}

// IsInSlice check is slice contain the elem
func IsInSlice[T comparable](v T, list []T) bool {
	if list == nil {
		return false
	}
	for i := 0; i < len(list); i++ {
		if list[i] == v {
			return true
		}
	}
	return false
}

func debug(opt *Options, level uint8, str string, args ...any) {
	if opt.DebugLevel < level {
		return
	}

	fs.Debugf(nil, str, args...)
}

func getStrBetween(raw, start, end string) string {
	regexPattern := fmt.Sprintf(`%s(.*?)%s`, regexp.QuoteMeta(start), regexp.QuoteMeta(end))
	regex := regexp.MustCompile(regexPattern)
	matches := regex.FindStringSubmatch(raw)
	if len(matches) < 2 {
		return ""
	}
	mid := matches[1]
	return mid
}

func sign(s1, s2 string) string {
	var a = make([]int, 256)
	var p = make([]int, 256)
	var o []byte
	var v = len(s1)

	for q := 0; q < 256; q++ {
		a[q] = int(s1[(q % v) : (q%v)+1][0])
		p[q] = q
	}

	for u, q := 0, 0; q < 256; q++ {
		u = (u + p[q] + a[q]) % 256
		p[q], p[u] = p[u], p[q]
	}

	for i, u, q := 0, 0, 0; q < len(s2); q++ {
		i = (i + 1) % 256
		u = (u + p[i]) % 256
		p[i], p[u] = p[u], p[i]
		k := p[((p[i] + p[u]) % 256)]
		o = append(o, byte(int(s2[q])^k))
	}

	return base64.StdEncoding.EncodeToString(o)
}
