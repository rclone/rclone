//go:build ignore
// +build ignore

// Cross compile rclone - in go because I hate bash ;-)

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/coreos/go-semver/semver"
)

var (
	// Flags
	debug           = flag.Bool("d", false, "Print commands instead of running them.")
	parallel        = flag.Int("parallel", runtime.NumCPU(), "Number of commands to run in parallel.")
	copyAs          = flag.String("release", "", "Make copies of the releases with this name")
	gitLog          = flag.String("git-log", "", "git log to include as well")
	include         = flag.String("include", "^.*$", "os/arch regexp to include")
	exclude         = flag.String("exclude", "^$", "os/arch regexp to exclude")
	cgo             = flag.Bool("cgo", false, "Use cgo for the build")
	noClean         = flag.Bool("no-clean", false, "Don't clean the build directory before running.")
	tags            = flag.String("tags", "", "Space separated list of build tags")
	buildmode       = flag.String("buildmode", "", "Passed to go build -buildmode flag")
	compileOnly     = flag.Bool("compile-only", false, "Just build the binary, not the zip.")
	extraEnv        = flag.String("env", "", "comma separated list of VAR=VALUE env vars to set")
	macOSSDK        = flag.String("macos-sdk", "", "macOS SDK to use")
	macOSArch       = flag.String("macos-arch", "", "macOS arch to use")
	extraCgoCFlags  = flag.String("cgo-cflags", "", "extra CGO_CFLAGS")
	extraCgoLdFlags = flag.String("cgo-ldflags", "", "extra CGO_LDFLAGS")
)

// GOOS/GOARCH pairs we build for
//
// If the GOARCH contains a - it is a synthetic arch with more parameters
var osarches = []string{
	"windows/386",
	"windows/amd64",
	"windows/arm64",
	"darwin/amd64",
	"darwin/arm64",
	"linux/386",
	"linux/amd64",
	"linux/arm",
	"linux/arm-v7",
	"linux/arm64",
	"linux/mips",
	"linux/mipsle",
	"freebsd/386",
	"freebsd/amd64",
	"freebsd/arm",
	"freebsd/arm-v7",
	"netbsd/386",
	"netbsd/amd64",
	"netbsd/arm",
	"netbsd/arm-v7",
	"openbsd/386",
	"openbsd/amd64",
	"plan9/386",
	"plan9/amd64",
	"solaris/amd64",
	"js/wasm",
}

// Special environment flags for a given arch
var archFlags = map[string][]string{
	"386":    {"GO386=softfloat"},
	"mips":   {"GOMIPS=softfloat"},
	"mipsle": {"GOMIPS=softfloat"},
	"arm-v7": {"GOARM=7"},
}

// Map Go architectures to NFPM architectures
// Any missing are passed straight through
var goarchToNfpm = map[string]string{
	"arm":    "arm6",
	"arm-v7": "arm7",
}

// runEnv - run a shell command with env
func runEnv(args, env []string) error {
	if *debug {
		args = append([]string{"echo"}, args...)
	}
	cmd := exec.Command(args[0], args[1:]...)
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	if *debug {
		log.Printf("args = %v, env = %v\n", args, cmd.Env)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Print("----------------------------")
		log.Printf("Failed to run %v: %v", args, err)
		log.Printf("Command output was:\n%s", out)
		log.Print("----------------------------")
	}
	return err
}

// run a shell command
func run(args ...string) {
	err := runEnv(args, nil)
	if err != nil {
		log.Fatalf("Exiting after error: %v", err)
	}
}

// chdir or die
func chdir(dir string) {
	err := os.Chdir(dir)
	if err != nil {
		log.Fatalf("Couldn't cd into %q: %v", dir, err)
	}
}

// substitute data from go template file in to file out
func substitute(inFile, outFile string, data interface{}) {
	t, err := template.ParseFiles(inFile)
	if err != nil {
		log.Fatalf("Failed to read template file %q: %v %v", inFile, err)
	}
	out, err := os.Create(outFile)
	if err != nil {
		log.Fatalf("Failed to create output file %q: %v %v", outFile, err)
	}
	defer func() {
		err := out.Close()
		if err != nil {
			log.Fatalf("Failed to close output file %q: %v %v", outFile, err)
		}
	}()
	err = t.Execute(out, data)
	if err != nil {
		log.Fatalf("Failed to substitute template file %q: %v %v", inFile, err)
	}
}

// build the zip package return its name
func buildZip(dir string) string {
	// Now build the zip
	run("cp", "-a", "../MANUAL.txt", filepath.Join(dir, "README.txt"))
	run("cp", "-a", "../MANUAL.html", filepath.Join(dir, "README.html"))
	run("cp", "-a", "../rclone.1", dir)
	if *gitLog != "" {
		run("cp", "-a", *gitLog, dir)
	}
	zip := dir + ".zip"
	run("zip", "-r9", zip, dir)
	return zip
}

// Build .deb and .rpm packages
//
// It returns a list of artifacts it has made
func buildDebAndRpm(dir, version, goarch string) []string {
	// Make internal version number acceptable to .deb and .rpm
	pkgVersion := version[1:]
	pkgVersion = strings.ReplaceAll(pkgVersion, "β", "-beta")
	pkgVersion = strings.ReplaceAll(pkgVersion, "-", ".")
	nfpmArch, ok := goarchToNfpm[goarch]
	if !ok {
		nfpmArch = goarch
	}

	// Make nfpm.yaml from the template
	substitute("../bin/nfpm.yaml", path.Join(dir, "nfpm.yaml"), map[string]string{
		"Version": pkgVersion,
		"Arch":    nfpmArch,
	})

	// build them
	var artifacts []string
	for _, pkg := range []string{".deb", ".rpm"} {
		artifact := dir + pkg
		run("bash", "-c", "cd "+dir+" && nfpm -f nfpm.yaml pkg -t ../"+artifact)
		artifacts = append(artifacts, artifact)
	}

	return artifacts
}

// generate system object (syso) file to be picked up by a following go build for embedding icon and version info resources into windows executable
func buildWindowsResourceSyso(goarch string, versionTag string) string {
	type M map[string]interface{}
	version := strings.TrimPrefix(versionTag, "v")
	semanticVersion := semver.New(version)

	// Build json input to goversioninfo utility
	bs, err := json.Marshal(M{
		"FixedFileInfo": M{
			"FileVersion": M{
				"Major": semanticVersion.Major,
				"Minor": semanticVersion.Minor,
				"Patch": semanticVersion.Patch,
			},
			"ProductVersion": M{
				"Major": semanticVersion.Major,
				"Minor": semanticVersion.Minor,
				"Patch": semanticVersion.Patch,
			},
		},
		"StringFileInfo": M{
			"CompanyName":      "https://rclone.org",
			"ProductName":      "Rclone",
			"FileDescription":  "Rsync for cloud storage",
			"InternalName":     "rclone",
			"OriginalFilename": "rclone.exe",
			"LegalCopyright":   "The Rclone Authors",
			"FileVersion":      version,
			"ProductVersion":   version,
		},
		"IconPath": "../graphics/logo/ico/logo_symbol_color.ico",
	})
	if err != nil {
		log.Printf("Failed to build version info json: %v", err)
		return ""
	}

	// Write json to temporary file that will only be used by the goversioninfo command executed below.
	jsonPath, err := filepath.Abs("versioninfo_windows_" + goarch + ".json") // Appending goos and goarch as suffix to avoid any race conditions
	if err != nil {
		log.Printf("Failed to resolve path: %v", err)
		return ""
	}
	err = ioutil.WriteFile(jsonPath, bs, 0644)
	if err != nil {
		log.Printf("Failed to write %s: %v", jsonPath, err)
		return ""
	}
	defer func() {
		if err := os.Remove(jsonPath); err != nil {
			if !os.IsNotExist(err) {
				log.Printf("Warning: Couldn't remove generated %s: %v. Please remove it manually.", jsonPath, err)
			}
		}
	}()

	// Execute goversioninfo utility using the json file as input.
	// It will produce a system object (syso) file that a following go build should pick up.
	sysoPath, err := filepath.Abs("../resource_windows_" + goarch + ".syso") // Appending goos and goarch as suffix to avoid any race conditions, and also it is recognized by go build and avoids any builds for other systems considering it
	if err != nil {
		log.Printf("Failed to resolve path: %v", err)
		return ""
	}
	args := []string{
		"goversioninfo",
		"-o",
		sysoPath,
	}
	if strings.Contains(goarch, "64") {
		args = append(args, "-64") // Make the syso a 64-bit coff file
	}
	if strings.Contains(goarch, "arm") {
		args = append(args, "-arm") // Make the syso an arm binary
	}
	args = append(args, jsonPath)
	err = runEnv(args, nil)
	if err != nil {
		return ""
	}

	return sysoPath
}

// delete generated system object (syso) resource file
func cleanupResourceSyso(sysoFilePath string) {
	if sysoFilePath == "" {
		return
	}
	if err := os.Remove(sysoFilePath); err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Warning: Couldn't remove generated %s: %v. Please remove it manually.", sysoFilePath, err)
		}
	}
}

// Trip a version suffix off the arch if present
func stripVersion(goarch string) string {
	i := strings.Index(goarch, "-")
	if i < 0 {
		return goarch
	}
	return goarch[:i]
}

// run the command returning trimmed output
func runOut(command ...string) string {
	out, err := exec.Command(command[0], command[1:]...).Output()
	if err != nil {
		log.Fatalf("Failed to run %q: %v", command, err)
	}
	return strings.TrimSpace(string(out))
}

// build the binary in dir returning success or failure
func compileArch(version, goos, goarch, dir string) bool {
	log.Printf("Compiling %s/%s into %s", goos, goarch, dir)
	output := filepath.Join(dir, "rclone")
	if goos == "windows" {
		output += ".exe"
		sysoPath := buildWindowsResourceSyso(goarch, version)
		if sysoPath == "" {
			log.Printf("Warning: Windows binaries will not have file information embedded")
		}
		defer cleanupResourceSyso(sysoPath)
	}
	err := os.MkdirAll(dir, 0777)
	if err != nil {
		log.Fatalf("Failed to mkdir: %v", err)
	}
	args := []string{
		"go", "build",
		"--ldflags", "-s -X github.com/rclone/rclone/fs.Version=" + version,
		"-trimpath",
		"-o", output,
		"-tags", *tags,
	}
	if *buildmode != "" {
		args = append(args,
			"-buildmode", *buildmode,
		)
	}
	args = append(args,
		"..",
	)
	env := []string{
		"GOOS=" + goos,
		"GOARCH=" + stripVersion(goarch),
	}
	if *extraEnv != "" {
		env = append(env, strings.Split(*extraEnv, ",")...)
	}
	var (
		cgoCFlags  []string
		cgoLdFlags []string
	)
	if *macOSSDK != "" {
		flag := "-isysroot " + runOut("xcrun", "--sdk", *macOSSDK, "--show-sdk-path")
		cgoCFlags = append(cgoCFlags, flag)
		cgoLdFlags = append(cgoLdFlags, flag)
	}
	if *macOSArch != "" {
		flag := "-arch " + *macOSArch
		cgoCFlags = append(cgoCFlags, flag)
		cgoLdFlags = append(cgoLdFlags, flag)
	}
	if *extraCgoCFlags != "" {
		cgoCFlags = append(cgoCFlags, *extraCgoCFlags)
	}
	if *extraCgoLdFlags != "" {
		cgoLdFlags = append(cgoLdFlags, *extraCgoLdFlags)
	}
	if len(cgoCFlags) > 0 {
		env = append(env, "CGO_CFLAGS="+strings.Join(cgoCFlags, " "))
	}
	if len(cgoLdFlags) > 0 {
		env = append(env, "CGO_LDFLAGS="+strings.Join(cgoLdFlags, " "))
	}
	if !*cgo {
		env = append(env, "CGO_ENABLED=0")
	} else {
		env = append(env, "CGO_ENABLED=1")
	}
	if flags, ok := archFlags[goarch]; ok {
		env = append(env, flags...)
	}
	err = runEnv(args, env)
	if err != nil {
		log.Printf("Error compiling %s/%s: %v", goos, goarch, err)
		return false
	}
	if !*compileOnly {
		if goos != "js" {
			artifacts := []string{buildZip(dir)}
			// build a .deb and .rpm if appropriate
			if goos == "linux" {
				artifacts = append(artifacts, buildDebAndRpm(dir, version, goarch)...)
			}
			if *copyAs != "" {
				for _, artifact := range artifacts {
					run("ln", artifact, strings.Replace(artifact, "-"+version, "-"+*copyAs, 1))
				}
			}
		}
		// tidy up
		run("rm", "-rf", dir)
	}
	log.Printf("Done compiling %s/%s", goos, goarch)
	return true
}

func compile(version string) {
	start := time.Now()
	wg := new(sync.WaitGroup)
	run := make(chan func(), *parallel)
	for i := 0; i < *parallel; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range run {
				f()
			}
		}()
	}
	includeRe, err := regexp.Compile(*include)
	if err != nil {
		log.Fatalf("Bad -include regexp: %v", err)
	}
	excludeRe, err := regexp.Compile(*exclude)
	if err != nil {
		log.Fatalf("Bad -exclude regexp: %v", err)
	}
	compiled := 0
	var failuresMu sync.Mutex
	var failures []string
	for _, osarch := range osarches {
		if excludeRe.MatchString(osarch) || !includeRe.MatchString(osarch) {
			continue
		}
		parts := strings.Split(osarch, "/")
		if len(parts) != 2 {
			log.Fatalf("Bad osarch %q", osarch)
		}
		goos, goarch := parts[0], parts[1]
		userGoos := goos
		if goos == "darwin" {
			userGoos = "osx"
		}
		dir := filepath.Join("rclone-" + version + "-" + userGoos + "-" + goarch)
		run <- func() {
			if !compileArch(version, goos, goarch, dir) {
				failuresMu.Lock()
				failures = append(failures, goos+"/"+goarch)
				failuresMu.Unlock()
			}
		}
		compiled++
	}
	close(run)
	wg.Wait()
	log.Printf("Compiled %d arches in %v", compiled, time.Since(start))
	if len(failures) > 0 {
		sort.Strings(failures)
		log.Printf("%d compile failures:\n  %s\n", len(failures), strings.Join(failures, "\n  "))
		os.Exit(1)
	}
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		log.Fatalf("Syntax: %s <version>", os.Args[0])
	}
	version := args[0]
	if !*noClean {
		run("rm", "-rf", "build")
		run("mkdir", "build")
	}
	chdir("build")
	err := ioutil.WriteFile("version.txt", []byte(fmt.Sprintf("rclone %s\n", version)), 0666)
	if err != nil {
		log.Fatalf("Couldn't write version.txt: %v", err)
	}
	compile(version)
}
