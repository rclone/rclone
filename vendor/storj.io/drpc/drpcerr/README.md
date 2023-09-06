# package drpcerr

`import "storj.io/drpc/drpcerr"`

Package drpcerr lets one associate error codes with errors.

## Usage

```go
const (
	// Unimplemented is the code used by the generated unimplemented
	// servers when returning errors.
	Unimplemented = 12
)
```

#### func  Code

```go
func Code(err error) uint64
```
Code returns the error code associated with the error or 0 if none is.

#### func  WithCode

```go
func WithCode(err error, code uint64) error
```
WithCode associates the code with the error if it is non nil and the code is
non-zero.
