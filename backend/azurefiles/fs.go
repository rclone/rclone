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

const sleepDurationBetweenRecursiveMkdirPutCalls = time.Millisecond * 500
const fourTbInBytes = 4398046511104

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
//
// Does not return ErrorIsDir when a directory exists instead of file. since the documentation
// for [rclone.fs.Fs.NewObject] rqeuires no extra work to determine whether it is directory
//
// Inspired by azureblob store, this initiates a network request and returns an error if object is not found.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	fileClient := f.fileClient(remote)
	resp, err := fileClient.GetProperties(ctx, nil)
	if fileerror.HasCode(err, fileerror.ParentNotFound, fileerror.ResourceNotFound) {
		return nil, fs.ErrorObjectNotFound
	} else if err != nil {
		return nil, fmt.Errorf("unable to find object remote=%s : %w", remote, err)
	}

	ob := objectInstance(f, remote, *resp.ContentLength, resp.ContentMD5, *resp.FileLastWriteTime)
	return &ob, nil
}

// Mkdir creates nested directories as indicated by test FsMkdirRmdirSubdir
// TODO: write custom test case where parent directories are created
// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, remote string) error {
	return f.mkdirRelativeToRootOfShare(ctx, f.decodedFullPath(remote))
}

// rclone completes commands such as rclone copy localdir remote:parentcontainer/childcontainer
// where localdir is a tree of files and directories. The above command is expected to complete even
// when parentcontainer and childcontainer directors do not exist on the remote. The following
// code with emphasis on fullPathRelativeToShareRoot is written to handle such cases by recursiely creating
// parent directories all the way to the root of the share
//
// When path argument is an empty string, windows and linux return and error. However, this
// implementation does not return an error
func (f *Fs) mkdirRelativeToRootOfShare(ctx context.Context, fullPathRelativeToShareRoot string) error {
	fp := fullPathRelativeToShareRoot
	if fp == "" {
		return nil
	}
	dirClient := f.newSubdirectoryClientFromEncodedPathRelativeToShareRoot(f.encodePath(fp))
	// now := time.Now()
	// smbProps := &file.SMBProperties{
	// 	LastWriteTime: &now,
	// }
	// dirCreateOptions := &directory.CreateOptions{
	// 	FileSMBProperties: smbProps,
	// }

	_, createDirErr := dirClient.Create(ctx, nil)
	if fileerror.HasCode(createDirErr, fileerror.ParentNotFound) {
		parentDir := path.Dir(fp)
		if parentDir == fp {
			log.Fatal("This will lead to infinite recursion since parent and remote are equal")
		}
		makeParentErr := f.mkdirRelativeToRootOfShare(ctx, parentDir)
		if makeParentErr != nil {
			return fmt.Errorf("could not make parent of %s : %w", fp, makeParentErr)
		}
		log.Printf("Mkdir: waiting for %s after making parent=%s", sleepDurationBetweenRecursiveMkdirPutCalls.String(), parentDir)
		time.Sleep(sleepDurationBetweenRecursiveMkdirPutCalls)
		return f.mkdirRelativeToRootOfShare(ctx, fp)
	} else if fileerror.HasCode(createDirErr, fileerror.ResourceAlreadyExists) {
		return nil
	} else if createDirErr != nil {
		return fmt.Errorf("unable to MkDir: %w", createDirErr)
	}
	return nil
}

// Rmdir deletes the root folder
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, remote string) error {
	dirClient := f.dirClient(remote)
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

// Put the object
//
// Copies the reader in to the new object. This new object is returned.
//
// The new object may have been created if an error is returned
// TODO: when file.CLient.Creat is being used, provide HTTP headesr such as content type and content MD5
// TODO: maybe replace PUT with NewObject + Update
// TODO: in case file is created but there is a problem on upload, what happens
// TODO: what happens when file already exists at the location
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	if src.Size() > fourTbInBytes {
		return nil, fmt.Errorf("max supported file size is 4TB. provided size is %d", src.Size())
	} else if src.Size() < 0 {
		// TODO: what should happened when src.Size == 0
		return nil, fmt.Errorf("src.Size is a required to be a whole number : %d", src.Size())
	}
	fc := f.fileClient(src.Remote())

	_, createErr := fc.Create(ctx, src.Size(), nil)
	if fileerror.HasCode(createErr, fileerror.ParentNotFound) {
		parentDir := path.Dir(src.Remote())
		if mkDirErr := f.Mkdir(ctx, parentDir); mkDirErr != nil {
			return nil, fmt.Errorf("unable to make parent directories : %w", mkDirErr)
		}
		log.Printf("Mkdir: waiting for %s after making parent=%s", sleepDurationBetweenRecursiveMkdirPutCalls.String(), parentDir)
		time.Sleep(sleepDurationBetweenRecursiveMkdirPutCalls)
		return f.Put(ctx, in, src, options...)
	} else if createErr != nil {
		return nil, fmt.Errorf("unable to create file : %w", createErr)
	}

	obj := &Object{
		common: common{
			f:      f,
			remote: src.Remote(),
		},
	}
	if updateErr := obj.upload(ctx, in, src, true, options...); updateErr != nil {
		err := fmt.Errorf("while executing update after creating file as part of fs.Put : %w", updateErr)
		if _, delErr := fc.Delete(ctx, nil); delErr != nil {
			return nil, errors.Join(delErr, updateErr)
		}
		return obj, err
	}

	return obj, nil
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String converts this Fs to a string
func (f *Fs) String() string {
	return fmt.Sprintf("azurefiles root '%s'", f.root)
}

// Precision return the precision of this Fs
//
// One second. FileREST API times are in RFC1123 which in the example shows a precision of seconds
// Source: https://learn.microsoft.com/en-us/rest/api/storageservices/representation-of-date-time-values-in-headers
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// Hashes returns the supported hash sets.
//
// MD5: since it is listed as header in the response for file properties
// Source: https://learn.microsoft.com/en-us/rest/api/storageservices/get-file-properties
func (f *Fs) Hashes() hash.Set {
	return hash.NewHashSet(hash.MD5)
}

// Features returns the optional features of this Fs
//
// TODO: add features:- public link, SlowModTime, SlowHash,
// ReadMetadata, WriteMetadata,UserMetadata,PutUnchecked, PutStream
// PartialUploads: Maybe????
// FileID and DirectoryID can be implemented. They are atleast returned as part of listing response
func (f *Fs) Features() *fs.Features {
	return &fs.Features{
		CanHaveEmptyDirectories: true,
		// Copy: func(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
		// 	return f.CopyFile(ctx, src, remote)
		// },
	}
}

// List the objects and directories in dir into entries. The entries can be
// returned in any order but should be for a complete directory.
//
// dir should be "" to list the root, and should not have trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't found.
//
// TODO: handle case regariding "" and "/". I remember reading about them somewhere
func (f *Fs) List(ctx context.Context, remote string) (fs.DirEntries, error) {
	var entries fs.DirEntries
	subDirClient := f.dirClient(remote)

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
						lastWriteTime: *dir.Properties.LastWriteTime,
					}},
			}
			entries = append(entries, de)
		}

		for _, file := range resp.Segment.Files {
			de := &Object{
				common{f: f,
					remote: path.Join(remote, f.decodePath(*file.Name)),
					properties: properties{
						contentLength: *file.Properties.ContentLength,
						lastWriteTime: *file.Properties.LastWriteTime,
					}},
			}
			entries = append(entries, de)
		}
	}

	return entries, nil

}

type encodedPath string

func (f *Fs) decodedFullPath(decodedRemote string) string {
	return path.Join(f.root, decodedRemote)
}

func (f *Fs) dirClient(decodedRemote string) *directory.Client {
	fullPathDecoded := f.decodedFullPath(decodedRemote)
	fullPathEncoded := f.encodePath(fullPathDecoded)
	return f.newSubdirectoryClientFromEncodedPathRelativeToShareRoot(fullPathEncoded)
}

func (f *Fs) newSubdirectoryClientFromEncodedPathRelativeToShareRoot(p encodedPath) *directory.Client {
	return f.shareRootDirClient.NewSubdirectoryClient(string(p))
}

func (f *Fs) fileClient(decodedRemote string) *file.Client {
	fullPathDecoded := f.decodedFullPath(decodedRemote)
	fullPathEncoded := f.encodePath(fullPathDecoded)
	return f.fileClientFromEncodedPathRelativeToShareRoot(fullPathEncoded)
}

func (f *Fs) fileClientFromEncodedPathRelativeToShareRoot(p encodedPath) *file.Client {
	return f.shareRootDirClient.NewFileClient(string(p))
}

func (f *Fs) encodePath(p string) encodedPath {
	return encodedPath(f.opt.Enc.FromStandardPath(p))
}

func (f *Fs) decodePath(p string) string {
	return f.opt.Enc.ToStandardPath(p)
}

// on 20231019 at 1324 work to be continued at trying to fix  FAIL: TestIntegration/FsMkdir/FsPutFiles/FromRoot
