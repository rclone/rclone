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

package sharing

import "encoding/json"

type listSharedLinksResult struct {
	Links   []sharedLinkMetadataUnion `json:"links"`
	HasMore bool                      `json:"has_more"`
	Cursor  string                    `json:"cursor,omitempty"`
}

// UnmarshalJSON deserializes into a ListSharedLinksResult instance
func (r *ListSharedLinksResult) UnmarshalJSON(b []byte) error {
	var l listSharedLinksResult
	if err := json.Unmarshal(b, &l); err != nil {
		return err
	}
	r.Cursor = l.Cursor
	r.HasMore = l.HasMore
	r.Links = make([]IsSharedLinkMetadata, len(l.Links))
	for i, e := range l.Links {
		switch e.Tag {
		case "file":
			r.Links[i] = e.File
		case "folder":
			r.Links[i] = e.Folder
		}
	}
	return nil
}
