package telegram

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/gotd/td/tg"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/lib/encoder"
)

func TestNormalizeChatTarget(t *testing.T) {
	tests := []struct {
		input string
		want  string
		bad   bool
	}{
		{input: "", want: ""},
		{input: " me ", want: ""},
		{input: "@rclone", want: "rclone"},
		{input: "rclone", want: "rclone"},
		{input: "-10012345", bad: true},
		{input: "@", bad: true},
		{input: "public/channel", bad: true},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%q", test.input), func(t *testing.T) {
			got, err := normalizeChatTarget(test.input)
			if test.bad {
				if err == nil {
					t.Fatalf("expected an error for %q", test.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != test.want {
				t.Fatalf("normalizeChatTarget(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestUserAuthenticatorPromptsForPhoneAndCode(t *testing.T) {
	oldReadLine := config.ReadLine
	t.Cleanup(func() { config.ReadLine = oldReadLine })
	var prompts []string
	config.ReadLine = func(prompt string) string {
		prompts = append(prompts, prompt)
		if len(prompts) == 1 {
			return "+420123456789"
		}
		return "12345"
	}

	authenticator := userAuthenticator{interactive: true}
	phone, err := authenticator.Phone(context.Background())
	if err != nil || phone != "+420123456789" {
		t.Fatalf("Phone() = (%q, %v), want international phone", phone, err)
	}
	code, err := authenticator.Code(context.Background(), nil)
	if err != nil || code != "12345" {
		t.Fatalf("Code() = (%q, %v), want prompted code", code, err)
	}
	if len(prompts) != 2 {
		t.Fatalf("got %d prompts, want phone and code prompts", len(prompts))
	}
}

func TestUserAuthenticatorRequiresCodeWhenNonInteractive(t *testing.T) {
	_, err := (userAuthenticator{}).Code(context.Background(), nil)
	if err == nil {
		t.Fatal("expected a clear error when no OTP is configured in non-interactive mode")
	}
	_, err = (userAuthenticator{code: " 12345 "}).Code(context.Background(), nil)
	if err != nil {
		t.Fatalf("configured OTP should be accepted: %v", err)
	}
}

func TestConfigSessionStorageObscuresSession(t *testing.T) {
	mapper := configmap.Simple{}
	storage := &configSessionStorage{mapper: mapper}
	want := []byte("test-session-data")
	if err := storage.StoreSession(context.Background(), want); err != nil {
		t.Fatalf("StoreSession returned an error: %v", err)
	}
	stored, ok := mapper.Get("session_string")
	if !ok || stored == string(want) {
		t.Fatal("session was not stored in obscured form")
	}
	got, err := storage.LoadSession(context.Background())
	if err != nil {
		t.Fatalf("LoadSession returned an error: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadSession() = %q, want %q", got, want)
	}
}

func TestRevealConfigSecret(t *testing.T) {
	const want = "telegram-api-hash"
	value, err := obscure.Obscure(want)
	if err != nil {
		t.Fatalf("Obscure returned an error: %v", err)
	}
	got, err := revealConfigSecret("api_hash", value)
	if err != nil {
		t.Fatalf("revealConfigSecret returned an error: %v", err)
	}
	if got != want {
		t.Fatalf("revealConfigSecret() = %q, want %q", got, want)
	}
	if _, err := revealConfigSecret("api_hash", "not-obscured"); err == nil {
		t.Fatal("expected an error for a non-obscured secret")
	}
}

func TestCollectMessagesPaginates(t *testing.T) {
	firstPage := make([]tg.MessageClass, historyPageSize)
	for i := range firstPage {
		firstPage[i] = &tg.Message{ID: 200 - i}
	}
	secondPage := []tg.MessageClass{&tg.Message{ID: 100}}
	getter := &historyStub{pages: [][]tg.MessageClass{firstPage, secondPage}}

	messages, err := collectMessages(context.Background(), &tg.InputPeerSelf{}, getter)
	if err != nil {
		t.Fatalf("collectMessages returned an error: %v", err)
	}
	if len(messages) != 101 {
		t.Fatalf("got %d messages, want 101", len(messages))
	}
	if !reflect.DeepEqual(getter.offsets, []int{0, 101}) {
		t.Fatalf("history offsets = %v, want [0 101]", getter.offsets)
	}
}

type historyStub struct {
	pages   [][]tg.MessageClass
	offsets []int
}

func (s *historyStub) MessagesGetHistory(_ context.Context, request *tg.MessagesGetHistoryRequest) (tg.MessagesMessagesClass, error) {
	s.offsets = append(s.offsets, request.OffsetID)
	pageIndex := len(s.offsets) - 1
	if pageIndex >= len(s.pages) {
		return &tg.MessagesMessages{}, nil
	}
	return &tg.MessagesMessages{Messages: s.pages[pageIndex]}, nil
}

func TestEntriesFromMessagesKeepsNewestDuplicate(t *testing.T) {
	f := &Fs{opt: Options{Encoding: encoder.Base}}
	messages := []*tg.Message{
		testDocumentMessage(30, "report.txt"),
		testDocumentMessage(20, "report.txt"),
		testDocumentMessage(10, ""),
	}

	entries := f.entriesFromMessages(messages)
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	first, ok := entries[0].(*Object)
	if !ok {
		t.Fatalf("first entry has type %T, want *Object", entries[0])
	}
	if first.Remote() != "report.txt" || first.messageID != 30 {
		t.Fatalf("newest duplicate = (%q, %d), want (report.txt, 30)", first.Remote(), first.messageID)
	}
	second, ok := entries[1].(*Object)
	if !ok || second.Remote() != "file-10" {
		t.Fatalf("filename fallback = %T %v, want file-10", entries[1], entries[1])
	}
}

func TestObjectFromMessageEncodesPathSeparatorAndKeepsFileReference(t *testing.T) {
	f := &Fs{opt: Options{Encoding: encoder.Base}}
	message := testDocumentMessage(7, "report/2026.txt")
	object, ok := f.objectFromMessage(message)
	if !ok {
		t.Fatal("document message was not recognized")
	}
	if object.Remote() != "report／2026.txt" {
		t.Fatalf("remote name = %q, want encoded slash", object.Remote())
	}
	if bytes.Equal(object.fileReference, nil) || !bytes.Equal(object.fileReference, []byte{7, 8, 9}) {
		t.Fatalf("file reference = %v, want [7 8 9]", object.fileReference)
	}
	if object.documentID != 700 || object.accessHash != 8 {
		t.Fatalf("document identity = (%d, %d), want (700, 8)", object.documentID, object.accessHash)
	}
}

func testDocumentMessage(id int, fileName string) *tg.Message {
	attributes := []tg.DocumentAttributeClass{}
	if fileName != "" {
		attributes = append(attributes, &tg.DocumentAttributeFilename{FileName: fileName})
	}
	return &tg.Message{
		ID:   id,
		Date: 1_700_000_000,
		Media: &tg.MessageMediaDocument{Document: &tg.Document{
			ID:            int64(id) * 100,
			AccessHash:    int64(id) + 1,
			FileReference: []byte{byte(id), 8, 9},
			Date:          1_700_000_000,
			MimeType:      "application/octet-stream",
			Size:          3,
			Attributes:    attributes,
		}},
	}
}

func TestMessageFromUpdates(t *testing.T) {
	message := testDocumentMessage(42, "file.bin")
	updates := &tg.Updates{Updates: []tg.UpdateClass{
		&tg.UpdateNewChannelMessage{Message: message},
	}}
	got, id := messageFromUpdates(updates)
	if got != message || id != 42 {
		t.Fatalf("messageFromUpdates = (%p, %d), want (%p, 42)", got, id, message)
	}

	short, id := messageFromUpdates(&tg.UpdateShortSentMessage{ID: 43})
	if short != nil || id != 43 {
		t.Fatalf("short message = (%v, %d), want (nil, 43)", short, id)
	}
}

func TestRangeWriter(t *testing.T) {
	var output bytes.Buffer
	writer := &rangeWriter{writer: &output, skip: 3, remaining: 4}
	for _, chunk := range []string{"abcde", "fghij"} {
		if n, err := writer.Write([]byte(chunk)); err != nil || n != len(chunk) {
			t.Fatalf("Write(%q) = (%d, %v), want (%d, nil)", chunk, n, err, len(chunk))
		}
	}
	if got, want := output.String(), "defg"; got != want {
		t.Fatalf("range output = %q, want %q", got, want)
	}
}
