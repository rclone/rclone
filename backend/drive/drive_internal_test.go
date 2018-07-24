package drive

import (
	"encoding/json"
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
}

func TestInternalParseExtensions(t *testing.T) {
	for _, test := range []struct {
		in      string
		want    []string
		wantErr error
	}{
		{"doc", []string{"doc"}, nil},
		{" docx ,XLSX, 	pptx,svg", []string{"docx", "xlsx", "pptx", "svg"}, nil},
		{"docx,svg,Docx", []string{"docx", "svg"}, nil},
		{"docx,potato,docx", []string{"docx"}, errors.New(`couldn't find mime type for extension "potato"`)},
	} {
		f := new(Fs)
		gotErr := f.parseExtensions(test.in)
		if test.wantErr == nil {
			assert.NoError(t, gotErr)
		} else {
			assert.EqualError(t, gotErr, test.wantErr.Error())
		}
		assert.Equal(t, test.want, f.extensions)
	}

	// Test it is appending
	f := new(Fs)
	assert.Nil(t, f.parseExtensions("docx,svg"))
	assert.Nil(t, f.parseExtensions("docx,svg,xlsx"))
	assert.Equal(t, []string{"docx", "svg", "xlsx"}, f.extensions)

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
		{[]string{"pdf"}, "pdf", "application/pdf"},
		{[]string{"pdf", "rtf", "xls"}, "pdf", "application/pdf"},
		{[]string{"xls", "rtf", "pdf"}, "rtf", "application/rtf"},
		{[]string{"xls", "csv", "svg"}, "", ""},
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
