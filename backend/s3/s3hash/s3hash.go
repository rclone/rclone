// Package s3hash implements the AWS muliparted hash, which is a modified MD5.
// See https://stackoverflow.com/questions/12186993/what-is-the-algorithm-to-compute-the-amazon-s3-etag-for-a-file-larger-than-5gb
//
// Say you uploaded a 14MB file to a bucket without server-side encryption, and your part size is 5MB.
// Calculate 3 MD5 checksums corresponding to each part, i.e. the checksum of the first 5MB,
// the second 5MB, and the last 4MB. Then take the checksum of their concatenation.
// MD5 checksums are often printed as hex representations of binary data, so make sure you take the MD5
// of the decoded binary concatenation, not of the ASCII or UTF-8 encoded concatenation.
// When that's done, add a hyphen and the number of parts to get the ETag.
//
// For example, a multipart Etag can be built with the code below.
// 	    hs := s3hash.New(8 * Mi)
// 	    for {
// 	        hs.Write(data)
//      }
//      Etag := hex.EncodeToString(hs.Sum(nil)) + "-" + strconv.Itoa(hs.GetPartsCount())
//      // result be like — 60b725f10c9c85c70d97880dfe8191b3-33
package s3hash

import (
	"crypto/md5"
	"encoding"
	"hash"
)

// S3Hash multipart hash
// MD5 is calculated for each chunk. The final MD5 calculates from MD5s of chunks.
type S3Hash struct {
	partSizeHashed int       // bytes (of single part) written into hash
	partsCount     int       // number hashed parts
	partSize       int       // configured part size
	digest         hash.Hash // underlying MD5
	finalDigest    hash.Hash // underlying MD5 of MD5 hashes
}

// New creates new S3Hash hash
func New(partSize int) hash.Hash {
	return &S3Hash{
		partSize:    partSize,
		digest:      md5.New(),
		finalDigest: nil, // lazy factory in the final()
	}
}

// GetPartsCount returns number hashed parts
func (s *S3Hash) GetPartsCount() int {
	if s.partSizeHashed == 0 {
		return s.partsCount
	}
	return s.partsCount + 1
}

func (s *S3Hash) final() hash.Hash {
	if s.finalDigest == nil {
		s.finalDigest = md5.New()
	}
	return s.finalDigest
}

// Write writes len(p) bytes from p to the underlying data stream. It returns
// the number of bytes written from p (0 <= n <= len(p)) and any error
// encountered that caused the write to stop early. Write returns a non-nil
// error if it returns n < len(p). Write doesn't modify the slice data, even
// temporarily.
func (s *S3Hash) Write(p []byte) (n int, err error) {
	if s.partSize == 0 {
		// if parts disabled behave like a normal md5
		return s.digest.Write(p)
	}
	if s.partSizeHashed+len(p) < s.partSize { // write to primary digest
		n, err = s.digest.Write(p)
		s.partSizeHashed += n
	} else if s.partSizeHashed+len(p) > s.partSize {
		// We have to write primary digest to final digest and recreate primary digest each part.
		// Read p by parts and do some stuff.
		var p2 = p
		for p2 != nil {
			if len(p2) > s.partSize-s.partSizeHashed {
				k, _ := s.digest.Write(p2[:s.partSize-s.partSizeHashed])
				n += k
				s.partSizeHashed += k
				p2 = p2[k:]
			} else {
				k, _ := s.digest.Write(p2)
				s.partSizeHashed += k
				n += k
				p2 = nil
			}
			if s.partSizeHashed == s.partSize {
				s.final().Write(s.digest.Sum(nil))
				s.partsCount++
				s.digest.Reset()
				s.partSizeHashed = 0
			}
		}
	} else { // s.partSizeHashed+len(p) == s.partSize
		// write to primary digest, primary digest write to final digest and recreate primary digest
		n, _ = s.digest.Write(p)
		s.final().Write(s.digest.Sum(nil))
		s.digest.Reset()
		s.partSizeHashed = 0
		s.partsCount++
	}
	return
}

// Sum appends the current hash to b and returns the resulting slice.
// It does not change the underlying hash state.
func (s *S3Hash) Sum(b []byte) []byte {
	if s.partSize == 0 || s.partsCount == 0 {
		return s.digest.Sum(b)
	} else if s.partSizeHashed == 0 {
		return s.final().Sum(b)
	} else {
		// We keep internal digest state because new data may be available.
		// By this reason we clone hash via BinaryMarshaler and BinaryUnmarshaler.
		cp, _ := s.final().(encoding.BinaryMarshaler).MarshalBinary()
		finalDigest := md5.New()
		err := finalDigest.(encoding.BinaryUnmarshaler).UnmarshalBinary(cp)
		if err != nil {
			panic("unable to unmarshal final S3Hash: " + err.Error())
		}
		finalDigest.Write(s.digest.Sum(nil))
		return finalDigest.Sum(b)
	}
}

// Reset resets the Hash to its initial state.
func (s *S3Hash) Reset() {
	s.finalDigest = nil
	s.digest.Reset()
	s.partSizeHashed = 0
	s.partsCount = 0
}

// Size returns the number of bytes Sum will return.
func (s *S3Hash) Size() int {
	return md5.Size
}

// BlockSize returns the hash's underlying block size.
// The Write method must be able to accept any amount
// of data, but it may operate more efficiently if all writes
// are a multiple of the block size.
func (s S3Hash) BlockSize() int {
	return md5.BlockSize
}
