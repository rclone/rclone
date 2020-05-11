# package drpcctx

`import "storj.io/drpc/drpcctx"`

Package drpcctx has helpers to interact with context.Context.

## Usage

#### func  Transport

```go
func Transport(ctx context.Context) (drpc.Transport, bool)
```
Transport returns the drpc.Transport associated with the context and a bool if
it existed.

#### func  WithTransport

```go
func WithTransport(ctx context.Context, tr drpc.Transport) context.Context
```
WithTransport associates the drpc.Transport as a value on the context.

#### type Tracker

```go
type Tracker struct {
	context.Context
}
```

Tracker keeps track of launched goroutines with a context.

#### func  NewTracker

```go
func NewTracker(ctx context.Context) *Tracker
```
NewTracker creates a Tracker bound to the provided context.

#### func (*Tracker) Cancel

```go
func (t *Tracker) Cancel()
```
Cancel cancels the tracker's context.

#### func (*Tracker) Run

```go
func (t *Tracker) Run(cb func(ctx context.Context))
```
Run starts a goroutine running the callback with the tracker as the context.

#### func (*Tracker) Wait

```go
func (t *Tracker) Wait()
```
Wait blocks until all callbacks started with Run have exited.
