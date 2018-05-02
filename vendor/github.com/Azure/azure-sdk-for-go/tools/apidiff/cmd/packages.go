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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/azure-sdk-for-go/tools/apidiff/exports"
	"github.com/Azure/azure-sdk-for-go/tools/apidiff/repo"
	"github.com/spf13/cobra"
)

var packagesCmd = &cobra.Command{
	Use:   "packages <package search dir> (<base commit> <target commit(s)>) | (<commit sequence>)",
	Short: "Generates a report for all packages under the specified directory containing the delta between commits.",
	Long: `The packages command generates a report for all of the packages under the directory specified in <package dir>.
Commits can be specified as either a base and one or more target commits or a sequence of commits.
For a base/target pair each target commit is compared against the base commit.
For a commit sequence each commit N in the sequence is compared against commit N+1.
Commit sequences must be comma-delimited.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		rpt, err := thePackagesCmd(args)
		if err != nil {
			return err
		}
		evalReportStatus(rpt)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(packagesCmd)
}

// ExecPackagesCmd is the programmatic interface for the packages command.
func ExecPackagesCmd(pkgDir string, commitSeq string, flags CommandFlags) (CommitPkgsReport, error) {
	flags.apply()
	return thePackagesCmd([]string{pkgDir, commitSeq})
}

// split into its own func as we can't call os.Exit from it (the defer won't get executed)
func thePackagesCmd(args []string) (rpt CommitPkgsReport, err error) {
	cloneRepo, err := processArgsAndClone(args)
	if err != nil {
		return
	}

	rpt.CommitsReports = map[string]pkgsReport{}
	worker := func(rootDir string, cloneRepo repo.WorkingTree, baseCommit, targetCommit string) error {
		// get for lhs
		vprintf("checking out base commit %s and gathering exports\n", baseCommit)
		lhs, err := getRepoContentForCommit(cloneRepo, rootDir, baseCommit)
		if err != nil {
			return err
		}

		// get for rhs
		vprintf("checking out target commit %s and gathering exports\n", targetCommit)
		var rhs repoContent
		rhs, err = getRepoContentForCommit(cloneRepo, rootDir, targetCommit)
		if err != nil {
			return err
		}
		r := getPkgsReport(lhs, rhs)
		rpt.updateAffectedPackages(targetCommit, r)
		if r.hasBreakingChanges() {
			rpt.BreakingChanges = append(rpt.BreakingChanges, targetCommit)
		}
		rpt.CommitsReports[fmt.Sprintf("%s:%s", baseCommit, targetCommit)] = r
		return nil
	}

	err = generateReports(args, cloneRepo, worker)
	if err != nil {
		return
	}

	err = printReport(rpt)
	return
}

func getRepoContentForCommit(wt repo.WorkingTree, dir, commit string) (r repoContent, err error) {
	err = wt.Checkout(commit)
	if err != nil {
		err = fmt.Errorf("failed to check out commit '%s': %s", commit, err)
		return
	}

	pkgDirs := []string{}
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// check if leaf dir
			fi, err := ioutil.ReadDir(path)
			if err != nil {
				return err
			}
			hasSubDirs := false
			for _, f := range fi {
				if f.IsDir() {
					hasSubDirs = true
					break
				}
			}
			if !hasSubDirs {
				pkgDirs = append(pkgDirs, path)
			}
		}
		return nil
	})
	if err != nil {
		return
	}
	if verboseFlag {
		fmt.Println("found the following package directories")
		for _, d := range pkgDirs {
			fmt.Printf("\t%s\n", d)
		}
	}

	r, err = getExportsForPackages(pkgDirs)
	if err != nil {
		err = fmt.Errorf("failed to get exports for commit '%s': %s", commit, err)
	}
	return
}

// contains repo content, it's structured as "package":"apiversion":content
type repoContent map[string]map[string]exports.Content

// returns repoContent based on the provided slice of package directories
func getExportsForPackages(pkgDirs []string) (repoContent, error) {
	exps := repoContent{}
	for _, pkgDir := range pkgDirs {
		vprintf("getting exports for %s\n", pkgDir)
		// pkgDir = "D:\work\src\github.com\Azure\azure-sdk-for-go\services\analysisservices\mgmt\2016-05-16\analysisservices"
		// we want ver = "2016-05-16", pkg = "analysisservices"
		i := strings.LastIndexByte(pkgDir, os.PathSeparator)
		j := strings.LastIndexByte(pkgDir[:i], os.PathSeparator)
		ver := pkgDir[j+1 : j+(i-j)]
		pkg := pkgDir[i+1:]

		if _, ok := exps[pkg]; !ok {
			exps[pkg] = map[string]exports.Content{}
		}

		if _, ok := exps[pkg][ver]; !ok {
			exp, err := exports.Get(pkgDir)
			if err != nil {
				return nil, err
			}
			exps[pkg][ver] = exp
		}
	}
	return exps, nil
}

// contains a collection of packages, it's structured as "package":{"apiver1", "apiver2",...}
type pkgsList map[string][]string

// contains a collection of package reports, it's structured as "package":"apiversion":pkgReport
type modifiedPackages map[string]map[string]pkgReport

// CommitPkgsReport represents a collection of reports, one for each commit hash.
type CommitPkgsReport struct {
	AffectedPackages pkgsList              `json:"affectedPackages"`
	BreakingChanges  []string              `json:"breakingChanges,omitempty"`
	CommitsReports   map[string]pkgsReport `json:"deltas"`
}

// returns true if the report contains no data
func (c CommitPkgsReport) isEmpty() bool {
	for _, r := range c.CommitsReports {
		if !r.isEmpty() {
			return false
		}
	}
	return true
}

// returns true if the report contains breaking changes
func (c CommitPkgsReport) hasBreakingChanges() bool {
	for _, r := range c.CommitsReports {
		if r.hasBreakingChanges() {
			return true
		}
	}
	return false
}

// returns true if the package contains additive changes
func (c CommitPkgsReport) hasAdditiveChanges() bool {
	for _, r := range c.CommitsReports {
		if r.hasAdditiveChanges() {
			return true
		}
	}
	return false
}

// updates the collection of affected packages with the packages that were touched in the specified commit
func (c *CommitPkgsReport) updateAffectedPackages(commit string, r pkgsReport) {
	if c.AffectedPackages == nil {
		c.AffectedPackages = map[string][]string{}
	}

	for pkgName, apiVers := range r.AddedPackages {
		for _, apiVer := range apiVers {
			c.AffectedPackages[commit] = append(c.AffectedPackages[commit], fmt.Sprintf("%s/%s", pkgName, apiVer))
		}
	}

	for pkgName, apiVers := range r.ModifiedPackages {
		for apiVer := range apiVers {
			c.AffectedPackages[commit] = append(c.AffectedPackages[commit], fmt.Sprintf("%s/%s", pkgName, apiVer))
		}
	}

	for pkgName, apiVers := range r.RemovedPackages {
		for _, apiVer := range apiVers {
			c.AffectedPackages[commit] = append(c.AffectedPackages[commit], fmt.Sprintf("%s/%s", pkgName, apiVer))
		}
	}
}

// represents a complete report of added, removed, and modified packages
type pkgsReport struct {
	AddedPackages      pkgsList         `json:"added,omitempty"`
	ModifiedPackages   modifiedPackages `json:"modified,omitempty"`
	RemovedPackages    pkgsList         `json:"removed,omitempty"`
	modPkgHasAdditions bool
	modPkgHasBreaking  bool
}

// returns true if the package report contains breaking changes
func (r pkgsReport) hasBreakingChanges() bool {
	return len(r.RemovedPackages) > 0 || r.modPkgHasBreaking
}

// returns true if the package report contains additive changes
func (r pkgsReport) hasAdditiveChanges() bool {
	return len(r.AddedPackages) > 0 || r.modPkgHasAdditions
}

// returns true if the report contains no data
func (r pkgsReport) isEmpty() bool {
	return len(r.AddedPackages) == 0 && len(r.ModifiedPackages) == 0 && len(r.RemovedPackages) == 0
}

// generates a pkgsReport based on the delta between lhs and rhs
func getPkgsReport(lhs, rhs repoContent) pkgsReport {
	report := pkgsReport{}

	if !onlyBreakingChangesFlag {
		report.AddedPackages = getPkgsList(lhs, rhs)
	}
	if !onlyAdditionsFlag {
		report.RemovedPackages = getPkgsList(rhs, lhs)
	}

	// diff packages
	for rhsK, rhsV := range rhs {
		if _, ok := lhs[rhsK]; !ok {
			continue
		}
		for rhsAPI, rhsCnt := range rhsV {
			if _, ok := lhs[rhsK][rhsAPI]; !ok {
				continue
			}
			if r := getPkgReport(lhs[rhsK][rhsAPI], rhsCnt); !r.isEmpty() {
				if r.hasBreakingChanges() {
					report.modPkgHasBreaking = true
				}
				if r.hasAdditiveChanges() {
					report.modPkgHasAdditions = true
				}
				// only add an entry if the report contains data
				if report.ModifiedPackages == nil {
					report.ModifiedPackages = modifiedPackages{}
				}
				if _, ok := report.ModifiedPackages[rhsK]; !ok {
					report.ModifiedPackages[rhsK] = map[string]pkgReport{}
				}
				report.ModifiedPackages[rhsK][rhsAPI] = r
			}
		}
	}

	return report
}

// returns a list of packages in rhs that aren't in lhs
func getPkgsList(lhs, rhs repoContent) pkgsList {
	list := pkgsList{}
	for rhsK, rhsV := range rhs {
		if lhsV, ok := lhs[rhsK]; !ok {
			// package doesn't exist, add all API versions
			apis := []string{}
			for rhsAPI := range rhsV {
				apis = append(apis, rhsAPI)
			}
			list[rhsK] = apis
		} else {
			// package exists, check for any new API versions
			for rhsAPI := range rhsV {
				apis := []string{}
				if _, ok := lhsV[rhsAPI]; !ok {
					// API version is new, add to slice
					apis = append(apis, rhsAPI)
				}
				if len(apis) > 0 {
					list[rhsK] = apis
				}
			}
		}
	}
	return list
}
