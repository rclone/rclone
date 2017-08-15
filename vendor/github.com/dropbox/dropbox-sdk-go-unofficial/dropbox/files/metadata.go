// Copyright (c) Dropbox, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package files

import "encoding/json"

type listFolderResult struct {
	Entries []metadataUnion `json:"entries"`
	Cursor  string          `json:"cursor"`
	HasMore bool            `json:"has_more"`
}

// UnmarshalJSON deserializes into a ListFolderResult instance
func (r *ListFolderResult) UnmarshalJSON(b []byte) error {
	var l listFolderResult
	if err := json.Unmarshal(b, &l); err != nil {
		return err
	}
	r.Cursor = l.Cursor
	r.HasMore = l.HasMore
	r.Entries = make([]IsMetadata, len(l.Entries))
	for i, e := range l.Entries {
		switch e.Tag {
		case "file":
			r.Entries[i] = e.File
		case "folder":
			r.Entries[i] = e.Folder
		case "deleted":
			r.Entries[i] = e.Deleted
		}
	}
	return nil
}

type searchMatch struct {
	MatchType *SearchMatchType `json:"match_type"`
	Metadata  metadataUnion    `json:"metadata"`
}

// UnmarshalJSON deserializes into a SearchMatch instance
func (s *SearchMatch) UnmarshalJSON(b []byte) error {
	var m searchMatch
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	s.MatchType = m.MatchType
	e := m.Metadata
	switch e.Tag {
	case "file":
		s.Metadata = e.File
	case "folder":
		s.Metadata = e.Folder
	case "deleted":
		s.Metadata = e.Deleted
	}
	return nil
}

type deleteResult struct {
	FileOpsResult
	Metadata metadataUnion `json:"metadata"`
}

// UnmarshalJSON deserializes into a SearchMatch instance
func (s *DeleteResult) UnmarshalJSON(b []byte) error {
	var m deleteResult
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	s.FileOpsResult = m.FileOpsResult
	e := m.Metadata
	switch e.Tag {
	case "file":
		s.Metadata = e.File
	case "folder":
		s.Metadata = e.Folder
	case "deleted":
		s.Metadata = e.Deleted
	}
	return nil
}
