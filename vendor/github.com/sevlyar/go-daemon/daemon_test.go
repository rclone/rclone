package daemon

import (
	"flag"
	"log"
	"os"
	"syscall"
	"time"
)

func Example() {
	signal := flag.String("s", "", "send signal to daemon")

	handler := func(sig os.Signal) error {
		log.Println("signal:", sig)
		if sig == syscall.SIGTERM {
			return ErrStop
		}
		return nil
	}

	// Define command: command-line arg, system signal and handler
	AddCommand(StringFlag(signal, "term"), syscall.SIGTERM, handler)
	AddCommand(StringFlag(signal, "reload"), syscall.SIGHUP, handler)
	flag.Parse()

	// Define daemon context
	dmn := &Context{
		PidFileName: "/var/run/daemon.pid",
		PidFilePerm: 0644,
		LogFileName: "/var/log/daemon.log",
		LogFilePerm: 0640,
		WorkDir:     "/",
		Umask:       027,
	}

	// Send commands if needed
	if len(ActiveFlags()) > 0 {
		d, err := dmn.Search()
		if err != nil {
			log.Fatalln("Unable send signal to the daemon:", err)
		}
		SendCommands(d)
		return
	}

	// Process daemon operations - send signal if present flag or daemonize
	child, err := dmn.Reborn()
	if err != nil {
		log.Fatalln(err)
	}
	if child != nil {
		return
	}
	defer dmn.Release()

	// Run main operation
	go func() {
		for {
			time.Sleep(0)
		}
	}()

	err = ServeSignals()
	if err != nil {
		log.Println("Error:", err)
	}
}
