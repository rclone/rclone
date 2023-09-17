// Copyright 2018 The goftp Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package server

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type listFormatter []FileInfo

// Short returns a string that lists the collection of files by name only,
// one per line
func (formatter listFormatter) Short() []byte {
	var buf bytes.Buffer
	for _, file := range formatter {
		fmt.Fprintf(&buf, "%s\r\n", file.Name())
	}
	return buf.Bytes()
}

// Detailed returns a string that lists the collection of files with extra
// detail, one per line
func (formatter listFormatter) Detailed() []byte {
	var buf bytes.Buffer
	for _, file := range formatter {
		fmt.Fprint(&buf, file.Mode().String())
		fmt.Fprintf(&buf, " 1 %s %s ", file.Owner(), file.Group())
		fmt.Fprint(&buf, lpad(strconv.FormatInt(file.Size(), 10), 12))
		if file.ModTime().Before(time.Now().AddDate(-1, 0, 0)) {
			fmt.Fprint(&buf, file.ModTime().Format(" Jan _2  2006 "))
		} else{
			fmt.Fprint(&buf, file.ModTime().Format(" Jan _2 15:04 "))
		}
		fmt.Fprintf(&buf, "%s\r\n", file.Name())
	}
	return buf.Bytes()
}

func lpad(input string, length int) (result string) {
	if len(input) < length {
		result = strings.Repeat(" ", length-len(input)) + input
	} else if len(input) == length {
		result = input
	} else {
		result = input[0:length]
	}
	return
}
