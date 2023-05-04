// Package api provides types used by the Sia API.
package api

import "strings"

type FileInfo struct {
	Name string `json:"name"`
	Size uint64 `json:"size"`
}

type ObjectResponse struct {
	Object ObjectInfo `json:"object"`
}

type ObjectInfo struct {
	Slabs []Slab `json:"Slabs"`
}

func (o *ObjectInfo) Size() int64 {
	size := int64(0)
	for _, slab := range o.Slabs {
		size += int64(slab.Length)
	}
	return size
}

type Slab struct {
	Length uint64 `json:"Length"`
}

// Error contains an error message per https://sia.tech/docs/#error
type Error struct {
	Message    string `json:"message"`
	Status     string
	StatusCode int
}

// Error returns a string for the error and satisfies the error interface
func (e *Error) Error() string {
	var out []string
	if e.Message != "" {
		out = append(out, e.Message)
	}
	if e.Status != "" {
		out = append(out, e.Status)
	}
	if len(out) == 0 {
		return "Siad Error"
	}
	return strings.Join(out, ": ")
}
