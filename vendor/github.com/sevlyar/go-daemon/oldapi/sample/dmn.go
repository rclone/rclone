package main

import (
	"encoding/json"
	"flag"
	daemon "github.com/sevlyar/go-daemon/oldapi"
	"log"
	"os"
	"syscall"
	"time"
)

const (
	pidFileName = "dmn.pid"
	logFileName = "dmn.log"

	fileMask = 0600
)
const (
	ret_OK = iota
	ret_ALREADYRUN
	ret_PIDFERROR
	ret_REBORNERROR
	ret_CONFERROR
)

var (
	status = flag.Bool("status", false,
		`Check status of the daemon. The program immediately exits after these 
		checks with either a return code of 0 (Daemon Stopped) or return code 
		not equal to 0 (Daemon Running)`)

	silent = flag.Bool("silent", false, "Don't write in stdout")

	test = flag.Bool("t", false,
		`Run syntax tests for configuration files only. The program 
		immediately exits after these syntax parsing tests with either 
		a return code of 0 (Syntax OK) or return code not equal to 0 
		(Syntax Error)`)

	configFileName = flag.String("f", "dmn.conf",
		`Specifies the name of the configuration file. The default is dmn.conf. 
		Daemon refuses to start if there is no configuration file.`)
)

var confProv = make(chan Config, 8)

func main() {
	flag.Parse()

	setupLogging()

	conf, err := loadConfig(*configFileName)
	if err != nil {
		log.Println("Config error:", err)
		os.Exit(ret_CONFERROR)
	}
	if *test {
		os.Exit(ret_OK)
	}

	pidf := lockPidFile()
	err = daemon.Reborn(027, "./")
	if err != nil {
		log.Println("Reborn error:", err)
		os.Exit(ret_REBORNERROR)
	}

	confProv <- conf
	go watchdog(confProv)

	serveSignals()

	pidf.Unlock()
}

func setupLogging() {
	if daemon.WasReborn() {
		file, _ := os.OpenFile(logFileName, os.O_WRONLY|os.O_CREATE|os.O_APPEND, fileMask)
		daemon.RedirectStream(os.Stdout, file)
		daemon.RedirectStream(os.Stderr, file)
		file.Close()
		log.Println("--- log ---")
	} else {
		log.SetFlags(0)
		if *silent {
			file, _ := os.OpenFile(os.DevNull, os.O_WRONLY, fileMask)
			daemon.RedirectStream(os.Stdout, file)
			daemon.RedirectStream(os.Stderr, file)
			file.Close()
		}
	}
}

type Config []string

func loadConfig(path string) (config Config, err error) {
	var file *os.File
	file, err = os.OpenFile(path, os.O_RDONLY, 0700)
	if err != nil {
		return
	}
	defer file.Close()

	config = make([]string, 0)
	err = json.NewDecoder(file).Decode(&config)
	if err != nil {
		return
	}
	for _, path = range config {
		if _, err = os.Stat(path); os.IsNotExist(err) {
			return
		}
	}

	return
}

func lockPidFile() *daemon.PidFile {
	pidf, err := daemon.LockPidFile(pidFileName, fileMask)
	if err != nil {
		if err == daemon.ErrWouldBlock {
			log.Println("daemon copy is already running")
			os.Exit(ret_ALREADYRUN)
		} else {
			log.Println("pid file creation error:", err)
			os.Exit(ret_PIDFERROR)
		}
	}

	if !daemon.WasReborn() {
		pidf.Unlock()
	}

	if *status {
		os.Exit(ret_OK)
	}

	return pidf
}

func watchdog(confProv <-chan Config) {
	states := make(map[string]time.Time)
	conf := <-confProv
	for {
		select {
		case conf = <-confProv:
		default:
		}

		for _, path := range conf {
			fi, err := os.Stat(path)
			if err != nil {
				log.Println(err)
				continue
			}

			cur := fi.ModTime()
			if pre, exists := states[path]; exists {
				if pre != cur {
					log.Printf("file %s modified at %s", path, cur)
				}
			}
			states[path] = cur
		}
		time.Sleep(time.Second)
	}
}

func serveSignals() {
	daemon.SetHandler(termHandler, syscall.SIGTERM, syscall.SIGKILL)
	daemon.SetHandler(hupHandler, syscall.SIGHUP)

	err := daemon.ServeSignals()
	if err != nil {
		log.Println("Error:", err)
	}

	log.Println("--- end ---")
}

func termHandler(sig os.Signal) error {
	log.Println("SIGTERM:", sig)
	return daemon.ErrStop
}

func hupHandler(sig os.Signal) error {
	log.Println("SIGHUP:", sig)

	conf, err := loadConfig(*configFileName)
	if err != nil {
		log.Println("Config error:", err)
	} else {
		confProv <- conf
	}

	return nil
}
