package readers

// ErrorReader wraps an error to return on Read
type ErrorReader struct {
	Err error
}

// Read always returns the error
func (er ErrorReader) Read(p []byte) (n int, err error) {
	return 0, er.Err
}
