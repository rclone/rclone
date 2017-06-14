package check

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckHostAndPort(t *testing.T) {
	assert.False(t, HostAndPort("127.0.0.1:80:90"))
	assert.False(t, HostAndPort("127.0.0.1"))
	assert.False(t, HostAndPort("mysql"))
	assert.False(t, HostAndPort("mysql:mysql"))
	assert.True(t, HostAndPort("mysql:3306"))
	assert.True(t, HostAndPort("172.16.70.50:6379"))
}
