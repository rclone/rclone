package drive

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"mime"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkg/errors"
	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/drive/v3"
)

func TestDriveScopes(t *testing.T) {
	for _, test := range []struct {
		in       string
		want     []string
		wantFlag bool
	}{
		{"", []string{
			"https://www.googleapis.com/auth/drive",
		}, false},
		{" drive.file , drive.readonly", []string{
			"https://www.googleapis.com/auth/drive.file",
			"https://www.googleapis.com/auth/drive.readonly",
		}, false},
		{" drive.file , drive.appfolder", []string{
			"https://www.googleapis.com/auth/drive.file",
			"https://www.googleapis.com/auth/drive.appfolder",
		}, true},
	} {
		got := driveScopes(test.in)
		assert.Equal(t, test.want, got, test.in)
		gotFlag := driveScopesContainsAppFolder(got)
		assert.Equal(t, test.wantFlag, gotFlag, test.in)
	}
}

/*
var additionalMimeTypes = map[string]string{
	"application/vnd.ms-excel.sheet.macroenabled.12":                          ".xlsm",
	"application/vnd.ms-excel.template.macroenabled.12":                       ".xltm",
	"application/vnd.ms-powerpoint.presentation.macroenabled.12":              ".pptm",
	"application/vnd.ms-powerpoint.slideshow.macroenabled.12":                 ".ppsm",
	"application/vnd.ms-powerpoint.template.macroenabled.12":                  ".potm",
	"application/vnd.ms-powerpoint":                                           ".ppt",
	"application/vnd.ms-word.document.macroenabled.12":                        ".docm",
	"application/vnd.ms-word.template.macroenabled.12":                        ".dotm",
	"application/vnd.openxmlformats-officedocument.presentationml.template":   ".potx",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.template":    ".xltx",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.template": ".dotx",
	"application/vnd.sun.xml.writer":                                          ".sxw",
	"text/richtext":                                                           ".rtf",
}
*/

// Load the example export formats into exportFormats for testing
func TestInternalLoadExampleFormats(t *testing.T) {
	fetchFormatsOnce.Do(func() {})
	buf, err := ioutil.ReadFile(filepath.FromSlash("test/about.json"))
	var about struct {
		ExportFormats map[string][]string `json:"exportFormats,omitempty"`
		ImportFormats map[string][]string `json:"importFormats,omitempty"`
	}
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(buf, &about))
	_exportFormats = fixMimeTypeMap(about.ExportFormats)
	_importFormats = fixMimeTypeMap(about.ImportFormats)
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
		extensions, _, gotErr := parseExtensions(test.in)
		if test.wantErr == nil {
			assert.NoError(t, gotErr)
		} else {
			assert.EqualError(t, gotErr, test.wantErr.Error())
		}
		assert.Equal(t, test.want, extensions)
	}

	// Test it is appending
	extensions, _, gotErr := parseExtensions("docx,svg", "docx,svg,xlsx")
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
		f.exportExtensions = test.extensions
		gotExtension, gotFilename, gotMimeType, gotIsDocument := f.findExportFormat(item)
		assert.Equal(t, test.wantExtension, gotExtension)
		if test.wantExtension != "" {
			assert.Equal(t, item.Name+gotExtension, gotFilename)
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

func TestExtensionsForImportFormats(t *testing.T) {
	t.Skip()
	if _importFormats == nil {
		t.Error("_importFormats == nil")
	}
	for fromMT := range _importFormats {
		if !isInternalMimeType(fromMT) {
			extensions, err := mime.ExtensionsByType(fromMT)
			assert.NoError(t, err, "invalid MIME type %q", fromMT)
			assert.NotEmpty(t, extensions, "No extension found for %q", fromMT)
		}
	}
}

func (f *Fs) InternalTestDocumentImport(t *testing.T) {
	oldAllow := f.opt.AllowImportNameChange
	f.opt.AllowImportNameChange = true
	defer func() {
		f.opt.AllowImportNameChange = oldAllow
	}()

	testFilesPath, err := filepath.Abs(filepath.FromSlash("test/files"))
	require.NoError(t, err)

	testFilesFs, err := fs.NewFs(testFilesPath)
	require.NoError(t, err)

	_, f.importMimeTypes, err = parseExtensions("odt,ods,doc")
	require.NoError(t, err)

	err = operations.CopyFile(context.Background(), f, testFilesFs, "example2.doc", "example2.doc")
	require.NoError(t, err)
}

func (f *Fs) InternalTestDocumentUpdate(t *testing.T) {
	testFilesPath, err := filepath.Abs(filepath.FromSlash("test/files"))
	require.NoError(t, err)

	testFilesFs, err := fs.NewFs(testFilesPath)
	require.NoError(t, err)

	_, f.importMimeTypes, err = parseExtensions("odt,ods,doc")
	require.NoError(t, err)

	err = operations.CopyFile(context.Background(), f, testFilesFs, "example2.xlsx", "example1.ods")
	require.NoError(t, err)
}

func (f *Fs) InternalTestDocumentExport(t *testing.T) {
	var buf bytes.Buffer
	var err error

	f.exportExtensions, _, err = parseExtensions("txt")
	require.NoError(t, err)

	obj, err := f.NewObject(context.Background(), "example2.txt")
	require.NoError(t, err)

	rc, err := obj.Open(context.Background())
	require.NoError(t, err)
	defer func() { require.NoError(t, rc.Close()) }()

	_, err = io.Copy(&buf, rc)
	require.NoError(t, err)
	text := buf.String()

	for _, excerpt := range []string{
		"Lorem ipsum dolor sit amet, consectetur",
		"porta at ultrices in, consectetur at augue.",
	} {
		require.Contains(t, text, excerpt)
	}
}

func (f *Fs) InternalTestDocumentLink(t *testing.T) {
	var buf bytes.Buffer
	var err error

	f.exportExtensions, _, err = parseExtensions("link.html")
	require.NoError(t, err)

	obj, err := f.NewObject(context.Background(), "example2.link.html")
	require.NoError(t, err)

	rc, err := obj.Open(context.Background())
	require.NoError(t, err)
	defer func() { require.NoError(t, rc.Close()) }()

	_, err = io.Copy(&buf, rc)
	require.NoError(t, err)
	text := buf.String()

	require.True(t, strings.HasPrefix(text, "<html>"))
	require.True(t, strings.HasSuffix(text, "</html>\n"))
	for _, excerpt := range []string{
		`<meta http-equiv="refresh"`,
		`Loading <a href="`,
	} {
		require.Contains(t, text, excerpt)
	}
}

// TestIntegration/FsMkdir/FsPutFiles/Internal/Shortcuts
func (f *Fs) InternalTestShortcuts(t *testing.T) {
	const (
		// from fstest/fstests/fstests.go
		existingDir    = "hello? sausage"
		existingFile   = `hello? sausage/êé/Hello, 世界/ " ' @ < > & ? + ≠/z.txt`
		existingSubDir = "êé"
	)
	ctx := context.Background()
	srcObj, err := f.NewObject(ctx, existingFile)
	require.NoError(t, err)
	srcHash, err := srcObj.Hash(ctx, hash.MD5)
	require.NoError(t, err)
	assert.NotEqual(t, "", srcHash)
	t.Run("Errors", func(t *testing.T) {
		_, err := f.makeShortcut(ctx, "", f, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "can't be root")

		_, err = f.makeShortcut(ctx, "notfound", f, "dst")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "can't find source")

		_, err = f.makeShortcut(ctx, existingFile, f, existingFile)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not overwriting")
		assert.Contains(t, err.Error(), "existing file")

		_, err = f.makeShortcut(ctx, existingFile, f, existingDir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not overwriting")
		assert.Contains(t, err.Error(), "existing directory")
	})
	t.Run("File", func(t *testing.T) {
		dstObj, err := f.makeShortcut(ctx, existingFile, f, "shortcut.txt")
		require.NoError(t, err)
		require.NotNil(t, dstObj)
		assert.Equal(t, "shortcut.txt", dstObj.Remote())
		dstHash, err := dstObj.Hash(ctx, hash.MD5)
		require.NoError(t, err)
		assert.Equal(t, srcHash, dstHash)
		require.NoError(t, dstObj.Remove(ctx))
	})
	t.Run("Dir", func(t *testing.T) {
		dstObj, err := f.makeShortcut(ctx, existingDir, f, "shortcutdir")
		require.NoError(t, err)
		require.Nil(t, dstObj)
		entries, err := f.List(ctx, "shortcutdir")
		require.NoError(t, err)
		require.Equal(t, 1, len(entries))
		require.Equal(t, "shortcutdir/"+existingSubDir, entries[0].Remote())
		require.NoError(t, f.Rmdir(ctx, "shortcutdir"))
	})
	t.Run("Command", func(t *testing.T) {
		_, err := f.Command(ctx, "shortcut", []string{"one"}, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "need exactly 2 arguments")

		_, err = f.Command(ctx, "shortcut", []string{"one", "two"}, map[string]string{
			"target": "doesnotexistremote:",
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "couldn't find target")

		_, err = f.Command(ctx, "shortcut", []string{"one", "two"}, map[string]string{
			"target": ".",
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "target is not a drive backend")

		dstObjI, err := f.Command(ctx, "shortcut", []string{existingFile, "shortcut2.txt"}, map[string]string{
			"target": fs.ConfigString(f),
		})
		require.NoError(t, err)
		dstObj := dstObjI.(*Object)
		assert.Equal(t, "shortcut2.txt", dstObj.Remote())
		dstHash, err := dstObj.Hash(ctx, hash.MD5)
		require.NoError(t, err)
		assert.Equal(t, srcHash, dstHash)
		require.NoError(t, dstObj.Remove(ctx))

		dstObjI, err = f.Command(ctx, "shortcut", []string{existingFile, "shortcut3.txt"}, nil)
		require.NoError(t, err)
		dstObj = dstObjI.(*Object)
		assert.Equal(t, "shortcut3.txt", dstObj.Remote())
		dstHash, err = dstObj.Hash(ctx, hash.MD5)
		require.NoError(t, err)
		assert.Equal(t, srcHash, dstHash)
		require.NoError(t, dstObj.Remove(ctx))
	})
}

func (f *Fs) InternalTest(t *testing.T) {
	// These tests all depend on each other so run them as nested tests
	t.Run("DocumentImport", func(t *testing.T) {
		f.InternalTestDocumentImport(t)
		t.Run("DocumentUpdate", func(t *testing.T) {
			f.InternalTestDocumentUpdate(t)
			t.Run("DocumentExport", func(t *testing.T) {
				f.InternalTestDocumentExport(t)
				t.Run("DocumentLink", func(t *testing.T) {
					f.InternalTestDocumentLink(t)
				})
			})
		})
	})
	t.Run("Shortcuts", f.InternalTestShortcuts)
}

var _ fstests.InternalTester = (*Fs)(nil)
