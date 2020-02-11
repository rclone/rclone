package matchers

import (
	"bytes"
	"encoding/binary"
)

// Mp3 matches an mp3 file.
func Mp3(in []byte) bool {
	if len(in) < 3 {
		return false
	}

	if bytes.HasPrefix(in, []byte("ID3")) {
		// MP3s with an ID3v2 tag will start with "ID3"
		// ID3v1 tags, however appear at the end of the file.
		return true
	}

	// Match MP3 files without tags
	switch binary.BigEndian.Uint16(in[:2]) & 0xFFFE {
	case 0xFFFA:
		// MPEG ADTS, layer III, v1
		return true
	case 0xFFF2:
		// MPEG ADTS, layer III, v2
		return true
	case 0xFFE2:
		// MPEG ADTS, layer III, v2.5
		return true
	}

	return false
}

// Flac matches a Free Lossless Audio Codec file.
func Flac(in []byte) bool {
	return bytes.HasPrefix(in, []byte("\x66\x4C\x61\x43\x00\x00\x00\x22"))
}

// Midi matches a Musical Instrument Digital Interface file.
func Midi(in []byte) bool {
	return bytes.HasPrefix(in, []byte("\x4D\x54\x68\x64"))
}

// Ape matches a Monkey's Audio file.
func Ape(in []byte) bool {
	return bytes.HasPrefix(in, []byte("\x4D\x41\x43\x20\x96\x0F\x00\x00\x34\x00\x00\x00\x18\x00\x00\x00\x90\xE3"))
}

// MusePack matches a Musepack file.
func MusePack(in []byte) bool {
	return bytes.HasPrefix(in, []byte("MPCK"))
}

// Wav matches a Waveform Audio File Format file.
func Wav(in []byte) bool {
	return len(in) > 12 &&
		bytes.Equal(in[:4], []byte("RIFF")) &&
		bytes.Equal(in[8:12], []byte("\x57\x41\x56\x45"))
}

// Aiff matches Audio Interchange File Format file.
func Aiff(in []byte) bool {
	return len(in) > 12 &&
		bytes.Equal(in[:4], []byte("\x46\x4F\x52\x4D")) &&
		bytes.Equal(in[8:12], []byte("\x41\x49\x46\x46"))
}

// Au matches a Sun Microsystems au file.
func Au(in []byte) bool {
	return bytes.HasPrefix(in, []byte("\x2E\x73\x6E\x64"))
}

// Amr matches an Adaptive Multi-Rate file.
func Amr(in []byte) bool {
	return bytes.HasPrefix(in, []byte("\x23\x21\x41\x4D\x52"))
}

// Aac matches an Advanced Audio Coding file.
func Aac(in []byte) bool {
	return bytes.HasPrefix(in, []byte{0xFF, 0xF1}) || bytes.HasPrefix(in, []byte{0xFF, 0xF9})
}

// Voc matches a Creative Voice file.
func Voc(in []byte) bool {
	return bytes.HasPrefix(in, []byte("Creative Voice File"))
}

// Qcp matches a Qualcomm Pure Voice file.
func Qcp(in []byte) bool {
	return len(in) > 12 &&
		bytes.Equal(in[:4], []byte("RIFF")) &&
		bytes.Equal(in[8:12], []byte("QLCM"))
}
