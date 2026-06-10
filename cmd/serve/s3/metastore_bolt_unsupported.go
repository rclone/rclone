//go:build plan9 || js

package s3

import "fmt"

func newBoltMetaStore(path string) (metadataStore, error) {
	return nil, fmt.Errorf("persistent metadata database is not supported on this platform")
}
