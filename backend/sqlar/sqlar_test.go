package sqlar_test

import (
	"os"
	"testing"

	sqlite3 "github.com/ncruces/go-sqlite3"
	"github.com/rclone/rclone/backend/sqlar"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// TestMain configures the wazero runtime to use the interpreter engine
// instead of the JIT compiler. The default JIT compiler has higher peak
// memory usage from native code generation. Combined with go test ./...
// running many large test binaries in parallel, this is enough to OOM on
// memory-constrained CI runners (e.g. Windows GitHub Actions with 7 GB).
// The interpreter avoids this overhead and is fast enough for test workloads.
func TestMain(m *testing.M) {
	sqlite3.RuntimeConfig = wazero.NewRuntimeConfigInterpreter().
		WithMemoryLimitPages(4096). // 256 MB (same as default)
		WithCoreFeatures(api.CoreFeaturesV2)
	os.Exit(m.Run())
}

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName:  ":sqlar,path=" + t.TempDir() + "/test.sqlar:",
		NilObject:   (*sqlar.Object)(nil),
		QuickTestOK: true,
	})
}
