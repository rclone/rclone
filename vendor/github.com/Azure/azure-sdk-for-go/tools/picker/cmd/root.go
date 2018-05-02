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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	apidiff "github.com/Azure/azure-sdk-for-go/tools/apidiff/cmd"
	"github.com/Azure/azure-sdk-for-go/tools/apidiff/repo"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "picker <from> <to>",
	Short: "Cherry-picks commits with non-breaking changes between two branches.",
	Long: `This tool will find the list of commits in branch <from> that are not in
branch <to>, and for each commit found it will cherry-pick it into <to> if
the commit contains no breaking changes.  If a cherry-pick contains a merge
conflict the process will pause so the conflicts can be resolved.
NOTE: running this tool will modify your working tree!`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return theCommand(args)
	},
}

// Execute executes the specified command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(-1)
	}
}

func theCommand(args []string) error {
	if len(args) < 2 {
		return errors.New("not enough arguments were supplied")
	}

	from := args[0]
	to := args[1]

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get the current working directory: %v", err)
	}

	wt, err := repo.Get(cwd)
	if err != nil {
		return fmt.Errorf("failed to get the working tree: %v", err)
	}

	// checkout "from" branch
	fmt.Printf("checking out branch '%s' to get list of candidate commits for cherry-picking\n", from)
	err = wt.Checkout(from)
	if err != nil {
		return fmt.Errorf("checkout failed: %v", err)
	}

	commits, err := wt.Cherry(to)
	if err != nil {
		return fmt.Errorf("the command 'git cherry' failed: %v", err)
	}

	// for each commit, if it wasn't found in the "to" branch add it to the candidate list
	candidates := []string{}
	for _, commit := range commits {
		if !commit.Found {
			candidates = append(candidates, commit.Hash)
		}
	}

	if len(candidates) == 0 {
		fmt.Println("didn't find any candidate commits")
		return nil
	}

	fmt.Printf("found %v candidate commits, looking for breaking changes...\n", len(candidates))

	// generate report to find the breaking changes
	report, err := apidiff.ExecPackagesCmd(filepath.Join(wt.Root(), "services"),
		fmt.Sprintf("%s~1,%s", candidates[0], strings.Join(candidates, ",")),
		apidiff.CommandFlags{Quiet: true})
	if err != nil {
		return fmt.Errorf("failed to obtain the breaking changes report: %v", err)
	}

	forPicking := pruneCandidates(candidates, report)
	if len(forPicking) == 0 {
		fmt.Println("didn't find any commits to cherry-pick")
		return nil
	}

	// now cherry-pick the commits
	fmt.Printf("checking out branch '%s' to begin cherry-picking\n", to)
	err = wt.Checkout(to)
	if err != nil {
		return fmt.Errorf("checkout failed: %v", err)
	}

	for _, commit := range forPicking {
		fmt.Printf("cherry-picking commit %s\n", commit)
		err = wt.CherryPick(commit)
		if err != nil {
			fmt.Printf("there was an error: %v\n", err)
			fmt.Println("if this is due to a merge conflict fix the conflict then press ENTER to continue cherry-picking, else press CTRL-C to abort")
			dummy := ""
			fmt.Scanln(&dummy)
		}
	}
	return nil
}

// performs a linear search of ss, looking for s
func contains(ss []string, s string) bool {
	for i := range ss {
		if ss[i] == s {
			return true
		}
	}
	return false
}

// removes commits and decendents thereof that contain breaking changes
func pruneCandidates(candidates []string, report apidiff.CommitPkgsReport) []string {
	pkgsToSkip := map[string]string{}
	forPicking := []string{}
	for _, commit := range candidates {
		if contains(report.BreakingChanges, commit) {
			fmt.Printf("omitting %s as it contains breaking changes\n", commit)
			// add the affected packages to the collection of packages to skip
			for _, pkg := range report.AffectedPackages[commit] {
				if _, ok := pkgsToSkip[pkg]; !ok {
					pkgsToSkip[pkg] = commit
				}
			}
			continue
		}

		// check the packages impacted by this commit, if any of them are in
		// the list of packages to skip then this commit can't be cherry-picked
		include := true
		for _, pkg := range report.AffectedPackages[commit] {
			if _, ok := pkgsToSkip[pkg]; ok {
				include = false
				fmt.Printf("omitting %s as it's an aggregate of breaking changes commit %s\n", commit, pkgsToSkip[pkg])
				break
			}
		}

		if include {
			forPicking = append(forPicking, commit)
		}
	}
	return forPicking
}
