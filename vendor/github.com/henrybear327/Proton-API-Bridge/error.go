package proton_api_bridge

import "errors"

var (
	ErrMainSharePreconditionsFailed          = errors.New("the main share assumption has failed")
	ErrDataFolderNameIsEmpty                 = errors.New("please supply a DataFolderName to enabling file downloading")
	ErrLinkTypeMustToBeFolderType            = errors.New("the link type must be of folder type")
	ErrLinkTypeMustToBeFileType              = errors.New("the link type must be of file type")
	ErrFolderIsNotEmpty                      = errors.New("folder can't be deleted because it is not empty")
	ErrCantLocateRevision                    = errors.New("can't create a new file upload request and can't find an active/draft revision")
	ErrInternalErrorOnFileUpload             = errors.New("either link or file creation request should be not nil")
	ErrMissingInputUploadAndCollectBlockData = errors.New("missing either session key or key ring")
	ErrLinkMustNotBeNil                      = errors.New("missing input proton link")
	ErrLinkMustBeActive                      = errors.New("can not operate on link state other than active")
	ErrDownloadedBlockHashVerificationFailed = errors.New("the hash of the downloaded block doesn't match the original hash")
	ErrDraftExists                           = errors.New("a draft exist - usually this means a file is being uploaded at another client, or, there was a failed upload attempt")
	ErrCantFindActiveRevision                = errors.New("can't find an active revision")
	ErrCantFindDraftRevision                 = errors.New("can't find a draft revision")
	ErrWrongUsageOfGetLinkKR                 = errors.New("internal error for GetLinkKR - nil passed in for link")
	ErrWrongUsageOfGetLink                   = errors.New("internal error for getLink - empty linkID passed in")
	ErrSeekOffsetAfterSkippingBlocks         = errors.New("internal error for download seek - the offset after skipping blocks is wrong")
)
