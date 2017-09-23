package main

import (
	"errors"
	"os"
	"path"

	"github.com/DATA-DOG/godog"

	qs "github.com/yunify/qingstor-sdk-go/service"
)

// ImageFeatureContext provides feature context for image.
func ImageFeatureContext(s *godog.Suite) {
	s.Step(`^image process with key "([^"]*)" and query "([^"]*)"$`, imageProcessWithKeyAndQuery)
	s.Step(`^image process status code is (\d+)$`, imageProcessStatusCodeIs)

}

var imageName string

func imageProcessWithKeyAndQuery(objectKey, query string) error {
	if bucket == nil {
		return errors.New("The bucket is not exist")
	}
	file, err := os.Open(path.Join("features", "fixtures", objectKey))
	if err != nil {
		return err
	}
	defer file.Close()

	imageName = objectKey

	_, err = bucket.PutObject(imageName, &qs.PutObjectInput{Body: file})
	if err != nil {
		return err
	}

	output, err := bucket.ImageProcess(objectKey, &qs.ImageProcessInput{
		Action: &query})
	if err != nil {
		return err
	}
	imageProcessOutput = output
	return nil
}

var imageProcessOutput *qs.ImageProcessOutput

func imageProcessStatusCodeIs(statusCode int) error {
	defer deleteImage(imageName)
	return checkEqual(qs.IntValue(imageProcessOutput.StatusCode), statusCode)
}

var oOutput *qs.DeleteObjectOutput

func deleteImage(imageName string) error {

	if bucket == nil {
		return errors.New("The bucket is not exist")
	}

	oOutput, err = bucket.DeleteObject(imageName)
	if err != nil {
		return err
	}
	return checkEqual(qs.IntValue(oOutput.StatusCode), 204)
}
