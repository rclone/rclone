# package drpcdebug

`import "storj.io/drpc/drpcdebug"`

Package drpcdebug provides helpers for debugging.

## Usage

```go
const Enabled = enabled
```
Enabled is a constant describing if logs are enabled or not.

#### func  Log

```go
func Log(cb func() (who, what, why string))
```
Log executes the callback for a string to log if built with the debug tag.
