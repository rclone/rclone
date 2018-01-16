package upload

import (
	"errors"
	"github.com/yunify/qingstor-sdk-go/logger"
	"github.com/yunify/qingstor-sdk-go/service"
	"io"
)

// Uploader struct provides a struct to upload
type Uploader struct {
	bucket   *service.Bucket
	partSize int
}

const smallestPartSize int = 1024 * 1024 * 4

//Init creates a uploader struct
func Init(bucket *service.Bucket, partSize int) *Uploader {
	return &Uploader{
		bucket:   bucket,
		partSize: partSize,
	}
}

// Upload uploads multi parts of large object
func (u *Uploader) Upload(fd io.Reader, objectKey string) error {
	if u.partSize < smallestPartSize {
		logger.Errorf(nil, "Part size error")
		return errors.New("the part size is too small")
	}

	uploadID, err := u.init(objectKey)
	if err != nil {
		logger.Errorf(nil, "Init multipart upload error, %v.", err)
		return err
	}

	partNumbers, err := u.upload(fd, uploadID, objectKey)
	if err != nil {
		logger.Errorf(nil, "Upload multipart error, %v.", err)
		return err
	}

	err = u.complete(objectKey, uploadID, partNumbers)
	if err != nil {
		logger.Errorf(nil, "Complete upload error, %v.", err)
		return err
	}

	return nil
}

func (u *Uploader) init(objectKey string) (*string, error) {
	output, err := u.bucket.InitiateMultipartUpload(
		objectKey,
		&service.InitiateMultipartUploadInput{},
	)
	if err != nil {
		return nil, err
	}
	return output.UploadID, nil
}

func (u *Uploader) upload(fd io.Reader, uploadID *string, objectKey string) ([]*service.ObjectPartType, error) {
	var partCnt int
	partNumbers := []*service.ObjectPartType{}
	fileReader := newChunk(fd, u.partSize)
	for {
		partBody, err := fileReader.nextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			logger.Errorf(nil, "Get next part failed, %v", err)
			return nil, err
		}
		_, err = u.bucket.UploadMultipart(
			objectKey,
			&service.UploadMultipartInput{
				UploadID:   uploadID,
				PartNumber: &partCnt,
				Body:       partBody,
			},
		)
		if err != nil {
			logger.Errorf(nil, "Upload multipart failed, %v", err)
			return nil, err
		}
		partNumbers = append(partNumbers, &service.ObjectPartType{
			PartNumber: service.Int(partCnt - 0),
		})
		partCnt++
	}
	return partNumbers, nil
}

func (u *Uploader) complete(objectKey string, uploadID *string, partNumbers []*service.ObjectPartType) error {
	_, err := u.bucket.CompleteMultipartUpload(
		objectKey,
		&service.CompleteMultipartUploadInput{
			UploadID:    uploadID,
			ObjectParts: partNumbers,
		},
	)
	if err != nil {
		return err
	}
	return nil
}
