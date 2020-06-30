// Copyright 2018 The goftp Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package server

import (
	"encoding/binary"
	"fmt"
	"log"
	"strconv"
	"strings"
)

// Command represents a Command interface to a ftp command
type Command interface {
	IsExtend() bool
	RequireParam() bool
	RequireAuth() bool
	Execute(*Conn, string)
}

type commandMap map[string]Command

var (
	commands = commandMap{
		"ADAT": commandAdat{},
		"ALLO": commandAllo{},
		"APPE": commandAppe{},
		"AUTH": commandAuth{},
		"CDUP": commandCdup{},
		"CWD":  commandCwd{},
		"CCC":  commandCcc{},
		"CONF": commandConf{},
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
		"STOR": commandStor{},
		"STRU": commandStru{},
		"SYST": commandSyst{},
		"TYPE": commandType{},
		"USER": commandUser{},
		"XCUP": commandCdup{},
		"XCWD": commandCwd{},
		"XMKD": commandMkd{},
		"XPWD": commandPwd{},
		"XRMD": commandRmd{},
	}
)

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

func (cmd commandAllo) Execute(conn *Conn, param string) {
	conn.writeMessage(202, "Obsolete")
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

func (cmd commandAppe) Execute(conn *Conn, param string) {
	targetPath := conn.buildPath(param)
	conn.writeMessage(150, "Data transfer starting")

	conn.server.notifiers.BeforePutFile(conn, targetPath)
	size, err := conn.driver.PutFile(targetPath, conn.dataConn, true)
	conn.server.notifiers.AfterFilePut(conn, targetPath, size, err)
	if err == nil {
		msg := fmt.Sprintf("OK, received %d bytes", size)
		conn.writeMessage(226, msg)
	} else {
		conn.writeMessage(450, fmt.Sprint("error during transfer: ", err))
	}

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

func (cmd commandOpts) Execute(conn *Conn, param string) {
	parts := strings.Fields(param)
	if len(parts) != 2 {
		conn.writeMessage(550, "Unknow params")
		return
	}
	if strings.ToUpper(parts[0]) != "UTF8" {
		conn.writeMessage(550, "Unknow params")
		return
	}

	if strings.ToUpper(parts[1]) == "ON" {
		conn.writeMessage(200, "UTF8 mode enabled")
	} else {
		conn.writeMessage(550, "Unsupported non-utf8 mode")
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

var (
	feats    = "Extensions supported:\n%s"
	featCmds = " UTF8\n"
)

func init() {
	for k, v := range commands {
		if v.IsExtend() {
			featCmds = featCmds + " " + k + "\n"
		}
	}
}

func (cmd commandFeat) Execute(conn *Conn, param string) {
	conn.writeMessageMultiline(211, conn.server.feats)
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

func (cmd commandCdup) Execute(conn *Conn, param string) {
	otherCmd := &commandCwd{}
	otherCmd.Execute(conn, "..")
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

func (cmd commandCwd) Execute(conn *Conn, param string) {
	path := conn.buildPath(param)
	info, err := conn.driver.Stat(path)
	if err != nil {
		conn.writeMessage(550, fmt.Sprint("Directory change to ", path, " failed: ", err))
		return
	}
	if !info.IsDir() {
		conn.writeMessage(550, fmt.Sprint("Directory change to ", path, " is a file"))
		return
	}

	conn.server.notifiers.BeforeChangeCurDir(conn, conn.curDir, path)
	err = conn.changeCurDir(path)
	conn.server.notifiers.AfterCurDirChanged(conn, conn.curDir, path, err)
	if err == nil {
		conn.writeMessage(250, "Directory changed to "+path)
	} else {
		conn.writeMessage(550, fmt.Sprint("Directory change to ", path, " failed: ", err))
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

func (cmd commandDele) Execute(conn *Conn, param string) {
	path := conn.buildPath(param)
	conn.server.notifiers.BeforeDeleteFile(conn, path)
	err := conn.driver.DeleteFile(path)
	conn.server.notifiers.AfterFileDeleted(conn, path, err)
	if err == nil {
		conn.writeMessage(250, "File deleted")
	} else {
		conn.writeMessage(550, fmt.Sprint("File delete failed: ", err))
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

func (cmd commandEprt) Execute(conn *Conn, param string) {
	delim := string(param[0:1])
	parts := strings.Split(param, delim)
	addressFamily, err := strconv.Atoi(parts[1])
	host := parts[2]
	port, err := strconv.Atoi(parts[3])
	if addressFamily != 1 && addressFamily != 2 {
		conn.writeMessage(522, "Network protocol not supported, use (1,2)")
		return
	}
	socket, err := newActiveSocket(host, port, conn.logger, conn.sessionID)
	if err != nil {
		conn.writeMessage(425, "Data connection failed")
		return
	}
	conn.dataConn = socket
	conn.writeMessage(200, "Connection established ("+strconv.Itoa(port)+")")
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

func (cmd commandLprt) Execute(conn *Conn, param string) {
	// No tests for this code yet

	parts := strings.Split(param, ",")

	addressFamily, err := strconv.Atoi(parts[0])
	if addressFamily != 4 {
		conn.writeMessage(522, "Network protocol not supported, use 4")
		return
	}

	addressLength, err := strconv.Atoi(parts[1])
	if addressLength != 4 {
		conn.writeMessage(522, "Network IP length not supported, use 4")
		return
	}

	host := strings.Join(parts[2:2+addressLength], ".")

	portLength, err := strconv.Atoi(parts[2+addressLength])
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
	if conn.dataConn.Host() == host && conn.dataConn.Port() == port {
		return
	}

	socket, err := newActiveSocket(host, port, conn.logger, conn.sessionID)
	if err != nil {
		conn.writeMessage(425, "Data connection failed")
		return
	}
	conn.dataConn = socket
	conn.writeMessage(200, "Connection established ("+strconv.Itoa(port)+")")
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

func (cmd commandEpsv) Execute(conn *Conn, param string) {
	socket, err := conn.newPassiveSocket()
	if err != nil {
		log.Println(err)
		conn.writeMessage(425, "Data connection failed")
		return
	}

	msg := fmt.Sprintf("Entering Extended Passive Mode (|||%d|)", socket.Port())
	conn.writeMessage(229, msg)
}

// commandList responds to the LIST FTP command. It allows the client to retreive
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

func (cmd commandList) Execute(conn *Conn, param string) {
	path := conn.buildPath(parseListParam(param))
	info, err := conn.driver.Stat(path)
	if err != nil {
		conn.writeMessage(550, err.Error())
		return
	}

	if info == nil {
		conn.logger.Printf(conn.sessionID, "%s: no such file or directory.\n", path)
		return
	}

	var files []FileInfo
	if info.IsDir() {
		err = conn.driver.ListDir(path, func(f FileInfo) error {
			files = append(files, f)
			return nil
		})
		if err != nil {
			conn.writeMessage(550, err.Error())
			return
		}
	} else {
		files = append(files, info)
	}

	conn.writeMessage(150, "Opening ASCII mode data connection for file list")
	conn.sendOutofbandData(listFormatter(files).Detailed())
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
// retreive a list of filenames in the current directory.
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

func (cmd commandNlst) Execute(conn *Conn, param string) {
	path := conn.buildPath(parseListParam(param))
	info, err := conn.driver.Stat(path)
	if err != nil {
		conn.writeMessage(550, err.Error())
		return
	}
	if !info.IsDir() {
		conn.writeMessage(550, param+" is not a directory")
		return
	}

	var files []FileInfo
	err = conn.driver.ListDir(path, func(f FileInfo) error {
		files = append(files, f)
		return nil
	})
	if err != nil {
		conn.writeMessage(550, err.Error())
		return
	}
	conn.writeMessage(150, "Opening ASCII mode data connection for file list")
	conn.sendOutofbandData(listFormatter(files).Short())
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

func (cmd commandMdtm) Execute(conn *Conn, param string) {
	path := conn.buildPath(param)
	stat, err := conn.driver.Stat(path)
	if err == nil {
		conn.writeMessage(213, stat.ModTime().Format("20060102150405"))
	} else {
		conn.writeMessage(450, "File not available")
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

func (cmd commandMkd) Execute(conn *Conn, param string) {
	path := conn.buildPath(param)
	conn.server.notifiers.BeforeCreateDir(conn, path)
	err := conn.driver.MakeDir(path)
	conn.server.notifiers.AfterDirCreated(conn, path, err)
	if err == nil {
		conn.writeMessage(257, "Directory created")
	} else {
		conn.writeMessage(550, fmt.Sprint("Action not taken: ", err))
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

func (cmd commandMode) Execute(conn *Conn, param string) {
	if strings.ToUpper(param) == "S" {
		conn.writeMessage(200, "OK")
	} else {
		conn.writeMessage(504, "MODE is an obsolete command")
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

func (cmd commandNoop) Execute(conn *Conn, param string) {
	conn.writeMessage(200, "OK")
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

func (cmd commandPass) Execute(conn *Conn, param string) {
	auth := conn.server.Auth
	// If Driver implements Auth then call that instead of the Server version
	if driverAuth, found := conn.driver.(Auth); found {
		auth = driverAuth
	}
	ok, err := auth.CheckPasswd(conn.reqUser, param)
	conn.server.notifiers.AfterUserLogin(conn, conn.reqUser, param, ok, err)
	if err != nil {
		conn.writeMessage(550, "Checking password error")
		return
	}

	if ok {
		conn.user = conn.reqUser
		conn.reqUser = ""
		conn.writeMessage(230, "Password ok, continue")
	} else {
		conn.writeMessage(530, "Incorrect password, not logged in")
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

func (cmd commandPasv) Execute(conn *Conn, param string) {
	listenIP := conn.passiveListenIP()
	// TODO: IPv6 for this command is not implemented
	if strings.HasPrefix(listenIP, "::") {
		conn.writeMessage(550, fmt.Sprint("Action not taken "))
		return
	}

	socket, err := conn.newPassiveSocket()
	if err != nil {
		conn.writeMessage(425, "Data connection failed")
		return
	}

	p1 := socket.Port() / 256
	p2 := socket.Port() - (p1 * 256)

	quads := strings.Split(listenIP, ".")
	target := fmt.Sprintf("(%s,%s,%s,%s,%d,%d)", quads[0], quads[1], quads[2], quads[3], p1, p2)
	msg := "Entering Passive Mode " + target
	conn.writeMessage(227, msg)
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

func (cmd commandPort) Execute(conn *Conn, param string) {
	nums := strings.Split(param, ",")
	portOne, _ := strconv.Atoi(nums[4])
	portTwo, _ := strconv.Atoi(nums[5])
	port := (portOne * 256) + portTwo
	host := nums[0] + "." + nums[1] + "." + nums[2] + "." + nums[3]
	socket, err := newActiveSocket(host, port, conn.logger, conn.sessionID)
	if err != nil {
		conn.writeMessage(425, "Data connection failed")
		return
	}
	conn.dataConn = socket
	conn.writeMessage(200, "Connection established ("+strconv.Itoa(port)+")")
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

func (cmd commandPwd) Execute(conn *Conn, param string) {
	conn.writeMessage(257, "\""+conn.curDir+"\" is the current directory")
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

func (cmd commandQuit) Execute(conn *Conn, param string) {
	conn.writeMessage(221, "Goodbye")
	conn.Close()
}

// commandRetr responds to the RETR FTP command. It allows the client to
// download a file.
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

func (cmd commandRetr) Execute(conn *Conn, param string) {
	path := conn.buildPath(param)
	defer func() {
		conn.lastFilePos = 0
		conn.appendData = false
	}()
	conn.server.notifiers.BeforeDownloadFile(conn, path)
	size, data, err := conn.driver.GetFile(path, conn.lastFilePos)
	if err == nil {
		defer data.Close()
		conn.writeMessage(150, fmt.Sprintf("Data transfer starting %d bytes", size))
		err = conn.sendOutofBandDataWriter(data)
		conn.server.notifiers.AfterFileDownloaded(conn, path, size, err)
		if err != nil {
			conn.writeMessage(551, "Error reading file")
		}
	} else {
		conn.server.notifiers.AfterFileDownloaded(conn, path, size, err)
		conn.writeMessage(551, "File not available")
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

func (cmd commandRest) Execute(conn *Conn, param string) {
	var err error
	conn.lastFilePos, err = strconv.ParseInt(param, 10, 64)
	if err != nil {
		conn.writeMessage(551, "File not available")
		return
	}

	conn.appendData = true

	conn.writeMessage(350, fmt.Sprint("Start transfer from ", conn.lastFilePos))
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

func (cmd commandRnfr) Execute(conn *Conn, param string) {
	conn.renameFrom = conn.buildPath(param)
	conn.writeMessage(350, "Requested file action pending further information.")
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

func (cmd commandRnto) Execute(conn *Conn, param string) {
	toPath := conn.buildPath(param)
	err := conn.driver.Rename(conn.renameFrom, toPath)
	defer func() {
		conn.renameFrom = ""
	}()

	if err == nil {
		conn.writeMessage(250, "File renamed")
	} else {
		conn.writeMessage(550, fmt.Sprint("Action not taken: ", err))
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

func (cmd commandRmd) Execute(conn *Conn, param string) {
	path := conn.buildPath(param)
	conn.server.notifiers.BeforeDeleteDir(conn, path)
	err := conn.driver.DeleteDir(path)
	conn.server.notifiers.AfterDirDeleted(conn, path, err)
	if err == nil {
		conn.writeMessage(250, "Directory deleted")
	} else {
		conn.writeMessage(550, fmt.Sprint("Directory delete failed: ", err))
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

func (cmd commandAdat) Execute(conn *Conn, param string) {
	conn.writeMessage(550, "Action not taken")
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

func (cmd commandAuth) Execute(conn *Conn, param string) {
	if param == "TLS" && conn.tlsConfig != nil {
		conn.writeMessage(234, "AUTH command OK")
		err := conn.upgradeToTLS()
		if err != nil {
			conn.logger.Printf("Error upgrading connection to TLS %v", err.Error())
		}
	} else {
		conn.writeMessage(550, "Action not taken")
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

func (cmd commandCcc) Execute(conn *Conn, param string) {
	conn.writeMessage(550, "Action not taken")
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

func (cmd commandEnc) Execute(conn *Conn, param string) {
	conn.writeMessage(550, "Action not taken")
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

func (cmd commandMic) Execute(conn *Conn, param string) {
	conn.writeMessage(550, "Action not taken")
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

func (cmd commandPbsz) Execute(conn *Conn, param string) {
	if conn.tls && param == "0" {
		conn.writeMessage(200, "OK")
	} else {
		conn.writeMessage(550, "Action not taken")
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

func (cmd commandProt) Execute(conn *Conn, param string) {
	if conn.tls && param == "P" {
		conn.writeMessage(200, "OK")
	} else if conn.tls {
		conn.writeMessage(536, "Only P level is supported")
	} else {
		conn.writeMessage(550, "Action not taken")
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

func (cmd commandConf) Execute(conn *Conn, param string) {
	conn.writeMessage(550, "Action not taken")
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

func (cmd commandSize) Execute(conn *Conn, param string) {
	path := conn.buildPath(param)
	stat, err := conn.driver.Stat(path)
	if err != nil {
		log.Printf("Size: error(%s)", err)
		conn.writeMessage(450, fmt.Sprint("path", path, "not found"))
	} else {
		conn.writeMessage(213, strconv.Itoa(int(stat.Size())))
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

func (cmd commandStor) Execute(conn *Conn, param string) {
	targetPath := conn.buildPath(param)
	conn.writeMessage(150, "Data transfer starting")

	defer func() {
		conn.appendData = false
	}()

	conn.server.notifiers.BeforePutFile(conn, targetPath)
	size, err := conn.driver.PutFile(targetPath, conn.dataConn, conn.appendData)
	conn.server.notifiers.AfterFilePut(conn, targetPath, size, err)
	if err == nil {
		msg := fmt.Sprintf("OK, received %d bytes", size)
		conn.writeMessage(226, msg)
	} else {
		conn.writeMessage(450, fmt.Sprint("error during transfer: ", err))
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

func (cmd commandStru) Execute(conn *Conn, param string) {
	if strings.ToUpper(param) == "F" {
		conn.writeMessage(200, "OK")
	} else {
		conn.writeMessage(504, "STRU is an obsolete command")
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

func (cmd commandSyst) Execute(conn *Conn, param string) {
	conn.writeMessage(215, "UNIX Type: L8")
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

func (cmd commandType) Execute(conn *Conn, param string) {
	if strings.ToUpper(param) == "A" {
		conn.writeMessage(200, "Type set to ASCII")
	} else if strings.ToUpper(param) == "I" {
		conn.writeMessage(200, "Type set to binary")
	} else {
		conn.writeMessage(500, "Invalid type")
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

func (cmd commandUser) Execute(conn *Conn, param string) {
	conn.reqUser = param
	conn.server.notifiers.BeforeLoginUser(conn, conn.reqUser)
	if conn.tls || conn.tlsConfig == nil {
		conn.writeMessage(331, "User name ok, password required")
	} else {
		conn.writeMessage(534, "Unsecured login not allowed. AUTH TLS required")
	}
}
