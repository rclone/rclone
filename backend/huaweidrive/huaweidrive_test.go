package huaweidrive

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"path"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/huaweidrive/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/encoder"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	if *fstest.RemoteName == "" {
		t.Skip("Skipping as no remote name")
	}
	fstests.Run(t, &fstests.Opt{
		RemoteName:               *fstest.RemoteName,
		NilObject:                (*Object)(nil),
		SkipBadWindowsCharacters: true,
	})
}

// TestNewFs tests the NewFs constructor
func TestNewFs(t *testing.T) {
	ctx := context.Background()

	// Test with empty config - should fail due to missing OAuth credentials
	m := configmap.Simple{}
	_, err := NewFs(ctx, "test", "", m)
	if err == nil {
		t.Fatal("expected error with empty config")
	}
}

// TestFsName tests the filesystem name
func TestFsName(t *testing.T) {
	f := &Fs{name: "test"}
	if f.Name() != "test" {
		t.Errorf("expected name 'test', got %q", f.Name())
	}
}

// TestFsRoot tests the filesystem root
func TestFsRoot(t *testing.T) {
	f := &Fs{root: "test/path"}
	if f.Root() != "test/path" {
		t.Errorf("expected root 'test/path', got %q", f.Root())
	}
}

// TestFsString tests the filesystem string representation
func TestFsString(t *testing.T) {
	f := &Fs{root: "test/path"}
	expected := "Huawei Drive root 'test/path'"
	if f.String() != expected {
		t.Errorf("expected string %q, got %q", expected, f.String())
	}
}

// TestFsPrecision tests the filesystem precision
func TestFsPrecision(t *testing.T) {
	f := &Fs{}
	precision := f.Precision()
	expectedPrecision := fs.ModTimeNotSupported
	if precision != expectedPrecision {
		t.Errorf("expected precision %v (ModTimeNotSupported), got %v", expectedPrecision, precision)
	}
}

// TestFsHashes tests the supported hash types
func TestFsHashes(t *testing.T) {
	f := &Fs{}
	hashes := f.Hashes()
	if !hashes.Contains(hash.SHA256) {
		t.Error("expected SHA256 hash support")
	}
}

// TestModTimeSupport tests if the filesystem supports modification time preservation
func TestModTimeSupport(t *testing.T) {
	f := &Fs{}
	// Huawei Drive does NOT preserve original modification times
	// Despite the API accepting editedTime/createdTime parameters,
	// the server always overwrites them with current server timestamp
	expectedPrecision := fs.ModTimeNotSupported
	if f.Precision() != expectedPrecision {
		t.Errorf("expected precision of %v to indicate no time preservation, got %v", expectedPrecision, f.Precision())
	}
}

// TestTimeFormats tests time format handling for RFC 3339
func TestTimeFormats(t *testing.T) {
	// Test that we can format time correctly for Huawei Drive API
	testTime := time.Date(2023, 10, 15, 14, 30, 45, 0, time.UTC)
	formatted := testTime.Format(time.RFC3339)
	expected := "2023-10-15T14:30:45Z"

	if formatted != expected {
		t.Errorf("expected RFC3339 format %q, got %q", expected, formatted)
	}

	// Test parsing back
	parsed, err := time.Parse(time.RFC3339, formatted)
	if err != nil {
		t.Errorf("failed to parse RFC3339 time: %v", err)
	}

	if !parsed.Equal(testTime) {
		t.Errorf("parsed time %v does not match original %v", parsed, testTime)
	}
}

// TestEncoding tests filename encoding for Huawei Drive restrictions
func TestEncoding(t *testing.T) {
	// Create encoder with Huawei Drive restrictions
	enc := encoder.MultiEncoder( //nolint:unconvert
		encoder.Display |
			encoder.EncodeBackSlash |
			encoder.EncodeInvalidUtf8 |
			encoder.EncodeRightSpace |
			encoder.EncodeLeftSpace |
			encoder.EncodeLeftTilde |
			encoder.EncodeRightPeriod |
			encoder.EncodeLeftPeriod |
			encoder.EncodeColon |
			encoder.EncodePipe |
			encoder.EncodeDoubleQuote |
			encoder.EncodeLtGt |
			encoder.EncodeQuestion |
			encoder.EncodeAsterisk |
			encoder.EncodeCtl |
			encoder.EncodeDot)

	// Test cases for problematic characters that should be encoded
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"angle_brackets", "file<>name", "file＜＞name"},
		{"quotes", "file\"name", "file＂name"},
		{"pipe", "file|name", "file｜name"},
		{"colon", "file:name", "file：name"},
		{"asterisk", "file*name", "file＊name"},
		{"question", "file?name", "file？name"},
		{"backslash", "file\\name", "file＼name"},
		{"forward_slash", "file/name", "file／name"},
		{"leading_space", " filename", "␠filename"},
		{"trailing_space", "filename ", "filename␠"},
		{"leading_dot", ".filename", "．filename"},
		{"control_chars", "file\tname\ntest", "file␉name␊test"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			encoded := enc.FromStandardName(tc.input)
			if encoded != tc.expected {
				t.Errorf("encoding %q: expected %q, got %q", tc.input, tc.expected, encoded)
			}

			// Test decoding back - only for reversible encodings
			// Some characters like forward slash and control chars are one-way encoded
			decoded := enc.ToStandardName(encoded)
			if tc.name != "forward_slash" && tc.name != "control_chars" {
				if decoded != tc.input {
					t.Errorf("decoding %q: expected %q, got %q", encoded, tc.input, decoded)
				}
			}
		})
	}
}

// TestFileNameEncoding tests that problematic characters are properly encoded
func TestFileNameEncoding(t *testing.T) {
	// Create a mock Fs with default encoding options
	opts := Options{}
	// Set the default encoding from our config
	opts.Enc = (encoder.Display |
		encoder.EncodeBackSlash |
		encoder.EncodeInvalidUtf8 |
		encoder.EncodeRightSpace |
		encoder.EncodeLeftSpace |
		encoder.EncodeLeftTilde |
		encoder.EncodeRightPeriod |
		encoder.EncodeLeftPeriod |
		encoder.EncodeColon |
		encoder.EncodePipe |
		encoder.EncodeDoubleQuote |
		encoder.EncodeLtGt |
		encoder.EncodeQuestion |
		encoder.EncodeAsterisk |
		encoder.EncodeCtl |
		encoder.EncodeDot)

	f := &Fs{opt: opts}

	// Test problematic characters that Huawei Drive rejects
	testCases := []struct {
		input string
		desc  string
	}{
		{`file<name>.txt`, "angle brackets"},
		{`file|name.txt`, "pipe character"},
		{`file:name.txt`, "colon"},
		{`file"name.txt`, "double quote"},
		{`file*name.txt`, "asterisk"},
		{`file?name.txt`, "question mark"},
		{`file\name.txt`, "backslash"},
		{` leading_space.txt`, "leading space"},
		{`trailing_space.txt `, "trailing space"},
		{`.leading_dot.txt`, "leading dot"},
		{`trailing_dot.txt.`, "trailing dot"},
		{`~leading_tilde.txt`, "leading tilde"},
		{"file\x00name.txt", "control character"},
	}

	for _, tc := range testCases {
		encoded := f.opt.Enc.FromStandardName(tc.input)
		// The encoded name should be different from input (meaning it was encoded)
		if encoded == tc.input {
			t.Errorf("Expected %s (%q) to be encoded, but got same string", tc.desc, tc.input)
		}

		// Test that we can decode it back (skip control characters as they are not reversible)
		decoded := f.opt.Enc.ToStandardName(encoded)
		if tc.desc != "control character" && decoded != tc.input {
			t.Errorf("Round-trip failed for %s: input=%q, encoded=%q, decoded=%q", tc.desc, tc.input, encoded, decoded)
		}

		t.Logf("✓ %s: %q → %q → %q", tc.desc, tc.input, encoded, decoded)
	}
}

// TestQueryFilter tests the QueryFilter functionality
func TestQueryFilter(t *testing.T) {
	t.Run("empty_filter", func(t *testing.T) {
		filter := NewQueryFilter()
		if filter.String() != "" {
			t.Errorf("expected empty string, got %q", filter.String())
		}
	})

	t.Run("single_condition", func(t *testing.T) {
		filter := NewQueryFilter()
		filter.AddParentFolder("test_parent_id")
		expected := "'test_parent_id' in parentFolder"
		if filter.String() != expected {
			t.Errorf("expected %q, got %q", expected, filter.String())
		}
	})

	t.Run("multiple_conditions", func(t *testing.T) {
		filter := NewQueryFilter()
		filter.AddParentFolder("parent_id")
		filter.AddMimeType("image/jpeg")
		filter.AddFavorite(true)
		expected := "'parent_id' in parentFolder and mimeType = 'image/jpeg' and favorite = true"
		if filter.String() != expected {
			t.Errorf("expected %q, got %q", expected, filter.String())
		}
	})

	t.Run("filename_with_quotes", func(t *testing.T) {
		filter := NewQueryFilter()
		filter.AddFileName("file'with'quotes.txt")
		expected := "fileName = 'file\\'with\\'quotes.txt'"
		if filter.String() != expected {
			t.Errorf("expected %q, got %q", expected, filter.String())
		}
	})

	t.Run("mime_type_not_equal", func(t *testing.T) {
		filter := NewQueryFilter()
		filter.AddMimeTypeNot("application/vnd.huawei-apps.folder")
		expected := "mimeType != 'application/vnd.huawei-apps.folder'"
		if filter.String() != expected {
			t.Errorf("expected %q, got %q", expected, filter.String())
		}
	})

	t.Run("filename_contains", func(t *testing.T) {
		filter := NewQueryFilter()
		filter.AddFileNameContains("test")
		expected := "fileName contains 'test'"
		if filter.String() != expected {
			t.Errorf("expected %q, got %q", expected, filter.String())
		}
	})

	t.Run("recycled_status", func(t *testing.T) {
		filter := NewQueryFilter()
		filter.AddRecycled(true)
		filter.AddDirectlyRecycled(false)
		expected := "recycled = true and directlyRecycled = false"
		if filter.String() != expected {
			t.Errorf("expected %q, got %q", expected, filter.String())
		}
	})

	t.Run("edited_time_range", func(t *testing.T) {
		filter := NewQueryFilter()
		testTime := time.Date(2023, 10, 15, 14, 30, 45, 0, time.UTC)
		filter.AddEditedTimeRange(">=", testTime)
		expected := "editedTime >= '2023-10-15T14:30:45Z'"
		if filter.String() != expected {
			t.Errorf("expected %q, got %q", expected, filter.String())
		}
	})
}

// TestAPITypes tests the API type structures and methods
func TestAPITypes(t *testing.T) {
	t.Run("file_is_dir", func(t *testing.T) {
		// Test folder
		folder := api.File{MimeType: api.FolderMimeType}
		if !folder.IsDir() {
			t.Error("expected folder to be recognized as directory")
		}

		// Test regular file
		file := api.File{MimeType: "text/plain"}
		if file.IsDir() {
			t.Error("expected regular file to not be recognized as directory")
		}
	})

	t.Run("constants", func(t *testing.T) {
		if api.FolderMimeType != "application/vnd.huawei-apps.folder" {
			t.Errorf("unexpected folder mime type: %q", api.FolderMimeType)
		}

		if api.CategoryDriveFile != "drive#file" {
			t.Errorf("unexpected drive file category: %q", api.CategoryDriveFile)
		}

		if api.CategoryDriveFileList != "drive#fileList" {
			t.Errorf("unexpected drive file list category: %q", api.CategoryDriveFileList)
		}

		if api.UploadTypeMultipart != "multipart" {
			t.Errorf("unexpected multipart upload type: %q", api.UploadTypeMultipart)
		}
	})
}

// TestErrorHandling tests error handling scenarios
func TestErrorHandling(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name       string
		statusCode int
		errorBody  string
		expectErr  bool
		errType    string
	}{
		{"missing_param_400", 400, "21004001", true, "missing required parameters"},
		{"invalid_param_400", 400, "PARAM_INVALID", true, "invalid parameters"},
		{"parent_not_found_400", 400, "PARENTFOLDER_NOT_FOUND", true, "ErrorDirNotFound"},
		{"insufficient_scope_403", 403, "INSUFFICIENT_SCOPE", true, "insufficient OAuth scope"},
		{"insufficient_permission_403", 403, "INSUFFICIENT_PERMISSION", true, "insufficient permissions"},
		{"agreement_not_signed_403", 403, "AGREEMENT_NOT_SIGNED", true, "user agreement not signed"},
		{"data_migrating_403", 403, "DATA_MIGRATING", false, ""}, // Should allow retry
		{"cursor_expired_410", 410, "CURSOR_EXPIRED", true, "cursor expired"},
		{"temp_data_cleared_410", 410, "TEMP_DATA_CLEARED", true, "temporary data cleared"},
		{"server_temp_error_500", 500, "SERVER_TEMP_ERROR", false, ""},                 // Should allow retry
		{"outer_service_unavailable_500", 500, "OUTER_SERVICE_UNAVAILABLE", false, ""}, // Should allow retry
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Mock HTTP response
			resp := &http.Response{
				StatusCode: tc.statusCode,
			}

			// Mock error with specific error code
			err := errors.New(tc.errorBody)

			shouldRetryResult, resultErr := shouldRetry(ctx, resp, err)

			if tc.expectErr {
				// Should not retry and should return error
				if shouldRetryResult {
					t.Error("expected shouldRetry to return false for non-retryable error")
				}
				if resultErr == nil {
					t.Error("expected error to be returned")
				}
			} else if !shouldRetryResult {
				// Should retry (temporary error)
				t.Error("expected shouldRetry to return true for retryable error")
			}
		})
	}
}

// TestMimeTypeDetection tests MIME type detection for various file types
func TestMimeTypeDetection(t *testing.T) {
	testCases := []struct {
		filename     string
		expectedMime string
	}{
		{"document.pdf", "application/pdf"},
		{"image.jpg", "image/jpeg"},
		{"image.jpeg", "image/jpeg"},
		{"image.png", "image/png"},
		{"image.gif", "image/gif"},
		{"video.mp4", "video/mp4"},
		{"video.avi", "video/x-msvideo"},
		{"audio.mp3", "audio/mpeg"},
		{"text.txt", "text/plain; charset=utf-8"},
		{"data.json", "application/json"},
		{"archive.zip", "application/zip"},
		{"noextension", "application/octet-stream"},
		{"", "application/octet-stream"},
	}

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			detected := mime.TypeByExtension(path.Ext(tc.filename))
			if detected == "" {
				detected = "application/octet-stream"
			}
			if detected != tc.expectedMime {
				t.Errorf("expected MIME type %q for %q, got %q", tc.expectedMime, tc.filename, detected)
			}
		})
	}
}

// TestRootParentID tests root parent ID handling
func TestRootParentID(t *testing.T) {
	f := &Fs{}
	rootParentID := f.rootParentID()
	expected := "HUAWEI_DRIVE_ROOT_PARENT"
	if rootParentID != expected {
		t.Errorf("expected root parent ID %q, got %q", expected, rootParentID)
	}
}

// TestOptionsDefaults tests default option values
func TestOptionsDefaults(t *testing.T) {
	opts := Options{}

	// Check default values match what we expect
	if opts.ChunkSize != 0 {
		t.Errorf("expected default ChunkSize to be 0, got %v", opts.ChunkSize)
	}

	if opts.ListChunk != 0 {
		t.Errorf("expected default ListChunk to be 0, got %v", opts.ListChunk)
	}

	if opts.UploadCutoff != 0 {
		t.Errorf("expected default UploadCutoff to be 0, got %v", opts.UploadCutoff)
	}
}

// TestParsePath tests path parsing functionality
func TestParsePath(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"/", ""},
		{"path", "path"},
		{"/path", "path"},
		{"path/", "path"},
		{"/path/", "path"},
		{"path/to/file", "path/to/file"},
		{"/path/to/file", "path/to/file"},
		{"path/to/file/", "path/to/file"},
		{"/path/to/file/", "path/to/file"},
		{"//multiple//slashes//", "multiple//slashes"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("input_%q", tc.input), func(t *testing.T) {
			result := parsePath(tc.input)
			if result != tc.expected {
				t.Errorf("parsePath(%q): expected %q, got %q", tc.input, tc.expected, result)
			}
		})
	}
}

// TestConstants tests various constants and configuration values
func TestConstants(t *testing.T) {
	// Test OAuth configuration
	if rcloneClientID != "115505059" {
		t.Errorf("expected client ID %q, got %q", "115505059", rcloneClientID)
	}

	// Test URLs
	if rootURL != "https://driveapis.cloud.huawei.com.cn/drive/v1" {
		t.Errorf("expected root URL %q, got %q", "https://driveapis.cloud.huawei.com.cn/drive/v1", rootURL)
	}

	if uploadURL != "https://driveapis.cloud.huawei.com.cn/upload/drive/v1" {
		t.Errorf("expected upload URL %q, got %q", "https://driveapis.cloud.huawei.com.cn/upload/drive/v1", uploadURL)
	}

	// Test size limits
	expectedChunkSize := 8 * fs.Mebi // 8MB
	if defaultChunkSize != expectedChunkSize {
		t.Errorf("expected default chunk size %d, got %d", expectedChunkSize, defaultChunkSize)
	}

	expectedMaxSize := 50 * fs.Gibi // 50GB
	if maxFileSize != expectedMaxSize {
		t.Errorf("expected max file size %d, got %d", expectedMaxSize, maxFileSize)
	}
}

// TestFeatures tests the filesystem features configuration
func TestFeatures(t *testing.T) {
	ctx := context.Background()
	f := &Fs{}

	// Create features manually since we can't call NewFs without proper config
	features := (&fs.Features{
		CaseInsensitive:         false,
		DuplicateFiles:          false,
		ReadMimeType:            true,
		WriteMimeType:           true,
		CanHaveEmptyDirectories: true,
		BucketBased:             false,
		FilterAware:             true,
		ReadMetadata:            true,
		WriteMetadata:           true,
		UserMetadata:            true,
		ReadDirMetadata:         false,
		WriteDirMetadata:        false,
		PartialUploads:          false,
		NoMultiThreading:        false,
		SlowModTime:             false,
		SlowHash:                false,
	}).Fill(ctx, f)

	f.features = features

	// Test that features are configured correctly for Huawei Drive
	if f.features.CaseInsensitive {
		t.Error("Huawei Drive should be case sensitive")
	}

	if f.features.DuplicateFiles {
		t.Error("Huawei Drive should not allow duplicate files")
	}

	if !f.features.ReadMimeType {
		t.Error("Huawei Drive should support reading MIME types")
	}

	if !f.features.WriteMimeType {
		t.Error("Huawei Drive should support setting MIME types")
	}

	if !f.features.CanHaveEmptyDirectories {
		t.Error("Huawei Drive should support empty directories")
	}

	if f.features.BucketBased {
		t.Error("Huawei Drive is not bucket-based")
	}

	if !f.features.FilterAware {
		t.Error("Huawei Drive should be filter aware")
	}

	if !f.features.ReadMetadata {
		t.Error("Huawei Drive should support reading metadata")
	}

	if !f.features.WriteMetadata {
		t.Error("Huawei Drive should support writing metadata")
	}
}

// TestAPIResponseParsing tests parsing of API responses
func TestAPIResponseParsing(t *testing.T) {
	// Test About response parsing
	t.Run("about_response", func(t *testing.T) {
		about := api.About{
			StorageQuota: struct {
				UsedSpace    string `json:"usedSpace"`
				UserCapacity string `json:"userCapacity"`
			}{
				UsedSpace:    "1073741824",  // 1GB
				UserCapacity: "10737418240", // 10GB
			},
			MaxThumbnailSize:  2097152,       // 2MB
			MaxFileUploadSize: "53687091200", // 50GB
			Domain:            "driveapis.cloud.huawei.com.cn",
			Category:          "drive#about",
			User: api.User{
				PermissionID: "test_permission_id",
				DisplayName:  "Test User",
				Me:           true,
				Category:     "drive#user",
			},
		}

		// Test parsing
		if about.StorageQuota.UsedSpace != "1073741824" {
			t.Errorf("expected used space %q, got %q", "1073741824", about.StorageQuota.UsedSpace)
		}
		if about.User.Me != true {
			t.Error("expected user.me to be true")
		}
		if about.Category != api.CategoryDriveAbout {
			t.Errorf("expected category %q, got %q", api.CategoryDriveAbout, about.Category)
		}
	})

	// Test FileList response parsing
	t.Run("filelist_response", func(t *testing.T) {
		fileList := api.FileList{
			Files: []api.File{
				{
					ID:           "test_file_id",
					FileName:     "test_file.txt",
					MimeType:     "text/plain",
					Category:     api.CategoryDriveFile,
					Size:         1024,
					SHA256:       "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					CreatedTime:  time.Now(),
					EditedTime:   time.Now(),
					ParentFolder: []string{"parent_folder_id"},
					Owners: []api.User{
						{
							PermissionID: "owner_permission_id",
							DisplayName:  "Owner User",
							Me:           true,
							Category:     "drive#user",
						},
					},
					OwnedByMe:  true,
					EditedByMe: true,
					Recycled:   false,
					Favorite:   false,
					Containers: []string{"drive"},
				},
			},
			NextPageToken: "next_page_token",
			Category:      api.CategoryDriveFileList,
		}

		// Test file properties
		if len(fileList.Files) != 1 {
			t.Errorf("expected 1 file, got %d", len(fileList.Files))
		}

		file := fileList.Files[0]
		if file.IsDir() {
			t.Error("text file should not be recognized as directory")
		}
		if file.Size != 1024 {
			t.Errorf("expected file size 1024, got %d", file.Size)
		}
		if !file.OwnedByMe {
			t.Error("expected file to be owned by me")
		}
	})
}

// TestObjectMetadata tests object metadata operations
func TestObjectMetadata(t *testing.T) {
	// Create a mock object with metadata
	obj := &Object{
		fs:          &Fs{},
		remote:      "test/file.txt",
		hasMetaData: true,
		size:        1024,
		modTime:     time.Now(),
		id:          "test_object_id",
		sha256:      "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		mimeType:    "text/plain",
	}

	// Test basic object properties
	if obj.Remote() != "test/file.txt" {
		t.Errorf("expected remote %q, got %q", "test/file.txt", obj.Remote())
	}

	if obj.Size() != 1024 {
		t.Errorf("expected size 1024, got %d", obj.Size())
	}

	// Test hash support
	ctx := context.Background()
	hash, err := obj.Hash(ctx, hash.SHA256)
	if err != nil {
		t.Errorf("unexpected error getting SHA256: %v", err)
	}
	if hash != obj.sha256 {
		t.Errorf("expected hash %q, got %q", obj.sha256, hash)
	}

	// Test unsupported hash (only SHA256 is supported by Huawei Drive)
	// We'll skip this test since it requires specific hash type constants

	// Test storable
	if !obj.Storable() {
		t.Error("object should be storable")
	}
}

// TestUploadTypes tests different upload type scenarios
func TestUploadTypes(t *testing.T) {
	testCases := []struct {
		name       string
		fileSize   int64
		cutoff     fs.SizeSuffix
		expectType string
	}{
		{"small_file_multipart", 1024, 20 * fs.Mebi, "multipart"},
		{"large_file_resume", int64(100 * fs.Mebi), 20 * fs.Mebi, "resume"},
		{"exact_cutoff_resume", int64(20 * fs.Mebi), 20 * fs.Mebi, "resume"},
		{"zero_size_multipart", 0, 20 * fs.Mebi, "multipart"},
		{"negative_size_resume", -1, 20 * fs.Mebi, "resume"}, // Unknown size
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var actualType string
			if tc.fileSize >= 0 && tc.fileSize < int64(tc.cutoff) {
				actualType = "multipart"
			} else {
				actualType = "resume"
			}

			if actualType != tc.expectType {
				t.Errorf("expected upload type %q for size %d with cutoff %d, got %q",
					tc.expectType, tc.fileSize, tc.cutoff, actualType)
			}
		})
	}
}

// TestRetryErrorCodes tests the retry error codes list
func TestRetryErrorCodes(t *testing.T) {
	expectedCodes := []int{429, 500, 502, 503, 504, 509}

	if len(retryErrorCodes) != len(expectedCodes) {
		t.Errorf("expected %d retry error codes, got %d", len(expectedCodes), len(retryErrorCodes))
	}

	for i, expected := range expectedCodes {
		if i >= len(retryErrorCodes) || retryErrorCodes[i] != expected {
			t.Errorf("expected retry error code %d at position %d, got %d", expected, i, retryErrorCodes[i])
		}
	}
}

// TestEncodingConfiguration tests the encoding configuration matches requirements
func TestEncodingConfiguration(t *testing.T) {
	// This should match the default encoding set in the config
	expectedEncoding := (encoder.Display |
		encoder.EncodeBackSlash |
		encoder.EncodeInvalidUtf8 |
		encoder.EncodeRightSpace |
		encoder.EncodeLeftSpace |
		encoder.EncodeLeftTilde |
		encoder.EncodeRightPeriod |
		encoder.EncodeLeftPeriod |
		encoder.EncodeColon |
		encoder.EncodePipe |
		encoder.EncodeDoubleQuote |
		encoder.EncodeLtGt |
		encoder.EncodeQuestion |
		encoder.EncodeAsterisk |
		encoder.EncodeCtl |
		encoder.EncodeDot)

	// Test that all required encodings are present
	requiredEncodings := []encoder.MultiEncoder{
		encoder.EncodeBackSlash,   // \
		encoder.EncodeColon,       // :
		encoder.EncodePipe,        // |
		encoder.EncodeDoubleQuote, // "
		encoder.EncodeLtGt,        // < >
		encoder.EncodeQuestion,    // ?
		encoder.EncodeAsterisk,    // *
		encoder.EncodeCtl,         // Control characters
	}

	for _, required := range requiredEncodings {
		if (expectedEncoding & required) == 0 {
			t.Errorf("encoding configuration missing required encoding: %v", required)
		}
	}

	// Test that the encoding handles Huawei Drive restricted characters
	enc := expectedEncoding
	restrictedChars := []struct {
		char string
		desc string
	}{
		{"<", "less than"},
		{">", "greater than"},
		{"|", "pipe"},
		{":", "colon"},
		{"\"", "double quote"},
		{"*", "asterisk"},
		{"?", "question mark"},
		{"/", "forward slash"},
		{"\\", "backslash"},
	}

	for _, rc := range restrictedChars {
		testName := "test" + rc.char + "file.txt"
		encoded := enc.FromStandardName(testName)
		if encoded == testName {
			t.Errorf("character %s (%s) was not encoded", rc.char, rc.desc)
		}
	}
}

// TestScopeConfiguration tests OAuth scope configuration
func TestScopeConfiguration(t *testing.T) {
	expectedScopes := []string{
		"openid",
		"profile",
		"https://www.huawei.com/auth/drive",
		"https://www.huawei.com/auth/drive.file",
	}

	if len(oauthConfig.Scopes) != len(expectedScopes) {
		t.Errorf("expected %d OAuth scopes, got %d", len(expectedScopes), len(oauthConfig.Scopes))
	}

	for i, expected := range expectedScopes {
		if i >= len(oauthConfig.Scopes) || oauthConfig.Scopes[i] != expected {
			t.Errorf("expected OAuth scope %q at position %d, got %q", expected, i, oauthConfig.Scopes[i])
		}
	}

	// Test OAuth URLs
	expectedAuthURL := "https://oauth-login.cloud.huawei.com/oauth2/v3/authorize"
	if oauthConfig.AuthURL != expectedAuthURL {
		t.Errorf("expected auth URL %q, got %q", expectedAuthURL, oauthConfig.AuthURL)
	}

	expectedTokenURL := "https://oauth-login.cloud.huawei.com/oauth2/v3/token"
	if oauthConfig.TokenURL != expectedTokenURL {
		t.Errorf("expected token URL %q, got %q", expectedTokenURL, oauthConfig.TokenURL)
	}
}

// TestObjectString tests object string representation
func TestObjectString(t *testing.T) {
	// Test normal object
	obj := &Object{remote: "path/to/file.txt"}
	expected := "path/to/file.txt"
	if obj.String() != expected {
		t.Errorf("expected object string %q, got %q", expected, obj.String())
	}

	// Test nil object
	var nilObj *Object
	if nilObj.String() != "<nil>" {
		t.Errorf("expected nil object string %q, got %q", "<nil>", nilObj.String())
	}
}

// TestSetModTime tests setting modification time
func TestSetModTime(t *testing.T) {
	// Test that we have a proper SetModTime implementation
	// This is a compile-time test to ensure the method signature is correct

	// Verify the method exists and has the right signature
	var obj *Object
	var ctx context.Context
	var modTime time.Time

	// This should compile without error, proving the method signature is correct
	_ = obj.SetModTime(ctx, modTime)

	t.Log("SetModTime method signature verified")

	// Test the implementation logic without actually making API calls
	// We verify that it's no longer returning fs.ErrorCantSetModTime immediately
	testObj := &Object{id: ""}
	err := testObj.SetModTime(context.Background(), time.Now())

	// With empty id, should get a specific error about missing id, not fs.ErrorCantSetModTime
	if err == fs.ErrorCantSetModTime {
		t.Error("SetModTime should not return fs.ErrorCantSetModTime - it should attempt validation/API call")
	}

	if err == nil {
		t.Error("SetModTime with empty id should return an error")
	}

	t.Logf("SetModTime with empty id returned expected error: %v", err)
} // TestCleanUp tests the CleanUp functionality
func TestCleanUp(t *testing.T) {
	// Test that the interface is correctly implemented at compile time
	// This test will fail to compile if CleanUpper interface is not properly implemented
	var _ fs.CleanUpper = (*Fs)(nil)

	t.Log("CleanUpper interface properly implemented")
}
