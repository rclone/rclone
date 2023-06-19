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

	ikurl "github.com/rclone/rclone/backend/imagekit/client/url"
)

// Url generates url from UrlParam
func (ik *ImageKit) Url(params ikurl.UrlParam) (string, error) {
	var resultUrl string
	var url *neturl.URL
	var err error
	var endpoint = params.UrlEndpoint

	if endpoint == "" {
		endpoint = ik.Config.Cloud.UrlEndpoint
	}

	endpoint = strings.TrimRight(endpoint, "/") + "/"

	if params.QueryParameters == nil {
		params.QueryParameters = make(map[string]string)
	}

	if params.Src == "" {
		if url, err = neturl.Parse(endpoint); err != nil {
			return "", err
		}

		if params.Transformations == nil {
			if url, err = neturl.Parse(endpoint + params.Path); err != nil {
				return "", err
			}
		} else {
			if params.TransformationPosition == ikurl.QUERY {
				params.QueryParameters["tr"] = joinTransformations(params.Transformations...)
				url, err = neturl.Parse(endpoint + params.Path)

			} else {
				url, err = neturl.Parse(url.String() +
					"tr:" + joinTransformations(params.Transformations...) +
					"/" + strings.TrimLeft(params.Path, "/"))
			}
		}
	} else {
		if url, err = neturl.Parse(params.Src); err != nil {
			return "", err
		}

		if params.Transformations != nil {
			params.QueryParameters["tr"] = joinTransformations(params.Transformations...)
		}
	}

	if err != nil {
		return "", nil
	}

	query := url.Query()

	for k, v := range params.QueryParameters {
		query.Set(k, v)
	}
	url.RawQuery = query.Encode()
	resultUrl = url.String()

	if params.Signed {
		var now int64

		if params.UnixTime == nil {
			now = time.Now().Unix()
		} else {
			now = params.UnixTime()
		}

		var expires = strconv.FormatInt(now+int64(params.ExpireSeconds), 10)
		var path = strings.Replace(resultUrl, endpoint, "", 1)

		path = path + expires
		mac := hmac.New(sha1.New, []byte(ik.Config.Cloud.PrivateKey))
		mac.Write([]byte(path))
		signature := hex.EncodeToString(mac.Sum(nil))

		if strings.Index(resultUrl, "?") > -1 {
			resultUrl = resultUrl + "&" + fmt.Sprintf("ik-t=%s&ik-s=%s", expires, signature)
		} else {
			resultUrl = resultUrl + "?" + fmt.Sprintf("ik-t=%s&ik-s=%s", expires, signature)
		}
	}

	return resultUrl, nil
}

func joinTransformations(args ...map[string]any) string {
	var parts []string

	for _, v := range args {
		parts = append(parts, transform(v))
	}
	return strings.Join(parts, ":")
}

func transform(tr map[string]any) string {
	var parts []string

	for k, v := range tr {
		value := fmt.Sprintf("%v", v)

		if k == "raw" {
			parts = append(parts, value)
			continue
		}
		prefix, ok := ikurl.TransformationCode[k]

		if !ok {
			parts = append(parts, value)
			continue
		}

		if v == "-" {
			parts = append(parts, prefix)
		} else {
			if prefix == "di" || prefix == "oi" {
				value = strings.ReplaceAll(strings.Trim(value, "/"), "/", "@@")
			}
			parts = append(parts, prefix+"-"+value)
		}
	}

	return strings.Join(parts, ",")
}