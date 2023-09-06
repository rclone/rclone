//go:build !android
// +build !android

package crypto

// GetFilename returns the file name of the message as a string.
func (msg *PlainMessage) GetFilename() string {
	return msg.Filename
}

// GetTime returns the modification time of a file (if provided in the ciphertext).
func (msg *PlainMessage) GetTime() uint32 {
	return msg.Time
}
