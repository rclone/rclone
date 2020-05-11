# package invoke

`import  "storj.io/drpc/drpcmetadata/invoke"`

Package invoke defines the proto messages exposed by drpc for sending metadata
across the wire.

## Usage

#### type InvokeMetadata

```go
type InvokeMetadata struct {
	Data                 map[string]string `protobuf:"bytes,2,rep,name=data,proto3" json:"data,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	XXX_NoUnkeyedLiteral struct{}          `json:"-"`
	XXX_unrecognized     []byte            `json:"-"`
	XXX_sizecache        int32             `json:"-"`
}
```


#### func (*InvokeMetadata) Descriptor

```go
func (*InvokeMetadata) Descriptor() ([]byte, []int)
```

#### func (*InvokeMetadata) GetData

```go
func (m *InvokeMetadata) GetData() map[string]string
```

#### func (*InvokeMetadata) ProtoMessage

```go
func (*InvokeMetadata) ProtoMessage()
```

#### func (*InvokeMetadata) Reset

```go
func (m *InvokeMetadata) Reset()
```

#### func (*InvokeMetadata) String

```go
func (m *InvokeMetadata) String() string
```

#### func (*InvokeMetadata) XXX_DiscardUnknown

```go
func (m *InvokeMetadata) XXX_DiscardUnknown()
```

#### func (*InvokeMetadata) XXX_Marshal

```go
func (m *InvokeMetadata) XXX_Marshal(b []byte, deterministic bool) ([]byte, error)
```

#### func (*InvokeMetadata) XXX_Merge

```go
func (m *InvokeMetadata) XXX_Merge(src proto.Message)
```

#### func (*InvokeMetadata) XXX_Size

```go
func (m *InvokeMetadata) XXX_Size() int
```

#### func (*InvokeMetadata) XXX_Unmarshal

```go
func (m *InvokeMetadata) XXX_Unmarshal(b []byte) error
```
