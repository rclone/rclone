package protondrive

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"testing"

	"github.com/rclone/go-proton-api"
	"github.com/stretchr/testify/assert"
)

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

func TestShouldRetry(t *testing.T) {
	ctx := context.Background()
	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()

	apiErr := func(status int, code proton.Code) error {
		return &proton.APIError{Status: status, Code: code, Message: "test"}
	}

	for _, tc := range []struct {
		name      string
		ctx       context.Context
		err       error
		wantRetry bool
	}{
		{"nil error", ctx, nil, false},
		{"cancelled context", cancelledCtx, errors.New("some error"), false},
		{"permanent validation error Code=200501 Status=422 (not retried)", ctx, apiErr(422, 200501), false},
		{"transient storage block error Code=200501 Status=500 (retried)", ctx, apiErr(500, 200501), true},
		{"server error Status=500", ctx, apiErr(500, 0), true},
		{"server error Status=502", ctx, apiErr(502, 0), true},
		{"server error Status=504", ctx, apiErr(504, 0), true},
		{"server error Status=503 (handled by SDK, not retried here)", ctx, apiErr(503, 0), false},
		{"rate limit Status=429 (handled by SDK, not retried here)", ctx, apiErr(429, 0), false},
		{"client error Status=400", ctx, apiErr(400, 0), false},
		{"client error Status=404", ctx, apiErr(404, 0), false},
		{"wrapped API error retried via errors.As", ctx, fmt.Errorf("wrapped: %w", &proton.APIError{Status: 500}), true},
		{"non-API error falls back to fserrors.ShouldRetry", ctx, errors.New("plain error"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gotRetry, _ := shouldRetry(tc.ctx, tc.err)
			assert.Equal(t, tc.wantRetry, gotRetry)
		})
	}
}
