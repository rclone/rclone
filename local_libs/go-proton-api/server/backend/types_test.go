package backend

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestID(t *testing.T) {
	var v ID

	// We can set the ID from a string.
	require.NoError(t, v.FromString("AQIDBA=="))

	// We can get the ID as a string.
	require.Equal(t, "AQIDBA==", v.String())

	// We can get the ID as bytes.
	require.Equal(t, []byte{1, 2, 3, 4}, v.Bytes())

	// The ID is correct.
	require.Equal(t, ID(0x01020304), v)
}
