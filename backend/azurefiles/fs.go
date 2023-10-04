package azurefiles

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"path"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/directory"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/file"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/fileerror"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
)

// Inspired by azureblob store, this initiates a network request and returns an error if object is not found
// Returns ErrorIsDir when a directory exists instead of file.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	fileClient := f.NewFileClient(remote)
	resp, err := fileClient.GetProperties(ctx, nil)
	if fileerror.HasCode(err, fileerror.ParentNotFound) {
		return nil, fs.ErrorObjectNotFound
	} else if fileerror.HasCode(err, fileerror.ResourceNotFound) {
		if isDir, _ := f.isDirectory(ctx, remote); isDir {
			return nil, fs.ErrorIsDir
		}
		return nil, fs.ErrorObjectNotFound
	} else if err != nil {
		return nil, fmt.Errorf("unable to find object remote=%s : %w", remote, err)
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

// Checks whether path is a directory
// Only confirms whether a path is a directory. A false result does not mean
// that the remote is a file.
func (f *Fs) isDirectory(ctx context.Context, remote string) (bool, error) {
	dirClient := f.NewSubdirectoryClient(remote)
	_, err := dirClient.GetProperties(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("isDirectory remote=%s : %w", remote, err)
	}
	return true, nil
}

// Mkdir creates nested directories as indicated by test FsMkdirRmdirSubdir
// TODO: write custom test case where parent directories are created
func (f *Fs) Mkdir(ctx context.Context, remote string) error {
	fp := f.DecodedFullPath(remote)
	if fp == "" {
		return nil
	}
	dirClient := f.NewSubdirectoryClient(remote)

	_, createDirErr := dirClient.Create(ctx, nil)
	if fileerror.HasCode(createDirErr, fileerror.ParentNotFound) {
		parentDir := path.Dir(remote)
		makeParentErr := f.Mkdir(ctx, parentDir)
		if makeParentErr != nil {
			return fmt.Errorf("could not make parent of %s : %w", remote, makeParentErr)
		}
		log.Printf("Mkdir: waiting for half a second after making parent=%s", parentDir)
		time.Sleep(time.Millisecond * 500)
		return f.Mkdir(ctx, remote)
	} else if fileerror.HasCode(createDirErr, fileerror.ResourceAlreadyExists) {
		return nil
	} else if createDirErr != nil {
		return fmt.Errorf("unable to MkDir: %w", createDirErr)
	}
	return nil
}

// should return error if the directory is not empty or does not exist
func (f *Fs) Rmdir(ctx context.Context, remote string) error {
	// Following if statement is added to pass test 'FsRmdirEmpty'
	// if f.DecodedFullPath(remote) == "" {
	// 	// Checking whether root directory is empty
	// 	des, err := f.List(ctx, remote)
	// 	if err != nil {
	// 		return fmt.Errorf("could not determine whether root directory is emtpy :%w ", err)
	// 	}
	// 	if len(des) > 0 {
	// 		return fs.ErrorDirectoryNotEmpty
	// 	}
	// 	//FIXME- this error wraps fs.ErrorDirNotFound to pass TestIntegration/FsMkdir/FsNewObjectNotFound
	// 	return fmt.Errorf("cannot delete root dir. it is empty :%w", fs.ErrorDirNotFound)
	// }
	dirClient := f.NewSubdirectoryClient(remote)
	_, err := dirClient.Delete(ctx, nil)
	if err != nil {
		if fileerror.HasCode(err, fileerror.DirectoryNotEmpty) {
			return fs.ErrorDirectoryNotEmpty
		} else if fileerror.HasCode(err, fileerror.ResourceNotFound) {
			return fs.ErrorDirNotFound
		}
		return fmt.Errorf("could not rmdir dir=\"%s\" : %w", remote, err)
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
	fc := f.NewFileClient(src.Remote())
	parentDir := path.Dir(src.Remote())
	_, createErr := fc.Create(ctx, fileSize, nil)
	if fileerror.HasCode(createErr, fileerror.ParentNotFound) {

		if mkDirErr := f.Mkdir(ctx, parentDir); mkDirErr != nil {
			return nil, fmt.Errorf("unable to make parent directories : %w", mkDirErr)
		}
		log.Printf("Put: waiting for half a second after making parent=%s", parentDir)
		time.Sleep(time.Millisecond * 500)
		return f.Put(ctx, in, src, options...)
	} else if createErr != nil {
		return nil, fmt.Errorf("unable to create file : %w", createErr)
	}

	if err := uploadStreamSetMd5(ctx, fc, in, options...); err != nil {
		if _, delErr := fc.Delete(ctx, nil); delErr != nil {
			return nil, errors.Join(delErr, err)
		}
		return nil, err
	}

	fsObj, err := f.NewObject(ctx, src.Remote())
	if err != nil {
		return nil, fmt.Errorf("unable to get NewObject : %w", err)
	}
	obj := fsObj.(*Object)

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

// TODO: handle case regariding "" and "/". I remember reading about them somewhere
func (f *Fs) List(ctx context.Context, remote string) (fs.DirEntries, error) {
	var entries fs.DirEntries
	subDirClient := f.NewSubdirectoryClient(remote)

	// Checking whether directory exists
	_, err := subDirClient.GetProperties(ctx, nil)
	if fileerror.HasCode(err, fileerror.ParentNotFound, fileerror.ResourceNotFound) {
		return entries, fs.ErrorDirNotFound
	} else if err != nil {
		return entries, err
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
					remote: path.Join(remote, f.decodePath(*dir.Name)),
					properties: properties{
						changeTime: dir.Properties.ChangeTime,
					}},
			}
			entries = append(entries, de)
		}

		for _, file := range resp.Segment.Files {
			de := &Object{
				common{f: f,
					remote: path.Join(remote, f.decodePath(*file.Name)),
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

type encodedPath string

func (f *Fs) DecodedFullPath(decodedRemote string) string {
	return path.Join(f.root, decodedRemote)
}

func (f *Fs) NewSubdirectoryClient(decodedRemote string) *directory.Client {
	fullPathDecoded := f.DecodedFullPath(decodedRemote)
	fullPathEncoded := f.encodePath(fullPathDecoded)
	return f.NewSubdirectoryClientFromEncodedPath(fullPathEncoded)
}

func (f *Fs) NewSubdirectoryClientFromEncodedPath(p encodedPath) *directory.Client {
	return f.shareRootDirClient.NewSubdirectoryClient(string(p))
}

func (f *Fs) NewFileClient(decodedRemote string) *file.Client {
	fullPathDecoded := f.DecodedFullPath(decodedRemote)
	fullPathEncoded := f.encodePath(fullPathDecoded)
	return f.NewFileClientFromEncodedPath(fullPathEncoded)
}

func (f *Fs) NewFileClientFromEncodedPath(p encodedPath) *file.Client {
	return f.shareRootDirClient.NewFileClient(string(p))
}

func (f *Fs) encodePath(p string) encodedPath {
	return encodedPath(f.opt.Enc.FromStandardPath(p))
}

func (f *Fs) decodePath(p string) string {
	return f.opt.Enc.ToStandardPath(p)
}
