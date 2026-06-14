// Package api provides types used by the Gmail API.
package api

// Error returned by Gmail API
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

// Error satisfies the error interface
func (e *Error) Error() string { return e.Message }

// Thread represents a Gmail thread
type Thread struct {
	ID       string    `json:"id"`
	Messages []Message `json:"messages"`
}

// Message represents a Gmail message
type Message struct {
	ID           string `json:"id"`
	ThreadID     string `json:"threadId"`
	InternalDate string `json:"internalDate"` // epoch ms, returned as a JSON string by the Gmail API
	Snippet      string `json:"snippet"`
	Payload      *Part  `json:"payload"`
}

// Part is a MIME part
type Part struct {
	PartID   string    `json:"partId"`
	MimeType string    `json:"mimeType"`
	Filename string    `json:"filename"`
	Headers  []Header  `json:"headers"`
	Body     *PartBody `json:"body"`
	Parts    []Part    `json:"parts"`
}

// PartBody holds the body data
type PartBody struct {
	AttachmentID string `json:"attachmentId"`
	Size         int64  `json:"size"`
	Data         string `json:"data"`
}

// Header is a name-value pair
type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ThreadList is returned by threads.list
type ThreadList struct {
	Threads       []ThreadRef `json:"threads"`
	NextPageToken string      `json:"nextPageToken"`
}

// ThreadRef is a thread reference in a list
type ThreadRef struct {
	ID      string `json:"id"`
	Snippet string `json:"snippet"`
}

// AttachmentBody is returned by messages.attachments.get
type AttachmentBody struct {
	Size int64  `json:"size"`
	Data string `json:"data"`
}

