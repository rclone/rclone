package mediavfs

import (
	"context"
	"fmt"
	"testing"
)

func TestSync(t *testing.T) {
	ctx := context.Background()

	// Create sync manager
	sm := &SyncManager{
		user: "test@example.com",
		dbPath: "/tmp/test-gphoto.db",
		tableName: "remote_media",
	}

	// Initialize database
	if err := sm.InitDatabase(ctx); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	// Create API client
	api := NewGPhotoAPI("test@example.com")

	// Run sync
	fmt.Println("\n=== Starting sync test ===")
	if err := sm.Sync(ctx, api); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	fmt.Println("\n=== Sync completed ===")
}
