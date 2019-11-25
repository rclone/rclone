// +build noencode

package encodings

import (
	"github.com/rclone/rclone/lib/encoder"
)

// Fake encodings used for testing
const (
	Base = encoder.MultiEncoder(
		encoder.EncodeZero |
			encoder.EncodeSlash)
	Display            = Base
	LocalUnix          = Base
	LocalWindows       = Base
	AmazonCloudDrive   = Base
	AzureBlob          = Base
	B2                 = Base
	Box                = Base
	Drive              = Base
	Dropbox            = Base
	FTP                = Base
	Fichier            = Base
	GoogleCloudStorage = Base
	JottaCloud         = Base
	Koofr              = Base
	Mailru             = Base
	Mega               = Base
	OneDrive           = Base
	OpenDrive          = Base
	Pcloud             = Base
	PremiumizeMe       = Base
	Putio              = Base
	QingStor           = Base
	S3                 = Base
	Sharefile          = Base
	Swift              = Base
	Yandex             = Base
)

// ByName returns the encoder for a give backend name or nil
func ByName(name string) encoder.Encoder {
	return Base
}

// Local returns the local encoding for the current platform
func Local() encoder.MultiEncoder {
	return Base
}

// Names returns the list of known encodings as accepted by ByName
func Names() []string {
	return []string{
		"base",
	}
}
