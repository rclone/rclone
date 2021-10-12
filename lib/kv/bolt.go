//go:build !plan9 && !js
// +build !plan9,!js

package kv

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/lib/encoder"
	"go.etcd.io/bbolt"
)

const (
	initTime   = 24 * time.Hour // something reasonably long
	dbFileMode = 0600
	dbDirMode  = 0700
	queueSize  = 2
)

// DB represents a key-value database
type DB struct {
	name      string
	path      string
	facility  string
	refs      int
	bolt      *bbolt.DB
	mu        sync.Mutex
	canWrite  bool
	queue     chan *request
	lockTime  time.Duration
	idleTime  time.Duration
	openTime  time.Duration
	idleTimer *time.Timer
	lockTimer *time.Timer
}

var (
	dbMap = map[string]*DB{}
	dbMut = sync.Mutex{}
)

// Supported returns true on supported OSes
func Supported() bool { return true }

// makeName makes a store name
func makeName(facility string, f fs.Fs) string {
	var name string
	if f != nil {
		name = f.Name()
		if idx := strings.Index(name, "{"); idx != -1 {
			name = name[:idx]
		}
		name = encoder.OS.FromStandardPath(name)
		name += "~"
	}
	return name + facility + ".bolt"
}

// Start a new key-value database
func Start(ctx context.Context, facility string, f fs.Fs) (*DB, error) {
	if db := Get(facility, f); db != nil {
		return db, nil
	}

	dir := filepath.Join(config.GetCacheDir(), "kv")
	if err := os.MkdirAll(dir, dbDirMode); err != nil {
		return nil, err
	}

	name := makeName(facility, f)
	lockTime := fs.GetConfig(ctx).KvLockTime

	db := &DB{
		name:      name,
		path:      filepath.Join(dir, name),
		facility:  facility,
		refs:      1,
		lockTime:  lockTime,
		idleTime:  lockTime / 4,
		openTime:  lockTime * 2,
		idleTimer: time.NewTimer(initTime),
		lockTimer: time.NewTimer(initTime),
		queue:     make(chan *request, queueSize),
	}

	fi, err := os.Stat(db.path)
	if strings.HasSuffix(os.Args[0], ".test") || (err == nil && fi.Size() == 0) {
		_ = os.Remove(db.path)
		fs.Infof(db.name, "drop cache remaining after unit test")
	}

	if err = db.open(ctx, false); err != nil && err != ErrEmpty {
		return nil, errors.Wrapf(err, "cannot open db: %s", db.path)
	}

	// Initialization above was performed without locks..
	dbMut.Lock()
	defer dbMut.Unlock()
	if dbOther := dbMap[name]; dbOther != nil {
		// Races between concurrent Start's are rare but possible, the 1st one wins.
		_ = db.close()
		return dbOther, nil
	}
	go db.loop() // Start queue handling
	return db, nil
}

// Get returns database record for given filesystem and facility
func Get(facility string, f fs.Fs) *DB {
	name := makeName(facility, f)
	dbMut.Lock()
	db := dbMap[name]
	if db != nil {
		db.mu.Lock()
		db.refs++
		db.mu.Unlock()
	}
	dbMut.Unlock()
	return db
}

// free database record
func (db *DB) free() {
	dbMut.Lock()
	db.mu.Lock()
	db.refs--
	if db.refs <= 0 {
		delete(dbMap, db.name)
	}
	db.mu.Unlock()
	dbMut.Unlock()
}

// Path returns database path
func (db *DB) Path() string { return db.path }

var modeNames = map[bool]string{false: "reading", true: "writing"}

func (db *DB) open(ctx context.Context, forWrite bool) (err error) {
	if db.bolt != nil && (db.canWrite || !forWrite) {
		return nil
	}
	_ = db.close()

	db.canWrite = forWrite
	if !forWrite {
		// mitigate https://github.com/etcd-io/bbolt/issues/98
		_, err = os.Stat(db.path)
		if os.IsNotExist(err) {
			return ErrEmpty
		}
	}

	opt := &bbolt.Options{
		Timeout:  db.openTime,
		ReadOnly: !forWrite,
	}
	openMode := modeNames[forWrite]
	startTime := time.Now()
	var bolt *bbolt.DB
	retry := 1
	maxRetries := fs.GetConfig(ctx).LowLevelRetries
	for {
		bolt, err = bbolt.Open(db.path, dbFileMode, opt)
		if err == nil || retry >= maxRetries {
			break
		}
		fs.Debugf(db.name, "Retry #%d opening for %s: %v", retry, openMode, err)
		retry++
	}
	if err != nil {
		return err
	}

	fs.Debugf(db.name, "Opened for %s in %v", openMode, time.Since(startTime))
	_ = db.lockTimer.Reset(db.lockTime)
	_ = db.idleTimer.Reset(db.idleTime)
	db.bolt = bolt
	return nil
}

func (db *DB) close() (err error) {
	if db.bolt != nil {
		_ = db.lockTimer.Stop()
		_ = db.idleTimer.Stop()
		err = db.bolt.Close()
		db.bolt = nil
		fs.Debugf(db.name, "released")
	}
	return
}

// loop over database operations sequentially
func (db *DB) loop() {
	ctx := context.Background()
	for db.queue != nil {
		select {
		case req := <-db.queue:
			req.handle(ctx, db)
			_ = db.idleTimer.Reset(db.idleTime)
		case <-db.idleTimer.C:
			_ = db.close()
		case <-db.lockTimer.C:
			_ = db.close()
		}
	}
	db.free()
}

// Do a key-value operation and return error when done
func (db *DB) Do(write bool, op Op) error {
	if db.queue == nil {
		return ErrInactive
	}
	r := &request{
		op: op,
		wr: write,
	}
	r.wg.Add(1)
	db.queue <- r
	r.wg.Wait()
	return r.err
}

// request encapsulates a synchronous operation and its results
type request struct {
	op  Op
	wr  bool
	err error
	wg  sync.WaitGroup
}

// handle a key-value request with given DB
func (r *request) handle(ctx context.Context, db *DB) {
	db.mu.Lock()
	if op, stop := r.op.(*opStop); stop {
		r.err = db.close()
		if op.remove {
			if err := os.Remove(db.path); !os.IsNotExist(err) {
				r.err = err
			}
		}
		db.queue = nil
	} else {
		r.err = db.execute(ctx, r.op, r.wr)
	}
	db.mu.Unlock()
	r.wg.Done()
}

// execute a key-value DB operation
func (db *DB) execute(ctx context.Context, op Op, write bool) error {
	if err := db.open(ctx, write); err != nil {
		return err
	}
	if write {
		return db.bolt.Update(func(tx *bbolt.Tx) error {
			b, err := tx.CreateBucketIfNotExists([]byte(db.facility))
			if err != nil || b == nil {
				return ErrEmpty
			}
			return op.Do(ctx, &bucketAdapter{b})
		})
	}
	return db.bolt.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(db.facility))
		if b == nil {
			return ErrEmpty
		}
		return op.Do(ctx, &bucketAdapter{b})
	})
}

// bucketAdapter is a thin wrapper adapting kv.Bucket to bbolt.Bucket
type bucketAdapter struct {
	*bbolt.Bucket
}

func (b *bucketAdapter) Cursor() Cursor {
	return b.Bucket.Cursor()
}

// Stop a database loop, optionally removing the file
func (db *DB) Stop(remove bool) error {
	return db.Do(false, &opStop{remove: remove})
}

// opStop: close database and stop operation loop
type opStop struct {
	remove bool
}

func (*opStop) Do(context.Context, Bucket) error {
	return nil
}

// Exit stops all databases
func Exit() {
	dbMut.Lock()
	for _, s := range dbMap {
		_ = s.Stop(false)
	}
	dbMut.Unlock()
}
