package main

import (
	"flag"
	"github.com/sevlyar/go-daemon"
	"log"
	"os"
	"syscall"
	"time"
)

var (
	signal = flag.String("s", "", `send signal to the daemon
        stop — shutdown`)
)

const logFileName = "log"

func main() {
	flag.Parse()
	daemon.AddCommand(daemon.StringFlag(signal, "stop"), syscall.SIGTERM, termHandler)

	cntxt := &daemon.Context{
		PidFileName: "pid",
		PidFilePerm: 0644,
		LogFileName: logFileName,
		LogFilePerm: 0640,
		WorkDir:     "./",
		Umask:       027,
		Args:        []string{"[go-daemon sample]"},
	}

	if len(daemon.ActiveFlags()) > 0 {
		d, err := cntxt.Search()
		if err != nil {
			log.Fatalln("Unable send signal to the daemon:", err)
		}
		daemon.SendCommands(d)
		return
	}

	d, err := cntxt.Reborn()
	if err != nil {
		log.Fatalln(err)
	}
	if d != nil {
		return
	}
	defer cntxt.Release()

	log.Println("- - - - - - - - - - - - - - -")
	log.Println("daemon started")

	setupLog()

	go worker()

	err = daemon.ServeSignals()
	if err != nil {
		log.Println("Error:", err)
	}
	log.Println("daemon terminated")
}

func setupLog() {
	lf, err := NewLogFile(logFileName, os.Stderr)
	if err != nil {
		log.Fatal("Unable to create log file: ", err)
	}
	log.SetOutput(lf)
	// rotate log every 30 seconds.
	rotateLogSignal := time.Tick(30 * time.Second)
	go func() {
		for {
			<-rotateLogSignal
			if err := lf.Rotate(); err != nil {
				log.Fatal("Unable to rotate log: ", err)
			}
		}
	}()
}

var (
	stop = make(chan struct{})
	done = make(chan struct{})
)

func worker() {
LOOP:
	for {
		// spam to log every one second (as payload).
		log.Print("+ ", time.Now().Unix())
		time.Sleep(time.Second)
		select {
		case <-stop:
			break LOOP
		default:
		}
	}
	done <- struct{}{}
}

func termHandler(sig os.Signal) error {
	log.Println("terminating...")
	stop <- struct{}{}
	if sig == syscall.SIGQUIT {
		<-done
	}
	return daemon.ErrStop
}
