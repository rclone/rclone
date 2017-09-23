# QingStor Image Processing Usage Guide

For processing the image stored in QingStor by a variety of basic operations, such as format, crop, watermark and so on.
Please see [QingStor Image API](https://docs.qingcloud.com/qingstor/data_process/image_process/index.html).

## Usage
Before using the image service, you need to initialize the [Configuration](https://github.com/yunify/qingstor-sdk-go/blob/master/docs/configuration.md) and [QingStor Service](https://github.com/yunify/qingstor-sdk-go/blob/master/docs/qingstor_service_usage.md).

``` go
//Import the latest version API
import (
	"github.com/yunify/qingstor-sdk-go/config"
	"github.com/yunify/qingstor-sdk-go/client/image"
	qs "github.com/yunify/qingstor-sdk-go/service"
)
```

## Code Snippet

Create configuration from Access Key and Initialize the QingStor service with a configuration.
``` go
// Initialize the QingStor service with a configuration
config, _ := config.New("ACCESS_KEY_ID", "SECRET_ACCESS_KEY")
service, _ := qs.Init(config)
```
Initialize a QingStor bucket.
``` go
bucket, _ := service.Bucket("bucketName", "zoneID")
```
Initialize a image.
``` go
img := image.Init(bucket, "imageName")
```

Now you can use the the high level APIs or basic image process API to do the image operation.

Get the information of the image
``` go
imageProcessOutput, _ := img.Info().Process()
```

Crop the image.
``` go
imageProcessOutput, _  := img.Crop(&image.CropParam{
	...operation_param...
}).Process()
```

Rotate the image.
``` go
imageProcessOutput, _ := img.Rotate(&image.RotateParam{
	...operation_param...
}).Process()
```
Resize the image.
``` go
imageProcessOutput, _ := img.Resize(&image.ResizeParam{
	...operation_param...
}).Process()
```
Watermark the image.
``` go
imageProcessOutput, _ := img.WaterMark(&image.WaterMarkParam{
	...operation_param...
}).Process()
```
WaterMarkImage the image.
``` go
imageProcessOutput, _ : = img.WaterMarkImage(&image.WaterMarkImageParam{
	...operation_param...
}).Process()
```
Format the image.
``` go
imageProcessOutput, _ := img.Format(&image.Format{
	...operation_param...
}).Process()
```
Operation pipline, the image will be processed by order. The maximum number of operations in the pipeline is 10.
``` go
// Rotate and then resize the image
imageProcessOutput, _ := img.Rotate(&image.RotateParam{
	... operation_param...
}).Resize(&image.ResizeParam{
	... operation_param...
}).Process()
```
Use the original basic API to rotate the image 90 angles.
``` go
operation := "rotate:a_90"
imageProcessOutput, err := bucket.ImageProcess("imageName", &qs.ImageProcessInput{
	Action: &operation})
```

`operation_param` is the image operation param, which definined in `qingstor-sdk-go/client/image/image.go`.
``` go
import "github.com/yunify/qingstor-sdk-go/service"
// client/image/image.go
type Image struct {
	key    *string
	bucket *service.Bucket
	input  *service.ImageProcessInput
}

// About cropping image definition
type CropGravity int
const (
	CropCenter CropGravity = iota
	CropNorth
	CropEast
	CropSouth
	CropWest
	CropNorthWest
	CropNorthEast
	CropSouthWest
	CropSouthEast
	CropAuto
)
type CropParam struct {
	Width   int         `schema:"w,omitempty"`
	Height  int         `schema:"h,omitempty"`
	Gravity CropGravity `schema:"g"`
}

// About rotating image definitions
type RotateParam struct {
	Angle int `schema:"a"`
}

// About resizing image definitions
type ResizeMode int
type ResizeParam struct {
	Width  int        `schema:"w,omitempty"`
	Height int        `schema:"h,omitempty"`
	Mode   ResizeMode `schema:"m"`
}

// On the definition of text watermarking
type WaterMarkParam struct {
	Dpi     int     `schema:"d,omitempty"`
	Opacity float64 `schema:"p,omitempty"`
	Text    string  `schema:"t"`
	Color   string  `schema:"c"`
}

// On the definition of image watermarking
 type WaterMarkImageParam struct {
	Left    int     `schema:"l"`
	Top     int     `schema:"t"`
	Opacity float64 `schema:"p,omitempty"`
	URL     string  `schema:"u"`
}

// About image format conversion definitions
type FormatParam struct {
	Type string `schema:"t"`
}

```

__Quick Start Code Example:__

Include a complete example, but the code needs to fill in your own information

``` go
package main

import (
	"log"

	"github.com/yunify/qingstor-sdk-go/client/image"
	"github.com/yunify/qingstor-sdk-go/config"
	qs "github.com/yunify/qingstor-sdk-go/service"
)

func main() {
	// Load your configuration
	// Replace here with your key pair
	config, err := config.New("ACCESS_KEY_ID", "SECRET_ACCESS_KEY")
	checkErr(err)

	// Initialize QingStror Service
	service, err := qs.Init(config)
	checkErr(err)

	// Initialize Bucket
	// Replace here with your bucketName and zoneID
	bucket, err := service.Bucket("bucketName", "zoneID")
	checkErr(err)

	// Initialize Image
	// Replace here with your your ImageName
	img := image.Init(bucket, "imageName")
	checkErr(err)

	// Because 0 is an invalid parameter, default not modify
	imageProcessOutput, err := img.Crop(&image.CropParam{Width: 0}).Process()
	checkErr(err)
	testOutput(imageProcessOutput)

	// Rotate the image 90 angles
	imageProcessOutput, err = img.Rotate(&image.RotateParam{Angle: 90}).Process()
	checkErr(err)
	testOutput(imageProcessOutput)

	// Text watermark, Watermark text content, encoded by base64.
	imageProcessOutput, err = img.WaterMark(&image.WaterMarkParam{
		Text: "5rC05Y2w5paH5a2X",
	}).Process()
	checkErr(err)
	testOutput(imageProcessOutput)

	// Image watermark, Watermark image url encoded by base64.
	imageProcessOutput, err = img.WaterMarkImage(&image.WaterMarkImageParam{
		URL: "aHR0cHM6Ly9wZWszYS5xaW5nc3Rvci5jb20vaW1nLWRvYy1lZy9xaW5jbG91ZC5wbmc",
	}).Process()
	checkErr(err)
	testOutput(imageProcessOutput)

	// Reszie the image with width 300px and height 400 px
	imageProcessOutput, err = img.Resize(&image.ResizeParam{
		Width:  300,
		Height: 400,
	}).Process()
	checkErr(err)
	testOutput(imageProcessOutput)

	// Swap format to jpeg
	imageProcessOutput, err = img.Format(&image.FormatParam{
		Type: "jpeg",
	}).Process()
	checkErr(err)
	testOutput(imageProcessOutput)

	// Pipline model
	// The maximum number of operations in the pipeline is 10
	imageProcessOutput, err = img.Rotate(&image.RotateParam{
		Angle: 270,
	}).Resize(&image.ResizeParam{
		Width:  300,
		Height: 300,
	}).Process()
	checkErr(err)
	testOutput(imageProcessOutput)

	// Get the information of the image
	imageProcessOutput, err = img.Info().Process()
	checkErr(err)
	testOutput(imageProcessOutput)

	// Use the original api to rotate the image 90 angles
	operation := "rotate:a_90"
	imageProcessOutput, err = bucket.ImageProcess("imageName", &qs.ImageProcessInput{
		Action: &operation})
	checkErr(err)
	testOutput(imageProcessOutput)
}

// *qs.ImageProcessOutput: github.com/yunify/qingstor-sdk-go/service/object.go
func testOutput(out *qs.ImageProcessOutput) {
	log.Println(*out.StatusCode)
	log.Println(*out.RequestID)
	log.Println(out.Body)
	log.Println(*out.ContentLength)
}

func checkErr(err error) {
	if err != nil {
		log.Println(err)
	}
}
```
