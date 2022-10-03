package signature

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	accessKeyMinLen = 3
	// secretKeyMinLen = 8
)

// check if the access key is valid and recognized, additionally
// also returns if the access key is owner/admin.
func checkKeyValid(r *http.Request, accessKey string) (Credentials, bool, ErrorCode) {

	u, ok := credStore.Load(accessKey)
	if !ok {
		return Credentials{}, false, errInvalidAccessKeyID
	}
	return u.(Credentials), true, ErrNone
}

// LoadKeys parse and load accessKey-secretKey pair from user input
//
// example: abc123abc123-ac8bef6aaccd
func LoadKeys(pairString string) {
	pairs := strings.Split(pairString, ",")
	for _, val := range pairs {
		if val == "" {
			continue
		}
		keyPair := strings.Split(val, "-")
		if len(keyPair) != 2 {
			continue
		}
		accessKey := keyPair[0]
		secretKey := keyPair[1]
		credStore.Store(accessKey, Credentials{
			AccessKey: accessKey,
			SecretKey: secretKey,
		})
	}
}

func sumHMAC(key []byte, data []byte) []byte {
	hash := hmac.New(sha256.New, key)
	hash.Write(data)
	return hash.Sum(nil)
}

func contains(slice interface{}, elem interface{}) bool {
	v := reflect.ValueOf(slice)
	if v.Kind() == reflect.Slice {
		for i := 0; i < v.Len(); i++ {
			if v.Index(i).Interface() == elem {
				return true
			}
		}
	}
	return false
}

// encodePath from minio/s3utils.EncodePath

// if object matches reserved string, no need to encode them
var reservedObjectNames = regexp.MustCompile("^[a-zA-Z0-9-_.~/]+$")

// EncodePath encode the strings from UTF-8 byte representations to HTML hex escape sequences
//
// This is necessary since regular url.Parse() and url.Encode() functions do not support UTF-8
// non english characters cannot be parsed due to the nature in which url.Encode() is written
//
// This function on the other hand is a direct replacement for url.Encode() technique to support
// pretty much every UTF-8 character.
func encodePath(pathName string) string {
	if reservedObjectNames.MatchString(pathName) {
		return pathName
	}
	var encodedPathname strings.Builder
	for _, s := range pathName {
		if 'A' <= s && s <= 'Z' || 'a' <= s && s <= 'z' || '0' <= s && s <= '9' { // §2.3 Unreserved characters (mark)
			encodedPathname.WriteRune(s)
			continue
		}
		switch s {
		case '-', '_', '.', '~', '/': // §2.3 Unreserved characters (mark)
			encodedPathname.WriteRune(s)
			continue
		default:
			len := utf8.RuneLen(s)
			if len < 0 {
				// if utf8 cannot convert return the same string as is
				return pathName
			}
			u := make([]byte, len)
			utf8.EncodeRune(u, s)
			for _, r := range u {
				hex := hex.EncodeToString([]byte{r})
				encodedPathname.WriteString("%" + strings.ToUpper(hex))
			}
		}
	}
	return encodedPathname.String()
}
