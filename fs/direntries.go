package fs

import "fmt"

// DirEntries is a slice of Object or *Dir
type DirEntries []DirEntry

// Len is part of sort.Interface.
func (ds DirEntries) Len() int {
	return len(ds)
}

// Swap is part of sort.Interface.
func (ds DirEntries) Swap(i, j int) {
	ds[i], ds[j] = ds[j], ds[i]
}

// Less is part of sort.Interface.
func (ds DirEntries) Less(i, j int) bool {
	return CompareDirEntries(ds[i], ds[j]) < 0
}

// ForObject runs the function supplied on every object in the entries
func (ds DirEntries) ForObject(fn func(o Object)) {
	for _, entry := range ds {
		o, ok := entry.(Object)
		if ok {
			fn(o)
		}
	}
}

// ForObjectError runs the function supplied on every object in the entries
func (ds DirEntries) ForObjectError(fn func(o Object) error) error {
	for _, entry := range ds {
		o, ok := entry.(Object)
		if ok {
			err := fn(o)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// ForDir runs the function supplied on every Directory in the entries
func (ds DirEntries) ForDir(fn func(dir Directory)) {
	for _, entry := range ds {
		dir, ok := entry.(Directory)
		if ok {
			fn(dir)
		}
	}
}

// ForDirError runs the function supplied on every Directory in the entries
func (ds DirEntries) ForDirError(fn func(dir Directory) error) error {
	for _, entry := range ds {
		dir, ok := entry.(Directory)
		if ok {
			err := fn(dir)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// DirEntryType returns a string description of the DirEntry, either
// "object", "directory" or "unknown type XXX"
func DirEntryType(d DirEntry) string {
	switch d.(type) {
	case Object:
		return "object"
	case Directory:
		return "directory"
	}
	return fmt.Sprintf("unknown type %T", d)
}

// CompareDirEntries returns 1 if a > b, 0 if a == b and -1 if a < b
// If two dir entries have the same name, compare their types (directories are before objects)
func CompareDirEntries(a, b DirEntry) int {
	aName := a.Remote()
	bName := b.Remote()

	if aName > bName {
		return 1
	} else if aName < bName {
		return -1
	}

	typeA := DirEntryType(a)
	typeB := DirEntryType(b)

	// same name, compare types
	if typeA > typeB {
		return 1
	} else if typeA < typeB {
		return -1
	}

	return 0
}
