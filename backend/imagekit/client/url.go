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

type trpos string

const (
	PATH  trpos = "path"
	QUERY trpos = "query"
)

type UrlParam struct {
	Path                string
	Src                 string
	UrlEndpoint         string
	Transformations     []map[string]any
	NamedTransformation string // n-trname

	Signed                 bool
	ExpireSeconds          int64
	TransformationPosition trpos
	QueryParameters        map[string]string
	UnixTime               func() int64
}


// TransformationCode represents mapping between parameter and url prefix code
var TransformationCode = map[string]string{
	"height":                    "h",
	"width":                     "w",
	"aspectRatio":               "ar",
	"quality":                   "q",
	"crop":                      "c",
	"cropMode":                  "cm",
	"x":                         "x",
	"y":                         "y",
	"xc":                        "xc",
	"yc":                        "yc",
	"focus":                     "fo",
	"format":                    "f",
	"radius":                    "r",
	"background":                "bg",
	"border":                    "b",
	"rotation":                  "rt",
	"blur":                      "bl",
	"named":                     "n",
	"overlayX":                  "ox",
	"overlayY":                  "oy",
	"overlayFocus":              "ofo",
	"overlayHeight":             "oh",
	"overlayWidth":              "ow",
	"overlayImage":              "oi",
	"overlayImageX":             "oix",
	"overlayImageY":             "oiy",
	"overlayImageXc":            "oixc",
	"overlayImageYc":            "oiyc",
	"overlayImageAspectRatio":   "oiar",
	"overlayImageBackground":    "oibg",
	"overlayImageBorder":        "oib",
	"overlayImageDPR":           "oidpr",
	"overlayImageQuality":       "oiq",
	"overlayImageCropping":      "oic",
	"overlayImageFocus":         "oifo",
	"overlayImageTrim":          "oit",
	"overlayText":               "ot",
	"overlayTextFontSize":       "ots",
	"overlayTextFontFamily":     "otf",
	"overlayTextColor":          "otc",
	"overlayTextTransparency":   "oa",
	"overlayAlpha":              "oa",
	"overlayTextTypography":     "ott",
	"overlayBackground":         "obg",
	"overlayTextEncoded":        "ote",
	"overlayTextWidth":          "otw",
	"overlayTextBackground":     "otbg",
	"overlayTextPadding":        "otp",
	"overlayTextInnerAlignment": "otia",
	"overlayRadius":             "or",
	"progressive":               "pr",
	"lossless":                  "lo",
	"trim":                      "t",
	"metadata":                  "md",
	"colorProfile":              "cp",
	"defaultImage":              "di",
	"dpr":                       "dpr",
	"effectSharpen":             "e-sharpen",
	"effectUSM":                 "e-usm",
	"effectContrast":            "e-contrast",
	"effectGray":                "e-grayscale",
	"original":                  "orig",
}

// Url generates url from UrlParam
func (ik *ImageKit) Url(params UrlParam) (string, error) {
	var resultUrl string
	var url *neturl.URL
	var err error
	var endpoint = params.UrlEndpoint

	if endpoint == "" {
		endpoint = ik.Config.UrlEndpoint
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
			if params.TransformationPosition == QUERY {
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
		mac := hmac.New(sha1.New, []byte(ik.Config.PrivateKey))
		mac.Write([]byte(path))
		signature := hex.EncodeToString(mac.Sum(nil))

		if strings.Contains(resultUrl, "?") {
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
		prefix, ok := TransformationCode[k]

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