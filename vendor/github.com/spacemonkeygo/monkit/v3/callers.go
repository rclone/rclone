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
	slash_pieces := strings.Split(frame.Function, "/")
	dot_pieces := strings.SplitN(slash_pieces[len(slash_pieces)-1], ".", 2)
	return dot_pieces[len(dot_pieces)-1]
}
