// Fake Gmail API HTTP server for unit tests.
//
// The List/Open tests in gmailfs_test.go build a bare *Fs via newTestFs (no
// network). TestMain points the package-level testSrv hook at this fake server
// so the API-calling code paths (threads.list, threads.get, attachments.get)
// run against deterministic, pre-baked responses.
package gmailfs

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/rclone/rclone/backend/gmailfs/api"
	"github.com/rclone/rclone/lib/rest"
)

func TestMain(m *testing.M) {
	server := httptest.NewServer(http.HandlerFunc(fakeGmailHandler))
	testSrv = rest.NewClient(server.Client()).SetRoot(server.URL)
	code := m.Run()
	server.Close()
	testSrv = nil
	os.Exit(code)
}

func b64url(b []byte) string { return base64.URLEncoding.EncodeToString(b) }

// fakeGmailHandler serves pre-baked Gmail API responses for the test fixtures.
func fakeGmailHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	w.Header().Set("Content-Type", "application/json")

	switch {
	case path == "users/me/threads":
		writeThreadsList(w, r)
	case strings.HasPrefix(path, "users/me/threads/"):
		writeThreadGet(w, strings.TrimPrefix(path, "users/me/threads/"))
	case strings.Contains(path, "/attachments/"):
		writeAttachment(w, path)
	case strings.HasPrefix(path, "users/me/messages/"):
		writeMessageGet(w, strings.TrimPrefix(path, "users/me/messages/"))
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

// slashSubject is the SMIG fixture subject containing raw "/" runes. The
// sanitized form must replace every "/" with U+2215 ("∕").
const slashSubject = "SMIG, Tuesday, June 23, 2-3 pm EST/1-2 pm CST/11 am PST"

// slashFilename is an attachment filename containing a raw "/".
const slashFilename = "report 1/2.pdf"

func writeThreadsList(w http.ResponseWriter, r *http.Request) {
	// Two-page pagination so TestList_DayExhaustsPagination sees >= 2 threads.
	// Page 2 also carries a "/"-subject thread for the slash-encoding tests.
	if r.URL.Query().Get("pageToken") == "" {
		_ = json.NewEncoder(w).Encode(api.ThreadList{
			Threads:       []api.ThreadRef{{ID: "18c0e1abcd1234ef", Snippet: "Hello there"}},
			NextPageToken: "page2",
		})
		return
	}
	_ = json.NewEncoder(w).Encode(api.ThreadList{
		Threads: []api.ThreadRef{
			{ID: "18c0noattach", Snippet: "Plain text"},
			{ID: "18c0slashthread", Snippet: "SMIG"},
		},
	})
}

func writeThreadGet(w http.ResponseWriter, threadID string) {
	headers := func(subject, msgID string) []api.Header {
		return []api.Header{
			{Name: "Date", Value: "Mon, 15 Jan 2024 00:00:00 +0000"},
			{Name: "From", Value: "alice@example.com"},
			{Name: "To", Value: "bob@example.com"},
			{Name: "Subject", Value: subject},
			{Name: "Message-ID", Value: "<" + msgID + "@example.com>"},
		}
	}
	switch threadID {
	case "18c0e1abcd1234ef":
		_ = json.NewEncoder(w).Encode(api.Thread{
			ID: "18c0e1abcd1234ef",
			Messages: []api.Message{{
				ID:           "18c0msg01",
				ThreadID:     "18c0e1abcd1234ef",
				InternalDate: "1705276800000",
				Payload: &api.Part{
					MimeType: "multipart/mixed",
					Headers:  headers("Hello there", "18c0msg01"),
					Parts: []api.Part{
						{MimeType: "text/plain", Body: &api.PartBody{Data: b64url([]byte("Hello there"))}},
						{MimeType: "application/pdf", Filename: "invoice.pdf", Body: &api.PartBody{AttachmentID: "att01", Size: 22}},
					},
				},
			}},
		})
	case "18c0noattach":
		_ = json.NewEncoder(w).Encode(api.Thread{
			ID: "18c0noattach",
			Messages: []api.Message{{
				ID:           "18c0noattach01",
				ThreadID:     "18c0noattach",
				InternalDate: "1705276800000",
				Payload: &api.Part{
					MimeType: "text/plain",
					Headers:  headers("Plain", "noattach"),
					Body:     &api.PartBody{Data: b64url([]byte("Plain text"))},
				},
			}},
		})
	case "18c0slashthread":
		// A thread whose Subject and one attachment Filename both contain raw "/".
		_ = json.NewEncoder(w).Encode(api.Thread{
			ID: "18c0slashthread",
			Messages: []api.Message{{
				ID:           "18c0slashmsg01",
				ThreadID:     "18c0slashthread",
				InternalDate: "1705276800000",
				Payload: &api.Part{
					MimeType: "multipart/mixed",
					Headers:  headers(slashSubject, "18c0slashmsg01"),
					Parts: []api.Part{
						{MimeType: "text/plain", Body: &api.PartBody{Data: b64url([]byte("see attached"))}},
						{MimeType: "application/pdf", Filename: slashFilename, Body: &api.PartBody{AttachmentID: "att01", Size: 11}},
					},
				},
			}},
		})
	default:
		http.Error(w, "thread not found", http.StatusNotFound)
	}
}

func writeMessageGet(w http.ResponseWriter, messageID string) {
	_ = json.NewEncoder(w).Encode(api.Message{
		ID:           messageID,
		InternalDate: "1705276800000",
		Payload: &api.Part{
			MimeType: "text/plain",
			Headers: []api.Header{
				{Name: "Date", Value: "Mon, 15 Jan 2024 00:00:00 +0000"},
				{Name: "From", Value: "alice@example.com"},
				{Name: "To", Value: "bob@example.com"},
				{Name: "Subject", Value: "Hello"},
				{Name: "Message-ID", Value: "<" + messageID + "@example.com>"},
			},
			Body: &api.PartBody{Data: b64url([]byte("body"))},
		},
	})
}

func writeAttachment(w http.ResponseWriter, path string) {
	parts := strings.Split(path, "/")
	attID := parts[len(parts)-1]
	switch attID {
	case "att01":
		_ = json.NewEncoder(w).Encode(api.AttachmentBody{Data: b64url([]byte("PDF-bytes\x00\x01\x02hello"))})
	case "att02":
		_ = json.NewEncoder(w).Encode(api.AttachmentBody{Data: b64url([]byte{0xfb, 0xff, 0xbf})})
	default:
		http.Error(w, "attachment not found", http.StatusNotFound)
	}
}
