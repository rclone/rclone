package signature

import (
	"net/http"
	"strconv"
)

var (
	emptySHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

// extractSignedHeaders extract signed headers from Authorization header
func extractSignedHeaders(signedHeaders []string, r *http.Request) (http.Header, ErrorCode) {
	reqHeaders := r.Header
	reqQueries := r.Form
	// find whether "host" is part of list of signed headers.
	// if not return ErrUnsignedHeaders. "host" is mandatory.
	if !contains(signedHeaders, "host") {
		return nil, errUnsignedHeaders
	}
	extractedSignedHeaders := make(http.Header)
	for _, header := range signedHeaders {
		// `host` will not be found in the headers, can be found in r.Host.
		// but its alway necessary that the list of signed headers containing host in it.
		val, ok := reqHeaders[http.CanonicalHeaderKey(header)]
		if !ok {
			// try to set headers from Query String
			val, ok = reqQueries[header]
		}
		if ok {
			extractedSignedHeaders[http.CanonicalHeaderKey(header)] = val
			continue
		}
		switch header {
		case "expect":
			// Golang http server strips off 'Expect' header, if the
			// client sent this as part of signed headers we need to
			// handle otherwise we would see a signature mismatch.
			// `aws-cli` sets this as part of signed headers.
			//
			// According to
			// http://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.20
			// Expect header is always of form:
			//
			//   Expect       =  "Expect" ":" 1#expectation
			//   expectation  =  "100-continue" | expectation-extension
			//
			// So it safe to assume that '100-continue' is what would
			// be sent, for the time being keep this work around.
			// Adding a *TODO* to remove this later when Golang server
			// doesn't filter out the 'Expect' header.
			extractedSignedHeaders.Set(header, "100-continue")
		case "host":
			// Go http server removes "host" from Request.Header
			extractedSignedHeaders.Set(header, r.Host)
		case "transfer-encoding":
			// Go http server removes "host" from Request.Header
			extractedSignedHeaders[http.CanonicalHeaderKey(header)] = r.TransferEncoding
		case "content-length":
			// Signature-V4 spec excludes Content-Length from signed headers list for signature calculation.
			// But some clients deviate from this rule. Hence we consider Content-Length for signature
			// calculation to be compatible with such clients.
			extractedSignedHeaders.Set(header, strconv.FormatInt(r.ContentLength, 10))
		default:
			return nil, errUnsignedHeaders
		}
	}
	return extractedSignedHeaders, ErrNone
}

// Returns SHA256 for calculating canonical-request.
func getContentSha256Cksum(r *http.Request) string {
	var (
		defaultSha256Cksum string
		v                  []string
		ok                 bool
	)

	defaultSha256Cksum = emptySHA256
	v, ok = r.Header[amzContentSha256]

	// We found 'X-Amz-Content-Sha256' return the captured value.
	if ok {
		return v[0]
	}

	// We couldn't find 'X-Amz-Content-Sha256'.
	return defaultSha256Cksum
}
