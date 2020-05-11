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
func AddPairs(ctx context.Context, md map[string]string) context.Context
```
AddPairs attaches metadata onto a context and return the context.

#### func  Decode

```go
func Decode(data []byte) (*invoke.InvokeMetadata, error)
```
Decode translate byte form of metadata into metadata struct defined by protobuf.

#### func  Encode

```go
func Encode(buffer []byte) ([]byte, error)
```
Encode generates byte form of the metadata and appends it onto the passed in
buffer.
