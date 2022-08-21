package structs

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// returns the "%p" representation of the thing passed in
func ptr(p interface{}) string {
	return fmt.Sprintf("%p", p)
}

func TestSetDefaults(t *testing.T) {
	old := http.DefaultTransport.(*http.Transport)
	newT := new(http.Transport)
	SetDefaults(newT, old)
	// Can't use assert.Equal or reflect.DeepEqual for this as it has functions in
	// Check functions by comparing the "%p" representations of them
	assert.Equal(t, ptr(old.Proxy), ptr(newT.Proxy), "when checking .Proxy")
	assert.Equal(t, ptr(old.DialContext), ptr(newT.DialContext), "when checking .DialContext")
	assert.Equal(t, ptr(old.DialTLSContext), ptr(newT.DialTLSContext), "when checking .DialTLSContext")
	assert.Equal(t, old.TLSClientConfig, newT.TLSClientConfig, "when checking .TLSClientConfig")
	assert.Equal(t, old.TLSHandshakeTimeout, newT.TLSHandshakeTimeout, "when checking .TLSHandshakeTimeout")
	assert.Equal(t, old.DisableKeepAlives, newT.DisableKeepAlives, "when checking .DisableKeepAlives")
	assert.Equal(t, old.DisableCompression, newT.DisableCompression, "when checking .DisableCompression")
	assert.Equal(t, old.MaxIdleConns, newT.MaxIdleConns, "when checking .MaxIdleConns")
	assert.Equal(t, old.MaxIdleConnsPerHost, newT.MaxIdleConnsPerHost, "when checking .MaxIdleConnsPerHost")
	assert.Equal(t, old.IdleConnTimeout, newT.IdleConnTimeout, "when checking .IdleConnTimeout")
	assert.Equal(t, old.ResponseHeaderTimeout, newT.ResponseHeaderTimeout, "when checking .ResponseHeaderTimeout")
	assert.Equal(t, old.ExpectContinueTimeout, newT.ExpectContinueTimeout, "when checking .ExpectContinueTimeout")
	assert.Equal(t, old.TLSNextProto, newT.TLSNextProto, "when checking .TLSNextProto")
	assert.Equal(t, old.MaxResponseHeaderBytes, newT.MaxResponseHeaderBytes, "when checking .MaxResponseHeaderBytes")
}

type aType struct {
	Matching      string
	OnlyA         string
	MatchingInt   int
	DifferentType string
}

type bType struct {
	Matching      string
	OnlyB         string
	MatchingInt   int
	DifferentType int
	Unused        string
}

func TestSetFrom(t *testing.T) {
	a := aType{
		Matching:      "a",
		OnlyA:         "onlyA",
		MatchingInt:   1,
		DifferentType: "surprise",
	}

	b := bType{
		Matching:      "b",
		OnlyB:         "onlyB",
		MatchingInt:   2,
		DifferentType: 7,
		Unused:        "Ha",
	}
	bBefore := b

	SetFrom(&a, &b)

	assert.Equal(t, aType{
		Matching:      "b",
		OnlyA:         "onlyA",
		MatchingInt:   2,
		DifferentType: "surprise",
	}, a)

	assert.Equal(t, bBefore, b)
}

func TestSetFromReversed(t *testing.T) {
	a := aType{
		Matching:      "a",
		OnlyA:         "onlyA",
		MatchingInt:   1,
		DifferentType: "surprise",
	}
	aBefore := a

	b := bType{
		Matching:      "b",
		OnlyB:         "onlyB",
		MatchingInt:   2,
		DifferentType: 7,
		Unused:        "Ha",
	}

	SetFrom(&b, &a)

	assert.Equal(t, bType{
		Matching:      "a",
		OnlyB:         "onlyB",
		MatchingInt:   1,
		DifferentType: 7,
		Unused:        "Ha",
	}, b)

	assert.Equal(t, aBefore, a)
}
