package smb2

// struct based implementation; for encoding requests
type Encoder interface {
	Size() int
	Encode(b []byte)
}

// bytes based implementation; for decoding responses
type Decoder interface {
	IsInvalid() bool
	// Decode() Encoder
}
