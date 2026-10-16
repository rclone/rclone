package operations

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	fslog "github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/fstest/mockfs"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/rclone/rclone/lib/transferaccounter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	fetchCopyPayload = "payload"
	fetchCopyURL     = "https://source.example/object?token=secret"
)

type fetchCopyFs struct {
	fs.Fs
	features *fs.Features
	put      func(context.Context, io.Reader, fs.ObjectInfo, ...fs.OpenOption) (fs.Object, error)
}

func (f *fetchCopyFs) Features() *fs.Features {
	return f.features
}

func (f *fetchCopyFs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.put(ctx, in, src, options...)
}

type fetchCopyObject struct {
	*mockobject.ContentMockObject
	opens int
}

func (o *fetchCopyObject) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	o.opens++
	return o.ContentMockObject.Open(ctx, options...)
}

type fetchCopyFixture struct {
	t          *testing.T
	ctx        context.Context
	ci         *fs.ConfigInfo
	srcFs      *fetchCopyFs
	dstFs      *fetchCopyFs
	src        *fetchCopyObject
	calls      []string
	putOptions []fs.OpenOption
}

func newFetchCopyFixture(t *testing.T, srcName, dstName string) *fetchCopyFixture {
	t.Helper()

	ctx, ci := fs.AddConfig(context.Background())
	ci.Inplace = true
	ci.LowLevelRetries = 1
	ci.MultiThreadStreams = 0
	ctx = accounting.WithStatsGroup(ctx, t.Name())
	accounting.NewStatsGroup(ctx, t.Name())

	srcBase, err := mockfs.NewFs(ctx, srcName, "source-root", nil)
	require.NoError(t, err)
	dstBase, err := mockfs.NewFs(ctx, dstName, "destination-root", nil)
	require.NoError(t, err)

	f := &fetchCopyFixture{
		t:     t,
		ctx:   ctx,
		ci:    ci,
		srcFs: &fetchCopyFs{Fs: srcBase, features: &fs.Features{}},
		dstFs: &fetchCopyFs{Fs: dstBase, features: &fs.Features{}},
	}
	f.src = &fetchCopyObject{ContentMockObject: mockobject.New("source/original").WithContent([]byte(fetchCopyPayload), mockobject.SeekModeNone)}
	f.src.SetFs(f.srcFs)
	f.dstFs.put = func(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
		f.calls = append(f.calls, "manual")
		body, err := io.ReadAll(in)
		if err != nil {
			return nil, err
		}
		f.putOptions = append([]fs.OpenOption(nil), options...)
		return f.object(src.Remote(), body), nil
	}
	return f
}

func (f *fetchCopyFixture) object(remote string, content []byte) fs.Object {
	o := mockobject.New(remote).WithContent(content, mockobject.SeekModeNone)
	o.SetFs(f.dstFs)
	return o
}

func (f *fetchCopyFixture) enablePublicLink() {
	f.srcFs.features.PublicLinkIsDirect = true
	f.srcFs.features.PublicLink = func(ctx context.Context, remote string, expire fs.Duration, unlink bool) (string, error) {
		f.calls = append(f.calls, "link")
		assert.Equal(f.t, "source/original", remote)
		assert.Equal(f.t, fs.Duration(24*time.Hour), expire)
		assert.False(f.t, unlink)
		return fetchCopyURL, nil
	}
}

func (f *fetchCopyFixture) enableFetch(fetchErr error) {
	f.dstFs.features.ServerSideFetchURL = func(ctx context.Context, remote, sourceURL string, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
		f.calls = append(f.calls, "fetch")
		assert.Equal(f.t, "target", remote)
		assert.Equal(f.t, fetchCopyURL, sourceURL)
		assert.Same(f.t, f.src, src)
		if fetchErr != nil {
			return nil, fetchErr
		}
		return f.object(remote, []byte(fetchCopyPayload)), nil
	}
}

func (f *fetchCopyFixture) copy() (fs.Object, error) {
	return Copy(f.ctx, f.dstFs, nil, "target", f.src)
}

func TestCopyServerSideFetchSelection(t *testing.T) {
	tests := []struct {
		name                            string
		native, safe, publicLink, fetch bool
		fetchErr                        error
		wantCalls                       []string
		wantOpens                       int
		wantErr                         error
	}{
		{"native wins", true, true, true, true, nil, []string{"native"}, 0, nil},
		{"fetch succeeds", false, true, true, true, nil, []string{"link", "fetch"}, 0, nil},
		{"unsafe source", false, false, true, true, nil, []string{"manual"}, 1, nil},
		{"no link", false, true, false, true, nil, []string{"manual"}, 1, nil},
		{"no fetch", false, true, true, false, nil, []string{"manual"}, 1, nil},
		{"fetch unavailable", false, true, true, true, fs.ErrorCantCopy, []string{"link", "fetch", "manual"}, 1, nil},
		{"fetch canceled", false, true, true, true, context.Canceled, []string{"link", "fetch"}, 0, context.Canceled},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFetchCopyFixture(t, "same-config", "same-config")
			if test.publicLink {
				f.enablePublicLink()
			}
			f.srcFs.features.PublicLinkIsDirect = test.safe
			if test.fetch {
				f.enableFetch(test.fetchErr)
			}
			if test.native {
				f.dstFs.features.Copy = func(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
					f.calls = append(f.calls, "native")
					assert.Same(t, f.src, src)
					assert.Equal(t, "target", remote)
					return f.object(remote, []byte(fetchCopyPayload)), nil
				}
			}

			got, err := f.copy()

			assert.Equal(t, test.wantCalls, f.calls)
			assert.Equal(t, test.wantOpens, f.src.opens)
			if test.wantErr != nil {
				require.ErrorIs(t, err, test.wantErr)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				require.NotNil(t, got)
				assert.Equal(t, int64(len(fetchCopyPayload)), got.Size())
			}
		})
	}
}

func TestCopyServerSideFetchAfterNativeCantCopy(t *testing.T) {
	f := newFetchCopyFixture(t, "same-config", "same-config")
	f.enablePublicLink()
	f.enableFetch(nil)
	f.dstFs.features.Copy = func(context.Context, fs.Object, string) (fs.Object, error) {
		f.calls = append(f.calls, "native")
		return nil, fs.ErrorCantCopy
	}

	got, err := f.copy()

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, []string{"native", "link", "fetch"}, f.calls)
	assert.Zero(t, f.src.opens)
}

func TestCopyServerSideFetchNativeErrorStopsFallback(t *testing.T) {
	destinationErr := errors.New("destination auth failure")
	f := newFetchCopyFixture(t, "same-config", "same-config")
	f.enablePublicLink()
	f.enableFetch(nil)
	f.dstFs.features.Copy = func(context.Context, fs.Object, string) (fs.Object, error) {
		f.calls = append(f.calls, "native")
		return nil, destinationErr
	}

	got, err := f.copy()

	require.ErrorIs(t, err, destinationErr)
	assert.Nil(t, got)
	assert.Equal(t, []string{"native"}, f.calls)
	assert.Zero(t, f.src.opens)
}

func TestCopyServerSideFetchDestinationErrorStopsFallback(t *testing.T) {
	destinationErr := errors.New("destination auth failure")
	f := newFetchCopyFixture(t, "same-config", "same-config")
	f.enablePublicLink()
	f.enableFetch(destinationErr)

	got, err := f.copy()

	require.ErrorIs(t, err, destinationErr)
	assert.Nil(t, got)
	assert.Equal(t, []string{"link", "fetch"}, f.calls)
	assert.Zero(t, f.src.opens)
}

func TestCopyServerSideFetchAcrossConfigs(t *testing.T) {
	f := newFetchCopyFixture(t, "source-config", "destination-config")
	f.enablePublicLink()
	f.enableFetch(nil)
	f.dstFs.features.ServerSideAcrossConfigs = false
	f.ci.ServerSideAcrossConfigs = false
	f.dstFs.features.Copy = func(context.Context, fs.Object, string) (fs.Object, error) {
		f.calls = append(f.calls, "native")
		return nil, errors.New("native copy must not be called")
	}

	got, err := f.copy()

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, []string{"link", "fetch"}, f.calls)
	assert.Zero(t, f.src.opens)
}

func TestCopyServerSideFetchPreflight(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*fetchCopyFixture)
		wantCalls []string
		wantOpens int
		wantErr   error
	}{
		{"dry run", func(f *fetchCopyFixture) { f.ci.DryRun = true }, nil, 0, nil},
		{"negative size", func(f *fetchCopyFixture) { f.src.SetUnknownSize(true) }, []string{"manual"}, 1, nil},
		{"download headers", func(f *fetchCopyFixture) { f.ci.DownloadHeaders = []*fs.HTTPOption{{Key: "X-Source", Value: "value"}} }, []string{"manual"}, 1, nil},
		{"global headers", func(f *fetchCopyFixture) { f.ci.Headers = []*fs.HTTPOption{{Key: "X-Global", Value: "value"}} }, []string{"manual"}, 1, nil},
		{"client certificate", func(f *fetchCopyFixture) { f.ci.ClientCert = "client.pem" }, []string{"manual"}, 1, nil},
		{"client key", func(f *fetchCopyFixture) { f.ci.ClientKey = "client.key" }, []string{"manual"}, 1, nil},
		{"HTTP dump", func(f *fetchCopyFixture) { f.ci.Dump = fs.DumpHeaders }, []string{"manual"}, 1, nil},
		{"destination feature disabled", func(f *fetchCopyFixture) { f.dstFs.features.ServerSideFetchURL = nil }, []string{"manual"}, 1, nil},
		{"canceled parent", func(f *fetchCopyFixture) {
			ctx, cancel := context.WithCancel(f.ctx)
			cancel()
			f.ctx = ctx
		}, nil, 0, context.Canceled},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFetchCopyFixture(t, "same-config", "same-config")
			f.enablePublicLink()
			f.enableFetch(nil)
			test.mutate(f)

			got, err := f.copy()

			assert.Equal(t, test.wantCalls, f.calls)
			assert.Equal(t, test.wantOpens, f.src.opens)
			if test.wantErr != nil {
				require.ErrorIs(t, err, test.wantErr)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCopyServerSideFetchPublicLinkFallback(t *testing.T) {
	tests := []struct {
		name string
		url  string
		err  error
	}{
		{"failed", "", errors.New("presign failed for " + fetchCopyURL)},
		{"empty", "", nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFetchCopyFixture(t, "same-config", "same-config")
			f.enablePublicLink()
			f.enableFetch(nil)
			f.srcFs.features.PublicLink = func(context.Context, string, fs.Duration, bool) (string, error) {
				f.calls = append(f.calls, "link")
				return test.url, test.err
			}

			got, err := f.copy()

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, []string{"link", "manual"}, f.calls)
			assert.Equal(t, 1, f.src.opens)
		})
	}
}

func captureFetchCopyLogs(run func()) string {
	var mu sync.Mutex
	var logs bytes.Buffer
	oldLevel := fslog.Handler.SetLevel(slog.LevelDebug)
	fslog.Handler.SetOutput(func(_ slog.Level, text string) {
		mu.Lock()
		defer mu.Unlock()
		logs.WriteString(text)
	})
	defer func() {
		fslog.Handler.ResetOutput()
		fslog.Handler.SetLevel(oldLevel)
	}()
	run()
	mu.Lock()
	defer mu.Unlock()
	return logs.String()
}

func TestCopyServerSideFetchPublicLinkErrorDoesNotLogURL(t *testing.T) {
	f := newFetchCopyFixture(t, "same-config", "same-config")
	f.enablePublicLink()
	f.enableFetch(nil)
	f.srcFs.features.PublicLink = func(context.Context, string, fs.Duration, bool) (string, error) {
		f.calls = append(f.calls, "link")
		return "", fmt.Errorf("failed URL %s", fetchCopyURL)
	}

	var err error
	logs := captureFetchCopyLogs(func() { _, err = f.copy() })

	require.NoError(t, err)
	assert.NotContains(t, logs, fetchCopyURL)
}

func TestCopyServerSideFetchPublicLinkCancellationDoesNotLogURL(t *testing.T) {
	f := newFetchCopyFixture(t, "same-config", "same-config")
	f.enablePublicLink()
	f.enableFetch(nil)
	f.srcFs.features.PublicLink = func(context.Context, string, fs.Duration, bool) (string, error) {
		f.calls = append(f.calls, "link")
		return "", fmt.Errorf("failed URL %s: %w", fetchCopyURL, context.Canceled)
	}

	var got fs.Object
	var err error
	logs := captureFetchCopyLogs(func() { got, err = f.copy() })

	require.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, got)
	assert.Equal(t, []string{"link"}, f.calls)
	assert.NotContains(t, logs, fetchCopyURL)
}

func TestCopyServerSideFetchCancellationStopsManualFallback(t *testing.T) {
	tests := []struct {
		name   string
		cancel string
	}{
		{"during URL generation", "link"},
		{"during failed fetch", "fetch"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFetchCopyFixture(t, "same-config", "same-config")
			ctx, cancel := context.WithCancel(f.ctx)
			f.ctx = ctx
			f.enablePublicLink()
			f.enableFetch(nil)
			if test.cancel == "link" {
				f.srcFs.features.PublicLink = func(context.Context, string, fs.Duration, bool) (string, error) {
					f.calls = append(f.calls, "link")
					cancel()
					return fetchCopyURL, nil
				}
			} else {
				f.dstFs.features.ServerSideFetchURL = func(context.Context, string, string, fs.ObjectInfo, ...fs.OpenOption) (fs.Object, error) {
					f.calls = append(f.calls, "fetch")
					cancel()
					return nil, fs.ErrorCantCopy
				}
			}

			got, err := f.copy()

			require.ErrorIs(t, err, context.Canceled)
			assert.Nil(t, got)
			if test.cancel == "link" {
				assert.Equal(t, []string{"link"}, f.calls)
			} else {
				assert.Equal(t, []string{"link", "fetch"}, f.calls)
			}
			assert.Zero(t, f.src.opens)
		})
	}
}

func TestCopyServerSideFetchPartialName(t *testing.T) {
	f := newFetchCopyFixture(t, "same-config", "same-config")
	f.ci.Inplace = false
	f.dstFs.features.PartialUploads = true
	f.enablePublicLink()
	var fetchedRemote, movedRemote string
	f.dstFs.features.ServerSideFetchURL = func(ctx context.Context, remote, sourceURL string, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
		f.calls = append(f.calls, "fetch")
		fetchedRemote = remote
		return f.object(remote, []byte(fetchCopyPayload)), nil
	}
	f.dstFs.features.Move = func(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
		f.calls = append(f.calls, "move")
		movedRemote = src.Remote()
		assert.Equal(t, "target", remote)
		return f.object(remote, []byte(fetchCopyPayload)), nil
	}

	got, err := f.copy()

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "target", got.Remote())
	assert.NotEqual(t, "target", fetchedRemote)
	assert.True(t, strings.HasSuffix(fetchedRemote, ".partial"), fetchedRemote)
	assert.Equal(t, fetchedRemote, movedRemote)
	assert.Equal(t, []string{"link", "fetch", "move"}, f.calls)
	assert.Zero(t, f.src.opens)
}

func assertFetchCopyUploadOptions(t *testing.T, got []fs.OpenOption, header *fs.HTTPOption, metadata fs.Metadata) {
	t.Helper()
	require.Len(t, got, 3)
	assert.IsType(t, &fs.HashesOption{}, got[0])
	assert.Same(t, header, got[1])
	assert.Equal(t, fs.MetadataOption(metadata), got[2])
}

func TestCopyServerSideFetchUploadOptions(t *testing.T) {
	for _, test := range []struct {
		name  string
		fetch bool
	}{
		{"fetch", true},
		{"manual", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newFetchCopyFixture(t, "same-config", "same-config")
			header := &fs.HTTPOption{Key: "Content-Type", Value: "text/custom"}
			metadata := fs.Metadata{"owner": "test"}
			f.ci.UploadHeaders = []*fs.HTTPOption{header}
			f.ci.MetadataSet = metadata
			var fetchOptions []fs.OpenOption
			if test.fetch {
				f.enablePublicLink()
				f.dstFs.features.ServerSideFetchURL = func(ctx context.Context, remote, sourceURL string, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
					f.calls = append(f.calls, "fetch")
					fetchOptions = append([]fs.OpenOption(nil), options...)
					return f.object(remote, []byte(fetchCopyPayload)), nil
				}
			}

			_, err := f.copy()

			require.NoError(t, err)
			if test.fetch {
				assert.Equal(t, []string{"link", "fetch"}, f.calls)
				assertFetchCopyUploadOptions(t, fetchOptions, header, metadata)
			} else {
				assert.Equal(t, []string{"manual"}, f.calls)
				assertFetchCopyUploadOptions(t, f.putOptions, header, metadata)
			}
		})
	}
}

func TestCopyServerSideFetchAccounting(t *testing.T) {
	for _, test := range []struct {
		name             string
		backendProgress  bool
		fetchErr         error
		wantCalls        []string
		wantServerCopies int64
		wantServerBytes  int64
	}{
		{"success without backend progress", false, nil, []string{"link", "fetch"}, 1, int64(len(fetchCopyPayload))},
		{"success with backend progress", true, nil, []string{"link", "fetch"}, 1, int64(len(fetchCopyPayload))},
		{"fallback resets progress", true, fmt.Errorf("source rejected: %w", fs.ErrorCantCopy), []string{"link", "fetch", "manual"}, 0, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newFetchCopyFixture(t, "same-config", "same-config")
			f.enablePublicLink()
			f.dstFs.features.ServerSideFetchURL = func(ctx context.Context, remote, sourceURL string, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
				f.calls = append(f.calls, "fetch")
				if test.backendProgress {
					ta := transferaccounter.Get(ctx)
					ta.Start()
					n := int64(len(fetchCopyPayload))
					if test.fetchErr != nil {
						n = 3
					}
					ta.Add(n)
				}
				if test.fetchErr != nil {
					return nil, test.fetchErr
				}
				return f.object(remote, []byte(fetchCopyPayload)), nil
			}

			_, err := f.copy()

			require.NoError(t, err)
			assert.Equal(t, test.wantCalls, f.calls)
			stats := accounting.Stats(f.ctx)
			assert.Equal(t, int64(len(fetchCopyPayload)), stats.GetBytes())
			remoteStats, err := stats.RemoteStats(false)
			require.NoError(t, err)
			assert.Equal(t, test.wantServerCopies, remoteStats["serverSideCopies"])
			assert.Equal(t, test.wantServerBytes, remoteStats["serverSideCopyBytes"])
		})
	}
}

func TestCopyServerSideFetchNilDestinationIsError(t *testing.T) {
	f := newFetchCopyFixture(t, "same-config", "same-config")
	f.enablePublicLink()
	f.dstFs.features.ServerSideFetchURL = func(context.Context, string, string, fs.ObjectInfo, ...fs.OpenOption) (fs.Object, error) {
		f.calls = append(f.calls, "fetch")
		return nil, nil
	}

	got, err := f.copy()

	require.EqualError(t, err, "server-side URL fetch returned no destination object")
	assert.Nil(t, got)
	assert.Equal(t, []string{"link", "fetch"}, f.calls)
	assert.Zero(t, f.src.opens)
}
