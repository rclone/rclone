package record // import "bazil.org/fuse/fs/fstestutil/record"

import (
	"sync"
	"sync/atomic"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"golang.org/x/net/context"
)

// Writes gathers data from FUSE Write calls.
type Writes struct {
	buf Buffer
}

var _ = fs.HandleWriter(&Writes{})

func (w *Writes) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	n, err := w.buf.Write(req.Data)
	resp.Size = n
	if err != nil {
		return err
	}
	return nil
}

func (w *Writes) RecordedWriteData() []byte {
	return w.buf.Bytes()
}

// Counter records number of times a thing has occurred.
type Counter struct {
	count uint32
}

func (r *Counter) Inc() {
	atomic.AddUint32(&r.count, 1)
}

func (r *Counter) Count() uint32 {
	return atomic.LoadUint32(&r.count)
}

// MarkRecorder records whether a thing has occurred.
type MarkRecorder struct {
	count Counter
}

func (r *MarkRecorder) Mark() {
	r.count.Inc()
}

func (r *MarkRecorder) Recorded() bool {
	return r.count.Count() > 0
}

// Flushes notes whether a FUSE Flush call has been seen.
type Flushes struct {
	rec MarkRecorder
}

var _ = fs.HandleFlusher(&Flushes{})

func (r *Flushes) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	r.rec.Mark()
	return nil
}

func (r *Flushes) RecordedFlush() bool {
	return r.rec.Recorded()
}

type Recorder struct {
	mu  sync.Mutex
	val interface{}
}

// Record that we've seen value. A nil value is indistinguishable from
// no value recorded.
func (r *Recorder) Record(value interface{}) {
	r.mu.Lock()
	r.val = value
	r.mu.Unlock()
}

func (r *Recorder) Recorded() interface{} {
	r.mu.Lock()
	val := r.val
	r.mu.Unlock()
	return val
}

type RequestRecorder struct {
	rec Recorder
}

// Record a fuse.Request, after zeroing header fields that are hard to
// reproduce.
//
// Make sure to record a copy, not the original request.
func (r *RequestRecorder) RecordRequest(req fuse.Request) {
	hdr := req.Hdr()
	*hdr = fuse.Header{}
	r.rec.Record(req)
}

func (r *RequestRecorder) Recorded() fuse.Request {
	val := r.rec.Recorded()
	if val == nil {
		return nil
	}
	return val.(fuse.Request)
}

// Setattrs records a Setattr request and its fields.
type Setattrs struct {
	rec RequestRecorder
}

var _ = fs.NodeSetattrer(&Setattrs{})

func (r *Setattrs) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {
	tmp := *req
	r.rec.RecordRequest(&tmp)
	return nil
}

func (r *Setattrs) RecordedSetattr() fuse.SetattrRequest {
	val := r.rec.Recorded()
	if val == nil {
		return fuse.SetattrRequest{}
	}
	return *(val.(*fuse.SetattrRequest))
}

// Fsyncs records an Fsync request and its fields.
type Fsyncs struct {
	rec RequestRecorder
}

var _ = fs.NodeFsyncer(&Fsyncs{})

func (r *Fsyncs) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	tmp := *req
	r.rec.RecordRequest(&tmp)
	return nil
}

func (r *Fsyncs) RecordedFsync() fuse.FsyncRequest {
	val := r.rec.Recorded()
	if val == nil {
		return fuse.FsyncRequest{}
	}
	return *(val.(*fuse.FsyncRequest))
}

// Mkdirs records a Mkdir request and its fields.
type Mkdirs struct {
	rec RequestRecorder
}

var _ = fs.NodeMkdirer(&Mkdirs{})

// Mkdir records the request and returns an error. Most callers should
// wrap this call in a function that returns a more useful result.
func (r *Mkdirs) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	tmp := *req
	r.rec.RecordRequest(&tmp)
	return nil, fuse.EIO
}

// RecordedMkdir returns information about the Mkdir request.
// If no request was seen, returns a zero value.
func (r *Mkdirs) RecordedMkdir() fuse.MkdirRequest {
	val := r.rec.Recorded()
	if val == nil {
		return fuse.MkdirRequest{}
	}
	return *(val.(*fuse.MkdirRequest))
}

// Symlinks records a Symlink request and its fields.
type Symlinks struct {
	rec RequestRecorder
}

var _ = fs.NodeSymlinker(&Symlinks{})

// Symlink records the request and returns an error. Most callers should
// wrap this call in a function that returns a more useful result.
func (r *Symlinks) Symlink(ctx context.Context, req *fuse.SymlinkRequest) (fs.Node, error) {
	tmp := *req
	r.rec.RecordRequest(&tmp)
	return nil, fuse.EIO
}

// RecordedSymlink returns information about the Symlink request.
// If no request was seen, returns a zero value.
func (r *Symlinks) RecordedSymlink() fuse.SymlinkRequest {
	val := r.rec.Recorded()
	if val == nil {
		return fuse.SymlinkRequest{}
	}
	return *(val.(*fuse.SymlinkRequest))
}

// Links records a Link request and its fields.
type Links struct {
	rec RequestRecorder
}

var _ = fs.NodeLinker(&Links{})

// Link records the request and returns an error. Most callers should
// wrap this call in a function that returns a more useful result.
func (r *Links) Link(ctx context.Context, req *fuse.LinkRequest, old fs.Node) (fs.Node, error) {
	tmp := *req
	r.rec.RecordRequest(&tmp)
	return nil, fuse.EIO
}

// RecordedLink returns information about the Link request.
// If no request was seen, returns a zero value.
func (r *Links) RecordedLink() fuse.LinkRequest {
	val := r.rec.Recorded()
	if val == nil {
		return fuse.LinkRequest{}
	}
	return *(val.(*fuse.LinkRequest))
}

// Mknods records a Mknod request and its fields.
type Mknods struct {
	rec RequestRecorder
}

var _ = fs.NodeMknoder(&Mknods{})

// Mknod records the request and returns an error. Most callers should
// wrap this call in a function that returns a more useful result.
func (r *Mknods) Mknod(ctx context.Context, req *fuse.MknodRequest) (fs.Node, error) {
	tmp := *req
	r.rec.RecordRequest(&tmp)
	return nil, fuse.EIO
}

// RecordedMknod returns information about the Mknod request.
// If no request was seen, returns a zero value.
func (r *Mknods) RecordedMknod() fuse.MknodRequest {
	val := r.rec.Recorded()
	if val == nil {
		return fuse.MknodRequest{}
	}
	return *(val.(*fuse.MknodRequest))
}

// Opens records a Open request and its fields.
type Opens struct {
	rec RequestRecorder
}

var _ = fs.NodeOpener(&Opens{})

// Open records the request and returns an error. Most callers should
// wrap this call in a function that returns a more useful result.
func (r *Opens) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	tmp := *req
	r.rec.RecordRequest(&tmp)
	return nil, fuse.EIO
}

// RecordedOpen returns information about the Open request.
// If no request was seen, returns a zero value.
func (r *Opens) RecordedOpen() fuse.OpenRequest {
	val := r.rec.Recorded()
	if val == nil {
		return fuse.OpenRequest{}
	}
	return *(val.(*fuse.OpenRequest))
}

// Getxattrs records a Getxattr request and its fields.
type Getxattrs struct {
	rec RequestRecorder
}

var _ = fs.NodeGetxattrer(&Getxattrs{})

// Getxattr records the request and returns an error. Most callers should
// wrap this call in a function that returns a more useful result.
func (r *Getxattrs) Getxattr(ctx context.Context, req *fuse.GetxattrRequest, resp *fuse.GetxattrResponse) error {
	tmp := *req
	r.rec.RecordRequest(&tmp)
	return fuse.ErrNoXattr
}

// RecordedGetxattr returns information about the Getxattr request.
// If no request was seen, returns a zero value.
func (r *Getxattrs) RecordedGetxattr() fuse.GetxattrRequest {
	val := r.rec.Recorded()
	if val == nil {
		return fuse.GetxattrRequest{}
	}
	return *(val.(*fuse.GetxattrRequest))
}

// Listxattrs records a Listxattr request and its fields.
type Listxattrs struct {
	rec RequestRecorder
}

var _ = fs.NodeListxattrer(&Listxattrs{})

// Listxattr records the request and returns an error. Most callers should
// wrap this call in a function that returns a more useful result.
func (r *Listxattrs) Listxattr(ctx context.Context, req *fuse.ListxattrRequest, resp *fuse.ListxattrResponse) error {
	tmp := *req
	r.rec.RecordRequest(&tmp)
	return fuse.ErrNoXattr
}

// RecordedListxattr returns information about the Listxattr request.
// If no request was seen, returns a zero value.
func (r *Listxattrs) RecordedListxattr() fuse.ListxattrRequest {
	val := r.rec.Recorded()
	if val == nil {
		return fuse.ListxattrRequest{}
	}
	return *(val.(*fuse.ListxattrRequest))
}

// Setxattrs records a Setxattr request and its fields.
type Setxattrs struct {
	rec RequestRecorder
}

var _ = fs.NodeSetxattrer(&Setxattrs{})

// Setxattr records the request and returns an error. Most callers should
// wrap this call in a function that returns a more useful result.
func (r *Setxattrs) Setxattr(ctx context.Context, req *fuse.SetxattrRequest) error {
	tmp := *req
	// The byte slice points to memory that will be reused, so make a
	// deep copy.
	tmp.Xattr = append([]byte(nil), req.Xattr...)
	r.rec.RecordRequest(&tmp)
	return nil
}

// RecordedSetxattr returns information about the Setxattr request.
// If no request was seen, returns a zero value.
func (r *Setxattrs) RecordedSetxattr() fuse.SetxattrRequest {
	val := r.rec.Recorded()
	if val == nil {
		return fuse.SetxattrRequest{}
	}
	return *(val.(*fuse.SetxattrRequest))
}

// Removexattrs records a Removexattr request and its fields.
type Removexattrs struct {
	rec RequestRecorder
}

var _ = fs.NodeRemovexattrer(&Removexattrs{})

// Removexattr records the request and returns an error. Most callers should
// wrap this call in a function that returns a more useful result.
func (r *Removexattrs) Removexattr(ctx context.Context, req *fuse.RemovexattrRequest) error {
	tmp := *req
	r.rec.RecordRequest(&tmp)
	return nil
}

// RecordedRemovexattr returns information about the Removexattr request.
// If no request was seen, returns a zero value.
func (r *Removexattrs) RecordedRemovexattr() fuse.RemovexattrRequest {
	val := r.rec.Recorded()
	if val == nil {
		return fuse.RemovexattrRequest{}
	}
	return *(val.(*fuse.RemovexattrRequest))
}

// Creates records a Create request and its fields.
type Creates struct {
	rec RequestRecorder
}

var _ = fs.NodeCreater(&Creates{})

// Create records the request and returns an error. Most callers should
// wrap this call in a function that returns a more useful result.
func (r *Creates) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	tmp := *req
	r.rec.RecordRequest(&tmp)
	return nil, nil, fuse.EIO
}

// RecordedCreate returns information about the Create request.
// If no request was seen, returns a zero value.
func (r *Creates) RecordedCreate() fuse.CreateRequest {
	val := r.rec.Recorded()
	if val == nil {
		return fuse.CreateRequest{}
	}
	return *(val.(*fuse.CreateRequest))
}
