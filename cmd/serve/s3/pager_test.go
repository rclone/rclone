package s3

import (
	"testing"
	"time"

	"github.com/rclone/gofakes3"
)

func TestPagerSortsContentsByKey(t *testing.T) {
	list := gofakes3.NewObjectList()
	list.Add(&gofakes3.Content{
		Key:          "b.txt",
		LastModified: gofakes3.NewContentTime(time.Unix(100, 0)),
	})
	list.Add(&gofakes3.Content{
		Key:          "a.txt",
		LastModified: gofakes3.NewContentTime(time.Unix(200, 0)),
	})

	got, err := (&s3Backend{}).pager(list, gofakes3.ListBucketPage{MaxKeys: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Contents) != 2 {
		t.Fatalf("expected 2 contents, got %d", len(got.Contents))
	}

	if got.Contents[0].Key != "a.txt" || got.Contents[1].Key != "b.txt" {
		t.Fatalf("expected lexicographic key order [a.txt b.txt], got [%s %s]", got.Contents[0].Key, got.Contents[1].Key)
	}
}
