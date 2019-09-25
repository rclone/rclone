// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package ansiterm

import (
	"fmt"
	"sort"
	"strings"
)

type attribute int

const (
	unknownAttribute attribute = -1
	reset            attribute = 0
)

// sgr returns the escape sequence for the Select Graphic Rendition
// for the attribute.
func (a attribute) sgr() string {
	if a < 0 {
		return ""
	}
	return fmt.Sprintf("\x1b[%dm", a)
}

type attributes []attribute

func (a attributes) Len() int           { return len(a) }
func (a attributes) Less(i, j int) bool { return a[i] < a[j] }
func (a attributes) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

// sgr returns the combined escape sequence for the Select Graphic Rendition
// for the sequence of attributes.
func (a attributes) sgr() string {
	switch len(a) {
	case 0:
		return ""
	case 1:
		return a[0].sgr()
	default:
		sort.Sort(a)
		var values []string
		for _, attr := range a {
			values = append(values, fmt.Sprint(attr))
		}
		return fmt.Sprintf("\x1b[%sm", strings.Join(values, ";"))
	}
}
