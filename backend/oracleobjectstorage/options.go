//go:build !plan9 && !solaris && !js
// +build !plan9,!solaris,!js

package oracleobjectstorage

import (
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/lib/encoder"
)

const (
	maxSizeForCopy             = 4768 * 1024 * 1024
	maxUploadParts             = 10000
	defaultUploadConcurrency   = 10
	minChunkSize               = fs.SizeSuffix(5 * 1024 * 1024)
	defaultUploadCutoff        = fs.SizeSuffix(200 * 1024 * 1024)
	maxUploadCutoff            = fs.SizeSuffix(5 * 1024 * 1024 * 1024)
	minSleep                   = 10 * time.Millisecond
	defaultCopyTimeoutDuration = fs.Duration(time.Minute)
)

const (
	userPrincipal     = "user_principal_auth"
	instancePrincipal = "instance_principal_auth"
	resourcePrincipal = "resource_principal_auth"
	workloadIdentity  = "workload_identity_auth"
	environmentAuth   = "env_auth"
	noAuth            = "no_auth"

	userPrincipalHelpText = `use an OCI user and an API key for authentication.
you’ll need to put in a config file your tenancy OCID, user OCID, region, the path, fingerprint to an API key.
https://docs.oracle.com/en-us/iaas/Content/API/Concepts/sdkconfig.htm`

	instancePrincipalHelpText = `use instance principals to authorize an instance to make API calls. 
each instance has its own identity, and authenticates using the certificates that are read from instance metadata. 
https://docs.oracle.com/en-us/iaas/Content/Identity/Tasks/callingservicesfrominstances.htm`

	workloadIdentityHelpText = `use workload identity to grant OCI Container Engine for Kubernetes workloads policy-driven access to OCI resources using OCI Identity and Access Management (IAM).
https://docs.oracle.com/en-us/iaas/Content/ContEng/Tasks/contenggrantingworkloadaccesstoresources.htm`
	resourcePrincipalHelpText = `use resource principals to make API calls`

	environmentAuthHelpText = `automatically pickup the credentials from runtime(env), first one to provide auth wins`

	noAuthHelpText = `no credentials needed, this is typically for reading public buckets`
)

// Options defines the configuration for this backend
type Options struct {
	Provider             string               `config:"provider"`
	Compartment          string               `config:"compartment"`
	Namespace            string               `config:"namespace"`
	Region               string               `config:"region"`
	Endpoint             string               `config:"endpoint"`
	Enc                  encoder.MultiEncoder `config:"encoding"`
	ConfigFile           string               `config:"config_file"`
	ConfigProfile        string               `config:"config_profile"`
	UploadCutoff         fs.SizeSuffix        `config:"upload_cutoff"`
	ChunkSize            fs.SizeSuffix        `config:"chunk_size"`
	MaxUploadParts       int                  `config:"max_upload_parts"`
	UploadConcurrency    int                  `config:"upload_concurrency"`
	DisableChecksum      bool                 `config:"disable_checksum"`
	CopyCutoff           fs.SizeSuffix        `config:"copy_cutoff"`
	CopyTimeout          fs.Duration          `config:"copy_timeout"`
	StorageTier          string               `config:"storage_tier"`
	LeavePartsOnError    bool                 `config:"leave_parts_on_error"`
	AttemptResumeUpload  bool                 `config:"attempt_resume_upload"`
	NoCheckBucket        bool                 `config:"no_check_bucket"`
	SSEKMSKeyID          string               `config:"sse_kms_key_id"`
	SSECustomerAlgorithm string               `config:"sse_customer_algorithm"`
	SSECustomerKey       string               `config:"sse_customer_key"`
	SSECustomerKeyFile   string               `config:"sse_customer_key_file"`
	SSECustomerKeySha256 string               `config:"sse_customer_key_sha256"`
}

func newOptions() []fs.Option {
	return []fs.Option{{
		Name:     fs.ConfigProvider,
		Help:     "Choose your Auth Provider",
		Required: true,
		Default:  environmentAuth,
		Examples: []fs.OptionExample{{
			Value: environmentAuth,
			Help:  environmentAuthHelpText,
		}, {
			Value: userPrincipal,
			Help:  userPrincipalHelpText,
		}, {
			Value: instancePrincipal,
			Help:  instancePrincipalHelpText,
		}, {
			Value: workloadIdentity,
			Help:  workloadIdentityHelpText,
		}, {
			Value: resourcePrincipal,
			Help:  resourcePrincipalHelpText,
		}, {
			Value: noAuth,
			Help:  noAuthHelpText,
		}},
	}, {
		Name:      "namespace",
		Help:      "Object storage namespace",
		Required:  true,
		Sensitive: true,
	}, {
		Name:      "compartment",
		Help:      "Object storage compartment OCID",
		Provider:  "!no_auth",
		Required:  true,
		Sensitive: true,
	}, {
		Name:     "region",
		Help:     "Object storage Region",
		Required: true,
	}, {
		Name:     "endpoint",
		Help:     "Endpoint for Object storage API.\n\nLeave blank to use the default endpoint for the region.",
		Required: false,
	}, {
		Name:     "config_file",
		Help:     "Path to OCI config file",
		Provider: userPrincipal,
		Default:  "~/.oci/config",
		Examples: []fs.OptionExample{{
			Value: "~/.oci/config",
			Help:  "oci configuration file location",
		}},
	}, {
		Name:     "config_profile",
		Help:     "Profile name inside the oci config file",
		Provider: userPrincipal,
		Default:  "Default",
		Examples: []fs.OptionExample{{
			Value: "Default",
			Help:  "Use the default profile",
		}},
	}, {
		// Mapping from here: https://github.com/oracle/oci-go-sdk/blob/master/objectstorage/storage_tier.go
		Name:     "storage_tier",
		Help:     "The storage class to use when storing new objects in storage. https://docs.oracle.com/en-us/iaas/Content/Object/Concepts/understandingstoragetiers.htm",
		Default:  "Standard",
		Advanced: true,
		Examples: []fs.OptionExample{{
			Value: "Standard",
			Help:  "Standard storage tier, this is the default tier",
		}, {
			Value: "InfrequentAccess",
			Help:  "InfrequentAccess storage tier",
		}, {
			Value: "Archive",
			Help:  "Archive storage tier",
		}},
	}, {
		Name: "upload_cutoff",
		Help: `Cutoff for switching to chunked upload.

Any files larger than this will be uploaded in chunks of chunk_size.
The minimum is 0 and the maximum is 5 GiB.`,
		Default:  defaultUploadCutoff,
		Advanced: true,
	}, {
		Name: "chunk_size",
		Help: `Chunk size to use for uploading.

When uploading files larger than upload_cutoff or files with unknown
size (e.g. from "rclone rcat" or uploaded with "rclone mount" they will be uploaded 
as multipart uploads using this chunk size.

Note that "upload_concurrency" chunks of this size are buffered
in memory per transfer.

If you are transferring large files over high-speed links and you have
enough memory, then increasing this will speed up the transfers.

Rclone will automatically increase the chunk size when uploading a
large file of known size to stay below the 10,000 chunks limit.

Files of unknown size are uploaded with the configured
chunk_size. Since the default chunk size is 5 MiB and there can be at
most 10,000 chunks, this means that by default the maximum size of
a file you can stream upload is 48 GiB.  If you wish to stream upload
larger files then you will need to increase chunk_size.

Increasing the chunk size decreases the accuracy of the progress
statistics displayed with "-P" flag.
`,
		Default:  minChunkSize,
		Advanced: true,
	}, {
		Name: "max_upload_parts",
		Help: `Maximum number of parts in a multipart upload.

This option defines the maximum number of multipart chunks to use
when doing a multipart upload.

OCI has max parts limit of 10,000 chunks.

Rclone will automatically increase the chunk size when uploading a
large file of a known size to stay below this number of chunks limit.
`,
		Default:  maxUploadParts,
		Advanced: true,
	}, {
		Name: "upload_concurrency",
		Help: `Concurrency for multipart uploads.

This is the number of chunks of the same file that are uploaded
concurrently.

If you are uploading small numbers of large files over high-speed links
and these uploads do not fully utilize your bandwidth, then increasing
this may help to speed up the transfers.`,
		Default:  defaultUploadConcurrency,
		Advanced: true,
	}, {
		Name: "copy_cutoff",
		Help: `Cutoff for switching to multipart copy.

Any files larger than this that need to be server-side copied will be
copied in chunks of this size.

The minimum is 0 and the maximum is 5 GiB.`,
		Default:  fs.SizeSuffix(maxSizeForCopy),
		Advanced: true,
	}, {
		Name: "copy_timeout",
		Help: `Timeout for copy.

Copy is an asynchronous operation, specify timeout to wait for copy to succeed
`,
		Default:  defaultCopyTimeoutDuration,
		Advanced: true,
	}, {
		Name: "disable_checksum",
		Help: `Don't store MD5 checksum with object metadata.

Normally rclone will calculate the MD5 checksum of the input before
uploading it so it can add it to metadata on the object. This is great
for data integrity checking but can cause long delays for large files
to start uploading.`,
		Default:  false,
		Advanced: true,
	}, {
		Name:     config.ConfigEncoding,
		Help:     config.ConfigEncodingHelp,
		Advanced: true,
		// Any UTF-8 character is valid in a key, however it can't handle
		// invalid UTF-8 and / have a special meaning.
		//
		// The SDK can't seem to handle uploading files called '.
		// - initial / encoding
		// - doubled / encoding
		// - trailing / encoding
		// so that OSS keys are always valid file names
		Default: encoder.EncodeInvalidUtf8 |
			encoder.EncodeSlash |
			encoder.EncodeDot,
	}, {
		Name: "leave_parts_on_error",
		Help: `If true avoid calling abort upload on a failure, leaving all successfully uploaded parts for manual recovery.

It should be set to true for resuming uploads across different sessions.

WARNING: Storing parts of an incomplete multipart upload counts towards space usage on object storage and will add
additional costs if not cleaned up.
`,
		Default:  false,
		Advanced: true,
	}, {
		Name: "attempt_resume_upload",
		Help: `If true attempt to resume previously started multipart upload for the object.
This will be helpful to speed up multipart transfers by resuming uploads from past session.

WARNING: If chunk size differs in resumed session from past incomplete session, then the resumed multipart upload is 
aborted and a new multipart upload is started with the new chunk size.

The flag leave_parts_on_error must be true to resume and optimize to skip parts that were already uploaded successfully.
`,
		Default:  false,
		Advanced: true,
	}, {
		Name: "no_check_bucket",
		Help: `If set, don't attempt to check the bucket exists or create it.

This can be useful when trying to minimise the number of transactions
rclone does if you know the bucket exists already.

It can also be needed if the user you are using does not have bucket
creation permissions.
`,
		Default:  false,
		Advanced: true,
	}, {
		Name: "sse_customer_key_file",
		Help: `To use SSE-C, a file containing the base64-encoded string of the AES-256 encryption key associated
with the object. Please note only one of sse_customer_key_file|sse_customer_key|sse_kms_key_id is needed.'`,
		Advanced: true,
		Examples: []fs.OptionExample{{
			Value: "",
			Help:  "None",
		}},
	}, {
		Name: "sse_customer_key",
		Help: `To use SSE-C, the optional header that specifies the base64-encoded 256-bit encryption key to use to
encrypt or  decrypt the data. Please note only one of sse_customer_key_file|sse_customer_key|sse_kms_key_id is
needed. For more information, see Using Your Own Keys for Server-Side Encryption 
(https://docs.cloud.oracle.com/Content/Object/Tasks/usingyourencryptionkeys.htm)`,
		Advanced: true,
		Examples: []fs.OptionExample{{
			Value: "",
			Help:  "None",
		}},
	}, {
		Name: "sse_customer_key_sha256",
		Help: `If using SSE-C, The optional header that specifies the base64-encoded SHA256 hash of the encryption
key. This value is used to check the integrity of the encryption key. see Using Your Own Keys for 
Server-Side Encryption (https://docs.cloud.oracle.com/Content/Object/Tasks/usingyourencryptionkeys.htm).`,
		Advanced: true,
		Examples: []fs.OptionExample{{
			Value: "",
			Help:  "None",
		}},
	}, {
		Name: "sse_kms_key_id",
		Help: `if using your own master key in vault, this header specifies the
OCID (https://docs.cloud.oracle.com/Content/General/Concepts/identifiers.htm) of a master encryption key used to call
the Key Management service to generate a data encryption key or to encrypt or decrypt a data encryption key.
Please note only one of sse_customer_key_file|sse_customer_key|sse_kms_key_id is needed.`,
		Advanced: true,
		Examples: []fs.OptionExample{{
			Value: "",
			Help:  "None",
		}},
	}, {
		Name: "sse_customer_algorithm",
		Help: `If using SSE-C, the optional header that specifies "AES256" as the encryption algorithm.
Object Storage supports "AES256" as the encryption algorithm. For more information, see
Using Your Own Keys for Server-Side Encryption (https://docs.cloud.oracle.com/Content/Object/Tasks/usingyourencryptionkeys.htm).`,
		Advanced: true,
		Examples: []fs.OptionExample{{
			Value: "",
			Help:  "None",
		}, {
			Value: sseDefaultAlgorithm,
			Help:  sseDefaultAlgorithm,
		}},
	}}
}
