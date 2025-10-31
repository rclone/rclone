// Package api has general type definitions for doi
package api

// DoiResolverResponse is returned by the DOI resolver API
//
// Reference: https://www.doi.org/the-identifier/resources/factsheets/doi-resolution-documentation
type DoiResolverResponse struct {
	ResponseCode int                        `json:"responseCode"`
	Handle       string                     `json:"handle"`
	Values       []DoiResolverResponseValue `json:"values"`
}

// DoiResolverResponseValue is a single handle record value
type DoiResolverResponseValue struct {
	Index     int                          `json:"index"`
	Type      string                       `json:"type"`
	Data      DoiResolverResponseValueData `json:"data"`
	TTL       int                          `json:"ttl"`
	Timestamp string                       `json:"timestamp"`
}

// DoiResolverResponseValueData is the data held in a handle value
type DoiResolverResponseValueData struct {
	Format string `json:"format"`
	Value  any    `json:"value"`
}
