// +build gofuzz

package sftp

import "bytes"

type sinkfuzz struct{}

func (*sinkfuzz) Close() error                { return nil }
func (*sinkfuzz) Write(p []byte) (int, error) { return len(p), nil }

var devnull = &sinkfuzz{}

// To run: go-fuzz-build && go-fuzz
func Fuzz(data []byte) int {
	c, err := NewClientPipe(bytes.NewReader(data), devnull)
	if err != nil {
		return 0
	}
	c.Close()
	return 1
}
