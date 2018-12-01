package resolver

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"sync"

	"github.com/gobuffalo/packr/v2/file/resolver/encoding/hex"
	"github.com/gobuffalo/packr/v2/plog"

	"github.com/gobuffalo/packr/v2/file"
	"github.com/pkg/errors"
)

var _ Resolver = &HexGzip{}

type HexGzip struct {
	packed   map[string]string
	unpacked map[string]string
	moot     *sync.RWMutex
}

func (hg HexGzip) String() string {
	return String(&hg)
}

var _ file.FileMappable = &HexGzip{}

func (hg *HexGzip) FileMap() map[string]file.File {
	hg.moot.RLock()
	var names []string
	for k := range hg.packed {
		names = append(names, k)
	}
	hg.moot.RUnlock()
	m := map[string]file.File{}
	for _, n := range names {
		if f, err := hg.Resolve("", n); err == nil {
			m[n] = f
		}
	}
	return m
}

func (hg *HexGzip) Resolve(box string, name string) (file.File, error) {
	plog.Debug(hg, "Resolve", "box", box, "name", name)
	hg.moot.Lock()
	defer hg.moot.Unlock()

	if s, ok := hg.unpacked[name]; ok {
		return file.NewFile(name, []byte(s))
	}
	packed, ok := hg.packed[name]
	if !ok {
		return nil, os.ErrNotExist
	}

	unpacked, err := UnHexGzipString(packed)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	f, err := file.NewFile(OsPath(name), []byte(unpacked))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	hg.unpacked[name] = f.String()
	return f, nil
}

func NewHexGzip(files map[string]string) (*HexGzip, error) {
	if files == nil {
		files = map[string]string{}
	}

	hg := &HexGzip{
		packed:   files,
		unpacked: map[string]string{},
		moot:     &sync.RWMutex{},
	}

	return hg, nil
}

func HexGzipString(s string) (string, error) {
	bb := &bytes.Buffer{}
	enc := hex.NewEncoder(bb)
	zw := gzip.NewWriter(enc)
	io.Copy(zw, strings.NewReader(s))
	zw.Close()

	return bb.String(), nil
}

func UnHexGzipString(packed string) (string, error) {
	br := bytes.NewBufferString(packed)
	dec := hex.NewDecoder(br)
	zr, err := gzip.NewReader(dec)
	if err != nil {
		return "", errors.WithStack(err)
	}
	defer zr.Close()

	b, err := ioutil.ReadAll(zr)
	if err != nil {
		return "", errors.WithStack(err)
	}
	return string(b), nil
}
