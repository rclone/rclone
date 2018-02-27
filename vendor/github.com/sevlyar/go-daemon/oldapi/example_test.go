package daemon_test

import (
	"fmt"
	"github.com/sevlyar/go-daemon/oldapi"
	"log"
	"os"
	"syscall"
)

func ExampleReborn() {
	err := daemon.Reborn(027, "/")
	if err != nil {
		log.Println("Error:", err)
		os.Exit(1)
	}

	daemon.ServeSignals()
}

func ExampleRedirectStream() {
	file, err := os.OpenFile("/tmp/daemon-log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		os.Exit(1)
	}

	if err = daemon.RedirectStream(os.Stdout, file); err != nil {
		os.Exit(2)
	}
	if err = daemon.RedirectStream(os.Stderr, file); err != nil {
		os.Exit(2)
	}
	file.Close()

	fmt.Println("some message")
	log.Println("some message")
}

func ExampleServeSignals() {
	TermHandler := func(sig os.Signal) error {
		log.Println("SIGTERM:", sig)
		return daemon.ErrStop
	}

	HupHandler := func(sig os.Signal) error {
		log.Println("SIGHUP:", sig)
		return nil
	}

	daemon.SetHandler(TermHandler, syscall.SIGTERM, syscall.SIGKILL)
	daemon.SetHandler(HupHandler, syscall.SIGHUP)

	err := daemon.ServeSignals()
	if err != nil {
		log.Println("Error:", err)
	}
}

func ExampleLockPidFile() {
	pidf, err := daemon.LockPidFile("name.pid", 0600)
	if err != nil {
		if err == daemon.ErrWouldBlock {
			log.Println("daemon already exists")
		} else {
			log.Println("pid file creation error:", err)
		}
		return
	}
	defer pidf.Unlock()

	daemon.ServeSignals()
}
