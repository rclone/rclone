package utils

import (
	"strings"

	"github.com/yunify/qingstor-sdk-go/v3/request/errors"
)

// IsMetaDataValid check whether the metadata-KV follows rule in API document
func IsMetaDataValid(XQSMetaData *map[string]string) error {
	XQSMetaDataIsValid := true
	wrongKey := ""
	wrongValue := ""

	metadataValuelength := 0
	metadataKeylength := 0

	for k, v := range *XQSMetaData {
		metadataKeylength += len(k)
		metadataValuelength += len(v)
		startstr := strings.Split(k, "-")
		if len(startstr) < 4 {
			wrongKey = k
			wrongValue = v
			XQSMetaDataIsValid = false
			break
		}
		if startstr[0] != "x" || startstr[1] != "qs" || startstr[2] != "meta" || startstr[3] == "" {
			wrongKey = k
			wrongValue = v
			XQSMetaDataIsValid = false
			break
		}

		for i := 0; i < len(k); i++ {
			ch := k[i]
			if !(ch >= 65 && ch <= 90 || ch >= 97 && ch <= 122 || ch <= 57 && ch >= 48 || ch == 45 || ch == 46) {
				wrongKey = k
				wrongValue = v
				XQSMetaDataIsValid = false
				break
			}
		}
		for i := 0; i < len(v); i++ {
			ch := v[i]
			if ch < 32 || ch > 126 {
				wrongKey = k
				wrongValue = v
				XQSMetaDataIsValid = false
				break
			}
		}
		if metadataKeylength > 512 {
			wrongKey = k
			wrongValue = v
			XQSMetaDataIsValid = false
			break
		}
		if metadataValuelength > 2048 {
			wrongKey = k
			wrongValue = v
			XQSMetaDataIsValid = false
			break
		}
	}

	if !XQSMetaDataIsValid {
		return errors.ParameterValueNotAllowedError{
			ParameterName:  "XQSMetaData",
			ParameterValue: "map[" + wrongKey + "]=" + wrongValue,
			AllowedValues:  []string{"https://docs.qingcloud.com/qingstor/api/common/metadata.html"},
		}
	}
	return nil
}
