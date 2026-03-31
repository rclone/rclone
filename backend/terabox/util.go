package terabox

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/textproto"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
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

// TBPath update the path with leading slash "/"
func TBPath(path string) string {
	if len(path) > 0 && path[0:1] != "/" {
		return "/" + path
	} else if len(path) == 0 {
		return "/"
	}

	return path
}

type confSupportedTypes interface {
	string | int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64 | bool | *bool
}

func configSet(name, key string, val any) error {
	switch v := val.(type) {
	case string:
		config.FileSetValue(name, key, v)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		config.FileSetValue(name, key, fmt.Sprintf("%d", v))
	case bool:
		if v {
			config.FileSetValue(name, key, "true")
		} else {
			config.FileSetValue(name, key, "false")
		}
	case time.Time:
		config.FileSetValue(name, key, v.Format(time.RFC3339))
	default:
		return errors.New("unsupported value type")
	}

	return nil
}

// ConfigSet write value to config file
func ConfigSet(name, key string, val any) error {
	if err := configSet(name, key, val); err != nil {
		return err
	}

	config.SaveConfig()
	return nil
}

// ConfigSetExpire write value to config file with expiration date
func ConfigSetExpire(name, key string, val any, expire time.Time) error {
	if err := configSet(name, key, val); err != nil {
		return err
	}

	if err := configSet(name, key+"_expire", expire); err != nil {
		return err
	}

	config.SaveConfig()
	return nil
}

func readConfig[T confSupportedTypes](config configmap.Mapper, key string) (T, bool) {
	v, ok := config.Get(key)
	if !ok {
		return *new(T), false
	}

	switch any(*new(T)).(type) {
	case string:
		return any(v).(T), true

	case int:
		if vInt, err := strconv.Atoi(v); err == nil {
			return any(vInt).(T), true
		}

	case int8:
		if vInt, err := strconv.Atoi(v); err == nil {
			return any(int8(vInt)).(T), true
		}

	case int16:
		if vInt, err := strconv.Atoi(v); err == nil {
			return any(int16(vInt)).(T), true
		}

	case int32:
		if vInt, err := strconv.Atoi(v); err == nil {
			return any(int32(vInt)).(T), true
		}

	case int64:
		if vInt, err := strconv.Atoi(v); err == nil {
			return any(int64(vInt)).(T), true
		}

	case uint:
		if vInt, err := strconv.Atoi(v); err == nil {
			return any(uint(vInt)).(T), true
		}

	case uint8:
		if vInt, err := strconv.Atoi(v); err == nil {
			return any(uint8(vInt)).(T), true
		}

	case uint16:
		if vInt, err := strconv.Atoi(v); err == nil {
			return any(uint16(vInt)).(T), true
		}

	case uint32:
		if vInt, err := strconv.Atoi(v); err == nil {
			return any(uint32(vInt)).(T), true

		}

	case uint64:
		if vInt, err := strconv.Atoi(v); err == nil {
			return any(uint64(vInt)).(T), true
		}

	case bool:
		return any(v == "true").(T), true

	case *bool:
		vb := v == "true"
		return any(&vb).(T), true
	}

	return *new(T), false
}

// ConfigGetDefault read string value from config and convert it into required type
func ConfigGetDefault[T confSupportedTypes](config configmap.Mapper, key string, def T) T {
	if v, ok := readConfig[T](config, key); ok {
		return v
	}

	return def
}

// ConfigGetDefaultNotExpired read string value from config and convert it into required type, return value if not expired
func ConfigGetDefaultNotExpired[T confSupportedTypes](config configmap.Mapper, key string, def T) T {
	if v, ok := readConfig[T](config, key); ok {
		if expStr, ok := readConfig[string](config, key+"_expire"); ok {
			exp, err := time.Parse(time.RFC3339, expStr)
			if err != nil || exp.IsZero() {
				return def
			}

			if exp.After(time.Now()) {
				return v
			}
		}
	}

	return def
}
