// +build ignore

// Cross compile rclone - in go because I hate bash ;-)

package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	// Flags
	debug       = flag.Bool("d", false, "Print commands instead of running them.")
	parallel    = flag.Int("parallel", runtime.NumCPU(), "Number of commands to run in parallel.")
	copyAs      = flag.String("release", "", "Make copies of the releases with this name")
	gitLog      = flag.String("git-log", "", "git log to include as well")
	include     = flag.String("include", "^.*$", "os/arch regexp to include")
	exclude     = flag.String("exclude", "^$", "os/arch regexp to exclude")
	cgo         = flag.Bool("cgo", false, "Use cgo for the build")
	noClean     = flag.Bool("no-clean", false, "Don't clean the build directory before running.")
	tags        = flag.String("tags", "", "Space separated list of build tags")
	compileOnly = flag.Bool("compile-only", false, "Just build the binary, not the zip.")
)

// GOOS/GOARCH pairs we build for
var osarches = []string{
	"windows/386",
	"windows/amd64",
	"darwin/386",
	"darwin/amd64",
	"linux/386",
	"linux/amd64",
	"linux/arm",
	"linux/arm64",
	"linux/mips",
	"linux/mipsle",
	"freebsd/386",
	"freebsd/amd64",
	"freebsd/arm",
	"netbsd/386",
	"netbsd/amd64",
	"netbsd/arm",
	"openbsd/386",
	"openbsd/amd64",
	"plan9/386",
	"plan9/amd64",
	"solaris/amd64",
}

// Special environment flags for a given arch
var archFlags = map[string][]string{
	"386": {"GO386=387"},
}

// runEnv - run a shell command with env
func runEnv(args, env []string) {
	if *debug {
		args = append([]string{"echo"}, args...)
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	if *debug {
		log.Printf("args = %v, env = %v\n", args, cmd.Env)
	}
	err := cmd.Run()
	if err != nil {
		log.Fatalf("Failed to run %v: %v", args, err)
	}
}

// run a shell command
func run(args ...string) {
	runEnv(args, nil)
}

// build the binary in dir
func compileArch(version, goos, goarch, dir string) {
	log.Printf("Compiling %s/%s", goos, goarch)
	output := filepath.Join(dir, "rclone")
	if goos == "windows" {
		output += ".exe"
	}
	err := os.MkdirAll(dir, 0777)
	if err != nil {
		log.Fatalf("Failed to mkdir: %v", err)
	}
	args := []string{
		"go", "build",
		"--ldflags", "-s -X github.com/ncw/rclone/fs.Version=" + version,
		"-i",
		"-o", output,
		"-tags", *tags,
		"..",
	}
	env := []string{
		"GOOS=" + goos,
		"GOARCH=" + goarch,
	}
	if !*cgo {
		env = append(env, "CGO_ENABLED=0")
	} else {
		env = append(env, "CGO_ENABLED=1")
	}
	if flags, ok := archFlags[goarch]; ok {
		env = append(env, flags...)
	}
	runEnv(args, env)
	if !*compileOnly {
		// Now build the zip
		run("cp", "-a", "../MANUAL.txt", filepath.Join(dir, "README.txt"))
		run("cp", "-a", "../MANUAL.html", filepath.Join(dir, "README.html"))
		run("cp", "-a", "../rclone.1", dir)
		if *gitLog != "" {
			run("cp", "-a", *gitLog, dir)
		}
		zip := dir + ".zip"
		run("zip", "-r9", zip, dir)
		if *copyAs != "" {
			copyAsZip := strings.Replace(zip, "-"+version, "-"+*copyAs, 1)
			run("ln", zip, copyAsZip)
		}
		run("rm", "-rf", dir)
	}
	log.Printf("Done compiling %s/%s", goos, goarch)
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
			compileArch(version, goos, goarch, dir)
		}
		compiled++
	}
	close(run)
	wg.Wait()
	log.Printf("Compiled %d arches in %v", compiled, time.Since(start))
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
	err := os.Chdir("build")
	if err != nil {
		log.Fatalf("Couldn't cd into build dir: %v", err)
	}
	compile(version)
}
