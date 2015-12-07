package src

import "fmt"

// PreviewSize struct
type PreviewSize struct {
	size string
}

// PredefinedSizeS - set preview size
func (s *PreviewSize) PredefinedSizeS() *PreviewSize {
	return &PreviewSize{
		size: "S",
	}
}

// PredefinedSizeM - set preview size
func (s *PreviewSize) PredefinedSizeM() *PreviewSize {
	return &PreviewSize{
		size: "M",
	}
}

// PredefinedSizeL - set preview size
func (s *PreviewSize) PredefinedSizeL() *PreviewSize {
	return &PreviewSize{
		size: "L",
	}
}

// PredefinedSizeXL - set preview size
func (s *PreviewSize) PredefinedSizeXL() *PreviewSize {
	return &PreviewSize{
		size: "XL",
	}
}

// PredefinedSizeXXL - set preview size
func (s *PreviewSize) PredefinedSizeXXL() *PreviewSize {
	return &PreviewSize{
		size: "XXL",
	}
}

// PredefinedSizeXXXL - set preview size
func (s *PreviewSize) PredefinedSizeXXXL() *PreviewSize {
	return &PreviewSize{
		size: "XXXL",
	}
}

// ExactWidth - set preview size
func (s *PreviewSize) ExactWidth(width uint32) *PreviewSize {
	return &PreviewSize{
		size: fmt.Sprintf("%dx", width),
	}
}

// ExactHeight - set preview size
func (s *PreviewSize) ExactHeight(height uint32) *PreviewSize {
	return &PreviewSize{
		size: fmt.Sprintf("x%d", height),
	}
}

// ExactSize - set preview size
func (s *PreviewSize) ExactSize(width uint32, height uint32) *PreviewSize {
	return &PreviewSize{
		size: fmt.Sprintf("%dx%d", width, height),
	}
}

func (s *PreviewSize) String() string {
	return s.size
}
