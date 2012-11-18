// Sync files and directories to and from swift
// 
// Nick Craig-Wood <nick@craig-wood.com>
package main

import (
	//"bytes"
	"flag"
	"fmt"
	//"io"
	//"io/ioutil"
	"log"
	//"math/rand"
	"os"
	//"os/signal"
	//"path/filepath"
	//"regexp"
	//"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	//"sync"
	//"syscall"
	//"time"
	"github.com/ncw/swift"
)

// Globals
var (
	// Flags
	//fileSize      = flag.Int64("s", 1E9, "Size of the check files")
	cpuprofile = flag.String("cpuprofile", "", "Write cpu profile to file")
	//duration      = flag.Duration("duration", time.Hour*24, "Duration to run test")
	//statsInterval = flag.Duration("stats", time.Minute*1, "Interval to print stats")
	//logfile       = flag.String("logfile", "stressdisk.log", "File to write log to set to empty to ignore")

	snet    = flag.Bool("snet", false, "Use internal service network") // FIXME not implemented
	verbose = flag.Bool("verbose", false, "Print lots more stuff")
	quiet   = flag.Bool("quiet", false, "Print as little stuff as possible")
	// FIXME make these part of swift so we get a standard set of flags?
	authUrl  = flag.String("auth", os.Getenv("ST_AUTH"), "Auth URL for server. Defaults to environment var ST_AUTH.")
	userName = flag.String("user", os.Getenv("ST_USER"), "User name. Defaults to environment var ST_USER.")
	apiKey   = flag.String("key", os.Getenv("ST_KEY"), "API key (password). Defaults to environment var ST_KEY.")
)

// Turns a number of ns into a floating point string in seconds
//
// Trims trailing zeros and guaranteed to be perfectly accurate
func nsToFloatString(ns int64) string {
	if ns < 0 {
		return "-" + nsToFloatString(-ns)
	}
	result := fmt.Sprintf("%010d", ns)
	split := len(result) - 9
	result, decimals := result[:split], result[split:]
	decimals = strings.TrimRight(decimals, "0")
	if decimals != "" {
		result += "."
		result += decimals
	}
	return result
}

// Turns a floating point string in seconds into a ns integer
//
// Guaranteed to be perfectly accurate
func floatStringToNs(s string) (ns int64, err error) {
	if s != "" && s[0] == '-' {
		ns, err = floatStringToNs(s[1:])
		return -ns, err
	}
	point := strings.IndexRune(s, '.')
	if point >= 0 {
		tail := s[point+1:]
		if len(tail) > 0 {
			if len(tail) > 9 {
				tail = tail[:9]
			}
			uns, err := strconv.ParseUint(tail, 10, 64)
			if err != nil {
				return 0, err
			}
			ns = int64(uns)
			for i := 9 - len(tail); i > 0; i-- {
				ns *= 10
			}
		}
		s = s[:point]
	}
	secs, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}
	ns += int64(1000000000) * secs
	return ns, nil
}

// syntaxError prints the syntax
func syntaxError() {
	fmt.Fprintf(os.Stderr, `Sync files and directores to and from swift

FIXME

Full options:
`)
	flag.PrintDefaults()
}

// Exit with the message
func fatal(message string, args ...interface{}) {
	syntaxError()
	fmt.Fprintf(os.Stderr, message, args...)
	os.Exit(1)
}

func main() {
	flag.Usage = syntaxError
	flag.Parse()
	//args := flag.Args()
	//runtime.GOMAXPROCS(3)

	// Setup profiling if desired
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	// if len(args) < 1 {
	// 	fatal("No command supplied\n")
	// }

	if *userName == "" {
		log.Fatal("Need --user or environmental variable ST_USER")
	}
	if *apiKey == "" {
		log.Fatal("Need --key or environmental variable ST_KEY")
	}
	if *authUrl == "" {
		log.Fatal("Need --auth or environmental variable ST_AUTH")
	}
	c := swift.Connection{
		UserName: *userName,
		ApiKey:   *apiKey,
		AuthUrl:  *authUrl,
	}
	err := c.Authenticate()
	if err != nil {
		log.Fatal("Failed to authenticate", err)
	}

}
