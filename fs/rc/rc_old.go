//+build !go1.7

package rc

import (
	"encoding/json"
	"io"
)

// WriteJSON writes JSON in out to w
func WriteJSON(w io.Writer, out Params) error {
	enc := json.NewEncoder(w)
	return enc.Encode(out)
}
