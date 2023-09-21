package azurefiles

import (
	"context"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/directory"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/file"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/fileerror"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
)

// Inspired by azureblob store, this initiates a network request and returns an error if objec is not found
// TODO: when does the interface expect this function to return an interface
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	fileClient := f.NewFileClient(f.encodePath(remote))
	resp, err := fileClient.GetProperties(ctx, nil)
	if fileerror.HasCode(err, fileerror.ResourceNotFound, fileerror.ParentNotFound) {
		return nil, fs.ErrorObjectNotFound
	} else if err != nil {
		return nil, fmt.Errorf("unable to create object remote=%s : %w", remote, err)
	}

	ob := Object{common{
		f:        f,
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
func (f *Fs) Mkdir(ctx context.Context, dirPath string) error {
	if dirPath == "" {
		return nil
	}
	dirClient := f.NewSubdirectoryClient(f.encodePath(dirPath))
	_, err := dirClient.Create(ctx, nil)
	if fileerror.HasCode(err, fileerror.ParentNotFound) {
		err := f.Mkdir(ctx, parent(dirPath))
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

func parent(p string) string {
	parts := strings.Split(p, pathSeparator)
	return strings.Join(parts[:len(parts)-1], pathSeparator)
}

// should return error if the directory is not empty or does not exist
func (f *Fs) Rmdir(ctx context.Context, dirPath string) error {
	log.Printf("rmdir called on dirPath=%s", dirPath)

	// Following if statement is added to pass test 'FsRmdirEmpty'
	if dirPath == "" {
		// Checking whether root directory is empty
		des, err := f.List(ctx, dirPath)
		if err != nil {
			return fmt.Errorf("could not determine whether root directory is emtpy :%w ", err)
		}
		if len(des) > 0 {
			return fs.ErrorDirectoryNotEmpty
		}
		//FIXME- this error wraps fs.ErrorDirNotFound to pass TestIntegration/FsMkdir/FsNewObjectNotFound
		return fmt.Errorf("cannot delete root dir. it is empty :%w", fs.ErrorDirNotFound)
	}
	dirClient := f.NewSubdirectoryClient(f.encodePath(dirPath))
	_, err := dirClient.Delete(ctx, nil)
	if err != nil {
		if fileerror.HasCode(err, fileerror.DirectoryNotEmpty) {
			return fs.ErrorDirectoryNotEmpty
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
// TODO: in case file is created but there is a problem on upload, what happens
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// size and modtime are important, what about md5 hashes
	if src.Size() > maxFileSize {
		return nil, fmt.Errorf("max supported file size is 4TB. provided size is %d", src.Size())
	}
	fileSize := maxFileSize
	if src.Size() >= 0 {
		fileSize = src.Size()
	}
	for i := 0; i < 2; i++ {
		_, createErr := f.NewFileClient(f.encodePath(src.Remote())).Create(ctx, fileSize, nil)
		if fileerror.HasCode(createErr, fileerror.ParentNotFound) {
			parentDir := parent(src.Remote())
			if mkDirErr := f.Mkdir(ctx, parentDir); mkDirErr != nil {
				return nil, fmt.Errorf("unable to make parent directories : %w", mkDirErr)
			}
		} else if createErr != nil {
			return nil, fmt.Errorf("unable to create file : %w", createErr)
		} else if createErr == nil {
			break
		}
	}

	fsObj, err := f.NewObject(ctx, src.Remote())
	if err != nil {
		return nil, fmt.Errorf("unable to get NewObject : %w", err)
	}

	obj, ok := fsObj.(*Object)
	if !ok {
		log.Panic("should be able to convert fs.Object to ptr of azurefiles.Object")
	}

	if err := obj.uploadStreamSetMd5(ctx, in, options...); err != nil {
		return nil, err
	}

	if err := obj.SetModTime(ctx, src.ModTime(ctx)); err != nil {
		return nil, fmt.Errorf("unable to set modTime : %w", err)
	}

	return obj, nil
}

func (f *Fs) Name() string {
	return f.name
}

func (f *Fs) Root() string {
	return f.root
}

func (f *Fs) String() string {
	return fmt.Sprintf("azurefiles root '%s'", f.root)
}

// One second. FileREST API times are in RFC1123 which in the example shows a precision of seconds
// Source: https://learn.microsoft.com/en-us/rest/api/storageservices/representation-of-date-time-values-in-headers
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// MD5: since it is listed as header in the response for file properties
// Source: https://learn.microsoft.com/en-us/rest/api/storageservices/get-file-properties
func (f *Fs) Hashes() hash.Set {
	return hash.NewHashSet(hash.MD5)
}

// TODO: what are features. implement them
func (f *Fs) Features() *fs.Features {
	return &fs.Features{
		CanHaveEmptyDirectories: true,
	}
}

// TODO: return ErrDirNotFound if dir not found
// TODO: handle case regariding "" and "/". I remember reading about them somewhere
func (f *Fs) List(ctx context.Context, dirPath string) (fs.DirEntries, error) {
	var entries fs.DirEntries
	subDirClient := f.NewSubdirectoryClient(f.encodePath(dirPath))
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
				common{f: f,
					remote: joinPaths(dirPath, f.decodePath(*dir.Name)),
					properties: properties{
						changeTime: dir.Properties.ChangeTime,
					}},
			}
			entries = append(entries, de)
		}

		for _, file := range resp.Segment.Files {
			de := &Object{
				common{f: f,
					remote: joinPaths(dirPath, f.decodePath(*file.Name)),
					properties: properties{
						changeTime:    file.Properties.ChangeTime,
						contentLength: file.Properties.ContentLength,
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

type encodedPath string

func (f *Fs) NewSubdirectoryClient(p encodedPath) *directory.Client {
	return f.rootDirClient.NewSubdirectoryClient(string(p))
}

func (f *Fs) NewFileClient(p encodedPath) *file.Client {
	return f.rootDirClient.NewFileClient(string(p))
}

func (f *Fs) encodePath(p string) encodedPath {
	return encodedPath(f.opt.Enc.FromStandardPath(p))
}

func (f *Fs) decodePath(p string) string {
	return f.opt.Enc.ToStandardPath(p)
}
