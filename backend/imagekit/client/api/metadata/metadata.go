package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	neturl "net/url"

	"github.com/rclone/rclone/backend/imagekit/client/api"
	"github.com/rclone/rclone/backend/imagekit/client/config"
)

// API is the main struct for media
type API struct {
	Config config.Configuration
	Client api.HttpClient
}

// MetadataResponse represents main struct of metadata response of the sdk
type MetadataResponse struct {
	Data Metadata
	api.Response
}

// Metadata represents struct of metadata response from api
type Metadata struct {
	Height          int
	Width           int
	Size            int64
	Format          string
	HasColorProfile bool
	Quality         int
	Density         int
	HasTransparency bool
	PHash           string
	Exif            Mexif
}

type Mexif struct {
	Image            ImageExif
	Thumbnail        ThumbnailExif
	Exif             Exif
	Gps              Gps
	Interoperability Interoperability
	Makernote        map[string]interface{}
}

type ImageExif struct {
	Make             string
	Model            string
	Orientation      string
	XResolution      int
	YResolution      int
	ResolutionUnit   int
	Software         string
	ModifyDate       time.Time
	YCbCrPositioning int
	ExifOffset       int
	GPSInfo          int
}

type ThumbnailExif struct {
	Compression     int
	XResolution     int
	YResolution     int
	ResolutionUnit  int
	ThumbnailOffset int
	ThumbnailLength int
}

type Exif struct {
	ExposureTime             time.Time
	FNumber                  float32
	ExposureProgram          int
	ISO                      int
	ExifVersion              string
	DateTimeOriginal         time.Time
	CreateDate               time.Time
	ShutterSpeedValue        float32
	ApertureValue            float32
	ExposureCompensation     int
	MeteringMode             int
	Flash                    int
	FocalLength              int
	SubSEcTime               string
	SubSecTimeOriginal       string
	FlashpixVersion          string
	ColorSpace               int
	ExifImageWidth           int
	ExifImageHeight          int
	InteropOffset            int
	FocalPlaneXResolution    float32
	FocalPlaneYResolution    float32
	FocalPlaneResolutionUnit int
	CustomRendered           int
	ExposureMode             int
	WhiteBalance             int
	SceneCaptutureType       int
}

type Gps struct {
	GPSVersionID []int
}

type Interoperability struct {
	InteropIndex   string
	InteropVersion string
}

func (m *API) get(ctx context.Context, url string, query map[string]string, ms api.MetaSetter) (*http.Response, error) {
	var err error
	urlObj, err := neturl.Parse(api.BuildPath(m.Config.API.MetadataPrefix, url))
	if err != nil {
		return nil, err
	}

	values := urlObj.Query()
	for k, v := range query {
		values.Set(k, v)
	}

	q := values.Encode()

	sUrl := urlObj.String()
	if q != "" {
		sUrl = sUrl + "?" + values.Encode()
	}

	req, err := http.NewRequest(http.MethodGet, sUrl, nil)

	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(m.Config.Cloud.PrivateKey, "")

	resp, err := m.Client.Do(req.WithContext(ctx))
	defer api.DeferredBodyClose(resp)

	api.SetResponseMeta(resp, ms)

	return resp, err
}

// FromFile fetches metadata of media library file
func (m *API) FromFile(ctx context.Context, fileId string) (*MetadataResponse, error) {
	if fileId == "" {
		return nil, errors.New("fileId can not be blank")
	}

	var response = &MetadataResponse{}

	resp, err := m.get(ctx, fmt.Sprintf("files/%s/metadata", fileId), nil, response)

	if err != nil {
		return response, err
	}

	if resp.StatusCode != 200 {
		err = response.ParseError()
	} else {
		err = json.Unmarshal(response.Body(), &response.Data)
	}

	return response, err
}
