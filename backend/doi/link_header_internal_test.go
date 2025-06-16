package doi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseLinkHeader(t *testing.T) {
	header := "<https://zenodo.org/api/records/15063252> ; rel=\"linkset\" ; type=\"application/linkset+json\""
	links := parseLinkHeader(header)
	expected := headerLink{
		Href:   "https://zenodo.org/api/records/15063252",
		Rel:    "linkset",
		Type:   "application/linkset+json",
		Extras: map[string]string{},
	}
	assert.Contains(t, links, expected)

	header = "<https://api.example.com/issues?page=2>; rel=\"prev\", <https://api.example.com/issues?page=4>; rel=\"next\", <https://api.example.com/issues?page=10>; rel=\"last\", <https://api.example.com/issues?page=1>; rel=\"first\""
	links = parseLinkHeader(header)
	expectedList := []headerLink{{
		Href:   "https://api.example.com/issues?page=2",
		Rel:    "prev",
		Type:   "",
		Extras: map[string]string{},
	}, {
		Href:   "https://api.example.com/issues?page=4",
		Rel:    "next",
		Type:   "",
		Extras: map[string]string{},
	}, {
		Href:   "https://api.example.com/issues?page=10",
		Rel:    "last",
		Type:   "",
		Extras: map[string]string{},
	}, {
		Href:   "https://api.example.com/issues?page=1",
		Rel:    "first",
		Type:   "",
		Extras: map[string]string{},
	}}
	assert.Equal(t, links, expectedList)
}
