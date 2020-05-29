# package drpccache

`import "storj.io/drpc/drpccache"`

Package drpccache implements per stream cache for drpc.

## Usage

#### func  WithContext

```go
func WithContext(parent context.Context, cache *Cache) context.Context
```
WithContext returns a context with the value cache associated with the context.

#### type Cache

```go
type Cache struct {
}
```

Cache is a per stream cache.

#### func  FromContext

```go
func FromContext(ctx context.Context) *Cache
```
FromContext returns a cache from a context.

Example usage:

    cache := drpccache.FromContext(stream.Context())
    if cache != nil {
           value := cache.LoadOrCreate("initialized", func() (interface{}) {
                   return 42
           })
    }

#### func  New

```go
func New() *Cache
```
New returns a new cache.

#### func (*Cache) Clear

```go
func (cache *Cache) Clear()
```
Clear clears the cache.

#### func (*Cache) Load

```go
func (cache *Cache) Load(key interface{}) interface{}
```
Load returns the value with the given key.

#### func (*Cache) LoadOrCreate

```go
func (cache *Cache) LoadOrCreate(key interface{}, fn func() interface{}) interface{}
```
LoadOrCreate returns the value with the given key.

#### func (*Cache) Store

```go
func (cache *Cache) Store(key, value interface{})
```
Store sets the value at a key.
