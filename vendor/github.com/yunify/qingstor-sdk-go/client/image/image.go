package image

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/yunify/qingstor-sdk-go/service"
)

const (
	// ActionSep is separator of action.
	ActionSep = ":"

	// OPSep is separator of operation.
	OPSep = "|"

	// KVSep is separator of Key-Value.
	KVSep = "_"

	//KVPairSep is separator of args.
	KVPairSep = ","
)

const (
	// InfoOperation is string of info operation.
	InfoOperation string = "info"

	// CropOperation is string of crop operation.
	CropOperation string = "crop"

	// FormatOperation is string of format operation.
	FormatOperation string = "format"

	// ResizeOperation is string of resize operation.
	ResizeOperation string = "resize"

	// RotateOperation is string of rotate operation.
	RotateOperation string = "rotate"

	// WaterMarkOperation is string of watermark operation.
	WaterMarkOperation string = "watermark"

	// WaterMarkImageOperation is string of watermark image operation.
	WaterMarkImageOperation string = "watermark_image"
)

// ResizeMode is the type of resize mode.
type ResizeMode int

const (
	// ResizeFixed resizes image to fix  width and height.
	ResizeFixed ResizeMode = iota

	// ResizeForce resizes image to force witdth and height.
	ResizeForce

	// ResizeThumbnail resizes image to thumbnail width and height.
	ResizeThumbnail
)

// CropGravity is the type of crop gravity.
type CropGravity int

const (

	// CropCenter crops image to center width and height.
	CropCenter CropGravity = iota

	// CropNorth crops image to north width and height.
	CropNorth

	// CropEast crops image to east width and height.
	CropEast

	// CropSouth crops image to south width and height.
	CropSouth

	// CropWest crops image to west width and height.
	CropWest

	// CropNorthWest crops image to north west width and height.
	CropNorthWest

	// CropNorthEast crops image to north east width and height.
	CropNorthEast

	// CropSouthWest crops image to south west width and height.
	CropSouthWest

	// CropSouthEast crops image to south east width and height.
	CropSouthEast

	// CropAuto crops image to auto width and height.
	CropAuto
)

// Image is struct of Image process.
type Image struct {
	key    *string
	bucket *service.Bucket
	input  *service.ImageProcessInput
}

// Init initializes an image to process.
func Init(bucket *service.Bucket, objectKey string) *Image {
	return &Image{
		key:    &objectKey,
		bucket: bucket,
		input:  &service.ImageProcessInput{},
	}
}

// Info gets the information of the image.
func (image *Image) Info() *Image {
	return image.setActionParam(InfoOperation, nil)
}

// RotateParam is param of the rotate operation.
type RotateParam struct {
	Angle int `schema:"a"`
}

// Rotate image.
func (image *Image) Rotate(param *RotateParam) *Image {
	return image.setActionParam(RotateOperation, param)
}

// ResizeParam is param of the resize operation.
type ResizeParam struct {
	Width  int        `schema:"w,omitempty"`
	Height int        `schema:"h,omitempty"`
	Mode   ResizeMode `schema:"m"`
}

// Resize image.
func (image *Image) Resize(param *ResizeParam) *Image {
	return image.setActionParam(ResizeOperation, param)
}

// CropParam is param of the crop operation.
type CropParam struct {
	Width   int         `schema:"w,omitempty"`
	Height  int         `schema:"h,omitempty"`
	Gravity CropGravity `schema:"g"`
}

// Crop image.
func (image *Image) Crop(param *CropParam) *Image {
	return image.setActionParam(CropOperation, param)
}

// FormatParam is param of the format operation.
type FormatParam struct {
	Type string `schema:"t"`
}

// Format image.
func (image *Image) Format(param *FormatParam) *Image {
	return image.setActionParam(FormatOperation, param)
}

// WaterMarkParam is param of the wartermark operation.
type WaterMarkParam struct {
	Dpi     int     `schema:"d,omitempty"`
	Opacity float64 `schema:"p,omitempty"`
	Text    string  `schema:"t"`
	Color   string  `schema:"c"`
}

// WaterMark is operation of watermark text content.
func (image *Image) WaterMark(param *WaterMarkParam) *Image {
	return image.setActionParam(WaterMarkOperation, param)
}

// WaterMarkImageParam is param of the  waterMark image operation
type WaterMarkImageParam struct {
	Left    int     `schema:"l"`
	Top     int     `schema:"t"`
	Opacity float64 `schema:"p,omitempty"`
	URL     string  `schema:"u"`
}

// WaterMarkImage is operation of watermark image.
func (image *Image) WaterMarkImage(param *WaterMarkImageParam) *Image {
	return image.setActionParam(WaterMarkImageOperation, param)
}

// Process does Image process.
func (image *Image) Process() (*service.ImageProcessOutput, error) {
	defer func(input *service.ImageProcessInput) {
		input.Action = nil
	}(image.input)
	return image.bucket.ImageProcess(*image.key, image.input)
}

func (image *Image) setActionParam(operation string, param interface{}) *Image {
	uri := operation
	if param != nil {
		uri = fmt.Sprintf("%s%s%s", uri, ActionSep, buildOptParamStr(param))
	}
	if image.input.Action != nil {
		uri = fmt.Sprintf("%s%s%s", *image.input.Action, OPSep, uri)
	}
	image.input.Action = &uri
	return image
}

func buildOptParamStr(param interface{}) string {
	v := reflect.ValueOf(param).Elem()
	var kvPairs []string

	for i := 0; i < v.NumField(); i++ {
		vf := v.Field(i)
		tf := v.Type().Field(i)
		key := tf.Tag.Get("schema")
		value := vf.Interface()
		tagValues := strings.Split(key, ",")
		if isEmptyValue(vf) &&
			len(tagValues) == 2 &&
			tagValues[1] == "omitempty" {
			continue
		}
		key = tagValues[0]
		kvPairs = append(kvPairs, fmt.Sprintf("%v%s%v", key, KVSep, value))
	}
	return strings.Join(kvPairs, KVPairSep)
}

func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array,
		reflect.Map,
		reflect.Slice,
		reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:
		return v.Int() == 0
	case reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32,
		reflect.Float64:
		return v.Float() == 0
	case reflect.Ptr:
		return v.IsNil()
	}
	return false
}
