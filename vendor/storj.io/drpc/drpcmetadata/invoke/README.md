# package invoke

`import "storj.io/drpc/drpcmetadata/invoke"`

Package invoke defines the proto messages exposed by drpc for sending metadata
across the wire.

## Usage

#### type Metadata

```go
type Metadata struct {
	Data                 map[string]string `protobuf:"bytes,1,rep,name=data,proto3" json:"data,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	XXX_NoUnkeyedLiteral struct{}          `json:"-"`
	XXX_unrecognized     []byte            `json:"-"`
	XXX_sizecache        int32             `json:"-"`
}
```


#### func (*Metadata) Descriptor

```go
func (*Metadata) Descriptor() ([]byte, []int)
```

#### func (*Metadata) GetData

```go
func (m *Metadata) GetData() map[string]string
```

#### func (*Metadata) ProtoMessage

```go
func (*Metadata) ProtoMessage()
```

#### func (*Metadata) Reset

```go
func (m *Metadata) Reset()
```

#### func (*Metadata) String

```go
func (m *Metadata) String() string
```

#### func (*Metadata) XXX_DiscardUnknown

```go
func (m *Metadata) XXX_DiscardUnknown()
```

#### func (*Metadata) XXX_Marshal

```go
func (m *Metadata) XXX_Marshal(b []byte, deterministic bool) ([]byte, error)
```

#### func (*Metadata) XXX_Merge

```go
func (m *Metadata) XXX_Merge(src proto.Message)
```

#### func (*Metadata) XXX_Size

```go
func (m *Metadata) XXX_Size() int
```

#### func (*Metadata) XXX_Unmarshal

```go
func (m *Metadata) XXX_Unmarshal(b []byte) error
```
