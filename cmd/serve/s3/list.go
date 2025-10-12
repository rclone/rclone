package s3

import (
	"path"
	"strings"

	"github.com/rclone/gofakes3"
	"github.com/rclone/rclone/vfs"
)

func (b *s3Backend) entryListR(_vfs *vfs.VFS, bucket, fdPath, name string, addPrefix bool, response *gofakes3.ObjectList) error {
	fp := path.Join(bucket, fdPath)

	dirEntries, err := getDirEntries(fp, _vfs)
	if err != nil {
		return err
	}

	for _, entry := range dirEntries {
		object := entry.Name()

		// workaround for control-chars detect
		objectPath := path.Join(fdPath, object)

		if !strings.HasPrefix(object, name) {
			continue
		}

		if entry.IsDir() {
			if addPrefix {
				prefixWithTrailingSlash := objectPath + "/"
				response.AddPrefix(prefixWithTrailingSlash)
				continue
			}
			err := b.entryListR(_vfs, bucket, path.Join(fdPath, object), "", false, response)
			if err != nil {
				return err
			}
		} else {
			item := &gofakes3.Content{
				Key:          objectPath,
				LastModified: gofakes3.NewContentTime(entry.ModTime()),
				ETag:         getFileHash(entry, b.s.etagHashType),
				Size:         entry.Size(),
				StorageClass: gofakes3.StorageStandard,
			}
			response.Add(item)
		}
	}
	return nil
}
