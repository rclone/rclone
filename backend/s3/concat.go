package s3

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	"io"
	"io/ioutil"
	"path"
	"strconv"
	"time"
)

const uploadDir = ".%!%uploads%!%"    // TODO make configurable
const uploadLifetime = time.Hour * 24 // TODO make configurable

func (f *Fs) uploadFragment(ctx context.Context, fragment fs.Object, bucket, bucketPath string, partNumber int64, uploadID *string) (part *s3.CompletedPart, err error) {
	return f.uploadFragmentRange(ctx, fragment, bucket, bucketPath, partNumber, uploadID, 0, 0)
}

func (f *Fs) uploadFragmentRange(ctx context.Context, fragment fs.Object, bucket, bucketPath string, partNumber int64, uploadID *string, start, end uint64) (part *s3.CompletedPart, err error) {
	// TODO If fragment can be directly transferred between backends, stage the object in the .%!%uploads%!% folder on the target backend (use f.Copy())
	if operations.Same(fragment.Fs(), f) { // If fragment is on same backend, just pass a reference
		var sourceRange *string = nil
		if start != 0 || end != 0 {
			if start != 0 && end == 0 {
				end = uint64(fragment.Size()) - 1
			}
			tmp := fmt.Sprintf("bytes=%d-%d", start, end)
			sourceRange = &tmp
		}
		part, err := f.c.UploadPartCopy(&s3.UploadPartCopyInput{
			Bucket:          aws.String(bucket),
			CopySource:      aws.String(fragment.Remote()),
			CopySourceRange: sourceRange,
			Key:             aws.String(bucketPath),
			PartNumber:      aws.Int64(partNumber),
			UploadId:        uploadID,
		})
		if err != nil {
			return
		}
		return &s3.CompletedPart{ETag: part.CopyPartResult.ETag, PartNumber: aws.Int64(partNumber)}, nil
	} else { // else stream data from fragment backend
		body, err := fragment.Open(ctx)
		if err != nil {
			return
		}
		if start != 0 {
			_, err := io.CopyN(ioutil.Discard, body, int64(start))
			if err != nil {
				return
			}
		}
		stream := body.(io.Reader)
		if end != 0 {
			stream = io.LimitReader(body, int64(end-start))
		}
		part, err := f.c.UploadPart(&s3.UploadPartInput{
			Bucket:     aws.String(bucket),
			Body:       aws.ReadSeekCloser(stream),
			Key:        aws.String(bucketPath),
			PartNumber: aws.Int64(partNumber),
			UploadId:   uploadID,
		})
		err = body.Close()
		if err != nil {
			return
		}
		return &s3.CompletedPart{ETag: part.ETag, PartNumber: aws.Int64(partNumber)}, nil
	}
}

func (f *Fs) Concat(ctx context.Context, fragments fs.Objects, remote string) (result fs.Object, err error) {
	if len(fragments) == 0 {
		return nil, errors.New("no fragments to concatenate")
	}
	if len(fragments) == 1 {
		return fragments[0], nil
	}

	// Keep track of intermediate fragments
	tmpFragments := make(fs.Objects, len(fragments))
	tmpFragmentCount := uint64(0)
	defer func() { // defer runs LIFO and this will be executed after completing the upload below
		// Clean up temporary fragments
		for i := uint64(0); i < tmpFragmentCount; i++ {
			_ = tmpFragments[i].Remove(ctx)
		}
	}()

	if maxUploadParts > 0 {
		fragLen := uint64(len(fragments))
		if fragLen > maxUploadParts {
			// Handle more than maxUploadParts
			bucket, bucketPath, err := f.makeUploadBucket(ctx)
			if err != nil {
				return
			}
			for fragLen > maxUploadParts {
				manyFragments := fragments
				chunks := fragLen / maxUploadParts
				remainder := fragLen % maxUploadParts
				fragments = make(fs.Objects, chunks+remainder)
				for i := uint64(0); i < chunks; i++ {
					cat := path.Join(bucket, bucketPath, remote, "concat_"+strconv.FormatUint(i, 10))
					toMerge := manyFragments[i*maxUploadParts : (i+1)*maxUploadParts]
					fragments[i], err = f.Concat(ctx, toMerge, cat)
					tmpFragments[tmpFragmentCount] = fragments[i]
					tmpFragmentCount++
					if err != nil {
						return
					}
				}
				copy(fragments[chunks:], manyFragments[fragLen-remainder:])
				fragLen = uint64(len(fragments))
			}
		}
	}

	o := &Object{
		fs:     f,
		remote: remote,
	}
	result = o
	bucket, bucketPath := o.split()
	err = o.fs.makeBucket(ctx, bucket)
	if err != nil {
		return
	}

	expires := time.Now().Add(uploadLifetime)
	upload, err := f.c.CreateMultipartUpload(&s3.CreateMultipartUploadInput{
		Bucket:  aws.String(bucket),
		Key:     aws.String(bucketPath),
		Expires: &expires,
	})
	if err != nil {
		return
	}
	parts := make([]*s3.CompletedPart, len(fragments))
	partCount := int64(0)
	defer func() {
		// Complete or abort upload
		if err == nil && partCount > 0 {
			_, err = f.c.CompleteMultipartUpload(&s3.CompleteMultipartUploadInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(bucketPath),
				MultipartUpload: &s3.CompletedMultipartUpload{
					Parts: parts[:partCount],
				},
				UploadId: upload.UploadId,
			})
		} else {
			_, _ = f.c.AbortMultipartUpload(&s3.AbortMultipartUploadInput{
				Bucket:   aws.String(bucket),
				Key:      aws.String(bucketPath),
				UploadId: upload.UploadId,
			})
		}
	}()

	for i := 0; i < len(fragments); i++ {
		missing := int64(minChunkSize) - fragments[i].Size()
		if minChunkSize > 0 && missing > 0 {
			// Handle fragments less than minChunkSize
			if i == 0 { // if first part too small
				// Stream chunks until big enough
				readClosers := make([]io.ReadCloser, 0)
				for ; missing > 0 && i < len(fragments); i++ {
					readCloser, err := fragments[i].Open(ctx)
					if err != nil {
						break
					}
					if fragments[i].Size() > int64(minChunkSize)+missing { // should always be false for i == 0
						// steal range from next to avoid streaming entire next fragment
						readClosers = append(readClosers, io.LimitReader(readCloser, missing).(io.ReadCloser))

						parts[partCount+1], err = f.uploadFragmentRange(ctx, fragments[i], bucket, bucketPath, partCount+1, upload.UploadId, uint64(missing), 0)
						// partCount will be incremented below
						if err != nil {
							break
						}
					} else {
						readClosers = append(readClosers, readCloser)
					}
					missing -= fragments[i].Size()
				}

				if missing > 0 {
					err = errors.New("total concatenated size is smaller than minimum")
				}

				if err == nil {
					// Convert to []io.Reader
					readers := make([]io.Reader, len(readClosers)+1)
					for j, reader := range readClosers {
						readers[j] = reader
					}
					part, e := f.c.UploadPart(&s3.UploadPartInput{
						Bucket:     aws.String(bucket),
						Body:       aws.ReadSeekCloser(io.MultiReader(readers...)),
						Key:        aws.String(bucketPath),
						PartNumber: aws.Int64(partCount),
						UploadId:   upload.UploadId,
					})
					err = e
					if part != nil {
						parts[partCount] = &s3.CompletedPart{ETag: part.ETag, PartNumber: aws.Int64(partCount)}
						partCount++
						if parts[partCount] != nil {
							// Catch up partCount if a range from the next fragment was taken
							partCount++
						}
					}
				}

				// Close all streams
				for _, reader := range readClosers {
					e := reader.Close()
					if err != nil { // Continue closing but preserve error. Hopefully it is the same error if multiple.
						err = e
					}
				}
				if err != nil {
					return
				}
			} else {
				// Exploit last part no minimum
				cat := path.Join(bucket, bucketPath, remote, "merge_"+strconv.Itoa(i))
				toMerge := fs.Objects{fragments[i], fragments[i+1]}
				tmpFragment, err := f.Concat(ctx, toMerge, cat)
				if err != nil {
					return
				}
				tmpFragments[tmpFragmentCount] = tmpFragment
				tmpFragmentCount++
				part, err := f.c.UploadPartCopy(&s3.UploadPartCopyInput{
					Bucket:     aws.String(bucket),
					CopySource: aws.String(tmpFragment.Remote()),
					Key:        aws.String(bucketPath),
					PartNumber: aws.Int64(partCount),
					UploadId:   upload.UploadId,
				})
				if err != nil {
					return
				}
				parts[partCount] = &s3.CompletedPart{ETag: part.CopyPartResult.ETag, PartNumber: aws.Int64(partCount)}
				partCount++
				i++ // Two fragments consumed, increment an extra time
			}
		} else {
			parts[partCount], err = f.uploadFragment(ctx, fragments[i], bucket, bucketPath, int64(partCount), upload.UploadId)
			partCount++
			if err != nil {
				return
			}
		}
	}
	return
}
