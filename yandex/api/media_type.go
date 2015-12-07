package src

// MediaType struct - media types
type MediaType struct {
	mediaType string
}

// Audio - media type
func (m *MediaType) Audio() *MediaType {
	return &MediaType{
		mediaType: "audio",
	}
}

// Backup - media type
func (m *MediaType) Backup() *MediaType {
	return &MediaType{
		mediaType: "backup",
	}
}

// Book - media type
func (m *MediaType) Book() *MediaType {
	return &MediaType{
		mediaType: "book",
	}
}

// Compressed - media type
func (m *MediaType) Compressed() *MediaType {
	return &MediaType{
		mediaType: "compressed",
	}
}

// Data - media type
func (m *MediaType) Data() *MediaType {
	return &MediaType{
		mediaType: "data",
	}
}

// Development - media type
func (m *MediaType) Development() *MediaType {
	return &MediaType{
		mediaType: "development",
	}
}

// Diskimage - media type
func (m *MediaType) Diskimage() *MediaType {
	return &MediaType{
		mediaType: "diskimage",
	}
}

// Document - media type
func (m *MediaType) Document() *MediaType {
	return &MediaType{
		mediaType: "document",
	}
}

// Encoded - media type
func (m *MediaType) Encoded() *MediaType {
	return &MediaType{
		mediaType: "encoded",
	}
}

// Executable - media type
func (m *MediaType) Executable() *MediaType {
	return &MediaType{
		mediaType: "executable",
	}
}

// Flash - media type
func (m *MediaType) Flash() *MediaType {
	return &MediaType{
		mediaType: "flash",
	}
}

// Font - media type
func (m *MediaType) Font() *MediaType {
	return &MediaType{
		mediaType: "font",
	}
}

// Image - media type
func (m *MediaType) Image() *MediaType {
	return &MediaType{
		mediaType: "image",
	}
}

// Settings - media type
func (m *MediaType) Settings() *MediaType {
	return &MediaType{
		mediaType: "settings",
	}
}

// Spreadsheet - media type
func (m *MediaType) Spreadsheet() *MediaType {
	return &MediaType{
		mediaType: "spreadsheet",
	}
}

// Text - media type
func (m *MediaType) Text() *MediaType {
	return &MediaType{
		mediaType: "text",
	}
}

// Unknown - media type
func (m *MediaType) Unknown() *MediaType {
	return &MediaType{
		mediaType: "unknown",
	}
}

// Video - media type
func (m *MediaType) Video() *MediaType {
	return &MediaType{
		mediaType: "video",
	}
}

// Web - media type
func (m *MediaType) Web() *MediaType {
	return &MediaType{
		mediaType: "web",
	}
}

// String - media type
func (m *MediaType) String() string {
	return m.mediaType
}
