package azblob_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"math/rand"

	"github.com/Azure/azure-pipeline-go/pipeline"
	"github.com/Azure/azure-storage-blob-go/2018-03-28/azblob"
)

// https://godoc.org/github.com/fluhus/godoc-tricks

func accountInfo() (string, string) {
	return os.Getenv("ACCOUNT_NAME"), os.Getenv("ACCOUNT_KEY")
}

// This example shows how to get started using the Azure Storage Blob SDK for Go.
func Example() {
	// From the Azure portal, get your Storage account's name and account key.
	accountName, accountKey := accountInfo()

	// Use your Storage account's name and key to create a credential object; this is used to access your account.
	credential := azblob.NewSharedKeyCredential(accountName, accountKey)

	// Create a request pipeline that is used to process HTTP(S) requests and responses. It requires
	// your account credentials. In more advanced scenarios, you can configure telemetry, retry policies,
	// logging, and other options. Also, you can configure multiple request pipelines for different scenarios.
	p := azblob.NewPipeline(credential, azblob.PipelineOptions{})

	// From the Azure portal, get your Storage account blob service URL endpoint.
	// The URL typically looks like this:
	u, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net", accountName))

	// Create an ServiceURL object that wraps the service URL and a request pipeline.
	serviceURL := azblob.NewServiceURL(*u, p)

	// Now, you can use the serviceURL to perform various container and blob operations.

	// All HTTP operations allow you to specify a Go context.Context object to control cancellation/timeout.
	ctx := context.Background() // This example uses a never-expiring context.

	// This example shows several common operations just to get you started.

	// Create a URL that references a to-be-created container in your Azure Storage account.
	// This returns a ContainerURL object that wraps the container's URL and a request pipeline (inherited from serviceURL)
	containerURL := serviceURL.NewContainerURL("mycontainer") // Container names require lowercase

	// Create the container on the service (with no metadata and no public access)
	_, err := containerURL.Create(ctx, azblob.Metadata{}, azblob.PublicAccessNone)
	if err != nil {
		log.Fatal(err)
	}

	// Create a URL that references a to-be-created blob in your Azure Storage account's container.
	// This returns a BlockBlobURL object that wraps the blob's URL and a request pipeline (inherited from containerURL)
	blobURL := containerURL.NewBlockBlobURL("HelloWorld.txt") // Blob names can be mixed case

	// Create the blob with string (plain text) content.
	data := "Hello World!"
	_, err = blobURL.Upload(ctx, strings.NewReader(data), azblob.BlobHTTPHeaders{ContentType: "text/plain"}, azblob.Metadata{}, azblob.BlobAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}

	// Download the blob's contents and verify that it worked correctly
	get, err := blobURL.Download(ctx, 0, 0, azblob.BlobAccessConditions{}, false)
	if err != nil {
		log.Fatal(err)
	}

	downloadedData := &bytes.Buffer{}
	reader := get.Body(azblob.RetryReaderOptions{})
	downloadedData.ReadFrom(reader)
	reader.Close() // The client must close the response body when finished with it
	if data != downloadedData.String() {
		log.Fatal("downloaded data doesn't match uploaded data")
	}

	// List the blob(s) in our container; since a container may hold millions of blobs, this is done 1 segment at a time.
	for marker := (azblob.Marker{}); marker.NotDone(); { // The parens around Marker{} are required to avoid compiler error.
		// Get a result segment starting with the blob indicated by the current Marker.
		listBlob, err := containerURL.ListBlobsFlatSegment(ctx, marker, azblob.ListBlobsSegmentOptions{})
		if err != nil {
			log.Fatal(err)
		}
		// IMPORTANT: ListBlobs returns the start of the next segment; you MUST use this to get
		// the next segment (after processing the current result segment).
		marker = listBlob.NextMarker

		// Process the blobs returned in this result segment (if the segment is empty, the loop body won't execute)
		for _, blobInfo := range listBlob.Segment.BlobItems {
			fmt.Print("Blob name: " + blobInfo.Name + "\n")
		}
	}

	// Delete the blob we created earlier.
	_, err = blobURL.Delete(ctx, azblob.DeleteSnapshotsOptionNone, azblob.BlobAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}

	// Delete the container we created earlier.
	_, err = containerURL.Delete(ctx, azblob.ContainerAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}
}

// This example shows how you can configure a pipeline for making HTTP requests to the Azure Storage Blob Service.
func ExampleNewPipeline() {
	// This example shows how to wire in your own logging mechanism (this example uses
	// Go's standard logger to write log information to standard error)
	logger := log.New(os.Stderr, "", log.Ldate|log.Lmicroseconds)

	// Create/configure a request pipeline options object.
	// All PipelineOptions' fields are optional; reasonable defaults are set for anything you do not specify
	po := azblob.PipelineOptions{
		// Set RetryOptions to control how HTTP request are retried when retryable failures occur
		Retry: azblob.RetryOptions{
			Policy:        azblob.RetryPolicyExponential, // Use exponential backoff as opposed to linear
			MaxTries:      3,                             // Try at most 3 times to perform the operation (set to 1 to disable retries)
			TryTimeout:    time.Second * 3,               // Maximum time allowed for any single try
			RetryDelay:    time.Second * 1,               // Backoff amount for each retry (exponential or linear)
			MaxRetryDelay: time.Second * 3,               // Max delay between retries
		},

		// Set RequestLogOptions to control how each HTTP request & its response is logged
		RequestLog: azblob.RequestLogOptions{
			LogWarningIfTryOverThreshold: time.Millisecond * 200, // A successful response taking more than this time to arrive is logged as a warning
		},

		// Set LogOptions to control what & where all pipeline log events go
		Log: pipeline.LogOptions{
			Log: func(s pipeline.LogLevel, m string) { // This func is called to log each event
				// This method is not called for filtered-out severities.
				logger.Output(2, m) // This example uses Go's standard logger
			},
			ShouldLog: func(level pipeline.LogLevel) bool {
				return level <= pipeline.LogWarning // Log all events from warning to more severe
			},
		},
	}

	// Create a request pipeline object configured with credentials and with pipeline options. Once created,
	// a pipeline object is goroutine-safe and can be safely used with many XxxURL objects simultaneously.
	p := azblob.NewPipeline(azblob.NewAnonymousCredential(), po) // A pipeline always requires some credential object

	// Once you've created a pipeline object, associate it with an XxxURL object so that you can perform HTTP requests with it.
	u, _ := url.Parse("https://myaccount.blob.core.windows.net")
	serviceURL := azblob.NewServiceURL(*u, p)
	// Use the serviceURL as desired...

	// NOTE: When you use an XxxURL object to create another XxxURL object, the new XxxURL object inherits the
	// same pipeline object as its parent. For example, the containerURL and blobURL objects (created below)
	// all share the same pipeline. Any HTTP operations you perform with these objects share the behavior (retry, logging, etc.)
	containerURL := serviceURL.NewContainerURL("mycontainer")
	blobURL := containerURL.NewBlockBlobURL("ReadMe.txt")

	// If you'd like to perform some operations with different behavior, create a new pipeline object and
	// associate it with a new XxxURL object by passing the new pipeline to the XxxURL object's WithPipeline method.

	// In this example, I reconfigure the retry policies, create a new pipeline, and then create a new
	// ContainerURL object that has the same URL as its parent.
	po.Retry = azblob.RetryOptions{
		Policy:        azblob.RetryPolicyFixed, // Use fixed time backoff
		MaxTries:      4,                       // Try at most 3 times to perform the operation (set to 1 to disable retries)
		TryTimeout:    time.Minute * 1,         // Maximum time allowed for any single try
		RetryDelay:    time.Second * 5,         // Backoff amount for each retry (exponential or linear)
		MaxRetryDelay: time.Second * 10,        // Max delay between retries
	}
	newContainerURL := containerURL.WithPipeline(azblob.NewPipeline(azblob.NewAnonymousCredential(), po))

	// Now, any XxxBlobURL object created using newContainerURL inherits the pipeline with the new retry policy.
	newBlobURL := newContainerURL.NewBlockBlobURL("ReadMe.txt")
	_, _ = blobURL, newBlobURL // Avoid compiler's "declared and not used" error
}

func ExampleStorageError() {
	// This example shows how to handle errors returned from various XxxURL methods. All these methods return an
	// object implementing the pipeline.Response interface and an object implementing Go's error interface.
	// The error result is nil if the request was successful; your code can safely use the Response interface object.
	// If error is non-nil, the error could be due to:

	// 1. An invalid argument passed to the method. You should not write code to handle these errors;
	//    instead, fix these errors as they appear during development/testing.

	// 2. A network request didn't reach an Azure Storage Service. This usually happens due to a bad URL or
	//    faulty networking infrastructure (like a router issue). In this case, an object implementing the
	//    net.Error interface will be returned. The net.Error interface offers Timeout and Temporary methods
	//    which return true if the network error is determined to be a timeout or temporary condition. If
	//    your pipeline uses the retry policy factory, then this policy looks for Timeout/Temporary and
	//    automatically retries based on the retry options you've configured. Because of the retry policy,
	//    your code will usually not call the Timeout/Temporary methods explicitly other than possibly logging
	//    the network failure.

	// 3. A network request did reach the Azure Storage Service but the service failed to perform the
	//    requested operation. In this case, an object implementing the StorageError interface is returned.
	//    The StorageError interface also implements the net.Error interface and, if you use the retry policy,
	//    you would most likely ignore the Timeout/Temporary methods. However, the StorageError interface exposes
	//    richer information such as a service error code, an error description, details data, and the
	//    service-returned http.Response. And, from the http.Response, you can get the initiating http.Request.

	u, _ := url.Parse("http://myaccount.blob.core.windows.net/mycontainer")
	containerURL := azblob.NewContainerURL(*u, azblob.NewPipeline(azblob.NewAnonymousCredential(), azblob.PipelineOptions{}))
	create, err := containerURL.Create(context.Background(), azblob.Metadata{}, azblob.PublicAccessNone)

	if err != nil { // An error occurred
		if stgErr, ok := err.(azblob.StorageError); ok { // This error is a Service-specific error
			// StorageError also implements net.Error so you could call its Timeout/Temporary methods if you want.
			switch stgErr.ServiceCode() { // Compare serviceCode to various ServiceCodeXxx constants
			case azblob.ServiceCodeContainerAlreadyExists:
				// You can also look at the http.Response object that failed.
				if failedResponse := stgErr.Response(); failedResponse != nil {
					// From the response object, you can get the initiating http.Request object
					failedRequest := failedResponse.Request
					_ = failedRequest // Avoid compiler's "declared and not used" error
				}

			case azblob.ServiceCodeContainerBeingDeleted:
				// Handle this error ...
			default:
				// Handle other errors ...
			}
		}
		log.Fatal(err) // Error is not due to Azure Storage service; networking infrastructure failure
	}

	// If err is nil, then the method was successful; use the response to access the result
	_ = create // Avoid compiler's "declared and not used" error
}

// This example shows how to break a URL into its parts so you can
// examine and/or change some of its values and then construct a new URL.
func ExampleBlobURLParts() {
	// Let's start with a URL that identifies a snapshot of a blob in a container.
	// The URL also contains a Shared Access Signature (SAS):
	u, _ := url.Parse("https://myaccount.blob.core.windows.net/mycontainter/ReadMe.txt?" +
		"snapshot=2011-03-09T01:42:34Z&" +
		"sv=2015-02-21&sr=b&st=2111-01-09T01:42:34.936Z&se=2222-03-09T01:42:34.936Z&sp=rw&sip=168.1.5.60-168.1.5.70&" +
		"spr=https,http&si=myIdentifier&ss=bf&srt=s&sig=92836758923659283652983562==")

	// You can parse this URL into its constituent parts:
	parts := azblob.NewBlobURLParts(*u)

	// Now, we access the parts (this example prints them).
	fmt.Println(parts.Host, parts.ContainerName, parts.BlobName, parts.Snapshot)
	sas := parts.SAS
	fmt.Println(sas.Version(), sas.Resource(), sas.StartTime(), sas.ExpiryTime(), sas.Permissions(),
		sas.IPRange(), sas.Protocol(), sas.Identifier(), sas.Services(), sas.Signature())

	// You can then change some of the fields and construct a new URL:
	parts.SAS = azblob.SASQueryParameters{} // Remove the SAS query parameters
	parts.Snapshot = ""                     // Remove the snapshot timestamp
	parts.ContainerName = "othercontainer"  // Change the container name
	// In this example, we'll keep the blob name as is.

	// Construct a new URL from the parts:
	newURL := parts.URL()
	fmt.Print(newURL.String())
	// NOTE: You can pass the new URL to NewBlockBlobURL (or similar methods) to manipulate the blob.
}

// This example shows how to create and use an Azure Storage account Shared Access Signature (SAS).
func ExampleAccountSASSignatureValues() {
	// From the Azure portal, get your Storage account's name and account key.
	accountName, accountKey := accountInfo()

	// Use your Storage account's name and key to create a credential object; this is required to sign a SAS.
	credential := azblob.NewSharedKeyCredential(accountName, accountKey)

	// Set the desired SAS signature values and sign them with the shared key credentials to get the SAS query parameters.
	sasQueryParams := azblob.AccountSASSignatureValues{
		Protocol:      azblob.SASProtocolHTTPS,              // Users MUST use HTTPS (not HTTP)
		ExpiryTime:    time.Now().UTC().Add(48 * time.Hour), // 48-hours before expiration
		Permissions:   azblob.AccountSASPermissions{Read: true, List: true}.String(),
		Services:      azblob.AccountSASServices{Blob: true}.String(),
		ResourceTypes: azblob.AccountSASResourceTypes{Container: true, Object: true}.String(),
	}.NewSASQueryParameters(credential)

	qp := sasQueryParams.Encode()
	urlToSendToSomeone := fmt.Sprintf("https://%s.blob.core.windows.net?%s", accountName, qp)
	// At this point, you can send the urlToSendToSomeone to someone via email or any other mechanism you choose.

	// ************************************************************************************************

	// When someone receives the URL, they access the SAS-protected resource with code like this:
	u, _ := url.Parse(urlToSendToSomeone)

	// Create an ServiceURL object that wraps the service URL (and its SAS) and a pipeline.
	// When using a SAS URLs, anonymous credentials are required.
	serviceURL := azblob.NewServiceURL(*u, azblob.NewPipeline(azblob.NewAnonymousCredential(), azblob.PipelineOptions{}))
	// Now, you can use this serviceURL just like any other to make requests of the resource.

	// You can parse a URL into its constituent parts:
	blobURLParts := azblob.NewBlobURLParts(serviceURL.URL())
	fmt.Printf("SAS expiry time=%v", blobURLParts.SAS.ExpiryTime())

	_ = serviceURL // Avoid compiler's "declared and not used" error
}

// This example shows how to create and use a Blob Service Shared Access Signature (SAS).
func ExampleBlobSASSignatureValues() {
	// From the Azure portal, get your Storage account's name and account key.
	accountName, accountKey := accountInfo()

	// Use your Storage account's name and key to create a credential object; this is required to sign a SAS.
	credential := azblob.NewSharedKeyCredential(accountName, accountKey)

	// This is the name of the container and blob that we're creating a SAS to.
	containerName := "mycontainer" // Container names require lowercase
	blobName := "HelloWorld.txt"   // Blob names can be mixed case

	// Set the desired SAS signature values and sign them with the shared key credentials to get the SAS query parameters.
	sasQueryParams := azblob.BlobSASSignatureValues{
		Protocol:      azblob.SASProtocolHTTPS,              // Users MUST use HTTPS (not HTTP)
		ExpiryTime:    time.Now().UTC().Add(48 * time.Hour), // 48-hours before expiration
		ContainerName: containerName,
		BlobName:      blobName,

		// To produce a container SAS (as opposed to a blob SAS), assign to Permissions using
		// ContainerSASPermissions and make sure the BlobName field is "" (the default).
		Permissions: azblob.BlobSASPermissions{Add: true, Read: true, Write: true}.String(),
	}.NewSASQueryParameters(credential)

	// Create the URL of the resource you wish to access and append the SAS query parameters.
	// Since this is a blob SAS, the URL is to the Azure storage blob.
	qp := sasQueryParams.Encode()
	urlToSendToSomeone := fmt.Sprintf("https://%s.blob.core.windows.net/%s/%s?%s",
		accountName, containerName, blobName, qp)
	// At this point, you can send the urlToSendToSomeone to someone via email or any other mechanism you choose.

	// ************************************************************************************************

	// When someone receives the URL, they access the SAS-protected resource with code like this:
	u, _ := url.Parse(urlToSendToSomeone)

	// Create an BlobURL object that wraps the blob URL (and its SAS) and a pipeline.
	// When using a SAS URLs, anonymous credentials are required.
	blobURL := azblob.NewBlobURL(*u, azblob.NewPipeline(azblob.NewAnonymousCredential(), azblob.PipelineOptions{}))
	// Now, you can use this blobURL just like any other to make requests of the resource.

	// If you have a SAS query parameter string, you can parse it into its parts:
	blobURLParts := azblob.NewBlobURLParts(blobURL.URL())
	fmt.Printf("SAS expiry time=%v", blobURLParts.SAS.ExpiryTime())

	_ = blobURL // Avoid compiler's "declared and not used" error
}

// This example shows how to manipulate a container's permissions.
func ExampleContainerURL_SetContainerAccessPolicy() {
	// From the Azure portal, get your Storage account's name and account key.
	accountName, accountKey := accountInfo()

	// Use your Storage account's name and key to create a credential object; this is used to access your account.
	credential := azblob.NewSharedKeyCredential(accountName, accountKey)

	// Create an ContainerURL object that wraps the container's URL and a default pipeline.
	u, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/mycontainer", accountName))
	containerURL := azblob.NewContainerURL(*u, azblob.NewPipeline(credential, azblob.PipelineOptions{}))

	// All operations allow you to specify a timeout via a Go context.Context object.
	ctx := context.Background() // This example uses a never-expiring context

	// Create the container (with no metadata and no public access)
	_, err := containerURL.Create(ctx, azblob.Metadata{}, azblob.PublicAccessNone)
	if err != nil {
		log.Fatal(err)
	}

	// Create a URL that references a to-be-created blob in your Azure Storage account's container.
	// This returns a BlockBlobURL object that wraps the blob's URL and a request pipeline (inherited from containerURL)
	blobURL := containerURL.NewBlockBlobURL("HelloWorld.txt") // Blob names can be mixed case

	// Create the blob and put some text in it
	_, err = blobURL.Upload(ctx, strings.NewReader("Hello World!"), azblob.BlobHTTPHeaders{ContentType: "text/plain"},
		azblob.Metadata{}, azblob.BlobAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}

	// Attempt to read the blob via a simple HTTP GET operation
	rawBlobURL := blobURL.URL()
	get, err := http.Get(rawBlobURL.String())
	if err != nil {
		log.Fatal(err)
	}
	if get.StatusCode == http.StatusNotFound {
		// We expected this error because the service returns an HTTP 404 status code when a blob
		// exists but the requester does not have permission to access it.
		// This is how we change the container's permission to allow public/anonymous aceess:
		_, err := containerURL.SetAccessPolicy(ctx, azblob.PublicAccessBlob, []azblob.SignedIdentifier{}, azblob.ContainerAccessConditions{})
		if err != nil {
			log.Fatal(err)
		}

		// Now, this works:
		get, err = http.Get(rawBlobURL.String())
		if err != nil {
			log.Fatal(err)
		}
		defer get.Body.Close()
		var text bytes.Buffer
		text.ReadFrom(get.Body)
		fmt.Print(text.String())
	}
}

// This example shows how to perform operations on blob conditionally.
func ExampleBlobAccessConditions() {
	// From the Azure portal, get your Storage account's name and account key.
	accountName, accountKey := accountInfo()

	// Create a BlockBlobURL object that wraps a blob's URL and a default pipeline.
	u, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/mycontainer/Data.txt", accountName))
	blobURL := azblob.NewBlockBlobURL(*u, azblob.NewPipeline(azblob.NewSharedKeyCredential(accountName, accountKey), azblob.PipelineOptions{}))

	ctx := context.Background() // This example uses a never-expiring context

	// This helper function displays the results of an operation; it is called frequently below.
	showResult := func(response pipeline.Response, err error) {
		if err != nil {
			if stgErr, ok := err.(azblob.StorageError); !ok {
				log.Fatal(err) // Network failure
			} else {
				fmt.Print("Failure: " + stgErr.Response().Status + "\n")
			}
		} else {
			if get, ok := response.(*azblob.DownloadResponse); ok {
				get.Body(azblob.RetryReaderOptions{}).Close() // The client must close the response body when finished with it
			}
			fmt.Print("Success: " + response.Response().Status + "\n")
		}
	}

	// Create the blob (unconditionally; succeeds)
	upload, err := blobURL.Upload(ctx, strings.NewReader("Text-1"), azblob.BlobHTTPHeaders{}, azblob.Metadata{}, azblob.BlobAccessConditions{})
	showResult(upload, err)

	// Download blob content if the blob has been modified since we uploaded it (fails):
	showResult(blobURL.Download(ctx, 0, 0,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfModifiedSince: upload.LastModified()}}, false))

	// Download blob content if the blob hasn't been modified in the last 24 hours (fails):
	showResult(blobURL.Download(ctx, 0, 0,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfUnmodifiedSince: time.Now().UTC().Add(time.Hour * -24)}}, false))

	// Upload new content if the blob hasn't changed since the version identified by ETag (succeeds):
	upload, err = blobURL.Upload(ctx, strings.NewReader("Text-2"), azblob.BlobHTTPHeaders{}, azblob.Metadata{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfMatch: upload.ETag()}})
	showResult(upload, err)

	// Download content if it has changed since the version identified by ETag (fails):
	showResult(blobURL.Download(ctx, 0, 0,
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: upload.ETag()}}, false))

	// Upload content if the blob doesn't already exist (fails):
	showResult(blobURL.Upload(ctx, strings.NewReader("Text-3"), azblob.BlobHTTPHeaders{}, azblob.Metadata{},
		azblob.BlobAccessConditions{HTTPAccessConditions: azblob.HTTPAccessConditions{IfNoneMatch: azblob.ETagAny}}))
}

// This examples shows how to create a container with metadata and then how to read & update the metadata.
func ExampleMetadata_containers() {
	// From the Azure portal, get your Storage account blob service URL endpoint.
	accountName, accountKey := accountInfo()

	// Create a ContainerURL object that wraps a soon-to-be-created container's URL and a default pipeline.
	u, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/mycontainer", accountName))
	containerURL := azblob.NewContainerURL(*u,
		azblob.NewPipeline(azblob.NewSharedKeyCredential(accountName, accountKey), azblob.PipelineOptions{}))

	ctx := context.Background() // This example uses a never-expiring context

	// Create a container with some metadata (string key/value pairs)
	// NOTE: Metadata key names are always converted to lowercase before being sent to the Storage Service.
	// Therefore, you should always use lowercase letters; especially when querying a map for a metadata key.
	creatingApp, _ := os.Executable()
	_, err := containerURL.Create(ctx, azblob.Metadata{"author": "Jeffrey", "app": creatingApp}, azblob.PublicAccessNone)
	if err != nil {
		log.Fatal(err)
	}

	// Query the container's metadata
	get, err := containerURL.GetProperties(ctx, azblob.LeaseAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}

	// Show the container's metadata
	metadata := get.NewMetadata()
	for k, v := range metadata {
		fmt.Print(k + "=" + v + "\n")
	}

	// Update the metadata and write it back to the container
	metadata["author"] = "Aidan" // NOTE: The keyname is in all lowercase letters
	_, err = containerURL.SetMetadata(ctx, metadata, azblob.ContainerAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}

	// NOTE: The SetMetadata & SetProperties methods update the container's ETag & LastModified properties
}

// This examples shows how to create a blob with metadata and then how to read & update
// the blob's read-only properties and metadata.
func ExampleMetadata_blobs() {
	// From the Azure portal, get your Storage account blob service URL endpoint.
	accountName, accountKey := accountInfo()

	// Create a ContainerURL object that wraps a soon-to-be-created blob's URL and a default pipeline.
	u, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/mycontainer/ReadMe.txt", accountName))
	blobURL := azblob.NewBlockBlobURL(*u,
		azblob.NewPipeline(azblob.NewSharedKeyCredential(accountName, accountKey), azblob.PipelineOptions{}))

	ctx := context.Background() // This example uses a never-expiring context

	// Create a blob with metadata (string key/value pairs)
	// NOTE: Metadata key names are always converted to lowercase before being sent to the Storage Service.
	// Therefore, you should always use lowercase letters; especially when querying a map for a metadata key.
	creatingApp, _ := os.Executable()
	_, err := blobURL.Upload(ctx, strings.NewReader("Some text"), azblob.BlobHTTPHeaders{},
		azblob.Metadata{"author": "Jeffrey", "app": creatingApp}, azblob.BlobAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}

	// Query the blob's properties and metadata
	get, err := blobURL.GetProperties(ctx, azblob.BlobAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}

	// Show some of the blob's read-only properties
	fmt.Println(get.BlobType(), get.ETag(), get.LastModified())

	// Show the blob's metadata
	metadata := get.NewMetadata()
	for k, v := range metadata {
		fmt.Print(k + "=" + v + "\n")
	}

	// Update the blob's metadata and write it back to the blob
	metadata["editor"] = "Grant" // Add a new key/value; NOTE: The keyname is in all lowercase letters
	_, err = blobURL.SetMetadata(ctx, metadata, azblob.BlobAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}

	// NOTE: The SetMetadata method updates the blob's ETag & LastModified properties
}

// This examples shows how to create a blob with HTTP Headers and then how to read & update
// the blob's HTTP headers.
func ExampleBlobHTTPHeaders() {
	// From the Azure portal, get your Storage account blob service URL endpoint.
	accountName, accountKey := accountInfo()

	// Create a ContainerURL object that wraps a soon-to-be-created blob's URL and a default pipeline.
	u, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/mycontainer/ReadMe.txt", accountName))
	blobURL := azblob.NewBlockBlobURL(*u,
		azblob.NewPipeline(azblob.NewSharedKeyCredential(accountName, accountKey), azblob.PipelineOptions{}))

	ctx := context.Background() // This example uses a never-expiring context

	// Create a blob with HTTP headers
	_, err := blobURL.Upload(ctx, strings.NewReader("Some text"),
		azblob.BlobHTTPHeaders{
			ContentType:        "text/html; charset=utf-8",
			ContentDisposition: "attachment",
		}, azblob.Metadata{}, azblob.BlobAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}

	// GetMetadata returns the blob's properties, HTTP headers, and metadata
	get, err := blobURL.GetProperties(ctx, azblob.BlobAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}

	// Show some of the blob's read-only properties
	fmt.Println(get.BlobType(), get.ETag(), get.LastModified())

	// Shows some of the blob's HTTP Headers
	httpHeaders := get.NewHTTPHeaders()
	fmt.Println(httpHeaders.ContentType, httpHeaders.ContentDisposition)

	// Update the blob's HTTP Headers and write them back to the blob
	httpHeaders.ContentType = "text/plain"
	_, err = blobURL.SetHTTPHeaders(ctx, httpHeaders, azblob.BlobAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}

	// NOTE: The SetMetadata method updates the blob's ETag & LastModified properties
}

// ExampleBlockBlobURL shows how to upload a lot of data (in blocks) to a blob.
// A block blob can have a maximum of 50,000 blocks; each block can have a maximum of 100MB.
// Therefore, the maximum size of a block blob is slightly more than 4.75 TB (100 MB X 50,000 blocks).
func ExampleBlockBlobURL() {
	// From the Azure portal, get your Storage account blob service URL endpoint.
	accountName, accountKey := accountInfo()

	// Create a ContainerURL object that wraps a soon-to-be-created blob's URL and a default pipeline.
	u, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/mycontainer/MyBlockBlob.txt", accountName))
	blobURL := azblob.NewBlockBlobURL(*u,
		azblob.NewPipeline(azblob.NewSharedKeyCredential(accountName, accountKey), azblob.PipelineOptions{}))

	ctx := context.Background() // This example uses a never-expiring context

	// These helper functions convert a binary block ID to a base-64 string and vice versa
	// NOTE: The blockID must be <= 64 bytes and ALL blockIDs for the block must be the same length
	blockIDBinaryToBase64 := func(blockID []byte) string { return base64.StdEncoding.EncodeToString(blockID) }
	blockIDBase64ToBinary := func(blockID string) []byte { binary, _ := base64.StdEncoding.DecodeString(blockID); return binary }

	// These helper functions convert an int block ID to a base-64 string and vice versa
	blockIDIntToBase64 := func(blockID int) string {
		binaryBlockID := (&[4]byte{})[:] // All block IDs are 4 bytes long
		binary.LittleEndian.PutUint32(binaryBlockID, uint32(blockID))
		return blockIDBinaryToBase64(binaryBlockID)
	}
	blockIDBase64ToInt := func(blockID string) int {
		blockIDBase64ToBinary(blockID)
		return int(binary.LittleEndian.Uint32(blockIDBase64ToBinary(blockID)))
	}

	// Upload 4 blocks to the blob (these blocks are tiny; they can be up to 100MB each)
	words := []string{"Azure ", "Storage ", "Block ", "Blob."}
	base64BlockIDs := make([]string, len(words)) // The collection of block IDs (base 64 strings)

	// Upload each block sequentially (one after the other); for better performance, you want to upload multiple blocks in parallel)
	for index, word := range words {
		// This example uses the index as the block ID; convert the index/ID into a base-64 encoded string as required by the service.
		// NOTE: Over the lifetime of a blob, all block IDs (before base 64 encoding) must be the same length (this example uses 4 byte block IDs).
		base64BlockIDs[index] = blockIDIntToBase64(index) // Some people use UUIDs for block IDs

		// Upload a block to this blob specifying the Block ID and its content (up to 100MB); this block is uncommitted.
		_, err := blobURL.StageBlock(ctx, base64BlockIDs[index], strings.NewReader(word), azblob.LeaseAccessConditions{})
		if err != nil {
			log.Fatal(err)
		}
	}

	// After all the blocks are uploaded, atomically commit them to the blob.
	_, err := blobURL.CommitBlockList(ctx, base64BlockIDs, azblob.BlobHTTPHeaders{}, azblob.Metadata{}, azblob.BlobAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}

	// For the blob, show each block (ID and size) that is a committed part of it.
	getBlock, err := blobURL.GetBlockList(ctx, azblob.BlockListAll, azblob.LeaseAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}
	for _, block := range getBlock.CommittedBlocks {
		fmt.Printf("Block ID=%d, Size=%d\n", blockIDBase64ToInt(block.Name), block.Size)
	}

	// Download the blob in its entirety; download operations do not take blocks into account.
	// NOTE: For really large blobs, downloading them like allocates a lot of memory.
	get, err := blobURL.Download(ctx, 0, 0, azblob.BlobAccessConditions{}, false)
	if err != nil {
		log.Fatal(err)
	}
	blobData := &bytes.Buffer{}
	reader := get.Body(azblob.RetryReaderOptions{})
	blobData.ReadFrom(reader)
	reader.Close() // The client must close the response body when finished with it
	fmt.Println(blobData)
}

// ExampleAppendBlobURL shows how to append data (in blocks) to an append blob.
// An append blob can have a maximum of 50,000 blocks; each block can have a maximum of 100MB.
// Therefore, the maximum size of an append blob is slightly more than 4.75 TB (100 MB X 50,000 blocks).
func ExampleAppendBlobURL() {
	// From the Azure portal, get your Storage account blob service URL endpoint.
	accountName, accountKey := accountInfo()

	// Create a ContainerURL object that wraps a soon-to-be-created blob's URL and a default pipeline.
	u, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/mycontainer/MyAppendBlob.txt", accountName))
	appendBlobURL := azblob.NewAppendBlobURL(*u, azblob.NewPipeline(azblob.NewSharedKeyCredential(accountName, accountKey), azblob.PipelineOptions{}))

	ctx := context.Background() // This example uses a never-expiring context
	_, err := appendBlobURL.Create(ctx, azblob.BlobHTTPHeaders{}, azblob.Metadata{}, azblob.BlobAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}

	for i := 0; i < 5; i++ { // Append 5 blocks to the append blob
		_, err := appendBlobURL.AppendBlock(ctx, strings.NewReader(fmt.Sprintf("Appending block #%d\n", i)), azblob.BlobAccessConditions{})
		if err != nil {
			log.Fatal(err)
		}
	}

	// Download the entire append blob's contents and show it.
	get, err := appendBlobURL.Download(ctx, 0, 0, azblob.BlobAccessConditions{}, false)
	if err != nil {
		log.Fatal(err)
	}
	b := bytes.Buffer{}
	reader := get.Body(azblob.RetryReaderOptions{})
	b.ReadFrom(reader)
	reader.Close() // The client must close the response body when finished with it
	fmt.Println(b.String())
}

// ExamplePageBlobURL shows how to manipulate a page blob with PageBlobURL.
// A page blob is a collection of 512-byte pages optimized for random read and write operations.
// The maximum size for a page blob is 8 TB.
func ExamplePageBlobURL() {
	// From the Azure portal, get your Storage account blob service URL endpoint.
	accountName, accountKey := accountInfo()

	// Create a ContainerURL object that wraps a soon-to-be-created blob's URL and a default pipeline.
	u, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/mycontainer/MyPageBlob.txt", accountName))
	blobURL := azblob.NewPageBlobURL(*u,
		azblob.NewPipeline(azblob.NewSharedKeyCredential(accountName, accountKey), azblob.PipelineOptions{}))

	ctx := context.Background() // This example uses a never-expiring context
	_, err := blobURL.Create(ctx, azblob.PageBlobPageBytes*4, 0, azblob.BlobHTTPHeaders{},
		azblob.Metadata{}, azblob.BlobAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}

	page := [azblob.PageBlobPageBytes]byte{}
	copy(page[:], "Page 0")
	_, err = blobURL.UploadPages(ctx, 0*azblob.PageBlobPageBytes, bytes.NewReader(page[:]), azblob.BlobAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}

	copy(page[:], "Page 1")
	_, err = blobURL.UploadPages(ctx, 2*azblob.PageBlobPageBytes, bytes.NewReader(page[:]), azblob.BlobAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}

	getPages, err := blobURL.GetPageRanges(ctx, 0*azblob.PageBlobPageBytes, 10*azblob.PageBlobPageBytes, azblob.BlobAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}
	for _, pr := range getPages.PageRange {
		fmt.Printf("Start=%d, End=%d\n", pr.Start, pr.End)
	}

	_, err = blobURL.ClearPages(ctx, 0*azblob.PageBlobPageBytes, 1*azblob.PageBlobPageBytes, azblob.BlobAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}

	getPages, err = blobURL.GetPageRanges(ctx, 0*azblob.PageBlobPageBytes, 10*azblob.PageBlobPageBytes, azblob.BlobAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}
	for _, pr := range getPages.PageRange {
		fmt.Printf("Start=%d, End=%d\n", pr.Start, pr.End)
	}

	get, err := blobURL.Download(ctx, 0, 0, azblob.BlobAccessConditions{}, false)
	if err != nil {
		log.Fatal(err)
	}
	blobData := &bytes.Buffer{}
	reader := get.Body(azblob.RetryReaderOptions{})
	blobData.ReadFrom(reader)
	reader.Close() // The client must close the response body when finished with it
	fmt.Printf("%#v", blobData.Bytes())
}

// This example show how to create a blob, take a snapshot of it, update the base blob,
// read from the blob snapshot, list blobs with their snapshots, and hot to delete blob snapshots.
func Example_blobSnapshots() {
	// From the Azure portal, get your Storage account blob service URL endpoint.
	accountName, accountKey := accountInfo()

	// Create a ContainerURL object to a container where we'll create a blob and its snapshot.
	u, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/mycontainer", accountName))
	containerURL := azblob.NewContainerURL(*u,
		azblob.NewPipeline(azblob.NewSharedKeyCredential(accountName, accountKey), azblob.PipelineOptions{}))

	// Create a BlockBlobURL object to a blob in the container.
	baseBlobURL := containerURL.NewBlockBlobURL("Original.txt")

	ctx := context.Background() // This example uses a never-expiring context

	// Create the original blob:
	_, err := baseBlobURL.Upload(ctx, strings.NewReader("Some text"), azblob.BlobHTTPHeaders{}, azblob.Metadata{}, azblob.BlobAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}

	// Create a snapshot of the original blob & save its timestamp:
	createSnapshot, err := baseBlobURL.CreateSnapshot(ctx, azblob.Metadata{}, azblob.BlobAccessConditions{})
	snapshot := createSnapshot.Snapshot()

	// Modify the original blob & show it:
	_, err = baseBlobURL.Upload(ctx, strings.NewReader("New text"), azblob.BlobHTTPHeaders{}, azblob.Metadata{}, azblob.BlobAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}

	get, err := baseBlobURL.Download(ctx, 0, 0, azblob.BlobAccessConditions{}, false)
	b := bytes.Buffer{}
	reader := get.Body(azblob.RetryReaderOptions{})
	b.ReadFrom(reader)
	reader.Close() // The client must close the response body when finished with it
	fmt.Println(b.String())

	// Show snapshot blob via original blob URI & snapshot time:
	snapshotBlobURL := baseBlobURL.WithSnapshot(snapshot)
	get, err = snapshotBlobURL.Download(ctx, 0, 0, azblob.BlobAccessConditions{}, false)
	b.Reset()
	reader = get.Body(azblob.RetryReaderOptions{})
	b.ReadFrom(reader)
	reader.Close() // The client must close the response body when finished with it
	fmt.Println(b.String())

	// FYI: You can get the base blob URL from one of its snapshot by passing "" to WithSnapshot:
	baseBlobURL = snapshotBlobURL.WithSnapshot("")

	// Show all blobs in the container with their snapshots:
	// List the blob(s) in our container; since a container may hold millions of blobs, this is done 1 segment at a time.
	for marker := (azblob.Marker{}); marker.NotDone(); { // The parens around Marker{} are required to avoid compiler error.
		// Get a result segment starting with the blob indicated by the current Marker.
		listBlobs, err := containerURL.ListBlobsFlatSegment(ctx, marker, azblob.ListBlobsSegmentOptions{
			Details: azblob.BlobListingDetails{Snapshots: true}})
		if err != nil {
			log.Fatal(err)
		}
		// IMPORTANT: ListBlobs returns the start of the next segment; you MUST use this to get
		// the next segment (after processing the current result segment).
		marker = listBlobs.NextMarker

		// Process the blobs returned in this result segment (if the segment is empty, the loop body won't execute)
		for _, blobInfo := range listBlobs.Segment.BlobItems {
			snaptime := "N/A"
			if blobInfo.Snapshot != "" {
				snaptime = blobInfo.Snapshot
			}
			fmt.Printf("Blob name: %s, Snapshot: %s\n", blobInfo.Name, snaptime)
		}
	}

	// Promote read-only snapshot to writable base blob:
	_, err = baseBlobURL.StartCopyFromURL(ctx, snapshotBlobURL.URL(), azblob.Metadata{}, azblob.BlobAccessConditions{}, azblob.BlobAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}

	// When calling Delete on a base blob:
	// DeleteSnapshotsOptionOnly deletes all the base blob's snapshots but not the base blob itself
	// DeleteSnapshotsOptionInclude deletes the base blob & all its snapshots.
	// DeleteSnapshotOptionNone produces an error if the base blob has any snapshots.
	_, err = baseBlobURL.Delete(ctx, azblob.DeleteSnapshotsOptionInclude, azblob.BlobAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}
}

func Example_progressUploadDownload() {
	// Create a request pipeline using your Storage account's name and account key.
	accountName, accountKey := accountInfo()
	credential := azblob.NewSharedKeyCredential(accountName, accountKey)
	p := azblob.NewPipeline(credential, azblob.PipelineOptions{})

	// From the Azure portal, get your Storage account blob service URL endpoint.
	cURL, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/mycontainer", accountName))

	// Create an ServiceURL object that wraps the service URL and a request pipeline to making requests.
	containerURL := azblob.NewContainerURL(*cURL, p)

	ctx := context.Background() // This example uses a never-expiring context
	// Here's how to create a blob with HTTP headers and metadata (I'm using the same metadata that was put on the container):
	blobURL := containerURL.NewBlockBlobURL("Data.bin")

	// requestBody is the stream of data to write
	requestBody := strings.NewReader("Some text to write")

	// Wrap the request body in a RequestBodyProgress and pass a callback function for progress reporting.
	_, err := blobURL.Upload(ctx,
		pipeline.NewRequestBodyProgress(requestBody, func(bytesTransferred int64) {
			fmt.Printf("Wrote %d of %d bytes.", bytesTransferred, requestBody.Len())
		}),
		azblob.BlobHTTPHeaders{
			ContentType:        "text/html; charset=utf-8",
			ContentDisposition: "attachment",
		}, azblob.Metadata{}, azblob.BlobAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}

	// Here's how to read the blob's data with progress reporting:
	get, err := blobURL.Download(ctx, 0, 0, azblob.BlobAccessConditions{}, false)
	if err != nil {
		log.Fatal(err)
	}

	// Wrap the response body in a ResponseBodyProgress and pass a callback function for progress reporting.
	responseBody := pipeline.NewResponseBodyProgress(get.Body(azblob.RetryReaderOptions{}),
		func(bytesTransferred int64) {
			fmt.Printf("Read %d of %d bytes.", bytesTransferred, get.ContentLength())
		})

	downloadedData := &bytes.Buffer{}
	downloadedData.ReadFrom(responseBody)
	responseBody.Close() // The client must close the response body when finished with it
	// The downloaded blob data is in downloadData's buffer
}

// This example shows how to copy a source document on the Internet to a blob.
func ExampleBlobURL_startCopy() {
	// From the Azure portal, get your Storage account blob service URL endpoint.
	accountName, accountKey := accountInfo()

	// Create a ContainerURL object to a container where we'll create a blob and its snapshot.
	// Create a BlockBlobURL object to a blob in the container.
	u, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/mycontainer/CopiedBlob.bin", accountName))
	blobURL := azblob.NewBlobURL(*u,
		azblob.NewPipeline(azblob.NewSharedKeyCredential(accountName, accountKey), azblob.PipelineOptions{}))

	ctx := context.Background() // This example uses a never-expiring context

	src, _ := url.Parse("https://cdn2.auth0.com/docs/media/addons/azure_blob.svg")
	startCopy, err := blobURL.StartCopyFromURL(ctx, *src, nil, azblob.BlobAccessConditions{}, azblob.BlobAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}

	copyID := startCopy.CopyID()
	copyStatus := startCopy.CopyStatus()
	for copyStatus == azblob.CopyStatusPending {
		time.Sleep(time.Second * 2)
		getMetadata, err := blobURL.GetProperties(ctx, azblob.BlobAccessConditions{})
		if err != nil {
			log.Fatal(err)
		}
		copyStatus = getMetadata.CopyStatus()
	}
	fmt.Printf("Copy from %s to %s: ID=%s, Status=%s\n", src.String(), blobURL, copyID, copyStatus)
}

// This example shows how to copy a large stream in blocks (chunks) to a block blob.
func ExampleUploadFileToBlockBlobAndDownloadItBack() {
	file, err := os.Open("BigFile.bin") // Open the file we want to upload
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	fileSize, err := file.Stat() // Get the size of the file (stream)
	if err != nil {
		log.Fatal(err)
	}

	// From the Azure portal, get your Storage account blob service URL endpoint.
	accountName, accountKey := accountInfo()

	// Create a BlockBlobURL object to a blob in the container (we assume the container already exists).
	u, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/mycontainer/BigBlockBlob.bin", accountName))
	blockBlobURL := azblob.NewBlockBlobURL(*u, azblob.NewPipeline(azblob.NewSharedKeyCredential(accountName, accountKey), azblob.PipelineOptions{}))

	ctx := context.Background() // This example uses a never-expiring context

	// Pass the Context, stream, stream size, block blob URL, and options to StreamToBlockBlob
	response, err := azblob.UploadFileToBlockBlob(ctx, file, blockBlobURL,
		azblob.UploadToBlockBlobOptions{
			// If Progress is non-nil, this function is called periodically as bytes are uploaded.
			Progress: func(bytesTransferred int64) {
				fmt.Printf("Uploaded %d of %d bytes.\n", bytesTransferred, fileSize.Size())
			},
		})
	if err != nil {
		log.Fatal(err)
	}
	_ = response // Avoid compiler's "declared and not used" error

	// Set up file to download the blob to
	destFileName := "BigFile-downloaded.bin"
	destFile, err := os.Create(destFileName)
	defer destFile.Close()

	// Perform download
	err = azblob.DownloadBlobToFile(context.Background(), blockBlobURL.BlobURL, 0, azblob.CountToEnd, azblob.BlobAccessConditions{}, destFile,
		azblob.DownloadFromBlobOptions{
			// If Progress is non-nil, this function is called periodically as bytes are uploaded.
			Progress: func(bytesTransferred int64) {
				fmt.Printf("Downloaded %d of %d bytes.\n", bytesTransferred, fileSize.Size())
			}})

	if err != nil {
		log.Fatal(err)
	}
}

// This example shows how to download a large stream with intelligent retries. Specifically, if
// the connection fails while reading, continuing to read from this stream initiates a new
// GetBlob call passing a range that starts from the last byte successfully read before the failure.
func ExampleBlobUrl_Download() {
	// From the Azure portal, get your Storage account blob service URL endpoint.
	accountName, accountKey := accountInfo()

	// Create a BlobURL object to a blob in the container (we assume the container & blob already exist).
	u, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/mycontainer/BigBlob.bin", accountName))
	blobURL := azblob.NewBlobURL(*u, azblob.NewPipeline(azblob.NewSharedKeyCredential(accountName, accountKey), azblob.PipelineOptions{}))

	contentLength := int64(0) // Used for progress reporting to report the total number of bytes being downloaded.

	// Download returns an intelligent retryable stream around a blob; it returns an io.ReadCloser.
	dr, err := blobURL.Download(context.TODO(), 0, -1, azblob.BlobAccessConditions{}, false)
	if err != nil {
		log.Fatal(err)
	}
	rs := dr.Body(azblob.RetryReaderOptions{})

	// NewResponseBodyProgress wraps the GetRetryStream with progress reporting; it returns an io.ReadCloser.
	stream := pipeline.NewResponseBodyProgress(rs,
		func(bytesTransferred int64) {
			fmt.Printf("Downloaded %d of %d bytes.\n", bytesTransferred, contentLength)
		})
	defer stream.Close() // The client must close the response body when finished with it

	file, err := os.Create("BigFile.bin") // Create the file to hold the downloaded blob contents.
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	written, err := io.Copy(file, stream) // Write to the file by reading from the blob (with intelligent retries).
	if err != nil {
		log.Fatal(err)
	}
	_ = written // Avoid compiler's "declared and not used" error
}

func ExampleUploadStreamToBlockBlob() {
	// From the Azure portal, get your Storage account blob service URL endpoint.
	accountName, accountKey := accountInfo()

	// Create a BlockBlobURL object to a blob in the container (we assume the container already exists).
	u, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/mycontainer/BigBlockBlob.bin", accountName))
	blockBlobURL := azblob.NewBlockBlobURL(*u, azblob.NewPipeline(azblob.NewSharedKeyCredential(accountName, accountKey), azblob.PipelineOptions{}))

	ctx := context.Background() // This example uses a never-expiring context

	// Create some data to test the upload stream
	blobSize := 8 * 1024 * 1024
	data := make([]byte, blobSize)
	rand.Read(data)

	// Perform UploadStreamToBlockBlob
	bufferSize := 2 * 1024 * 1024 // Configure the size of the rotating buffers that are used when uploading
	maxBuffers := 3               // Configure the number of rotating buffers that are used when uploading
	_, err := azblob.UploadStreamToBlockBlob(ctx, bytes.NewReader(data), blockBlobURL,
		azblob.UploadStreamToBlockBlobOptions{BufferSize: bufferSize, MaxBuffers: maxBuffers})

	// Verify that upload was successful
	if err != nil {
		log.Fatal(err)
	}
}

// This example shows how to perform various lease operations on a container.
// The same lease operations can be performed on individual blobs as well.
// A lease on a container prevents it from being deleted by others, while a lease on a blob
// protects it from both modifications and deletions.
func ExampleLeaseContainer() {
	// From the Azure portal, get your Storage account's name and account key.
	accountName, accountKey := accountInfo()

	// Use your Storage account's name and key to create a credential object; this is used to access your account.
	credential := azblob.NewSharedKeyCredential(accountName, accountKey)

	// Create an ContainerURL object that wraps the container's URL and a default pipeline.
	u, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/mycontainer", accountName))
	containerURL := azblob.NewContainerURL(*u, azblob.NewPipeline(credential, azblob.PipelineOptions{}))

	// All operations allow you to specify a timeout via a Go context.Context object.
	ctx := context.Background() // This example uses a never-expiring context

	// Now acquire a lease on the container.
	// You can choose to pass an empty string for proposed ID so that the service automatically assigns one for you.
	acquireLeaseResponse, err := containerURL.AcquireLease(ctx, "", 60, azblob.HTTPAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("The container is leased for delete operations with lease ID", acquireLeaseResponse.LeaseID())

	// The container cannot be deleted without providing the lease ID.
	_, err = containerURL.Delete(ctx, azblob.ContainerAccessConditions{})
	if err == nil {
		log.Fatal("delete should have failed")
	}
	fmt.Println("The container cannot be deleted while there is an active lease")

	// We can release the lease now and the container can be deleted.
	_, err = containerURL.ReleaseLease(ctx, acquireLeaseResponse.LeaseID(), azblob.HTTPAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("The lease on the container is now released")

	// Acquire a lease again to perform other operations.
	acquireLeaseResponse, err = containerURL.AcquireLease(ctx, "", 60, azblob.HTTPAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("The container is leased again with lease ID", acquireLeaseResponse.LeaseID())

	// We can change the ID of an existing lease.
	// A lease ID can be any valid GUID string format.
	newLeaseID := uuid{}
	newLeaseID[0] = 1
	changeLeaseResponse, err := containerURL.ChangeLease(ctx, acquireLeaseResponse.LeaseID(), newLeaseID.String(), azblob.HTTPAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("The lease ID was changed to", changeLeaseResponse.LeaseID())

	// The lease can be renewed.
	renewLeaseResponse, err := containerURL.RenewLease(ctx, changeLeaseResponse.LeaseID(), azblob.HTTPAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("The lease was renewed with the same ID", renewLeaseResponse.LeaseID())

	// Finally, the lease can be broken and we could prevent others from acquiring a lease for a period of time
	_, err = containerURL.BreakLease(ctx, 60, azblob.HTTPAccessConditions{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("The lease was borken, and nobody can acquire a lease for 60 seconds")
}

// This example shows how to list blobs with hierarchy, by using a delimiter.
func ExampleListBlobsHierarchy() {
	// From the Azure portal, get your Storage account's name and account key.
	accountName, accountKey := accountInfo()

	// Use your Storage account's name and key to create a credential object; this is used to access your account.
	credential := azblob.NewSharedKeyCredential(accountName, accountKey)

	// Create an ContainerURL object that wraps the container's URL and a default pipeline.
	u, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/mycontainer", accountName))
	containerURL := azblob.NewContainerURL(*u, azblob.NewPipeline(credential, azblob.PipelineOptions{}))

	// All operations allow you to specify a timeout via a Go context.Context object.
	ctx := context.Background() // This example uses a never-expiring context

	// Create 4 blobs: 3 of which have a virtual directory
	blobNames := []string{"a/1", "a/2", "b/1", "boaty_mcboatface"}
	for _, blobName := range blobNames {
		blobURL := containerURL.NewBlockBlobURL(blobName)
		_, err := blobURL.Upload(ctx, strings.NewReader("test"), azblob.BlobHTTPHeaders{}, nil, azblob.BlobAccessConditions{})

		if err != nil {
			log.Fatal("an error occurred while creating blobs for the example setup")
		}
	}

	// Perform a listing operation on blobs with hierarchy
	resp, err := containerURL.ListBlobsHierarchySegment(ctx, azblob.Marker{}, "/", azblob.ListBlobsSegmentOptions{})
	if err != nil {
		log.Fatal("an error occurred while listing blobs")
	}

	// When a delimiter is used, the listing operation returns BlobPrefix elements that acts as
	// a placeholder for all blobs whose names begin with the same substring up to the appearance of the delimiter character.
	// In our example, this means that a/ and b/ will be both returned
	fmt.Println("======First listing=====")
	for _, blobPrefix := range resp.Segment.BlobPrefixes {
		fmt.Println("The blob prefix with name", blobPrefix.Name, "was returned in the listing operation")
	}

	// The blobs that do not contain the delimiter are still returned
	for _, blob := range resp.Segment.BlobItems {
		fmt.Println("The blob with name", blob.Name, "was returned in the listing operation")
	}

	// For the prefixes that are returned, we can perform another listing operation on them, to see their contents
	resp, err = containerURL.ListBlobsHierarchySegment(ctx, azblob.Marker{}, "/", azblob.ListBlobsSegmentOptions{
		Prefix: "a/",
	})
	if err != nil {
		log.Fatal("an error occurred while listing blobs")
	}

	// This time, there is no blob prefix returned, since nothing under a/ has another / in its name.
	// In other words, in the virtual directory of a/, there aren't any sub-level virtual directory.
	fmt.Println("======Second listing=====")
	fmt.Println("No prefiex should be returned now, and the actual count is", len(resp.Segment.BlobPrefixes))

	// The blobs a/1 and a/2 should be returned
	for _, blob := range resp.Segment.BlobItems {
		fmt.Println("The blob with name", blob.Name, "was returned in the listing operation")
	}

	// Delete the blobs created by this example
	for _, blobName := range blobNames {
		blobURL := containerURL.NewBlockBlobURL(blobName)
		_, err := blobURL.Delete(ctx, azblob.DeleteSnapshotsOptionNone, azblob.BlobAccessConditions{})

		if err != nil {
			log.Fatal("an error occurred while deleting the blobs created by the example")
		}
	}
}
