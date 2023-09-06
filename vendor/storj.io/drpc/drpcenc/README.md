# package drpcenc

`import "storj.io/drpc/drpcenc"`

Package drpcenc holds some helper functions for encoding messages.

## Usage

#### func  MarshalAppend

```go
func MarshalAppend(msg drpc.Message, enc drpc.Encoding, buf []byte) (data []byte, err error)
```
MarshalAppend calls enc.Marshal(msg) and returns the data appended to buf. If
enc implements MarshalAppend, that is called instead.
