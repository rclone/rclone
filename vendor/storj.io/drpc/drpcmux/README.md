# package drpcmux

`import "storj.io/drpc/drpcmux"`

Package drpcmux is a handler to dispatch rpcs to implementations.

## Usage

#### type Mux

```go
type Mux struct {
}
```

Mux is an implementation of Handler to serve drpc connections to the appropriate
Receivers registered by Descriptions.

#### func  New

```go
func New() *Mux
```
New constructs a new Mux.

#### func (*Mux) HandleRPC

```go
func (m *Mux) HandleRPC(stream drpc.Stream, rpc string) (err error)
```
HandleRPC handles the rpc that has been requested by the stream.

#### func (*Mux) Register

```go
func (m *Mux) Register(srv interface{}, desc drpc.Description) error
```
Register associates the rpcs described by the description in the server. It
returns an error if there was a problem registering it.
