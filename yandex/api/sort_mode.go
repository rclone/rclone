package src

import "strings"

// SortMode struct - sort mode
type SortMode struct {
	mode string
}

// Default - sort mode
func (m *SortMode) Default() *SortMode {
	return &SortMode{
		mode: "",
	}
}

// ByName - sort mode
func (m *SortMode) ByName() *SortMode {
	return &SortMode{
		mode: "name",
	}
}

// ByPath - sort mode
func (m *SortMode) ByPath() *SortMode {
	return &SortMode{
		mode: "path",
	}
}

// ByCreated - sort mode
func (m *SortMode) ByCreated() *SortMode {
	return &SortMode{
		mode: "created",
	}
}

// ByModified - sort mode
func (m *SortMode) ByModified() *SortMode {
	return &SortMode{
		mode: "modified",
	}
}

// BySize - sort mode
func (m *SortMode) BySize() *SortMode {
	return &SortMode{
		mode: "size",
	}
}

// Reverse - sort mode
func (m *SortMode) Reverse() *SortMode {
	if strings.HasPrefix(m.mode, "-") {
		return &SortMode{
			mode: m.mode[1:],
		}
	}
	return &SortMode{
		mode: "-" + m.mode,
	}
}

func (m *SortMode) String() string {
	return m.mode
}

// UnmarshalJSON sort mode
func (m *SortMode) UnmarshalJSON(value []byte) error {
	if value == nil || len(value) == 0 {
		m.mode = ""
		return nil
	}
	m.mode = string(value)
	if strings.HasPrefix(m.mode, "\"") && strings.HasSuffix(m.mode, "\"") {
		m.mode = m.mode[1 : len(m.mode)-1]
	}
	return nil
}
