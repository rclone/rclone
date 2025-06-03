// Package googlecloudstorage provides an interface to Google Cloud Storage
package googlecloudstorage

/*
Notes

Can't set Updated but can set Metadata on object creation

Patch needs full_control not just read_write

FIXME Patch/Delete/Get isn't working with files with spaces in - giving 404 error
- https://code.google.com/p/google-api-go-client/issues/detail?id=64
*/

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/list"
	"github.com/rclone/rclone/lib/bucket"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/env"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/pacer"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/googleapi"
	option "google.golang.org/api/option"

	// NOTE: This API is deprecated
	storage "google.golang.org/api/storage/v1"
)

const (
	rcloneClientID              = "202264815644.apps.googleusercontent.com"
	rcloneEncryptedClientSecret = "Uj7C9jGfb9gmeaV70Lh058cNkWvepr-Es9sBm0zdgil7JaOWF1VySw"
	timeFormat                  = time.RFC3339Nano
	metaMtime                   = "mtime"                    // key to store mtime in metadata
	metaMtimeGsutil             = "goog-reserved-file-mtime" // key used by GSUtil to store mtime in metadata
	listChunks                  = 1000                       // chunk size to read directory listings
	minSleep                    = 10 * time.Millisecond
)

var (
	// Description of how to auth for this app
	storageConfig = &oauthutil.Config{
		Scopes:       []string{storage.DevstorageReadWriteScope},
		AuthURL:      google.Endpoint.AuthURL,
		TokenURL:     google.Endpoint.TokenURL,
		ClientID:     rcloneClientID,
		ClientSecret: obscure.MustReveal(rcloneEncryptedClientSecret),
		RedirectURL:  oauthutil.RedirectURL,
	}
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "google cloud storage",
		Prefix:      "gcs",
		Description: "Google Cloud Storage (this is not Google Drive)",
		NewFs:       NewFs,
		Config: func(ctx context.Context, name string, m configmap.Mapper, config fs.ConfigIn) (*fs.ConfigOut, error) {
			saFile, _ := m.Get("service_account_file")
			saCreds, _ := m.Get("service_account_credentials")
			anonymous, _ := m.Get("anonymous")
			envAuth, _ := m.Get("env_auth")
			if saFile != "" || saCreds != "" || anonymous == "true" || envAuth == "true" {
				return nil, nil
			}
			return oauthutil.ConfigOut("", &oauthutil.Options{
				OAuth2Config: storageConfig,
			})
		},
		Options: append(oauthutil.SharedOptions, []fs.Option{{
			Name:      "project_number",
			Help:      "Project number.\n\nOptional - needed only for list/create/delete buckets - see your developer console.",
			Sensitive: true,
		}, {
			Name:      "user_project",
			Help:      "User project.\n\nOptional - needed only for requester pays.",
			Sensitive: true,
		}, {
			Name: "service_account_file",
			Help: "Service Account Credentials JSON file path.\n\nLeave blank normally.\nNeeded only if you want use SA instead of interactive login." + env.ShellExpandHelp,
		}, {
			Name:      "service_account_credentials",
			Help:      "Service Account Credentials JSON blob.\n\nLeave blank normally.\nNeeded only if you want use SA instead of interactive login.",
			Hide:      fs.OptionHideBoth,
			Sensitive: true,
		}, {
			Name:      "access_token",
			Help:      "Short-lived access token.\n\nLeave blank normally.\nNeeded only if you want use short-lived access token instead of interactive login.",
			Hide:      fs.OptionHideConfigurator,
			Sensitive: true,
			Advanced:  true,
		}, {
			Name:    "anonymous",
			Help:    "Access public buckets and objects without credentials.\n\nSet to 'true' if you just want to download files and don't configure credentials.",
			Default: false,
		}, {
			Name: "object_acl",
			Help: "Access Control List for new objects.",
			Examples: []fs.OptionExample{{
				Value: "authenticatedRead",
				Help:  "Object owner gets OWNER access.\nAll Authenticated Users get READER access.",
			}, {
				Value: "bucketOwnerFullControl",
				Help:  "Object owner gets OWNER access.\nProject team owners get OWNER access.",
			}, {
				Value: "bucketOwnerRead",
				Help:  "Object owner gets OWNER access.\nProject team owners get READER access.",
			}, {
				Value: "private",
				Help:  "Object owner gets OWNER access.\nDefault if left blank.",
			}, {
				Value: "projectPrivate",
				Help:  "Object owner gets OWNER access.\nProject team members get access according to their roles.",
			}, {
				Value: "publicRead",
				Help:  "Object owner gets OWNER access.\nAll Users get READER access.",
			}},
		}, {
			Name: "bucket_acl",
			Help: "Access Control List for new buckets.",
			Examples: []fs.OptionExample{{
				Value: "authenticatedRead",
				Help:  "Project team owners get OWNER access.\nAll Authenticated Users get READER access.",
			}, {
				Value: "private",
				Help:  "Project team owners get OWNER access.\nDefault if left blank.",
			}, {
				Value: "projectPrivate",
				Help:  "Project team members get access according to their roles.",
			}, {
				Value: "publicRead",
				Help:  "Project team owners get OWNER access.\nAll Users get READER access.",
			}, {
				Value: "publicReadWrite",
				Help:  "Project team owners get OWNER access.\nAll Users get WRITER access.",
			}},
		}, {
			Name: "bucket_policy_only",
			Help: `Access checks should use bucket-level IAM policies.

If you want to upload objects to a bucket with Bucket Policy Only set
then you will need to set this.

When it is set, rclone:

- ignores ACLs set on buckets
- ignores ACLs set on objects
- creates buckets with Bucket Policy Only set

Docs: https://cloud.google.com/storage/docs/bucket-policy-only
`,
			Default: false,
		}, {
			Name: "location",
			Help: "Location for the newly created buckets.",
			Examples: []fs.OptionExample{{
				Value: "",
				Help:  "Empty for default location (US)",
			}, {
				Value: "asia",
				Help:  "Multi-regional location for Asia",
			}, {
				Value: "eu",
				Help:  "Multi-regional location for Europe",
			}, {
				Value: "us",
				Help:  "Multi-regional location for United States",
			}, {
				Value: "asia-east1",
				Help:  "Taiwan",
			}, {
				Value: "asia-east2",
				Help:  "Hong Kong",
			}, {
				Value: "asia-northeast1",
				Help:  "Tokyo",
			}, {
				Value: "asia-northeast2",
				Help:  "Osaka",
			}, {
				Value: "asia-northeast3",
				Help:  "Seoul",
			}, {
				Value: "asia-south1",
				Help:  "Mumbai",
			}, {
				Value: "asia-south2",
				Help:  "Delhi",
			}, {
				Value: "asia-southeast1",
				Help:  "Singapore",
			}, {
				Value: "asia-southeast2",
				Help:  "Jakarta",
			}, {
				Value: "australia-southeast1",
				Help:  "Sydney",
			}, {
				Value: "australia-southeast2",
				Help:  "Melbourne",
			}, {
				Value: "europe-north1",
				Help:  "Finland",
			}, {
				Value: "europe-west1",
				Help:  "Belgium",
			}, {
				Value: "europe-west2",
				Help:  "London",
			}, {
				Value: "europe-west3",
				Help:  "Frankfurt",
			}, {
				Value: "europe-west4",
				Help:  "Netherlands",
			}, {
				Value: "europe-west6",
				Help:  "Zürich",
			}, {
				Value: "europe-central2",
				Help:  "Warsaw",
			}, {
				Value: "us-central1",
				Help:  "Iowa",
			}, {
				Value: "us-east1",
				Help:  "South Carolina",
			}, {
				Value: "us-east4",
				Help:  "Northern Virginia",
			}, {
				Value: "us-west1",
				Help:  "Oregon",
			}, {
				Value: "us-west2",
				Help:  "California",
			}, {
				Value: "us-west3",
				Help:  "Salt Lake City",
			}, {
				Value: "us-west4",
				Help:  "Las Vegas",
			}, {
				Value: "northamerica-northeast1",
				Help:  "Montréal",
			}, {
				Value: "northamerica-northeast2",
				Help:  "Toronto",
			}, {
				Value: "southamerica-east1",
				Help:  "São Paulo",
			}, {
				Value: "southamerica-west1",
				Help:  "Santiago",
			}, {
				Value: "asia1",
				Help:  "Dual region: asia-northeast1 and asia-northeast2.",
			}, {
				Value: "eur4",
				Help:  "Dual region: europe-north1 and europe-west4.",
			}, {
				Value: "nam4",
				Help:  "Dual region: us-central1 and us-east1.",
			}},
		}, {
			Name: "storage_class",
			Help: "The storage class to use when storing objects in Google Cloud Storage.",
			Examples: []fs.OptionExample{{
				Value: "",
				Help:  "Default",
			}, {
				Value: "MULTI_REGIONAL",
				Help:  "Multi-regional storage class",
			}, {
				Value: "REGIONAL",
				Help:  "Regional storage class",
			}, {
				Value: "NEARLINE",
				Help:  "Nearline storage class",
			}, {
				Value: "COLDLINE",
				Help:  "Coldline storage class",
			}, {
				Value: "ARCHIVE",
				Help:  "Archive storage class",
			}, {
				Value: "DURABLE_REDUCED_AVAILABILITY",
				Help:  "Durable reduced availability storage class",
			}},
		}, {
			Name:     "directory_markers",
			Default:  false,
			Advanced: true,
			Help: `Upload an empty object with a trailing slash when a new directory is created

Empty folders are unsupported for bucket based remotes, this option creates an empty
object ending with "/", to persist the folder.
`,
		}, {
			Name: "no_check_bucket",
			Help: `If set, don't attempt to check the bucket exists or create it.

This can be useful when trying to minimise the number of transactions
rclone does if you know the bucket exists already.
`,
			Default:  false,
			Advanced: true,
		}, {
			Name: "decompress",
			Help: `If set this will decompress gzip encoded objects.

It is possible to upload objects to GCS with "Content-Encoding: gzip"
set. Normally rclone will download these files as compressed objects.

If this flag is set then rclone will decompress these files with
"Content-Encoding: gzip" as they are received. This means that rclone
can't check the size and hash but the file contents will be decompressed.
`,
			Advanced: true,
			Default:  false,
		}, {
			Name:     "endpoint",
			Help:     "Endpoint for the service.\n\nLeave blank normally.",
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: (encoder.Base |
				encoder.EncodeCrLf |
				encoder.EncodeInvalidUtf8),
		}, {
			Name:    "env_auth",
			Help:    "Get GCP IAM credentials from runtime (environment variables or instance meta data if no env vars).\n\nOnly applies if service_account_file and service_account_credentials is blank.",
			Default: false,
			Examples: []fs.OptionExample{{
				Value: "false",
				Help:  "Enter credentials in the next step.",
			}, {
				Value: "true",
				Help:  "Get GCP IAM credentials from the environment (env vars or IAM).",
			}},
		}}...),
	})
}

// Options defines the configuration for this backend
type Options struct {
	ProjectNumber             string               `config:"project_number"`
	UserProject               string               `config:"user_project"`
	ServiceAccountFile        string               `config:"service_account_file"`
	ServiceAccountCredentials string               `config:"service_account_credentials"`
	Anonymous                 bool                 `config:"anonymous"`
	ObjectACL                 string               `config:"object_acl"`
	BucketACL                 string               `config:"bucket_acl"`
	BucketPolicyOnly          bool                 `config:"bucket_policy_only"`
	Location                  string               `config:"location"`
	StorageClass              string               `config:"storage_class"`
	NoCheckBucket             bool                 `config:"no_check_bucket"`
	Decompress                bool                 `config:"decompress"`
	Endpoint                  string               `config:"endpoint"`
	Enc                       encoder.MultiEncoder `config:"encoding"`
	EnvAuth                   bool                 `config:"env_auth"`
	DirectoryMarkers          bool                 `config:"directory_markers"`
	AccessToken               string               `config:"access_token"`
}

// Fs represents a remote storage server
type Fs struct {
	name           string           // name of this remote
	root           string           // the path we are working on if any
	opt            Options          // parsed options
	features       *fs.Features     // optional features
	svc            *storage.Service // the connection to the storage server
	client         *http.Client     // authorized client
	rootBucket     string           // bucket part of root (if any)
	rootDirectory  string           // directory part of root (if any)
	cache          *bucket.Cache    // cache of bucket status
	pacer          *fs.Pacer        // To pace the API calls
	warnCompressed sync.Once        // warn once about compressed files
}

// Object describes a storage object
//
// Will definitely have info but maybe not meta
type Object struct {
	fs       *Fs       // what this object is part of
	remote   string    // The remote path
	url      string    // download path
	md5sum   string    // The MD5Sum of the object
	bytes    int64     // Bytes in the object
	modTime  time.Time // Modified time of the object
	mimeType string
	gzipped  bool // set if object has Content-Encoding: gzip
}

// ------------------------------------------------------------

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
	if f.rootBucket == "" {
		return "GCS root"
	}
	if f.rootDirectory == "" {
		return fmt.Sprintf("GCS bucket %s", f.rootBucket)
	}
	return fmt.Sprintf("GCS bucket %s path %s", f.rootBucket, f.rootDirectory)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// shouldRetry determines whether a given err rates being retried
func shouldRetry(ctx context.Context, err error) (again bool, errOut error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	again = false
	if err != nil {
		if fserrors.ShouldRetry(err) {
			again = true
		} else {
			switch gerr := err.(type) {
			case *googleapi.Error:
				if gerr.Code >= 500 && gerr.Code < 600 {
					// All 5xx errors should be retried
					again = true
				} else if len(gerr.Errors) > 0 {
					reason := gerr.Errors[0].Reason
					if reason == "rateLimitExceeded" || reason == "userRateLimitExceeded" {
						again = true
					}
				}
			}
		}
	}
	return again, err
}

// parsePath parses a remote 'url'
func parsePath(path string) (root string) {
	root = strings.Trim(path, "/")
	return
}

// split returns bucket and bucketPath from the rootRelativePath
// relative to f.root
func (f *Fs) split(rootRelativePath string) (bucketName, bucketPath string) {
	bucketName, bucketPath = bucket.Split(bucket.Join(f.root, rootRelativePath))
	return f.opt.Enc.FromStandardName(bucketName), f.opt.Enc.FromStandardPath(bucketPath)
}

// split returns bucket and bucketPath from the object
func (o *Object) split() (bucket, bucketPath string) {
	return o.fs.split(o.remote)
}

func getServiceAccountClient(ctx context.Context, credentialsData []byte) (*http.Client, error) {
	conf, err := google.JWTConfigFromJSON(credentialsData, storageConfig.Scopes...)
	if err != nil {
		return nil, fmt.Errorf("error processing credentials: %w", err)
	}
	ctxWithSpecialClient := oauthutil.Context(ctx, fshttp.NewClient(ctx))
	return oauth2.NewClient(ctxWithSpecialClient, conf.TokenSource(ctxWithSpecialClient)), nil
}

// setRoot changes the root of the Fs
func (f *Fs) setRoot(root string) {
	f.root = parsePath(root)
	f.rootBucket, f.rootDirectory = bucket.Split(f.root)
}

// NewFs constructs an Fs from the path, bucket:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	var oAuthClient *http.Client

	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	if opt.ObjectACL == "" {
		opt.ObjectACL = "private"
	}
	if opt.BucketACL == "" {
		opt.BucketACL = "private"
	}

	// try loading service account credentials from env variable, then from a file
	if opt.ServiceAccountCredentials == "" && opt.ServiceAccountFile != "" {
		loadedCreds, err := os.ReadFile(env.ShellExpand(opt.ServiceAccountFile))
		if err != nil {
			return nil, fmt.Errorf("error opening service account credentials file: %w", err)
		}
		opt.ServiceAccountCredentials = string(loadedCreds)
	}
	if opt.Anonymous {
		oAuthClient = fshttp.NewClient(ctx)
	} else if opt.ServiceAccountCredentials != "" {
		oAuthClient, err = getServiceAccountClient(ctx, []byte(opt.ServiceAccountCredentials))
		if err != nil {
			return nil, fmt.Errorf("failed configuring Google Cloud Storage Service Account: %w", err)
		}
	} else if opt.EnvAuth {
		oAuthClient, err = google.DefaultClient(ctx, storage.DevstorageFullControlScope)
		if err != nil {
			return nil, fmt.Errorf("failed to configure Google Cloud Storage: %w", err)
		}
	} else if opt.AccessToken != "" {
		ts := oauth2.Token{AccessToken: opt.AccessToken}
		oAuthClient = oauth2.NewClient(ctx, oauth2.StaticTokenSource(&ts))
	} else {
		oAuthClient, _, err = oauthutil.NewClient(ctx, name, m, storageConfig)
		if err != nil {
			ctx := context.Background()
			oAuthClient, err = google.DefaultClient(ctx, storage.DevstorageFullControlScope)
			if err != nil {
				return nil, fmt.Errorf("failed to configure Google Cloud Storage: %w", err)
			}
		}
	}

	f := &Fs{
		name:  name,
		root:  root,
		opt:   *opt,
		pacer: fs.NewPacer(ctx, pacer.NewS3(pacer.MinSleep(minSleep))),
		cache: bucket.NewCache(),
	}
	f.setRoot(root)
	f.features = (&fs.Features{
		ReadMimeType:      true,
		WriteMimeType:     true,
		BucketBased:       true,
		BucketBasedRootOK: true,
	}).Fill(ctx, f)
	if opt.DirectoryMarkers {
		f.features.CanHaveEmptyDirectories = true
	}

	// Create a new authorized Drive client.
	f.client = oAuthClient
	gcsOpts := []option.ClientOption{option.WithHTTPClient(f.client)}
	if opt.Endpoint != "" {
		gcsOpts = append(gcsOpts, option.WithEndpoint(opt.Endpoint))
	}
	f.svc, err = storage.NewService(context.Background(), gcsOpts...)
	if err != nil {
		return nil, fmt.Errorf("couldn't create Google Cloud Storage client: %w", err)
	}

	if f.rootBucket != "" && f.rootDirectory != "" {
		// Check to see if the object exists
		encodedDirectory := f.opt.Enc.FromStandardPath(f.rootDirectory)
		err = f.pacer.Call(func() (bool, error) {
			get := f.svc.Objects.Get(f.rootBucket, encodedDirectory).Context(ctx)
			if f.opt.UserProject != "" {
				get = get.UserProject(f.opt.UserProject)
			}
			_, err = get.Do()
			return shouldRetry(ctx, err)
		})
		if err == nil {
			newRoot := path.Dir(f.root)
			if newRoot == "." {
				newRoot = ""
			}
			f.setRoot(newRoot)
			// return an error with an fs which points to the parent
			return f, fs.ErrorIsFile
		}
	}
	return f, nil
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *storage.Object) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	if info != nil {
		o.setMetaData(info)
	} else {
		err := o.readMetaData(ctx) // reads info and meta, returning an error
		if err != nil {
			return nil, err
		}
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(ctx, remote, nil)
}

// listFn is called from list to handle an object.
type listFn func(remote string, object *storage.Object, isDirectory bool) error

// list the objects into the function supplied
//
// dir is the starting directory, "" for root
//
// Set recurse to read sub directories.
//
// The remote has prefix removed from it and if addBucket is set
// then it adds the bucket to the start.
func (f *Fs) list(ctx context.Context, bucket, directory, prefix string, addBucket bool, recurse bool, fn listFn) (err error) {
	if prefix != "" {
		prefix += "/"
	}
	if directory != "" {
		directory += "/"
	}
	list := f.svc.Objects.List(bucket).Prefix(directory).MaxResults(listChunks)
	if f.opt.UserProject != "" {
		list = list.UserProject(f.opt.UserProject)
	}
	if !recurse {
		list = list.Delimiter("/")
	}
	foundItems := 0
	for {
		var objects *storage.Objects
		err = f.pacer.Call(func() (bool, error) {
			objects, err = list.Context(ctx).Do()
			return shouldRetry(ctx, err)
		})
		if err != nil {
			if gErr, ok := err.(*googleapi.Error); ok {
				if gErr.Code == http.StatusNotFound {
					err = fs.ErrorDirNotFound
				}
			}
			return err
		}
		if !recurse {
			foundItems += len(objects.Prefixes)
			var object storage.Object
			for _, remote := range objects.Prefixes {
				if !strings.HasSuffix(remote, "/") {
					continue
				}
				remote = f.opt.Enc.ToStandardPath(remote)
				if !strings.HasPrefix(remote, prefix) {
					fs.Logf(f, "Odd name received %q", remote)
					continue
				}
				remote = remote[len(prefix) : len(remote)-1]
				if addBucket {
					remote = path.Join(bucket, remote)
				}
				err = fn(remote, &object, true)
				if err != nil {
					return err
				}
			}
		}
		foundItems += len(objects.Items)
		for _, object := range objects.Items {
			remote := f.opt.Enc.ToStandardPath(object.Name)
			if !strings.HasPrefix(remote, prefix) {
				fs.Logf(f, "Odd name received %q", object.Name)
				continue
			}
			isDirectory := remote == "" || strings.HasSuffix(remote, "/")
			// is this a directory marker?
			if isDirectory {
				// Don't insert the root directory
				if remote == f.opt.Enc.ToStandardPath(directory) {
					continue
				}
				// process directory markers as directories
				remote = strings.TrimRight(remote, "/")
			}
			remote = remote[len(prefix):]
			if addBucket {
				remote = path.Join(bucket, remote)
			}

			err = fn(remote, object, isDirectory)
			if err != nil {
				return err
			}
		}
		if objects.NextPageToken == "" {
			break
		}
		list.PageToken(objects.NextPageToken)
	}
	if f.opt.DirectoryMarkers && foundItems == 0 && directory != "" {
		// Determine whether the directory exists or not by whether it has a marker
		_, err := f.readObjectInfo(ctx, bucket, directory)
		if err != nil {
			if err == fs.ErrorObjectNotFound {
				return fs.ErrorDirNotFound
			}
			return err
		}
	}

	return nil
}

// Convert a list item into a DirEntry
func (f *Fs) itemToDirEntry(ctx context.Context, remote string, object *storage.Object, isDirectory bool) (fs.DirEntry, error) {
	if isDirectory {
		d := fs.NewDir(remote, time.Time{}).SetSize(int64(object.Size))
		return d, nil
	}
	o, err := f.newObjectWithInfo(ctx, remote, object)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// listDir lists a single directory
func (f *Fs) listDir(ctx context.Context, bucket, directory, prefix string, addBucket bool) (entries fs.DirEntries, err error) {
	// List the objects
	err = f.list(ctx, bucket, directory, prefix, addBucket, false, func(remote string, object *storage.Object, isDirectory bool) error {
		entry, err := f.itemToDirEntry(ctx, remote, object, isDirectory)
		if err != nil {
			return err
		}
		if entry != nil {
			entries = append(entries, entry)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// bucket must be present if listing succeeded
	f.cache.MarkOK(bucket)
	return entries, err
}

// listBuckets lists the buckets
func (f *Fs) listBuckets(ctx context.Context) (entries fs.DirEntries, err error) {
	if f.opt.ProjectNumber == "" {
		return nil, errors.New("can't list buckets without project number")
	}
	listBuckets := f.svc.Buckets.List(f.opt.ProjectNumber).MaxResults(listChunks)
	if f.opt.UserProject != "" {
		listBuckets = listBuckets.UserProject(f.opt.UserProject)
	}
	for {
		var buckets *storage.Buckets
		err = f.pacer.Call(func() (bool, error) {
			buckets, err = listBuckets.Context(ctx).Do()
			return shouldRetry(ctx, err)
		})
		if err != nil {
			return nil, err
		}
		for _, bucket := range buckets.Items {
			d := fs.NewDir(f.opt.Enc.ToStandardName(bucket.Name), time.Time{})
			entries = append(entries, d)
		}
		if buckets.NextPageToken == "" {
			break
		}
		listBuckets.PageToken(buckets.NextPageToken)
	}
	return entries, nil
}

// List the objects and directories in dir into entries.  The
// entries can be returned in any order but should be for a
// complete directory.
//
// dir should be "" to list the root, and should not have
// trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	bucket, directory := f.split(dir)
	if bucket == "" {
		if directory != "" {
			return nil, fs.ErrorListBucketRequired
		}
		return f.listBuckets(ctx)
	}
	return f.listDir(ctx, bucket, directory, f.rootDirectory, f.rootBucket == "")
}

// ListR lists the objects and directories of the Fs starting
// from dir recursively into out.
//
// dir should be "" to start from the root, and should not
// have trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
//
// It should call callback for each tranche of entries read.
// These need not be returned in any particular order.  If
// callback returns an error then the listing will stop
// immediately.
//
// Don't implement this unless you have a more efficient way
// of listing recursively that doing a directory traversal.
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) (err error) {
	bucket, directory := f.split(dir)
	list := list.NewHelper(callback)
	listR := func(bucket, directory, prefix string, addBucket bool) error {
		return f.list(ctx, bucket, directory, prefix, addBucket, true, func(remote string, object *storage.Object, isDirectory bool) error {
			entry, err := f.itemToDirEntry(ctx, remote, object, isDirectory)
			if err != nil {
				return err
			}
			return list.Add(entry)
		})
	}
	if bucket == "" {
		entries, err := f.listBuckets(ctx)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			err = list.Add(entry)
			if err != nil {
				return err
			}
			bucket := entry.Remote()
			err = listR(bucket, "", f.rootDirectory, true)
			if err != nil {
				return err
			}
			// bucket must be present if listing succeeded
			f.cache.MarkOK(bucket)
		}
	} else {
		err = listR(bucket, directory, f.rootDirectory, f.rootBucket == "")
		if err != nil {
			return err
		}
		// bucket must be present if listing succeeded
		f.cache.MarkOK(bucket)
	}
	return list.Flush()
}

// Put the object into the bucket
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// Temporary Object under construction
	o := &Object{
		fs:     f,
		remote: src.Remote(),
	}
	return o, o.Update(ctx, in, src, options...)
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// Create directory marker file and parents
func (f *Fs) createDirectoryMarker(ctx context.Context, bucket, dir string) error {
	if !f.opt.DirectoryMarkers || bucket == "" {
		return nil
	}

	// Object to be uploaded
	o := &Object{
		fs:      f,
		modTime: time.Now(),
	}

	for {
		_, bucketPath := f.split(dir)
		// Don't create the directory marker if it is the bucket or at the very root
		if bucketPath == "" {
			break
		}
		o.remote = dir + "/"

		// Check to see if object already exists
		_, err := o.readObjectInfo(ctx)
		if err == nil {
			return nil
		}

		// Upload it if not
		fs.Debugf(o, "Creating directory marker")
		content := io.Reader(strings.NewReader(""))
		err = o.Update(ctx, content, o)
		if err != nil {
			return fmt.Errorf("creating directory marker failed: %w", err)
		}

		// Now check parent directory exists
		dir = path.Dir(dir)
		if dir == "/" || dir == "." {
			break
		}
	}

	return nil
}

// Mkdir creates the bucket if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) (err error) {
	bucket, _ := f.split(dir)
	e := f.checkBucket(ctx, bucket)
	if e != nil {
		return e
	}
	return f.createDirectoryMarker(ctx, bucket, dir)
}

// mkdirParent creates the parent bucket/directory if it doesn't exist
func (f *Fs) mkdirParent(ctx context.Context, remote string) error {
	remote = strings.TrimRight(remote, "/")
	dir := path.Dir(remote)
	if dir == "/" || dir == "." {
		dir = ""
	}
	return f.Mkdir(ctx, dir)
}

// makeBucket creates the bucket if it doesn't exist
func (f *Fs) makeBucket(ctx context.Context, bucket string) (err error) {
	return f.cache.Create(bucket, func() error {
		// List something from the bucket to see if it exists.  Doing it like this enables the use of a
		// service account that only has the "Storage Object Admin" role.  See #2193 for details.
		err = f.pacer.Call(func() (bool, error) {
			list := f.svc.Objects.List(bucket).MaxResults(1).Context(ctx)
			if f.opt.UserProject != "" {
				list = list.UserProject(f.opt.UserProject)
			}
			_, err = list.Do()
			return shouldRetry(ctx, err)
		})
		if err == nil {
			// Bucket already exists
			return nil
		} else if gErr, ok := err.(*googleapi.Error); ok {
			if gErr.Code != http.StatusNotFound {
				return fmt.Errorf("failed to get bucket: %w", err)
			}
		} else {
			return fmt.Errorf("failed to get bucket: %w", err)
		}

		if f.opt.ProjectNumber == "" {
			return errors.New("can't make bucket without project number")
		}

		bucket := storage.Bucket{
			Name:         bucket,
			Location:     f.opt.Location,
			StorageClass: f.opt.StorageClass,
		}
		if f.opt.BucketPolicyOnly {
			bucket.IamConfiguration = &storage.BucketIamConfiguration{
				BucketPolicyOnly: &storage.BucketIamConfigurationBucketPolicyOnly{
					Enabled: true,
				},
			}
		}
		return f.pacer.Call(func() (bool, error) {
			insertBucket := f.svc.Buckets.Insert(f.opt.ProjectNumber, &bucket)
			if !f.opt.BucketPolicyOnly {
				insertBucket.PredefinedAcl(f.opt.BucketACL)
			}
			insertBucket = insertBucket.Context(ctx)
			if f.opt.UserProject != "" {
				insertBucket = insertBucket.UserProject(f.opt.UserProject)
			}
			_, err = insertBucket.Do()
			return shouldRetry(ctx, err)
		})
	}, nil)
}

// checkBucket creates the bucket if it doesn't exist unless NoCheckBucket is true
func (f *Fs) checkBucket(ctx context.Context, bucket string) error {
	if f.opt.NoCheckBucket {
		return nil
	}
	return f.makeBucket(ctx, bucket)
}

// Rmdir deletes the bucket if the fs is at the root
//
// Returns an error if it isn't empty: Error 409: The bucket you tried
// to delete was not empty.
func (f *Fs) Rmdir(ctx context.Context, dir string) (err error) {
	bucket, directory := f.split(dir)
	// Remove directory marker file
	if f.opt.DirectoryMarkers && bucket != "" && dir != "" {
		o := &Object{
			fs:     f,
			remote: dir + "/",
		}
		fs.Debugf(o, "Removing directory marker")
		err := o.Remove(ctx)
		if err != nil {
			return fmt.Errorf("removing directory marker failed: %w", err)
		}
	}
	if bucket == "" || directory != "" {
		return nil
	}
	return f.cache.Remove(bucket, func() error {
		return f.pacer.Call(func() (bool, error) {
			deleteBucket := f.svc.Buckets.Delete(bucket).Context(ctx)
			if f.opt.UserProject != "" {
				deleteBucket = deleteBucket.UserProject(f.opt.UserProject)
			}
			err = deleteBucket.Do()
			return shouldRetry(ctx, err)
		})
	})
}

// Precision returns the precision
func (f *Fs) Precision() time.Duration {
	return time.Nanosecond
}

// Copy src to this remote using server-side copy operations.
//
// This is stored with the remote path given.
//
// It returns the destination Object and a possible error.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	dstBucket, dstPath := f.split(remote)
	err := f.mkdirParent(ctx, remote)
	if err != nil {
		return nil, err
	}
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	srcBucket, srcPath := srcObj.split()

	// Temporary Object under construction
	dstObj := &Object{
		fs:     f,
		remote: remote,
	}

	rewriteRequest := f.svc.Objects.Rewrite(srcBucket, srcPath, dstBucket, dstPath, nil)
	if !f.opt.BucketPolicyOnly {
		rewriteRequest.DestinationPredefinedAcl(f.opt.ObjectACL)
	}
	var rewriteResponse *storage.RewriteResponse
	for {
		err = f.pacer.Call(func() (bool, error) {
			rewriteRequest = rewriteRequest.Context(ctx)
			if f.opt.UserProject != "" {
				rewriteRequest.UserProject(f.opt.UserProject)
			}
			rewriteResponse, err = rewriteRequest.Do()
			return shouldRetry(ctx, err)
		})
		if err != nil {
			return nil, err
		}
		if rewriteResponse.Done {
			break
		}
		rewriteRequest.RewriteToken(rewriteResponse.RewriteToken)
		fs.Debugf(dstObj, "Continuing rewrite %d bytes done", rewriteResponse.TotalBytesRewritten)
	}
	// Set the metadata for the new object while we have it
	dstObj.setMetaData(rewriteResponse.Resource)
	return dstObj, nil
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.MD5)
}

// ------------------------------------------------------------

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Return a string version
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// Hash returns the Md5sum of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	return o.md5sum, nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.bytes
}

// setMetaData sets the fs data from a storage.Object
func (o *Object) setMetaData(info *storage.Object) {
	o.url = info.MediaLink
	o.bytes = int64(info.Size)
	o.mimeType = info.ContentType
	o.gzipped = info.ContentEncoding == "gzip"

	// Read md5sum
	md5sumData, err := base64.StdEncoding.DecodeString(info.Md5Hash)
	if err != nil {
		fs.Logf(o, "Bad MD5 decode: %v", err)
	} else {
		o.md5sum = hex.EncodeToString(md5sumData)
	}

	// read mtime out of metadata if available
	mtimeString, ok := info.Metadata[metaMtime]
	if ok {
		modTime, err := time.Parse(timeFormat, mtimeString)
		if err == nil {
			o.modTime = modTime
			return
		}
		fs.Debugf(o, "Failed to read mtime from metadata: %s", err)
	}

	// Fallback to GSUtil mtime
	mtimeGsutilString, ok := info.Metadata[metaMtimeGsutil]
	if ok {
		unixTimeSec, err := strconv.ParseInt(mtimeGsutilString, 10, 64)
		if err == nil {
			o.modTime = time.Unix(unixTimeSec, 0)
			return
		}
		fs.Debugf(o, "Failed to read GSUtil mtime from metadata: %s", err)
	}

	// Fallback to the Updated time
	modTime, err := time.Parse(timeFormat, info.Updated)
	if err != nil {
		fs.Logf(o, "Bad time decode: %v", err)
	} else {
		o.modTime = modTime
	}

	// If gunzipping then size and md5sum are unknown
	if o.gzipped && o.fs.opt.Decompress {
		o.bytes = -1
		o.md5sum = ""
	}
}

// readObjectInfo reads the definition for an object
func (o *Object) readObjectInfo(ctx context.Context) (object *storage.Object, err error) {
	bucket, bucketPath := o.split()
	return o.fs.readObjectInfo(ctx, bucket, bucketPath)
}

// readObjectInfo reads the definition for an object
func (f *Fs) readObjectInfo(ctx context.Context, bucket, bucketPath string) (object *storage.Object, err error) {
	err = f.pacer.Call(func() (bool, error) {
		get := f.svc.Objects.Get(bucket, bucketPath).Context(ctx)
		if f.opt.UserProject != "" {
			get = get.UserProject(f.opt.UserProject)
		}
		object, err = get.Do()
		return shouldRetry(ctx, err)
	})
	if err != nil {
		if gErr, ok := err.(*googleapi.Error); ok {
			if gErr.Code == http.StatusNotFound {
				return nil, fs.ErrorObjectNotFound
			}
		}
		return nil, err
	}
	return object, nil
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *Object) readMetaData(ctx context.Context) (err error) {
	if !o.modTime.IsZero() {
		return nil
	}
	object, err := o.readObjectInfo(ctx)
	if err != nil {
		return err
	}
	o.setMetaData(object)
	return nil
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) time.Time {
	err := o.readMetaData(ctx)
	if err != nil {
		// fs.Logf(o, "Failed to read metadata: %v", err)
		return time.Now()
	}
	return o.modTime
}

// Returns metadata for an object
func metadataFromModTime(modTime time.Time) map[string]string {
	metadata := make(map[string]string, 1)
	metadata[metaMtime] = modTime.Format(timeFormat)
	metadata[metaMtimeGsutil] = strconv.FormatInt(modTime.Unix(), 10)
	return metadata
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) (err error) {
	// read the complete existing object first
	object, err := o.readObjectInfo(ctx)
	if err != nil {
		return err
	}
	// Add the mtime to the existing metadata
	if object.Metadata == nil {
		object.Metadata = make(map[string]string, 1)
	}
	object.Metadata[metaMtime] = modTime.Format(timeFormat)
	object.Metadata[metaMtimeGsutil] = strconv.FormatInt(modTime.Unix(), 10)
	// Copy the object to itself to update the metadata
	// Using PATCH requires too many permissions
	bucket, bucketPath := o.split()
	var newObject *storage.Object
	err = o.fs.pacer.Call(func() (bool, error) {
		copyObject := o.fs.svc.Objects.Copy(bucket, bucketPath, bucket, bucketPath, object)
		if !o.fs.opt.BucketPolicyOnly {
			copyObject.DestinationPredefinedAcl(o.fs.opt.ObjectACL)
		}
		copyObject = copyObject.Context(ctx)
		if o.fs.opt.UserProject != "" {
			copyObject = copyObject.UserProject(o.fs.opt.UserProject)
		}
		newObject, err = copyObject.Do()
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return err
	}
	o.setMetaData(newObject)
	return nil
}

// Storable returns a boolean as to whether this object is storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	url := o.url
	if o.fs.opt.UserProject != "" {
		url += "&userProject=" + o.fs.opt.UserProject
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	fs.FixRangeOption(options, o.bytes)
	if o.gzipped && !o.fs.opt.Decompress {
		// Allow files which are stored on the cloud storage system
		// compressed to be downloaded without being decompressed.  Note
		// that setting this here overrides the automatic decompression
		// in the Transport.
		//
		// See: https://cloud.google.com/storage/docs/transcoding
		req.Header.Set("Accept-Encoding", "gzip")
		o.fs.warnCompressed.Do(func() {
			fs.Logf(o, "Not decompressing 'Content-Encoding: gzip' compressed file. Use --gcs-decompress to override")
		})
	}
	fs.OpenOptionAddHTTPHeaders(req.Header, options)
	var res *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		res, err = o.fs.client.Do(req)
		if err == nil {
			err = googleapi.CheckResponse(res)
			if err != nil {
				_ = res.Body.Close() // ignore error
			}
		}
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, err
	}
	_, isRanging := req.Header["Range"]
	if !(res.StatusCode == http.StatusOK || (isRanging && res.StatusCode == http.StatusPartialContent)) {
		_ = res.Body.Close() // ignore error
		return nil, fmt.Errorf("bad response: %d: %s", res.StatusCode, res.Status)
	}
	return res.Body, nil
}

// Update the object with the contents of the io.Reader, modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	bucket, bucketPath := o.split()
	// Create parent dir/bucket if not saving directory marker
	if !strings.HasSuffix(o.remote, "/") {
		err = o.fs.mkdirParent(ctx, o.remote)
		if err != nil {
			return err
		}
	}
	modTime := src.ModTime(ctx)

	object := storage.Object{
		Bucket:      bucket,
		Name:        bucketPath,
		ContentType: fs.MimeType(ctx, src),
		Metadata:    metadataFromModTime(modTime),
	}
	// Apply upload options
	for _, option := range options {
		key, value := option.Header()
		lowerKey := strings.ToLower(key)
		switch lowerKey {
		case "":
			// ignore
		case "cache-control":
			object.CacheControl = value
		case "content-disposition":
			object.ContentDisposition = value
		case "content-encoding":
			object.ContentEncoding = value
		case "content-language":
			object.ContentLanguage = value
		case "content-type":
			object.ContentType = value
		case "x-goog-storage-class":
			object.StorageClass = value
		default:
			const googMetaPrefix = "x-goog-meta-"
			if strings.HasPrefix(lowerKey, googMetaPrefix) {
				metaKey := lowerKey[len(googMetaPrefix):]
				object.Metadata[metaKey] = value
			} else {
				fs.Errorf(o, "Don't know how to set key %q on upload", key)
			}
		}
	}
	var newObject *storage.Object
	err = o.fs.pacer.CallNoRetry(func() (bool, error) {
		insertObject := o.fs.svc.Objects.Insert(bucket, &object).Media(in, googleapi.ContentType("")).Name(object.Name)
		if !o.fs.opt.BucketPolicyOnly {
			insertObject.PredefinedAcl(o.fs.opt.ObjectACL)
		}
		insertObject = insertObject.Context(ctx)
		if o.fs.opt.UserProject != "" {
			insertObject = insertObject.UserProject(o.fs.opt.UserProject)
		}
		newObject, err = insertObject.Do()
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return err
	}
	// Set the metadata for the new object while we have it
	o.setMetaData(newObject)
	return nil
}

// Remove an object
func (o *Object) Remove(ctx context.Context) (err error) {
	bucket, bucketPath := o.split()
	err = o.fs.pacer.Call(func() (bool, error) {
		deleteBucket := o.fs.svc.Objects.Delete(bucket, bucketPath).Context(ctx)
		if o.fs.opt.UserProject != "" {
			deleteBucket = deleteBucket.UserProject(o.fs.opt.UserProject)
		}
		err = deleteBucket.Do()
		return shouldRetry(ctx, err)
	})
	return err
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType(ctx context.Context) string {
	return o.mimeType
}

// Check the interfaces are satisfied
var (
	_ fs.Fs          = &Fs{}
	_ fs.Copier      = &Fs{}
	_ fs.PutStreamer = &Fs{}
	_ fs.ListRer     = &Fs{}
	_ fs.Object      = &Object{}
	_ fs.MimeTyper   = &Object{}
)
