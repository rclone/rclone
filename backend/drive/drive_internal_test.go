package drive

import (
	"encoding/json"
	"mime"
	"testing"

	"google.golang.org/api/drive/v3"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

const exampleExportFormats = `{
	"application/vnd.google-apps.document": [
		"application/rtf",
		"application/vnd.oasis.opendocument.text",
		"text/html",
		"application/pdf",
		"application/epub+zip",
		"application/zip",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"text/plain"
	],
	"application/vnd.google-apps.spreadsheet": [
		"application/x-vnd.oasis.opendocument.spreadsheet",
		"text/tab-separated-values",
		"application/pdf",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"text/csv",
		"application/zip",
		"application/vnd.oasis.opendocument.spreadsheet"
	],
	"application/vnd.google-apps.jam": [
		"application/pdf"
	],
	"application/vnd.google-apps.script": [
		"application/vnd.google-apps.script+json"
	],
	"application/vnd.google-apps.presentation": [
		"application/vnd.oasis.opendocument.presentation",
		"application/pdf",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"text/plain"
	],
	"application/vnd.google-apps.form": [
		"application/zip"
	],
	"application/vnd.google-apps.drawing": [
		"image/svg+xml",
		"image/png",
		"application/pdf",
		"image/jpeg"
	]
}`

// Load the example export formats into exportFormats for testing
func TestInternalLoadExampleExportFormats(t *testing.T) {
	exportFormatsOnce.Do(func() {})
	assert.NoError(t, json.Unmarshal([]byte(exampleExportFormats), &_exportFormats))
	_exportFormats = fixMimeTypeMap(_exportFormats)
}

func TestInternalParseExtensions(t *testing.T) {
	for _, test := range []struct {
		in      string
		want    []string
		wantErr error
	}{
		{"doc", []string{".doc"}, nil},
		{" docx ,XLSX, 	pptx,svg", []string{".docx", ".xlsx", ".pptx", ".svg"}, nil},
		{"docx,svg,Docx", []string{".docx", ".svg"}, nil},
		{"docx,potato,docx", []string{".docx"}, errors.New(`couldn't find MIME type for extension ".potato"`)},
	} {
		extensions, gotErr := parseExtensions(test.in)
		if test.wantErr == nil {
			assert.NoError(t, gotErr)
		} else {
			assert.EqualError(t, gotErr, test.wantErr.Error())
		}
		assert.Equal(t, test.want, extensions)
	}

	// Test it is appending
	extensions, gotErr := parseExtensions("docx,svg", "docx,svg,xlsx")
	assert.NoError(t, gotErr)
	assert.Equal(t, []string{".docx", ".svg", ".xlsx"}, extensions)
}

func TestInternalFindExportFormat(t *testing.T) {
	item := &drive.File{
		Name:     "file",
		MimeType: "application/vnd.google-apps.document",
	}
	for _, test := range []struct {
		extensions    []string
		wantExtension string
		wantMimeType  string
	}{
		{[]string{}, "", ""},
		{[]string{".pdf"}, ".pdf", "application/pdf"},
		{[]string{".pdf", ".rtf", ".xls"}, ".pdf", "application/pdf"},
		{[]string{".xls", ".rtf", ".pdf"}, ".rtf", "application/rtf"},
		{[]string{".xls", ".csv", ".svg"}, "", ""},
	} {
		f := new(Fs)
		f.extensions = test.extensions
		gotExtension, gotFilename, gotMimeType, gotIsDocument := f.findExportFormat(item)
		assert.Equal(t, test.wantExtension, gotExtension)
		if test.wantExtension != "" {
			assert.Equal(t, item.Name+"."+gotExtension, gotFilename)
		} else {
			assert.Equal(t, "", gotFilename)
		}
		assert.Equal(t, test.wantMimeType, gotMimeType)
		assert.Equal(t, true, gotIsDocument)
	}
}

func TestMimeTypesToExtension(t *testing.T) {
	for mimeType, extension := range _mimeTypeToExtension {
		extensions, err := mime.ExtensionsByType(mimeType)
		assert.NoError(t, err)
		assert.Contains(t, extensions, extension)
	}
}

func TestExtensionToMimeType(t *testing.T) {
	for mimeType, extension := range _mimeTypeToExtension {
		gotMimeType := mime.TypeByExtension(extension)
		mediatype, _, err := mime.ParseMediaType(gotMimeType)
		assert.NoError(t, err)
		assert.Equal(t, mimeType, mediatype)
	}
}

func TestExtensionsForExportFormats(t *testing.T) {
	if _exportFormats == nil {
		t.Error("exportFormats == nil")
	}
	for fromMT, toMTs := range _exportFormats {
		for _, toMT := range toMTs {
			if !isInternalMimeType(toMT) {
				extensions, err := mime.ExtensionsByType(toMT)
				assert.NoError(t, err, "invalid MIME type %q", toMT)
				assert.NotEmpty(t, extensions, "No extension found for %q (from: %q)", fromMT, toMT)
			}
		}
	}
}
