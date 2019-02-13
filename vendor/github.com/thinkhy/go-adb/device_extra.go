package adb

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	retryablehttp "github.com/hashicorp/go-retryablehttp"
)

type Process struct {
	User string
	Pid  int
	Name string
}

// ListProcesses return list of Process
func (c *Device) ListProcesses() (ps []Process, err error) {
	reader, err := c.OpenCommand("ps")
	if err != nil {
		return
	}
	defer reader.Close()
	var fieldNames []string
	bufrd := bufio.NewReader(reader)
	for {
		line, _, err := bufrd.ReadLine()
		fields := strings.Fields(strings.TrimSpace(string(line)))
		if len(fields) == 0 {
			break
		}
		if err == io.EOF {
			break
		}
		if fieldNames == nil {
			fieldNames = fields
			continue
		}
		var process Process
		/* example output of command "ps"
		USER     PID   PPID  VSIZE  RSS     WCHAN    PC         NAME
		root      1     0     684    540   ffffffff 00000000 S /init
		root      2     0     0      0     ffffffff 00000000 S kthreadd
		*/
		if len(fields) != len(fieldNames)+1 {
			continue
		}
		for index, name := range fieldNames {
			value := fields[index]
			switch strings.ToUpper(name) {
			case "PID":
				process.Pid, _ = strconv.Atoi(value)
			case "NAME":
				process.Name = fields[len(fields)-1]
			case "USER":
				process.User = value
			}
		}
		if process.Pid == 0 {
			continue
		}
		ps = append(ps, process)
	}
	return
}

// KillProcessByName return if killed success
func (c *Device) KillProcessByName(name string, sig int) error {
	ps, err := c.ListProcesses()
	if err != nil {
		return err
	}
	for _, p := range ps {
		if p.Name != name {
			continue
		}
		// log.Printf("kill %s with pid: %d", p.Name, p.Pid)
		_, _, er := c.RunCommandWithExitCode("kill", "-"+strconv.Itoa(sig), strconv.Itoa(p.Pid))
		if er != nil {
			return er
		}
	}
	return nil
}

type PackageInfo struct {
	Name    string
	Path    string
	Version struct {
		Code int
		Name string
	}
}

var (
	rePkgPath = regexp.MustCompile(`codePath=([^\s]+)`)
	reVerCode = regexp.MustCompile(`versionCode=(\d+)`)
	reVerName = regexp.MustCompile(`versionName=([^\s]+)`)
)

// StatPackage returns PackageInfo
// If package not found, err will be ErrPackageNotExist
func (c *Device) StatPackage(packageName string) (pi PackageInfo, err error) {
	pi.Name = packageName
	out, err := c.RunCommand("dumpsys", "package", packageName)
	if err != nil {
		return
	}

	matches := rePkgPath.FindStringSubmatch(out)
	if len(matches) == 0 {
		err = ErrPackageNotExist
		return
	}
	pi.Path = matches[1]

	matches = reVerCode.FindStringSubmatch(out)
	if len(matches) == 0 {
		err = ErrPackageNotExist
		return
	}
	pi.Version.Code, _ = strconv.Atoi(matches[1])

	matches = reVerName.FindStringSubmatch(out)
	if len(matches) == 0 {
		err = ErrPackageNotExist
		return
	}
	pi.Version.Name = matches[1]
	return
}

// Properties extract info from $ adb shell getprop
func (c *Device) Properties() (props map[string]string, err error) {
	propOutput, err := c.RunCommand("getprop")
	if err != nil {
		return nil, err
	}
	re := regexp.MustCompile(`\[(.*?)\]:\s*\[(.*?)\]`)
	matches := re.FindAllStringSubmatch(propOutput, -1)
	props = make(map[string]string)
	for _, m := range matches {
		var key = m[1]
		var val = m[2]
		props[key] = val
	}
	return
}

/*
RunCommandWithExitCode use a little tricky to get exit code

The tricky is append "; echo :$?" to the command,
and parse out the exit code from output
*/
func (c *Device) RunCommandWithExitCode(cmd string, args ...string) (string, int, error) {
	exArgs := append(args, ";", "echo", ":$?")
	outStr, err := c.RunCommand(cmd, exArgs...)
	if err != nil {
		return outStr, 0, err
	}
	idx := strings.LastIndexByte(outStr, ':')
	if idx == -1 {
		return outStr, 0, fmt.Errorf("adb shell aborted, can not parse exit code")
	}
	exitCode, _ := strconv.Atoi(strings.TrimSpace(outStr[idx+1:]))
	if exitCode != 0 {
		commandLine, _ := prepareCommandLine(cmd, args...)
		err = ShellExitError{commandLine, exitCode}
	}
	outStr = strings.Replace(outStr[0:idx], "\r\n", "\n", -1) // put somewhere else
	return outStr, exitCode, err
}

type ShellExitError struct {
	Command  string
	ExitCode int
}

func (s ShellExitError) Error() string {
	return fmt.Sprintf("shell %s exit code %d", strconv.Quote(s.Command), s.ExitCode)
}

// DoWriteFile return an object, use this object can Cancel write and get Process
func (c *Device) DoSyncFile(path string, rd io.ReadCloser, size int64, perms os.FileMode) (aw *AsyncWriter, err error) {
	dst, err := c.OpenWrite(path, perms, time.Now())
	if err != nil {
		return nil, err
	}
	awr := newAsyncWriter(c, dst, path, size)
	go func() {
		awr.doCopy(rd)
		rd.Close()
	}()
	return awr, nil
}

func (c *Device) DoSyncLocalFile(dst string, src string, perms os.FileMode) (aw *AsyncWriter, err error) {
	f, err := os.Open(src)
	if err != nil {
		return
	}

	finfo, err := f.Stat()
	if err != nil {
		return
	}
	return c.DoSyncFile(dst, f, finfo.Size(), perms)
}

func (c *Device) DoSyncHTTPFile(dst string, srcUrl string, perms os.FileMode) (aw *AsyncWriter, err error) {
	res, err := retryablehttp.Get(srcUrl)

	if err != nil {
		return
	}
	var length int64
	fmt.Sscanf(res.Header.Get("Content-Length"), "%d", &length)
	return c.DoSyncFile(dst, res.Body, length, perms)
}

// WriteToFile write a reader stream to device
func (c *Device) WriteToFile(path string, rd io.Reader, perms os.FileMode) (written int64, err error) {
	dst, err := c.OpenWrite(path, perms, time.Now())
	if err != nil {
		return
	}
	defer func() {
		dst.Close()
		if err != nil || written == 0 {
			return
		}
		// wait until write finished.
		fromTime := time.Now()
		for {
			if time.Since(fromTime) > time.Second*600 {
				err = fmt.Errorf("write file to device timeout (10min)")
				return
			}
			finfo, er := c.Stat(path)
			if er != nil && !HasErrCode(er, FileNoExistError) {
				err = er
				return
			}
			if finfo == nil {
				err = fmt.Errorf("target file %s not created", strconv.Quote(path))
				return
			}
			if finfo != nil && finfo.Size == int32(written) {
				break
			}
			time.Sleep(time.Duration(200+rand.Intn(100)) * time.Millisecond)
		}
	}()
	written, err = io.Copy(dst, rd)
	return
}

// WriteHttpToFile download http resource to device
func (c *Device) WriteHttpToFile(path string, urlStr string, perms os.FileMode) (written int64, err error) {
	res, err := retryablehttp.Get(urlStr)
	if err != nil {
		return
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("http download <%s> status %v", urlStr, res.Status)
		return
	}
	return c.WriteToFile(path, res.Body, perms)
}
