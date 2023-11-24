//go:build !go1.20

package random

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"fmt"
	mathrand "math/rand"
)

// Seed the global math/rand with crypto strong data
//
// This doesn't make it OK to use math/rand in crypto sensitive
// environments - don't do that! However it does help to mitigate the
// problem if that happens accidentally. This would have helped with
// CVE-2020-28924 - #4783
//
// As of Go 1.20 there is no reason to call math/rand.Seed with a
// random value as it is self seeded to a random 64 bit number.
func Seed() error {
	var seed int64
	err := binary.Read(cryptorand.Reader, binary.LittleEndian, &seed)
	if err != nil {
		return fmt.Errorf("failed to read random seed: %w", err)
	}
	mathrand.Seed(seed)
	return nil
}
