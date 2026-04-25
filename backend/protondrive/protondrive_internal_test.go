package protondrive

import (
	"regexp"
	"testing"
)

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
