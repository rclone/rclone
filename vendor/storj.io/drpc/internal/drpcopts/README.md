# package drpcopts

`import "storj.io/drpc/internal/drpcopts"`

Package drpcopts contains internal options.

This package allows options to exist that are too sharp to provide to typical
users of the library that are not required to be backward compatible.

## Usage

#### func  GetStreamFin

```go
func GetStreamFin(opts *Stream) chan<- struct{}
```
GetStreamFin returns the chan<- struct{} stored in the options.

#### func  GetStreamKind

```go
func GetStreamKind(opts *Stream) string
```
GetStreamKind returns the kind debug string stored in the options.

#### func  GetStreamTransport

```go
func GetStreamTransport(opts *Stream) drpc.Transport
```
GetStreamTransport returns the drpc.Transport stored in the options.

#### func  SetStreamFin

```go
func SetStreamFin(opts *Stream, fin chan<- struct{})
```
SetStreamFin sets the chan<- struct{} stored in the options.

#### func  SetStreamKind

```go
func SetStreamKind(opts *Stream, kind string)
```
SetStreamKind sets the kind debug string stored in the options.

#### func  SetStreamTransport

```go
func SetStreamTransport(opts *Stream, tr drpc.Transport)
```
SetStreamTransport sets the drpc.Transport stored in the options.

#### type Stream

```go
type Stream struct {
}
```

Stream contains internal options for the drpcstream package.
