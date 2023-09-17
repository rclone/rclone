// Copyright 2018 The goftp Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package server

import "os"

// Perm represents a perm interface
type Perm interface {
	GetOwner(string) (string, error)
	GetGroup(string) (string, error)
	GetMode(string) (os.FileMode, error)

	ChOwner(string, string) error
	ChGroup(string, string) error
	ChMode(string, os.FileMode) error
}

// SimplePerm implements Perm interface that all files are owned by special owner and group
type SimplePerm struct {
	owner, group string
}

// NewSimplePerm creates a SimplePerm
func NewSimplePerm(owner, group string) *SimplePerm {
	return &SimplePerm{
		owner: owner,
		group: group,
	}
}

// GetOwner returns the file's owner
func (s *SimplePerm) GetOwner(string) (string, error) {
	return s.owner, nil
}

// GetGroup returns the group of the file
func (s *SimplePerm) GetGroup(string) (string, error) {
	return s.group, nil
}

// GetMode returns the file's mode
func (s *SimplePerm) GetMode(string) (os.FileMode, error) {
	return os.ModePerm, nil
}

// ChOwner changed the file's owner
func (s *SimplePerm) ChOwner(string, string) error {
	return nil
}

// ChGroup changed the file's group
func (s *SimplePerm) ChGroup(string, string) error {
	return nil
}

// ChMode changed the file's mode
func (s *SimplePerm) ChMode(string, os.FileMode) error {
	return nil
}
