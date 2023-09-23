package azurefiles

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"time"
)

func RandomString(charCount int) string {
	bs := make([]byte, charCount)
	for i := 0; i < charCount; i++ {
		bs[i] = byte(97 + rand.Intn(26))
	}
	return string(bs)
}

func randomTime() time.Time {
	return time.Unix(int64(rand.Int31()), 0)
}

func RandomPuttableObject(remote string) (io.Reader, *Object) {
	var fileSize int64 = int64(10 + rand.Intn(50))
	fileContent := RandomString(int(fileSize))
	r := bytes.NewReader([]byte(fileContent))
	metaData := make(map[string]*string)
	modTime := randomTime().Truncate(time.Second)
	nowStr := fmt.Sprintf("%d", modTime.Unix())
	metaData[modTimeKey] = &nowStr
	return r, &Object{common{
		remote:     remote,
		metaData:   metaData,
		properties: properties{contentLength: &fileSize},
	}}
}
