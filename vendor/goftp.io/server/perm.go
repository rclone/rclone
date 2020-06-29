// Copyright 2018 The goftp Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package server

import "os"

type Perm interface {
	GetOwner(string) (string, error)
	GetGroup(string) (string, error)
	GetMode(string) (os.FileMode, error)

	ChOwner(string, string) error
	ChGroup(string, string) error
	ChMode(string, os.FileMode) error
}

type SimplePerm struct {
	owner, group string
}

func NewSimplePerm(owner, group string) *SimplePerm {
	return &SimplePerm{
		owner: owner,
		group: group,
	}
}

func (s *SimplePerm) GetOwner(string) (string, error) {
	return s.owner, nil
}

func (s *SimplePerm) GetGroup(string) (string, error) {
	return s.group, nil
}

func (s *SimplePerm) GetMode(string) (os.FileMode, error) {
	return os.ModePerm, nil
}

func (s *SimplePerm) ChOwner(string, string) error {
	return nil
}

func (s *SimplePerm) ChGroup(string, string) error {
	return nil
}

func (s *SimplePerm) ChMode(string, os.FileMode) error {
	return nil
}
