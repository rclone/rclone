package selfupdate

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/cmount"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/lib/buildinfo"
	"github.com/rclone/rclone/lib/random"
	"github.com/spf13/cobra"

	versionCmd "github.com/rclone/rclone/cmd/version"
)

// Options contains options for the self-update command
type Options struct {
	Check   bool
	Output  string // output path
	Beta    bool   // mutually exclusive with Stable (false means "stable")
	Stable  bool   // mutually exclusive with Beta
	Version string
	Package string // package format: zip, deb, rpm (empty string means "zip")
}

// Opt is options set via command line
var Opt = Options{}

func init() {
	cmd.Root.AddCommand(cmdSelfUpdate)
	cmdFlags := cmdSelfUpdate.Flags()
	flags.BoolVarP(cmdFlags, &Opt.Check, "check", "", Opt.Check, "Check for latest release, do not download.")
	flags.StringVarP(cmdFlags, &Opt.Output, "output", "", Opt.Output, "Save the downloaded binary at a given path (default: replace running binary)")
	flags.BoolVarP(cmdFlags, &Opt.Stable, "stable", "", Opt.Stable, "Install stable release (this is the default)")
	flags.BoolVarP(cmdFlags, &Opt.Beta, "beta", "", Opt.Beta, "Install beta release.")
	flags.StringVarP(cmdFlags, &Opt.Version, "version", "", Opt.Version, "Install the given rclone version (default: latest)")
	flags.StringVarP(cmdFlags, &Opt.Package, "package", "", Opt.Package, "Package format: zip|deb|rpm (default: zip)")
}

var cmdSelfUpdate = &cobra.Command{
	Use:     "selfupdate",
	Aliases: []string{"self-update"},
	Short:   `Update the rclone binary.`,
	Long:    strings.ReplaceAll(selfUpdateHelp, "|", "`"),
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(0, 0, command, args)
		if Opt.Package == "" {
			Opt.Package = "zip"
		}
		gotActionFlags := Opt.Stable || Opt.Beta || Opt.Output != "" || Opt.Version != "" || Opt.Package != "zip"
		if Opt.Check && !gotActionFlags {
			versionCmd.CheckVersion()
			return
		}
		if Opt.Package != "zip" {
			if Opt.Package != "deb" && Opt.Package != "rpm" {
				log.Fatalf("--package should be one of zip|deb|rpm")
			}
			if runtime.GOOS != "linux" {
				log.Fatalf(".deb and .rpm packages are supported only on Linux")
			} else if os.Geteuid() != 0 && !Opt.Check {
				log.Fatalf(".deb and .rpm must be installed by root")
			}
			if Opt.Output != "" && !Opt.Check {
				fmt.Println("Warning: --output is ignored with --package deb|rpm")
			}
		}
		if err := InstallUpdate(context.Background(), &Opt); err != nil {
			log.Fatalf("Error: %v", err)
		}
	},
}

// GetVersion can get the latest release number from the download site
// or massage a stable release number - prepend semantic "v" prefix
// or find the latest micro release for a given major.minor release.
// Note: this will not be applied to beta releases.
func GetVersion(ctx context.Context, beta bool, version string) (newVersion, siteURL string, err error) {
	siteURL = "https://downloads.rclone.org"
	if beta {
		siteURL = "https://beta.rclone.org"
	}

	if version == "" {
		// Request the latest release number from the download site
		_, newVersion, _, err = versionCmd.GetVersion(siteURL + "/version.txt")
		return
	}

	newVersion = version
	if version[0] != 'v' {
		newVersion = "v" + version
	}
	if beta {
		return
	}

	if valid, _ := regexp.MatchString(`^v\d+\.\d+(\.\d+)?$`, newVersion); !valid {
		return "", siteURL, errors.New("invalid semantic version")
	}

	// Find the latest stable micro release
	if strings.Count(newVersion, ".") == 1 {
		html, err := downloadFile(ctx, siteURL)
		if err != nil {
			return "", siteURL, errors.Wrap(err, "failed to get list of releases")
		}
		reSubver := fmt.Sprintf(`href="\./%s\.\d+/"`, regexp.QuoteMeta(newVersion))
		allSubvers := regexp.MustCompile(reSubver).FindAllString(string(html), -1)
		if allSubvers == nil {
			return "", siteURL, errors.New("could not find the minor release")
		}
		// Use the fact that releases in the index are sorted by date
		lastSubver := allSubvers[len(allSubvers)-1]
		newVersion = lastSubver[8 : len(lastSubver)-2]
	}
	return
}

// InstallUpdate performs rclone self-update
func InstallUpdate(ctx context.Context, opt *Options) error {
	// Find the latest release number
	if opt.Stable && opt.Beta {
		return errors.New("--stable and --beta are mutually exclusive")
	}

	gotCmount := false
	for _, tag := range buildinfo.Tags {
		if tag == "cmount" {
			gotCmount = true
			break
		}
	}
	if gotCmount && !cmount.ProvidedBy(runtime.GOOS) {
		return errors.New("updating would discard the mount FUSE capability, aborting")
	}

	newVersion, siteURL, err := GetVersion(ctx, opt.Beta, opt.Version)
	if err != nil {
		return errors.Wrap(err, "unable to detect new version")
	}

	oldVersion := fs.Version
	if newVersion == oldVersion {
		fmt.Println("rclone is up to date")
		return nil
	}

	// Install .deb/.rpm package if requested by user
	if opt.Package == "deb" || opt.Package == "rpm" {
		if opt.Check {
			fmt.Println("Warning: --package flag is ignored in --check mode")
		} else {
			err := installPackage(ctx, opt.Beta, newVersion, siteURL, opt.Package)
			if err == nil {
				fmt.Printf("Successfully updated rclone package from version %s to version %s\n", oldVersion, newVersion)
			}
			return err
		}
	}

	// Get the current executable path
	executable, err := os.Executable()
	if err != nil {
		return errors.Wrap(err, "unable to find executable")
	}

	targetFile := opt.Output
	if targetFile == "" {
		targetFile = executable
	}

	if opt.Check {
		fmt.Printf("Without --check this would install rclone version %s at %s\n", newVersion, targetFile)
		return nil
	}

	// Make temporary file names and check for possible access errors in advance
	var newFile string
	if newFile, err = makeRandomExeName(targetFile, "new"); err != nil {
		return err
	}
	savedFile := ""
	if runtime.GOOS == "windows" {
		savedFile = targetFile
		if strings.HasSuffix(savedFile, ".exe") {
			savedFile = savedFile[:len(savedFile)-4]
		}
		savedFile += ".old.exe"
	}

	if savedFile == executable || newFile == executable {
		return fmt.Errorf("%s: a temporary file would overwrite the executable, specify a different --output path", targetFile)
	}

	if err := verifyAccess(targetFile); err != nil {
		return err
	}

	// Download the update as a temporary file
	err = downloadUpdate(ctx, opt.Beta, newVersion, siteURL, newFile, "zip")
	if err != nil {
		return errors.Wrap(err, "failed to update rclone")
	}

	err = replaceExecutable(targetFile, newFile, savedFile)
	if err == nil {
		fmt.Printf("Successfully updated rclone from version %s to version %s\n", oldVersion, newVersion)
	}
	return err
}

func installPackage(ctx context.Context, beta bool, version, siteURL, packageFormat string) error {
	tempFile, err := ioutil.TempFile("", "rclone.*."+packageFormat)
	if err != nil {
		return errors.Wrap(err, "unable to write temporary package")
	}
	packageFile := tempFile.Name()
	_ = tempFile.Close()
	defer func() {
		if rmErr := os.Remove(packageFile); rmErr != nil {
			fs.Errorf(nil, "%s: could not remove temporary package: %v", packageFile, rmErr)
		}
	}()
	if err := downloadUpdate(ctx, beta, version, siteURL, packageFile, packageFormat); err != nil {
		return err
	}

	packageCommand := "dpkg"
	if packageFormat == "rpm" {
		packageCommand = "rpm"
	}
	cmd := exec.Command(packageCommand, "-i", packageFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run %s: %v", packageCommand, err)
	}
	return nil
}

func replaceExecutable(targetFile, newFile, savedFile string) error {
	// Copy permission bits from the old executable
	// (it was extracted with mode 0755)
	fileInfo, err := os.Lstat(targetFile)
	if err == nil {
		if err = os.Chmod(newFile, fileInfo.Mode()); err != nil {
			return errors.Wrap(err, "failed to set permission")
		}
	}

	if err = os.Remove(targetFile); os.IsNotExist(err) {
		err = nil
	}

	if err != nil && savedFile != "" {
		// Windows forbids removal of a running executable so we rename it.
		// For starters, rename download as the original file with ".old.exe" appended.
		var saveErr error
		if saveErr = os.Remove(savedFile); os.IsNotExist(saveErr) {
			saveErr = nil
		}
		if saveErr == nil {
			saveErr = os.Rename(targetFile, savedFile)
		}
		if saveErr != nil {
			// The ".old" file cannot be removed or cannot be renamed to.
			// This usually means that the running executable has a name with ".old".
			// This can happen in very rare cases, but we ought to handle it.
			// Try inserting a randomness in the name to mitigate it.
			fs.Debugf(nil, "%s: cannot replace old file, randomizing name", savedFile)

			savedFile, saveErr = makeRandomExeName(targetFile, "old")
			if saveErr == nil {
				if saveErr = os.Remove(savedFile); os.IsNotExist(saveErr) {
					saveErr = nil
				}
			}
			if saveErr == nil {
				saveErr = os.Rename(targetFile, savedFile)
			}
		}
		if saveErr == nil {
			fmt.Printf("The old executable was saved as %s\n", savedFile)
			err = nil
		}
	}

	if err == nil {
		err = os.Rename(newFile, targetFile)
	}
	if err != nil {
		if rmErr := os.Remove(newFile); rmErr != nil {
			fs.Errorf(nil, "%s: could not remove temporary file: %v", newFile, rmErr)
		}
		return err
	}
	return nil
}

func makeRandomExeName(baseName, extension string) (string, error) {
	const maxAttempts = 5

	if runtime.GOOS == "windows" {
		if strings.HasSuffix(baseName, ".exe") {
			baseName = baseName[:len(baseName)-4]
		}
		extension += ".exe"
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		filename := fmt.Sprintf("%s.%s.%s", baseName, random.String(4), extension)
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			return filename, nil
		}
	}

	return "", fmt.Errorf("cannot find a file name like %s.xxxx.%s", baseName, extension)
}

func downloadUpdate(ctx context.Context, beta bool, version, siteURL, newFile, packageFormat string) error {
	osName := runtime.GOOS
	arch := runtime.GOARCH
	if arch == "darwin" {
		arch = "osx"
	}

	archiveFilename := fmt.Sprintf("rclone-%s-%s-%s.%s", version, osName, arch, packageFormat)
	archiveURL := fmt.Sprintf("%s/%s/%s", siteURL, version, archiveFilename)
	archiveBuf, err := downloadFile(ctx, archiveURL)
	if err != nil {
		return err
	}
	gotHash := sha256.Sum256(archiveBuf)
	strHash := hex.EncodeToString(gotHash[:])
	fs.Debugf(nil, "downloaded release archive with hashsum %s from %s", strHash, archiveURL)

	// CI/CD does not provide hashsums for beta releases
	if !beta {
		if err := verifyHashsum(ctx, siteURL, version, archiveFilename, gotHash[:]); err != nil {
			return err
		}
	}

	if packageFormat == "deb" || packageFormat == "rpm" {
		if err := ioutil.WriteFile(newFile, archiveBuf, 0644); err != nil {
			return errors.Wrap(err, "cannot write temporary ."+packageFormat)
		}
		return nil
	}

	entryName := fmt.Sprintf("rclone-%s-%s-%s/rclone", version, osName, arch)
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
		if rmErr := os.Remove(newFile); rmErr != nil {
			fs.Errorf(nil, "%s: could not remove temporary file: %v", newFile, rmErr)
		}
	}
	return err
}

func downloadFile(ctx context.Context, url string) ([]byte, error) {
	resp, err := fshttp.NewClient(ctx).Get(url)
	if err != nil {
		return nil, err
	}
	defer fs.CheckClose(resp.Body, &err)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed with %s downloading %s", resp.Status, url)
	}
	return ioutil.ReadAll(resp.Body)
}
