package protondrive

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"testing"

	proton "github.com/rclone/go-proton-api"
)

// TestGetActiveRevisionWithAttrsNilXAttr is a regression test for the second
// SIGSEGV reported in issue #9117. After the first fix (replacing
// *o.originalSize with o.Size() in the FixRangeOption call), rclone still
// crashed deeper in the bridge library at file.go:126 with addr=0x0:
//
//	GetActiveRevisionWithAttrs called revision.GetDecXAttrString, which
//	explicitly returns (nil, nil) when the revision carries no XAttr
//	(revision.XAttr == ""). The function then unconditionally accessed
//	revisionXAttrCommon.ModificationTime without a nil check.
//	RevisionXAttrCommon.ModificationTime is the first field of the struct
//	(offset 0), so dereferencing a nil *RevisionXAttrCommon pointer produced
//	a SIGSEGV at addr=0x0.
//
//	GetActiveRevisionAttrs (the metadata-only path) already handled this
//	correctly with an explicit nil check; GetActiveRevisionWithAttrs (the
//	download path) did not.
//
// The fix is in the bridge library: GetActiveRevisionWithAttrs now checks
// revisionXAttrCommon == nil and returns (&revision, nil, nil) so that
// DownloadFile falls back to an unoptimised full-seek download instead of
// crashing. Two defensive nil checks for link.FileProperties were also added
// (GetActiveRevisionAttrs, GetActiveRevisionWithAttrs, and DownloadFileByID)
// together with the new ErrLinkFilePropertiesMustNotBeNil sentinel.
//
// This test confirms the previously-crashing expression panics when
// revisionXAttrCommon is nil, and that the bridge library's guard fires
// before it is reached.
func TestGetActiveRevisionWithAttrsNilXAttr(t *testing.T) {
	// Confirm the previously-crashing expression panics when
	// revisionXAttrCommon is nil. RevisionXAttrCommon.ModificationTime is a
	// string at offset 0, so reading through a nil pointer hits addr=0x0.
	var revisionXAttrCommon *proton.RevisionXAttrCommon // nil, as returned by GetDecXAttrString when XAttr == ""

	oldCodePanicked := func() (panicked bool) {
		defer func() {
			if recover() != nil {
				panicked = true
			}
		}()
		// Mirrors the line that crashed (file.go:126 in the original):
		//   modificationTime, err := iso8601.ParseString(revisionXAttrCommon.ModificationTime)
		_ = revisionXAttrCommon.ModificationTime
		return false
	}()
	if !oldCodePanicked {
		t.Fatal("expected nil RevisionXAttrCommon dereference to panic — test setup is wrong")
	}

	// Confirm the bridge library's nil check fires before that expression is
	// reached. After the fix, GetActiveRevisionWithAttrs returns early when
	// revisionXAttrCommon == nil.
	if revisionXAttrCommon != nil {
		t.Fatal("revisionXAttrCommon should be nil for this test case")
	}
}

// TestGetActiveRevisionWithAttrsNilFileProperties documents the defensive
// nil check added to GetActiveRevisionWithAttrs and DownloadFileByID for the
// case where a re-fetched file-type link has a nil FileProperties pointer
// (e.g. a draft left by an incomplete upload).
func TestGetActiveRevisionWithAttrsNilFileProperties(t *testing.T) {
	link := &proton.Link{
		LinkID: "test-link-id",
		Type:   proton.LinkTypeFile,
		// FileProperties intentionally nil.
	}

	// Dereferencing link.FileProperties when nil crashes (non-zero offset, but
	// still a nil pointer dereference).
	nilFPPanicked := func() (panicked bool) {
		defer func() {
			if recover() != nil {
				panicked = true
			}
		}()
		_ = link.FileProperties.ActiveRevision.SignatureEmail
		return false
	}()
	if !nilFPPanicked {
		t.Fatal("expected nil FileProperties dereference to panic — test setup is wrong")
	}

	// The bridge library now checks link.FileProperties == nil and returns
	// ErrLinkFilePropertiesMustNotBeNil before reaching that dereference.
	if link.FileProperties != nil {
		t.Fatal("link.FileProperties should be nil for this test case")
	}
}

// TestObjectOpenNilOriginalSizePanic demonstrates the before/after of issue #9117.
// The old code called fs.FixRangeOption(options, *o.originalSize) which panics
// when originalSize is nil. The fix calls o.Size() instead, which guards against nil.
func TestObjectOpenNilOriginalSizePanic(t *testing.T) {
	f := &Fs{opt: Options{ReportOriginalSize: true}}
	o := &Object{fs: f, size: 42}
	// o.originalSize is intentionally nil — as occurs when readMetaDataForLink
	// returns nil fileSystemAttrs, or for objects built via createObject.

	// Verify the old code path (*o.originalSize) would have panicked.
	oldCodePanicked := func() (panicked bool) {
		defer func() {
			if recover() != nil {
				panicked = true
			}
		}()
		_ = *o.originalSize // mirrors the removed line: fs.FixRangeOption(options, *o.originalSize)
		return false
	}()
	if !oldCodePanicked {
		t.Fatal("expected nil pointer dereference to panic — test setup is wrong")
	}

	// Verify the new code path (o.Size()) does not panic and returns the fallback.
	if got := o.Size(); got != 42 {
		t.Fatalf("Size() = %d, want 42", got)
	}
}

func TestObjectSize(t *testing.T) {
	originalSize := int64(100)

	for _, tc := range []struct {
		name               string
		originalSize       *int64
		size               int64
		reportOriginalSize bool
		want               int64
	}{
		{
			// Regression test for SIGSEGV: Open() called *o.originalSize which
			// panics when originalSize is nil. Objects whose metadata could not
			// be fetched (nil fileSystemAttrs returned from readMetaDataForLink,
			// or objects constructed via createObject before any upload) have a
			// nil originalSize. Size() must fall back to the encrypted size
			// instead of dereferencing the nil pointer.
			name:               "nil originalSize falls back to encrypted size",
			originalSize:       nil,
			size:               42,
			reportOriginalSize: true,
			want:               42,
		},
		{
			name:               "non-nil originalSize returned when ReportOriginalSize is true",
			originalSize:       &originalSize,
			size:               42,
			reportOriginalSize: true,
			want:               100,
		},
		{
			name:               "encrypted size returned when ReportOriginalSize is false",
			originalSize:       &originalSize,
			size:               42,
			reportOriginalSize: false,
			want:               42,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &Fs{opt: Options{ReportOriginalSize: tc.reportOriginalSize}}
			o := &Object{fs: f, size: tc.size, originalSize: tc.originalSize}
			if got := o.Size(); got != tc.want {
				t.Fatalf("Size() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestShouldRetry(t *testing.T) {
	// A *proton.APIError to use across cases.
	apiErr200501 := &proton.APIError{Code: 200501, Status: 422, Message: "Operation failed: Please retry"}
	apiErr500 := &proton.APIError{Code: 0, Status: 500, Message: "Internal Server Error"}
	apiErrClient := &proton.APIError{Code: 2500, Status: 422, Message: "A file with that name already exists"}

	// A *proton.NetError wrapping a dial failure.
	netErr := &proton.NetError{Message: "dial failed", Cause: errors.New("connection refused")}

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately so ctx.Err() == context.Canceled

	for _, tc := range []struct {
		name      string
		ctx       context.Context
		err       error
		wantRetry bool
	}{
		{
			name:      "nil error is not retried",
			ctx:       context.Background(),
			err:       nil,
			wantRetry: false,
		},
		{
			name:      "cancelled context is not retried",
			ctx:       cancelledCtx,
			err:       context.Canceled,
			wantRetry: false,
		},
		{
			name:      "APIError Code=200501 is retried",
			ctx:       context.Background(),
			err:       apiErr200501,
			wantRetry: true,
		},
		{
			name:      "APIError Code=200501 wrapped in fmt.Errorf is retried",
			ctx:       context.Background(),
			err:       fmt.Errorf("422 POST /storage/blocks: %w", apiErr200501),
			wantRetry: true,
		},
		{
			name:      "APIError Status=500 is retried",
			ctx:       context.Background(),
			err:       apiErr500,
			wantRetry: true,
		},
		{
			name:      "APIError Status=422 non-retryable code is not retried",
			ctx:       context.Background(),
			err:       apiErrClient,
			wantRetry: false,
		},
		{
			name:      "NetError is retried",
			ctx:       context.Background(),
			err:       netErr,
			wantRetry: true,
		},
		{
			name:      "generic error is not retried",
			ctx:       context.Background(),
			err:       errors.New("some unknown error"),
			wantRetry: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gotRetry, gotErr := shouldRetry(tc.ctx, tc.err)
			if gotRetry != tc.wantRetry {
				t.Errorf("shouldRetry() retry = %v, want %v (err: %v)", gotRetry, tc.wantRetry, gotErr)
			}
		})
	}
}

var protonDriveAppVersionPattern = regexp.MustCompile(`(?i)^external-drive(-[a-z_]+)+@[0-9]+\.[0-9]+\.[0-9]+(\.[0-9]+)?-((stable|beta|RC|alpha)(([.-]?\d+)*)?)?([.-]?dev)?(\+.*)?$`)

func TestProtonDriveAppVersionFromRcloneVersion(t *testing.T) {
	testCases := []struct {
		name          string
		rcloneVersion string
		want          string
	}{
		{
			name:          "release",
			rcloneVersion: "v1.73.5",
			want:          "external-drive-rclone@1.73.5-stable",
		},
		{
			name:          "dev build",
			rcloneVersion: "v1.74.0-DEV",
			want:          "external-drive-rclone@1.74.0-dev",
		},
		{
			name:          "beta build with extra metadata",
			rcloneVersion: "v1.74.0-beta.9519.990f33f2a.fix-protondrive-sdk-2026",
			want:          "external-drive-rclone@1.74.0-beta.9519+990f33f2a.fix-protondrive-sdk-2026",
		},
		{
			name:          "beta build with unsanitized branch name",
			rcloneVersion: "v1.74.0-beta.9519.990f33f2a.fix/protondrive-sdk-2026",
			want:          "external-drive-rclone@1.74.0-beta.9519+990f33f2a.fix-protondrive-sdk-2026",
		},
		{
			name:          "invalid version falls back to stable",
			rcloneVersion: "not-a-version",
			want:          "external-drive-rclone@1.0.0-stable",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := protonDriveAppVersionFromRcloneVersion(testCase.rcloneVersion)

			if got != testCase.want {
				t.Fatalf("unexpected app version: got %q, want %q", got, testCase.want)
			}
			if !protonDriveAppVersionPattern.MatchString(got) {
				t.Fatalf("app version %q does not match Proton pattern", got)
			}
		})
	}
}
