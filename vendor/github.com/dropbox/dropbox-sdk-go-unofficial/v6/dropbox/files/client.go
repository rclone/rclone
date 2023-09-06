// Copyright (c) Dropbox, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package files

import (
	"encoding/json"
	"io"
	"log"

	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox/async"
	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox/auth"
	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox/file_properties"
)

// Client interface describes all routes in this namespace
type Client interface {
	// AlphaGetMetadata : Returns the metadata for a file or folder. This is an
	// alpha endpoint compatible with the properties API. Note: Metadata for the
	// root folder is unsupported.
	// Deprecated: Use `GetMetadata` instead
	AlphaGetMetadata(arg *AlphaGetMetadataArg) (res IsMetadata, err error)
	// AlphaUpload : Create a new file with the contents provided in the
	// request. Note that the behavior of this alpha endpoint is unstable and
	// subject to change. Do not use this to upload a file larger than 150 MB.
	// Instead, create an upload session with `uploadSessionStart`.
	// Deprecated: Use `Upload` instead
	AlphaUpload(arg *UploadArg, content io.Reader) (res *FileMetadata, err error)
	// Copy : Copy a file or folder to a different location in the user's
	// Dropbox. If the source path is a folder all its contents will be copied.
	CopyV2(arg *RelocationArg) (res *RelocationResult, err error)
	// Copy : Copy a file or folder to a different location in the user's
	// Dropbox. If the source path is a folder all its contents will be copied.
	// Deprecated: Use `CopyV2` instead
	Copy(arg *RelocationArg) (res IsMetadata, err error)
	// CopyBatch : Copy multiple files or folders to different locations at once
	// in the user's Dropbox. This route will replace `copyBatch`. The main
	// difference is this route will return status for each entry, while
	// `copyBatch` raises failure if any entry fails. This route will either
	// finish synchronously, or return a job ID and do the async copy job in
	// background. Please use `copyBatchCheck` to check the job status.
	CopyBatchV2(arg *RelocationBatchArgBase) (res *RelocationBatchV2Launch, err error)
	// CopyBatch : Copy multiple files or folders to different locations at once
	// in the user's Dropbox. This route will return job ID immediately and do
	// the async copy job in background. Please use `copyBatchCheck` to check
	// the job status.
	// Deprecated: Use `CopyBatchV2` instead
	CopyBatch(arg *RelocationBatchArg) (res *RelocationBatchLaunch, err error)
	// CopyBatchCheck : Returns the status of an asynchronous job for
	// `copyBatch`. It returns list of results for each entry.
	CopyBatchCheckV2(arg *async.PollArg) (res *RelocationBatchV2JobStatus, err error)
	// CopyBatchCheck : Returns the status of an asynchronous job for
	// `copyBatch`. If success, it returns list of results for each entry.
	// Deprecated: Use `CopyBatchCheckV2` instead
	CopyBatchCheck(arg *async.PollArg) (res *RelocationBatchJobStatus, err error)
	// CopyReferenceGet : Get a copy reference to a file or folder. This
	// reference string can be used to save that file or folder to another
	// user's Dropbox by passing it to `copyReferenceSave`.
	CopyReferenceGet(arg *GetCopyReferenceArg) (res *GetCopyReferenceResult, err error)
	// CopyReferenceSave : Save a copy reference returned by `copyReferenceGet`
	// to the user's Dropbox.
	CopyReferenceSave(arg *SaveCopyReferenceArg) (res *SaveCopyReferenceResult, err error)
	// CreateFolder : Create a folder at a given path.
	CreateFolderV2(arg *CreateFolderArg) (res *CreateFolderResult, err error)
	// CreateFolder : Create a folder at a given path.
	// Deprecated: Use `CreateFolderV2` instead
	CreateFolder(arg *CreateFolderArg) (res *FolderMetadata, err error)
	// CreateFolderBatch : Create multiple folders at once. This route is
	// asynchronous for large batches, which returns a job ID immediately and
	// runs the create folder batch asynchronously. Otherwise, creates the
	// folders and returns the result synchronously for smaller inputs. You can
	// force asynchronous behaviour by using the
	// `CreateFolderBatchArg.force_async` flag.  Use `createFolderBatchCheck` to
	// check the job status.
	CreateFolderBatch(arg *CreateFolderBatchArg) (res *CreateFolderBatchLaunch, err error)
	// CreateFolderBatchCheck : Returns the status of an asynchronous job for
	// `createFolderBatch`. If success, it returns list of result for each
	// entry.
	CreateFolderBatchCheck(arg *async.PollArg) (res *CreateFolderBatchJobStatus, err error)
	// Delete : Delete the file or folder at a given path. If the path is a
	// folder, all its contents will be deleted too. A successful response
	// indicates that the file or folder was deleted. The returned metadata will
	// be the corresponding `FileMetadata` or `FolderMetadata` for the item at
	// time of deletion, and not a `DeletedMetadata` object.
	DeleteV2(arg *DeleteArg) (res *DeleteResult, err error)
	// Delete : Delete the file or folder at a given path. If the path is a
	// folder, all its contents will be deleted too. A successful response
	// indicates that the file or folder was deleted. The returned metadata will
	// be the corresponding `FileMetadata` or `FolderMetadata` for the item at
	// time of deletion, and not a `DeletedMetadata` object.
	// Deprecated: Use `DeleteV2` instead
	Delete(arg *DeleteArg) (res IsMetadata, err error)
	// DeleteBatch : Delete multiple files/folders at once. This route is
	// asynchronous, which returns a job ID immediately and runs the delete
	// batch asynchronously. Use `deleteBatchCheck` to check the job status.
	DeleteBatch(arg *DeleteBatchArg) (res *DeleteBatchLaunch, err error)
	// DeleteBatchCheck : Returns the status of an asynchronous job for
	// `deleteBatch`. If success, it returns list of result for each entry.
	DeleteBatchCheck(arg *async.PollArg) (res *DeleteBatchJobStatus, err error)
	// Download : Download a file from a user's Dropbox.
	Download(arg *DownloadArg) (res *FileMetadata, content io.ReadCloser, err error)
	// DownloadZip : Download a folder from the user's Dropbox, as a zip file.
	// The folder must be less than 20 GB in size and any single file within
	// must be less than 4 GB in size. The resulting zip must have fewer than
	// 10,000 total file and folder entries, including the top level folder. The
	// input cannot be a single file. Note: this endpoint does not support HTTP
	// range requests.
	DownloadZip(arg *DownloadZipArg) (res *DownloadZipResult, content io.ReadCloser, err error)
	// Export : Export a file from a user's Dropbox. This route only supports
	// exporting files that cannot be downloaded directly  and whose
	// `ExportResult.file_metadata` has `ExportInfo.export_as` populated.
	Export(arg *ExportArg) (res *ExportResult, content io.ReadCloser, err error)
	// GetFileLockBatch : Return the lock metadata for the given list of paths.
	GetFileLockBatch(arg *LockFileBatchArg) (res *LockFileBatchResult, err error)
	// GetMetadata : Returns the metadata for a file or folder. Note: Metadata
	// for the root folder is unsupported.
	GetMetadata(arg *GetMetadataArg) (res IsMetadata, err error)
	// GetPreview : Get a preview for a file. Currently, PDF previews are
	// generated for files with the following extensions: .ai, .doc, .docm,
	// .docx, .eps, .gdoc, .gslides, .odp, .odt, .pps, .ppsm, .ppsx, .ppt,
	// .pptm, .pptx, .rtf. HTML previews are generated for files with the
	// following extensions: .csv, .ods, .xls, .xlsm, .gsheet, .xlsx. Other
	// formats will return an unsupported extension error.
	GetPreview(arg *PreviewArg) (res *FileMetadata, content io.ReadCloser, err error)
	// GetTemporaryLink : Get a temporary link to stream content of a file. This
	// link will expire in four hours and afterwards you will get 410 Gone. This
	// URL should not be used to display content directly in the browser. The
	// Content-Type of the link is determined automatically by the file's mime
	// type.
	GetTemporaryLink(arg *GetTemporaryLinkArg) (res *GetTemporaryLinkResult, err error)
	// GetTemporaryUploadLink : Get a one-time use temporary upload link to
	// upload a file to a Dropbox location.  This endpoint acts as a delayed
	// `upload`. The returned temporary upload link may be used to make a POST
	// request with the data to be uploaded. The upload will then be perfomed
	// with the `CommitInfo` previously provided to `getTemporaryUploadLink` but
	// evaluated only upon consumption. Hence, errors stemming from invalid
	// `CommitInfo` with respect to the state of the user's Dropbox will only be
	// communicated at consumption time. Additionally, these errors are surfaced
	// as generic HTTP 409 Conflict responses, potentially hiding issue details.
	// The maximum temporary upload link duration is 4 hours. Upon consumption
	// or expiration, a new link will have to be generated. Multiple links may
	// exist for a specific upload path at any given time.  The POST request on
	// the temporary upload link must have its Content-Type set to
	// "application/octet-stream".  Example temporary upload link consumption
	// request:  curl -X POST
	// https://content.dropboxapi.com/apitul/1/bNi2uIYF51cVBND --header
	// "Content-Type: application/octet-stream" --data-binary @local_file.txt  A
	// successful temporary upload link consumption request returns the content
	// hash of the uploaded data in JSON format.  Example successful temporary
	// upload link consumption response: {"content-hash":
	// "599d71033d700ac892a0e48fa61b125d2f5994"}  An unsuccessful temporary
	// upload link consumption request returns any of the following status
	// codes:  HTTP 400 Bad Request: Content-Type is not one of
	// application/octet-stream and text/plain or request is invalid. HTTP 409
	// Conflict: The temporary upload link does not exist or is currently
	// unavailable, the upload failed, or another error happened. HTTP 410 Gone:
	// The temporary upload link is expired or consumed.  Example unsuccessful
	// temporary upload link consumption response: Temporary upload link has
	// been recently consumed.
	GetTemporaryUploadLink(arg *GetTemporaryUploadLinkArg) (res *GetTemporaryUploadLinkResult, err error)
	// GetThumbnail : Get a thumbnail for an image. This method currently
	// supports files with the following file extensions: jpg, jpeg, png, tiff,
	// tif, gif, webp, ppm and bmp. Photos that are larger than 20MB in size
	// won't be converted to a thumbnail.
	GetThumbnail(arg *ThumbnailArg) (res *FileMetadata, content io.ReadCloser, err error)
	// GetThumbnail : Get a thumbnail for an image. This method currently
	// supports files with the following file extensions: jpg, jpeg, png, tiff,
	// tif, gif, webp, ppm and bmp. Photos that are larger than 20MB in size
	// won't be converted to a thumbnail.
	GetThumbnailV2(arg *ThumbnailV2Arg) (res *PreviewResult, content io.ReadCloser, err error)
	// GetThumbnailBatch : Get thumbnails for a list of images. We allow up to
	// 25 thumbnails in a single batch. This method currently supports files
	// with the following file extensions: jpg, jpeg, png, tiff, tif, gif, webp,
	// ppm and bmp. Photos that are larger than 20MB in size won't be converted
	// to a thumbnail.
	GetThumbnailBatch(arg *GetThumbnailBatchArg) (res *GetThumbnailBatchResult, err error)
	// ListFolder : Starts returning the contents of a folder. If the result's
	// `ListFolderResult.has_more` field is true, call `listFolderContinue` with
	// the returned `ListFolderResult.cursor` to retrieve more entries. If
	// you're using `ListFolderArg.recursive` set to true to keep a local cache
	// of the contents of a Dropbox account, iterate through each entry in order
	// and process them as follows to keep your local state in sync: For each
	// `FileMetadata`, store the new entry at the given path in your local
	// state. If the required parent folders don't exist yet, create them. If
	// there's already something else at the given path, replace it and remove
	// all its children. For each `FolderMetadata`, store the new entry at the
	// given path in your local state. If the required parent folders don't
	// exist yet, create them. If there's already something else at the given
	// path, replace it but leave the children as they are. Check the new
	// entry's `FolderSharingInfo.read_only` and set all its children's
	// read-only statuses to match. For each `DeletedMetadata`, if your local
	// state has something at the given path, remove it and all its children. If
	// there's nothing at the given path, ignore this entry. Note:
	// `auth.RateLimitError` may be returned if multiple `listFolder` or
	// `listFolderContinue` calls with same parameters are made simultaneously
	// by same API app for same user. If your app implements retry logic, please
	// hold off the retry until the previous request finishes.
	ListFolder(arg *ListFolderArg) (res *ListFolderResult, err error)
	// ListFolderContinue : Once a cursor has been retrieved from `listFolder`,
	// use this to paginate through all files and retrieve updates to the
	// folder, following the same rules as documented for `listFolder`.
	ListFolderContinue(arg *ListFolderContinueArg) (res *ListFolderResult, err error)
	// ListFolderGetLatestCursor : A way to quickly get a cursor for the
	// folder's state. Unlike `listFolder`, `listFolderGetLatestCursor` doesn't
	// return any entries. This endpoint is for app which only needs to know
	// about new files and modifications and doesn't need to know about files
	// that already exist in Dropbox.
	ListFolderGetLatestCursor(arg *ListFolderArg) (res *ListFolderGetLatestCursorResult, err error)
	// ListFolderLongpoll : A longpoll endpoint to wait for changes on an
	// account. In conjunction with `listFolderContinue`, this call gives you a
	// low-latency way to monitor an account for file changes. The connection
	// will block until there are changes available or a timeout occurs. This
	// endpoint is useful mostly for client-side apps. If you're looking for
	// server-side notifications, check out our `webhooks documentation`
	// <https://www.dropbox.com/developers/reference/webhooks>.
	ListFolderLongpoll(arg *ListFolderLongpollArg) (res *ListFolderLongpollResult, err error)
	// ListRevisions : Returns revisions for files based on a file path or a
	// file id. The file path or file id is identified from the latest file
	// entry at the given file path or id. This end point allows your app to
	// query either by file path or file id by setting the mode parameter
	// appropriately. In the `ListRevisionsMode.path` (default) mode, all
	// revisions at the same file path as the latest file entry are returned. If
	// revisions with the same file id are desired, then mode must be set to
	// `ListRevisionsMode.id`. The `ListRevisionsMode.id` mode is useful to
	// retrieve revisions for a given file across moves or renames.
	ListRevisions(arg *ListRevisionsArg) (res *ListRevisionsResult, err error)
	// LockFileBatch : Lock the files at the given paths. A locked file will be
	// writable only by the lock holder. A successful response indicates that
	// the file has been locked. Returns a list of the locked file paths and
	// their metadata after this operation.
	LockFileBatch(arg *LockFileBatchArg) (res *LockFileBatchResult, err error)
	// Move : Move a file or folder to a different location in the user's
	// Dropbox. If the source path is a folder all its contents will be moved.
	// Note that we do not currently support case-only renaming.
	MoveV2(arg *RelocationArg) (res *RelocationResult, err error)
	// Move : Move a file or folder to a different location in the user's
	// Dropbox. If the source path is a folder all its contents will be moved.
	// Deprecated: Use `MoveV2` instead
	Move(arg *RelocationArg) (res IsMetadata, err error)
	// MoveBatch : Move multiple files or folders to different locations at once
	// in the user's Dropbox. Note that we do not currently support case-only
	// renaming. This route will replace `moveBatch`. The main difference is
	// this route will return status for each entry, while `moveBatch` raises
	// failure if any entry fails. This route will either finish synchronously,
	// or return a job ID and do the async move job in background. Please use
	// `moveBatchCheck` to check the job status.
	MoveBatchV2(arg *MoveBatchArg) (res *RelocationBatchV2Launch, err error)
	// MoveBatch : Move multiple files or folders to different locations at once
	// in the user's Dropbox. This route will return job ID immediately and do
	// the async moving job in background. Please use `moveBatchCheck` to check
	// the job status.
	// Deprecated: Use `MoveBatchV2` instead
	MoveBatch(arg *RelocationBatchArg) (res *RelocationBatchLaunch, err error)
	// MoveBatchCheck : Returns the status of an asynchronous job for
	// `moveBatch`. It returns list of results for each entry.
	MoveBatchCheckV2(arg *async.PollArg) (res *RelocationBatchV2JobStatus, err error)
	// MoveBatchCheck : Returns the status of an asynchronous job for
	// `moveBatch`. If success, it returns list of results for each entry.
	// Deprecated: Use `MoveBatchCheckV2` instead
	MoveBatchCheck(arg *async.PollArg) (res *RelocationBatchJobStatus, err error)
	// PaperCreate : Creates a new Paper doc with the provided content.
	PaperCreate(arg *PaperCreateArg, content io.Reader) (res *PaperCreateResult, err error)
	// PaperUpdate : Updates an existing Paper doc with the provided content.
	PaperUpdate(arg *PaperUpdateArg, content io.Reader) (res *PaperUpdateResult, err error)
	// PermanentlyDelete : Permanently delete the file or folder at a given path
	// (see https://www.dropbox.com/en/help/40). If the given file or folder is
	// not yet deleted, this route will first delete it. It is possible for this
	// route to successfully delete, then fail to permanently delete. Note: This
	// endpoint is only available for Dropbox Business apps.
	PermanentlyDelete(arg *DeleteArg) (err error)
	// PropertiesAdd : has no documentation (yet)
	// Deprecated:
	PropertiesAdd(arg *file_properties.AddPropertiesArg) (err error)
	// PropertiesOverwrite : has no documentation (yet)
	// Deprecated:
	PropertiesOverwrite(arg *file_properties.OverwritePropertyGroupArg) (err error)
	// PropertiesRemove : has no documentation (yet)
	// Deprecated:
	PropertiesRemove(arg *file_properties.RemovePropertiesArg) (err error)
	// PropertiesTemplateGet : has no documentation (yet)
	// Deprecated:
	PropertiesTemplateGet(arg *file_properties.GetTemplateArg) (res *file_properties.GetTemplateResult, err error)
	// PropertiesTemplateList : has no documentation (yet)
	// Deprecated:
	PropertiesTemplateList() (res *file_properties.ListTemplateResult, err error)
	// PropertiesUpdate : has no documentation (yet)
	// Deprecated:
	PropertiesUpdate(arg *file_properties.UpdatePropertiesArg) (err error)
	// Restore : Restore a specific revision of a file to the given path.
	Restore(arg *RestoreArg) (res *FileMetadata, err error)
	// SaveUrl : Save the data from a specified URL into a file in user's
	// Dropbox. Note that the transfer from the URL must complete within 5
	// minutes, or the operation will time out and the job will fail. If the
	// given path already exists, the file will be renamed to avoid the conflict
	// (e.g. myfile (1).txt).
	SaveUrl(arg *SaveUrlArg) (res *SaveUrlResult, err error)
	// SaveUrlCheckJobStatus : Check the status of a `saveUrl` job.
	SaveUrlCheckJobStatus(arg *async.PollArg) (res *SaveUrlJobStatus, err error)
	// Search : Searches for files and folders. Note: Recent changes will be
	// reflected in search results within a few seconds and older revisions of
	// existing files may still match your query for up to a few days.
	// Deprecated: Use `SearchV2` instead
	Search(arg *SearchArg) (res *SearchResult, err error)
	// Search : Searches for files and folders. Note: `search` along with
	// `searchContinue` can only be used to retrieve a maximum of 10,000
	// matches. Recent changes may not immediately be reflected in search
	// results due to a short delay in indexing. Duplicate results may be
	// returned across pages. Some results may not be returned.
	SearchV2(arg *SearchV2Arg) (res *SearchV2Result, err error)
	// SearchContinue : Fetches the next page of search results returned from
	// `search`. Note: `search` along with `searchContinue` can only be used to
	// retrieve a maximum of 10,000 matches. Recent changes may not immediately
	// be reflected in search results due to a short delay in indexing.
	// Duplicate results may be returned across pages. Some results may not be
	// returned.
	SearchContinueV2(arg *SearchV2ContinueArg) (res *SearchV2Result, err error)
	// TagsAdd : Add a tag to an item. A tag is a string. The strings are
	// automatically converted to lowercase letters. No more than 20 tags can be
	// added to a given item.
	TagsAdd(arg *AddTagArg) (err error)
	// TagsGet : Get list of tags assigned to items.
	TagsGet(arg *GetTagsArg) (res *GetTagsResult, err error)
	// TagsRemove : Remove a tag from an item.
	TagsRemove(arg *RemoveTagArg) (err error)
	// UnlockFileBatch : Unlock the files at the given paths. A locked file can
	// only be unlocked by the lock holder or, if a business account, a team
	// admin. A successful response indicates that the file has been unlocked.
	// Returns a list of the unlocked file paths and their metadata after this
	// operation.
	UnlockFileBatch(arg *UnlockFileBatchArg) (res *LockFileBatchResult, err error)
	// Upload : Create a new file with the contents provided in the request. Do
	// not use this to upload a file larger than 150 MB. Instead, create an
	// upload session with `uploadSessionStart`. Calls to this endpoint will
	// count as data transport calls for any Dropbox Business teams with a limit
	// on the number of data transport calls allowed per month. For more
	// information, see the `Data transport limit page`
	// <https://www.dropbox.com/developers/reference/data-transport-limit>.
	Upload(arg *UploadArg, content io.Reader) (res *FileMetadata, err error)
	// UploadSessionAppend : Append more data to an upload session. When the
	// parameter close is set, this call will close the session. A single
	// request should not upload more than 150 MB. The maximum size of a file
	// one can upload to an upload session is 350 GB. Calls to this endpoint
	// will count as data transport calls for any Dropbox Business teams with a
	// limit on the number of data transport calls allowed per month. For more
	// information, see the `Data transport limit page`
	// <https://www.dropbox.com/developers/reference/data-transport-limit>.
	UploadSessionAppendV2(arg *UploadSessionAppendArg, content io.Reader) (err error)
	// UploadSessionAppend : Append more data to an upload session. A single
	// request should not upload more than 150 MB. The maximum size of a file
	// one can upload to an upload session is 350 GB. Calls to this endpoint
	// will count as data transport calls for any Dropbox Business teams with a
	// limit on the number of data transport calls allowed per month. For more
	// information, see the `Data transport limit page`
	// <https://www.dropbox.com/developers/reference/data-transport-limit>.
	// Deprecated: Use `UploadSessionAppendV2` instead
	UploadSessionAppend(arg *UploadSessionCursor, content io.Reader) (err error)
	// UploadSessionFinish : Finish an upload session and save the uploaded data
	// to the given file path. A single request should not upload more than 150
	// MB. The maximum size of a file one can upload to an upload session is 350
	// GB. Calls to this endpoint will count as data transport calls for any
	// Dropbox Business teams with a limit on the number of data transport calls
	// allowed per month. For more information, see the `Data transport limit
	// page`
	// <https://www.dropbox.com/developers/reference/data-transport-limit>.
	UploadSessionFinish(arg *UploadSessionFinishArg, content io.Reader) (res *FileMetadata, err error)
	// UploadSessionFinishBatch : This route helps you commit many files at once
	// into a user's Dropbox. Use `uploadSessionStart` and `uploadSessionAppend`
	// to upload file contents. We recommend uploading many files in parallel to
	// increase throughput. Once the file contents have been uploaded, rather
	// than calling `uploadSessionFinish`, use this route to finish all your
	// upload sessions in a single request. `UploadSessionStartArg.close` or
	// `UploadSessionAppendArg.close` needs to be true for the last
	// `uploadSessionStart` or `uploadSessionAppend` call. The maximum size of a
	// file one can upload to an upload session is 350 GB. This route will
	// return a job_id immediately and do the async commit job in background.
	// Use `uploadSessionFinishBatchCheck` to check the job status. For the same
	// account, this route should be executed serially. That means you should
	// not start the next job before current job finishes. We allow up to 1000
	// entries in a single request. Calls to this endpoint will count as data
	// transport calls for any Dropbox Business teams with a limit on the number
	// of data transport calls allowed per month. For more information, see the
	// `Data transport limit page`
	// <https://www.dropbox.com/developers/reference/data-transport-limit>.
	// Deprecated: Use `UploadSessionFinishBatchV2` instead
	UploadSessionFinishBatch(arg *UploadSessionFinishBatchArg) (res *UploadSessionFinishBatchLaunch, err error)
	// UploadSessionFinishBatch : This route helps you commit many files at once
	// into a user's Dropbox. Use `uploadSessionStart` and `uploadSessionAppend`
	// to upload file contents. We recommend uploading many files in parallel to
	// increase throughput. Once the file contents have been uploaded, rather
	// than calling `uploadSessionFinish`, use this route to finish all your
	// upload sessions in a single request. `UploadSessionStartArg.close` or
	// `UploadSessionAppendArg.close` needs to be true for the last
	// `uploadSessionStart` or `uploadSessionAppend` call of each upload
	// session. The maximum size of a file one can upload to an upload session
	// is 350 GB. We allow up to 1000 entries in a single request. Calls to this
	// endpoint will count as data transport calls for any Dropbox Business
	// teams with a limit on the number of data transport calls allowed per
	// month. For more information, see the `Data transport limit page`
	// <https://www.dropbox.com/developers/reference/data-transport-limit>.
	UploadSessionFinishBatchV2(arg *UploadSessionFinishBatchArg) (res *UploadSessionFinishBatchResult, err error)
	// UploadSessionFinishBatchCheck : Returns the status of an asynchronous job
	// for `uploadSessionFinishBatch`. If success, it returns list of result for
	// each entry.
	UploadSessionFinishBatchCheck(arg *async.PollArg) (res *UploadSessionFinishBatchJobStatus, err error)
	// UploadSessionStart : Upload sessions allow you to upload a single file in
	// one or more requests, for example where the size of the file is greater
	// than 150 MB.  This call starts a new upload session with the given data.
	// You can then use `uploadSessionAppend` to add more data and
	// `uploadSessionFinish` to save all the data to a file in Dropbox. A single
	// request should not upload more than 150 MB. The maximum size of a file
	// one can upload to an upload session is 350 GB. An upload session can be
	// used for a maximum of 7 days. Attempting to use an
	// `UploadSessionStartResult.session_id` with `uploadSessionAppend` or
	// `uploadSessionFinish` more than 7 days after its creation will return a
	// `UploadSessionLookupError.not_found`. Calls to this endpoint will count
	// as data transport calls for any Dropbox Business teams with a limit on
	// the number of data transport calls allowed per month. For more
	// information, see the `Data transport limit page`
	// <https://www.dropbox.com/developers/reference/data-transport-limit>. By
	// default, upload sessions require you to send content of the file in
	// sequential order via consecutive `uploadSessionStart`,
	// `uploadSessionAppend`, `uploadSessionFinish` calls. For better
	// performance, you can instead optionally use a
	// `UploadSessionType.concurrent` upload session. To start a new concurrent
	// session, set `UploadSessionStartArg.session_type` to
	// `UploadSessionType.concurrent`. After that, you can send file data in
	// concurrent `uploadSessionAppend` requests. Finally finish the session
	// with `uploadSessionFinish`. There are couple of constraints with
	// concurrent sessions to make them work. You can not send data with
	// `uploadSessionStart` or `uploadSessionFinish` call, only with
	// `uploadSessionAppend` call. Also data uploaded in `uploadSessionAppend`
	// call must be multiple of 4194304 bytes (except for last
	// `uploadSessionAppend` with `UploadSessionStartArg.close` to true, that
	// may contain any remaining data).
	UploadSessionStart(arg *UploadSessionStartArg, content io.Reader) (res *UploadSessionStartResult, err error)
	// UploadSessionStartBatch : This route starts batch of upload_sessions.
	// Please refer to `upload_session/start` usage. Calls to this endpoint will
	// count as data transport calls for any Dropbox Business teams with a limit
	// on the number of data transport calls allowed per month. For more
	// information, see the `Data transport limit page`
	// <https://www.dropbox.com/developers/reference/data-transport-limit>.
	UploadSessionStartBatch(arg *UploadSessionStartBatchArg) (res *UploadSessionStartBatchResult, err error)
}

type apiImpl dropbox.Context

//AlphaGetMetadataAPIError is an error-wrapper for the alpha/get_metadata route
type AlphaGetMetadataAPIError struct {
	dropbox.APIError
	EndpointError *AlphaGetMetadataError `json:"error"`
}

func (dbx *apiImpl) AlphaGetMetadata(arg *AlphaGetMetadataArg) (res IsMetadata, err error) {
	log.Printf("WARNING: API `AlphaGetMetadata` is deprecated")
	log.Printf("Use API `GetMetadata` instead")

	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "alpha/get_metadata",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr AlphaGetMetadataAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	var tmp metadataUnion
	err = json.Unmarshal(resp, &tmp)
	if err != nil {
		return
	}
	switch tmp.Tag {
	case "file":
		res = tmp.File

	case "folder":
		res = tmp.Folder

	case "deleted":
		res = tmp.Deleted

	}
	_ = respBody
	return
}

//AlphaUploadAPIError is an error-wrapper for the alpha/upload route
type AlphaUploadAPIError struct {
	dropbox.APIError
	EndpointError *UploadError `json:"error"`
}

func (dbx *apiImpl) AlphaUpload(arg *UploadArg, content io.Reader) (res *FileMetadata, err error) {
	log.Printf("WARNING: API `AlphaUpload` is deprecated")
	log.Printf("Use API `Upload` instead")

	req := dropbox.Request{
		Host:         "content",
		Namespace:    "files",
		Route:        "alpha/upload",
		Auth:         "user",
		Style:        "upload",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, content)
	if err != nil {
		var appErr AlphaUploadAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//CopyV2APIError is an error-wrapper for the copy_v2 route
type CopyV2APIError struct {
	dropbox.APIError
	EndpointError *RelocationError `json:"error"`
}

func (dbx *apiImpl) CopyV2(arg *RelocationArg) (res *RelocationResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "copy_v2",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr CopyV2APIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//CopyAPIError is an error-wrapper for the copy route
type CopyAPIError struct {
	dropbox.APIError
	EndpointError *RelocationError `json:"error"`
}

func (dbx *apiImpl) Copy(arg *RelocationArg) (res IsMetadata, err error) {
	log.Printf("WARNING: API `Copy` is deprecated")
	log.Printf("Use API `CopyV2` instead")

	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "copy",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr CopyAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	var tmp metadataUnion
	err = json.Unmarshal(resp, &tmp)
	if err != nil {
		return
	}
	switch tmp.Tag {
	case "file":
		res = tmp.File

	case "folder":
		res = tmp.Folder

	case "deleted":
		res = tmp.Deleted

	}
	_ = respBody
	return
}

//CopyBatchV2APIError is an error-wrapper for the copy_batch_v2 route
type CopyBatchV2APIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) CopyBatchV2(arg *RelocationBatchArgBase) (res *RelocationBatchV2Launch, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "copy_batch_v2",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr CopyBatchV2APIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//CopyBatchAPIError is an error-wrapper for the copy_batch route
type CopyBatchAPIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) CopyBatch(arg *RelocationBatchArg) (res *RelocationBatchLaunch, err error) {
	log.Printf("WARNING: API `CopyBatch` is deprecated")
	log.Printf("Use API `CopyBatchV2` instead")

	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "copy_batch",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr CopyBatchAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//CopyBatchCheckV2APIError is an error-wrapper for the copy_batch/check_v2 route
type CopyBatchCheckV2APIError struct {
	dropbox.APIError
	EndpointError *async.PollError `json:"error"`
}

func (dbx *apiImpl) CopyBatchCheckV2(arg *async.PollArg) (res *RelocationBatchV2JobStatus, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "copy_batch/check_v2",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr CopyBatchCheckV2APIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//CopyBatchCheckAPIError is an error-wrapper for the copy_batch/check route
type CopyBatchCheckAPIError struct {
	dropbox.APIError
	EndpointError *async.PollError `json:"error"`
}

func (dbx *apiImpl) CopyBatchCheck(arg *async.PollArg) (res *RelocationBatchJobStatus, err error) {
	log.Printf("WARNING: API `CopyBatchCheck` is deprecated")
	log.Printf("Use API `CopyBatchCheckV2` instead")

	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "copy_batch/check",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr CopyBatchCheckAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//CopyReferenceGetAPIError is an error-wrapper for the copy_reference/get route
type CopyReferenceGetAPIError struct {
	dropbox.APIError
	EndpointError *GetCopyReferenceError `json:"error"`
}

func (dbx *apiImpl) CopyReferenceGet(arg *GetCopyReferenceArg) (res *GetCopyReferenceResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "copy_reference/get",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr CopyReferenceGetAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//CopyReferenceSaveAPIError is an error-wrapper for the copy_reference/save route
type CopyReferenceSaveAPIError struct {
	dropbox.APIError
	EndpointError *SaveCopyReferenceError `json:"error"`
}

func (dbx *apiImpl) CopyReferenceSave(arg *SaveCopyReferenceArg) (res *SaveCopyReferenceResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "copy_reference/save",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr CopyReferenceSaveAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//CreateFolderV2APIError is an error-wrapper for the create_folder_v2 route
type CreateFolderV2APIError struct {
	dropbox.APIError
	EndpointError *CreateFolderError `json:"error"`
}

func (dbx *apiImpl) CreateFolderV2(arg *CreateFolderArg) (res *CreateFolderResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "create_folder_v2",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr CreateFolderV2APIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//CreateFolderAPIError is an error-wrapper for the create_folder route
type CreateFolderAPIError struct {
	dropbox.APIError
	EndpointError *CreateFolderError `json:"error"`
}

func (dbx *apiImpl) CreateFolder(arg *CreateFolderArg) (res *FolderMetadata, err error) {
	log.Printf("WARNING: API `CreateFolder` is deprecated")
	log.Printf("Use API `CreateFolderV2` instead")

	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "create_folder",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr CreateFolderAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//CreateFolderBatchAPIError is an error-wrapper for the create_folder_batch route
type CreateFolderBatchAPIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) CreateFolderBatch(arg *CreateFolderBatchArg) (res *CreateFolderBatchLaunch, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "create_folder_batch",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr CreateFolderBatchAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//CreateFolderBatchCheckAPIError is an error-wrapper for the create_folder_batch/check route
type CreateFolderBatchCheckAPIError struct {
	dropbox.APIError
	EndpointError *async.PollError `json:"error"`
}

func (dbx *apiImpl) CreateFolderBatchCheck(arg *async.PollArg) (res *CreateFolderBatchJobStatus, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "create_folder_batch/check",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr CreateFolderBatchCheckAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//DeleteV2APIError is an error-wrapper for the delete_v2 route
type DeleteV2APIError struct {
	dropbox.APIError
	EndpointError *DeleteError `json:"error"`
}

func (dbx *apiImpl) DeleteV2(arg *DeleteArg) (res *DeleteResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "delete_v2",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr DeleteV2APIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//DeleteAPIError is an error-wrapper for the delete route
type DeleteAPIError struct {
	dropbox.APIError
	EndpointError *DeleteError `json:"error"`
}

func (dbx *apiImpl) Delete(arg *DeleteArg) (res IsMetadata, err error) {
	log.Printf("WARNING: API `Delete` is deprecated")
	log.Printf("Use API `DeleteV2` instead")

	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "delete",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr DeleteAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	var tmp metadataUnion
	err = json.Unmarshal(resp, &tmp)
	if err != nil {
		return
	}
	switch tmp.Tag {
	case "file":
		res = tmp.File

	case "folder":
		res = tmp.Folder

	case "deleted":
		res = tmp.Deleted

	}
	_ = respBody
	return
}

//DeleteBatchAPIError is an error-wrapper for the delete_batch route
type DeleteBatchAPIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) DeleteBatch(arg *DeleteBatchArg) (res *DeleteBatchLaunch, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "delete_batch",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr DeleteBatchAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//DeleteBatchCheckAPIError is an error-wrapper for the delete_batch/check route
type DeleteBatchCheckAPIError struct {
	dropbox.APIError
	EndpointError *async.PollError `json:"error"`
}

func (dbx *apiImpl) DeleteBatchCheck(arg *async.PollArg) (res *DeleteBatchJobStatus, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "delete_batch/check",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr DeleteBatchCheckAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//DownloadAPIError is an error-wrapper for the download route
type DownloadAPIError struct {
	dropbox.APIError
	EndpointError *DownloadError `json:"error"`
}

func (dbx *apiImpl) Download(arg *DownloadArg) (res *FileMetadata, content io.ReadCloser, err error) {
	req := dropbox.Request{
		Host:         "content",
		Namespace:    "files",
		Route:        "download",
		Auth:         "user",
		Style:        "download",
		Arg:          arg,
		ExtraHeaders: arg.ExtraHeaders,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr DownloadAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	content = respBody
	return
}

//DownloadZipAPIError is an error-wrapper for the download_zip route
type DownloadZipAPIError struct {
	dropbox.APIError
	EndpointError *DownloadZipError `json:"error"`
}

func (dbx *apiImpl) DownloadZip(arg *DownloadZipArg) (res *DownloadZipResult, content io.ReadCloser, err error) {
	req := dropbox.Request{
		Host:         "content",
		Namespace:    "files",
		Route:        "download_zip",
		Auth:         "user",
		Style:        "download",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr DownloadZipAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	content = respBody
	return
}

//ExportAPIError is an error-wrapper for the export route
type ExportAPIError struct {
	dropbox.APIError
	EndpointError *ExportError `json:"error"`
}

func (dbx *apiImpl) Export(arg *ExportArg) (res *ExportResult, content io.ReadCloser, err error) {
	req := dropbox.Request{
		Host:         "content",
		Namespace:    "files",
		Route:        "export",
		Auth:         "user",
		Style:        "download",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr ExportAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	content = respBody
	return
}

//GetFileLockBatchAPIError is an error-wrapper for the get_file_lock_batch route
type GetFileLockBatchAPIError struct {
	dropbox.APIError
	EndpointError *LockFileError `json:"error"`
}

func (dbx *apiImpl) GetFileLockBatch(arg *LockFileBatchArg) (res *LockFileBatchResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "get_file_lock_batch",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr GetFileLockBatchAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//GetMetadataAPIError is an error-wrapper for the get_metadata route
type GetMetadataAPIError struct {
	dropbox.APIError
	EndpointError *GetMetadataError `json:"error"`
}

func (dbx *apiImpl) GetMetadata(arg *GetMetadataArg) (res IsMetadata, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "get_metadata",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr GetMetadataAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	var tmp metadataUnion
	err = json.Unmarshal(resp, &tmp)
	if err != nil {
		return
	}
	switch tmp.Tag {
	case "file":
		res = tmp.File

	case "folder":
		res = tmp.Folder

	case "deleted":
		res = tmp.Deleted

	}
	_ = respBody
	return
}

//GetPreviewAPIError is an error-wrapper for the get_preview route
type GetPreviewAPIError struct {
	dropbox.APIError
	EndpointError *PreviewError `json:"error"`
}

func (dbx *apiImpl) GetPreview(arg *PreviewArg) (res *FileMetadata, content io.ReadCloser, err error) {
	req := dropbox.Request{
		Host:         "content",
		Namespace:    "files",
		Route:        "get_preview",
		Auth:         "user",
		Style:        "download",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr GetPreviewAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	content = respBody
	return
}

//GetTemporaryLinkAPIError is an error-wrapper for the get_temporary_link route
type GetTemporaryLinkAPIError struct {
	dropbox.APIError
	EndpointError *GetTemporaryLinkError `json:"error"`
}

func (dbx *apiImpl) GetTemporaryLink(arg *GetTemporaryLinkArg) (res *GetTemporaryLinkResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "get_temporary_link",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr GetTemporaryLinkAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//GetTemporaryUploadLinkAPIError is an error-wrapper for the get_temporary_upload_link route
type GetTemporaryUploadLinkAPIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) GetTemporaryUploadLink(arg *GetTemporaryUploadLinkArg) (res *GetTemporaryUploadLinkResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "get_temporary_upload_link",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr GetTemporaryUploadLinkAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//GetThumbnailAPIError is an error-wrapper for the get_thumbnail route
type GetThumbnailAPIError struct {
	dropbox.APIError
	EndpointError *ThumbnailError `json:"error"`
}

func (dbx *apiImpl) GetThumbnail(arg *ThumbnailArg) (res *FileMetadata, content io.ReadCloser, err error) {
	req := dropbox.Request{
		Host:         "content",
		Namespace:    "files",
		Route:        "get_thumbnail",
		Auth:         "user",
		Style:        "download",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr GetThumbnailAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	content = respBody
	return
}

//GetThumbnailV2APIError is an error-wrapper for the get_thumbnail_v2 route
type GetThumbnailV2APIError struct {
	dropbox.APIError
	EndpointError *ThumbnailV2Error `json:"error"`
}

func (dbx *apiImpl) GetThumbnailV2(arg *ThumbnailV2Arg) (res *PreviewResult, content io.ReadCloser, err error) {
	req := dropbox.Request{
		Host:         "content",
		Namespace:    "files",
		Route:        "get_thumbnail_v2",
		Auth:         "app, user",
		Style:        "download",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr GetThumbnailV2APIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	content = respBody
	return
}

//GetThumbnailBatchAPIError is an error-wrapper for the get_thumbnail_batch route
type GetThumbnailBatchAPIError struct {
	dropbox.APIError
	EndpointError *GetThumbnailBatchError `json:"error"`
}

func (dbx *apiImpl) GetThumbnailBatch(arg *GetThumbnailBatchArg) (res *GetThumbnailBatchResult, err error) {
	req := dropbox.Request{
		Host:         "content",
		Namespace:    "files",
		Route:        "get_thumbnail_batch",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr GetThumbnailBatchAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//ListFolderAPIError is an error-wrapper for the list_folder route
type ListFolderAPIError struct {
	dropbox.APIError
	EndpointError *ListFolderError `json:"error"`
}

func (dbx *apiImpl) ListFolder(arg *ListFolderArg) (res *ListFolderResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "list_folder",
		Auth:         "app, user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr ListFolderAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//ListFolderContinueAPIError is an error-wrapper for the list_folder/continue route
type ListFolderContinueAPIError struct {
	dropbox.APIError
	EndpointError *ListFolderContinueError `json:"error"`
}

func (dbx *apiImpl) ListFolderContinue(arg *ListFolderContinueArg) (res *ListFolderResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "list_folder/continue",
		Auth:         "app, user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr ListFolderContinueAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//ListFolderGetLatestCursorAPIError is an error-wrapper for the list_folder/get_latest_cursor route
type ListFolderGetLatestCursorAPIError struct {
	dropbox.APIError
	EndpointError *ListFolderError `json:"error"`
}

func (dbx *apiImpl) ListFolderGetLatestCursor(arg *ListFolderArg) (res *ListFolderGetLatestCursorResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "list_folder/get_latest_cursor",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr ListFolderGetLatestCursorAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//ListFolderLongpollAPIError is an error-wrapper for the list_folder/longpoll route
type ListFolderLongpollAPIError struct {
	dropbox.APIError
	EndpointError *ListFolderLongpollError `json:"error"`
}

func (dbx *apiImpl) ListFolderLongpoll(arg *ListFolderLongpollArg) (res *ListFolderLongpollResult, err error) {
	req := dropbox.Request{
		Host:         "notify",
		Namespace:    "files",
		Route:        "list_folder/longpoll",
		Auth:         "noauth",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr ListFolderLongpollAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//ListRevisionsAPIError is an error-wrapper for the list_revisions route
type ListRevisionsAPIError struct {
	dropbox.APIError
	EndpointError *ListRevisionsError `json:"error"`
}

func (dbx *apiImpl) ListRevisions(arg *ListRevisionsArg) (res *ListRevisionsResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "list_revisions",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr ListRevisionsAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//LockFileBatchAPIError is an error-wrapper for the lock_file_batch route
type LockFileBatchAPIError struct {
	dropbox.APIError
	EndpointError *LockFileError `json:"error"`
}

func (dbx *apiImpl) LockFileBatch(arg *LockFileBatchArg) (res *LockFileBatchResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "lock_file_batch",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr LockFileBatchAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MoveV2APIError is an error-wrapper for the move_v2 route
type MoveV2APIError struct {
	dropbox.APIError
	EndpointError *RelocationError `json:"error"`
}

func (dbx *apiImpl) MoveV2(arg *RelocationArg) (res *RelocationResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "move_v2",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MoveV2APIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MoveAPIError is an error-wrapper for the move route
type MoveAPIError struct {
	dropbox.APIError
	EndpointError *RelocationError `json:"error"`
}

func (dbx *apiImpl) Move(arg *RelocationArg) (res IsMetadata, err error) {
	log.Printf("WARNING: API `Move` is deprecated")
	log.Printf("Use API `MoveV2` instead")

	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "move",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MoveAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	var tmp metadataUnion
	err = json.Unmarshal(resp, &tmp)
	if err != nil {
		return
	}
	switch tmp.Tag {
	case "file":
		res = tmp.File

	case "folder":
		res = tmp.Folder

	case "deleted":
		res = tmp.Deleted

	}
	_ = respBody
	return
}

//MoveBatchV2APIError is an error-wrapper for the move_batch_v2 route
type MoveBatchV2APIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) MoveBatchV2(arg *MoveBatchArg) (res *RelocationBatchV2Launch, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "move_batch_v2",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MoveBatchV2APIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MoveBatchAPIError is an error-wrapper for the move_batch route
type MoveBatchAPIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) MoveBatch(arg *RelocationBatchArg) (res *RelocationBatchLaunch, err error) {
	log.Printf("WARNING: API `MoveBatch` is deprecated")
	log.Printf("Use API `MoveBatchV2` instead")

	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "move_batch",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MoveBatchAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MoveBatchCheckV2APIError is an error-wrapper for the move_batch/check_v2 route
type MoveBatchCheckV2APIError struct {
	dropbox.APIError
	EndpointError *async.PollError `json:"error"`
}

func (dbx *apiImpl) MoveBatchCheckV2(arg *async.PollArg) (res *RelocationBatchV2JobStatus, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "move_batch/check_v2",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MoveBatchCheckV2APIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//MoveBatchCheckAPIError is an error-wrapper for the move_batch/check route
type MoveBatchCheckAPIError struct {
	dropbox.APIError
	EndpointError *async.PollError `json:"error"`
}

func (dbx *apiImpl) MoveBatchCheck(arg *async.PollArg) (res *RelocationBatchJobStatus, err error) {
	log.Printf("WARNING: API `MoveBatchCheck` is deprecated")
	log.Printf("Use API `MoveBatchCheckV2` instead")

	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "move_batch/check",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr MoveBatchCheckAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//PaperCreateAPIError is an error-wrapper for the paper/create route
type PaperCreateAPIError struct {
	dropbox.APIError
	EndpointError *PaperCreateError `json:"error"`
}

func (dbx *apiImpl) PaperCreate(arg *PaperCreateArg, content io.Reader) (res *PaperCreateResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "paper/create",
		Auth:         "user",
		Style:        "upload",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, content)
	if err != nil {
		var appErr PaperCreateAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//PaperUpdateAPIError is an error-wrapper for the paper/update route
type PaperUpdateAPIError struct {
	dropbox.APIError
	EndpointError *PaperUpdateError `json:"error"`
}

func (dbx *apiImpl) PaperUpdate(arg *PaperUpdateArg, content io.Reader) (res *PaperUpdateResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "paper/update",
		Auth:         "user",
		Style:        "upload",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, content)
	if err != nil {
		var appErr PaperUpdateAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//PermanentlyDeleteAPIError is an error-wrapper for the permanently_delete route
type PermanentlyDeleteAPIError struct {
	dropbox.APIError
	EndpointError *DeleteError `json:"error"`
}

func (dbx *apiImpl) PermanentlyDelete(arg *DeleteArg) (err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "permanently_delete",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr PermanentlyDeleteAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	_ = resp
	_ = respBody
	return
}

//PropertiesAddAPIError is an error-wrapper for the properties/add route
type PropertiesAddAPIError struct {
	dropbox.APIError
	EndpointError *file_properties.AddPropertiesError `json:"error"`
}

func (dbx *apiImpl) PropertiesAdd(arg *file_properties.AddPropertiesArg) (err error) {
	log.Printf("WARNING: API `PropertiesAdd` is deprecated")

	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "properties/add",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr PropertiesAddAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	_ = resp
	_ = respBody
	return
}

//PropertiesOverwriteAPIError is an error-wrapper for the properties/overwrite route
type PropertiesOverwriteAPIError struct {
	dropbox.APIError
	EndpointError *file_properties.InvalidPropertyGroupError `json:"error"`
}

func (dbx *apiImpl) PropertiesOverwrite(arg *file_properties.OverwritePropertyGroupArg) (err error) {
	log.Printf("WARNING: API `PropertiesOverwrite` is deprecated")

	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "properties/overwrite",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr PropertiesOverwriteAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	_ = resp
	_ = respBody
	return
}

//PropertiesRemoveAPIError is an error-wrapper for the properties/remove route
type PropertiesRemoveAPIError struct {
	dropbox.APIError
	EndpointError *file_properties.RemovePropertiesError `json:"error"`
}

func (dbx *apiImpl) PropertiesRemove(arg *file_properties.RemovePropertiesArg) (err error) {
	log.Printf("WARNING: API `PropertiesRemove` is deprecated")

	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "properties/remove",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr PropertiesRemoveAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	_ = resp
	_ = respBody
	return
}

//PropertiesTemplateGetAPIError is an error-wrapper for the properties/template/get route
type PropertiesTemplateGetAPIError struct {
	dropbox.APIError
	EndpointError *file_properties.TemplateError `json:"error"`
}

func (dbx *apiImpl) PropertiesTemplateGet(arg *file_properties.GetTemplateArg) (res *file_properties.GetTemplateResult, err error) {
	log.Printf("WARNING: API `PropertiesTemplateGet` is deprecated")

	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "properties/template/get",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr PropertiesTemplateGetAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//PropertiesTemplateListAPIError is an error-wrapper for the properties/template/list route
type PropertiesTemplateListAPIError struct {
	dropbox.APIError
	EndpointError *file_properties.TemplateError `json:"error"`
}

func (dbx *apiImpl) PropertiesTemplateList() (res *file_properties.ListTemplateResult, err error) {
	log.Printf("WARNING: API `PropertiesTemplateList` is deprecated")

	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "properties/template/list",
		Auth:         "user",
		Style:        "rpc",
		Arg:          nil,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr PropertiesTemplateListAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//PropertiesUpdateAPIError is an error-wrapper for the properties/update route
type PropertiesUpdateAPIError struct {
	dropbox.APIError
	EndpointError *file_properties.UpdatePropertiesError `json:"error"`
}

func (dbx *apiImpl) PropertiesUpdate(arg *file_properties.UpdatePropertiesArg) (err error) {
	log.Printf("WARNING: API `PropertiesUpdate` is deprecated")

	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "properties/update",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr PropertiesUpdateAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	_ = resp
	_ = respBody
	return
}

//RestoreAPIError is an error-wrapper for the restore route
type RestoreAPIError struct {
	dropbox.APIError
	EndpointError *RestoreError `json:"error"`
}

func (dbx *apiImpl) Restore(arg *RestoreArg) (res *FileMetadata, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "restore",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr RestoreAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//SaveUrlAPIError is an error-wrapper for the save_url route
type SaveUrlAPIError struct {
	dropbox.APIError
	EndpointError *SaveUrlError `json:"error"`
}

func (dbx *apiImpl) SaveUrl(arg *SaveUrlArg) (res *SaveUrlResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "save_url",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr SaveUrlAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//SaveUrlCheckJobStatusAPIError is an error-wrapper for the save_url/check_job_status route
type SaveUrlCheckJobStatusAPIError struct {
	dropbox.APIError
	EndpointError *async.PollError `json:"error"`
}

func (dbx *apiImpl) SaveUrlCheckJobStatus(arg *async.PollArg) (res *SaveUrlJobStatus, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "save_url/check_job_status",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr SaveUrlCheckJobStatusAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//SearchAPIError is an error-wrapper for the search route
type SearchAPIError struct {
	dropbox.APIError
	EndpointError *SearchError `json:"error"`
}

func (dbx *apiImpl) Search(arg *SearchArg) (res *SearchResult, err error) {
	log.Printf("WARNING: API `Search` is deprecated")
	log.Printf("Use API `SearchV2` instead")

	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "search",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr SearchAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//SearchV2APIError is an error-wrapper for the search_v2 route
type SearchV2APIError struct {
	dropbox.APIError
	EndpointError *SearchError `json:"error"`
}

func (dbx *apiImpl) SearchV2(arg *SearchV2Arg) (res *SearchV2Result, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "search_v2",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr SearchV2APIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//SearchContinueV2APIError is an error-wrapper for the search/continue_v2 route
type SearchContinueV2APIError struct {
	dropbox.APIError
	EndpointError *SearchError `json:"error"`
}

func (dbx *apiImpl) SearchContinueV2(arg *SearchV2ContinueArg) (res *SearchV2Result, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "search/continue_v2",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr SearchContinueV2APIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//TagsAddAPIError is an error-wrapper for the tags/add route
type TagsAddAPIError struct {
	dropbox.APIError
	EndpointError *AddTagError `json:"error"`
}

func (dbx *apiImpl) TagsAdd(arg *AddTagArg) (err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "tags/add",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr TagsAddAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	_ = resp
	_ = respBody
	return
}

//TagsGetAPIError is an error-wrapper for the tags/get route
type TagsGetAPIError struct {
	dropbox.APIError
	EndpointError *BaseTagError `json:"error"`
}

func (dbx *apiImpl) TagsGet(arg *GetTagsArg) (res *GetTagsResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "tags/get",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr TagsGetAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//TagsRemoveAPIError is an error-wrapper for the tags/remove route
type TagsRemoveAPIError struct {
	dropbox.APIError
	EndpointError *RemoveTagError `json:"error"`
}

func (dbx *apiImpl) TagsRemove(arg *RemoveTagArg) (err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "tags/remove",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr TagsRemoveAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	_ = resp
	_ = respBody
	return
}

//UnlockFileBatchAPIError is an error-wrapper for the unlock_file_batch route
type UnlockFileBatchAPIError struct {
	dropbox.APIError
	EndpointError *LockFileError `json:"error"`
}

func (dbx *apiImpl) UnlockFileBatch(arg *UnlockFileBatchArg) (res *LockFileBatchResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "unlock_file_batch",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr UnlockFileBatchAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//UploadAPIError is an error-wrapper for the upload route
type UploadAPIError struct {
	dropbox.APIError
	EndpointError *UploadError `json:"error"`
}

func (dbx *apiImpl) Upload(arg *UploadArg, content io.Reader) (res *FileMetadata, err error) {
	req := dropbox.Request{
		Host:         "content",
		Namespace:    "files",
		Route:        "upload",
		Auth:         "user",
		Style:        "upload",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, content)
	if err != nil {
		var appErr UploadAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//UploadSessionAppendV2APIError is an error-wrapper for the upload_session/append_v2 route
type UploadSessionAppendV2APIError struct {
	dropbox.APIError
	EndpointError *UploadSessionAppendError `json:"error"`
}

func (dbx *apiImpl) UploadSessionAppendV2(arg *UploadSessionAppendArg, content io.Reader) (err error) {
	req := dropbox.Request{
		Host:         "content",
		Namespace:    "files",
		Route:        "upload_session/append_v2",
		Auth:         "user",
		Style:        "upload",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, content)
	if err != nil {
		var appErr UploadSessionAppendV2APIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	_ = resp
	_ = respBody
	return
}

//UploadSessionAppendAPIError is an error-wrapper for the upload_session/append route
type UploadSessionAppendAPIError struct {
	dropbox.APIError
	EndpointError *UploadSessionAppendError `json:"error"`
}

func (dbx *apiImpl) UploadSessionAppend(arg *UploadSessionCursor, content io.Reader) (err error) {
	log.Printf("WARNING: API `UploadSessionAppend` is deprecated")
	log.Printf("Use API `UploadSessionAppendV2` instead")

	req := dropbox.Request{
		Host:         "content",
		Namespace:    "files",
		Route:        "upload_session/append",
		Auth:         "user",
		Style:        "upload",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, content)
	if err != nil {
		var appErr UploadSessionAppendAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	_ = resp
	_ = respBody
	return
}

//UploadSessionFinishAPIError is an error-wrapper for the upload_session/finish route
type UploadSessionFinishAPIError struct {
	dropbox.APIError
	EndpointError *UploadSessionFinishError `json:"error"`
}

func (dbx *apiImpl) UploadSessionFinish(arg *UploadSessionFinishArg, content io.Reader) (res *FileMetadata, err error) {
	req := dropbox.Request{
		Host:         "content",
		Namespace:    "files",
		Route:        "upload_session/finish",
		Auth:         "user",
		Style:        "upload",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, content)
	if err != nil {
		var appErr UploadSessionFinishAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//UploadSessionFinishBatchAPIError is an error-wrapper for the upload_session/finish_batch route
type UploadSessionFinishBatchAPIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) UploadSessionFinishBatch(arg *UploadSessionFinishBatchArg) (res *UploadSessionFinishBatchLaunch, err error) {
	log.Printf("WARNING: API `UploadSessionFinishBatch` is deprecated")
	log.Printf("Use API `UploadSessionFinishBatchV2` instead")

	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "upload_session/finish_batch",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr UploadSessionFinishBatchAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//UploadSessionFinishBatchV2APIError is an error-wrapper for the upload_session/finish_batch_v2 route
type UploadSessionFinishBatchV2APIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) UploadSessionFinishBatchV2(arg *UploadSessionFinishBatchArg) (res *UploadSessionFinishBatchResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "upload_session/finish_batch_v2",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr UploadSessionFinishBatchV2APIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//UploadSessionFinishBatchCheckAPIError is an error-wrapper for the upload_session/finish_batch/check route
type UploadSessionFinishBatchCheckAPIError struct {
	dropbox.APIError
	EndpointError *async.PollError `json:"error"`
}

func (dbx *apiImpl) UploadSessionFinishBatchCheck(arg *async.PollArg) (res *UploadSessionFinishBatchJobStatus, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "upload_session/finish_batch/check",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr UploadSessionFinishBatchCheckAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//UploadSessionStartAPIError is an error-wrapper for the upload_session/start route
type UploadSessionStartAPIError struct {
	dropbox.APIError
	EndpointError *UploadSessionStartError `json:"error"`
}

func (dbx *apiImpl) UploadSessionStart(arg *UploadSessionStartArg, content io.Reader) (res *UploadSessionStartResult, err error) {
	req := dropbox.Request{
		Host:         "content",
		Namespace:    "files",
		Route:        "upload_session/start",
		Auth:         "user",
		Style:        "upload",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, content)
	if err != nil {
		var appErr UploadSessionStartAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

//UploadSessionStartBatchAPIError is an error-wrapper for the upload_session/start_batch route
type UploadSessionStartBatchAPIError struct {
	dropbox.APIError
	EndpointError struct{} `json:"error"`
}

func (dbx *apiImpl) UploadSessionStartBatch(arg *UploadSessionStartBatchArg) (res *UploadSessionStartBatchResult, err error) {
	req := dropbox.Request{
		Host:         "api",
		Namespace:    "files",
		Route:        "upload_session/start_batch",
		Auth:         "user",
		Style:        "rpc",
		Arg:          arg,
		ExtraHeaders: nil,
	}

	var resp []byte
	var respBody io.ReadCloser
	resp, respBody, err = (*dropbox.Context)(dbx).Execute(req, nil)
	if err != nil {
		var appErr UploadSessionStartBatchAPIError
		err = auth.ParseError(err, &appErr)
		if err == &appErr {
			err = appErr
		}
		return
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return
	}

	_ = respBody
	return
}

// New returns a Client implementation for this namespace
func New(c dropbox.Config) Client {
	ctx := apiImpl(dropbox.NewContext(c))
	return &ctx
}
