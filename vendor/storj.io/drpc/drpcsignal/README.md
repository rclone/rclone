# package drpcsignal

`import "storj.io/drpc/drpcsignal"`

Package drpcsignal holds a helper type to signal errors.

## Usage

#### type Chan

```go
type Chan struct {
}
```

Chan is a lazily allocated chan struct{} that avoids allocating if it is closed
before being used for anything.

#### func (*Chan) Close

```go
func (c *Chan) Close()
```
Close tries to set the channel to an already closed one if a fresh one has not
already been set, and closes the fresh one otherwise.

#### func (*Chan) Full

```go
func (c *Chan) Full() bool
```
Full returns true if the channel is currently full. The information is
immediately invalid in the sense that a send could always block.

#### func (*Chan) Get

```go
func (c *Chan) Get() chan struct{}
```
Get returns the channel, allocating if necessary.

#### func (*Chan) Make

```go
func (c *Chan) Make(cap uint)
```
Make sets the channel to a freshly allocated channel with the provided capacity.
It is a no-op if called after any other methods.

#### func (*Chan) Recv

```go
func (c *Chan) Recv()
```
Recv receives a value on the channel, allocating if necessary.

#### func (*Chan) Send

```go
func (c *Chan) Send()
```
Send sends a value on the channel, allocating if necessary.

#### type Signal

```go
type Signal struct {
}
```

Signal contains an error value that can be set one and exports a number of ways
to inspect it.

#### func (*Signal) Err

```go
func (s *Signal) Err() error
```
Err returns the error stored in the signal. Since one can store a nil error care
must be taken. A non-nil error returned from this method means that the Signal
has been set, but the inverse is not true.

#### func (*Signal) Get

```go
func (s *Signal) Get() (error, bool)
```
Get returns the error set with the signal and a boolean indicating if the result
is valid.

#### func (*Signal) IsSet

```go
func (s *Signal) IsSet() bool
```
IsSet returns true if the Signal is set.

#### func (*Signal) Set

```go
func (s *Signal) Set(err error) (ok bool)
```
Set stores the error in the signal. It only keeps track of the first error set,
and returns true if it was the first error set.

#### func (*Signal) Signal

```go
func (s *Signal) Signal() chan struct{}
```
Signal returns a channel that will be closed when the signal is set.

#### func (*Signal) Wait

```go
func (s *Signal) Wait()
```
Wait blocks until the signal has been Set.
