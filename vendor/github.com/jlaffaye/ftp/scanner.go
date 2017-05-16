package ftp

// A scanner for fields delimited by one or more whitespace characters
type scanner struct {
	bytes    []byte
	position int
}

// newScanner creates a new scanner
func newScanner(str string) *scanner {
	return &scanner{
		bytes: []byte(str),
	}
}

// NextFields returns the next `count` fields
func (s *scanner) NextFields(count int) []string {
	fields := make([]string, 0, count)
	for i := 0; i < count; i++ {
		if field := s.Next(); field != "" {
			fields = append(fields, field)
		} else {
			break
		}
	}
	return fields
}

// Next returns the next field
func (s *scanner) Next() string {
	sLen := len(s.bytes)

	// skip trailing whitespace
	for s.position < sLen {
		if s.bytes[s.position] != ' ' {
			break
		}
		s.position++
	}

	start := s.position

	// skip non-whitespace
	for s.position < sLen {
		if s.bytes[s.position] == ' ' {
			s.position++
			return string(s.bytes[start : s.position-1])
		}
		s.position++
	}

	return string(s.bytes[start:s.position])
}

// Remaining returns the remaining string
func (s *scanner) Remaining() string {
	return string(s.bytes[s.position:len(s.bytes)])
}
