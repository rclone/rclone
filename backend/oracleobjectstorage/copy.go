//go:build !plan9 && !solaris && !js

package oracleobjectstorage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/rclone/rclone/fs"
)

// ------------------------------------------------------------
// Implement Copier is an optional interfaces for Fs
//------------------------------------------------------------

// Copy src to this remote using server-side copy operations.
// This is stored with the remote path given
// It returns the destination Object and a possible error
// Will only be called if src.Fs().Name() == f.Name()
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	// fs.Debugf(f, "copying %v to %v", src.Remote(), remote)
	srcObj, ok := src.(*Object)
	if !ok {
		// fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	// Temporary Object under construction
	dstObj := &Object{
		fs:     f,
		remote: remote,
	}
	err := f.copy(ctx, dstObj, srcObj)
	if err != nil {
		return nil, err
	}
	return f.NewObject(ctx, remote)
}

// copy does a server-side copy from dstObj <- srcObj
//
// If newInfo is nil then the metadata will be copied otherwise it
// will be replaced with newInfo
func (f *Fs) copy(ctx context.Context, dstObj *Object, srcObj *Object) (err error) {
	srcBucket, srcPath := srcObj.split()
	dstBucket, dstPath := dstObj.split()
	if dstBucket != srcBucket {
		exists, err := f.bucketExists(ctx, dstBucket)
		if err != nil {
			return err
		}
		if !exists {
			err = f.makeBucket(ctx, dstBucket)
			if err != nil {
				return err
			}
		}
	}
	copyObjectDetails := objectstorage.CopyObjectDetails{
		SourceObjectName:          common.String(srcPath),
		DestinationRegion:         common.String(dstObj.fs.opt.Region),
		DestinationNamespace:      common.String(dstObj.fs.opt.Namespace),
		DestinationBucket:         common.String(dstBucket),
		DestinationObjectName:     common.String(dstPath),
		DestinationObjectMetadata: metadataWithOpcPrefix(srcObj.meta),
	}
	req := objectstorage.CopyObjectRequest{
		NamespaceName:     common.String(srcObj.fs.opt.Namespace),
		BucketName:        common.String(srcBucket),
		CopyObjectDetails: copyObjectDetails,
	}
	useBYOKCopyObject(f, &req)
	var resp objectstorage.CopyObjectResponse
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CopyObject(ctx, req)
		return shouldRetry(ctx, resp.HTTPResponse(), err)
	})
	if err != nil {
		return err
	}
	workRequestID := resp.OpcWorkRequestId
	timeout := time.Duration(f.opt.CopyTimeout)
	dstName := dstObj.String()
	// https://docs.oracle.com/en-us/iaas/Content/Object/Tasks/copyingobjects.htm
	// To enable server side copy object, customers will have to
	// grant policy to objectstorage service to manage object-family
	// Allow service objectstorage-<region_identifier> to manage object-family in tenancy
	// Another option to avoid the policy is to download and reupload the file.
	// This download upload will work for maximum file size limit of 5GB
	err = copyObjectWaitForWorkRequest(ctx, workRequestID, dstName, timeout, f.srv)
	if err != nil {
		return err
	}
	return err
}

func copyObjectWaitForWorkRequest(ctx context.Context, wID *string, entityType string, timeout time.Duration,
	client *objectstorage.ObjectStorageClient) error {

	stateConf := &StateChangeConf{
		Pending: []string{
			string(objectstorage.WorkRequestStatusAccepted),
			string(objectstorage.WorkRequestStatusInProgress),
			string(objectstorage.WorkRequestStatusCanceling),
		},
		Target: []string{
			string(objectstorage.WorkRequestSummaryStatusCompleted),
			string(objectstorage.WorkRequestSummaryStatusCanceled),
			string(objectstorage.WorkRequestStatusFailed),
		},
		Refresh: func() (interface{}, string, error) {
			getWorkRequestRequest := objectstorage.GetWorkRequestRequest{}
			getWorkRequestRequest.WorkRequestId = wID
			workRequestResponse, err := client.GetWorkRequest(context.Background(), getWorkRequestRequest)
			wr := &workRequestResponse.WorkRequest
			return workRequestResponse, string(wr.Status), err
		},
		Timeout: timeout,
	}

	wrr, e := stateConf.WaitForStateContext(ctx, entityType)
	if e != nil {
		return fmt.Errorf("work request did not succeed, workId: %s, entity: %s. Message: %s", *wID, entityType, e)
	}

	wr := wrr.(objectstorage.GetWorkRequestResponse).WorkRequest
	if wr.Status == objectstorage.WorkRequestStatusFailed {
		errorMessage, _ := getObjectStorageErrorFromWorkRequest(ctx, wID, client)
		return fmt.Errorf("work request did not succeed, workId: %s, entity: %s. Message: %s", *wID, entityType, errorMessage)
	}

	return nil
}

func getObjectStorageErrorFromWorkRequest(ctx context.Context, workRequestID *string, client *objectstorage.ObjectStorageClient) (string, error) {
	req := objectstorage.ListWorkRequestErrorsRequest{}
	req.WorkRequestId = workRequestID
	res, err := client.ListWorkRequestErrors(ctx, req)

	if err != nil {
		return "", err
	}

	allErrs := make([]string, 0)
	for _, errs := range res.Items {
		allErrs = append(allErrs, *errs.Message)
	}

	errorMessage := strings.Join(allErrs, "\n")
	return errorMessage, nil
}
