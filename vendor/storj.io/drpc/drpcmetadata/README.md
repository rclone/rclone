# package drpcmetadata

`import "storj.io/drpc/drpcmetadata"`

Package drpcmetadata define the structure of the metadata supported by drpc
library.

## Usage

#### func  Add

```go
func Add(ctx context.Context, key, value string) context.Context
```
Add associates a key/value pair on the context.

#### func  AddPairs

```go
func AddPairs(ctx context.Context, metadata map[string]string) context.Context
```
AddPairs attaches metadata onto a context and return the context.

#### func  Decode

```go
func Decode(data []byte) (map[string]string, error)
```
Decode translate byte form of metadata into key/value metadata.

#### func  Encode

```go
func Encode(buffer []byte, metadata map[string]string) ([]byte, error)
```
Encode generates byte form of the metadata and appends it onto the passed in
buffer.

#### func  Get

```go
func Get(ctx context.Context) (map[string]string, bool)
```
Get returns all key/value pairs on the given context.
