package azurefiles

import (
	"bytes"
	"crypto/md5"
	"io"
	"log"
	"math/rand"
	"time"
)

func randomString(charCount int) string {
	bs := make([]byte, charCount)
	for i := 0; i < charCount; i++ {
		bs[i] = byte(97 + rand.Intn(26))
	}
	return string(bs)
}

func randomTime() time.Time {
	return time.Unix(int64(rand.Int31()), 0)
}

func randomPuttableObjectWithSize(f *Fs, remote string, fileSize int64) (io.Reader, *Object) {
	fileContent := randomString(int(fileSize))
	hasher := md5.New()
	if _, err := hasher.Write([]byte(fileContent)); err != nil {
		log.Fatal("randomPuttableObject: writing to hasher : %w", err)
	}
	r := bytes.NewReader([]byte(fileContent))
	modTime := randomTime().Truncate(time.Second)
	return r, &Object{common{
		f:      f,
		remote: remote,
		properties: properties{
			contentLength: fileSize,
			lastWriteTime: modTime,
			md5Hash:       hasher.Sum(nil),
		},
	}}
}

func randomPuttableObject(f *Fs, remote string) (io.Reader, *Object) {
	fileSize := 10 + rand.Int63n(100)
	return randomPuttableObjectWithSize(f, remote, fileSize)
}
