package client

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	neturl "net/url"
	"strconv"
	"strings"
	"time"
)

// URLParam represents parameters for generating url
type URLParam struct {
	Path            string
	Src             string
	URLEndpoint     string
	Signed          bool
	ExpireSeconds   int64
	QueryParameters map[string]string
}

// URL generates url from URLParam
func (ik *ImageKit) URL(params URLParam) (string, error) {
	var resultURL string
	var url *neturl.URL
	var err error
	var endpoint = params.URLEndpoint

	if endpoint == "" {
		endpoint = ik.URLEndpoint
	}

	endpoint = strings.TrimRight(endpoint, "/") + "/"

	if params.QueryParameters == nil {
		params.QueryParameters = make(map[string]string)
	}

	if url, err = neturl.Parse(params.Src); err != nil {
		return "", err
	}

	query := url.Query()

	for k, v := range params.QueryParameters {
		query.Set(k, v)
	}
	url.RawQuery = query.Encode()
	resultURL = url.String()

	if params.Signed {
		now := time.Now().Unix()

		var expires = strconv.FormatInt(now+params.ExpireSeconds, 10)
		var path = strings.Replace(resultURL, endpoint, "", 1)

		path += expires
		mac := hmac.New(sha1.New, []byte(ik.PrivateKey))
		mac.Write([]byte(path))
		signature := hex.EncodeToString(mac.Sum(nil))

		if strings.Contains(resultURL, "?") {
			resultURL = resultURL + "&" + fmt.Sprintf("ik-t=%s&ik-s=%s", expires, signature)
		} else {
			resultURL = resultURL + "?" + fmt.Sprintf("ik-t=%s&ik-s=%s", expires, signature)
		}
	}

	return resultURL, nil
}
