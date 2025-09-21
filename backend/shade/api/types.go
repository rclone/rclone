package api

// -------------------------------------------------
// ListDir Response format
// -------------------------------------------------

type ListDirResponse struct {
	Type  string `json:"type"`  // "file" or "tree"
	Path  string `json:"path"`  // Full path including root
	Ino   int    `json:"ino"`   // inode number
	Mtime int64  `json:"mtime"` // Modified time in milliseconds
	Ctime int64  `json:"ctime"` // Created time in milliseconds
	Size  int64  `json:"size"`  // Size in bytes
	Hash  string `json:"hash"`  // MD5 hash
	Draft bool   `json:"draft"` // Whether this is a draft file
}

type PartURL struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

type CompletedPart struct {
	ETag       string
	PartNumber int32
}
