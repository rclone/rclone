// Copyright 2018 Microsoft Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/tools/apidiff/delta"
	"github.com/Azure/azure-sdk-for-go/tools/apidiff/exports"
	"github.com/Azure/azure-sdk-for-go/tools/apidiff/repo"
)

func printf(format string, a ...interface{}) {
	if !quietFlag {
		fmt.Printf(format, a...)
	}
}

func println(a ...interface{}) {
	if !quietFlag {
		fmt.Println(a...)
	}
}

func vprintf(format string, a ...interface{}) {
	if verboseFlag {
		printf(format, a...)
	}
}

func vprintln(a ...interface{}) {
	if verboseFlag {
		println(a...)
	}
}

// represents a set of breaking changes
type breakingChanges struct {
	Consts     map[string]delta.Signature    `json:"consts,omitempty"`
	Funcs      map[string]delta.FuncSig      `json:"funcs,omitempty"`
	Interfaces map[string]delta.InterfaceDef `json:"interfaces,omitempty"`
	Structs    map[string]delta.StructDef    `json:"structs,omitempty"`
	Removed    *exports.Content              `json:"removed,omitempty"`
}

// returns true if there are no breaking changes
func (bc breakingChanges) isEmpty() bool {
	return len(bc.Consts) == 0 && len(bc.Funcs) == 0 && len(bc.Interfaces) == 0 && len(bc.Structs) == 0
}

// represents a per-package report, contains additive and breaking changes
type pkgReport struct {
	AdditiveChanges *exports.Content `json:"additiveChanges,omitempty"`
	BreakingChanges *breakingChanges `json:"breakingChanges,omitempty"`
}

// returns true if the package report contains breaking changes
func (r pkgReport) hasBreakingChanges() bool {
	return r.BreakingChanges != nil && !r.BreakingChanges.isEmpty()
}

// returns true if the package report contains additive changes
func (r pkgReport) hasAdditiveChanges() bool {
	return r.AdditiveChanges != nil && !r.AdditiveChanges.IsEmpty()
}

// returns true if the report contains no data
func (r pkgReport) isEmpty() bool {
	return (r.AdditiveChanges == nil || r.AdditiveChanges.IsEmpty()) &&
		(r.BreakingChanges == nil || r.BreakingChanges.isEmpty())
}

// generates a package report based on the delta between lhs and rhs
func getPkgReport(lhs, rhs exports.Content) pkgReport {
	r := pkgReport{}
	if !onlyBreakingChangesFlag {
		if adds := delta.GetExports(lhs, rhs); !adds.IsEmpty() {
			r.AdditiveChanges = &adds
		}
	}

	if !onlyAdditionsFlag {
		breaks := breakingChanges{}
		breaks.Consts = delta.GetConstTypeChanges(lhs, rhs)
		breaks.Funcs = delta.GetFuncSigChanges(lhs, rhs)
		breaks.Interfaces = delta.GetInterfaceMethodSigChanges(lhs, rhs)
		breaks.Structs = delta.GetStructFieldChanges(lhs, rhs)
		if removed := delta.GetExports(rhs, lhs); !removed.IsEmpty() {
			breaks.Removed = &removed
		}
		if !breaks.isEmpty() {
			r.BreakingChanges = &breaks
		}
	}
	return r
}

type report interface {
	isEmpty() bool
}

func printReport(r report) error {
	if r.isEmpty() {
		println("no changes were found")
		return nil
	}

	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %v", err)
	}
	println(string(b))
	return nil
}

func processArgsAndClone(args []string) (dir string, cln repo.WorkingTree, clean func(), err error) {
	if onlyAdditionsFlag && onlyBreakingChangesFlag {
		err = errors.New("flags 'additions' and 'breakingchanges' are mutually exclusive")
		return
	}

	if len(args) < 3 {
		err = errors.New("not enough args were supplied")
		return
	}

	dir = args[0]

	src, err := repo.Get(dir)
	if err != nil {
		err = fmt.Errorf("failed to get repository: %v", err)
		return
	}

	tempRepoDir := path.Join(os.TempDir(), fmt.Sprintf("apidiff-%v", time.Now().Unix()))
	vprintf("cloning '%s' into '%s'...\n", src.Root(), tempRepoDir)
	cln, err = src.Clone(tempRepoDir)
	if err != nil {
		err = fmt.Errorf("failed to clone repository: %v", err)
		return
	}

	clean = func() {
		// delete clone
		vprintln("cleaning up clone")
		err = os.RemoveAll(cln.Root())
		if err != nil {
			vprintf("failed to delete temp repo: %v\n", err)
		}
	}

	// fix up pkgDir to the clone
	dir = strings.Replace(dir, src.Root(), cln.Root(), 1)
	return
}

type reportStatus interface {
	hasBreakingChanges() bool
	hasAdditiveChanges() bool
}

// compares report status with the desired report options (breaking/additions)
// to determine if the program should terminate with a non-zero exit code.
func evalReportStatus(r reportStatus) {
	if onlyBreakingChangesFlag && r.hasBreakingChanges() {
		os.Exit(1)
	}
	if onlyAdditionsFlag && !r.hasAdditiveChanges() {
		os.Exit(1)
	}
}
