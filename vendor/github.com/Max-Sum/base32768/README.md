# base32768
go implementation of base32768, optimized for UTF-16.

Check https://github.com/qntm/base32768 for information about base32768.

## Example

	message := "Hello, World"
	d := []byte(message)
	fmt.Println(message)
	encoding := base32768.SafeEncoding
  
	// Encode to string
	e := encoding.EncodeToString(d)
	fmt.Println(e)
	// Decode string
	d, _ = encoding.DecodeString(e)
	fmt.Println(string(d))
  
	// Encode
	elen := encoding.EncodedLen(len(d))
	eb := make([]byte, elen)
	encoding.Encode(eb, d)
	fmt.Println(eb)
	// Decode
	dlen := encoding.DecodedLen(len(eb))
	db := make([]byte, dlen)
	n, _ := encoding.Decode(db, eb)
	fmt.Println(string(db[:n]))
