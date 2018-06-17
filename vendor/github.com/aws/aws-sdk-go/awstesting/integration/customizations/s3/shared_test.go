// +build integration

package s3

import (
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/awstesting/integration"
	"github.com/aws/aws-sdk-go/service/s3"
)

const integBucketPrefix = "aws-sdk-go-integration"

var bucketName *string
var svc *s3.S3

func TestMain(m *testing.M) {
	setup()
	defer teardown() // only called if we panic

	result := m.Run()
	teardown()
	os.Exit(result)
}

// Create a bucket for testing
func setup() {
	svc = s3.New(integration.Session)
	bucketName = aws.String(
		fmt.Sprintf("%s-%s",
			integBucketPrefix, integration.UniqueID()))

	_, err := svc.CreateBucket(&s3.CreateBucketInput{Bucket: bucketName})
	if err != nil {
		panic(fmt.Sprintf("failed to create bucket %s, %v", *bucketName, err))
	}

	err = svc.WaitUntilBucketExists(&s3.HeadBucketInput{Bucket: bucketName})
	if err != nil {
		panic(fmt.Sprintf("failed waiting for bucket %s to be created", *bucketName))
	}
}

// Delete the bucket
func teardown() {
	resp, err := svc.ListObjects(&s3.ListObjectsInput{Bucket: bucketName})
	if err != nil {
		panic(fmt.Sprintf("failed to list s3 bucket %s objects, %v", *bucketName, err))
	}

	errs := []error{}
	for _, o := range resp.Contents {
		_, err = svc.DeleteObject(&s3.DeleteObjectInput{Bucket: bucketName, Key: o.Key})
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		panic(fmt.Sprintf("failed to delete objects, %s", errs))
	}

	svc.DeleteBucket(&s3.DeleteBucketInput{Bucket: bucketName})
}

func putTestFile(t *testing.T, filename, key string) {
	f, err := os.Open(filename)
	if err != nil {
		t.Fatalf("failed to open testfile, %v", err)
	}
	defer f.Close()

	putTestContent(t, f, key)
}

func putTestContent(t *testing.T, reader io.ReadSeeker, key string) {
	_, err := svc.PutObject(&s3.PutObjectInput{
		Bucket: bucketName,
		Key:    aws.String(key),
		Body:   reader,
	})
	if err != nil {
		t.Errorf("expect no error, got %v", err)
	}
}
