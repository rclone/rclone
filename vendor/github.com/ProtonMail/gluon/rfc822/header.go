package rfc822

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/textproto"
	"strings"
)

type headerEntry struct {
	parsedHeaderEntry

	mapKey string
	merged string
	prev   *headerEntry
	next   *headerEntry
}

func (he *headerEntry) getMerged(data []byte) string {
	if len(he.merged) == 0 {
		he.merged = mergeMultiline(he.getValue(data))
	}

	return he.merged
}

type Header struct {
	keys       map[string][]*headerEntry
	firstEntry *headerEntry
	lastEntry  *headerEntry
	data       []byte
}

// NewEmptyHeader returns an empty header that can be filled with values.
func NewEmptyHeader() *Header {
	h, err := NewHeader([]byte{'\r', '\n'})
	// The above code should never fail, but just in case.
	if err != nil {
		panic(err)
	}

	return h
}

func NewHeader(data []byte) (*Header, error) {
	h := &Header{
		keys: make(map[string][]*headerEntry),
		data: data,
	}

	parser := newHeaderParser(data)

	for {
		entry, err := parser.next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			} else {
				return nil, err
			}
		}

		hentry := &headerEntry{
			parsedHeaderEntry: entry,
			merged:            "",
			next:              nil,
		}

		if entry.hasKey() {
			hashKey := strings.ToLower(string(entry.getKey(data)))
			hentry.mapKey = hashKey

			if v, ok := h.keys[hashKey]; !ok {
				h.keys[hashKey] = []*headerEntry{hentry}
			} else {
				h.keys[hashKey] = append(v, hentry)
			}
		}

		if h.firstEntry == nil {
			h.firstEntry = hentry
			h.lastEntry = hentry
		} else {
			h.lastEntry.next = hentry
			hentry.prev = h.lastEntry
			h.lastEntry = hentry
		}
	}

	return h, nil
}

func (h *Header) Raw() []byte {
	return h.data
}

func (h *Header) Has(key string) bool {
	_, ok := h.keys[strings.ToLower(key)]

	return ok
}

func (h *Header) GetChecked(key string) (string, bool) {
	v, ok := h.keys[strings.ToLower(key)]
	if !ok {
		return "", false
	}

	return v[0].getMerged(h.data), true
}

func (h *Header) Get(key string) string {
	v, ok := h.keys[strings.ToLower(key)]
	if !ok {
		return ""
	}

	return v[0].getMerged(h.data)
}

func (h *Header) GetLine(key string) []byte {
	v, ok := h.keys[strings.ToLower(key)]
	if !ok {
		return nil
	}

	return v[0].getAll(h.data)
}

func (h *Header) getLines() [][]byte {
	var res [][]byte
	for e := h.firstEntry; e != nil; e = e.next {
		res = append(res, h.data[e.keyStart:e.valueEnd])
	}

	return res
}

func (h *Header) GetRaw(key string) []byte {
	v, ok := h.keys[strings.ToLower(key)]
	if !ok {
		return nil
	}

	return v[0].getValue(h.data)
}

func (h *Header) Set(key, val string) {
	// We can only add entries to the front of the header.
	key = textproto.CanonicalMIMEHeaderKey(key)
	mapKey := strings.ToLower(key)

	keyBytes := []byte(key)

	entryBytes := joinLine([]byte(key), []byte(val))
	newHeaderEntry := &headerEntry{
		parsedHeaderEntry: parsedHeaderEntry{
			keyStart:   0,
			keyEnd:     len(keyBytes),
			valueStart: len(keyBytes) + 2,
			valueEnd:   len(entryBytes),
		},
		mapKey: mapKey,
	}

	if v, ok := h.keys[mapKey]; !ok {
		h.keys[mapKey] = []*headerEntry{newHeaderEntry}
	} else {
		h.keys[mapKey] = append([]*headerEntry{newHeaderEntry}, v...)
	}

	if h.firstEntry == nil {
		h.data = entryBytes
		h.firstEntry = newHeaderEntry
	} else {
		insertOffset := h.firstEntry.keyStart
		newHeaderEntry.next = h.firstEntry
		h.firstEntry.prev = newHeaderEntry
		h.firstEntry = newHeaderEntry

		var buffer bytes.Buffer

		if insertOffset != 0 {
			if _, err := buffer.Write(h.data[0:insertOffset]); err != nil {
				panic("failed to write to byte buffer")
			}
		}

		if _, err := buffer.Write(entryBytes); err != nil {
			panic("failed to write to byte buffer")
		}

		if _, err := buffer.Write(h.data[insertOffset:]); err != nil {
			panic("failed to write to byte buffer")
		}

		h.data = buffer.Bytes()
		h.applyOffset(newHeaderEntry.next, len(entryBytes))
	}
}

func (h *Header) Del(key string) {
	mapKey := strings.ToLower(key)

	v, ok := h.keys[mapKey]
	if !ok {
		return
	}

	he := v[0]

	if len(v) == 1 {
		delete(h.keys, mapKey)
	} else {
		h.keys[mapKey] = v[1:]
	}

	if he.prev != nil {
		he.prev.next = he.next
	}

	if he.next != nil {
		he.next.prev = he.prev
	}

	dataLen := he.valueEnd - he.keyStart

	h.data = append(h.data[0:he.keyStart], h.data[he.valueEnd:]...)

	h.applyOffset(he.next, -dataLen)
}

func (h *Header) Fields(fields []string) []byte {
	wantFields := make(map[string]struct{})

	for _, field := range fields {
		wantFields[strings.ToLower(field)] = struct{}{}
	}

	var res []byte

	for e := h.firstEntry; e != nil; e = e.next {
		if len(bytes.TrimSpace(e.getAll(h.data))) == 0 {
			res = append(res, e.getAll(h.data)...)
			continue
		}

		if !e.hasKey() {
			continue
		}

		_, ok := wantFields[e.mapKey]
		if !ok {
			continue
		}

		res = append(res, e.getAll(h.data)...)
	}

	return res
}

func (h *Header) FieldsNot(fields []string) []byte {
	wantFieldsNot := make(map[string]struct{})

	for _, field := range fields {
		wantFieldsNot[strings.ToLower(field)] = struct{}{}
	}

	var res []byte

	for e := h.firstEntry; e != nil; e = e.next {
		if len(bytes.TrimSpace(e.getAll(h.data))) == 0 {
			res = append(res, e.getAll(h.data)...)
			continue
		}

		if !e.hasKey() {
			continue
		}

		_, ok := wantFieldsNot[e.mapKey]
		if ok {
			continue
		}

		res = append(res, e.getAll(h.data)...)
	}

	return res
}

func (h *Header) Entries(fn func(key, val string)) {
	for e := h.firstEntry; e != nil; e = e.next {
		if !e.hasKey() {
			continue
		}

		fn(string(e.getKey(h.data)), e.getMerged(h.data))
	}
}

func (h *Header) applyOffset(start *headerEntry, offset int) {
	for e := start; e != nil; e = e.next {
		e.applyOffset(offset)
	}
}

// SetHeaderValue is a helper method that sets a header value in a message literal.
// It does not check whether the existing value already exists.
func SetHeaderValue(literal []byte, key, val string) ([]byte, error) {
	reader, size, err := SetHeaderValueNoMemCopy(literal, key, val)
	if err != nil {
		return nil, err
	}

	var b bytes.Buffer

	b.Grow(size)

	if _, err := b.ReadFrom(reader); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

// SetHeaderValueNoMemCopy is the same as SetHeaderValue, except it does not allocate memory to modify the input literal.
// Instead, it returns an io.MultiReader that combines the sub-slices in the correct order. This enables us to only
// allocate memory for the new header field while re-using the old literal.
func SetHeaderValueNoMemCopy(literal []byte, key, val string) (io.Reader, int, error) {
	rawHeader, body := Split(literal)

	parser := newHeaderParser(rawHeader)

	var (
		foundFirstEntry   bool
		parsedHeaderEntry parsedHeaderEntry
	)

	// find first header entry.
	for {
		entry, err := parser.next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			} else {
				return nil, 0, err
			}
		}

		if entry.hasKey() {
			foundFirstEntry = true
			parsedHeaderEntry = entry

			break
		}
	}

	key = textproto.CanonicalMIMEHeaderKey(key)
	data := joinLine([]byte(key), []byte(val))

	if !foundFirstEntry {
		return io.MultiReader(bytes.NewReader(rawHeader), bytes.NewReader(data), bytes.NewReader(body)), len(rawHeader) + len(data) + len(body), nil
	}

	part1 := literal[0:parsedHeaderEntry.keyStart]
	part2 := literal[parsedHeaderEntry.keyStart:]

	return io.MultiReader(
		bytes.NewReader(part1),
		bytes.NewReader(data),
		bytes.NewReader(part2),
	), len(part1) + len(part2) + len(data), nil
}

// GetHeaderValue is a helper method that queries a header value in a message literal.
func GetHeaderValue(literal []byte, key string) (string, error) {
	rawHeader, _ := Split(literal)

	parser := newHeaderParser(rawHeader)

	for {
		entry, err := parser.next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			} else {
				return "", err
			}
		}

		if !entry.hasKey() {
			continue
		}

		if !strings.EqualFold(key, string(entry.getKey(rawHeader))) {
			continue
		}

		return mergeMultiline(entry.getValue(rawHeader)), nil
	}

	return "", nil
}

// EraseHeaderValue removes the header from a literal.
func EraseHeaderValue(literal []byte, key string) ([]byte, error) {
	rawHeader, _ := Split(literal)

	parser := newHeaderParser(rawHeader)

	var (
		foundEntry        bool
		parsedHeaderEntry parsedHeaderEntry
	)

	for {
		entry, err := parser.next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			} else {
				return nil, err
			}
		}

		if !entry.hasKey() {
			continue
		}

		if !strings.EqualFold(key, string(entry.getKey(rawHeader))) {
			continue
		}

		foundEntry = true
		parsedHeaderEntry = entry

		break
	}

	result := make([]byte, 0, len(literal))
	if !foundEntry {
		result = append(result, literal...)
	} else {
		result = append(result, literal[0:parsedHeaderEntry.keyStart]...)
		result = append(result, literal[parsedHeaderEntry.valueEnd:]...)
	}

	return result, nil
}

var (
	ErrNonASCIIHeaderKey = fmt.Errorf("header key contains invalid characters")
	ErrKeyNotFound       = fmt.Errorf("invalid header key")
	ErrParseHeader       = fmt.Errorf("failed to parse header")
)

func mergeMultiline(line []byte) string {
	remaining := line

	var builder strings.Builder

	for len(remaining) != 0 {
		index := bytes.Index(remaining, []byte{'\n'})
		if index < 0 {
			builder.Write(bytes.TrimSpace(remaining))
			break
		}

		var section []byte

		if index >= 1 && remaining[index-1] == '\r' {
			section = remaining[0 : index-1]
		} else {
			section = remaining[0:index]
		}

		remaining = remaining[index+1:]

		if len(section) != 0 {
			builder.Write(bytes.TrimSpace(section))

			if len(remaining) != 0 {
				builder.WriteRune(' ')
			}
		}
	}

	return builder.String()
}

func splitLine(line []byte) [][]byte {
	result := bytes.SplitN(line, []byte(`:`), 2)

	if len(result) > 1 && len(result[1]) > 0 && result[1][0] == ' ' {
		result[1] = result[1][1:]
	}

	return result
}

// TODO: Don't assume line ending is \r\n. Bad.
func joinLine(key, val []byte) []byte {
	return []byte(string(key) + ": " + string(val) + "\r\n")
}
