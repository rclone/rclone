package config

type API struct {
	Prefix         string `default:"https://api.imagekit.io/v2/"`
	UploadPrefix   string `default:"https://upload.imagekit.io/api/v2/"`
	MetadataPrefix string `default:"https://api.imagekit.io/v1/"`
	Timeout        int64  `default:"60"` // seconds
	UploadTimeout  int64  `upload_timeout"`
}
