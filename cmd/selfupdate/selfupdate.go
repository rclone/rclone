package selfupdate

import (
	"archive/zip"
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/spf13/cobra"

	versionCmd "github.com/rclone/rclone/cmd/version"
)

// Options contains options for the self-update command
type Options struct {
	Check   bool
	Output  string
	Beta    bool
	Stable  bool
	Version string
}

// Opt is options set via command line
var Opt = Options{}

func init() {
	cmd.Root.AddCommand(cmdSelfUpdate)
	cmdFlags := cmdSelfUpdate.Flags()
	flags.BoolVarP(cmdFlags, &Opt.Check, "check", "", Opt.Check, "Check for latest release, do not download.")
	flags.StringVarP(cmdFlags, &Opt.Output, "output", "", Opt.Output, "Save the downloaded binary at a given path (default: replace running binary)")
	flags.BoolVarP(cmdFlags, &Opt.Stable, "stable", "", Opt.Stable, "Install latest stable release (this is the default)")
	flags.BoolVarP(cmdFlags, &Opt.Beta, "beta", "", Opt.Beta, "Install latest beta release.")
	flags.StringVarP(cmdFlags, &Opt.Version, "version", "", Opt.Version, "Install the given rclone path (default: auto-detect)")
}

var cmdSelfUpdate = &cobra.Command{
	Use:     "selfupdate",
	Aliases: []string{"self-update"},
	Short:   `Update the rclone binary.`,
	Long: `
This command downloads the latest release of rclone and replaces
the currently running binary. The download is verified with a hashsum.

If you previously installed rclone via a package manager, the package may
include local documentation or configure services. This command will update
only rclone executable so the local manual may become inaccurate after it.

Note: Windows forbids deletion of a currently running executable so this
command will rename the old executable to 'rclone.exe.old' upon success.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(0, 0, command, args)
		if Opt.Check {
			if Opt.Stable || Opt.Beta || Opt.Output != "" || Opt.Version != "" {
				fmt.Println("Warning: --stable, --beta, --version and --output are ignored with --check")
			}
			versionCmd.CheckVersion()
			return
		}
		if err := InstallUpdate(&Opt); err != nil {
			log.Fatalf("Error: %v", err)
		}
	},
}

// GetVersion is a wrapper for versionCmd.GetVersion with extra outputs
func GetVersion(beta bool, version string) (newVersion, siteURL string, err error) {
	siteURL = "https://downloads.rclone.org"
	if beta {
		siteURL = "https://beta.rclone.org"
	}
	newVersion = version
	if newVersion == "" {
		_, newVersion, _, err = versionCmd.GetVersion(siteURL + "/version.txt")
	}
	return
}

// InstallUpdate performs rclone self-update
func InstallUpdate(opt *Options) error {
	// Find the latest release number
	if opt.Stable && opt.Beta {
		return errors.New("--stable and --beta are mutually exclusive")
	}

	newVersion, siteURL, err := GetVersion(opt.Beta, opt.Version)
	if err != nil {
		return errors.Wrap(err, "unable to detect new version")
	}

	if newVersion == "" {
		var err error
		_, newVersion, _, err = versionCmd.GetVersion(siteURL + "/version.txt")
		if err != nil {
			return errors.Wrap(err, "unable to detect new version")
		}
	}

	if newVersion == fs.Version {
		fmt.Println("rclone is up to date")
		return nil
	}

	// Find the executable path
	executable, err := os.Executable()
	if err != nil {
		return errors.Wrap(err, "unable to find executable")
	}

	targetFile := opt.Output
	if targetFile == "" {
		targetFile = executable
	}

	// Check for possible access errors in advance
	newFile := targetFile + ".new"
	savedFile := ""
	if runtime.GOOS == "windows" {
		savedFile = targetFile + ".old"
	}

	if savedFile == executable || newFile == executable {
		return fmt.Errorf("%s: a temporary file would overwrite executable, specify a different --output", targetFile)
	}

	if err := verifyAccess(targetFile); err != nil {
		return err
	}

	// Download the update as a temporary file
	if err := downloadUpdate(opt.Beta, newVersion, siteURL, newFile); err != nil {
		return errors.Wrap(err, "failed to update rclone")
	}

	// Copy permission bits from the old executable
	fileMode := os.FileMode(0755)
	if fileInfo, err := os.Lstat(targetFile); err == nil {
		fileMode = fileInfo.Mode()
	}
	if err := os.Chmod(newFile, fileMode); err != nil {
		return errors.Wrap(err, "failed to set permissions")
	}

	// Replace current executable by the new file
	if err = os.Remove(targetFile); os.IsNotExist(err) {
		err = nil
	}
	if err != nil && savedFile != "" {
		// Windows forbids removal of a running executable so we rename it
		var saveErr error
		if saveErr = os.Remove(savedFile); os.IsNotExist(saveErr) {
			saveErr = nil
		}
		if saveErr == nil {
			saveErr = os.Rename(targetFile, savedFile)
		}
		if saveErr == nil {
			fmt.Printf("The old executable was saved as %s\n", savedFile)
			err = nil
		} else {
			// The rename trick didn't work out, proceed like on Unix
			_ = os.Remove(savedFile)
		}
	}
	if err == nil {
		err = os.Rename(newFile, targetFile)
	}
	if err != nil {
		_ = os.Remove(newFile)
		return err
	}
	fmt.Printf("Successfully updated rclone to version %s\n", newVersion)
	return nil
}

func downloadUpdate(beta bool, version, siteURL, newFile string) error {
	archiveFilename := fmt.Sprintf("rclone-%s-%s-%s.zip", version, runtime.GOOS, runtime.GOARCH)
	archiveURL := fmt.Sprintf("%s/%s/%s", siteURL, version, archiveFilename)
	archiveBuf, err := downloadFile(archiveURL)
	if err != nil {
		return err
	}
	fs.Debugf(nil, "downloaded release archive: %s", archiveURL)
	gotHash := sha256.Sum256(archiveBuf)
	fs.Debugf(nil, "archive hashsum: %s", hex.EncodeToString(gotHash[:]))

	// CI/CD does not provide hashsums for beta releases
	if !beta {
		hashsumsURL := fmt.Sprintf("%s/%s/SHA256SUMS", siteURL, version)
		hashsumsBuf, err := downloadFile(hashsumsURL)
		if err != nil {
			return err
		}
		fs.Debugf(nil, "downloaded hashsum list: %s", hashsumsURL)

		wantHash, err := findFileHash(hashsumsBuf, archiveFilename)
		if err != nil {
			return err
		}

		if !bytes.Equal(wantHash, gotHash[:]) {
			return fmt.Errorf("hash mismatch: want %02x vs got %02x", wantHash, gotHash)
		}
	}

	entryName := fmt.Sprintf("rclone-%s-%s-%s/rclone", version, runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		entryName += ".exe"
	}

	// Extract executable to a temporary file, then replace it by an instant rename
	err = extractZipToFile(archiveBuf, entryName, newFile)
	if err != nil {
		return err
	}
	fs.Debugf(nil, "extracted %s to %s", entryName, newFile)
	return nil
}

func verifyAccess(file string) error {
	admin := "root"
	if runtime.GOOS == "windows" {
		admin = "Administrator"
	}

	fileInfo, fileErr := os.Lstat(file)

	if fileErr != nil {
		dir := filepath.Dir(file)
		dirInfo, dirErr := os.Lstat(dir)
		if dirErr != nil {
			return dirErr
		}
		if !dirInfo.Mode().IsDir() {
			return fmt.Errorf("%s: parent path is not a directory, specify a different path using --output", dir)
		}
		if !writable(dir) {
			return fmt.Errorf("%s: directory is not writable, please run self-update as %s", dir, admin)
		}
	}

	if fileErr == nil && !fileInfo.Mode().IsRegular() {
		return fmt.Errorf("%s: path is not a normal file, specify a different path using --output", file)
	}

	if fileErr == nil && !writable(file) {
		return fmt.Errorf("%s: file is not writable, run self-update as %s", file, admin)
	}

	return nil
}

func findFileHash(buf []byte, filename string) (hash []byte, err error) {
	lines := bufio.NewScanner(bytes.NewReader(buf))
	for lines.Scan() {
		tokens := strings.Split(lines.Text(), "  ")
		if len(tokens) == 2 && tokens[1] == filename {
			if hash, err := hex.DecodeString(tokens[0]); err == nil {
				return hash, nil
			}
		}
	}
	return nil, fmt.Errorf("%s: unable to find hash", filename)
}

func extractZipToFile(buf []byte, entryName, newFile string) error {
	zipReader, err := zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		return err
	}

	var reader io.ReadCloser
	for _, entry := range zipReader.File {
		if entry.Name == entryName {
			reader, err = entry.Open()
			break
		}
	}
	if reader == nil || err != nil {
		return fmt.Errorf("%s: file not found in archive", entryName)
	}
	defer func() {
		_ = reader.Close()
	}()

	err = os.Remove(newFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("%s: unable to create new file: %v", newFile, err)
	}
	writer, err := os.OpenFile(newFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.FileMode(0755))
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, reader)
	_ = writer.Close()
	if err != nil {
		_ = os.Remove(newFile)
	}
	return err
}

func downloadFile(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer fs.CheckClose(resp.Body, &err)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed with %s downloading %s", resp.Status, url)
	}
	return ioutil.ReadAll(resp.Body)
}
