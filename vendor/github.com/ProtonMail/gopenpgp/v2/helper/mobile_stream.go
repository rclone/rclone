package helper

import (
	"crypto/sha256"
	"hash"
	"io"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/pkg/errors"
)

// Mobile2GoWriter is used to wrap a writer in the mobile app runtime,
// to be usable in the golang runtime (via gomobile).
type Mobile2GoWriter struct {
	writer crypto.Writer
}

// NewMobile2GoWriter wraps a writer to be usable in the golang runtime (via gomobile).
func NewMobile2GoWriter(writer crypto.Writer) *Mobile2GoWriter {
	return &Mobile2GoWriter{writer}
}

// Write writes the data in the provided buffer in the wrapped writer.
// It clones the provided data to prevent errors with garbage collectors.
func (w *Mobile2GoWriter) Write(b []byte) (n int, err error) {
	bufferCopy := clone(b)
	return w.writer.Write(bufferCopy)
}

// Mobile2GoWriterWithSHA256 is used to wrap a writer in the mobile app runtime,
// to be usable in the golang runtime (via gomobile).
// It also computes the SHA256 hash of the data being written on the fly.
type Mobile2GoWriterWithSHA256 struct {
	writer crypto.Writer
	sha256 hash.Hash
}

// NewMobile2GoWriterWithSHA256 wraps a writer to be usable in the golang runtime (via gomobile).
// The wrapper also computes the SHA256 hash of the data being written on the fly.
func NewMobile2GoWriterWithSHA256(writer crypto.Writer) *Mobile2GoWriterWithSHA256 {
	return &Mobile2GoWriterWithSHA256{writer, sha256.New()}
}

// Write writes the data in the provided buffer in the wrapped writer.
// It clones the provided data to prevent errors with garbage collectors.
// It also computes the SHA256 hash of the data being written on the fly.
func (w *Mobile2GoWriterWithSHA256) Write(b []byte) (n int, err error) {
	bufferCopy := clone(b)
	n, err = w.writer.Write(bufferCopy)
	if err == nil {
		hashedTotal := 0
		for hashedTotal < n {
			hashed, err := w.sha256.Write(bufferCopy[hashedTotal:n])
			if err != nil {
				return 0, errors.Wrap(err, "gopenpgp: couldn't hash encrypted data")
			}
			hashedTotal += hashed
		}
	}
	return n, err
}

// GetSHA256 returns the SHA256 hash of the data that's been written so far.
func (w *Mobile2GoWriterWithSHA256) GetSHA256() []byte {
	return w.sha256.Sum(nil)
}

// MobileReader is the interface that readers in the mobile runtime must use and implement.
// This is a workaround to some of the gomobile limitations.
type MobileReader interface {
	Read(max int) (result *MobileReadResult, err error)
}

// MobileReadResult is what needs to be returned by MobileReader.Read.
// The read data is passed as a return value rather than passed as an argument to the reader.
// This avoids problems introduced by gomobile that prevent the use of native golang readers.
type MobileReadResult struct {
	N     int    // N, The number of bytes read
	IsEOF bool   // IsEOF, If true, then the reader has reached the end of the data to read.
	Data  []byte // Data, the data that has been read
}

// NewMobileReadResult initialize a MobileReadResult with the correct values.
// It clones the data to avoid the garbage collector freeing the data too early.
func NewMobileReadResult(n int, eof bool, data []byte) *MobileReadResult {
	return &MobileReadResult{N: n, IsEOF: eof, Data: clone(data)}
}

func clone(src []byte) (dst []byte) {
	dst = make([]byte, len(src))
	copy(dst, src)
	return
}

// Mobile2GoReader is used to wrap a MobileReader in the mobile app runtime,
// to be usable in the golang runtime (via gomobile) as a native Reader.
type Mobile2GoReader struct {
	reader MobileReader
}

// NewMobile2GoReader wraps a MobileReader to be usable in the golang runtime (via gomobile).
func NewMobile2GoReader(reader MobileReader) *Mobile2GoReader {
	return &Mobile2GoReader{reader}
}

// Read reads data from the wrapped MobileReader and copies the read data in the provided buffer.
// It also handles the conversion of EOF to an error.
func (r *Mobile2GoReader) Read(b []byte) (n int, err error) {
	result, err := r.reader.Read(len(b))
	if err != nil {
		return 0, errors.Wrap(err, "gopenpgp: couldn't read from mobile reader")
	}
	n = result.N
	if n > 0 {
		copy(b, result.Data[:n])
	}
	if result.IsEOF {
		err = io.EOF
	}
	return n, err
}

// Go2AndroidReader is used to wrap a native golang Reader in the golang runtime,
// to be usable in the android app runtime (via gomobile).
type Go2AndroidReader struct {
	isEOF  bool
	reader crypto.Reader
}

// NewGo2AndroidReader wraps a native golang Reader to be usable in the mobile app runtime (via gomobile).
// It doesn't follow the standard golang Reader behavior, and returns n = -1 on EOF.
func NewGo2AndroidReader(reader crypto.Reader) *Go2AndroidReader {
	return &Go2AndroidReader{isEOF: false, reader: reader}
}

// Read reads bytes into the provided buffer and returns the number of bytes read
// It doesn't follow the standard golang Reader behavior, and returns n = -1 on EOF.
func (r *Go2AndroidReader) Read(b []byte) (n int, err error) {
	if r.isEOF {
		return -1, nil
	}
	n, err = r.reader.Read(b)
	if errors.Is(err, io.EOF) {
		if n == 0 {
			return -1, nil
		} else {
			r.isEOF = true
			return n, nil
		}
	}
	return
}

// Go2IOSReader is used to wrap a native golang Reader in the golang runtime,
// to be usable in the iOS app runtime (via gomobile) as a MobileReader.
type Go2IOSReader struct {
	reader crypto.Reader
}

// NewGo2IOSReader wraps a native golang Reader to be usable in the ios app runtime (via gomobile).
func NewGo2IOSReader(reader crypto.Reader) *Go2IOSReader {
	return &Go2IOSReader{reader}
}

// Read reads at most <max> bytes from the wrapped Reader and returns the read data as a MobileReadResult.
func (r *Go2IOSReader) Read(max int) (result *MobileReadResult, err error) {
	b := make([]byte, max)
	n, err := r.reader.Read(b)
	result = &MobileReadResult{}
	if err != nil {
		if errors.Is(err, io.EOF) {
			result.IsEOF = true
		} else {
			return nil, err
		}
	}
	result.N = n
	if n > 0 {
		result.Data = b[:n]
	}
	return result, nil
}

// VerifySignatureExplicit calls the reader's VerifySignature()
// and tries to cast the returned error to a SignatureVerificationError.
func VerifySignatureExplicit(
	reader *crypto.PlainMessageReader,
) (signatureVerificationError *crypto.SignatureVerificationError, err error) {
	if reader == nil {
		return nil, errors.New("gopenppg: the reader can't be nil")
	}
	err = reader.VerifySignature()
	if err != nil {
		castedErr := &crypto.SignatureVerificationError{}
		isType := errors.As(err, castedErr)
		if !isType {
			return
		}
		signatureVerificationError = castedErr
		err = nil
	}
	return
}
