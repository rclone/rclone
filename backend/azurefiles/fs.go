package azurefiles

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/fileerror"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
)

// Inspired by azureblob store, this initiates a network request and returns an error if objec is not found
// TODO: when does the interface expect this function to return an interface
func (c *Client) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	fileClient := c.RootDirClient.NewFileClient(remote)
	resp, err := fileClient.GetProperties(ctx, nil)
	if fileerror.HasCode(err, fileerror.ResourceNotFound) {
		return nil, fs.ErrorObjectNotFound
	}

	ob := Object{common{
		c:        c,
		remote:   remote,
		metaData: resp.Metadata,
		properties: properties{
			changeTime:    resp.FileChangeTime,
			contentLength: resp.ContentLength,
		}},
	}
	return &ob, nil
}

// Mkdir creates nested directories as indicated by test FsMkdirRmdirSubdir
// TODO: write custom test case where parent directories are created
func (dc *Client) Mkdir(ctx context.Context, dirPath string) error {
	return dc.makeDir(ctx, dirPath)
}

func parent(p string) string {
	parts := strings.Split(p, pathSeparator)
	return strings.Join(parts[:len(parts)-1], pathSeparator)
}

func (dc *Client) makeDir(ctx context.Context, dirPath string) error {
	if dirPath == "" {
		return nil
	}
	dirClient := dc.RootDirClient.NewSubdirectoryClient(dirPath)
	_, err := dirClient.Create(ctx, nil)
	if fileerror.HasCode(err, fileerror.ParentNotFound) {
		err := dc.makeDir(ctx, parent(dirPath))
		if err != nil {
			return fmt.Errorf("could not make parent of %s : %w", dirPath, err)
		}
		_, err = dirClient.Create(ctx, nil)
		if err != nil {
			return fmt.Errorf("made parent but cannot make %s : %w", dirPath, err)
		}
	} else if fileerror.HasCode(err, fileerror.ResourceAlreadyExists) {
		return nil
	} else if err != nil {
		return fmt.Errorf("unable to MkDir: %w", err)
	}
	return nil
}

// should return error if the directory is not empty or does not exist
func (c *Client) Rmdir(ctx context.Context, dirPath string) error {

	// Following if statement is added to pass test 'FsRmdirEmpty'
	if dirPath == "" {
		return nil
	}
	dirClient := c.RootDirClient.NewSubdirectoryClient(dirPath)
	_, err := dirClient.Delete(ctx, nil)
	if err != nil {
		if fileerror.HasCode(err, fileerror.DirectoryNotEmpty) {
			return fmt.Errorf("cannot rmdir dir=\"%s\" is not empty : %w", dirPath, err)
		} else if fileerror.HasCode(err, fileerror.ResourceNotFound) {
			return fmt.Errorf("cannot rmdir dir=\"%s\" not found : %w", dirPath, err)
		}
		return fmt.Errorf("could not rmdir dir=\"%s\" : %w", dirPath, err)
	}
	return nil

}

// Copied mostly from local filesystem
// TODO: file system options
// TODO: when file.CLient.Creat is being used, provide HTTP headesr such as content type and content MD5
// TODO: maybe replace PUT with NewObject + Update
func (c *Client) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// size and modtime are important, what about md5 hashes
	fileSize := maxFileSize
	if src.Size() >= 0 {
		fileSize = src.Size()
	}

	fc := c.RootDirClient.NewFileClient(src.Remote())

	_, err := fc.Create(ctx, fileSize, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create file : %w", err)
	}
	if err := fc.UploadStream(ctx, in, nil); err != nil {
		return nil, fmt.Errorf("azurefiles: error uploading as stream : %w", err)
	}

	obj, err := c.NewObject(ctx, src.Remote())
	if err != nil {
		return nil, fmt.Errorf("uanble to get NewObject so that modTime can be set : %w", err)
	}
	if err := obj.SetModTime(ctx, src.ModTime(ctx)); err != nil {
		return nil, fmt.Errorf("uanble to get modTime : %w", err)
	}

	return obj, nil
}

func (c *Client) Name() string {
	return c.name
}

func (c *Client) Root() string {
	return c.root
}

func (c *Client) String() string {
	return fmt.Sprintf("azurefiles root '%s'", c.root)
}

// One second. FileREST API times are in RFC1123 which in the example shows a precision of seconds
// Source: https://learn.microsoft.com/en-us/rest/api/storageservices/representation-of-date-time-values-in-headers
func (c *Client) Precision() time.Duration {
	return time.Second
}

// MD5: since it is listed as header in the response for file properties
// Source: https://learn.microsoft.com/en-us/rest/api/storageservices/get-file-properties
func (c *Client) Hashes() hash.Set {
	return hash.NewHashSet(hash.MD5)
}

// TODO: what are features. implement them
func (c *Client) Features() *fs.Features {
	return &fs.Features{
		CanHaveEmptyDirectories: true,
	}
}

// TODO: return ErrDirNotFound if dir not found
// TODO: handle case regariding "" and "/". I remember reading about them somewhere
func (dc *Client) List(ctx context.Context, dirPath string) (fs.DirEntries, error) {
	var entries fs.DirEntries
	subDirClient := dc.RootDirClient.NewSubdirectoryClient(dirPath)
	_, err := subDirClient.GetProperties(ctx, nil)
	if err != nil {
		return fs.DirEntries(entries), fs.ErrorDirNotFound
	}
	pager := subDirClient.NewListFilesAndDirectoriesPager(listFilesAndDirectoriesOptions)
	for pager.More() {
		resp, err := pager.NextPage(ctx)
		if err != nil {
			return entries, err
		}
		for _, dir := range resp.Segment.Directories {
			de := &Directory{
				common{c: dc,
					remote: joinPaths(dirPath, *dir.Name),
					properties: properties{
						changeTime: dir.Properties.ChangeTime,
					}},
			}
			entries = append(entries, de)
		}

		for _, f := range resp.Segment.Files {
			de := &Object{
				common{c: dc,
					remote: joinPaths(dirPath, *f.Name),
					properties: properties{
						changeTime:    f.Properties.ChangeTime,
						contentLength: f.Properties.ContentLength,
					}},
			}
			entries = append(entries, de)
		}
	}

	return entries, nil

}

func joinPaths(s ...string) string {
	return filepath.ToSlash(filepath.Join(s...))
}
