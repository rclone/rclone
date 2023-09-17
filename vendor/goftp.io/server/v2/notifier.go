// Copyright 2020 The goftp Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package server

// Notifier represents a notification operator interface
type Notifier interface {
	BeforeLoginUser(ctx *Context, userName string)
	BeforePutFile(ctx *Context, dstPath string)
	BeforeDeleteFile(ctx *Context, dstPath string)
	BeforeChangeCurDir(ctx *Context, oldCurDir, newCurDir string)
	BeforeCreateDir(ctx *Context, dstPath string)
	BeforeDeleteDir(ctx *Context, dstPath string)
	BeforeDownloadFile(ctx *Context, dstPath string)
	AfterUserLogin(ctx *Context, userName, password string, passMatched bool, err error)
	AfterFilePut(ctx *Context, dstPath string, size int64, err error)
	AfterFileDeleted(ctx *Context, dstPath string, err error)
	AfterFileDownloaded(ctx *Context, dstPath string, size int64, err error)
	AfterCurDirChanged(ctx *Context, oldCurDir, newCurDir string, err error)
	AfterDirCreated(ctx *Context, dstPath string, err error)
	AfterDirDeleted(ctx *Context, dstPath string, err error)
}

type notifierList []Notifier

var (
	_ Notifier = notifierList{}
)

func (notifiers notifierList) BeforeLoginUser(ctx *Context, userName string) {
	for _, notifier := range notifiers {
		notifier.BeforeLoginUser(ctx, userName)
	}
}

func (notifiers notifierList) BeforePutFile(ctx *Context, dstPath string) {
	for _, notifier := range notifiers {
		notifier.BeforePutFile(ctx, dstPath)
	}
}

func (notifiers notifierList) BeforeDeleteFile(ctx *Context, dstPath string) {
	for _, notifier := range notifiers {
		notifier.BeforeDeleteFile(ctx, dstPath)
	}
}

func (notifiers notifierList) BeforeChangeCurDir(ctx *Context, oldCurDir, newCurDir string) {
	for _, notifier := range notifiers {
		notifier.BeforeChangeCurDir(ctx, oldCurDir, newCurDir)
	}
}

func (notifiers notifierList) BeforeCreateDir(ctx *Context, dstPath string) {
	for _, notifier := range notifiers {
		notifier.BeforeCreateDir(ctx, dstPath)
	}
}

func (notifiers notifierList) BeforeDeleteDir(ctx *Context, dstPath string) {
	for _, notifier := range notifiers {
		notifier.BeforeDeleteDir(ctx, dstPath)
	}
}

func (notifiers notifierList) BeforeDownloadFile(ctx *Context, dstPath string) {
	for _, notifier := range notifiers {
		notifier.BeforeDownloadFile(ctx, dstPath)
	}
}

func (notifiers notifierList) AfterUserLogin(ctx *Context, userName, password string, passMatched bool, err error) {
	for _, notifier := range notifiers {
		notifier.AfterUserLogin(ctx, userName, password, passMatched, err)
	}
}

func (notifiers notifierList) AfterFilePut(ctx *Context, dstPath string, size int64, err error) {
	for _, notifier := range notifiers {
		notifier.AfterFilePut(ctx, dstPath, size, err)
	}
}

func (notifiers notifierList) AfterFileDeleted(ctx *Context, dstPath string, err error) {
	for _, notifier := range notifiers {
		notifier.AfterFileDeleted(ctx, dstPath, err)
	}
}

func (notifiers notifierList) AfterFileDownloaded(ctx *Context, dstPath string, size int64, err error) {
	for _, notifier := range notifiers {
		notifier.AfterFileDownloaded(ctx, dstPath, size, err)
	}
}

func (notifiers notifierList) AfterCurDirChanged(ctx *Context, oldCurDir, newCurDir string, err error) {
	for _, notifier := range notifiers {
		notifier.AfterCurDirChanged(ctx, oldCurDir, newCurDir, err)
	}
}

func (notifiers notifierList) AfterDirCreated(ctx *Context, dstPath string, err error) {
	for _, notifier := range notifiers {
		notifier.AfterDirCreated(ctx, dstPath, err)
	}
}

func (notifiers notifierList) AfterDirDeleted(ctx *Context, dstPath string, err error) {
	for _, notifier := range notifiers {
		notifier.AfterDirDeleted(ctx, dstPath, err)
	}
}

// NullNotifier implements Notifier
type NullNotifier struct{}

var (
	_ Notifier = &NullNotifier{}
)

// BeforeLoginUser implements Notifier
func (NullNotifier) BeforeLoginUser(ctx *Context, userName string) {
}

// BeforePutFile implements Notifier
func (NullNotifier) BeforePutFile(ctx *Context, dstPath string) {
}

// BeforeDeleteFile implements Notifier
func (NullNotifier) BeforeDeleteFile(ctx *Context, dstPath string) {
}

// BeforeChangeCurDir implements Notifier
func (NullNotifier) BeforeChangeCurDir(ctx *Context, oldCurDir, newCurDir string) {
}

// BeforeCreateDir implements Notifier
func (NullNotifier) BeforeCreateDir(ctx *Context, dstPath string) {
}

// BeforeDeleteDir implements Notifier
func (NullNotifier) BeforeDeleteDir(ctx *Context, dstPath string) {
}

// BeforeDownloadFile implements Notifier
func (NullNotifier) BeforeDownloadFile(ctx *Context, dstPath string) {
}

// AfterUserLogin implements Notifier
func (NullNotifier) AfterUserLogin(ctx *Context, userName, password string, passMatched bool, err error) {
}

// AfterFilePut implements Notifier
func (NullNotifier) AfterFilePut(ctx *Context, dstPath string, size int64, err error) {
}

// AfterFileDeleted implements Notifier
func (NullNotifier) AfterFileDeleted(ctx *Context, dstPath string, err error) {
}

// AfterFileDownloaded implements Notifier
func (NullNotifier) AfterFileDownloaded(ctx *Context, dstPath string, size int64, err error) {
}

// AfterCurDirChanged implements Notifier
func (NullNotifier) AfterCurDirChanged(ctx *Context, oldCurDir, newCurDir string, err error) {
}

// AfterDirCreated implements Notifier
func (NullNotifier) AfterDirCreated(ctx *Context, dstPath string, err error) {
}

// AfterDirDeleted implements Notifier
func (NullNotifier) AfterDirDeleted(ctx *Context, dstPath string, err error) {
}
