package textproto

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/textproto"
	"sort"
	"strings"
)

type headerField struct {
	b []byte // Raw header field, including whitespace

	k string
	v string
}

func newHeaderField(k, v string, b []byte) *headerField {
	return &headerField{k: textproto.CanonicalMIMEHeaderKey(k), v: v, b: b}
}

func (f *headerField) raw() ([]byte, error) {
	if f.b != nil {
		return f.b, nil
	} else {
		for pos, ch := range f.k {
			// check if character is a printable US-ASCII except ':'
			if !(ch >= '!' && ch < ':' || ch > ':' && ch <= '~') {
				return nil, fmt.Errorf("field name contains incorrect symbols (\\x%x at %v)", ch, pos)
			}
		}

		if pos := strings.IndexAny(f.v, "\r\n"); pos != -1 {
			return nil, fmt.Errorf("field value contains \\r\\n (at %v)", pos)
		}

		return []byte(formatHeaderField(f.k, f.v)), nil
	}
}

// A Header represents the key-value pairs in a message header.
//
// The header representation is idempotent: if the header can be read and
// written, the result will be exactly the same as the original (including
// whitespace and header field ordering). This is required for e.g. DKIM.
//
// Mutating the header is restricted: the only two allowed operations are
// inserting a new header field at the top and deleting a header field. This is
// again necessary for DKIM.
type Header struct {
	// Fields are in reverse order so that inserting a new field at the top is
	// cheap.
	l []*headerField
	m map[string][]*headerField
}

func makeHeaderMap(fs []*headerField) map[string][]*headerField {
	if len(fs) == 0 {
		return nil
	}

	m := make(map[string][]*headerField, len(fs))
	for i, f := range fs {
		m[f.k] = append(m[f.k], fs[i])
	}
	return m
}

func newHeader(fs []*headerField) Header {
	// Reverse order
	for i := len(fs)/2 - 1; i >= 0; i-- {
		opp := len(fs) - 1 - i
		fs[i], fs[opp] = fs[opp], fs[i]
	}

	return Header{l: fs, m: makeHeaderMap(fs)}
}

// HeaderFromMap creates a header from a map of header fields.
//
// This function is provided for interoperability with the standard library.
// If possible, ReadHeader should be used instead to avoid loosing information.
// The map representation looses the ordering of the fields, the capitalization
// of the header keys, and the whitespace of the original header.
func HeaderFromMap(m map[string][]string) Header {
	fs := make([]*headerField, 0, len(m))
	for k, vs := range m {
		for _, v := range vs {
			fs = append(fs, newHeaderField(k, v, nil))
		}
	}

	sort.SliceStable(fs, func(i, j int) bool {
		return fs[i].k < fs[j].k
	})

	return newHeader(fs)
}

// AddRaw adds the raw key, value pair to the header.
//
// The supplied byte slice should be a complete field in the "Key: Value" form
// including trailing CRLF. If there is no comma in the input - AddRaw panics.
// No changes are made to kv contents and it will be copied into WriteHeader
// output as is.
//
// kv is directly added to the underlying structure and therefore should not be
// modified after the AddRaw call.
func (h *Header) AddRaw(kv []byte) {
	colon := bytes.IndexByte(kv, ':')
	if colon == -1 {
		panic("textproto: Header.AddRaw: missing colon")
	}
	k := textproto.CanonicalMIMEHeaderKey(string(trim(kv[:colon])))
	v := trimAroundNewlines(kv[colon+1:])

	if h.m == nil {
		h.m = make(map[string][]*headerField)
	}

	f := newHeaderField(k, v, kv)
	h.l = append(h.l, f)
	h.m[k] = append(h.m[k], f)
}

// Add adds the key, value pair to the header. It prepends to any existing
// fields associated with key.
//
// Key and value should obey character requirements of RFC 6532.
// If you need to format or fold lines manually, use AddRaw.
func (h *Header) Add(k, v string) {
	k = textproto.CanonicalMIMEHeaderKey(k)

	if h.m == nil {
		h.m = make(map[string][]*headerField)
	}

	f := newHeaderField(k, v, nil)
	h.l = append(h.l, f)
	h.m[k] = append(h.m[k], f)
}

// Get gets the first value associated with the given key. If there are no
// values associated with the key, Get returns "".
func (h *Header) Get(k string) string {
	fields := h.m[textproto.CanonicalMIMEHeaderKey(k)]
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1].v
}

// Raw gets the first raw header field associated with the given key.
//
// The returned bytes contain a complete field in the "Key: value" form,
// including trailing CRLF.
//
// The returned slice should not be modified and becomes invalid when the
// header is updated.
//
// An error is returned if the header field contains incorrect characters (see
// RFC 6532).
func (h *Header) Raw(k string) ([]byte, error) {
	fields := h.m[textproto.CanonicalMIMEHeaderKey(k)]
	if len(fields) == 0 {
		return nil, nil
	}
	return fields[len(fields)-1].raw()
}

// Values returns all values associated with the given key.
//
// The returned slice should not be modified and becomes invalid when the
// header is updated.
func (h *Header) Values(k string) []string {
	fields := h.m[textproto.CanonicalMIMEHeaderKey(k)]
	if len(fields) == 0 {
		return nil
	}
	l := make([]string, len(fields))
	for i, field := range fields {
		l[len(fields)-i-1] = field.v
	}
	return l
}

// Set sets the header fields associated with key to the single field value.
// It replaces any existing values associated with key.
func (h *Header) Set(k, v string) {
	h.Del(k)
	h.Add(k, v)
}

// Del deletes the values associated with key.
func (h *Header) Del(k string) {
	k = textproto.CanonicalMIMEHeaderKey(k)

	delete(h.m, k)

	// Delete existing keys
	for i := len(h.l) - 1; i >= 0; i-- {
		if h.l[i].k == k {
			h.l = append(h.l[:i], h.l[i+1:]...)
		}
	}
}

// Has checks whether the header has a field with the specified key.
func (h *Header) Has(k string) bool {
	_, ok := h.m[textproto.CanonicalMIMEHeaderKey(k)]
	return ok
}

// Copy creates an independent copy of the header.
func (h *Header) Copy() Header {
	l := make([]*headerField, len(h.l))
	copy(l, h.l)
	m := makeHeaderMap(l)
	return Header{l: l, m: m}
}

// Len returns the number of fields in the header.
func (h *Header) Len() int {
	return len(h.l)
}

// Map returns all header fields as a map.
//
// This function is provided for interoperability with the standard library.
// If possible, Fields should be used instead to avoid loosing information.
// The map representation looses the ordering of the fields, the capitalization
// of the header keys, and the whitespace of the original header.
func (h *Header) Map() map[string][]string {
	m := make(map[string][]string, h.Len())
	fields := h.Fields()
	for fields.Next() {
		m[fields.Key()] = append(m[fields.Key()], fields.Value())
	}
	return m
}

// HeaderFields iterates over header fields. Its cursor starts before the first
// field of the header. Use Next to advance from field to field.
type HeaderFields interface {
	// Next advances to the next header field. It returns true on success, or
	// false if there is no next field.
	Next() (more bool)
	// Key returns the key of the current field.
	Key() string
	// Value returns the value of the current field.
	Value() string
	// Raw returns the raw current header field. See Header.Raw.
	Raw() ([]byte, error)
	// Del deletes the current field.
	Del()
	// Len returns the amount of header fields in the subset of header iterated
	// by this HeaderFields instance.
	//
	// For Fields(), it will return the amount of fields in the whole header section.
	// For FieldsByKey(), it will return the amount of fields with certain key.
	Len() int
}

type headerFields struct {
	h   *Header
	cur int
}

func (fs *headerFields) Next() bool {
	fs.cur++
	return fs.cur < len(fs.h.l)
}

func (fs *headerFields) index() int {
	if fs.cur < 0 {
		panic("message: HeaderFields method called before Next")
	}
	if fs.cur >= len(fs.h.l) {
		panic("message: HeaderFields method called after Next returned false")
	}
	return len(fs.h.l) - fs.cur - 1
}

func (fs *headerFields) field() *headerField {
	return fs.h.l[fs.index()]
}

func (fs *headerFields) Key() string {
	return fs.field().k
}

func (fs *headerFields) Value() string {
	return fs.field().v
}

func (fs *headerFields) Raw() ([]byte, error) {
	return fs.field().raw()
}

func (fs *headerFields) Del() {
	f := fs.field()

	ok := false
	for i, ff := range fs.h.m[f.k] {
		if ff == f {
			ok = true
			fs.h.m[f.k] = append(fs.h.m[f.k][:i], fs.h.m[f.k][i+1:]...)
			if len(fs.h.m[f.k]) == 0 {
				delete(fs.h.m, f.k)
			}
			break
		}
	}
	if !ok {
		panic("message: field not found in Header.m")
	}

	fs.h.l = append(fs.h.l[:fs.index()], fs.h.l[fs.index()+1:]...)
	fs.cur--
}

func (fs *headerFields) Len() int {
	return len(fs.h.l)
}

// Fields iterates over all the header fields.
//
// The header may not be mutated while iterating, except using HeaderFields.Del.
func (h *Header) Fields() HeaderFields {
	return &headerFields{h, -1}
}

type headerFieldsByKey struct {
	h   *Header
	k   string
	cur int
}

func (fs *headerFieldsByKey) Next() bool {
	fs.cur++
	return fs.cur < len(fs.h.m[fs.k])
}

func (fs *headerFieldsByKey) index() int {
	if fs.cur < 0 {
		panic("message: headerfields.key or value called before next")
	}
	if fs.cur >= len(fs.h.m[fs.k]) {
		panic("message: headerfields.key or value called after next returned false")
	}
	return len(fs.h.m[fs.k]) - fs.cur - 1
}

func (fs *headerFieldsByKey) field() *headerField {
	return fs.h.m[fs.k][fs.index()]
}

func (fs *headerFieldsByKey) Key() string {
	return fs.field().k
}

func (fs *headerFieldsByKey) Value() string {
	return fs.field().v
}

func (fs *headerFieldsByKey) Raw() ([]byte, error) {
	return fs.field().raw()
}

func (fs *headerFieldsByKey) Del() {
	f := fs.field()

	ok := false
	for i := range fs.h.l {
		if f == fs.h.l[i] {
			ok = true
			fs.h.l = append(fs.h.l[:i], fs.h.l[i+1:]...)
			break
		}
	}
	if !ok {
		panic("message: field not found in Header.l")
	}

	fs.h.m[fs.k] = append(fs.h.m[fs.k][:fs.index()], fs.h.m[fs.k][fs.index()+1:]...)
	if len(fs.h.m[fs.k]) == 0 {
		delete(fs.h.m, fs.k)
	}
	fs.cur--
}

func (fs *headerFieldsByKey) Len() int {
	return len(fs.h.m[fs.k])
}

// FieldsByKey iterates over all fields having the specified key.
//
// The header may not be mutated while iterating, except using HeaderFields.Del.
func (h *Header) FieldsByKey(k string) HeaderFields {
	return &headerFieldsByKey{h, textproto.CanonicalMIMEHeaderKey(k), -1}
}

func readLineSlice(r *bufio.Reader, line []byte) ([]byte, error) {
	for {
		l, more, err := r.ReadLine()
		line = append(line, l...)
		if err != nil {
			return line, err
		}

		if !more {
			break
		}
	}

	return line, nil
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t'
}

func validHeaderKeyByte(b byte) bool {
	c := int(b)
	return c >= 33 && c <= 126 && c != ':'
}

// trim returns s with leading and trailing spaces and tabs removed.
// It does not assume Unicode or UTF-8.
func trim(s []byte) []byte {
	i := 0
	for i < len(s) && isSpace(s[i]) {
		i++
	}
	n := len(s)
	for n > i && isSpace(s[n-1]) {
		n--
	}
	return s[i:n]
}

func hasContinuationLine(r *bufio.Reader) bool {
	c, err := r.ReadByte()
	if err != nil {
		return false // bufio will keep err until next read.
	}
	r.UnreadByte()
	return isSpace(c)
}

func readContinuedLineSlice(r *bufio.Reader) ([]byte, error) {
	// Read the first line. We preallocate slice that it enough
	// for most fields.
	line, err := readLineSlice(r, make([]byte, 0, 256))
	if err == io.EOF && len(line) == 0 {
		// Header without a body
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	if len(line) == 0 { // blank line - no continuation
		return line, nil
	}

	line = append(line, '\r', '\n')

	// Read continuation lines.
	for hasContinuationLine(r) {
		line, err = readLineSlice(r, line)
		if err != nil {
			break // bufio will keep err until next read.
		}

		line = append(line, '\r', '\n')
	}

	return line, nil
}

func writeContinued(b *strings.Builder, l []byte) {
	// Strip trailing \r, if any
	if len(l) > 0 && l[len(l)-1] == '\r' {
		l = l[:len(l)-1]
	}
	l = trim(l)
	if len(l) == 0 {
		return
	}
	if b.Len() > 0 {
		b.WriteByte(' ')
	}
	b.Write(l)
}

// Strip newlines and spaces around newlines.
func trimAroundNewlines(v []byte) string {
	var b strings.Builder
	b.Grow(len(v))
	for {
		i := bytes.IndexByte(v, '\n')
		if i < 0 {
			writeContinued(&b, v)
			break
		}
		writeContinued(&b, v[:i])
		v = v[i+1:]
	}

	return b.String()
}

// ReadHeader reads a MIME header from r. The header is a sequence of possibly
// continued "Key: Value" lines ending in a blank line.
//
// To avoid denial of service attacks, the provided bufio.Reader should be
// reading from an io.LimitedReader or a similar Reader to bound the size of
// headers.
func ReadHeader(r *bufio.Reader) (Header, error) {
	fs := make([]*headerField, 0, 32)

	// The first line cannot start with a leading space.
	if buf, err := r.Peek(1); err == nil && isSpace(buf[0]) {
		line, err := readLineSlice(r, nil)
		if err != nil {
			return newHeader(fs), err
		}

		return newHeader(fs), fmt.Errorf("message: malformed MIME header initial line: %v", string(line))
	}

	for {
		kv, err := readContinuedLineSlice(r)
		if len(kv) == 0 {
			return newHeader(fs), err
		}

		// Key ends at first colon; should not have trailing spaces but they
		// appear in the wild, violating specs, so we remove them if present.
		i := bytes.IndexByte(kv, ':')
		if i < 0 {
			return newHeader(fs), fmt.Errorf("message: malformed MIME header line: %v", string(kv))
		}

		keyBytes := trim(kv[:i])

		// Verify that there are no invalid characters in the header key.
		// See RFC 5322 Section 2.2
		for _, c := range keyBytes {
			if !validHeaderKeyByte(c) {
				return newHeader(fs), fmt.Errorf("message: malformed MIME header key: %v", string(keyBytes))
			}
		}

		key := textproto.CanonicalMIMEHeaderKey(string(keyBytes))

		// As per RFC 7230 field-name is a token, tokens consist of one or more
		// chars. We could return a an error here, but better to be liberal in
		// what we accept, so if we get an empty key, skip it.
		if key == "" {
			continue
		}

		i++ // skip colon
		v := kv[i:]

		value := trimAroundNewlines(v)
		fs = append(fs, newHeaderField(key, value, kv))

		if err != nil {
			return newHeader(fs), err
		}
	}
}

func foldLine(v string, maxlen int) (line, next string, ok bool) {
	ok = true

	// We'll need to fold before maxlen
	foldBefore := maxlen + 1
	foldAt := len(v)

	var folding string
	if foldBefore > len(v) {
		// We reached the end of the string
		if v[len(v)-1] != '\n' {
			// If there isn't already a trailing CRLF, insert one
			folding = "\r\n"
		}
	} else {
		// Find the closest whitespace before maxlen
		foldAt = strings.LastIndexAny(v[:foldBefore], " \t\n")

		if foldAt == 0 {
			// The whitespace we found was the previous folding WSP
			foldAt = foldBefore - 1
		} else if foldAt < 0 {
			// We didn't find any whitespace, we have to insert one
			foldAt = foldBefore - 2
		}

		switch v[foldAt] {
		case ' ', '\t':
			if v[foldAt-1] != '\n' {
				folding = "\r\n" // The next char will be a WSP, don't need to insert one
			}
		case '\n':
			folding = "" // There is already a CRLF, nothing to do
		default:
			// Another char, we need to insert CRLF + WSP. This will insert an
			// extra space in the string, so this should be avoided if
			// possible.
			folding = "\r\n "
			ok = false
		}
	}

	return v[:foldAt] + folding, v[foldAt:], ok
}

const (
	preferredHeaderLen = 76
	maxHeaderLen       = 998
)

// formatHeaderField formats a header field, ensuring each line is no longer
// than 76 characters. It tries to fold lines at whitespace characters if
// possible. If the header contains a word longer than this limit, it will be
// split.
func formatHeaderField(k, v string) string {
	s := k + ": "

	if v == "" {
		return s + "\r\n"
	}

	first := true
	for len(v) > 0 {
		// If this is the first line, substract the length of the key
		keylen := 0
		if first {
			keylen = len(s)
		}

		// First try with a soft limit
		l, next, ok := foldLine(v, preferredHeaderLen-keylen)
		if !ok {
			// Folding failed to preserve the original header field value. Try
			// with a larger, hard limit.
			l, next, _ = foldLine(v, maxHeaderLen-keylen)
		}
		v = next
		s += l
		first = false
	}

	return s
}

// WriteHeader writes a MIME header to w.
func WriteHeader(w io.Writer, h Header) error {
	for i := len(h.l) - 1; i >= 0; i-- {
		f := h.l[i]
		if rawField, err := f.raw(); err == nil {
			if _, err := w.Write(rawField); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("failed to write header field #%v (%q): %w", len(h.l)-i, f.k, err)
		}
	}

	_, err := w.Write([]byte{'\r', '\n'})
	return err
}
