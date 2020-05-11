# package drpcdebug

`import "storj.io/drpc/drpcdebug"`

Package drpcdebug provides helpers for debugging.

## Usage

#### func  Log

```go
func Log(cb func() string)
```
Log executes the callback for a string to log if built with the debug tag.
