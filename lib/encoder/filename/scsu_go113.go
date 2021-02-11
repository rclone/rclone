// +build go1.13

package filename

import "github.com/dop251/scsu"

func init() {
	scsuDecode = scsu.Decode
	scsuEncodeStrict = scsu.EncodeStrict
}
