package filexfer

// ExtensionPair defines the extension-pair type defined in draft-ietf-secsh-filexfer-13.
// This type is backwards-compatible with how draft-ietf-secsh-filexfer-02 defines extensions.
//
// Defined in: https://tools.ietf.org/html/draft-ietf-secsh-filexfer-13#section-4.2
type ExtensionPair struct {
	Name string
	Data string
}

// Len returns the number of bytes e would marshal into.
func (e *ExtensionPair) Len() int {
	return 4 + len(e.Name) + 4 + len(e.Data)
}

// MarshalInto marshals e onto the end of the given Buffer.
func (e *ExtensionPair) MarshalInto(buf *Buffer) {
	buf.AppendString(e.Name)
	buf.AppendString(e.Data)
}

// MarshalBinary returns e as the binary encoding of e.
func (e *ExtensionPair) MarshalBinary() ([]byte, error) {
	buf := NewBuffer(make([]byte, 0, e.Len()))
	e.MarshalInto(buf)
	return buf.Bytes(), nil
}

// UnmarshalFrom unmarshals an ExtensionPair from the given Buffer into e.
func (e *ExtensionPair) UnmarshalFrom(buf *Buffer) (err error) {
	if e.Name, err = buf.ConsumeString(); err != nil {
		return err
	}

	if e.Data, err = buf.ConsumeString(); err != nil {
		return err
	}

	return nil
}

// UnmarshalBinary decodes the binary encoding of ExtensionPair into e.
func (e *ExtensionPair) UnmarshalBinary(data []byte) error {
	return e.UnmarshalFrom(NewBuffer(data))
}
