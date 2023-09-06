// Copyright (C) 2015 Space Monkey, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package monkit

import (
	"runtime"
	"strings"
)

func callerPackage(frames int) string {
	var pc [1]uintptr
	if runtime.Callers(frames+2, pc[:]) != 1 {
		return "unknown"
	}
	frame, _ := runtime.CallersFrames(pc[:]).Next()
	if frame.Func == nil {
		return "unknown"
	}
	slash_pieces := strings.Split(frame.Func.Name(), "/")
	dot_pieces := strings.SplitN(slash_pieces[len(slash_pieces)-1], ".", 2)
	return strings.Join(slash_pieces[:len(slash_pieces)-1], "/") + "/" + dot_pieces[0]
}

func callerFunc(frames int) string {
	var pc [1]uintptr
	if runtime.Callers(frames+3, pc[:]) != 1 {
		return "unknown"
	}
	frame, _ := runtime.CallersFrames(pc[:]).Next()
	if frame.Function == "" {
		return "unknown"
	}
	funcname, ok := extractFuncName(frame.Function)
	if !ok {
		return "unknown"
	}
	return funcname
}

// extractFuncName splits fully qualified function name:
//
// Input:
//   "github.com/spacemonkeygo/monkit/v3.BenchmarkTask.func1"
//   "main.DoThings.func1"
//   "main.DoThings"
// Output:
//   funcname: "BenchmarkTask.func1"
//   funcname: "DoThings.func1"
//   funcname: "DoThings"
func extractFuncName(fullyQualifiedName string) (funcname string, ok bool) {
	lastSlashPos := strings.LastIndexByte(fullyQualifiedName, '/')
	if lastSlashPos+1 >= len(fullyQualifiedName) {
		// fullyQualifiedName ended with slash.
		return "", false
	}

	qualifiedName := fullyQualifiedName[lastSlashPos+1:]
	packageDotPos := strings.IndexByte(qualifiedName, '.')
	if packageDotPos < 0 || packageDotPos+1 >= len(qualifiedName) {
		// qualifiedName ended with a dot
		return "", false
	}

	return qualifiedName[packageDotPos+1:], true
}
