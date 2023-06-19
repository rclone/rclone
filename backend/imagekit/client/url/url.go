package url

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