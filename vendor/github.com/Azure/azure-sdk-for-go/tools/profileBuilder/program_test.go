// +build go1.9

// Copyright 2017 Microsoft Corporation and contributors
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

// profileBuilder creates a series of packages filled entirely with alias types
// and functions supporting those alias types by directing traffic to the
// functions supporting the original types. This is useful associating a series
// of packages in separate API Versions for easier/safer use.
//
// The Azure-SDK-for-Go teams intends to use this tool to generated profiles
// that we will publish in this repository for general use. However, this tool
// in the case that one has their own list of Services at given API Versions,
// this may prove to be a useful tool for you.
package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func Test_getAliasPath(t *testing.T) {
	const profileName = "profile1"
	testCases := []struct {
		original string
		expected string
	}{
		{
			filepath.Join("services", "cdn", "mgmt", "2015-06-01", "cdn"),
			filepath.Join(profileName, "cdn", "mgmt", "cdn"),
		},
		{
			filepath.Join("services", "keyvault", "2016-10-01", "keyvault"),
			filepath.Join(profileName, "keyvault", "keyvault"),
		},
		{
			filepath.Join("services", "keyvault", "mgmt", "2016-10-01", "keyvault"),
			filepath.Join(profileName, "keyvault", "mgmt", "keyvault"),
		},
		{
			filepath.Join("services", "datalake", "analytics", "2016-11-01-preview", "catalog"),
			filepath.Join(profileName, "datalake", "analytics", "catalog"),
		},
	}

	const consistentSeperator = "/"

	pathNorm := func(location string) string {
		return strings.Replace(location, `\`, consistentSeperator, -1)
	}

	pathsEqual := func(left, right string) bool {
		left, right = pathNorm(left), pathNorm(right)
		pieceWiseLeft, pieceWiseRight := strings.Split(left, consistentSeperator), strings.Split(right, consistentSeperator)

		if len(pieceWiseLeft) != len(pieceWiseRight) {
			return false
		}

		for i, lval := range pieceWiseLeft {
			rval := pieceWiseRight[i]
			if lval != rval {
				return false
			}
		}
		return true
	}

	for _, tc := range testCases {
		t.Run(tc.original, func(t *testing.T) {
			got, err := getAliasPath(tc.original, profileName)
			if err != nil {
				t.Error(err)
			}

			if !pathsEqual(tc.expected, got) {
				t.Logf("\ngot: \t%q\nwant:\t%q", pathNorm(got), pathNorm(tc.expected))
				t.Fail()
			}
		})
	}
}
