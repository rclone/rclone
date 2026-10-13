package telegram

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fstest"
)

func TestIntegration(t *testing.T) {
	ctx := context.Background()
	fstest.Initialise()
	remoteName := *fstest.RemoteName
	if remoteName == "" {
		remoteName = "TestTelegram:"
	}
	remote, err := fs.NewFs(ctx, remoteName)
	if errors.Is(err, fs.ErrorNotFoundInConfigFile) {
		t.Skipf("Telegram test remote %q is not configured", remoteName)
	}
	if err != nil {
		t.Fatalf("create Telegram test remote: %v", err)
	}
	f, ok := remote.(*Fs)
	if !ok {
		t.Fatalf("expected *telegram.Fs, got %T", remote)
	}

	name := fmt.Sprintf("rclone-telegram-test-%d.txt", time.Now().UnixNano())
	defer func() {
		if object, err := f.NewObject(ctx, name); err == nil {
			if err := object.Remove(ctx); err != nil {
				t.Errorf("remove test object: %v", err)
			}
		}
		if err := f.Shutdown(ctx); err != nil {
			t.Errorf("shutdown Telegram client: %v", err)
		}
	}()

	first := []byte("first Telegram upload")
	firstInfo := object.NewStaticObjectInfo(name, time.Now(), int64(len(first)), true, nil, f)
	created, err := f.Put(ctx, bytes.NewReader(first), firstInfo)
	if err != nil {
		t.Fatalf("initial Put: %v", err)
	}
	if created.Remote() != name {
		t.Fatalf("initial Put remote = %q, want %q", created.Remote(), name)
	}

	second := []byte("replacement Telegram upload")
	secondInfo := object.NewStaticObjectInfo(name, time.Now(), int64(len(second)), true, nil, f)
	updated, err := f.Put(ctx, bytes.NewReader(second), secondInfo)
	if err != nil {
		t.Fatalf("replacement Put: %v", err)
	}
	if updated.Remote() != name {
		t.Fatalf("replacement Put remote = %q, want %q", updated.Remote(), name)
	}

	messages, err := f.listMessages(ctx)
	if err != nil {
		t.Fatalf("list Telegram messages: %v", err)
	}
	matchingMessages := 0
	for _, message := range messages {
		if object, ok := f.objectFromMessage(message); ok && object.remote == name {
			matchingMessages++
		}
	}
	if matchingMessages != 1 {
		t.Fatalf("got %d Telegram messages for %q after replacement, want 1", matchingMessages, name)
	}

	stored, err := f.NewObject(ctx, name)
	if err != nil {
		t.Fatalf("find replaced object: %v", err)
	}
	reader, err := stored.Open(ctx)
	if err != nil {
		t.Fatalf("open replaced object: %v", err)
	}
	contents, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil {
		t.Fatalf("read replaced object: %v", readErr)
	}
	if closeErr != nil {
		t.Fatalf("close replaced object: %v", closeErr)
	}
	if !bytes.Equal(contents, second) {
		t.Fatalf("downloaded content = %q, want %q", contents, second)
	}
}
