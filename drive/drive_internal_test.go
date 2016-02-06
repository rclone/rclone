package drive

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/api/drive/v2"
)

func TestInternalParseExtensions(t *testing.T) {
	for _, test := range []struct {
		in      string
		want    []string
		wantErr error
	}{
		{"doc", []string{"doc"}, nil},
		{" docx ,XLSX, 	pptx,svg", []string{"docx", "xlsx", "pptx", "svg"}, nil},
		{"docx,svg,Docx", []string{"docx", "svg"}, nil},
		{"docx,potato,docx", []string{"docx"}, fmt.Errorf(`Couldn't find mime type for extension "potato"`)},
	} {
		f := new(Fs)
		gotErr := f.parseExtensions(test.in)
		assert.Equal(t, test.wantErr, gotErr)
		assert.Equal(t, test.want, f.extensions)
	}

	// Test it is appending
	f := new(Fs)
	assert.Nil(t, f.parseExtensions("docx,svg"))
	assert.Nil(t, f.parseExtensions("docx,svg,xlsx"))
	assert.Equal(t, []string{"docx", "svg", "xlsx"}, f.extensions)

}

func TestInternalFindExportFormat(t *testing.T) {
	item := new(drive.File)
	item.ExportLinks = map[string]string{
		"application/pdf": "http://pdf",
		"application/rtf": "http://rtf",
	}
	for _, test := range []struct {
		extensions    []string
		wantExtension string
		wantLink      string
	}{
		{[]string{}, "", ""},
		{[]string{"pdf"}, "pdf", "http://pdf"},
		{[]string{"pdf", "rtf", "xls"}, "pdf", "http://pdf"},
		{[]string{"xls", "rtf", "pdf"}, "rtf", "http://rtf"},
		{[]string{"xls", "csv", "svg"}, "", ""},
	} {
		f := new(Fs)
		f.extensions = test.extensions
		gotExtension, gotLink := f.findExportFormat("file", item)
		assert.Equal(t, test.wantExtension, gotExtension)
		assert.Equal(t, test.wantLink, gotLink)
	}
}
