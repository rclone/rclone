// Utility program to generate Rclone-specific Windows resource system object
// file (.syso), that can be picked up by a following go build for embedding
// version information and icon resources into a rclone binary.
//
// Run it with "go generate", or "go run" to be able to customize with
// command-line flags. Note that this program is intended to be run directly
// from its original location in the source tree: Default paths are absolute
// within the current source tree, which is convenient because it makes it
// oblivious to the working directory, and it gives identical result whether
// run by "go generate" or "go run", but it will not make sense if this
// program's source is moved out from the source tree.
//
// Can be used for rclone.exe (default), and other binaries such as
// librclone.dll (must be specified with flag -binary).
//

//go:generate go run resource_windows.go
//go:build tools

package main

import (
	"flag"
	"fmt"
	"log"
	"path"
	"runtime"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/josephspurrier/goversioninfo"
	"github.com/rclone/rclone/fs"
)

func main() {
	// Get path of directory containing the current source file to use for absolute path references within the code tree (as described above)
	projectDir := ""
	_, sourceFile, _, ok := runtime.Caller(0)
	if ok {
		projectDir = path.Dir(path.Dir(sourceFile)) // Root of the current project working directory
	}

	// Define flags
	binary := flag.String("binary", "rclone.exe", `The name of the binary to generate resource for, e.g. "rclone.exe" or "librclone.dll"`)
	arch := flag.String("arch", runtime.GOARCH, `Architecture of resource file, or the target GOARCH, "386", "amd64", "arm", or "arm64"`)
	version := flag.String("version", fs.Version, "Version number or tag name")
	icon := flag.String("icon", path.Join(projectDir, "graphics/logo/ico/logo_symbol_color.ico"), "Path to icon file to embed in an .exe binary")
	dir := flag.String("dir", projectDir, "Path to output directory where to write the resulting system object file (.syso), with a default name according to -arch (resource_windows_<arch>.syso), only considered if not -syso is specified")
	syso := flag.String("syso", "", "Path to output resource system object file (.syso) to be created/overwritten, ignores -dir")

	// Parse command-line flags
	flag.Parse()

	// Handle default value for -file which depends on optional -dir and -arch
	if *syso == "" {
		// Use default filename, which includes target GOOS (hardcoded "windows")
		// and GOARCH (from argument -arch) as suffix, to avoid any race conditions,
		// and also this will be recognized by go build when it is consuming the
		// .syso file and will only be used for builds with matching os/arch.
		*syso = path.Join(*dir, fmt.Sprintf("resource_windows_%s.syso", *arch))
	}

	// Parse version/tag string argument as a SemVer
	stringVersion := strings.TrimPrefix(*version, "v")
	semanticVersion, err := semver.NewVersion(stringVersion)
	if err != nil {
		log.Fatalf("Invalid version number: %v", err)
	}

	// Extract binary extension
	binaryExt := path.Ext(*binary)

	// Create the version info configuration container
	vi := &goversioninfo.VersionInfo{}

	// FixedFileInfo
	vi.FixedFileInfo.FileOS = "040004" // VOS_NT_WINDOWS32
	if strings.EqualFold(binaryExt, ".exe") {
		vi.FixedFileInfo.FileType = "01" // VFT_APP
	} else if strings.EqualFold(binaryExt, ".dll") {
		vi.FixedFileInfo.FileType = "02" // VFT_DLL
	} else {
		log.Fatalf("Specified binary must have extension .exe or .dll")
	}
	// FixedFileInfo.FileVersion
	vi.FixedFileInfo.FileVersion.Major = int(semanticVersion.Major)
	vi.FixedFileInfo.FileVersion.Minor = int(semanticVersion.Minor)
	vi.FixedFileInfo.FileVersion.Patch = int(semanticVersion.Patch)
	vi.FixedFileInfo.FileVersion.Build = 0
	// FixedFileInfo.ProductVersion
	vi.FixedFileInfo.ProductVersion.Major = int(semanticVersion.Major)
	vi.FixedFileInfo.ProductVersion.Minor = int(semanticVersion.Minor)
	vi.FixedFileInfo.ProductVersion.Patch = int(semanticVersion.Patch)
	vi.FixedFileInfo.ProductVersion.Build = 0

	// StringFileInfo
	vi.StringFileInfo.CompanyName = "https://rclone.org"
	vi.StringFileInfo.ProductName = "Rclone"
	vi.StringFileInfo.FileDescription = "Rclone"
	vi.StringFileInfo.InternalName = (*binary)[:len(*binary)-len(binaryExt)]
	vi.StringFileInfo.OriginalFilename = *binary
	vi.StringFileInfo.LegalCopyright = "The Rclone Authors"
	vi.StringFileInfo.FileVersion = stringVersion
	vi.StringFileInfo.ProductVersion = stringVersion

	// Icon (only relevant for .exe, not .dll)
	if *icon != "" && strings.EqualFold(binaryExt, ".exe") {
		vi.IconPath = *icon
	}

	// Build native structures from the configuration data
	vi.Build()

	// Write the native structures as binary data to a buffer
	vi.Walk()

	// Write the binary data buffer to file
	if err := vi.WriteSyso(*syso, *arch); err != nil {
		log.Fatalf(`Failed to generate Windows %s resource system object file for %v with path "%v": %v`, *arch, *binary, *syso, err)
	}
}
