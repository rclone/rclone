package s3

import (
	"context"
	"path"
	"strings"

	"github.com/Mikubill/gofakes3"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/walk"
)

func (b *s3Backend) entryListR(bucket, fdPath, name string, acceptComPrefix bool, response *gofakes3.ObjectList) error {
	fp := path.Join(bucket, fdPath)

	dirEntries, err := getDirEntries(fp, b.vfs)
	if err != nil {
		return err
	}

	for _, entry := range dirEntries {
		object := entry.Name()

		// workround for control-chars detect
		objectPath := path.Join(fdPath, object)

		if !strings.HasPrefix(object, name) {
			continue
		}

		if entry.IsDir() {
			if acceptComPrefix {
				response.AddPrefix(gofakes3.URLEncode(objectPath))
				continue
			}
			err := b.entryListR(bucket, path.Join(fdPath, object), "", false, response)
			if err != nil {
				return err
			}
		} else {
			item := &gofakes3.Content{
				Key:          gofakes3.URLEncode(objectPath),
				LastModified: gofakes3.NewContentTime(entry.ModTime()),
				ETag:         getFileHash(entry),
				Size:         entry.Size(),
				StorageClass: gofakes3.StorageStandard,
			}
			response.Add(item)
		}
	}
	return nil
}

// getObjectsList lists the objects in the given bucket.
func (b *s3Backend) getObjectsListArbitrary(bucket string, prefix *gofakes3.Prefix, response *gofakes3.ObjectList) error {
	// ignore error - vfs may have uncommitted updates, such as new dir etc.
	_ = walk.ListR(context.Background(), b.vfs.Fs(), bucket, false, -1, walk.ListObjects, func(entries fs.DirEntries) error {
		for _, entry := range entries {
			entry := entry.(fs.Object)
			objName := entry.Remote()
			object := strings.TrimPrefix(objName, bucket)[1:]

			var matchResult gofakes3.PrefixMatch
			if prefix.Match(object, &matchResult) {
				if matchResult.CommonPrefix {
					response.AddPrefix(gofakes3.URLEncode(object))
					continue
				}

				item := &gofakes3.Content{
					Key:          gofakes3.URLEncode(object),
					LastModified: gofakes3.NewContentTime(entry.ModTime(context.Background())),
					ETag:         getFileHash(entry),
					Size:         entry.Size(),
					StorageClass: gofakes3.StorageStandard,
				}
				response.Add(item)
			}
		}

		return nil
	})

	return nil
}
