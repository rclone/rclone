package terabox

import (
	"encoding/base64"
	"fmt"
	"net/textproto"
	"net/url"
	"regexp"
	"strconv"
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

func valuedCookie(cookie string) string {
	cookie = textproto.TrimString(cookie)
	if len(cookie) > 2 && cookie[0] == '"' && cookie[len(cookie)-1] == '"' {
		cookie = cookie[1 : len(cookie)-1]
	}

	parts := strings.Split(cookie, ";")
	if len(parts) == 1 && !strings.Contains(parts[0], "=") {
		return fmt.Sprintf("ndus=%s; lang=en", parts[0])
	}

	return cookie
}

func decodeMD5(md5 string) string {
	if len(md5) != 32 {
		return md5
	}

	restoredHexChar := fmt.Sprintf("%x", md5[9]-'g')
	o := md5[:9] + restoredHexChar + md5[10:]

	// Apply XOR transformation to each character
	n := make([]byte, 0, 32)
	for i := 0; i < len(o); i++ {
		orig, _ := strconv.ParseInt(string(o[i]), 16, 8)
		xor := int(orig) ^ (i & 15)
		n = append(n, fmt.Sprintf("%x", xor)[0])
	}

	e := string(n[8:16]) + string(n[0:8]) + string(n[24:32]) + string(n[16:24])
	return e
}

func getChunkSize(fileSize int64, isVIP bool) int64 {
	const MiB = 1024 * 1024
	const GiB = 1024 * MiB

	limitSizes := []int64{4, 8, 16, 32, 64, 128}

	if !isVIP {
		return limitSizes[0] * MiB
	}

	for _, limit := range limitSizes {
		if fileSize <= limit*GiB {
			return limit * MiB
		}
	}

	return limitSizes[len(limitSizes)-1] * MiB
}
