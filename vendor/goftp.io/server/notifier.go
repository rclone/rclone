// Copyright 2020 The goftp Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package server

// Notifier represents a notification operator interface
type Notifier interface {
	BeforeLoginUser(conn *Conn, userName string)
	BeforePutFile(conn *Conn, dstPath string)
	BeforeDeleteFile(conn *Conn, dstPath string)
	BeforeChangeCurDir(conn *Conn, oldCurDir, newCurDir string)
	BeforeCreateDir(conn *Conn, dstPath string)
	BeforeDeleteDir(conn *Conn, dstPath string)
	BeforeDownloadFile(conn *Conn, dstPath string)
	AfterUserLogin(conn *Conn, userName, password string, passMatched bool, err error)
	AfterFilePut(conn *Conn, dstPath string, size int64, err error)
	AfterFileDeleted(conn *Conn, dstPath string, err error)
	AfterFileDownloaded(conn *Conn, dstPath string, size int64, err error)
	AfterCurDirChanged(conn *Conn, oldCurDir, newCurDir string, err error)
	AfterDirCreated(conn *Conn, dstPath string, err error)
	AfterDirDeleted(conn *Conn, dstPath string, err error)
}

type notifierList []Notifier

var (
	_ Notifier = notifierList{}
)

func (notifiers notifierList) BeforeLoginUser(conn *Conn, userName string) {
	for _, notifier := range notifiers {
		notifier.BeforeLoginUser(conn, userName)
	}
}

func (notifiers notifierList) BeforePutFile(conn *Conn, dstPath string) {
	for _, notifier := range notifiers {
		notifier.BeforePutFile(conn, dstPath)
	}
}

func (notifiers notifierList) BeforeDeleteFile(conn *Conn, dstPath string) {
	for _, notifier := range notifiers {
		notifier.BeforeDeleteFile(conn, dstPath)
	}
}

func (notifiers notifierList) BeforeChangeCurDir(conn *Conn, oldCurDir, newCurDir string) {
	for _, notifier := range notifiers {
		notifier.BeforeChangeCurDir(conn, oldCurDir, newCurDir)
	}
}

func (notifiers notifierList) BeforeCreateDir(conn *Conn, dstPath string) {
	for _, notifier := range notifiers {
		notifier.BeforeCreateDir(conn, dstPath)
	}
}

func (notifiers notifierList) BeforeDeleteDir(conn *Conn, dstPath string) {
	for _, notifier := range notifiers {
		notifier.BeforeDeleteDir(conn, dstPath)
	}
}

func (notifiers notifierList) BeforeDownloadFile(conn *Conn, dstPath string) {
	for _, notifier := range notifiers {
		notifier.BeforeDownloadFile(conn, dstPath)
	}
}

func (notifiers notifierList) AfterUserLogin(conn *Conn, userName, password string, passMatched bool, err error) {
	for _, notifier := range notifiers {
		notifier.AfterUserLogin(conn, userName, password, passMatched, err)
	}
}

func (notifiers notifierList) AfterFilePut(conn *Conn, dstPath string, size int64, err error) {
	for _, notifier := range notifiers {
		notifier.AfterFilePut(conn, dstPath, size, err)
	}
}

func (notifiers notifierList) AfterFileDeleted(conn *Conn, dstPath string, err error) {
	for _, notifier := range notifiers {
		notifier.AfterFileDeleted(conn, dstPath, err)
	}
}

func (notifiers notifierList) AfterFileDownloaded(conn *Conn, dstPath string, size int64, err error) {
	for _, notifier := range notifiers {
		notifier.AfterFileDownloaded(conn, dstPath, size, err)
	}
}

func (notifiers notifierList) AfterCurDirChanged(conn *Conn, oldCurDir, newCurDir string, err error) {
	for _, notifier := range notifiers {
		notifier.AfterCurDirChanged(conn, oldCurDir, newCurDir, err)
	}
}

func (notifiers notifierList) AfterDirCreated(conn *Conn, dstPath string, err error) {
	for _, notifier := range notifiers {
		notifier.AfterDirCreated(conn, dstPath, err)
	}
}

func (notifiers notifierList) AfterDirDeleted(conn *Conn, dstPath string, err error) {
	for _, notifier := range notifiers {
		notifier.AfterDirDeleted(conn, dstPath, err)
	}
}

// NullNotifier implements Notifier
type NullNotifier struct{}

var (
	_ Notifier = &NullNotifier{}
)

// BeforeLoginUser implements Notifier
func (NullNotifier) BeforeLoginUser(conn *Conn, userName string) {
}

// BeforePutFile implements Notifier
func (NullNotifier) BeforePutFile(conn *Conn, dstPath string) {
}

// BeforeDeleteFile implements Notifier
func (NullNotifier) BeforeDeleteFile(conn *Conn, dstPath string) {
}

// BeforeChangeCurDir implements Notifier
func (NullNotifier) BeforeChangeCurDir(conn *Conn, oldCurDir, newCurDir string) {
}

// BeforeCreateDir implements Notifier
func (NullNotifier) BeforeCreateDir(conn *Conn, dstPath string) {
}

// BeforeDeleteDir implements Notifier
func (NullNotifier) BeforeDeleteDir(conn *Conn, dstPath string) {
}

// BeforeDownloadFile implements Notifier
func (NullNotifier) BeforeDownloadFile(conn *Conn, dstPath string) {
}

// AfterUserLogin implements Notifier
func (NullNotifier) AfterUserLogin(conn *Conn, userName, password string, passMatched bool, err error) {
}

// AfterFilePut implements Notifier
func (NullNotifier) AfterFilePut(conn *Conn, dstPath string, size int64, err error) {
}

// AfterFileDeleted implements Notifier
func (NullNotifier) AfterFileDeleted(conn *Conn, dstPath string, err error) {
}

// AfterFileDownloaded implements Notifier
func (NullNotifier) AfterFileDownloaded(conn *Conn, dstPath string, size int64, err error) {
}

// AfterCurDirChanged implements Notifier
func (NullNotifier) AfterCurDirChanged(conn *Conn, oldCurDir, newCurDir string, err error) {
}

// AfterDirCreated implements Notifier
func (NullNotifier) AfterDirCreated(conn *Conn, dstPath string, err error) {
}

// AfterDirDeleted implements Notifier
func (NullNotifier) AfterDirDeleted(conn *Conn, dstPath string, err error) {
}
