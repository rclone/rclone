package doi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseDoi(t *testing.T) {
	// 10.1000/182 -> 10.1000/182
	doi := "10.1000/182"
	parsed := parseDoi(doi)
	assert.Equal(t, "10.1000/182", parsed)

	// https://doi.org/10.1000/182 -> 10.1000/182
	doi = "https://doi.org/10.1000/182"
	parsed = parseDoi(doi)
	assert.Equal(t, "10.1000/182", parsed)

	// https://dx.doi.org/10.1000/182 -> 10.1000/182
	doi = "https://dxdoi.org/10.1000/182"
	parsed = parseDoi(doi)
	assert.Equal(t, "10.1000/182", parsed)

	// doi:10.1000/182 -> 10.1000/182
	doi = "doi:10.1000/182"
	parsed = parseDoi(doi)
	assert.Equal(t, "10.1000/182", parsed)

	// doi://10.1000/182 -> 10.1000/182
	doi = "doi://10.1000/182"
	parsed = parseDoi(doi)
	assert.Equal(t, "10.1000/182", parsed)
}
