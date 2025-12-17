package makefiles

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/test"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/sync"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/lib/random"
	"github.com/spf13/cobra"
)

var (
	// Flags
	testTime = fs.Duration(15 * time.Second)
	fcap     = 100
	small    = fs.SizeSuffix(1024)
	medium   = fs.SizeSuffix(10 * 1024 * 1024)
	large    = fs.SizeSuffix(1024 * 1024 * 1024)
	useJSON  = false
)

func init() {
	test.Command.AddCommand(speedCmd)

	speedFlags := speedCmd.Flags()
	flags.FVarP(speedFlags, &testTime, "test-time", "", "Length for each test to run", "")
	flags.IntVarP(speedFlags, &fcap, "file-cap", "", fcap, "Maximum number of files to use in each test", "")
	flags.FVarP(speedFlags, &small, "small", "", "Size of small files", "")
	flags.FVarP(speedFlags, &medium, "medium", "", "Size of medium files", "")
	flags.FVarP(speedFlags, &large, "large", "", "Size of large files", "")
	flags.BoolVarP(speedFlags, &useJSON, "json", "", useJSON, "Output only results in JSON format", "")

	addCommonFlags(speedFlags)
}

func logf(text string, args ...any) {
	if !useJSON {
		fmt.Printf(text, args...)
	}
}

var speedCmd = &cobra.Command{
	Use:   "speed <remote> [flags]",
	Short: `Run a speed test to the remote`,
	Long: `Run a speed test to the remote.

This command runs a series of uploads and downloads to the remote, measuring
and printing the speed of each test using varying file sizes and numbers of
files.

Test time can be innaccurate with small file caps and large files. As it
uses the results of an initial test to determine how many files to use in
each subsequent test.

It is recommended to use -q flag for a simpler output. e.g.:

    rclone test speed remote: -q

**NB** This command will create and delete files on the remote in a randomly
named directory which will be automatically removed on a clean exit.

You can use the --json flag to only print the results in JSON format.`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.72",
	},
	RunE: func(command *cobra.Command, args []string) error {
		ctx := command.Context()
		cmd.CheckArgs(1, 1, command, args)
		commonInit()

		// initial test
		size := fs.SizeSuffix(1024 * 1024)
		logf("Running initial test for 4 files of size %v\n", size)
		stats, err := speedTest(ctx, 4, size, args[0])
		if err != nil {
			return fmt.Errorf("speed test failed: %w", err)
		}

		var results []*Stats

		// main tests
		logf("\nTest Time: %v, File cap: %d\n", testTime, fcap)
		for _, size := range []fs.SizeSuffix{small, medium, large} {
			numberOfFilesUpload := int((float64(stats.Upload.Speed) * time.Duration(testTime).Seconds()) / float64(size))
			numberOfFilesDownload := int((float64(stats.Download.Speed) * time.Duration(testTime).Seconds()) / float64(size))
			numberOfFiles := min(numberOfFilesUpload, numberOfFilesDownload)

			logf("\nNumber of files for upload and download: %v\n", numberOfFiles)
			if numberOfFiles < 1 {
				logf("Skipping test for file size %v as calculated number of files is 0\n", size)
				continue
			} else if numberOfFiles > fcap {
				numberOfFiles = fcap
				logf("Capping test for file size %v to %v files\n", size, fcap)
			}

			logf("Running test for %d files of size %v\n", numberOfFiles, size)
			s, err := speedTest(ctx, numberOfFiles, size, args[0])
			if err != nil {
				return fmt.Errorf("speed test failed: %w", err)
			}
			results = append(results, s)

		}

		if useJSON {
			b, err := json.MarshalIndent(results, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal results to JSON: %w", err)
			}
			fmt.Println(string(b))
		}

		return nil
	},
}

// Stats of a speed test
type Stats struct {
	Size          fs.SizeSuffix
	NumberOfFiles int
	Upload        TestResult
	Download      TestResult
}

// TestResult of a speed test operation
type TestResult struct {
	Bytes    int64
	Duration time.Duration
	Speed    fs.SizeSuffix
}

// measures stats for speedTest operations
func measure(desc string, f func() error, size fs.SizeSuffix, numberOfFiles int, tr *TestResult) error {
	start := time.Now()
	err := f()
	dt := time.Since(start)
	if err != nil {
		return err
	}
	tr.Duration = dt
	tr.Bytes = int64(size) * int64(numberOfFiles)
	tr.Speed = fs.SizeSuffix(float64(tr.Bytes) / dt.Seconds())
	logf("%-20s: %vB in %v at %vB/s\n", desc, tr.Bytes, dt.Round(time.Millisecond), tr.Speed)
	return err
}

func speedTest(ctx context.Context, numberOfFiles int, size fs.SizeSuffix, remote string) (*Stats, error) {
	stats := Stats{
		Size:          size,
		NumberOfFiles: numberOfFiles,
	}

	tempDirName := "rclone-speed-test-" + random.String(8)
	tempDirPath := path.Join(remote, tempDirName)
	fremote := cmd.NewFsDir([]string{tempDirPath})
	aErr := io.EOF
	defer atexit.OnError(&aErr, func() {
		err := operations.Purge(ctx, fremote, "")
		if err != nil {
			fs.Debugf(fremote, "Failed to remove temp dir %q: %v", tempDirPath, err)
		}
	})()

	flocalDir, err := os.MkdirTemp("", "rclone-speedtest-local-")
	if err != nil {
		return nil, fmt.Errorf("failed to create local temp dir: %w", err)
	}
	defer atexit.OnError(&aErr, func() { _ = os.RemoveAll(flocalDir) })()

	flocal, err := cache.Get(ctx, flocalDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create local fs: %w", err)
	}

	fdownloadDir, err := os.MkdirTemp("", "rclone-speedtest-download-")
	if err != nil {
		return nil, fmt.Errorf("failed to create download temp dir: %w", err)
	}
	defer atexit.OnError(&aErr, func() { _ = os.RemoveAll(fdownloadDir) })()

	fdownload, err := cache.Get(ctx, fdownloadDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create download fs: %w", err)
	}

	// make the largest amount of files we will need
	files := make([]string, numberOfFiles)
	for i := range files {
		files[i] = path.Join(flocalDir, fmt.Sprintf("file%03d-%v.bin", i, size))
	}
	makefiles(size, files)

	// upload files
	err = measure("Upload", func() error {
		return sync.CopyDir(ctx, fremote, flocal, false)
	}, size, numberOfFiles, &stats.Upload)
	if err != nil {
		return nil, fmt.Errorf("failed to Copy to remote: %w", err)
	}

	// download files
	err = measure("Download", func() error {
		return sync.CopyDir(ctx, fdownload, fremote, false)
	}, size, numberOfFiles, &stats.Download)
	if err != nil {
		return nil, fmt.Errorf("failed to Copy from remote: %w", err)
	}

	// check files
	opt := operations.CheckOpt{
		Fsrc:   flocal,
		Fdst:   fdownload,
		OneWay: false,
	}
	logf("Checking file integrity\n")
	err = operations.CheckDownload(ctx, &opt)
	if err != nil {
		return nil, fmt.Errorf("failed to check redownloaded files were identical: %w", err)
	}

	return &stats, nil
}
