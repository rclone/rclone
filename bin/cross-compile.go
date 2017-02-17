// +build ignore

// Cross compile rclone - in go because I hate bash ;-)

package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	// Flags
	debug    = flag.Bool("d", false, "Print commands instead of running them.")
	parallel = flag.Int("parallel", runtime.NumCPU(), "Number of commands to run in parallel.")
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
	err := os.MkdirAll(dir, 0777)
	if err != nil {
		log.Fatalf("Failed to mkdir: %v", err)
	}
	args := []string{
		"go", "build",
		"--ldflags", "-s -X github.com/ncw/rclone/fs.Version=" + version,
		"-i",
		"-o", output,
		"..",
	}
	env := []string{
		"GOOS=" + goos,
		"GOARCH=" + goarch,
		"CGO_ENABLED=0",
	}
	runEnv(args, env)
	// Now build the zip
	run("cp", "-a", "../MANUAL.txt", filepath.Join(dir, "README.txt"))
	run("cp", "-a", "../MANUAL.html", filepath.Join(dir, "README.html"))
	run("cp", "-a", "../rclone.1", dir)
	zip := dir + ".zip"
	run("zip", "-r9", zip, dir)
	currentZip := strings.Replace(zip, "-"+version, "-current", 1)
	run("ln", zip, currentZip)
	run("rm", "-rf", dir)
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
	for _, osarch := range osarches {
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
	}
	close(run)
	wg.Wait()
	log.Printf("Compiled %d arches in %v", len(osarches), time.Since(start))
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		log.Fatalf("Syntax: %s <version>", os.Args[0])
	}
	version := args[0]
	run("rm", "-rf", "build")
	run("mkdir", "build")
	err := os.Chdir("build")
	if err != nil {
		log.Fatalf("Couldn't cd into build dir: %v", err)
	}
	compile(version)
}
