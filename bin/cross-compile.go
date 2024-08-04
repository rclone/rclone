//go:build ignore

// Cross compile rclone - in go because I hate bash ;-)

package main

import (
	"flag"
	"fmt"
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
)

var (
	// Flags
	debug           = flag.Bool("d", false, "Print commands instead of running them")
	parallel        = flag.Int("parallel", runtime.NumCPU(), "Number of commands to run in parallel")
	copyAs          = flag.String("release", "", "Make copies of the releases with this name")
	gitLog          = flag.String("git-log", "", "git log to include as well")
	include         = flag.String("include", "^.*$", "os/arch regexp to include")
	exclude         = flag.String("exclude", "^$", "os/arch regexp to exclude")
	cgo             = flag.Bool("cgo", false, "Use cgo for the build")
	noClean         = flag.Bool("no-clean", false, "Don't clean the build directory before running")
	tags            = flag.String("tags", "", "Space separated list of build tags")
	buildmode       = flag.String("buildmode", "", "Passed to go build -buildmode flag")
	compileOnly     = flag.Bool("compile-only", false, "Just build the binary, not the zip")
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
	"linux/arm-v6",
	"linux/arm-v7",
	"linux/arm64",
	"linux/mips",
	"linux/mipsle",
	"freebsd/386",
	"freebsd/amd64",
	"freebsd/arm",
	"freebsd/arm-v6",
	"freebsd/arm-v7",
	"netbsd/386",
	"netbsd/amd64",
	"netbsd/arm",
	"netbsd/arm-v6",
	"netbsd/arm-v7",
	"openbsd/386",
	"openbsd/amd64",
	"plan9/386",
	"plan9/amd64",
	"solaris/amd64",
	// "js/wasm", // Rclone is too big for js/wasm until https://github.com/golang/go/issues/64856 is fixed
}

// Special environment flags for a given arch
var archFlags = map[string][]string{
	"386":    {"GO386=softfloat"},
	"mips":   {"GOMIPS=softfloat"},
	"mipsle": {"GOMIPS=softfloat"},
	"arm":    {"GOARM=5"},
	"arm-v6": {"GOARM=6"},
	"arm-v7": {"GOARM=7"},
}

// Map Go architectures to NFPM architectures
// Any missing are passed straight through
var goarchToNfpm = map[string]string{
	"arm":    "arm5",
	"arm-v6": "arm6",
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
		log.Fatalf("Failed to read template file %q: %v", inFile, err)
	}
	out, err := os.Create(outFile)
	if err != nil {
		log.Fatalf("Failed to create output file %q: %v", outFile, err)
	}
	defer func() {
		err := out.Close()
		if err != nil {
			log.Fatalf("Failed to close output file %q: %v", outFile, err)
		}
	}()
	err = t.Execute(out, data)
	if err != nil {
		log.Fatalf("Failed to substitute template file %q: %v", inFile, err)
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

// Generate Windows resource system object file (.syso), which can be picked
// up by the following go build for embedding version information and icon
// resources into the executable.
func generateResourceWindows(version, arch string) func() {
	sysoPath := fmt.Sprintf("../resource_windows_%s.syso", arch) // Use explicit destination filename, even though it should be same as default, so that we are sure we have the correct reference to it
	if err := os.Remove(sysoPath); !os.IsNotExist(err) {
		// Note: This one we choose to treat as fatal, to avoid any risk of picking up an old .syso file without noticing.
		log.Fatalf("Failed to remove existing Windows %s resource system object file %s: %v", arch, sysoPath, err)
	}
	args := []string{"go", "run", "../bin/resource_windows.go", "-arch", arch, "-version", version, "-syso", sysoPath}
	if err := runEnv(args, nil); err != nil {
		log.Printf("Warning: Couldn't generate Windows %s resource system object file, binaries will not have version information or icon embedded", arch)
		return nil
	}
	if _, err := os.Stat(sysoPath); err != nil {
		log.Printf("Warning: Couldn't find generated Windows %s resource system object file, binaries will not have version information or icon embedded", arch)
		return nil
	}
	return func() {
		if err := os.Remove(sysoPath); err != nil && !os.IsNotExist(err) {
			log.Printf("Warning: Couldn't remove generated Windows %s resource system object file %s: %v. Please remove it manually.", arch, sysoPath, err)
		}
	}
}

// build the binary in dir returning success or failure
func compileArch(version, goos, goarch, dir string) bool {
	log.Printf("Compiling %s/%s into %s", goos, goarch, dir)
	goarchBase := stripVersion(goarch)
	output := filepath.Join(dir, "rclone")
	if goos == "windows" {
		output += ".exe"
		if cleanupFn := generateResourceWindows(version, goarchBase); cleanupFn != nil {
			defer cleanupFn()
		}
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
		"GOARCH=" + goarchBase,
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
	err := os.WriteFile("version.txt", []byte(fmt.Sprintf("rclone %s\n", version)), 0666)
	if err != nil {
		log.Fatalf("Couldn't write version.txt: %v", err)
	}
	compile(version)
}
