package main

import (
	"os"
	"os/exec"
	"errors"

	"github.com/DATA-DOG/godog"
	"github.com/yunify/qingstor-sdk-go/client/upload"
)

var uploader *upload.Uploader

// UploadFeatureContext provides feature context for upload.
func UploadFeatureContext(s *godog.Suite) {
	s.Step("initialize uploader$", initializeUploader)
	s.Step("uploader is initialized$", uploaderIsInitialized)

	s.Step("upload a large file$", uploadLargeFile)
	s.Step("the large file is uploaded$", largeFileIsUploaded)
}

var fd *os.File

func initializeUploader() error {
	uploadSetup()
	PartSize := 4 * 1024 * 1024

	fd, err = os.Open("test_file")
	if err != nil {
		return err
	}

	uploader = upload.Init(bucket, PartSize)
	return nil
}

func uploaderIsInitialized() error {
	if uploader == nil {
		return errors.New("uploader not initialized")
	}
	return nil
}

var objectKey string

func uploadLargeFile() error {
	objectKey = "test_multipart_upload"
	err := uploader.Upload(fd, objectKey)
	if err != nil {
		return err
	}
	return nil
}

func largeFileIsUploaded() error {
	defer uploadTearDown()
	return nil
}

func uploadSetup() {
	exec.Command("dd", "if=/dev/zero", "of=test_file", "bs=1024", "count=20480").Output()
}

func uploadTearDown() {
	exec.Command("rm", "", "test_file").Output()
}
