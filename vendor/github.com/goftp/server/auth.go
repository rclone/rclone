// Copyright 2018 The goftp Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package server

// Auth is an interface to auth your ftp user login.
type Auth interface {
	CheckPasswd(string, string) (bool, error)
}

var (
	_ Auth = &SimpleAuth{}
)

// SimpleAuth implements Auth interface to provide a memory user login auth
type SimpleAuth struct {
	Name     string
	Password string
}

// CheckPasswd will check user's password
func (a *SimpleAuth) CheckPasswd(name, pass string) (bool, error) {
	if name != a.Name || pass != a.Password {
		return false, nil
	}
	return true, nil
}
