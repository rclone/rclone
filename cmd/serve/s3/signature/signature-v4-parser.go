package signature

import (
	"strings"
	"time"
)

// parse credentialHeader string into its structured form.
//
// example: Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request
func parseCredentialHeader(credElement string) (ch credentialHeader, err ErrorCode) {

	creds, err := extractFields(credElement, "Credential")
	if err != ErrNone {
		return ch, err
	}

	credElements := strings.Split(strings.TrimSpace(creds), slashSeparator)
	if len(credElements) < 5 {
		return ch, errCredMalformed
	}

	accessKey := strings.Join(credElements[:len(credElements)-4], slashSeparator)
	credElements = credElements[len(credElements)-4:]
	signDate, e := time.Parse(yyyymmdd, credElements[0])
	if e != nil {
		return ch, errMalformedCredentialDate
	}

	// Save access key id.
	cred := credentialHeader{
		accessKey: accessKey,
		scope: signScope{
			date:    signDate,
			region:  credElements[1],
			service: credElements[2],
			request: credElements[3],
		},
	}

	if len(accessKey) < accessKeyMinLen {
		return ch, errInvalidAccessKeyID
	}

	if cred.scope.service != serviceS3 {
		return ch, errInvalidServiceS3
	}

	if cred.scope.request != "aws4_request" {
		return ch, errInvalidRequestVersion
	}

	return cred, ErrNone
}

// Parse slice of signed headers from signed headers tag.
//
// example: SignedHeaders=host;range;x-amz-date
func parseSignedHeader(hdrElement string) ([]string, ErrorCode) {
	signedHdrFields, err := extractFields(hdrElement, "SignedHeaders")
	if err != ErrNone {
		return nil, err
	}
	signedHeaders := strings.Split(signedHdrFields, ";")
	return signedHeaders, ErrNone
}

// Parse signature from signature tag.
//
// exmaple: Signature=fe5f80f77d5fa3beca038a248ff027d0445342fe2855ddc963176630326f1024
func parseSignature(signElement string) (string, ErrorCode) {
	return extractFields(signElement, "Signature")
}

func extractFields(signElement, fieldName string) (string, ErrorCode) {
	signFields := strings.Split(strings.TrimSpace(signElement), "=")
	if len(signFields) != 2 {
		return "", errMissingFields
	}
	if signFields[0] != fieldName {
		return "", errMissingSignTag
	}
	if signFields[1] == "" {
		return "", errMissingFields
	}
	return signFields[1], ErrNone
}

// Parses signature version '4' header of the following form.
//
//	Authorization: algorithm Credential=accessKeyID/credScope,  SignedHeaders=signedHeaders, Signature=signature
func parseSignV4(v4Auth string) (sv signValues, err ErrorCode) {

	if !strings.HasPrefix(v4Auth, signV4Algorithm) {
		return sv, errUnsupportAlgorithm
	}

	rawCred := strings.ReplaceAll(strings.TrimPrefix(v4Auth, signV4Algorithm), " ", "")
	authFields := strings.Split(strings.TrimSpace(rawCred), ",")
	if len(authFields) != 3 {
		return sv, errMissingFields
	}

	// Initialize signature version '4' structured header.
	signV4Values := signValues{}

	// Save credentail values.
	signV4Values.Credential, err = parseCredentialHeader(authFields[0])
	if err != ErrNone {
		return sv, err
	}

	// Save signed headers.
	signV4Values.SignedHeaders, err = parseSignedHeader(authFields[1])
	if err != ErrNone {
		return sv, err
	}

	// Save signature.
	signV4Values.Signature, err = parseSignature(authFields[2])
	if err != ErrNone {
		return sv, err
	}

	// Return the structure here.
	return signV4Values, ErrNone
}
