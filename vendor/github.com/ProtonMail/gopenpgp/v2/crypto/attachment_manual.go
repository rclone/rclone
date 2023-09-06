package crypto

import (
	"io"
	"io/ioutil"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/pkg/errors"
)

// ManualAttachmentProcessor keeps track of the progress of encrypting an attachment
// (optimized for encrypting large files).
// With this processor, the caller has to first allocate
// a buffer large enough to hold the whole data packet.
type ManualAttachmentProcessor struct {
	keyPacket        []byte
	dataLength       int
	plaintextWriter  io.WriteCloser
	ciphertextWriter *io.PipeWriter
	err              error
	done             sync.WaitGroup
}

// GetKeyPacket returns the key packet for the attachment.
// This should be called only after Finish() has been called.
func (ap *ManualAttachmentProcessor) GetKeyPacket() []byte {
	return ap.keyPacket
}

// GetDataLength returns the number of bytes in the DataPacket.
// This should be called only after Finish() has been called.
func (ap *ManualAttachmentProcessor) GetDataLength() int {
	return ap.dataLength
}

// Process writes attachment data to be encrypted.
func (ap *ManualAttachmentProcessor) Process(plainData []byte) error {
	defer runtime.GC()
	_, err := ap.plaintextWriter.Write(plainData)
	return errors.Wrap(err, "gopenpgp: couldn't write attachment data")
}

// Finish tells the processor to finalize encryption.
func (ap *ManualAttachmentProcessor) Finish() error {
	defer runtime.GC()
	if ap.err != nil {
		return ap.err
	}
	if err := ap.plaintextWriter.Close(); err != nil {
		return errors.Wrap(err, "gopengpp: unable to close the plaintext writer")
	}
	if err := ap.ciphertextWriter.Close(); err != nil {
		return errors.Wrap(err, "gopengpp: unable to close the dataPacket writer")
	}
	ap.done.Wait()
	if ap.err != nil {
		return ap.err
	}
	return nil
}

// NewManualAttachmentProcessor creates an AttachmentProcessor which can be used
// to encrypt a file. It takes an estimatedSize and filename as hints about the
// file and a buffer to hold the DataPacket.
// It is optimized for low-memory environments and collects garbage every megabyte.
// The buffer for the data packet must be manually allocated by the caller.
// Make sure that the dataBuffer is large enough to hold the whole data packet
// otherwise Finish() will return an error.
func (keyRing *KeyRing) NewManualAttachmentProcessor(
	estimatedSize int, filename string, dataBuffer []byte,
) (*ManualAttachmentProcessor, error) {
	if len(dataBuffer) == 0 {
		return nil, errors.New("gopenpgp: can't give a nil or empty buffer to process the attachment")
	}

	// forces the gc to be called often
	debug.SetGCPercent(10)

	attachmentProc := &ManualAttachmentProcessor{}

	// hints for the encrypted file
	isBinary := true
	modTime := GetUnixTime()
	hints := &openpgp.FileHints{
		FileName: filename,
		IsBinary: isBinary,
		ModTime:  time.Unix(modTime, 0),
	}

	// encryption config
	config := &packet.Config{
		DefaultCipher: packet.CipherAES256,
		Time:          getTimeGenerator(),
	}

	// goroutine that reads the key packet
	// to be later returned to the caller via GetKeyPacket()
	keyReader, keyWriter := io.Pipe()
	attachmentProc.done.Add(1)
	go func() {
		defer attachmentProc.done.Done()
		keyPacket, err := ioutil.ReadAll(keyReader)
		if err != nil {
			attachmentProc.err = err
		} else {
			attachmentProc.keyPacket = clone(keyPacket)
		}
	}()

	// goroutine that reads the data packet into the provided buffer
	dataReader, dataWriter := io.Pipe()
	attachmentProc.done.Add(1)
	go func() {
		defer attachmentProc.done.Done()
		totalRead, err := readAll(dataBuffer, dataReader)
		if err != nil {
			attachmentProc.err = err
		} else {
			attachmentProc.dataLength = totalRead
		}
	}()

	// We generate the encrypting writer
	var ew io.WriteCloser
	var encryptErr error
	ew, encryptErr = openpgp.EncryptSplit(keyWriter, dataWriter, keyRing.entities, nil, hints, config)
	if encryptErr != nil {
		return nil, errors.Wrap(encryptErr, "gopengpp: unable to encrypt attachment")
	}

	attachmentProc.plaintextWriter = ew
	attachmentProc.ciphertextWriter = dataWriter

	// The key packet should have been already written, so we can close
	if err := keyWriter.Close(); err != nil {
		return nil, errors.Wrap(err, "gopenpgp: couldn't close the keyPacket writer")
	}

	// Check if the goroutines encountered errors
	if attachmentProc.err != nil {
		return nil, attachmentProc.err
	}
	return attachmentProc, nil
}

// readAll works a bit like ioutil.ReadAll
// but we can choose the buffer to write to
// and we don't grow the slice in case of overflow.
func readAll(buffer []byte, reader io.Reader) (int, error) {
	bufferLen := len(buffer)
	totalRead := 0
	offset := 0
	overflow := false
	reset := false
	for {
		// We read into the buffer
		n, err := reader.Read(buffer[offset:])
		totalRead += n
		offset += n
		if !overflow && reset && n != 0 {
			// In case we've started overwriting the beginning of the buffer
			// We will return an error at Finish()
			overflow = true
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return 0, errors.Wrap(err, "gopenpgp: couldn't read data from the encrypted reader")
		}
		if offset == bufferLen {
			// Here we've reached the end of the buffer
			// But we need to keep reading to not block the Process()
			// So we reset the buffer
			reset = true
			offset = 0
		}
	}
	if overflow {
		return 0, errors.New("gopenpgp: read more bytes that was allocated in the buffer")
	}
	return totalRead, nil
}
