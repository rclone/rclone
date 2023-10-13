package nfs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"io/fs"
	"os"
	"sort"

	"github.com/willscott/go-nfs-client/nfs/xdr"
)

type readDirArgs struct {
	Handle      []byte
	Cookie      uint64
	CookieVerif uint64
	Count       uint32
}

type readDirEntity struct {
	FileID uint64
	Name   []byte
	Cookie uint64
	Next   bool
}

func onReadDir(ctx context.Context, w *response, userHandle Handler) error {
	w.errorFmt = opAttrErrorFormatter
	obj := readDirArgs{}
	err := xdr.Read(w.req.Body, &obj)
	if err != nil {
		return &NFSStatusError{NFSStatusInval, err}
	}

	if obj.Count < 1024 {
		return &NFSStatusError{NFSStatusTooSmall, io.ErrShortBuffer}
	}

	fs, p, err := userHandle.FromHandle(obj.Handle)
	if err != nil {
		return &NFSStatusError{NFSStatusStale, err}
	}

	contents, verifier, err := getDirListingWithVerifier(userHandle, obj.Handle, obj.CookieVerif)
	if err != nil {
		return err
	}
	if obj.Cookie > 0 && obj.CookieVerif > 0 && verifier != obj.CookieVerif {
		return &NFSStatusError{NFSStatusBadCookie, nil}
	}

	entities := make([]readDirEntity, 0)
	maxBytes := uint32(100) // conservative overhead measure

	started := obj.Cookie == 0
	if started {
		// add '.' and '..' to entities
		dotdotFileID := uint64(0)
		if len(p) > 0 {
			ph := userHandle.ToHandle(fs, p[0:len(p)-1])
			dotdotFileID = binary.BigEndian.Uint64(ph[0:8])
		}
		entities = append(entities,
			readDirEntity{Name: []byte("."), Cookie: 0, Next: true, FileID: binary.BigEndian.Uint64(obj.Handle[0:8])},
			readDirEntity{Name: []byte(".."), Cookie: 1, Next: true, FileID: dotdotFileID},
		)
	}

	eof := true
	maxEntities := userHandle.HandleLimit() / 2
	for i, c := range contents {
		// cookie equates to index within contents + 2 (for '.' and '..')
		cookie := uint64(i + 2)
		if started {
			maxBytes += 512 // TODO: better estimation.
			if maxBytes > obj.Count || len(entities) > maxEntities {
				eof = false
				break
			}

			entities = append(entities, readDirEntity{
				FileID: 1337, // todo: does this matter?
				Name:   []byte(c.Name()),
				Cookie: cookie,
				Next:   true,
			})
		} else if cookie == obj.Cookie {
			started = true
		}
	}

	writer := bytes.NewBuffer([]byte{})
	if err := xdr.Write(writer, uint32(NFSStatusOk)); err != nil {
		return &NFSStatusError{NFSStatusServerFault, err}
	}
	if err := WritePostOpAttrs(writer, tryStat(fs, p)); err != nil {
		return &NFSStatusError{NFSStatusServerFault, err}
	}

	if err := xdr.Write(writer, verifier); err != nil {
		return &NFSStatusError{NFSStatusServerFault, err}
	}

	if err := xdr.Write(writer, len(entities) > 0); err != nil { // next
		return &NFSStatusError{NFSStatusServerFault, err}
	}
	if len(entities) > 0 {
		entities[len(entities)-1].Next = false
		// no next for last entity

		for _, e := range entities {
			if err := xdr.Write(writer, e); err != nil {
				return &NFSStatusError{NFSStatusServerFault, err}
			}
		}
	}
	if err := xdr.Write(writer, eof); err != nil {
		return &NFSStatusError{NFSStatusServerFault, err}
	}
	// TODO: track writer size at this point to validate maxcount estimation and stop early if needed.

	if err := w.Write(writer.Bytes()); err != nil {
		return &NFSStatusError{NFSStatusServerFault, err}
	}
	return nil
}

func getDirListingWithVerifier(userHandle Handler, fsHandle []byte, verifier uint64) ([]fs.FileInfo, uint64, error) {
	// figure out what directory it is.
	fs, p, err := userHandle.FromHandle(fsHandle)
	if err != nil {
		return nil, 0, &NFSStatusError{NFSStatusStale, err}
	}

	path := fs.Join(p...)
	// see if the verifier has this dir cached:
	if vh, ok := userHandle.(CachingHandler); verifier != 0 && ok {
		entries := vh.DataForVerifier(path, verifier)
		if entries != nil {
			return entries, verifier, nil
		}
	}
	// load the entries.
	contents, err := fs.ReadDir(path)
	if err != nil {
		if os.IsPermission(err) {
			return nil, 0, &NFSStatusError{NFSStatusAccess, err}
		}
		return nil, 0, &NFSStatusError{NFSStatusNotDir, err}
	}

	sort.Slice(contents, func(i, j int) bool {
		return contents[i].Name() < contents[j].Name()
	})

	if vh, ok := userHandle.(CachingHandler); ok {
		// let the user handler make a verifier if it can.
		v := vh.VerifierFor(path, contents)
		return contents, v, nil
	}

	id := hashPathAndContents(path, contents)
	return contents, id, nil
}

func hashPathAndContents(path string, contents []fs.FileInfo) uint64 {
	//calculate a cookie-verifier.
	vHash := sha256.New()

	// Add the path to avoid collisions of directories with the same content
	vHash.Write([]byte(path))

	for _, c := range contents {
		vHash.Write([]byte(c.Name())) // Never fails according to the docs
	}

	verify := vHash.Sum(nil)[0:8]
	return binary.BigEndian.Uint64(verify)
}
