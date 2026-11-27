package tarkle

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsUploadStep(t *testing.T) {
	for _, c := range []struct {
		method, path string
		step         bool
	}{
		{"POST", "/uploads", false}, // the start is counted by the server
		{"GET", "/uploads/u1", true},
		{"DELETE", "/uploads/u1", true},
		{"POST", "/uploads/u1/parts", true},
		{"POST", "/uploads/u1/complete", true},
		{"GET", "/uploads/u1/parts", false},
		{"POST", "/uploads/u1", false},
		{"POST", "/uploads/u1/other", false},
		{"POST", "/uploads/u1/complete/extra", false},
		{"GET", "/files/1", false},
		{"GET", "/uploads/", false},
	} {
		assert.Equal(t, c.step, isUploadStep(c.method, c.path), c.method+" "+c.path)
	}
}

// The server does not count the steps of an upload after its start in the per-minute window and answers them with a full one. If they
// drew from the local window, a run of uploads would use it up on calls the server never counted and wait for a minute it did not need.
func TestUploadStepsDoNotSpendTheBurstWindow(t *testing.T) {
	f, _ := newScriptServer(t, nil, func(s *scriptServer, c apiCall) (int, string) { return 200, `{}` })
	f.limit.remaining = 0 // the window is used up by counted calls
	f.limit.resetAt = time.Now().Add(time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	for _, call := range []rest.Opts{
		{Method: "POST", Path: "/uploads/u1/parts"},
		{Method: "POST", Path: "/uploads/u1/complete"},
		{Method: "GET", Path: "/uploads/u1"},
		{Method: "DELETE", Path: "/uploads/u1"},
	} {
		opts := call
		require.NoError(t, f.call(ctx, &opts, nil, nil), call.Method+" "+call.Path)
	}
	assert.Less(t, time.Since(start), 2*time.Second, "steps do not wait for the next window")
	assert.Equal(t, 0, f.limit.remaining, "and they do not change the window")

	// the start is a counted call: with the window used up it waits (here until the context ends)
	short, cancelShort := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancelShort()
	opts := rest.Opts{Method: "POST", Path: "/uploads"}
	err := f.call(short, &opts, nil, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded), "the start waits for the next window: %v", err)
}

// A step answer says the window is full (remaining = the limit). It must not raise a window the counted calls had used up.
func TestUploadStepAnswersDoNotRefillTheWindow(t *testing.T) {
	f, s := newScriptServer(t, nil, func(s *scriptServer, c apiCall) (int, string) { return 200, `{}` })
	s.header.Set("RateLimit-Remaining", "50")
	s.header.Set("RateLimit-Reset", "30")
	f.limit.remaining = 3
	f.limit.resetAt = time.Now().Add(30 * time.Second)
	opts := rest.Opts{Method: "POST", Path: "/uploads/u1/complete"}
	require.NoError(t, f.call(context.Background(), &opts, nil, nil))
	assert.Equal(t, 3, f.limit.remaining)
}

// Identical content is skipped, but rclone expects the updated file to carry the source's time: without it the next sync sees another
// time and uploads again.
func skipServer(t *testing.T, existingTime string) (*Fs, *scriptServer) {
	return newScriptServer(t, map[string]string{"hash_before_upload": "true"}, func(s *scriptServer, c apiCall) (int, string) {
		switch {
		case c.method == "POST" && c.path == "/uploads":
			return 200, `{"action":"skip","existing":{"id":"1","name":"f.txt"}}`
		case c.method == "GET" && c.path == "/files/1":
			return 200, `{"id":"1","type":"file","name":"f.txt","size":3,"parent_id":null,"modified_at":"` + existingTime + `"}`
		case c.method == "PATCH" && c.path == "/files/1":
			return 200, `{"id":"1","type":"file","name":"f.txt","size":3,"parent_id":null,"modified_at":"` + c.str("modified_at") + `"}`
		}
		return problem(http.StatusNotFound, "not_found")
	})
}

func TestSkippedUploadGetsTheSourceModTime(t *testing.T) {
	f, s := skipServer(t, "2020-01-02T03:04:05.000Z")
	want := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	o := &Object{fs: f, remote: "f.txt"}
	src := object.NewStaticObjectInfo("f.txt", want, 3, true, map[hash.Type]string{hash.SHA256: strings.Repeat("ab", 32)}, nil)
	require.NoError(t, o.Update(context.Background(), failReader{t}, src))
	patches := s.find("PATCH", "/files/1")
	require.Len(t, patches, 1, "the identical file is given the source's modification time")
	assert.Equal(t, formatTime(want), patches[0].str("modified_at"))
	assert.True(t, o.modTime.Equal(want), "the object reports the new time, not the old one: %v", o.modTime)
}

func TestSkippedUploadWithTheSameTimeSendsNothing(t *testing.T) {
	want := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	f, s := skipServer(t, formatTime(want))
	o := &Object{fs: f, remote: "f.txt"}
	src := object.NewStaticObjectInfo("f.txt", want.Add(300*time.Millisecond), 3, true, map[hash.Type]string{hash.SHA256: strings.Repeat("ab", 32)}, nil)
	require.NoError(t, o.Update(context.Background(), failReader{t}, src))
	assert.Empty(t, s.find("PATCH", "/files/1"), "the same time to the second: nothing to change")
}
