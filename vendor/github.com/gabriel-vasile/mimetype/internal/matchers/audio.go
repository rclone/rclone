package matchers

import (
	"bytes"
)

// Mp3 matches an mp3 file.
func Mp3(in []byte) bool {
	return bytes.HasPrefix(in, []byte("\x49\x44\x33"))
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
	return len(in) > 4 && bytes.Equal(in[:4], []byte("MPCK"))
}

// Wav matches a Waveform Audio File Format file.
func Wav(in []byte) bool {
	return len(in) > 12 &&
		bytes.Equal(in[:4], []byte("\x52\x49\x46\x46")) &&
		bytes.Equal(in[8:12], []byte("\x57\x41\x56\x45"))
}

// Aiff matches Audio Interchange File Format file.
func Aiff(in []byte) bool {
	return len(in) > 12 &&
		bytes.Equal(in[:4], []byte("\x46\x4F\x52\x4D")) &&
		bytes.Equal(in[8:12], []byte("\x41\x49\x46\x46"))
}

// Ogg matches an Ogg file.
func Ogg(in []byte) bool {
	return len(in) > 5 && bytes.Equal(in[:5], []byte("\x4F\x67\x67\x53\x00"))
}

// Au matches a Sun Microsystems au file.
func Au(in []byte) bool {
	return len(in) > 4 && bytes.Equal(in[:4], []byte("\x2E\x73\x6E\x64"))
}

// Amr matches an Adaptive Multi-Rate file.
func Amr(in []byte) bool {
	return len(in) > 5 && bytes.Equal(in[:5], []byte("\x23\x21\x41\x4D\x52"))
}
