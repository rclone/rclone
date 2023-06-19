package config

// Cloud defines the cloud configuration required to connect your application to ImageKit.io.
type Cloud struct {
	PrivateKey  string
	PublicKey   string
	UrlEndpoint string
}