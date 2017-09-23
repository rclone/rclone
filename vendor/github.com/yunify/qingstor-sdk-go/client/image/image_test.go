package image

import (
	"testing"

	"github.com/stretchr/testify/assert"
	qs "github.com/yunify/qingstor-sdk-go/service"
)

var image *Image

func init() {
	bucket := &qs.Bucket{}

	// test.jpg is only a string
	image = Init(bucket, "test.jpg")
}

func TestQueryString(t *testing.T) {
	var param interface{}
	param = &RotateParam{
		Angle: 90,
	}
	image.setActionParam(RotateOperation, param)
	assert.Equal(t, *image.input.Action, "rotate:a_90")

	param = &CropParam{
		Width:   300,
		Height:  400,
		Gravity: 0,
	}
	image.setActionParam(CropOperation, param)
	assert.Equal(t, *image.input.Action, "rotate:a_90|crop:w_300,h_400,g_0")

	param = &ResizeParam{
		Width:  500,
		Height: 500,
		Mode:   ResizeForce,
	}
	image.setActionParam(ResizeOperation, param)
	assert.Equal(t, *image.input.Action, "rotate:a_90|crop:w_300,h_400,g_0|resize:w_500,h_500,m_1")

	param = &FormatParam{
		Type: "png",
	}
	image.setActionParam(FormatOperation, param)
	assert.Equal(t, *image.input.Action, "rotate:a_90|crop:w_300,h_400,g_0|resize:w_500,h_500,m_1|format:t_png")

	param = &WaterMarkParam{
		Text: "5rC05Y2w5paH5a2X",
	}
	image.setActionParam(WaterMarkOperation, param)
	assert.Equal(t, *image.input.Action, "rotate:a_90|crop:w_300,h_400,g_0|resize:w_500,h_500,m_1|format:t_png|watermark:t_5rC05Y2w5paH5a2X,c_")

	param = &WaterMarkImageParam{
		URL: "aHR0cHM6Ly9wZWszYS5xaW5nc3Rvci5jb20vaW1nLWRvYy1lZy9xaW5jbG91ZC5wbmc",
	}
	image.setActionParam(WaterMarkImageOperation, param)
	assert.Equal(t, *image.input.Action, "rotate:a_90|crop:w_300,h_400,g_0|resize:w_500,h_500,m_1|format:t_png|watermark:t_5rC05Y2w5paH5a2X,c_|watermark_image:l_0,t_0,u_aHR0cHM6Ly9wZWszYS5xaW5nc3Rvci5jb20vaW1nLWRvYy1lZy9xaW5jbG91ZC5wbmc")

	image.setActionParam(InfoOperation, nil)
	assert.Equal(t, *image.input.Action, "rotate:a_90|crop:w_300,h_400,g_0|resize:w_500,h_500,m_1|format:t_png|watermark:t_5rC05Y2w5paH5a2X,c_|watermark_image:l_0,t_0,u_aHR0cHM6Ly9wZWszYS5xaW5nc3Rvci5jb20vaW1nLWRvYy1lZy9xaW5jbG91ZC5wbmc|info")
}
