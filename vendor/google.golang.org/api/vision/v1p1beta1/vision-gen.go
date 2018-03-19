// Package vision provides access to the Google Cloud Vision API.
//
// See https://cloud.google.com/vision/
//
// Usage example:
//
//   import "google.golang.org/api/vision/v1p1beta1"
//   ...
//   visionService, err := vision.New(oauthHttpClient)
package vision // import "google.golang.org/api/vision/v1p1beta1"

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	context "golang.org/x/net/context"
	ctxhttp "golang.org/x/net/context/ctxhttp"
	gensupport "google.golang.org/api/gensupport"
	googleapi "google.golang.org/api/googleapi"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Always reference these packages, just in case the auto-generated code
// below doesn't.
var _ = bytes.NewBuffer
var _ = strconv.Itoa
var _ = fmt.Sprintf
var _ = json.NewDecoder
var _ = io.Copy
var _ = url.Parse
var _ = gensupport.MarshalJSON
var _ = googleapi.Version
var _ = errors.New
var _ = strings.Replace
var _ = context.Canceled
var _ = ctxhttp.Do

const apiId = "vision:v1p1beta1"
const apiName = "vision"
const apiVersion = "v1p1beta1"
const basePath = "https://vision.googleapis.com/"

// OAuth2 scopes used by this API.
const (
	// View and manage your data across Google Cloud Platform services
	CloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

	// Apply machine learning models to understand and label images
	CloudVisionScope = "https://www.googleapis.com/auth/cloud-vision"
)

func New(client *http.Client) (*Service, error) {
	if client == nil {
		return nil, errors.New("client is nil")
	}
	s := &Service{client: client, BasePath: basePath}
	s.Images = NewImagesService(s)
	return s, nil
}

type Service struct {
	client    *http.Client
	BasePath  string // API endpoint base URL
	UserAgent string // optional additional User-Agent fragment

	Images *ImagesService
}

func (s *Service) userAgent() string {
	if s.UserAgent == "" {
		return googleapi.UserAgent
	}
	return googleapi.UserAgent + " " + s.UserAgent
}

func NewImagesService(s *Service) *ImagesService {
	rs := &ImagesService{s: s}
	return rs
}

type ImagesService struct {
	s *Service
}

// Color: Represents a color in the RGBA color space. This
// representation is designed
// for simplicity of conversion to/from color representations in
// various
// languages over compactness; for example, the fields of this
// representation
// can be trivially provided to the constructor of "java.awt.Color" in
// Java; it
// can also be trivially provided to UIColor's
// "+colorWithRed:green:blue:alpha"
// method in iOS; and, with just a little work, it can be easily
// formatted into
// a CSS "rgba()" string in JavaScript, as well. Here are some
// examples:
//
// Example (Java):
//
//      import com.google.type.Color;
//
//      // ...
//      public static java.awt.Color fromProto(Color protocolor) {
//        float alpha = protocolor.hasAlpha()
//            ? protocolor.getAlpha().getValue()
//            : 1.0;
//
//        return new java.awt.Color(
//            protocolor.getRed(),
//            protocolor.getGreen(),
//            protocolor.getBlue(),
//            alpha);
//      }
//
//      public static Color toProto(java.awt.Color color) {
//        float red = (float) color.getRed();
//        float green = (float) color.getGreen();
//        float blue = (float) color.getBlue();
//        float denominator = 255.0;
//        Color.Builder resultBuilder =
//            Color
//                .newBuilder()
//                .setRed(red / denominator)
//                .setGreen(green / denominator)
//                .setBlue(blue / denominator);
//        int alpha = color.getAlpha();
//        if (alpha != 255) {
//          result.setAlpha(
//              FloatValue
//                  .newBuilder()
//                  .setValue(((float) alpha) / denominator)
//                  .build());
//        }
//        return resultBuilder.build();
//      }
//      // ...
//
// Example (iOS / Obj-C):
//
//      // ...
//      static UIColor* fromProto(Color* protocolor) {
//         float red = [protocolor red];
//         float green = [protocolor green];
//         float blue = [protocolor blue];
//         FloatValue* alpha_wrapper = [protocolor alpha];
//         float alpha = 1.0;
//         if (alpha_wrapper != nil) {
//           alpha = [alpha_wrapper value];
//         }
//         return [UIColor colorWithRed:red green:green blue:blue
// alpha:alpha];
//      }
//
//      static Color* toProto(UIColor* color) {
//          CGFloat red, green, blue, alpha;
//          if (![color getRed:&red green:&green blue:&blue
// alpha:&alpha]) {
//            return nil;
//          }
//          Color* result = [Color alloc] init];
//          [result setRed:red];
//          [result setGreen:green];
//          [result setBlue:blue];
//          if (alpha <= 0.9999) {
//            [result setAlpha:floatWrapperWithValue(alpha)];
//          }
//          [result autorelease];
//          return result;
//     }
//     // ...
//
//  Example (JavaScript):
//
//     // ...
//
//     var protoToCssColor = function(rgb_color) {
//        var redFrac = rgb_color.red || 0.0;
//        var greenFrac = rgb_color.green || 0.0;
//        var blueFrac = rgb_color.blue || 0.0;
//        var red = Math.floor(redFrac * 255);
//        var green = Math.floor(greenFrac * 255);
//        var blue = Math.floor(blueFrac * 255);
//
//        if (!('alpha' in rgb_color)) {
//           return rgbToCssColor_(red, green, blue);
//        }
//
//        var alphaFrac = rgb_color.alpha.value || 0.0;
//        var rgbParams = [red, green, blue].join(',');
//        return ['rgba(', rgbParams, ',', alphaFrac, ')'].join('');
//     };
//
//     var rgbToCssColor_ = function(red, green, blue) {
//       var rgbNumber = new Number((red << 16) | (green << 8) | blue);
//       var hexString = rgbNumber.toString(16);
//       var missingZeros = 6 - hexString.length;
//       var resultBuilder = ['#'];
//       for (var i = 0; i < missingZeros; i++) {
//          resultBuilder.push('0');
//       }
//       resultBuilder.push(hexString);
//       return resultBuilder.join('');
//     };
//
//     // ...
type Color struct {
	// Alpha: The fraction of this color that should be applied to the
	// pixel. That is,
	// the final pixel color is defined by the equation:
	//
	//   pixel color = alpha * (this color) + (1.0 - alpha) * (background
	// color)
	//
	// This means that a value of 1.0 corresponds to a solid color,
	// whereas
	// a value of 0.0 corresponds to a completely transparent color.
	// This
	// uses a wrapper message rather than a simple float scalar so that it
	// is
	// possible to distinguish between a default value and the value being
	// unset.
	// If omitted, this color object is to be rendered as a solid color
	// (as if the alpha value had been explicitly given with a value of
	// 1.0).
	Alpha float64 `json:"alpha,omitempty"`

	// Blue: The amount of blue in the color as a value in the interval [0,
	// 1].
	Blue float64 `json:"blue,omitempty"`

	// Green: The amount of green in the color as a value in the interval
	// [0, 1].
	Green float64 `json:"green,omitempty"`

	// Red: The amount of red in the color as a value in the interval [0,
	// 1].
	Red float64 `json:"red,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Alpha") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Alpha") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *Color) MarshalJSON() ([]byte, error) {
	type NoMethod Color
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

func (s *Color) UnmarshalJSON(data []byte) error {
	type NoMethod Color
	var s1 struct {
		Alpha gensupport.JSONFloat64 `json:"alpha"`
		Blue  gensupport.JSONFloat64 `json:"blue"`
		Green gensupport.JSONFloat64 `json:"green"`
		Red   gensupport.JSONFloat64 `json:"red"`
		*NoMethod
	}
	s1.NoMethod = (*NoMethod)(s)
	if err := json.Unmarshal(data, &s1); err != nil {
		return err
	}
	s.Alpha = float64(s1.Alpha)
	s.Blue = float64(s1.Blue)
	s.Green = float64(s1.Green)
	s.Red = float64(s1.Red)
	return nil
}

// GoogleCloudVisionV1p1beta1AnnotateImageRequest: Request for
// performing Google Cloud Vision API tasks over a user-provided
// image, with user-requested features.
type GoogleCloudVisionV1p1beta1AnnotateImageRequest struct {
	// Features: Requested features.
	Features []*GoogleCloudVisionV1p1beta1Feature `json:"features,omitempty"`

	// Image: The image to be processed.
	Image *GoogleCloudVisionV1p1beta1Image `json:"image,omitempty"`

	// ImageContext: Additional context that may accompany the image.
	ImageContext *GoogleCloudVisionV1p1beta1ImageContext `json:"imageContext,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Features") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Features") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1AnnotateImageRequest) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1AnnotateImageRequest
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GoogleCloudVisionV1p1beta1AnnotateImageResponse: Response to an image
// annotation request.
type GoogleCloudVisionV1p1beta1AnnotateImageResponse struct {
	// CropHintsAnnotation: If present, crop hints have completed
	// successfully.
	CropHintsAnnotation *GoogleCloudVisionV1p1beta1CropHintsAnnotation `json:"cropHintsAnnotation,omitempty"`

	// Error: If set, represents the error message for the operation.
	// Note that filled-in image annotations are guaranteed to be
	// correct, even when `error` is set.
	Error *Status `json:"error,omitempty"`

	// FaceAnnotations: If present, face detection has completed
	// successfully.
	FaceAnnotations []*GoogleCloudVisionV1p1beta1FaceAnnotation `json:"faceAnnotations,omitempty"`

	// FullTextAnnotation: If present, text (OCR) detection or document
	// (OCR) text detection has
	// completed successfully.
	// This annotation provides the structural hierarchy for the OCR
	// detected
	// text.
	FullTextAnnotation *GoogleCloudVisionV1p1beta1TextAnnotation `json:"fullTextAnnotation,omitempty"`

	// ImagePropertiesAnnotation: If present, image properties were
	// extracted successfully.
	ImagePropertiesAnnotation *GoogleCloudVisionV1p1beta1ImageProperties `json:"imagePropertiesAnnotation,omitempty"`

	// LabelAnnotations: If present, label detection has completed
	// successfully.
	LabelAnnotations []*GoogleCloudVisionV1p1beta1EntityAnnotation `json:"labelAnnotations,omitempty"`

	// LandmarkAnnotations: If present, landmark detection has completed
	// successfully.
	LandmarkAnnotations []*GoogleCloudVisionV1p1beta1EntityAnnotation `json:"landmarkAnnotations,omitempty"`

	// LogoAnnotations: If present, logo detection has completed
	// successfully.
	LogoAnnotations []*GoogleCloudVisionV1p1beta1EntityAnnotation `json:"logoAnnotations,omitempty"`

	// SafeSearchAnnotation: If present, safe-search annotation has
	// completed successfully.
	SafeSearchAnnotation *GoogleCloudVisionV1p1beta1SafeSearchAnnotation `json:"safeSearchAnnotation,omitempty"`

	// TextAnnotations: If present, text (OCR) detection has completed
	// successfully.
	TextAnnotations []*GoogleCloudVisionV1p1beta1EntityAnnotation `json:"textAnnotations,omitempty"`

	// WebDetection: If present, web detection has completed successfully.
	WebDetection *GoogleCloudVisionV1p1beta1WebDetection `json:"webDetection,omitempty"`

	// ForceSendFields is a list of field names (e.g. "CropHintsAnnotation")
	// to unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "CropHintsAnnotation") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1AnnotateImageResponse) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1AnnotateImageResponse
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GoogleCloudVisionV1p1beta1BatchAnnotateImagesRequest: Multiple image
// annotation requests are batched into a single service call.
type GoogleCloudVisionV1p1beta1BatchAnnotateImagesRequest struct {
	// Requests: Individual image annotation requests for this batch.
	Requests []*GoogleCloudVisionV1p1beta1AnnotateImageRequest `json:"requests,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Requests") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Requests") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1BatchAnnotateImagesRequest) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1BatchAnnotateImagesRequest
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GoogleCloudVisionV1p1beta1BatchAnnotateImagesResponse: Response to a
// batch image annotation request.
type GoogleCloudVisionV1p1beta1BatchAnnotateImagesResponse struct {
	// Responses: Individual responses to image annotation requests within
	// the batch.
	Responses []*GoogleCloudVisionV1p1beta1AnnotateImageResponse `json:"responses,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Responses") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Responses") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1BatchAnnotateImagesResponse) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1BatchAnnotateImagesResponse
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GoogleCloudVisionV1p1beta1Block: Logical element on the page.
type GoogleCloudVisionV1p1beta1Block struct {
	// BlockType: Detected block type (text, image etc) for this block.
	//
	// Possible values:
	//   "UNKNOWN" - Unknown block type.
	//   "TEXT" - Regular text block.
	//   "TABLE" - Table block.
	//   "PICTURE" - Image block.
	//   "RULER" - Horizontal/vertical line box.
	//   "BARCODE" - Barcode block.
	BlockType string `json:"blockType,omitempty"`

	// BoundingBox: The bounding box for the block.
	// The vertices are in the order of top-left, top-right,
	// bottom-right,
	// bottom-left. When a rotation of the bounding box is detected the
	// rotation
	// is represented as around the top-left corner as defined when the text
	// is
	// read in the 'natural' orientation.
	// For example:
	//
	// * when the text is horizontal it might look like:
	//
	//         0----1
	//         |    |
	//         3----2
	//
	// * when it's rotated 180 degrees around the top-left corner it
	// becomes:
	//
	//         2----3
	//         |    |
	//         1----0
	//
	//   and the vertice order will still be (0, 1, 2, 3).
	BoundingBox *GoogleCloudVisionV1p1beta1BoundingPoly `json:"boundingBox,omitempty"`

	// Confidence: Confidence of the OCR results on the block. Range [0, 1].
	Confidence float64 `json:"confidence,omitempty"`

	// Paragraphs: List of paragraphs in this block (if this blocks is of
	// type text).
	Paragraphs []*GoogleCloudVisionV1p1beta1Paragraph `json:"paragraphs,omitempty"`

	// Property: Additional information detected for the block.
	Property *GoogleCloudVisionV1p1beta1TextAnnotationTextProperty `json:"property,omitempty"`

	// ForceSendFields is a list of field names (e.g. "BlockType") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "BlockType") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1Block) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1Block
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

func (s *GoogleCloudVisionV1p1beta1Block) UnmarshalJSON(data []byte) error {
	type NoMethod GoogleCloudVisionV1p1beta1Block
	var s1 struct {
		Confidence gensupport.JSONFloat64 `json:"confidence"`
		*NoMethod
	}
	s1.NoMethod = (*NoMethod)(s)
	if err := json.Unmarshal(data, &s1); err != nil {
		return err
	}
	s.Confidence = float64(s1.Confidence)
	return nil
}

// GoogleCloudVisionV1p1beta1BoundingPoly: A bounding polygon for the
// detected image annotation.
type GoogleCloudVisionV1p1beta1BoundingPoly struct {
	// Vertices: The bounding polygon vertices.
	Vertices []*GoogleCloudVisionV1p1beta1Vertex `json:"vertices,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Vertices") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Vertices") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1BoundingPoly) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1BoundingPoly
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GoogleCloudVisionV1p1beta1ColorInfo: Color information consists of
// RGB channels, score, and the fraction of
// the image that the color occupies in the image.
type GoogleCloudVisionV1p1beta1ColorInfo struct {
	// Color: RGB components of the color.
	Color *Color `json:"color,omitempty"`

	// PixelFraction: The fraction of pixels the color occupies in the
	// image.
	// Value in range [0, 1].
	PixelFraction float64 `json:"pixelFraction,omitempty"`

	// Score: Image-specific score for this color. Value in range [0, 1].
	Score float64 `json:"score,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Color") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Color") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1ColorInfo) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1ColorInfo
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

func (s *GoogleCloudVisionV1p1beta1ColorInfo) UnmarshalJSON(data []byte) error {
	type NoMethod GoogleCloudVisionV1p1beta1ColorInfo
	var s1 struct {
		PixelFraction gensupport.JSONFloat64 `json:"pixelFraction"`
		Score         gensupport.JSONFloat64 `json:"score"`
		*NoMethod
	}
	s1.NoMethod = (*NoMethod)(s)
	if err := json.Unmarshal(data, &s1); err != nil {
		return err
	}
	s.PixelFraction = float64(s1.PixelFraction)
	s.Score = float64(s1.Score)
	return nil
}

// GoogleCloudVisionV1p1beta1CropHint: Single crop hint that is used to
// generate a new crop when serving an image.
type GoogleCloudVisionV1p1beta1CropHint struct {
	// BoundingPoly: The bounding polygon for the crop region. The
	// coordinates of the bounding
	// box are in the original image's scale, as returned in `ImageParams`.
	BoundingPoly *GoogleCloudVisionV1p1beta1BoundingPoly `json:"boundingPoly,omitempty"`

	// Confidence: Confidence of this being a salient region.  Range [0, 1].
	Confidence float64 `json:"confidence,omitempty"`

	// ImportanceFraction: Fraction of importance of this salient region
	// with respect to the original
	// image.
	ImportanceFraction float64 `json:"importanceFraction,omitempty"`

	// ForceSendFields is a list of field names (e.g. "BoundingPoly") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "BoundingPoly") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1CropHint) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1CropHint
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

func (s *GoogleCloudVisionV1p1beta1CropHint) UnmarshalJSON(data []byte) error {
	type NoMethod GoogleCloudVisionV1p1beta1CropHint
	var s1 struct {
		Confidence         gensupport.JSONFloat64 `json:"confidence"`
		ImportanceFraction gensupport.JSONFloat64 `json:"importanceFraction"`
		*NoMethod
	}
	s1.NoMethod = (*NoMethod)(s)
	if err := json.Unmarshal(data, &s1); err != nil {
		return err
	}
	s.Confidence = float64(s1.Confidence)
	s.ImportanceFraction = float64(s1.ImportanceFraction)
	return nil
}

// GoogleCloudVisionV1p1beta1CropHintsAnnotation: Set of crop hints that
// are used to generate new crops when serving images.
type GoogleCloudVisionV1p1beta1CropHintsAnnotation struct {
	// CropHints: Crop hint results.
	CropHints []*GoogleCloudVisionV1p1beta1CropHint `json:"cropHints,omitempty"`

	// ForceSendFields is a list of field names (e.g. "CropHints") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "CropHints") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1CropHintsAnnotation) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1CropHintsAnnotation
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GoogleCloudVisionV1p1beta1CropHintsParams: Parameters for crop hints
// annotation request.
type GoogleCloudVisionV1p1beta1CropHintsParams struct {
	// AspectRatios: Aspect ratios in floats, representing the ratio of the
	// width to the height
	// of the image. For example, if the desired aspect ratio is 4/3,
	// the
	// corresponding float value should be 1.33333.  If not specified,
	// the
	// best possible crop is returned. The number of provided aspect ratios
	// is
	// limited to a maximum of 16; any aspect ratios provided after the 16th
	// are
	// ignored.
	AspectRatios []float64 `json:"aspectRatios,omitempty"`

	// ForceSendFields is a list of field names (e.g. "AspectRatios") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "AspectRatios") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1CropHintsParams) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1CropHintsParams
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GoogleCloudVisionV1p1beta1DominantColorsAnnotation: Set of dominant
// colors and their corresponding scores.
type GoogleCloudVisionV1p1beta1DominantColorsAnnotation struct {
	// Colors: RGB color values with their score and pixel fraction.
	Colors []*GoogleCloudVisionV1p1beta1ColorInfo `json:"colors,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Colors") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Colors") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1DominantColorsAnnotation) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1DominantColorsAnnotation
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GoogleCloudVisionV1p1beta1EntityAnnotation: Set of detected entity
// features.
type GoogleCloudVisionV1p1beta1EntityAnnotation struct {
	// BoundingPoly: Image region to which this entity belongs. Not
	// produced
	// for `LABEL_DETECTION` features.
	BoundingPoly *GoogleCloudVisionV1p1beta1BoundingPoly `json:"boundingPoly,omitempty"`

	// Confidence: **Deprecated. Use `score` instead.**
	// The accuracy of the entity detection in an image.
	// For example, for an image in which the "Eiffel Tower" entity is
	// detected,
	// this field represents the confidence that there is a tower in the
	// query
	// image. Range [0, 1].
	Confidence float64 `json:"confidence,omitempty"`

	// Description: Entity textual description, expressed in its `locale`
	// language.
	Description string `json:"description,omitempty"`

	// Locale: The language code for the locale in which the entity
	// textual
	// `description` is expressed.
	Locale string `json:"locale,omitempty"`

	// Locations: The location information for the detected entity.
	// Multiple
	// `LocationInfo` elements can be present because one location
	// may
	// indicate the location of the scene in the image, and another
	// location
	// may indicate the location of the place where the image was
	// taken.
	// Location information is usually present for landmarks.
	Locations []*GoogleCloudVisionV1p1beta1LocationInfo `json:"locations,omitempty"`

	// Mid: Opaque entity ID. Some IDs may be available in
	// [Google Knowledge Graph
	// Search
	// API](https://developers.google.com/knowledge-graph/).
	Mid string `json:"mid,omitempty"`

	// Properties: Some entities may have optional user-supplied `Property`
	// (name/value)
	// fields, such a score or string that qualifies the entity.
	Properties []*GoogleCloudVisionV1p1beta1Property `json:"properties,omitempty"`

	// Score: Overall score of the result. Range [0, 1].
	Score float64 `json:"score,omitempty"`

	// Topicality: The relevancy of the ICA (Image Content Annotation) label
	// to the
	// image. For example, the relevancy of "tower" is likely higher to an
	// image
	// containing the detected "Eiffel Tower" than to an image containing
	// a
	// detected distant towering building, even though the confidence
	// that
	// there is a tower in each image may be the same. Range [0, 1].
	Topicality float64 `json:"topicality,omitempty"`

	// ForceSendFields is a list of field names (e.g. "BoundingPoly") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "BoundingPoly") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1EntityAnnotation) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1EntityAnnotation
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

func (s *GoogleCloudVisionV1p1beta1EntityAnnotation) UnmarshalJSON(data []byte) error {
	type NoMethod GoogleCloudVisionV1p1beta1EntityAnnotation
	var s1 struct {
		Confidence gensupport.JSONFloat64 `json:"confidence"`
		Score      gensupport.JSONFloat64 `json:"score"`
		Topicality gensupport.JSONFloat64 `json:"topicality"`
		*NoMethod
	}
	s1.NoMethod = (*NoMethod)(s)
	if err := json.Unmarshal(data, &s1); err != nil {
		return err
	}
	s.Confidence = float64(s1.Confidence)
	s.Score = float64(s1.Score)
	s.Topicality = float64(s1.Topicality)
	return nil
}

// GoogleCloudVisionV1p1beta1FaceAnnotation: A face annotation object
// contains the results of face detection.
type GoogleCloudVisionV1p1beta1FaceAnnotation struct {
	// AngerLikelihood: Anger likelihood.
	//
	// Possible values:
	//   "UNKNOWN" - Unknown likelihood.
	//   "VERY_UNLIKELY" - It is very unlikely that the image belongs to the
	// specified vertical.
	//   "UNLIKELY" - It is unlikely that the image belongs to the specified
	// vertical.
	//   "POSSIBLE" - It is possible that the image belongs to the specified
	// vertical.
	//   "LIKELY" - It is likely that the image belongs to the specified
	// vertical.
	//   "VERY_LIKELY" - It is very likely that the image belongs to the
	// specified vertical.
	AngerLikelihood string `json:"angerLikelihood,omitempty"`

	// BlurredLikelihood: Blurred likelihood.
	//
	// Possible values:
	//   "UNKNOWN" - Unknown likelihood.
	//   "VERY_UNLIKELY" - It is very unlikely that the image belongs to the
	// specified vertical.
	//   "UNLIKELY" - It is unlikely that the image belongs to the specified
	// vertical.
	//   "POSSIBLE" - It is possible that the image belongs to the specified
	// vertical.
	//   "LIKELY" - It is likely that the image belongs to the specified
	// vertical.
	//   "VERY_LIKELY" - It is very likely that the image belongs to the
	// specified vertical.
	BlurredLikelihood string `json:"blurredLikelihood,omitempty"`

	// BoundingPoly: The bounding polygon around the face. The coordinates
	// of the bounding box
	// are in the original image's scale, as returned in `ImageParams`.
	// The bounding box is computed to "frame" the face in accordance with
	// human
	// expectations. It is based on the landmarker results.
	// Note that one or more x and/or y coordinates may not be generated in
	// the
	// `BoundingPoly` (the polygon will be unbounded) if only a partial
	// face
	// appears in the image to be annotated.
	BoundingPoly *GoogleCloudVisionV1p1beta1BoundingPoly `json:"boundingPoly,omitempty"`

	// DetectionConfidence: Detection confidence. Range [0, 1].
	DetectionConfidence float64 `json:"detectionConfidence,omitempty"`

	// FdBoundingPoly: The `fd_bounding_poly` bounding polygon is tighter
	// than the
	// `boundingPoly`, and encloses only the skin part of the face.
	// Typically, it
	// is used to eliminate the face from any image analysis that detects
	// the
	// "amount of skin" visible in an image. It is not based on
	// the
	// landmarker results, only on the initial face detection, hence
	// the <code>fd</code> (face detection) prefix.
	FdBoundingPoly *GoogleCloudVisionV1p1beta1BoundingPoly `json:"fdBoundingPoly,omitempty"`

	// HeadwearLikelihood: Headwear likelihood.
	//
	// Possible values:
	//   "UNKNOWN" - Unknown likelihood.
	//   "VERY_UNLIKELY" - It is very unlikely that the image belongs to the
	// specified vertical.
	//   "UNLIKELY" - It is unlikely that the image belongs to the specified
	// vertical.
	//   "POSSIBLE" - It is possible that the image belongs to the specified
	// vertical.
	//   "LIKELY" - It is likely that the image belongs to the specified
	// vertical.
	//   "VERY_LIKELY" - It is very likely that the image belongs to the
	// specified vertical.
	HeadwearLikelihood string `json:"headwearLikelihood,omitempty"`

	// JoyLikelihood: Joy likelihood.
	//
	// Possible values:
	//   "UNKNOWN" - Unknown likelihood.
	//   "VERY_UNLIKELY" - It is very unlikely that the image belongs to the
	// specified vertical.
	//   "UNLIKELY" - It is unlikely that the image belongs to the specified
	// vertical.
	//   "POSSIBLE" - It is possible that the image belongs to the specified
	// vertical.
	//   "LIKELY" - It is likely that the image belongs to the specified
	// vertical.
	//   "VERY_LIKELY" - It is very likely that the image belongs to the
	// specified vertical.
	JoyLikelihood string `json:"joyLikelihood,omitempty"`

	// LandmarkingConfidence: Face landmarking confidence. Range [0, 1].
	LandmarkingConfidence float64 `json:"landmarkingConfidence,omitempty"`

	// Landmarks: Detected face landmarks.
	Landmarks []*GoogleCloudVisionV1p1beta1FaceAnnotationLandmark `json:"landmarks,omitempty"`

	// PanAngle: Yaw angle, which indicates the leftward/rightward angle
	// that the face is
	// pointing relative to the vertical plane perpendicular to the image.
	// Range
	// [-180,180].
	PanAngle float64 `json:"panAngle,omitempty"`

	// RollAngle: Roll angle, which indicates the amount of
	// clockwise/anti-clockwise rotation
	// of the face relative to the image vertical about the axis
	// perpendicular to
	// the face. Range [-180,180].
	RollAngle float64 `json:"rollAngle,omitempty"`

	// SorrowLikelihood: Sorrow likelihood.
	//
	// Possible values:
	//   "UNKNOWN" - Unknown likelihood.
	//   "VERY_UNLIKELY" - It is very unlikely that the image belongs to the
	// specified vertical.
	//   "UNLIKELY" - It is unlikely that the image belongs to the specified
	// vertical.
	//   "POSSIBLE" - It is possible that the image belongs to the specified
	// vertical.
	//   "LIKELY" - It is likely that the image belongs to the specified
	// vertical.
	//   "VERY_LIKELY" - It is very likely that the image belongs to the
	// specified vertical.
	SorrowLikelihood string `json:"sorrowLikelihood,omitempty"`

	// SurpriseLikelihood: Surprise likelihood.
	//
	// Possible values:
	//   "UNKNOWN" - Unknown likelihood.
	//   "VERY_UNLIKELY" - It is very unlikely that the image belongs to the
	// specified vertical.
	//   "UNLIKELY" - It is unlikely that the image belongs to the specified
	// vertical.
	//   "POSSIBLE" - It is possible that the image belongs to the specified
	// vertical.
	//   "LIKELY" - It is likely that the image belongs to the specified
	// vertical.
	//   "VERY_LIKELY" - It is very likely that the image belongs to the
	// specified vertical.
	SurpriseLikelihood string `json:"surpriseLikelihood,omitempty"`

	// TiltAngle: Pitch angle, which indicates the upwards/downwards angle
	// that the face is
	// pointing relative to the image's horizontal plane. Range [-180,180].
	TiltAngle float64 `json:"tiltAngle,omitempty"`

	// UnderExposedLikelihood: Under-exposed likelihood.
	//
	// Possible values:
	//   "UNKNOWN" - Unknown likelihood.
	//   "VERY_UNLIKELY" - It is very unlikely that the image belongs to the
	// specified vertical.
	//   "UNLIKELY" - It is unlikely that the image belongs to the specified
	// vertical.
	//   "POSSIBLE" - It is possible that the image belongs to the specified
	// vertical.
	//   "LIKELY" - It is likely that the image belongs to the specified
	// vertical.
	//   "VERY_LIKELY" - It is very likely that the image belongs to the
	// specified vertical.
	UnderExposedLikelihood string `json:"underExposedLikelihood,omitempty"`

	// ForceSendFields is a list of field names (e.g. "AngerLikelihood") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "AngerLikelihood") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1FaceAnnotation) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1FaceAnnotation
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

func (s *GoogleCloudVisionV1p1beta1FaceAnnotation) UnmarshalJSON(data []byte) error {
	type NoMethod GoogleCloudVisionV1p1beta1FaceAnnotation
	var s1 struct {
		DetectionConfidence   gensupport.JSONFloat64 `json:"detectionConfidence"`
		LandmarkingConfidence gensupport.JSONFloat64 `json:"landmarkingConfidence"`
		PanAngle              gensupport.JSONFloat64 `json:"panAngle"`
		RollAngle             gensupport.JSONFloat64 `json:"rollAngle"`
		TiltAngle             gensupport.JSONFloat64 `json:"tiltAngle"`
		*NoMethod
	}
	s1.NoMethod = (*NoMethod)(s)
	if err := json.Unmarshal(data, &s1); err != nil {
		return err
	}
	s.DetectionConfidence = float64(s1.DetectionConfidence)
	s.LandmarkingConfidence = float64(s1.LandmarkingConfidence)
	s.PanAngle = float64(s1.PanAngle)
	s.RollAngle = float64(s1.RollAngle)
	s.TiltAngle = float64(s1.TiltAngle)
	return nil
}

// GoogleCloudVisionV1p1beta1FaceAnnotationLandmark: A face-specific
// landmark (for example, a face feature).
type GoogleCloudVisionV1p1beta1FaceAnnotationLandmark struct {
	// Position: Face landmark position.
	Position *GoogleCloudVisionV1p1beta1Position `json:"position,omitempty"`

	// Type: Face landmark type.
	//
	// Possible values:
	//   "UNKNOWN_LANDMARK" - Unknown face landmark detected. Should not be
	// filled.
	//   "LEFT_EYE" - Left eye.
	//   "RIGHT_EYE" - Right eye.
	//   "LEFT_OF_LEFT_EYEBROW" - Left of left eyebrow.
	//   "RIGHT_OF_LEFT_EYEBROW" - Right of left eyebrow.
	//   "LEFT_OF_RIGHT_EYEBROW" - Left of right eyebrow.
	//   "RIGHT_OF_RIGHT_EYEBROW" - Right of right eyebrow.
	//   "MIDPOINT_BETWEEN_EYES" - Midpoint between eyes.
	//   "NOSE_TIP" - Nose tip.
	//   "UPPER_LIP" - Upper lip.
	//   "LOWER_LIP" - Lower lip.
	//   "MOUTH_LEFT" - Mouth left.
	//   "MOUTH_RIGHT" - Mouth right.
	//   "MOUTH_CENTER" - Mouth center.
	//   "NOSE_BOTTOM_RIGHT" - Nose, bottom right.
	//   "NOSE_BOTTOM_LEFT" - Nose, bottom left.
	//   "NOSE_BOTTOM_CENTER" - Nose, bottom center.
	//   "LEFT_EYE_TOP_BOUNDARY" - Left eye, top boundary.
	//   "LEFT_EYE_RIGHT_CORNER" - Left eye, right corner.
	//   "LEFT_EYE_BOTTOM_BOUNDARY" - Left eye, bottom boundary.
	//   "LEFT_EYE_LEFT_CORNER" - Left eye, left corner.
	//   "RIGHT_EYE_TOP_BOUNDARY" - Right eye, top boundary.
	//   "RIGHT_EYE_RIGHT_CORNER" - Right eye, right corner.
	//   "RIGHT_EYE_BOTTOM_BOUNDARY" - Right eye, bottom boundary.
	//   "RIGHT_EYE_LEFT_CORNER" - Right eye, left corner.
	//   "LEFT_EYEBROW_UPPER_MIDPOINT" - Left eyebrow, upper midpoint.
	//   "RIGHT_EYEBROW_UPPER_MIDPOINT" - Right eyebrow, upper midpoint.
	//   "LEFT_EAR_TRAGION" - Left ear tragion.
	//   "RIGHT_EAR_TRAGION" - Right ear tragion.
	//   "LEFT_EYE_PUPIL" - Left eye pupil.
	//   "RIGHT_EYE_PUPIL" - Right eye pupil.
	//   "FOREHEAD_GLABELLA" - Forehead glabella.
	//   "CHIN_GNATHION" - Chin gnathion.
	//   "CHIN_LEFT_GONION" - Chin left gonion.
	//   "CHIN_RIGHT_GONION" - Chin right gonion.
	Type string `json:"type,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Position") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Position") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1FaceAnnotationLandmark) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1FaceAnnotationLandmark
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GoogleCloudVisionV1p1beta1Feature: The type of Google Cloud Vision
// API detection to perform, and the maximum
// number of results to return for that type. Multiple `Feature` objects
// can
// be specified in the `features` list.
type GoogleCloudVisionV1p1beta1Feature struct {
	// MaxResults: Maximum number of results of this type. Does not apply
	// to
	// `TEXT_DETECTION`, `DOCUMENT_TEXT_DETECTION`, or `CROP_HINTS`.
	MaxResults int64 `json:"maxResults,omitempty"`

	// Model: Model to use for the feature.
	// Supported values: "builtin/stable" (the default if unset)
	// and
	// "builtin/latest".
	Model string `json:"model,omitempty"`

	// Type: The feature type.
	//
	// Possible values:
	//   "TYPE_UNSPECIFIED" - Unspecified feature type.
	//   "FACE_DETECTION" - Run face detection.
	//   "LANDMARK_DETECTION" - Run landmark detection.
	//   "LOGO_DETECTION" - Run logo detection.
	//   "LABEL_DETECTION" - Run label detection.
	//   "TEXT_DETECTION" - Run text detection / optical character
	// recognition (OCR). Text detection
	// is optimized for areas of text within a larger image; if the image
	// is
	// a document, use `DOCUMENT_TEXT_DETECTION` instead.
	//   "DOCUMENT_TEXT_DETECTION" - Run dense text document OCR. Takes
	// precedence when both
	// `DOCUMENT_TEXT_DETECTION` and `TEXT_DETECTION` are present.
	//   "SAFE_SEARCH_DETECTION" - Run Safe Search to detect potentially
	// unsafe
	// or undesirable content.
	//   "IMAGE_PROPERTIES" - Compute a set of image properties, such as
	// the
	// image's dominant colors.
	//   "CROP_HINTS" - Run crop hints.
	//   "WEB_DETECTION" - Run web detection.
	Type string `json:"type,omitempty"`

	// ForceSendFields is a list of field names (e.g. "MaxResults") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "MaxResults") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1Feature) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1Feature
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GoogleCloudVisionV1p1beta1Image: Client image to perform Google Cloud
// Vision API tasks over.
type GoogleCloudVisionV1p1beta1Image struct {
	// Content: Image content, represented as a stream of bytes.
	// Note: As with all `bytes` fields, protobuffers use a pure
	// binary
	// representation, whereas JSON representations use base64.
	Content string `json:"content,omitempty"`

	// Source: Google Cloud Storage image location, or publicly-accessible
	// image
	// URL. If both `content` and `source` are provided for an image,
	// `content`
	// takes precedence and is used to perform the image annotation request.
	Source *GoogleCloudVisionV1p1beta1ImageSource `json:"source,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Content") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Content") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1Image) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1Image
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GoogleCloudVisionV1p1beta1ImageContext: Image context and/or
// feature-specific parameters.
type GoogleCloudVisionV1p1beta1ImageContext struct {
	// CropHintsParams: Parameters for crop hints annotation request.
	CropHintsParams *GoogleCloudVisionV1p1beta1CropHintsParams `json:"cropHintsParams,omitempty"`

	// LanguageHints: List of languages to use for TEXT_DETECTION. In most
	// cases, an empty value
	// yields the best results since it enables automatic language
	// detection. For
	// languages based on the Latin alphabet, setting `language_hints` is
	// not
	// needed. In rare cases, when the language of the text in the image is
	// known,
	// setting a hint will help get better results (although it will be
	// a
	// significant hindrance if the hint is wrong). Text detection returns
	// an
	// error if one or more of the specified languages is not one of
	// the
	// [supported languages](/vision/docs/languages).
	LanguageHints []string `json:"languageHints,omitempty"`

	// LatLongRect: lat/long rectangle that specifies the location of the
	// image.
	LatLongRect *GoogleCloudVisionV1p1beta1LatLongRect `json:"latLongRect,omitempty"`

	// WebDetectionParams: Parameters for web detection.
	WebDetectionParams *GoogleCloudVisionV1p1beta1WebDetectionParams `json:"webDetectionParams,omitempty"`

	// ForceSendFields is a list of field names (e.g. "CropHintsParams") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "CropHintsParams") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1ImageContext) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1ImageContext
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GoogleCloudVisionV1p1beta1ImageProperties: Stores image properties,
// such as dominant colors.
type GoogleCloudVisionV1p1beta1ImageProperties struct {
	// DominantColors: If present, dominant colors completed successfully.
	DominantColors *GoogleCloudVisionV1p1beta1DominantColorsAnnotation `json:"dominantColors,omitempty"`

	// ForceSendFields is a list of field names (e.g. "DominantColors") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "DominantColors") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1ImageProperties) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1ImageProperties
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GoogleCloudVisionV1p1beta1ImageSource: External image source (Google
// Cloud Storage or web URL image location).
type GoogleCloudVisionV1p1beta1ImageSource struct {
	// GcsImageUri: **Use `image_uri` instead.**
	//
	// The Google Cloud Storage  URI of the
	// form
	// `gs://bucket_name/object_name`. Object versioning is not supported.
	// See
	// [Google Cloud Storage
	// Request
	// URIs](https://cloud.google.com/storage/docs/reference-uris) for more
	// info.
	GcsImageUri string `json:"gcsImageUri,omitempty"`

	// ImageUri: The URI of the source image. Can be either:
	//
	// 1. A Google Cloud Storage URI of the form
	//    `gs://bucket_name/object_name`. Object versioning is not
	// supported. See
	//    [Google Cloud Storage Request
	//    URIs](https://cloud.google.com/storage/docs/reference-uris) for
	// more
	//    info.
	//
	// 2. A publicly-accessible image HTTP/HTTPS URL. When fetching images
	// from
	//    HTTP/HTTPS URLs, Google cannot guarantee that the request will be
	//    completed. Your request may fail if the specified host denies the
	//    request (e.g. due to request throttling or DOS prevention), or if
	// Google
	//    throttles requests to the site for abuse prevention. You should
	// not
	//    depend on externally-hosted images for production
	// applications.
	//
	// When both `gcs_image_uri` and `image_uri` are specified, `image_uri`
	// takes
	// precedence.
	ImageUri string `json:"imageUri,omitempty"`

	// ForceSendFields is a list of field names (e.g. "GcsImageUri") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "GcsImageUri") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1ImageSource) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1ImageSource
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GoogleCloudVisionV1p1beta1LatLongRect: Rectangle determined by min
// and max `LatLng` pairs.
type GoogleCloudVisionV1p1beta1LatLongRect struct {
	// MaxLatLng: Max lat/long pair.
	MaxLatLng *LatLng `json:"maxLatLng,omitempty"`

	// MinLatLng: Min lat/long pair.
	MinLatLng *LatLng `json:"minLatLng,omitempty"`

	// ForceSendFields is a list of field names (e.g. "MaxLatLng") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "MaxLatLng") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1LatLongRect) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1LatLongRect
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GoogleCloudVisionV1p1beta1LocationInfo: Detected entity location
// information.
type GoogleCloudVisionV1p1beta1LocationInfo struct {
	// LatLng: lat/long location coordinates.
	LatLng *LatLng `json:"latLng,omitempty"`

	// ForceSendFields is a list of field names (e.g. "LatLng") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "LatLng") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1LocationInfo) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1LocationInfo
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GoogleCloudVisionV1p1beta1Page: Detected page from OCR.
type GoogleCloudVisionV1p1beta1Page struct {
	// Blocks: List of blocks of text, images etc on this page.
	Blocks []*GoogleCloudVisionV1p1beta1Block `json:"blocks,omitempty"`

	// Confidence: Confidence of the OCR results on the page. Range [0, 1].
	Confidence float64 `json:"confidence,omitempty"`

	// Height: Page height in pixels.
	Height int64 `json:"height,omitempty"`

	// Property: Additional information detected on the page.
	Property *GoogleCloudVisionV1p1beta1TextAnnotationTextProperty `json:"property,omitempty"`

	// Width: Page width in pixels.
	Width int64 `json:"width,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Blocks") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Blocks") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1Page) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1Page
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

func (s *GoogleCloudVisionV1p1beta1Page) UnmarshalJSON(data []byte) error {
	type NoMethod GoogleCloudVisionV1p1beta1Page
	var s1 struct {
		Confidence gensupport.JSONFloat64 `json:"confidence"`
		*NoMethod
	}
	s1.NoMethod = (*NoMethod)(s)
	if err := json.Unmarshal(data, &s1); err != nil {
		return err
	}
	s.Confidence = float64(s1.Confidence)
	return nil
}

// GoogleCloudVisionV1p1beta1Paragraph: Structural unit of text
// representing a number of words in certain order.
type GoogleCloudVisionV1p1beta1Paragraph struct {
	// BoundingBox: The bounding box for the paragraph.
	// The vertices are in the order of top-left, top-right,
	// bottom-right,
	// bottom-left. When a rotation of the bounding box is detected the
	// rotation
	// is represented as around the top-left corner as defined when the text
	// is
	// read in the 'natural' orientation.
	// For example:
	//   * when the text is horizontal it might look like:
	//      0----1
	//      |    |
	//      3----2
	//   * when it's rotated 180 degrees around the top-left corner it
	// becomes:
	//      2----3
	//      |    |
	//      1----0
	//   and the vertice order will still be (0, 1, 2, 3).
	BoundingBox *GoogleCloudVisionV1p1beta1BoundingPoly `json:"boundingBox,omitempty"`

	// Confidence: Confidence of the OCR results for the paragraph. Range
	// [0, 1].
	Confidence float64 `json:"confidence,omitempty"`

	// Property: Additional information detected for the paragraph.
	Property *GoogleCloudVisionV1p1beta1TextAnnotationTextProperty `json:"property,omitempty"`

	// Words: List of words in this paragraph.
	Words []*GoogleCloudVisionV1p1beta1Word `json:"words,omitempty"`

	// ForceSendFields is a list of field names (e.g. "BoundingBox") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "BoundingBox") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1Paragraph) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1Paragraph
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

func (s *GoogleCloudVisionV1p1beta1Paragraph) UnmarshalJSON(data []byte) error {
	type NoMethod GoogleCloudVisionV1p1beta1Paragraph
	var s1 struct {
		Confidence gensupport.JSONFloat64 `json:"confidence"`
		*NoMethod
	}
	s1.NoMethod = (*NoMethod)(s)
	if err := json.Unmarshal(data, &s1); err != nil {
		return err
	}
	s.Confidence = float64(s1.Confidence)
	return nil
}

// GoogleCloudVisionV1p1beta1Position: A 3D position in the image, used
// primarily for Face detection landmarks.
// A valid Position must have both x and y coordinates.
// The position coordinates are in the same scale as the original image.
type GoogleCloudVisionV1p1beta1Position struct {
	// X: X coordinate.
	X float64 `json:"x,omitempty"`

	// Y: Y coordinate.
	Y float64 `json:"y,omitempty"`

	// Z: Z coordinate (or depth).
	Z float64 `json:"z,omitempty"`

	// ForceSendFields is a list of field names (e.g. "X") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "X") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1Position) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1Position
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

func (s *GoogleCloudVisionV1p1beta1Position) UnmarshalJSON(data []byte) error {
	type NoMethod GoogleCloudVisionV1p1beta1Position
	var s1 struct {
		X gensupport.JSONFloat64 `json:"x"`
		Y gensupport.JSONFloat64 `json:"y"`
		Z gensupport.JSONFloat64 `json:"z"`
		*NoMethod
	}
	s1.NoMethod = (*NoMethod)(s)
	if err := json.Unmarshal(data, &s1); err != nil {
		return err
	}
	s.X = float64(s1.X)
	s.Y = float64(s1.Y)
	s.Z = float64(s1.Z)
	return nil
}

// GoogleCloudVisionV1p1beta1Property: A `Property` consists of a
// user-supplied name/value pair.
type GoogleCloudVisionV1p1beta1Property struct {
	// Name: Name of the property.
	Name string `json:"name,omitempty"`

	// Uint64Value: Value of numeric properties.
	Uint64Value uint64 `json:"uint64Value,omitempty,string"`

	// Value: Value of the property.
	Value string `json:"value,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Name") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Name") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1Property) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1Property
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GoogleCloudVisionV1p1beta1SafeSearchAnnotation: Set of features
// pertaining to the image, computed by computer vision
// methods over safe-search verticals (for example, adult, spoof,
// medical,
// violence).
type GoogleCloudVisionV1p1beta1SafeSearchAnnotation struct {
	// Adult: Represents the adult content likelihood for the image. Adult
	// content may
	// contain elements such as nudity, pornographic images or cartoons,
	// or
	// sexual activities.
	//
	// Possible values:
	//   "UNKNOWN" - Unknown likelihood.
	//   "VERY_UNLIKELY" - It is very unlikely that the image belongs to the
	// specified vertical.
	//   "UNLIKELY" - It is unlikely that the image belongs to the specified
	// vertical.
	//   "POSSIBLE" - It is possible that the image belongs to the specified
	// vertical.
	//   "LIKELY" - It is likely that the image belongs to the specified
	// vertical.
	//   "VERY_LIKELY" - It is very likely that the image belongs to the
	// specified vertical.
	Adult string `json:"adult,omitempty"`

	// Medical: Likelihood that this is a medical image.
	//
	// Possible values:
	//   "UNKNOWN" - Unknown likelihood.
	//   "VERY_UNLIKELY" - It is very unlikely that the image belongs to the
	// specified vertical.
	//   "UNLIKELY" - It is unlikely that the image belongs to the specified
	// vertical.
	//   "POSSIBLE" - It is possible that the image belongs to the specified
	// vertical.
	//   "LIKELY" - It is likely that the image belongs to the specified
	// vertical.
	//   "VERY_LIKELY" - It is very likely that the image belongs to the
	// specified vertical.
	Medical string `json:"medical,omitempty"`

	// Racy: Likelihood that the request image contains racy content. Racy
	// content may
	// include (but is not limited to) skimpy or sheer clothing,
	// strategically
	// covered nudity, lewd or provocative poses, or close-ups of
	// sensitive
	// body areas.
	//
	// Possible values:
	//   "UNKNOWN" - Unknown likelihood.
	//   "VERY_UNLIKELY" - It is very unlikely that the image belongs to the
	// specified vertical.
	//   "UNLIKELY" - It is unlikely that the image belongs to the specified
	// vertical.
	//   "POSSIBLE" - It is possible that the image belongs to the specified
	// vertical.
	//   "LIKELY" - It is likely that the image belongs to the specified
	// vertical.
	//   "VERY_LIKELY" - It is very likely that the image belongs to the
	// specified vertical.
	Racy string `json:"racy,omitempty"`

	// Spoof: Spoof likelihood. The likelihood that an modification
	// was made to the image's canonical version to make it appear
	// funny or offensive.
	//
	// Possible values:
	//   "UNKNOWN" - Unknown likelihood.
	//   "VERY_UNLIKELY" - It is very unlikely that the image belongs to the
	// specified vertical.
	//   "UNLIKELY" - It is unlikely that the image belongs to the specified
	// vertical.
	//   "POSSIBLE" - It is possible that the image belongs to the specified
	// vertical.
	//   "LIKELY" - It is likely that the image belongs to the specified
	// vertical.
	//   "VERY_LIKELY" - It is very likely that the image belongs to the
	// specified vertical.
	Spoof string `json:"spoof,omitempty"`

	// Violence: Likelihood that this image contains violent content.
	//
	// Possible values:
	//   "UNKNOWN" - Unknown likelihood.
	//   "VERY_UNLIKELY" - It is very unlikely that the image belongs to the
	// specified vertical.
	//   "UNLIKELY" - It is unlikely that the image belongs to the specified
	// vertical.
	//   "POSSIBLE" - It is possible that the image belongs to the specified
	// vertical.
	//   "LIKELY" - It is likely that the image belongs to the specified
	// vertical.
	//   "VERY_LIKELY" - It is very likely that the image belongs to the
	// specified vertical.
	Violence string `json:"violence,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Adult") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Adult") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1SafeSearchAnnotation) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1SafeSearchAnnotation
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GoogleCloudVisionV1p1beta1Symbol: A single symbol representation.
type GoogleCloudVisionV1p1beta1Symbol struct {
	// BoundingBox: The bounding box for the symbol.
	// The vertices are in the order of top-left, top-right,
	// bottom-right,
	// bottom-left. When a rotation of the bounding box is detected the
	// rotation
	// is represented as around the top-left corner as defined when the text
	// is
	// read in the 'natural' orientation.
	// For example:
	//   * when the text is horizontal it might look like:
	//      0----1
	//      |    |
	//      3----2
	//   * when it's rotated 180 degrees around the top-left corner it
	// becomes:
	//      2----3
	//      |    |
	//      1----0
	//   and the vertice order will still be (0, 1, 2, 3).
	BoundingBox *GoogleCloudVisionV1p1beta1BoundingPoly `json:"boundingBox,omitempty"`

	// Confidence: Confidence of the OCR results for the symbol. Range [0,
	// 1].
	Confidence float64 `json:"confidence,omitempty"`

	// Property: Additional information detected for the symbol.
	Property *GoogleCloudVisionV1p1beta1TextAnnotationTextProperty `json:"property,omitempty"`

	// Text: The actual UTF-8 representation of the symbol.
	Text string `json:"text,omitempty"`

	// ForceSendFields is a list of field names (e.g. "BoundingBox") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "BoundingBox") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1Symbol) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1Symbol
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

func (s *GoogleCloudVisionV1p1beta1Symbol) UnmarshalJSON(data []byte) error {
	type NoMethod GoogleCloudVisionV1p1beta1Symbol
	var s1 struct {
		Confidence gensupport.JSONFloat64 `json:"confidence"`
		*NoMethod
	}
	s1.NoMethod = (*NoMethod)(s)
	if err := json.Unmarshal(data, &s1); err != nil {
		return err
	}
	s.Confidence = float64(s1.Confidence)
	return nil
}

// GoogleCloudVisionV1p1beta1TextAnnotation: TextAnnotation contains a
// structured representation of OCR extracted text.
// The hierarchy of an OCR extracted text structure is like this:
//     TextAnnotation -> Page -> Block -> Paragraph -> Word ->
// Symbol
// Each structural component, starting from Page, may further have their
// own
// properties. Properties describe detected languages, breaks etc..
// Please refer
// to the TextAnnotation.TextProperty message definition below for
// more
// detail.
type GoogleCloudVisionV1p1beta1TextAnnotation struct {
	// Pages: List of pages detected by OCR.
	Pages []*GoogleCloudVisionV1p1beta1Page `json:"pages,omitempty"`

	// Text: UTF-8 text detected on the pages.
	Text string `json:"text,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Pages") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Pages") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1TextAnnotation) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1TextAnnotation
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GoogleCloudVisionV1p1beta1TextAnnotationDetectedBreak: Detected start
// or end of a structural component.
type GoogleCloudVisionV1p1beta1TextAnnotationDetectedBreak struct {
	// IsPrefix: True if break prepends the element.
	IsPrefix bool `json:"isPrefix,omitempty"`

	// Type: Detected break type.
	//
	// Possible values:
	//   "UNKNOWN" - Unknown break label type.
	//   "SPACE" - Regular space.
	//   "SURE_SPACE" - Sure space (very wide).
	//   "EOL_SURE_SPACE" - Line-wrapping break.
	//   "HYPHEN" - End-line hyphen that is not present in text; does not
	// co-occur with
	// `SPACE`, `LEADER_SPACE`, or `LINE_BREAK`.
	//   "LINE_BREAK" - Line break that ends a paragraph.
	Type string `json:"type,omitempty"`

	// ForceSendFields is a list of field names (e.g. "IsPrefix") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "IsPrefix") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1TextAnnotationDetectedBreak) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1TextAnnotationDetectedBreak
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GoogleCloudVisionV1p1beta1TextAnnotationDetectedLanguage: Detected
// language for a structural component.
type GoogleCloudVisionV1p1beta1TextAnnotationDetectedLanguage struct {
	// Confidence: Confidence of detected language. Range [0, 1].
	Confidence float64 `json:"confidence,omitempty"`

	// LanguageCode: The BCP-47 language code, such as "en-US" or "sr-Latn".
	// For more
	// information,
	// see
	// http://www.unicode.org/reports/tr35/#Unicode_locale_identifier.
	LanguageCode string `json:"languageCode,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Confidence") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Confidence") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1TextAnnotationDetectedLanguage) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1TextAnnotationDetectedLanguage
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

func (s *GoogleCloudVisionV1p1beta1TextAnnotationDetectedLanguage) UnmarshalJSON(data []byte) error {
	type NoMethod GoogleCloudVisionV1p1beta1TextAnnotationDetectedLanguage
	var s1 struct {
		Confidence gensupport.JSONFloat64 `json:"confidence"`
		*NoMethod
	}
	s1.NoMethod = (*NoMethod)(s)
	if err := json.Unmarshal(data, &s1); err != nil {
		return err
	}
	s.Confidence = float64(s1.Confidence)
	return nil
}

// GoogleCloudVisionV1p1beta1TextAnnotationTextProperty: Additional
// information detected on the structural component.
type GoogleCloudVisionV1p1beta1TextAnnotationTextProperty struct {
	// DetectedBreak: Detected start or end of a text segment.
	DetectedBreak *GoogleCloudVisionV1p1beta1TextAnnotationDetectedBreak `json:"detectedBreak,omitempty"`

	// DetectedLanguages: A list of detected languages together with
	// confidence.
	DetectedLanguages []*GoogleCloudVisionV1p1beta1TextAnnotationDetectedLanguage `json:"detectedLanguages,omitempty"`

	// ForceSendFields is a list of field names (e.g. "DetectedBreak") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "DetectedBreak") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1TextAnnotationTextProperty) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1TextAnnotationTextProperty
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GoogleCloudVisionV1p1beta1Vertex: A vertex represents a 2D point in
// the image.
// NOTE: the vertex coordinates are in the same scale as the original
// image.
type GoogleCloudVisionV1p1beta1Vertex struct {
	// X: X coordinate.
	X int64 `json:"x,omitempty"`

	// Y: Y coordinate.
	Y int64 `json:"y,omitempty"`

	// ForceSendFields is a list of field names (e.g. "X") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "X") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1Vertex) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1Vertex
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GoogleCloudVisionV1p1beta1WebDetection: Relevant information for the
// image from the Internet.
type GoogleCloudVisionV1p1beta1WebDetection struct {
	// BestGuessLabels: Best guess text labels for the request image.
	BestGuessLabels []*GoogleCloudVisionV1p1beta1WebDetectionWebLabel `json:"bestGuessLabels,omitempty"`

	// FullMatchingImages: Fully matching images from the Internet.
	// Can include resized copies of the query image.
	FullMatchingImages []*GoogleCloudVisionV1p1beta1WebDetectionWebImage `json:"fullMatchingImages,omitempty"`

	// PagesWithMatchingImages: Web pages containing the matching images
	// from the Internet.
	PagesWithMatchingImages []*GoogleCloudVisionV1p1beta1WebDetectionWebPage `json:"pagesWithMatchingImages,omitempty"`

	// PartialMatchingImages: Partial matching images from the
	// Internet.
	// Those images are similar enough to share some key-point features.
	// For
	// example an original image will likely have partial matching for its
	// crops.
	PartialMatchingImages []*GoogleCloudVisionV1p1beta1WebDetectionWebImage `json:"partialMatchingImages,omitempty"`

	// VisuallySimilarImages: The visually similar image results.
	VisuallySimilarImages []*GoogleCloudVisionV1p1beta1WebDetectionWebImage `json:"visuallySimilarImages,omitempty"`

	// WebEntities: Deduced entities from similar images on the Internet.
	WebEntities []*GoogleCloudVisionV1p1beta1WebDetectionWebEntity `json:"webEntities,omitempty"`

	// ForceSendFields is a list of field names (e.g. "BestGuessLabels") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "BestGuessLabels") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1WebDetection) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1WebDetection
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GoogleCloudVisionV1p1beta1WebDetectionParams: Parameters for web
// detection request.
type GoogleCloudVisionV1p1beta1WebDetectionParams struct {
	// IncludeGeoResults: Whether to include results derived from the geo
	// information in the image.
	IncludeGeoResults bool `json:"includeGeoResults,omitempty"`

	// ForceSendFields is a list of field names (e.g. "IncludeGeoResults")
	// to unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "IncludeGeoResults") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1WebDetectionParams) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1WebDetectionParams
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GoogleCloudVisionV1p1beta1WebDetectionWebEntity: Entity deduced from
// similar images on the Internet.
type GoogleCloudVisionV1p1beta1WebDetectionWebEntity struct {
	// Description: Canonical description of the entity, in English.
	Description string `json:"description,omitempty"`

	// EntityId: Opaque entity ID.
	EntityId string `json:"entityId,omitempty"`

	// Score: Overall relevancy score for the entity.
	// Not normalized and not comparable across different image queries.
	Score float64 `json:"score,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Description") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Description") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1WebDetectionWebEntity) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1WebDetectionWebEntity
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

func (s *GoogleCloudVisionV1p1beta1WebDetectionWebEntity) UnmarshalJSON(data []byte) error {
	type NoMethod GoogleCloudVisionV1p1beta1WebDetectionWebEntity
	var s1 struct {
		Score gensupport.JSONFloat64 `json:"score"`
		*NoMethod
	}
	s1.NoMethod = (*NoMethod)(s)
	if err := json.Unmarshal(data, &s1); err != nil {
		return err
	}
	s.Score = float64(s1.Score)
	return nil
}

// GoogleCloudVisionV1p1beta1WebDetectionWebImage: Metadata for online
// images.
type GoogleCloudVisionV1p1beta1WebDetectionWebImage struct {
	// Score: (Deprecated) Overall relevancy score for the image.
	Score float64 `json:"score,omitempty"`

	// Url: The result image URL.
	Url string `json:"url,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Score") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Score") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1WebDetectionWebImage) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1WebDetectionWebImage
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

func (s *GoogleCloudVisionV1p1beta1WebDetectionWebImage) UnmarshalJSON(data []byte) error {
	type NoMethod GoogleCloudVisionV1p1beta1WebDetectionWebImage
	var s1 struct {
		Score gensupport.JSONFloat64 `json:"score"`
		*NoMethod
	}
	s1.NoMethod = (*NoMethod)(s)
	if err := json.Unmarshal(data, &s1); err != nil {
		return err
	}
	s.Score = float64(s1.Score)
	return nil
}

// GoogleCloudVisionV1p1beta1WebDetectionWebLabel: Label to provide
// extra metadata for the web detection.
type GoogleCloudVisionV1p1beta1WebDetectionWebLabel struct {
	// Label: Label for extra metadata.
	Label string `json:"label,omitempty"`

	// LanguageCode: The BCP-47 language code for `label`, such as "en-US"
	// or "sr-Latn".
	// For more information,
	// see
	// http://www.unicode.org/reports/tr35/#Unicode_locale_identifier.
	LanguageCode string `json:"languageCode,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Label") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Label") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1WebDetectionWebLabel) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1WebDetectionWebLabel
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GoogleCloudVisionV1p1beta1WebDetectionWebPage: Metadata for web
// pages.
type GoogleCloudVisionV1p1beta1WebDetectionWebPage struct {
	// FullMatchingImages: Fully matching images on the page.
	// Can include resized copies of the query image.
	FullMatchingImages []*GoogleCloudVisionV1p1beta1WebDetectionWebImage `json:"fullMatchingImages,omitempty"`

	// PageTitle: Title for the web page, may contain HTML markups.
	PageTitle string `json:"pageTitle,omitempty"`

	// PartialMatchingImages: Partial matching images on the page.
	// Those images are similar enough to share some key-point features.
	// For
	// example an original image will likely have partial matching for
	// its
	// crops.
	PartialMatchingImages []*GoogleCloudVisionV1p1beta1WebDetectionWebImage `json:"partialMatchingImages,omitempty"`

	// Score: (Deprecated) Overall relevancy score for the web page.
	Score float64 `json:"score,omitempty"`

	// Url: The result web page URL.
	Url string `json:"url,omitempty"`

	// ForceSendFields is a list of field names (e.g. "FullMatchingImages")
	// to unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "FullMatchingImages") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1WebDetectionWebPage) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1WebDetectionWebPage
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

func (s *GoogleCloudVisionV1p1beta1WebDetectionWebPage) UnmarshalJSON(data []byte) error {
	type NoMethod GoogleCloudVisionV1p1beta1WebDetectionWebPage
	var s1 struct {
		Score gensupport.JSONFloat64 `json:"score"`
		*NoMethod
	}
	s1.NoMethod = (*NoMethod)(s)
	if err := json.Unmarshal(data, &s1); err != nil {
		return err
	}
	s.Score = float64(s1.Score)
	return nil
}

// GoogleCloudVisionV1p1beta1Word: A word representation.
type GoogleCloudVisionV1p1beta1Word struct {
	// BoundingBox: The bounding box for the word.
	// The vertices are in the order of top-left, top-right,
	// bottom-right,
	// bottom-left. When a rotation of the bounding box is detected the
	// rotation
	// is represented as around the top-left corner as defined when the text
	// is
	// read in the 'natural' orientation.
	// For example:
	//   * when the text is horizontal it might look like:
	//      0----1
	//      |    |
	//      3----2
	//   * when it's rotated 180 degrees around the top-left corner it
	// becomes:
	//      2----3
	//      |    |
	//      1----0
	//   and the vertice order will still be (0, 1, 2, 3).
	BoundingBox *GoogleCloudVisionV1p1beta1BoundingPoly `json:"boundingBox,omitempty"`

	// Confidence: Confidence of the OCR results for the word. Range [0, 1].
	Confidence float64 `json:"confidence,omitempty"`

	// Property: Additional information detected for the word.
	Property *GoogleCloudVisionV1p1beta1TextAnnotationTextProperty `json:"property,omitempty"`

	// Symbols: List of symbols in the word.
	// The order of the symbols follows the natural reading order.
	Symbols []*GoogleCloudVisionV1p1beta1Symbol `json:"symbols,omitempty"`

	// ForceSendFields is a list of field names (e.g. "BoundingBox") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "BoundingBox") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleCloudVisionV1p1beta1Word) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleCloudVisionV1p1beta1Word
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

func (s *GoogleCloudVisionV1p1beta1Word) UnmarshalJSON(data []byte) error {
	type NoMethod GoogleCloudVisionV1p1beta1Word
	var s1 struct {
		Confidence gensupport.JSONFloat64 `json:"confidence"`
		*NoMethod
	}
	s1.NoMethod = (*NoMethod)(s)
	if err := json.Unmarshal(data, &s1); err != nil {
		return err
	}
	s.Confidence = float64(s1.Confidence)
	return nil
}

// LatLng: An object representing a latitude/longitude pair. This is
// expressed as a pair
// of doubles representing degrees latitude and degrees longitude.
// Unless
// specified otherwise, this must conform to the
// <a
// href="http://www.unoosa.org/pdf/icg/2012/template/WGS_84.pdf">WGS84
// st
// andard</a>. Values must be within normalized ranges.
type LatLng struct {
	// Latitude: The latitude in degrees. It must be in the range [-90.0,
	// +90.0].
	Latitude float64 `json:"latitude,omitempty"`

	// Longitude: The longitude in degrees. It must be in the range [-180.0,
	// +180.0].
	Longitude float64 `json:"longitude,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Latitude") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Latitude") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *LatLng) MarshalJSON() ([]byte, error) {
	type NoMethod LatLng
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

func (s *LatLng) UnmarshalJSON(data []byte) error {
	type NoMethod LatLng
	var s1 struct {
		Latitude  gensupport.JSONFloat64 `json:"latitude"`
		Longitude gensupport.JSONFloat64 `json:"longitude"`
		*NoMethod
	}
	s1.NoMethod = (*NoMethod)(s)
	if err := json.Unmarshal(data, &s1); err != nil {
		return err
	}
	s.Latitude = float64(s1.Latitude)
	s.Longitude = float64(s1.Longitude)
	return nil
}

// Status: The `Status` type defines a logical error model that is
// suitable for different
// programming environments, including REST APIs and RPC APIs. It is
// used by
// [gRPC](https://github.com/grpc). The error model is designed to
// be:
//
// - Simple to use and understand for most users
// - Flexible enough to meet unexpected needs
//
// # Overview
//
// The `Status` message contains three pieces of data: error code, error
// message,
// and error details. The error code should be an enum value
// of
// google.rpc.Code, but it may accept additional error codes if needed.
// The
// error message should be a developer-facing English message that
// helps
// developers *understand* and *resolve* the error. If a localized
// user-facing
// error message is needed, put the localized message in the error
// details or
// localize it in the client. The optional error details may contain
// arbitrary
// information about the error. There is a predefined set of error
// detail types
// in the package `google.rpc` that can be used for common error
// conditions.
//
// # Language mapping
//
// The `Status` message is the logical representation of the error
// model, but it
// is not necessarily the actual wire format. When the `Status` message
// is
// exposed in different client libraries and different wire protocols,
// it can be
// mapped differently. For example, it will likely be mapped to some
// exceptions
// in Java, but more likely mapped to some error codes in C.
//
// # Other uses
//
// The error model and the `Status` message can be used in a variety
// of
// environments, either with or without APIs, to provide a
// consistent developer experience across different
// environments.
//
// Example uses of this error model include:
//
// - Partial errors. If a service needs to return partial errors to the
// client,
//     it may embed the `Status` in the normal response to indicate the
// partial
//     errors.
//
// - Workflow errors. A typical workflow has multiple steps. Each step
// may
//     have a `Status` message for error reporting.
//
// - Batch operations. If a client uses batch request and batch
// response, the
//     `Status` message should be used directly inside batch response,
// one for
//     each error sub-response.
//
// - Asynchronous operations. If an API call embeds asynchronous
// operation
//     results in its response, the status of those operations should
// be
//     represented directly using the `Status` message.
//
// - Logging. If some API errors are stored in logs, the message
// `Status` could
//     be used directly after any stripping needed for security/privacy
// reasons.
type Status struct {
	// Code: The status code, which should be an enum value of
	// google.rpc.Code.
	Code int64 `json:"code,omitempty"`

	// Details: A list of messages that carry the error details.  There is a
	// common set of
	// message types for APIs to use.
	Details []googleapi.RawMessage `json:"details,omitempty"`

	// Message: A developer-facing error message, which should be in
	// English. Any
	// user-facing error message should be localized and sent in
	// the
	// google.rpc.Status.details field, or localized by the client.
	Message string `json:"message,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Code") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Code") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *Status) MarshalJSON() ([]byte, error) {
	type NoMethod Status
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// method id "vision.images.annotate":

type ImagesAnnotateCall struct {
	s                                                    *Service
	googlecloudvisionv1p1beta1batchannotateimagesrequest *GoogleCloudVisionV1p1beta1BatchAnnotateImagesRequest
	urlParams_                                           gensupport.URLParams
	ctx_                                                 context.Context
	header_                                              http.Header
}

// Annotate: Run image detection and annotation for a batch of images.
func (r *ImagesService) Annotate(googlecloudvisionv1p1beta1batchannotateimagesrequest *GoogleCloudVisionV1p1beta1BatchAnnotateImagesRequest) *ImagesAnnotateCall {
	c := &ImagesAnnotateCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.googlecloudvisionv1p1beta1batchannotateimagesrequest = googlecloudvisionv1p1beta1batchannotateimagesrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ImagesAnnotateCall) Fields(s ...googleapi.Field) *ImagesAnnotateCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ImagesAnnotateCall) Context(ctx context.Context) *ImagesAnnotateCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ImagesAnnotateCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ImagesAnnotateCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.googlecloudvisionv1p1beta1batchannotateimagesrequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1p1beta1/images:annotate")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "vision.images.annotate" call.
// Exactly one of *GoogleCloudVisionV1p1beta1BatchAnnotateImagesResponse
// or error will be non-nil. Any non-2xx status code is an error.
// Response headers are in either
// *GoogleCloudVisionV1p1beta1BatchAnnotateImagesResponse.ServerResponse.
// Header or (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *ImagesAnnotateCall) Do(opts ...googleapi.CallOption) (*GoogleCloudVisionV1p1beta1BatchAnnotateImagesResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &GoogleCloudVisionV1p1beta1BatchAnnotateImagesResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	target := &ret
	if err := gensupport.DecodeResponse(target, res); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "description": "Run image detection and annotation for a batch of images.",
	//   "flatPath": "v1p1beta1/images:annotate",
	//   "httpMethod": "POST",
	//   "id": "vision.images.annotate",
	//   "parameterOrder": [],
	//   "parameters": {},
	//   "path": "v1p1beta1/images:annotate",
	//   "request": {
	//     "$ref": "GoogleCloudVisionV1p1beta1BatchAnnotateImagesRequest"
	//   },
	//   "response": {
	//     "$ref": "GoogleCloudVisionV1p1beta1BatchAnnotateImagesResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform",
	//     "https://www.googleapis.com/auth/cloud-vision"
	//   ]
	// }

}
