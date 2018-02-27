// +build darwin dragonfly freebsd linux netbsd openbsd plan9 solaris

package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"syscall"

	"github.com/kardianos/osext"
)

func (d *Context) reborn() (child *os.Process, err error) {
	if !WasReborn() {
		child, err = d.parent()
	} else {
		err = d.child()
	}
	return
}

func (d *Context) search() (daemon *os.Process, err error) {
	if len(d.PidFileName) > 0 {
		var pid int
		if pid, err = ReadPidFile(d.PidFileName); err != nil {
			return
		}
		daemon, err = os.FindProcess(pid)
	}
	return
}

func (d *Context) parent() (child *os.Process, err error) {
	if err = d.prepareEnv(); err != nil {
		return
	}

	defer d.closeFiles()
	if err = d.openFiles(); err != nil {
		return
	}

	attr := &os.ProcAttr{
		Dir:   d.WorkDir,
		Env:   d.Env,
		Files: d.files(),
		Sys: &syscall.SysProcAttr{
			//Chroot:     d.Chroot,
			Credential: d.Credential,
			Setsid:     true,
		},
	}

	if child, err = os.StartProcess(d.abspath, d.Args, attr); err != nil {
		if d.pidFile != nil {
			d.pidFile.Remove()
		}
		return
	}

	d.rpipe.Close()
	encoder := json.NewEncoder(d.wpipe)
	err = encoder.Encode(d)

	return
}

func (d *Context) openFiles() (err error) {
	if d.PidFilePerm == 0 {
		d.PidFilePerm = FILE_PERM
	}
	if d.LogFilePerm == 0 {
		d.LogFilePerm = FILE_PERM
	}

	if d.nullFile, err = os.Open(os.DevNull); err != nil {
		return
	}

	if len(d.PidFileName) > 0 {
		if d.pidFile, err = OpenLockFile(d.PidFileName, d.PidFilePerm); err != nil {
			return
		}
		if err = d.pidFile.Lock(); err != nil {
			return
		}
	}

	if len(d.LogFileName) > 0 {
		if d.logFile, err = os.OpenFile(d.LogFileName,
			os.O_WRONLY|os.O_CREATE|os.O_APPEND, d.LogFilePerm); err != nil {
			return
		}
	}

	d.rpipe, d.wpipe, err = os.Pipe()
	return
}

func (d *Context) closeFiles() (err error) {
	cl := func(file **os.File) {
		if *file != nil {
			(*file).Close()
			*file = nil
		}
	}
	cl(&d.rpipe)
	cl(&d.wpipe)
	cl(&d.logFile)
	cl(&d.nullFile)
	if d.pidFile != nil {
		d.pidFile.Close()
		d.pidFile = nil
	}
	return
}

func (d *Context) prepareEnv() (err error) {
	if d.abspath, err = osext.Executable(); err != nil {
		return
	}

	if len(d.Args) == 0 {
		d.Args = os.Args
	}

	mark := fmt.Sprintf("%s=%s", MARK_NAME, MARK_VALUE)
	if len(d.Env) == 0 {
		d.Env = os.Environ()
	}
	d.Env = append(d.Env, mark)

	return
}

func (d *Context) files() (f []*os.File) {
	log := d.nullFile
	if d.logFile != nil {
		log = d.logFile
	}

	f = []*os.File{
		d.rpipe,    // (0) stdin
		log,        // (1) stdout
		log,        // (2) stderr
		d.nullFile, // (3) dup on fd 0 after initialization
	}

	if d.pidFile != nil {
		f = append(f, d.pidFile.File) // (4) pid file
	}
	return
}

var initialized = false

func (d *Context) child() (err error) {
	if initialized {
		return os.ErrInvalid
	}
	initialized = true

	if len(d.PidFileName) > 0 {
		d.pidFile = NewLockFile(os.NewFile(4, d.PidFileName))
		if err = d.pidFile.WritePid(); err != nil {
			return
		}
	}

	decoder := json.NewDecoder(os.Stdin)
	if err = decoder.Decode(d); err != nil {
		d.pidFile.Remove()
		return
	}

	if err = syscall.Close(0); err != nil {
		d.pidFile.Remove()
		return
	}
	if err = syscallDup(3, 0); err != nil {
		d.pidFile.Remove()
		return
	}

	if d.Umask != 0 {
		syscall.Umask(int(d.Umask))
	}
	if len(d.Chroot) > 0 {
		err = syscall.Chroot(d.Chroot)
		if err != nil {
			d.pidFile.Remove()
			return
		}
	}

	return
}

func (d *Context) release() (err error) {
	if !initialized {
		return
	}
	if d.pidFile != nil {
		err = d.pidFile.Remove()
	}
	return
}
