package main

// small wrapper around sftp server that allows it to be used as a separate process subsystem call by the ssh server.
// in practice this will statically link; however this allows unit testing from the sftp client.

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/pkg/sftp"
)

func main() {
	var (
		readOnly    bool
		debugStderr bool
		debugLevel  string
		options     []sftp.ServerOption
	)

	flag.BoolVar(&readOnly, "R", false, "read-only server")
	flag.BoolVar(&debugStderr, "e", false, "debug to stderr")
	flag.StringVar(&debugLevel, "l", "none", "debug level (ignored)")
	flag.Parse()

	debugStream := ioutil.Discard
	if debugStderr {
		debugStream = os.Stderr
	}
	options = append(options, sftp.WithDebug(debugStream))

	if readOnly {
		options = append(options, sftp.ReadOnly())
	}

	svr, _ := sftp.NewServer(
		struct {
			io.Reader
			io.WriteCloser
		}{os.Stdin,
			os.Stdout,
		},
		options...,
	)
	if err := svr.Serve(); err != nil {
		fmt.Fprintf(debugStream, "sftp server completed with error: %v", err)
		os.Exit(1)
	}
}
