// Copyright 2018 The goftp Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package server

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// Command represents a Command interface to a ftp command
type Command interface {
	IsExtend() bool
	RequireParam() bool
	RequireAuth() bool
	Execute(*Session, string)
}

var (
	defaultCommands = map[string]Command{
		"ADAT": commandAdat{},
		"ALLO": commandAllo{},
		"APPE": commandAppe{},
		"AUTH": commandAuth{},
		"CDUP": commandCdup{},
		"CWD":  commandCwd{},
		"CCC":  commandCcc{},
		"CONF": commandConf{},
		"CLNT": commandCLNT{},
		"DELE": commandDele{},
		"ENC":  commandEnc{},
		"EPRT": commandEprt{},
		"EPSV": commandEpsv{},
		"FEAT": commandFeat{},
		"LIST": commandList{},
		"LPRT": commandLprt{},
		"NLST": commandNlst{},
		"MDTM": commandMdtm{},
		"MIC":  commandMic{},
		"MLSD": commandMLSD{},
		"MKD":  commandMkd{},
		"MODE": commandMode{},
		"NOOP": commandNoop{},
		"OPTS": commandOpts{},
		"PASS": commandPass{},
		"PASV": commandPasv{},
		"PBSZ": commandPbsz{},
		"PORT": commandPort{},
		"PROT": commandProt{},
		"PWD":  commandPwd{},
		"QUIT": commandQuit{},
		"RETR": commandRetr{},
		"REST": commandRest{},
		"RNFR": commandRnfr{},
		"RNTO": commandRnto{},
		"RMD":  commandRmd{},
		"SIZE": commandSize{},
		"STAT": commandStat{},
		"STOR": commandStor{},
		"STRU": commandStru{},
		"SYST": commandSyst{},
		"TYPE": commandType{},
		"USER": commandUser{},
		"XCUP": commandCdup{},
		"XCWD": commandCwd{},
		"XMKD": commandMkd{},
		"XPWD": commandPwd{},
		"XRMD": commandXRmd{},
	}
)

// DefaultCommands returns the default commands
func DefaultCommands() map[string]Command {
	return defaultCommands
}

// commandAllo responds to the ALLO FTP command.
//
// This is essentially a ping from the client so we just respond with an
// basic OK message.
type commandAllo struct{}

func (cmd commandAllo) IsExtend() bool {
	return false
}

func (cmd commandAllo) RequireParam() bool {
	return false
}

func (cmd commandAllo) RequireAuth() bool {
	return false
}

func (cmd commandAllo) Execute(sess *Session, param string) {
	sess.writeMessage(202, "Obsolete")
}

// commandAppe responds to the APPE FTP command. It allows the user to upload a
// new file but always append if file exists otherwise create one.
type commandAppe struct{}

func (cmd commandAppe) IsExtend() bool {
	return false
}

func (cmd commandAppe) RequireParam() bool {
	return true
}

func (cmd commandAppe) RequireAuth() bool {
	return true
}

func (cmd commandAppe) Execute(sess *Session, param string) {
	targetPath := sess.buildPath(param)
	sess.writeMessage(150, "Data transfer starting")

	if sess.preCommand != "REST" {
		sess.lastFilePos = -1
	}
	defer func() {
		sess.lastFilePos = -1
	}()

	var ctx = Context{
		Sess:  sess,
		Cmd:   "APPE",
		Param: param,
		Data:  make(map[string]interface{}),
	}
	sess.server.notifiers.BeforePutFile(&ctx, targetPath)
	size, err := sess.server.Driver.PutFile(&ctx, targetPath, sess.dataConn, sess.lastFilePos)
	sess.server.notifiers.AfterFilePut(&ctx, targetPath, size, err)
	if err == nil {
		msg := fmt.Sprintf("OK, received %d bytes", size)
		sess.writeMessage(226, msg)
	} else {
		sess.writeMessage(450, fmt.Sprint("error during transfer: ", err))
	}
}

type commandCLNT struct{}

func (cmd commandCLNT) IsExtend() bool {
	return true
}

func (cmd commandCLNT) RequireParam() bool {
	return false
}

func (cmd commandCLNT) RequireAuth() bool {
	return false
}

func (cmd commandCLNT) Execute(sess *Session, param string) {
	sess.clientSoft = param
	sess.writeMessage(200, "OK")
}

type commandOpts struct{}

func (cmd commandOpts) IsExtend() bool {
	return false
}

func (cmd commandOpts) RequireParam() bool {
	return false
}

func (cmd commandOpts) RequireAuth() bool {
	return false
}

func (cmd commandOpts) Execute(sess *Session, param string) {
	parts := strings.Fields(param)
	if len(parts) != 2 {
		sess.writeMessage(550, "Unknow params")
		return
	}
	if strings.ToUpper(parts[0]) != "UTF8" {
		sess.writeMessage(550, "Unknow params")
		return
	}

	if strings.ToUpper(parts[1]) == "ON" {
		sess.writeMessage(200, "UTF8 mode enabled")
	} else {
		sess.writeMessage(550, "Unsupported non-utf8 mode")
	}
}

type commandFeat struct{}

func (cmd commandFeat) IsExtend() bool {
	return false
}

func (cmd commandFeat) RequireParam() bool {
	return false
}

func (cmd commandFeat) RequireAuth() bool {
	return false
}

func (cmd commandFeat) Execute(sess *Session, param string) {
	sess.writeMessageMultiline(211, sess.server.feats)
}

// cmdCdup responds to the CDUP FTP command.
//
// Allows the client change their current directory to the parent.
type commandCdup struct{}

func (cmd commandCdup) IsExtend() bool {
	return false
}

func (cmd commandCdup) RequireParam() bool {
	return false
}

func (cmd commandCdup) RequireAuth() bool {
	return true
}

func (cmd commandCdup) Execute(sess *Session, param string) {
	otherCmd := &commandCwd{}
	otherCmd.Execute(sess, "..")
}

// commandCwd responds to the CWD FTP command. It allows the client to change the
// current working directory.
type commandCwd struct{}

func (cmd commandCwd) IsExtend() bool {
	return false
}

func (cmd commandCwd) RequireParam() bool {
	return true
}

func (cmd commandCwd) RequireAuth() bool {
	return true
}

func (cmd commandCwd) Execute(sess *Session, param string) {
	path := sess.buildPath(param)
	var ctx = Context{
		Sess:  sess,
		Cmd:   "CWD",
		Param: param,
		Data:  make(map[string]interface{}),
	}
	info, err := sess.server.Driver.Stat(&ctx, path)
	if err != nil {
		sess.logf("%v", err)
		sess.writeMessage(550, fmt.Sprint("Directory change to ", path, " failed."))
		return
	}
	if !info.IsDir() {
		sess.writeMessage(550, fmt.Sprint("Directory change to ", path, " is a file"))
		return
	}

	sess.server.notifiers.BeforeChangeCurDir(&ctx, sess.curDir, path)
	err = sess.changeCurDir(path)
	sess.server.notifiers.AfterCurDirChanged(&ctx, sess.curDir, path, err)
	if err == nil {
		sess.writeMessage(250, "Directory changed to "+path)
	} else {
		sess.logf("%v", err)
		sess.writeMessage(550, fmt.Sprint("Directory change to ", path, " failed."))
	}
}

// commandDele responds to the DELE FTP command. It allows the client to delete
// a file
type commandDele struct{}

func (cmd commandDele) IsExtend() bool {
	return false
}

func (cmd commandDele) RequireParam() bool {
	return true
}

func (cmd commandDele) RequireAuth() bool {
	return true
}

func (cmd commandDele) Execute(sess *Session, param string) {
	path := sess.buildPath(param)
	var ctx = Context{
		Sess:  sess,
		Cmd:   "DELE",
		Param: param,
		Data:  make(map[string]interface{}),
	}
	sess.server.notifiers.BeforeDeleteFile(&ctx, path)
	err := sess.server.Driver.DeleteFile(&ctx, path)
	sess.server.notifiers.AfterFileDeleted(&ctx, path, err)
	if err == nil {
		sess.writeMessage(250, "File deleted")
	} else {
		sess.logf("%v", err)
		sess.writeMessage(550, "File delete failed. ")
	}
}

// commandEprt responds to the EPRT FTP command. It allows the client to
// request an active data socket with more options than the original PORT
// command. It mainly adds ipv6 support.
type commandEprt struct{}

func (cmd commandEprt) IsExtend() bool {
	return true
}

func (cmd commandEprt) RequireParam() bool {
	return true
}

func (cmd commandEprt) RequireAuth() bool {
	return true
}

func (cmd commandEprt) Execute(sess *Session, param string) {
	delim := string(param[0:1])
	parts := strings.Split(param, delim)
	addressFamily, err := strconv.Atoi(parts[1])
	if err != nil {
		sess.writeMessage(522, "Network protocol not supported, use (1,2)")
		return
	}
	if addressFamily != 1 && addressFamily != 2 {
		sess.writeMessage(522, "Network protocol not supported, use (1,2)")
		return
	}

	host := parts[2]
	port, err := strconv.Atoi(parts[3])
	if err != nil {
		sess.writeMessage(522, "Network protocol not supported, use (1,2)")
		return
	}
	socket, err := newActiveSocket(sess, host, port)
	if err != nil {
		sess.writeMessage(425, "Data connection failed")
		return
	}
	sess.dataConn = socket
	sess.writeMessage(200, "Connection established ("+strconv.Itoa(port)+")")
}

// commandLprt responds to the LPRT FTP command. It allows the client to
// request an active data socket with more options than the original PORT
// command.  FTP Operation Over Big Address Records.
type commandLprt struct{}

func (cmd commandLprt) IsExtend() bool {
	return true
}

func (cmd commandLprt) RequireParam() bool {
	return true
}

func (cmd commandLprt) RequireAuth() bool {
	return true
}

func (cmd commandLprt) Execute(sess *Session, param string) {
	// No tests for this code yet

	parts := strings.Split(param, ",")

	addressFamily, err := strconv.Atoi(parts[0])
	if err != nil {
		sess.writeMessage(522, "Network protocol not supported, use 4")
		return
	}
	if addressFamily != 4 {
		sess.writeMessage(522, "Network protocol not supported, use 4")
		return
	}

	addressLength, err := strconv.Atoi(parts[1])
	if err != nil {
		sess.writeMessage(522, "Network protocol not supported, use 4")
		return
	}
	if addressLength != 4 {
		sess.writeMessage(522, "Network IP length not supported, use 4")
		return
	}

	host := strings.Join(parts[2:2+addressLength], ".")

	portLength, err := strconv.Atoi(parts[2+addressLength])
	if err != nil {
		sess.writeMessage(522, "Network protocol not supported, use 4")
		return
	}
	portAddress := parts[3+addressLength : 3+addressLength+portLength]

	// Convert string[] to byte[]
	portBytes := make([]byte, portLength)
	for i := range portAddress {
		p, _ := strconv.Atoi(portAddress[i])
		portBytes[i] = byte(p)
	}

	// convert the bytes to an int
	port := int(binary.BigEndian.Uint16(portBytes))

	// if the existing connection is on the same host/port don't reconnect
	if sess.dataConn.Host() == host && sess.dataConn.Port() == port {
		return
	}

	socket, err := newActiveSocket(sess, host, port)
	if err != nil {
		sess.writeMessage(425, "Data connection failed")
		return
	}
	sess.dataConn = socket
	sess.writeMessage(200, "Connection established ("+strconv.Itoa(port)+")")
}

// commandEpsv responds to the EPSV FTP command. It allows the client to
// request a passive data socket with more options than the original PASV
// command. It mainly adds ipv6 support, although we don't support that yet.
type commandEpsv struct{}

func (cmd commandEpsv) IsExtend() bool {
	return true
}

func (cmd commandEpsv) RequireParam() bool {
	return false
}

func (cmd commandEpsv) RequireAuth() bool {
	return true
}

func (cmd commandEpsv) Execute(sess *Session, param string) {
	socket, err := sess.newPassiveSocket()
	if err != nil {
		sess.log(err)
		sess.writeMessage(425, "Data connection failed")
		return
	}

	msg := fmt.Sprintf("Entering Extended Passive Mode (|||%d|)", socket.Port())
	sess.writeMessage(229, msg)
}

// commandList responds to the LIST FTP command. It allows the client to retrieve
// a detailed listing of the contents of a directory.
type commandList struct{}

func (cmd commandList) IsExtend() bool {
	return false
}

func (cmd commandList) RequireParam() bool {
	return false
}

func (cmd commandList) RequireAuth() bool {
	return true
}

func convertFileInfo(sess *Session, f os.FileInfo, p string) (FileInfo, error) {
	mode, err := sess.server.Perm.GetMode(p)
	if err != nil {
		return nil, err
	}
	if f.IsDir() {
		mode |= os.ModeDir
	}
	owner, err := sess.server.Perm.GetOwner(p)
	if err != nil {
		return nil, err
	}
	group, err := sess.server.Perm.GetGroup(p)
	if err != nil {
		return nil, err
	}
	return &fileInfo{
		FileInfo: f,
		mode:     mode,
		owner:    owner,
		group:    group,
	}, nil
}

func list(sess *Session, cmd, p, param string) ([]FileInfo, error) {
	var ctx = &Context{
		Sess:  sess,
		Cmd:   cmd,
		Param: param,
		Data:  make(map[string]interface{}),
	}
	info, err := sess.server.Driver.Stat(ctx, p)
	if err != nil {
		return nil, err
	}

	if info == nil {
		sess.logf("%s: no such file or directory.\n", p)
		return []FileInfo{}, nil
	}

	var files []FileInfo
	if info.IsDir() {
		err = sess.server.Driver.ListDir(ctx, p, func(f os.FileInfo) error {
			info, err := convertFileInfo(sess, f, path.Join(p, f.Name()))
			if err != nil {
				return err
			}
			files = append(files, info)
			return nil
		})
		if err != nil {
			return nil, err
		}
	} else {
		newInfo, err := convertFileInfo(sess, info, p)
		if err != nil {
			return nil, err
		}
		files = append(files, newInfo)
	}
	return files, nil
}

func (cmd commandList) Execute(sess *Session, param string) {
	p := sess.buildPath(parseListParam(param))

	files, err := list(sess, "LIST", p, param)
	if err != nil {
		sess.writeMessage(550, err.Error())
		return
	}

	sess.writeMessage(150, "Opening ASCII mode data connection for file list")
	sess.sendOutofbandData(listFormatter(files).Detailed())
}

func parseListParam(param string) (path string) {
	if len(param) == 0 {
		path = param
	} else {
		fields := strings.Fields(param)
		i := 0
		for _, field := range fields {
			if !strings.HasPrefix(field, "-") {
				break
			}
			i = strings.LastIndex(param, " "+field) + len(field) + 1
		}
		path = strings.TrimLeft(param[i:], " ") //Get all the path even with space inside
	}
	return path
}

// commandNlst responds to the NLST FTP command. It allows the client to
// retrieve a list of filenames in the current directory.
type commandNlst struct{}

func (cmd commandNlst) IsExtend() bool {
	return false
}

func (cmd commandNlst) RequireParam() bool {
	return false
}

func (cmd commandNlst) RequireAuth() bool {
	return true
}

func (cmd commandNlst) Execute(sess *Session, param string) {
	var ctx = &Context{
		Sess:  sess,
		Cmd:   "NLST",
		Param: param,
		Data:  make(map[string]interface{}),
	}
	path := sess.buildPath(parseListParam(param))
	info, err := sess.server.Driver.Stat(ctx, path)
	if err != nil {
		sess.writeMessage(550, err.Error())
		return
	}
	if !info.IsDir() {
		sess.writeMessage(550, param+" is not a directory")
		return
	}

	var files []FileInfo
	err = sess.server.Driver.ListDir(ctx, path, func(f os.FileInfo) error {
		mode, err := sess.server.Perm.GetMode(path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			mode |= os.ModeDir
		}
		owner, err := sess.server.Perm.GetOwner(path)
		if err != nil {
			return err
		}
		group, err := sess.server.Perm.GetGroup(path)
		if err != nil {
			return err
		}
		files = append(files, &fileInfo{
			FileInfo: f,
			mode:     mode,
			owner:    owner,
			group:    group,
		})
		return nil
	})
	if err != nil {
		sess.writeMessage(550, err.Error())
		return
	}
	sess.writeMessage(150, "Opening ASCII mode data connection for file list")
	sess.sendOutofbandData(listFormatter(files).Short())
}

// commandMdtm responds to the MDTM FTP command. It allows the client to
// retreive the last modified time of a file.
type commandMdtm struct{}

func (cmd commandMdtm) IsExtend() bool {
	return false
}

func (cmd commandMdtm) RequireParam() bool {
	return true
}

func (cmd commandMdtm) RequireAuth() bool {
	return true
}

func (cmd commandMdtm) Execute(sess *Session, param string) {
	path := sess.buildPath(param)
	stat, err := sess.server.Driver.Stat(&Context{
		Sess:  sess,
		Cmd:   "MDTM",
		Param: param,
		Data:  make(map[string]interface{}),
	}, path)
	if err == nil {
		sess.writeMessage(213, stat.ModTime().Format("20060102150405"))
	} else {
		sess.writeMessage(450, "File not available")
	}
}

// commandMkd responds to the MKD FTP command. It allows the client to create
// a new directory
type commandMkd struct{}

func (cmd commandMkd) IsExtend() bool {
	return false
}

func (cmd commandMkd) RequireParam() bool {
	return true
}

func (cmd commandMkd) RequireAuth() bool {
	return true
}

func (cmd commandMkd) Execute(sess *Session, param string) {
	path := sess.buildPath(param)
	var ctx = Context{
		Sess:  sess,
		Cmd:   "MKD",
		Param: param,
		Data:  make(map[string]interface{}),
	}
	sess.server.notifiers.BeforeCreateDir(&ctx, path)
	err := sess.server.Driver.MakeDir(&ctx, path)
	sess.server.notifiers.AfterDirCreated(&ctx, path, err)
	if err == nil {
		sess.writeMessage(257, "Directory created")
	} else {
		sess.writeMessage(550, fmt.Sprint("Action not taken: ", err))
	}
}

// cmdMode responds to the MODE FTP command.
//
// the original FTP spec had various options for hosts to negotiate how data
// would be sent over the data socket, In reality these days (S)tream mode
// is all that is used for the mode - data is just streamed down the data
// socket unchanged.
type commandMode struct{}

func (cmd commandMode) IsExtend() bool {
	return false
}

func (cmd commandMode) RequireParam() bool {
	return true
}

func (cmd commandMode) RequireAuth() bool {
	return true
}

func (cmd commandMode) Execute(sess *Session, param string) {
	if strings.ToUpper(param) == "S" {
		sess.writeMessage(200, "OK")
	} else {
		sess.writeMessage(504, "MODE is an obsolete command")
	}
}

// cmdNoop responds to the NOOP FTP command.
//
// This is essentially a ping from the client so we just respond with an
// basic 200 message.
type commandNoop struct{}

func (cmd commandNoop) IsExtend() bool {
	return false
}

func (cmd commandNoop) RequireParam() bool {
	return false
}

func (cmd commandNoop) RequireAuth() bool {
	return false
}

func (cmd commandNoop) Execute(sess *Session, param string) {
	sess.writeMessage(200, "OK")
}

// commandPass respond to the PASS FTP command by asking the driver if the
// supplied username and password are valid
type commandPass struct{}

func (cmd commandPass) IsExtend() bool {
	return false
}

func (cmd commandPass) RequireParam() bool {
	return true
}

func (cmd commandPass) RequireAuth() bool {
	return false
}

func (cmd commandPass) Execute(sess *Session, param string) {
	auth := sess.server.Auth
	// If Driver implements Auth then call that instead of the Server version
	if driverAuth, found := sess.server.Driver.(Auth); found {
		auth = driverAuth
	}
	var ctx = Context{
		Sess:  sess,
		Cmd:   "PASS",
		Param: param,
		Data:  make(map[string]interface{}),
	}
	ok, err := auth.CheckPasswd(&ctx, sess.reqUser, param)
	sess.server.notifiers.AfterUserLogin(&ctx, sess.reqUser, param, ok, err)
	if err != nil {
		sess.writeMessage(550, "Checking password error")
		return
	}

	if ok {
		sess.user = sess.reqUser
		sess.reqUser = ""
		sess.writeMessage(230, "Password ok, continue")
	} else {
		sess.writeMessage(530, "Incorrect password, not logged in")
	}
}

// commandPasv responds to the PASV FTP command.
//
// The client is requesting us to open a new TCP listing socket and wait for them
// to connect to it.
type commandPasv struct{}

func (cmd commandPasv) IsExtend() bool {
	return false
}

func (cmd commandPasv) RequireParam() bool {
	return false
}

func (cmd commandPasv) RequireAuth() bool {
	return true
}

func (cmd commandPasv) Execute(sess *Session, param string) {
	listenIP := sess.passiveListenIP()
	// TODO: IPv6 for this command is not implemented
	if strings.HasPrefix(listenIP, "::") {
		sess.writeMessage(550, "Action not taken")
		return
	}

	socket, err := sess.newPassiveSocket()
	if err != nil {
		sess.writeMessage(425, "Data connection failed")
		return
	}

	p1 := socket.Port() / 256
	p2 := socket.Port() - (p1 * 256)

	quads := strings.Split(listenIP, ".")
	target := fmt.Sprintf("(%s,%s,%s,%s,%d,%d)", quads[0], quads[1], quads[2], quads[3], p1, p2)
	msg := "Entering Passive Mode " + target
	sess.writeMessage(227, msg)
}

// commandPort responds to the PORT FTP command.
//
// The client has opened a listening socket for sending out of band data and
// is requesting that we connect to it
type commandPort struct{}

func (cmd commandPort) IsExtend() bool {
	return false
}

func (cmd commandPort) RequireParam() bool {
	return true
}

func (cmd commandPort) RequireAuth() bool {
	return true
}

func (cmd commandPort) Execute(sess *Session, param string) {
	nums := strings.Split(param, ",")
	portOne, _ := strconv.Atoi(nums[4])
	portTwo, _ := strconv.Atoi(nums[5])
	port := (portOne * 256) + portTwo
	host := nums[0] + "." + nums[1] + "." + nums[2] + "." + nums[3]
	socket, err := newActiveSocket(sess, host, port)
	if err != nil {
		sess.writeMessage(425, "Data connection failed")
		return
	}
	sess.dataConn = socket
	sess.writeMessage(200, "Connection established ("+strconv.Itoa(port)+")")
}

// commandPwd responds to the PWD FTP command.
//
// Tells the client what the current working directory is.
type commandPwd struct{}

func (cmd commandPwd) IsExtend() bool {
	return false
}

func (cmd commandPwd) RequireParam() bool {
	return false
}

func (cmd commandPwd) RequireAuth() bool {
	return true
}

func (cmd commandPwd) Execute(sess *Session, param string) {
	sess.writeMessage(257, "\""+sess.curDir+"\" is the current directory")
}

// CommandQuit responds to the QUIT FTP command. The client has requested the
// connection be closed.
type commandQuit struct{}

func (cmd commandQuit) IsExtend() bool {
	return false
}

func (cmd commandQuit) RequireParam() bool {
	return false
}

func (cmd commandQuit) RequireAuth() bool {
	return false
}

func (cmd commandQuit) Execute(sess *Session, param string) {
	sess.writeMessage(221, "Goodbye")
	sess.Close()
}

// commandRetr responds to the RETR FTP command. It allows the client to
// download a file.
// REST can be followed by APPE, STOR, or RETR
type commandRetr struct{}

func (cmd commandRetr) IsExtend() bool {
	return false
}

func (cmd commandRetr) RequireParam() bool {
	return true
}

func (cmd commandRetr) RequireAuth() bool {
	return true
}

func (cmd commandRetr) Execute(sess *Session, param string) {
	path := sess.buildPath(param)
	if sess.preCommand != "REST" {
		sess.lastFilePos = -1
	}
	defer func() {
		sess.lastFilePos = -1
	}()
	var ctx = Context{
		Sess:  sess,
		Cmd:   "RETR",
		Param: param,
		Data:  make(map[string]interface{}),
	}
	sess.server.notifiers.BeforeDownloadFile(&ctx, path)
	var readPos = sess.lastFilePos
	if readPos < 0 {
		readPos = 0
	}
	size, data, err := sess.server.Driver.GetFile(&ctx, path, readPos)
	if err == nil {
		defer data.Close()
		sess.writeMessage(150, fmt.Sprintf("Data transfer starting %d bytes", size))
		err = sess.sendOutofBandDataWriter(data)
		sess.server.notifiers.AfterFileDownloaded(&ctx, path, size, err)
		if err != nil {
			sess.writeMessage(551, "Error reading file")
		}
	} else {
		sess.server.notifiers.AfterFileDownloaded(&ctx, path, size, err)
		sess.writeMessage(551, "File not available")
	}
}

type commandRest struct{}

func (cmd commandRest) IsExtend() bool {
	return false
}

func (cmd commandRest) RequireParam() bool {
	return true
}

func (cmd commandRest) RequireAuth() bool {
	return true
}

func (cmd commandRest) Execute(sess *Session, param string) {
	var err error
	sess.lastFilePos, err = strconv.ParseInt(param, 10, 64)
	if err != nil {
		sess.writeMessage(551, "File not available")
		return
	}

	sess.writeMessage(350, fmt.Sprint("Start transfer from ", sess.lastFilePos))
}

// commandRnfr responds to the RNFR FTP command. It's the first of two commands
// required for a client to rename a file.
type commandRnfr struct{}

func (cmd commandRnfr) IsExtend() bool {
	return false
}

func (cmd commandRnfr) RequireParam() bool {
	return true
}

func (cmd commandRnfr) RequireAuth() bool {
	return true
}

func (cmd commandRnfr) Execute(sess *Session, param string) {
	sess.renameFrom = ""
	p := sess.buildPath(param)
	if _, err := sess.server.Driver.Stat(&Context{
		Sess:  sess,
		Cmd:   "RNFR",
		Param: param,
		Data:  make(map[string]interface{}),
	}, p); err != nil {
		sess.writeMessage(550, fmt.Sprint("Action not taken: ", err))
		return
	}
	sess.renameFrom = p
	sess.writeMessage(350, "Requested file action pending further information.")
}

// cmdRnto responds to the RNTO FTP command. It's the second of two commands
// required for a client to rename a file.
type commandRnto struct{}

func (cmd commandRnto) IsExtend() bool {
	return false
}

func (cmd commandRnto) RequireParam() bool {
	return true
}

func (cmd commandRnto) RequireAuth() bool {
	return true
}

func (cmd commandRnto) Execute(sess *Session, param string) {
	toPath := sess.buildPath(param)
	err := sess.server.Driver.Rename(&Context{
		Sess:  sess,
		Cmd:   "RNTO",
		Param: param,
		Data:  make(map[string]interface{}),
	}, sess.renameFrom, toPath)
	defer func() {
		sess.renameFrom = ""
	}()

	if err == nil {
		sess.writeMessage(250, "File renamed")
	} else {
		sess.writeMessage(550, fmt.Sprint("Action not taken: ", err))
	}
}

// cmdRmd responds to the RMD FTP command. It allows the client to delete a
// directory.
type commandRmd struct{}

func (cmd commandRmd) IsExtend() bool {
	return false
}

func (cmd commandRmd) RequireParam() bool {
	return true
}

func (cmd commandRmd) RequireAuth() bool {
	return true
}

func (cmd commandRmd) Execute(sess *Session, param string) {
	executeRmd("RMD", sess, param)
}

// cmdXRmd responds to the RMD FTP command. It allows the client to delete a
// directory.
type commandXRmd struct{}

func (cmd commandXRmd) IsExtend() bool {
	return false
}

func (cmd commandXRmd) RequireParam() bool {
	return true
}

func (cmd commandXRmd) RequireAuth() bool {
	return true
}

func (cmd commandXRmd) Execute(sess *Session, param string) {
	executeRmd("XRMD", sess, param)
}

func executeRmd(cmd string, sess *Session, param string) {
	p := sess.buildPath(param)
	var ctx = Context{
		Sess:  sess,
		Cmd:   cmd,
		Param: param,
		Data:  make(map[string]interface{}),
	}
	if param == "/" || param == "" {
		sess.writeMessage(550, "Directory / cannot be deleted")
		return
	}

	var needChangeCurDir = strings.HasPrefix(param, sess.curDir)

	sess.server.notifiers.BeforeDeleteDir(&ctx, p)
	err := sess.server.Driver.DeleteDir(&ctx, p)
	if needChangeCurDir {
		sess.curDir = path.Dir(param)
	}
	sess.server.notifiers.AfterDirDeleted(&ctx, p, err)
	if err == nil {
		sess.writeMessage(250, "Directory deleted")
	} else {
		sess.writeMessage(550, fmt.Sprint("Directory delete failed: ", err))
	}
}

type commandAdat struct{}

func (cmd commandAdat) IsExtend() bool {
	return false
}

func (cmd commandAdat) RequireParam() bool {
	return true
}

func (cmd commandAdat) RequireAuth() bool {
	return true
}

func (cmd commandAdat) Execute(sess *Session, param string) {
	sess.writeMessage(550, "Action not taken")
}

type commandAuth struct{}

func (cmd commandAuth) IsExtend() bool {
	return false
}

func (cmd commandAuth) RequireParam() bool {
	return true
}

func (cmd commandAuth) RequireAuth() bool {
	return false
}

func (cmd commandAuth) Execute(sess *Session, param string) {
	if param == "TLS" && sess.server.tlsConfig != nil {
		sess.writeMessage(234, "AUTH command OK")
		err := sess.upgradeToTLS()
		if err != nil {
			sess.logf("Error upgrading connection to TLS %v", err.Error())
		}
	} else {
		sess.writeMessage(550, "Action not taken")
	}
}

type commandCcc struct{}

func (cmd commandCcc) IsExtend() bool {
	return false
}

func (cmd commandCcc) RequireParam() bool {
	return true
}

func (cmd commandCcc) RequireAuth() bool {
	return true
}

func (cmd commandCcc) Execute(sess *Session, param string) {
	sess.writeMessage(550, "Action not taken")
}

type commandEnc struct{}

func (cmd commandEnc) IsExtend() bool {
	return false
}

func (cmd commandEnc) RequireParam() bool {
	return true
}

func (cmd commandEnc) RequireAuth() bool {
	return true
}

func (cmd commandEnc) Execute(sess *Session, param string) {
	sess.writeMessage(550, "Action not taken")
}

type commandMic struct{}

func (cmd commandMic) IsExtend() bool {
	return false
}

func (cmd commandMic) RequireParam() bool {
	return true
}

func (cmd commandMic) RequireAuth() bool {
	return true
}

func (cmd commandMic) Execute(sess *Session, param string) {
	sess.writeMessage(550, "Action not taken")
}

type commandMLSD struct{}

func (cmd commandMLSD) IsExtend() bool {
	return true
}

func (cmd commandMLSD) RequireParam() bool {
	return false
}

func (cmd commandMLSD) RequireAuth() bool {
	return true
}

func toMLSDFormat(files []FileInfo) []byte {
	var buf bytes.Buffer
	for _, file := range files {
		var fileType = "file"
		if file.IsDir() {
			fileType = "dir"
		}
		/*Possible facts "Size" / "Modify" / "Create" /
				  "Type" / "Unique" / "Perm" /
				  "Lang" / "Media-Type" / "CharSet"
				  TODO: Perm pvals        = "a" / "c" / "d" / "e" / "f" /
		                     "l" / "m" / "p" / "r" / "w"
		*/
		fmt.Fprintf(&buf,
			"Type=%s;Modify=%s;Size=%d; %s\n",
			fileType,
			file.ModTime().Format("20060102150405"),
			file.Size(),
			file.Name(),
		)
	}
	return buf.Bytes()
}

func (cmd commandMLSD) Execute(sess *Session, param string) {
	if param == "" {
		param = sess.curDir
	}
	p := sess.buildPath(param)

	files, err := list(sess, "MLSD", p, param)
	if err != nil {
		sess.writeMessage(550, err.Error())
		return
	}

	sess.writeMessage(150, "Opening ASCII mode data connection for file list")
	sess.sendOutofbandData(toMLSDFormat(files))
}

type commandPbsz struct{}

func (cmd commandPbsz) IsExtend() bool {
	return false
}

func (cmd commandPbsz) RequireParam() bool {
	return true
}

func (cmd commandPbsz) RequireAuth() bool {
	return false
}

func (cmd commandPbsz) Execute(sess *Session, param string) {
	if sess.tls && param == "0" {
		sess.writeMessage(200, "OK")
	} else {
		sess.writeMessage(550, "Action not taken")
	}
}

type commandProt struct{}

func (cmd commandProt) IsExtend() bool {
	return false
}

func (cmd commandProt) RequireParam() bool {
	return true
}

func (cmd commandProt) RequireAuth() bool {
	return false
}

func (cmd commandProt) Execute(sess *Session, param string) {
	if sess.tls && param == "P" {
		sess.writeMessage(200, "OK")
	} else if sess.tls {
		sess.writeMessage(536, "Only P level is supported")
	} else {
		sess.writeMessage(550, "Action not taken")
	}
}

type commandConf struct{}

func (cmd commandConf) IsExtend() bool {
	return false
}

func (cmd commandConf) RequireParam() bool {
	return true
}

func (cmd commandConf) RequireAuth() bool {
	return true
}

func (cmd commandConf) Execute(sess *Session, param string) {
	sess.writeMessage(550, "Action not taken")
}

// commandSize responds to the SIZE FTP command. It returns the size of the
// requested path in bytes.
type commandSize struct{}

func (cmd commandSize) IsExtend() bool {
	return false
}

func (cmd commandSize) RequireParam() bool {
	return true
}

func (cmd commandSize) RequireAuth() bool {
	return true
}

func (cmd commandSize) Execute(sess *Session, param string) {
	path := sess.buildPath(param)
	stat, err := sess.server.Driver.Stat(&Context{
		Sess:  sess,
		Cmd:   "SIZE",
		Param: param,
		Data:  make(map[string]interface{}),
	}, path)
	if err != nil {
		log.Printf("Size: error(%s)", err)
		sess.writeMessage(450, fmt.Sprintf("path %s not found", param))
	} else {
		sess.writeMessage(213, strconv.Itoa(int(stat.Size())))
	}
}

// commandStat responds to the STAT FTP command. It returns the stat of the
// requested path.
type commandStat struct{}

func (cmd commandStat) IsExtend() bool {
	return false
}

func (cmd commandStat) RequireParam() bool {
	return false
}

func (cmd commandStat) RequireAuth() bool {
	return true
}

func (cmd commandStat) Execute(sess *Session, param string) {
	// system stat
	if param == "" {
		sess.writeMessage(211, fmt.Sprintf("%s FTP server status:\nVersion %s"+
			"Connected to %s (%s)\n"+
			"Logged in %s\n"+
			"TYPE: ASCII, FORM: Nonprint; STRUcture: File; transfer MODE: Stream\n"+
			"No data connection", sess.PublicIP(), version, sess.PublicIP(),
			version, sess.LoginUser()))
		sess.writeMessage(211, "End of status")
		return
	}

	var ctx = Context{
		Sess:  sess,
		Cmd:   "STAT",
		Param: param,
		Data:  make(map[string]interface{}),
	}

	// file or directory stat
	path := sess.buildPath(param)
	stat, err := sess.server.Driver.Stat(&ctx, path)
	if err != nil {
		log.Printf("Size: error(%s)", err)
		sess.writeMessage(450, fmt.Sprintf("path %s not found", path))
	} else {
		var files []FileInfo
		if stat.IsDir() {
			err = sess.server.Driver.ListDir(&ctx, path, func(f os.FileInfo) error {
				info, err := convertFileInfo(sess, f, filepath.Join(path, f.Name()))
				if err != nil {
					return err
				}
				files = append(files, info)
				return nil
			})
			if err != nil {
				sess.writeMessage(550, err.Error())
				return
			}
			sess.writeMessage(213, "Opening ASCII mode data connection for file list")
		} else {
			info, err := convertFileInfo(sess, stat, path)
			if err != nil {
				sess.writeMessage(550, err.Error())
				return
			}
			files = append(files, info)
			sess.writeMessage(212, "Opening ASCII mode data connection for file list")
		}
		sess.sendOutofbandData(listFormatter(files).Detailed())
	}
}

// commandStor responds to the STOR FTP command. It allows the user to upload a
// new file.
type commandStor struct{}

func (cmd commandStor) IsExtend() bool {
	return false
}

func (cmd commandStor) RequireParam() bool {
	return true
}

func (cmd commandStor) RequireAuth() bool {
	return true
}

func (cmd commandStor) Execute(sess *Session, param string) {
	targetPath := sess.buildPath(param)
	sess.writeMessage(150, "Data transfer starting")

	if sess.preCommand != "REST" {
		sess.lastFilePos = -1
	}

	defer func() {
		sess.lastFilePos = -1
	}()

	var ctx = Context{
		Sess:  sess,
		Cmd:   "STOR",
		Param: param,
		Data:  make(map[string]interface{}),
	}
	sess.server.notifiers.BeforePutFile(&ctx, targetPath)
	size, err := sess.server.Driver.PutFile(&ctx, targetPath, sess.dataConn, sess.lastFilePos)
	sess.server.notifiers.AfterFilePut(&ctx, targetPath, size, err)
	if err == nil {
		msg := fmt.Sprintf("OK, received %d bytes", size)
		sess.writeMessage(226, msg)
	} else {
		sess.writeMessage(450, fmt.Sprint("error during transfer: ", err))
	}
}

// commandStru responds to the STRU FTP command.
//
// like the MODE and TYPE commands, stru[cture] dates back to a time when the
// FTP protocol was more aware of the content of the files it was transferring,
// and would sometimes be expected to translate things like EOL markers on the
// fly.
//
// These days files are sent unmodified, and F(ile) mode is the only one we
// really need to support.
type commandStru struct{}

func (cmd commandStru) IsExtend() bool {
	return false
}

func (cmd commandStru) RequireParam() bool {
	return true
}

func (cmd commandStru) RequireAuth() bool {
	return true
}

func (cmd commandStru) Execute(sess *Session, param string) {
	if strings.ToUpper(param) == "F" {
		sess.writeMessage(200, "OK")
	} else {
		sess.writeMessage(504, "STRU is an obsolete command")
	}
}

// commandSyst responds to the SYST FTP command by providing a canned response.
type commandSyst struct{}

func (cmd commandSyst) IsExtend() bool {
	return false
}

func (cmd commandSyst) RequireParam() bool {
	return false
}

func (cmd commandSyst) RequireAuth() bool {
	return true
}

func (cmd commandSyst) Execute(sess *Session, param string) {
	sess.writeMessage(215, "UNIX Type: L8")
}

// commandType responds to the TYPE FTP command.
//
//  like the MODE and STRU commands, TYPE dates back to a time when the FTP
//  protocol was more aware of the content of the files it was transferring, and
//  would sometimes be expected to translate things like EOL markers on the fly.
//
//  Valid options were A(SCII), I(mage), E(BCDIC) or LN (for local type). Since
//  we plan to just accept bytes from the client unchanged, I think Image mode is
//  adequate. The RFC requires we accept ASCII mode however, so accept it, but
//  ignore it.
type commandType struct{}

func (cmd commandType) IsExtend() bool {
	return false
}

func (cmd commandType) RequireParam() bool {
	return false
}

func (cmd commandType) RequireAuth() bool {
	return true
}

func (cmd commandType) Execute(sess *Session, param string) {
	if strings.ToUpper(param) == "A" {
		sess.writeMessage(200, "Type set to ASCII")
	} else if strings.ToUpper(param) == "I" {
		sess.writeMessage(200, "Type set to binary")
	} else {
		sess.writeMessage(500, "Invalid type")
	}
}

// commandUser responds to the USER FTP command by asking for the password
type commandUser struct{}

func (cmd commandUser) IsExtend() bool {
	return false
}

func (cmd commandUser) RequireParam() bool {
	return true
}

func (cmd commandUser) RequireAuth() bool {
	return false
}

func (cmd commandUser) Execute(sess *Session, param string) {
	sess.reqUser = param
	sess.server.notifiers.BeforeLoginUser(&Context{
		Sess:  sess,
		Cmd:   "USER",
		Param: param,
		Data:  make(map[string]interface{}),
	}, sess.reqUser)
	sess.writeMessage(331, "User name ok, password required")
}
