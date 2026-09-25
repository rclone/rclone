package drive

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/googleapi"
)

func TestIsInsufficientFilePermissions(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil",
			err:  nil,
			want: false,
		},
		{
			name: "plain error",
			err:  errors.New("boom"),
			want: false,
		},
		{
			name: "403 with insufficientFilePermissions reason",
			err: &googleapi.Error{
				Code: http.StatusForbidden,
				Errors: []googleapi.ErrorItem{{
					Reason:  "insufficientFilePermissions",
					Message: "The user does not have sufficient permissions for this file.",
				}},
			},
			want: true,
		},
		{
			name: "403 with message only",
			err: &googleapi.Error{
				Code:    http.StatusForbidden,
				Message: "The user does not have sufficient permissions for this file.",
			},
			want: true,
		},
		{
			name: "403 wrapped",
			err: fmt.Errorf("delete failed: %w", &googleapi.Error{
				Code: http.StatusForbidden,
				Errors: []googleapi.ErrorItem{{
					Reason: "insufficientFilePermissions",
				}},
			}),
			want: true,
		},
		{
			name: "403 other reason",
			err: &googleapi.Error{
				Code: http.StatusForbidden,
				Errors: []googleapi.ErrorItem{{
					Reason:  "userRateLimitExceeded",
					Message: "User rate limit exceeded.",
				}},
			},
			want: false,
		},
		{
			name: "404 not found",
			err: &googleapi.Error{
				Code: http.StatusNotFound,
				Errors: []googleapi.ErrorItem{{
					Reason: "notFound",
				}},
			},
			want: false,
		},
		{
			name: "500 server error",
			err: &googleapi.Error{
				Code:    http.StatusInternalServerError,
				Message: "backend error",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isInsufficientFilePermissions(tt.err))
		})
	}
}

func TestUnlinkFromParentsDecision(t *testing.T) {
	// Document the parent-count rules used by unlinkFromParents without a live Drive API.
	tests := []struct {
		name        string
		parents     []string
		wantErr     string
		wantParent  string
		wantProceed bool
	}{
		{
			name:    "no parents",
			parents: nil,
			wantErr: "no parent folder to unlink from",
		},
		{
			name:    "empty parents",
			parents: []string{},
			wantErr: "no parent folder to unlink from",
		},
		{
			name:        "single parent",
			parents:     []string{"parent-1"},
			wantParent:  "parent-1",
			wantProceed: true,
		},
		{
			name:    "multiple parents",
			parents: []string{"parent-1", "parent-2"},
			wantErr: "can't delete safely - has multiple parents",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parent, err := chooseUnlinkParent("file-1", tt.parents)
			if tt.wantProceed {
				require.NoError(t, err)
				assert.Equal(t, tt.wantParent, parent)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
