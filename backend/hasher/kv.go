package hasher

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/lib/kv"
)

const (
	timeFormat     = "2006-01-02T15:04:05.000000000-0700"
	anyFingerprint = "*"
)

type hashMap map[hash.Type]string

type hashRecord struct {
	Fp      string // fingerprint
	Hashes  operations.HashSums
	Created time.Time
}

func (r *hashRecord) encode(key string) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(r); err != nil {
		fs.Debugf(key, "hasher encoding %v: %v", r, err)
		return nil, err
	}
	return buf.Bytes(), nil
}

func (r *hashRecord) decode(key string, data []byte) error {
	if err := gob.NewDecoder(bytes.NewBuffer(data)).Decode(r); err != nil {
		fs.Debugf(key, "hasher decoding %q failed: %v", data, err)
		return err
	}
	return nil
}

// kvPrune: prune a single hash
type kvPrune struct {
	key string
}

func (op *kvPrune) Do(ctx context.Context, b kv.Bucket) error {
	return b.Delete([]byte(op.key))
}

// kvPurge: delete a subtree
type kvPurge struct {
	dir string
}

func (op *kvPurge) Do(ctx context.Context, b kv.Bucket) error {
	dir := op.dir
	if !strings.HasSuffix(dir, "/") {
		dir += "/"
	}
	var items []string
	cur := b.Cursor()
	bkey, _ := cur.Seek([]byte(dir))
	for bkey != nil {
		key := string(bkey)
		if !strings.HasPrefix(key, dir) {
			break
		}
		items = append(items, key[len(dir):])
		bkey, _ = cur.Next()
	}
	nerr := 0
	for _, sub := range items {
		if err := b.Delete([]byte(dir + sub)); err != nil {
			nerr++
		}
	}
	fs.Debugf(dir, "%d hashes purged, %d failed", len(items)-nerr, nerr)
	return nil
}

// kvMove: assign hashes to new path
type kvMove struct {
	src string
	dst string
	dir bool
	fs  *Fs
}

func (op *kvMove) Do(ctx context.Context, b kv.Bucket) error {
	src, dst := op.src, op.dst
	if !op.dir {
		err := moveHash(b, src, dst)
		fs.Debugf(op.fs, "moving cached hash %s to %s (err: %v)", src, dst, err)
		return err
	}

	if !strings.HasSuffix(src, "/") {
		src += "/"
	}
	if !strings.HasSuffix(dst, "/") {
		dst += "/"
	}

	var items []string
	cur := b.Cursor()
	bkey, _ := cur.Seek([]byte(src))
	for bkey != nil {
		key := string(bkey)
		if !strings.HasPrefix(key, src) {
			break
		}
		items = append(items, key[len(src):])
		bkey, _ = cur.Next()
	}

	nerr := 0
	for _, suffix := range items {
		srcKey, dstKey := src+suffix, dst+suffix
		err := moveHash(b, srcKey, dstKey)
		fs.Debugf(op.fs, "Rename cache record %s -> %s (err: %v)", srcKey, dstKey, err)
		if err != nil {
			nerr++
		}
	}
	fs.Debugf(op.fs, "%d hashes moved, %d failed", len(items)-nerr, nerr)
	return nil
}

func moveHash(b kv.Bucket, src, dst string) error {
	data := b.Get([]byte(src))
	err := b.Delete([]byte(src))
	if err != nil || len(data) == 0 {
		return err
	}
	return b.Put([]byte(dst), data)
}

// kvGet: get single hash from database
type kvGet struct {
	key  string
	fp   string
	hash string
	val  string
	age  time.Duration
}

func (op *kvGet) Do(ctx context.Context, b kv.Bucket) error {
	data := b.Get([]byte(op.key))
	if len(data) == 0 {
		return errors.New("no record")
	}
	var r hashRecord
	if err := r.decode(op.key, data); err != nil {
		return errors.New("invalid record")
	}
	if !(r.Fp == anyFingerprint || op.fp == anyFingerprint || r.Fp == op.fp) {
		return errors.New("fingerprint changed")
	}
	if time.Since(r.Created) > op.age {
		return errors.New("record timed out")
	}
	if r.Hashes != nil {
		op.val = r.Hashes[op.hash]
	}
	return nil
}

// kvPut: set hashes for an object by key
type kvPut struct {
	key    string
	fp     string
	hashes operations.HashSums
	age    time.Duration
}

func (op *kvPut) Do(ctx context.Context, b kv.Bucket) (err error) {
	data := b.Get([]byte(op.key))
	var r hashRecord
	if len(data) > 0 {
		err = r.decode(op.key, data)
		if err != nil || r.Fp != op.fp || time.Since(r.Created) > op.age {
			r.Hashes = nil
		}
	}
	if len(r.Hashes) == 0 {
		r.Created = time.Now()
		r.Hashes = operations.HashSums{}
		r.Fp = op.fp
	}

	for hashType, hashVal := range op.hashes {
		r.Hashes[hashType] = hashVal
	}
	if data, err = r.encode(op.key); err != nil {
		return errors.Wrap(err, "marshal failed")
	}
	if err = b.Put([]byte(op.key), data); err != nil {
		return errors.Wrap(err, "put failed")
	}
	return err
}

// kvDump: dump the database.
// Note: long dump can cause concurrent operations to fail.
type kvDump struct {
	full  bool
	root  string
	path  string
	fs    *Fs
	num   int
	total int
}

func (op *kvDump) Do(ctx context.Context, b kv.Bucket) error {
	f, baseRoot, dbPath := op.fs, op.root, op.path

	if op.full {
		total := 0
		num := 0
		_ = b.ForEach(func(bkey, data []byte) error {
			total++
			key := string(bkey)
			include := (baseRoot == "" || key == baseRoot || strings.HasPrefix(key, baseRoot+"/"))
			var r hashRecord
			if err := r.decode(key, data); err != nil {
				fs.Errorf(nil, "%s: invalid record: %v", key, err)
				return nil
			}
			fmt.Println(f.dumpLine(&r, key, include, nil))
			if include {
				num++
			}
			return nil
		})
		fs.Infof(dbPath, "%d records out of %d", num, total)
		op.num, op.total = num, total // for unit tests
		return nil
	}

	num := 0
	cur := b.Cursor()
	var bkey, data []byte
	if baseRoot != "" {
		bkey, data = cur.Seek([]byte(baseRoot))
	} else {
		bkey, data = cur.First()
	}
	for bkey != nil {
		key := string(bkey)
		if !(baseRoot == "" || key == baseRoot || strings.HasPrefix(key, baseRoot+"/")) {
			break
		}
		var r hashRecord
		if err := r.decode(key, data); err != nil {
			fs.Errorf(nil, "%s: invalid record: %v", key, err)
			continue
		}
		if key = strings.TrimPrefix(key[len(baseRoot):], "/"); key == "" {
			key = "/"
		}
		fmt.Println(f.dumpLine(&r, key, true, nil))
		num++
		bkey, data = cur.Next()
	}
	fs.Infof(dbPath, "%d records", num)
	op.num = num // for unit tests
	return nil
}

func (f *Fs) dumpLine(r *hashRecord, path string, include bool, err error) string {
	var status string
	switch {
	case !include:
		status = "ext"
	case err != nil:
		status = "bad"
	case r.Fp == anyFingerprint:
		status = "stk"
	default:
		status = "ok "
	}

	var hashes []string
	for _, hashType := range f.keepHashes.Array() {
		hashName := hashType.String()
		hashVal := r.Hashes[hashName]
		if hashVal == "" || err != nil {
			hashVal = "-"
		}
		hashVal = fmt.Sprintf("%-*s", hash.Width(hashType), hashVal)
		hashes = append(hashes, hashName+":"+hashVal)
	}
	hashesStr := strings.Join(hashes, " ")

	age := time.Since(r.Created).Round(time.Second)
	if age > 24*time.Hour {
		age = age.Round(time.Hour)
	}
	if err != nil {
		age = 0
	}
	ageStr := age.String()
	if strings.HasSuffix(ageStr, "h0m0s") {
		ageStr = strings.TrimSuffix(ageStr, "0m0s")
	}

	return fmt.Sprintf("%s %s %9s %s", status, hashesStr, ageStr, path)
}
