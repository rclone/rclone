package sparse

import (
	"io"

	"github.com/rclone/rclone/fs"
)

func WriteSparse(writer fs.WriterAtCloser, baseOff int64, reader io.Reader, minSparseBlockSize int64) (int64, error) {
	buf := make([]byte, 32*1024)
	currentFileOff := baseOff
	total := int64(0)
	logicalTotal := int64(0)

	var n int
	var err error
	for n, err = reader.Read(buf); (err == io.EOF && n > 0) || err == nil; n, err = reader.Read(buf) {
		logicalTotal += int64(n)
		rdBuf := buf[:n]
		bufDenseBlockStart := 0
		bufDenseBlockCurrent := 0
		bufSparseBlockStart := 0
		bufSparseBlockCurrent := 0
		prevSparse := false
		for i, val := range rdBuf[:n] {
			isLastInBuf := len(rdBuf)-1 == i
			if val == 0 {
				if !prevSparse {
					bufSparseBlockStart = i
				}
				prevSparse = true
				bufSparseBlockCurrent = i + 1
			} else {
				if prevSparse && (i-bufSparseBlockCurrent) > int(minSparseBlockSize) {
					bufDenseBlockStart = i
				}
				prevSparse = false
				bufDenseBlockCurrent = i + 1
			}

			contigZeroCount := int64(bufSparseBlockCurrent - bufSparseBlockStart)
			if (val != 0 && contigZeroCount >= minSparseBlockSize) || isLastInBuf {

				if (bufDenseBlockStart < bufSparseBlockStart || isLastInBuf) && bufDenseBlockCurrent-bufDenseBlockStart > 0 {
					nWritten := int(0)
					nWritten, writeErr := writer.WriteAt(buf[bufDenseBlockStart:bufDenseBlockCurrent], currentFileOff+int64(bufDenseBlockStart))
					total += int64(nWritten)

					if writeErr != nil {
						return total, err
					}
				}
			}
		}
		currentFileOff += int64(len(rdBuf))
		rdBuf = nil
	}

	if err != io.EOF && err != nil {
		return 0, err
	}
	return logicalTotal, nil
}
