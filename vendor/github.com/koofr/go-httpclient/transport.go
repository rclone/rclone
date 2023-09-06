package httpclient

import (
	"crypto/tls"
	"net/http"
)

var HttpTransport = &http.Transport{
	DisableCompression: true,
	Proxy:              http.ProxyFromEnvironment,
}

var HttpClient = &http.Client{
	Transport: HttpTransport,
}

var InsecureTlsConfig = &tls.Config{
	InsecureSkipVerify: true,
}

var InsecureHttpTransport = &http.Transport{
	TLSClientConfig:    InsecureTlsConfig,
	DisableCompression: true,
	Proxy:              http.ProxyFromEnvironment,
}

var InsecureHttpClient = &http.Client{
	Transport: InsecureHttpTransport,
}
