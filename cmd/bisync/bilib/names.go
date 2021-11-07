package bilib

import (
	"bytes"
	"io/ioutil"
	"sort"
	"strconv"
)

// Names comprises a set of file names
type Names map[string]interface{}

// ToNames converts string slice to a set of names
func ToNames(list []string) Names {
	ns := Names{}
	for _, f := range list {
		ns.Add(f)
	}
	return ns
}

// Add adds new file name to the set
func (ns Names) Add(name string) {
	ns[name] = nil
}

// Has checks whether given name is present in the set
func (ns Names) Has(name string) bool {
	_, ok := ns[name]
	return ok
}

// NotEmpty checks whether set is not empty
func (ns Names) NotEmpty() bool {
	return len(ns) > 0
}

// ToList converts name set to string slice
func (ns Names) ToList() []string {
	list := []string{}
	for file := range ns {
		list = append(list, file)
	}
	sort.Strings(list)
	return list
}

// Save saves name set in a text file
func (ns Names) Save(path string) error {
	return SaveList(ns.ToList(), path)
}

// SaveList saves file name list in a text file
func SaveList(list []string, path string) error {
	buf := &bytes.Buffer{}
	for _, s := range list {
		_, _ = buf.WriteString(strconv.Quote(s))
		_ = buf.WriteByte('\n')
	}
	return ioutil.WriteFile(path, buf.Bytes(), PermSecure)
}
