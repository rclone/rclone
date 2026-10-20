package api

import (
	"testing"
)

func TestPropStatusOK(t *testing.T) {
	tests := []struct {
		name   string
		status []string
		want   bool
	}{
		{"Empty status", []string{}, true},
		{"Single 200 OK", []string{"HTTP/1.1 200 OK"}, true},
		{"Single 404 Not Found", []string{"HTTP/1.1 404 Not Found"}, false},
		{"Mixed status with 404 first", []string{"HTTP/1.1 404 Not Found", "HTTP/1.1 200 OK"}, true},
		{"Mixed status with 200 first", []string{"HTTP/1.1 200 OK", "HTTP/1.1 404 Not Found"}, true},
		{"Multiple non-2xx statuses", []string{"HTTP/1.1 404 Not Found", "HTTP/1.1 403 Forbidden"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Prop{Status: tt.status}
			if got := p.StatusOK(); got != tt.want {
				t.Errorf("Prop.StatusOK() = %v, want %v", got, tt.want)
			}
		})
	}
}
