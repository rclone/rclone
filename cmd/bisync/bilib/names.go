package bilib

import (
	"bytes"
	"os"
	"sort"
	"strconv"
)

// Names comprises a set of file names
type Names map[string]any

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
	return os.WriteFile(path, buf.Bytes(), PermSecure)
}

// AliasMap comprises a pair of names that are not equal but treated as equal for comparison purposes
// For example, when normalizing unicode and casing
// This helps reduce repeated normalization functions, which really slow things down
type AliasMap map[string]string

// Add adds new pair to the set, in both directions
func (am AliasMap) Add(name1, name2 string) {
	if name1 != name2 {
		am[name1] = name2
		am[name2] = name1
	}
}

// Alias returns the alternate version, if any, else the original.
func (am AliasMap) Alias(name1 string) string {
	// note: we don't need to check normalization settings, because we already did it in March.
	// the AliasMap will only exist if March paired up two unequal filenames.
	name2, ok := am[name1]
	if ok {
		return name2
	}
	return name1
}
