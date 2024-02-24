---
title: "HiDrive"
description: "Rclone docs for HiDrive"
versionIntroduced: "v1.59"
---

# {{< icon "fa fa-cloud" >}} HiDrive

Paths are specified as `remote:path`

Paths may be as deep as required, e.g. `remote:directory/subdirectory`.

The initial setup for hidrive involves getting a token from HiDrive
which you need to do in your browser.
`rclone config` walks you through it.

## Configuration

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / HiDrive
   \ "hidrive"
[snip]
Storage> hidrive
OAuth Client Id - Leave blank normally.
client_id>
OAuth Client Secret - Leave blank normally.
client_secret>
Access permissions that rclone should use when requesting access from HiDrive.
Leave blank normally.
scope_access>
Edit advanced config?
y/n> n
Use web browser to automatically authenticate rclone with remote?
 * Say Y if the machine running rclone has a web browser you can use
 * Say N if running rclone on a (remote) machine without web browser access
If not sure try Y. If Y failed, try N.
y/n> y
If your browser doesn't open automatically go to the following link: http://127.0.0.1:53682/auth?state=xxxxxxxxxxxxxxxxxxxxxx
Log in and authorize rclone for access
Waiting for code...
Got code
--------------------
[remote]
type = hidrive
token = {"access_token":"xxxxxxxxxxxxxxxxxxxx","token_type":"Bearer","refresh_token":"xxxxxxxxxxxxxxxxxxxxxxx","expiry":"xxxxxxxxxxxxxxxxxxxxxxx"}
--------------------
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d> y
```

**You should be aware that OAuth-tokens can be used to access your account
and hence should not be shared with other persons.**
See the [below section](#keeping-your-tokens-safe) for more information.

See the [remote setup docs](/remote_setup/) for how to set it up on a
machine with no Internet browser available.

Note that rclone runs a webserver on your local machine to collect the
token as returned from HiDrive. This only runs from the moment it opens
your browser to the moment you get back the verification code.
The webserver runs on `http://127.0.0.1:53682/`.
If local port `53682` is protected by a firewall you may need to temporarily
unblock the firewall to complete authorization.

Once configured you can then use `rclone` like this,

List directories in top level of your HiDrive root folder

    rclone lsd remote:

List all the files in your HiDrive filesystem

    rclone ls remote:

To copy a local directory to a HiDrive directory called backup

    rclone copy /home/source remote:backup

### Keeping your tokens safe

Any OAuth-tokens will be stored by rclone in the remote's configuration file as unencrypted text.
Anyone can use a valid refresh-token to access your HiDrive filesystem without knowing your password.
Therefore you should make sure no one else can access your configuration.

It is possible to encrypt rclone's configuration file.
You can find information on securing your configuration file by viewing the [configuration encryption docs](/docs/#configuration-encryption).

### Invalid refresh token

As can be verified [here](https://developer.hidrive.com/basics-flows/),
each `refresh_token` (for Native Applications) is valid for 60 days.
If used to access HiDrivei, its validity will be automatically extended.

This means that if you

  * Don't use the HiDrive remote for 60 days

then rclone will return an error which includes a text
that implies the refresh token is *invalid* or *expired*.

To fix this you will need to authorize rclone to access your HiDrive account again.

Using

    rclone config reconnect remote:

the process is very similar to the process of initial setup exemplified before.

### Modification times and hashes

HiDrive allows modification times to be set on objects accurate to 1 second.

HiDrive supports [its own hash type](https://static.hidrive.com/dev/0001)
which is used to verify the integrity of file contents after successful transfers.

### Restricted filename characters

HiDrive cannot store files or folders that include
`/` (0x2F) or null-bytes (0x00) in their name.
Any other characters can be used in the names of files or folders.
Additionally, files or folders cannot be named either of the following: `.` or `..`

Therefore rclone will automatically replace these characters,
if files or folders are stored or accessed with such names.

You can read about how this filename encoding works in general
[here](overview/#restricted-filenames).

Keep in mind that HiDrive only supports file or folder names
with a length of 255 characters or less.

### Transfers

HiDrive limits file sizes per single request to a maximum of 2 GiB.
To allow storage of larger files and allow for better upload performance,
the hidrive backend will use a chunked transfer for files larger than 96 MiB.
Rclone will upload multiple parts/chunks of the file at the same time.
Chunks in the process of being uploaded are buffered in memory,
so you may want to restrict this behaviour on systems with limited resources.

You can customize this behaviour using the following options:

* `chunk_size`: size of file parts
* `upload_cutoff`: files larger or equal to this in size will use a chunked transfer
* `upload_concurrency`: number of file-parts to upload at the same time

See the below section about configuration options for more details.

### Root folder

You can set the root folder for rclone.
This is the directory that rclone considers to be the root of your HiDrive.

Usually, you will leave this blank, and rclone will use the root of the account.

However, you can set this to restrict rclone to a specific folder hierarchy.

This works by prepending the contents of the `root_prefix` option
to any paths accessed by rclone.
For example, the following two ways to access the home directory are equivalent:

    rclone lsd --hidrive-root-prefix="/users/test/" remote:path

    rclone lsd remote:/users/test/path

See the below section about configuration options for more details.

### Directory member count

By default, rclone will know the number of directory members contained in a directory.
For example, `rclone lsd` uses this information.

The acquisition of this information will result in additional time costs for HiDrive's API.
When dealing with large directory structures, it may be desirable to circumvent this time cost,
especially when this information is not explicitly needed.
For this, the `disable_fetching_member_count` option can be used.

See the below section about configuration options for more details.

{{< rem autogenerated options start" - DO NOT EDIT - instead edit fs.RegInfo in backend/hidrive/hidrive.go then run make backenddocs" >}}
### Standard options

Here are the Standard options specific to hidrive (HiDrive).

#### --hidrive-client-id

OAuth Client Id.

Leave blank normally.

Properties:

- Config:      client_id
- Env Var:     RCLONE_HIDRIVE_CLIENT_ID
- Type:        string
- Required:    false

#### --hidrive-client-secret

OAuth Client Secret.

Leave blank normally.

Properties:

- Config:      client_secret
- Env Var:     RCLONE_HIDRIVE_CLIENT_SECRET
- Type:        string
- Required:    false

#### --hidrive-scope-access

Access permissions that rclone should use when requesting access from HiDrive.

Properties:

- Config:      scope_access
- Env Var:     RCLONE_HIDRIVE_SCOPE_ACCESS
- Type:        string
- Default:     "rw"
- Examples:
    - "rw"
        - Read and write access to resources.
    - "ro"
        - Read-only access to resources.

### Advanced options

Here are the Advanced options specific to hidrive (HiDrive).

#### --hidrive-token

OAuth Access Token as a JSON blob.

Properties:

- Config:      token
- Env Var:     RCLONE_HIDRIVE_TOKEN
- Type:        string
- Required:    false

#### --hidrive-auth-url

Auth server URL.

Leave blank to use the provider defaults.

Properties:

- Config:      auth_url
- Env Var:     RCLONE_HIDRIVE_AUTH_URL
- Type:        string
- Required:    false

#### --hidrive-token-url

Token server url.

Leave blank to use the provider defaults.

Properties:

- Config:      token_url
- Env Var:     RCLONE_HIDRIVE_TOKEN_URL
- Type:        string
- Required:    false

#### --hidrive-scope-role

User-level that rclone should use when requesting access from HiDrive.

Properties:

- Config:      scope_role
- Env Var:     RCLONE_HIDRIVE_SCOPE_ROLE
- Type:        string
- Default:     "user"
- Examples:
    - "user"
        - User-level access to management permissions.
        - This will be sufficient in most cases.
    - "admin"
        - Extensive access to management permissions.
    - "owner"
        - Full access to management permissions.

#### --hidrive-root-prefix

The root/parent folder for all paths.

Fill in to use the specified folder as the parent for all paths given to the remote.
This way rclone can use any folder as its starting point.

Properties:

- Config:      root_prefix
- Env Var:     RCLONE_HIDRIVE_ROOT_PREFIX
- Type:        string
- Default:     "/"
- Examples:
    - "/"
        - The topmost directory accessible by rclone.
        - This will be equivalent with "root" if rclone uses a regular HiDrive user account.
    - "root"
        - The topmost directory of the HiDrive user account
    - ""
        - This specifies that there is no root-prefix for your paths.
        - When using this you will always need to specify paths to this remote with a valid parent e.g. "remote:/path/to/dir" or "remote:root/path/to/dir".

#### --hidrive-endpoint

Endpoint for the service.

This is the URL that API-calls will be made to.

Properties:

- Config:      endpoint
- Env Var:     RCLONE_HIDRIVE_ENDPOINT
- Type:        string
- Default:     "https://api.hidrive.strato.com/2.1"

#### --hidrive-disable-fetching-member-count

Do not fetch number of objects in directories unless it is absolutely necessary.

Requests may be faster if the number of objects in subdirectories is not fetched.

Properties:

- Config:      disable_fetching_member_count
- Env Var:     RCLONE_HIDRIVE_DISABLE_FETCHING_MEMBER_COUNT
- Type:        bool
- Default:     false

#### --hidrive-chunk-size

Chunksize for chunked uploads.

Any files larger than the configured cutoff (or files of unknown size) will be uploaded in chunks of this size.

The upper limit for this is 2147483647 bytes (about 2.000Gi).
That is the maximum amount of bytes a single upload-operation will support.
Setting this above the upper limit or to a negative value will cause uploads to fail.

Setting this to larger values may increase the upload speed at the cost of using more memory.
It can be set to smaller values smaller to save on memory.

Properties:

- Config:      chunk_size
- Env Var:     RCLONE_HIDRIVE_CHUNK_SIZE
- Type:        SizeSuffix
- Default:     48Mi

#### --hidrive-upload-cutoff

Cutoff/Threshold for chunked uploads.

Any files larger than this will be uploaded in chunks of the configured chunksize.

The upper limit for this is 2147483647 bytes (about 2.000Gi).
That is the maximum amount of bytes a single upload-operation will support.
Setting this above the upper limit will cause uploads to fail.

Properties:

- Config:      upload_cutoff
- Env Var:     RCLONE_HIDRIVE_UPLOAD_CUTOFF
- Type:        SizeSuffix
- Default:     96Mi

#### --hidrive-upload-concurrency

Concurrency for chunked uploads.

This is the upper limit for how many transfers for the same file are running concurrently.
Setting this above to a value smaller than 1 will cause uploads to deadlock.

If you are uploading small numbers of large files over high-speed links
and these uploads do not fully utilize your bandwidth, then increasing
this may help to speed up the transfers.

Properties:

- Config:      upload_concurrency
- Env Var:     RCLONE_HIDRIVE_UPLOAD_CONCURRENCY
- Type:        int
- Default:     4

#### --hidrive-encoding

The encoding for the backend.

See the [encoding section in the overview](/overview/#encoding) for more info.

Properties:

- Config:      encoding
- Env Var:     RCLONE_HIDRIVE_ENCODING
- Type:        Encoding
- Default:     Slash,Dot

{{< rem autogenerated options stop >}}

## Limitations

### Symbolic links

HiDrive is able to store symbolic links (*symlinks*) by design,
for example, when unpacked from a zip archive.

There exists no direct mechanism to manage native symlinks in remotes.
As such this implementation has chosen to ignore any native symlinks present in the remote.
rclone will not be able to access or show any symlinks stored in the hidrive-remote.
This means symlinks cannot be individually removed, copied, or moved,
except when removing, copying, or moving the parent folder.

*This does not affect the `.rclonelink`-files
that rclone uses to encode and store symbolic links.*

### Sparse files

It is possible to store sparse files in HiDrive.

Note that copying a sparse file will expand the holes
into null-byte (0x00) regions that will then consume disk space.
Likewise, when downloading a sparse file,
the resulting file will have null-byte regions in the place of file holes.
