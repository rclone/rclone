# Adding a new s3 provider

It is quite easy to add a new S3 provider to rclone.

You'll then need to do add the following (optional tags are in [] and
do not get displayed in rclone config if empty):

The process is as follows: Create yaml -> add docs -> run tests ->
adjust yaml until tests pass.

All tags can be found in `backend/s3/providers.go` Provider Struct.
Looking through a few of the yaml files as examples should make things
clear. `AWS.yaml` as the most config. pasting.

## YAML

In `backend/s3/provider/YourProvider.yaml`

- name
- description
  - More like the full name often "YourProvider + Object Storage"
- [Region]
  - Any regions your provider supports or the defaults (use `region: {}` for this)
    - Example from AWS.yaml:

      ```yaml
      region:
        us-east-1: |-
          The default endpoint - a good choice if you are unsure.
          US Region, Northern Virginia, or Pacific Northwest.
          Leave location constraint empty.
      ```

    - The defaults (as seen in Rclone.yaml):

      ```yaml
      region:
        "": |-
          Use this if unsure.
          Will use v4 signatures and an empty region.
        other-v2-signature: |-
          Use this only if v4 signatures don't work.
          E.g. pre Jewel/v10 CEPH.
      ```

- [Endpoint]
  - Any endpoints your provider supports

    - Example from Mega.yaml

      ```yaml
        endpoint:
          s3.eu-central-1.s4.mega.io: Mega S4 eu-central-1 (Amsterdam)
      ```

- [Location Constraint]
  - The Location Constraint of your remote, often same as region.
    - Example from AWS.yaml

      ```yaml
      location_constraint:
        "": Empty for US Region, Northern Virginia, or Pacific Northwest
        us-east-2: US East (Ohio) Region
      ```

- [ACL]
  - Identical across *most* providers. Select the default with `acl: {}`
    - Example from AWS.yaml

      ```yaml
      acl:
        private: |-
          Owner gets FULL_CONTROL.
          No one else has access rights (default).
        public-read: |-
          Owner gets FULL_CONTROL.
          The AllUsers group gets READ access.
        public-read-write: |-
          Owner gets FULL_CONTROL.
          The AllUsers group gets READ and WRITE access.
          Granting this on a bucket is generally not recommended.
        authenticated-read: |-
          Owner gets FULL_CONTROL.
          The AuthenticatedUsers group gets READ access.
        bucket-owner-read: |-
          Object owner gets FULL_CONTROL.
          Bucket owner gets READ access.
          If you specify this canned ACL when creating a bucket, Amazon S3 ignores it.
        bucket-owner-full-control: |-
          Both the object owner and the bucket owner get FULL_CONTROL over the object.
          If you specify this canned ACL when creating a bucket, Amazon S3 ignores it.
      ```

- [Storage Class]
  - Identical across *most* providers.
    - Defaults from AWS.yaml

      ```yaml
      storage_class:
        "": Default
        STANDARD: Standard storage class
        REDUCED_REDUNDANCY: Reduced redundancy storage class
        STANDARD_IA: Standard Infrequent Access storage class
        ONEZONE_IA: One Zone Infrequent Access storage class
        GLACIER: Glacier Flexible Retrieval storage class
        DEEP_ARCHIVE: Glacier Deep Archive storage class
        INTELLIGENT_TIERING: Intelligent-Tiering storage class
        GLACIER_IR: Glacier Instant Retrieval storage class
      ```

- [Server Side Encryption]
  - Not common, identical across *most* providers.
    - Defaults from AWS.yaml

      ```yaml
        server_side_encryption:
          "": None
          AES256: AES256
          aws:kms: aws:kms
      ```

- [Advanced Options]
  - All advanced options are Boolean - if true the configurator asks about that
    value, if not it doesn't:

    ```go
    BucketACL             bool `yaml:"bucket_acl,omitempty"`
    DirectoryBucket       bool `yaml:"directory_bucket,omitempty"`
    LeavePartsOnError     bool `yaml:"leave_parts_on_error,omitempty"`
    RequesterPays         bool `yaml:"requester_pays,omitempty"`
    SSECustomerAlgorithm  bool `yaml:"sse_customer_algorithm,omitempty"`
    SSECustomerKey        bool `yaml:"sse_customer_key,omitempty"`
    SSECustomerKeyBase64  bool `yaml:"sse_customer_key_base64,omitempty"`
    SSECustomerKeyMd5     bool `yaml:"sse_customer_key_md5,omitempty"`
    SSEKmsKeyID           bool `yaml:"sse_kms_key_id,omitempty"`
    STSEndpoint           bool `yaml:"sts_endpoint,omitempty"`
    UseAccelerateEndpoint bool `yaml:"use_accelerate_endpoint,omitempty"`
    ```

    - Example from AWS.yaml:

      ```yaml
      bucket_acl: true
      directory_bucket: true
      leave_parts_on_error: true
      requester_pays: true
      sse_customer_algorithm: true
      sse_customer_key: true
      sse_customer_key_base64: true
      sse_customer_key_md5: true
      sse_kms_key_id: true
      sts_endpoint: true
      use_accelerate_endpoint: true
      ```

- Quirks
  - Quirks are discovered through documentation and running the tests as seen below.
    - Most quirks are *bool as to have 3 values, `true`, `false` and `dont care`.

      ```go
      type Quirks struct {
          ListVersion           *int   `yaml:"list_version,omitempty"`     // 1 or 2
          ForcePathStyle        *bool  `yaml:"force_path_style,omitempty"` // true = path-style
          ListURLEncode         *bool  `yaml:"list_url_encode,omitempty"`
          UseMultipartEtag      *bool  `yaml:"use_multipart_etag,omitempty"`
          UseAlreadyExists      *bool  `yaml:"use_already_exists,omitempty"`
          UseAcceptEncodingGzip *bool  `yaml:"use_accept_encoding_gzip,omitempty"`
          MightGzip             *bool  `yaml:"might_gzip,omitempty"`
          UseMultipartUploads   *bool  `yaml:"use_multipart_uploads,omitempty"`
          UseUnsignedPayload    *bool  `yaml:"use_unsigned_payload,omitempty"`
          UseXID                *bool  `yaml:"use_x_id,omitempty"`
          SignAcceptEncoding    *bool  `yaml:"sign_accept_encoding,omitempty"`
          CopyCutoff            *int64 `yaml:"copy_cutoff,omitempty"`
          MaxUploadParts        *int   `yaml:"max_upload_parts,omitempty"`
          MinChunkSize          *int64 `yaml:"min_chunk_size,omitempty"`
      }
        ```

      - Example from AWS.yaml

        ```yaml
        quirks:
          might_gzip: false # Never auto gzips objects
          use_unsigned_payload: false # AWS has trailer support
        ```

Note that if you omit a section, eg `region` then the user won't be
asked that question, and if you add an empty section e.g. `region: {}`
then the defaults from the `Other.yaml` will be used.

## DOCS

- `docs/content/s3.md`
  - Add the provider at the top of the page.
  - Add a section about the provider linked from there.
    - Make sure this is in alphabetical order in the `Providers` section.
  - Add a transcript of a trial `rclone config` session
    - Edit the transcript to remove things which might change in subsequent versions
  - **Do not** alter or add to the autogenerated parts of `s3.md`
    - Rule of thumb: don't edit anything not mentioned above.
  - **Do not** run `make backenddocs` or `bin/make_backend_docs.py s3`
    - This will make autogenerated changes!
- `README.md` - this is the home page in github
  - Add the provider and a link to the section you wrote in `docs/contents/s3.md`
- `docs/content/_index.md` - this is the home page of rclone.org
  - Add the provider and a link to the section you wrote in `docs/contents/s3.md`
- Once you've written the docs, run `make serve` and check they look OK
  in the web browser and the links (internal and external) all work.

## TESTS

Once you've written the code, test `rclone config` works to your
satisfaction and looks correct, and check the integration tests work
`go test -v -remote NewS3Provider:`. You may need to adjust the quirks
to get them to pass. Some providers just can't pass the tests with
control characters in the names so if these fail and the provider
doesn't support `urlEncodeListings` in the quirks then ignore them.
