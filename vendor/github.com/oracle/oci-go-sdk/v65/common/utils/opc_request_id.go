package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// GenerateOpcRequestId
// Reference: https://confluence.oci.oraclecorp.com/display/DEX/Request+IDs
// Maximum segment length:	32 characters
// Allowed segment contents: regular expression pattern /^[a-zA-Z0-9]{0,32}$/
func GenerateOpcRequestID() string {
	clientId := generateUniqueID()
	stackId := generateUniqueID()
	individualId := generateUniqueID()

	opcRequestId := fmt.Sprintf("%s/%s/%s", clientId, stackId, individualId)

	return opcRequestId
}

func generateUniqueID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return ""
	}

	return hex.EncodeToString(b)
}
