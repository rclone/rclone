# package drpcsignal

`import "storj.io/drpc/drpcsignal"`

Package drpcsignal holds a helper type to signal errors.

## Usage

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
