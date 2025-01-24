package namecrane

import (
	"fmt"
	"io"
	"os"
	"testing"
)

func TestClient(t *testing.T) {
	fsDirPath := "../../Dockerfile"

	fd, err := os.Open(fsDirPath)
	if err != nil {
		err = fmt.Errorf("failed to open directory %q: %w", fsDirPath, err)
		t.Fatal(err)
	}

	for {
		var fis []os.FileInfo
		// Windows and Plan9 read the directory entries with the stat information in which
		// shouldn't fail because of unreadable entries.
		fis, err = fd.Readdir(1024)
		if err == io.EOF && len(fis) == 0 {
			break
		}

		if err != nil {
			t.Fatal("Unable to read directory", err)
		}

		for _, fi := range fis {
			t.Log("Found", fi.Name())
		}
	}
}
