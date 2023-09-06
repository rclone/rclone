package rfc822

import (
	"bytes"
)

type Part struct {
	Data   []byte
	Offset int
}

type ByteScanner struct {
	data          []byte
	startBoundary []byte
	progress      int
}

func NewByteScanner(data []byte, boundary []byte) (*ByteScanner, error) {
	scanner := &ByteScanner{
		data:          data,
		startBoundary: append([]byte{'-', '-'}, boundary...),
	}

	scanner.readToBoundary()

	return scanner, nil
}

func (s *ByteScanner) ScanAll() []Part {
	var parts []Part

	for {
		offset := s.progress

		data, more := s.readToBoundary()

		if data != nil {
			parts = append(parts, Part{Data: data, Offset: offset})
		}

		if !more {
			return parts
		}
	}
}

func indexOfNewLineAfterBoundary(data []byte) int {
	dataLen := len(data)

	if dataLen == 0 {
		return -1
	}

	if dataLen == 1 && data[0] == '\n' {
		return 0
	}

	// consume extra '\r's
	index := 0
	for ; index < dataLen && data[index] == '\r'; index++ {
	}

	if index < dataLen && data[index] == '\n' {
		return index
	}

	return -1
}

func (s *ByteScanner) getPreviousLineBreakIndex(offset int) int {
	if s.progress == offset {
		return 0
	} else if s.data[offset-1] == '\n' {
		if offset-s.progress >= 2 && s.data[offset-2] == '\r' {
			return 2
		}
		return 1
	}

	return -1
}

// readToBoundary returns the slice matching to the boundary and whether this is the start or the end of said boundary.
func (s *ByteScanner) readToBoundary() ([]byte, bool) {
	boundarySuffix := []byte{'-', '-'}
	boundarySuffixLen := len(boundarySuffix)
	boundaryLen := len(s.startBoundary)
	dataLen := len(s.data)
	searchStart := s.progress

	for s.progress < dataLen {
		remaining := s.data[s.progress:]

		index := bytes.Index(remaining, s.startBoundary)
		if index < 0 {
			s.progress = len(s.data)
			return remaining, false
		}

		// Matched the pattern, now we need to check if the previous line break is available or not. It can also not be
		// available if the pattern just happens to match exactly at the offset search.
		prevNewLineOffset := s.getPreviousLineBreakIndex(s.progress + index)
		if prevNewLineOffset != -1 {
			// Since we matched the pattern we can check whether this is a starting or terminating pattern.
			if s.progress+index+boundaryLen+boundarySuffixLen <= dataLen &&
				bytes.Equal(remaining[index+boundaryLen:index+boundaryLen+boundarySuffixLen], boundarySuffix) {
				lineEndIndex := index + boundaryLen + boundarySuffixLen
				afterBoundary := remaining[lineEndIndex:]

				var newLineStartIndex int

				// It can happen that this boundary is at the end of the file/message with no new line.
				if len(afterBoundary) != 0 {
					newLineStartIndex = indexOfNewLineAfterBoundary(afterBoundary)
					// If there is no new line this can't be a boundary pattern. RFC 1341 states that tey are
					// immediately followed by either \r\n or \n.
					if newLineStartIndex < 0 {
						s.progress += index + boundaryLen + boundarySuffixLen
						continue
					}
				} else {
					newLineStartIndex = 0
				}

				result := s.data[searchStart : s.progress+index-prevNewLineOffset]
				s.progress += index + boundaryLen + boundarySuffixLen + newLineStartIndex + 1

				return result, false
			} else {

				// Check for new line.
				lineEndIndex := index + boundaryLen
				afterBoundary := remaining[lineEndIndex:]
				newLineStart := indexOfNewLineAfterBoundary(afterBoundary)

				// If there is no new line this can't be a boundary pattern. RFC 1341 states that tey are
				// immediately followed by either \r\n or \n.
				if newLineStart < 0 {
					s.progress += index + boundaryLen
					continue
				}

				result := s.data[searchStart : s.progress+index-prevNewLineOffset]
				s.progress += index + boundaryLen + newLineStart + 1
				return result, true
			}
		}

		s.progress += index + boundaryLen
	}

	return nil, false
}
