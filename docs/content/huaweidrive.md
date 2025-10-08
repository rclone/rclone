---
title: "Huawei Drive"
description: "Rclone docs for Huawei Drive"
versionIntroduced: "v1.68"
---

# {{< icon "fas fa-cloud" >}} Huawei Drive

Paths are specified as `huaweidrive:path`

Huawei Drive paths may be as deep as required, e.g. `huaweidrive:directory/subdirectory`.

## Configuration

The initial setup for Huawei Drive involves getting a token from Huawei Drive
which you need to do in your browser. `rclone config` walks you
through it.

Here is an example of how to make a remote called `remote`. First run:

```sh
rclone config
```

This will guide you through an interactive setup process:

```text
No remotes found, make a new one?
n) New remote
r) Rename remote
c) Copy remote
s) Set configuration password
q) Quit config
n/r/c/s/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / Huawei Drive
   \ "huaweidrive"
[snip]
Storage> huaweidrive
Huawei OAuth Client Id - leave blank to use rclone's default.
client_id> 
Huawei OAuth Client Secret - leave blank to use rclone's default.
client_secret>
Remote config
Use web browser to automatically authenticate rclone with remote?
 * Say Y if the machine running rclone has a web browser you can use
 * Say N if running rclone on a (remote) machine without web browser access
If not sure try Y. If Y failed, try N.
y) Yes
n) No
y/n> y
If your browser doesn't open automatically go to the following link: http://127.0.0.1:53682/auth
Log in and authorize rclone for access
Waiting for code...
Got code
Configuration complete.
Options:
- type: huaweidrive
- client_id: 
- client_secret: 
- token: {"access_token":"XXX","token_type":"Bearer","refresh_token":"XXX","expiry":"2024-03-16T13:57:58.955387075Z"}
Keep this "remote" remote?
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

See the [remote setup docs](/remote_setup/) for how to set it up on a
machine without an internet-connected web browser available.

Note that rclone runs a webserver on your local machine to collect the
token as returned from Huawei if using web browser to automatically
authenticate. This only runs from the moment it opens your browser to the moment you get back
the verification code. This is on `http://127.0.0.1:53682/` and it
may require you to unblock it temporarily if you are running a host
firewall, or use manual mode.

You can then use it like this,

List directories in top level of your drive

```sh
rclone lsd remote:
```

List all the files in your drive

```sh
rclone ls remote:
```

To copy a local directory to a drive directory called backup

```sh
rclone copy /home/source remote:backup
```

### Getting your own Client ID and Secret

When you use rclone with Huawei Drive in its default configuration you
are using rclone's client_id. This is shared between all the rclone
users. There is a global rate limit on the number of queries per
second that each client_id can do set by Huawei.

It is strongly recommended to use your own client ID as the default rclone ID is heavily used. If you have multiple services running, it is recommended to use a separate client ID for each service to avoid rate limiting issues.

Here is how to create your own Huawei Drive client ID for rclone:

Follow the detailed guide at [Huawei HMS Core Web App Preparations](https://developer.huawei.com/consumer/cn/doc/HMSCore-Guides/web-preparations-0000001050050891) for complete setup instructions.

Key steps summary:

1. Log into the [Huawei Developer Console](https://developer.huawei.com/consumer/en/console) with your Huawei account.

2. Create a new project or select an existing project.

3. Go to "Manage APIs" and enable the "Drive Kit" API.

4. Click on "Credentials" in the left side panel.

5. Click "Create Credentials" and choose "OAuth client ID".

6. Choose "Web application" as the application type.

7. Add `http://localhost:53682/` to the "Authorized redirect URIs".

8. Click "Create" and note down the Client ID and Client Secret.

9. Provide the noted client ID and client secret to rclone during configuration.

### Scopes

Rclone uses the following OAuth2 scopes when accessing Huawei Drive:

- `https://www.huawei.com/auth/drive` - Full access to Huawei Drive files (excluding application data folder)
- `openid` - OpenID Connect authentication 
- `profile` - Access to basic profile information

Alternative scopes available (not used by rclone by default):
- `https://www.huawei.com/auth/drive.file` - Access only to files created by the application
- `https://www.huawei.com/auth/drive.appdata` - Access to application data folder
- `https://www.huawei.com/auth/drive.readonly` - Read-only access to file metadata and content
- `https://www.huawei.com/auth/drive.metadata` - Read/write access to file metadata only
- `https://www.huawei.com/auth/drive.metadata.readonly` - Read-only access to file metadata only

The default scopes provide read/write access to all files in your Huawei Drive.

### Modification times and hashes

Huawei Drive stores modification times accurate to 1 second.

Hash algorithm SHA256 is supported for file integrity verification.

### Restricted filename characters

Huawei Drive has strict restrictions on file and directory names:

- Cannot contain: `< > | : " * ? / \` or emoji characters
- Cannot be exactly `. ..`
- Maximum length: 250 bytes
- Single quotes must be escaped with `\'` in API calls

These restrictions are handled by rclone's encoding:
- Backslashes `\` are encoded
- Invalid UTF-8 sequences are encoded  
- Right-side spaces are encoded

Invalid characters will be [replaced](/overview/#invalid-utf8) according to the encoding settings.

### Resumable uploads

Huawei Drive supports resumable uploads for files larger than the upload cutoff (default 20MB).
Files are uploaded in chunks which allows for recovery from network interruptions.

### File size limits

The maximum file size for Huawei Drive is 50 GiB.

### Quota information

To view your current quota you can use the `rclone about remote:`
command which will display your usage and quota information.

{{< rem autogenerated options start" - DO NOT EDIT - instead edit fs.RegInfo in backend/huaweidrive/huaweidrive.go then run make backenddocs" >}}
### Standard options

Here are the Standard options specific to huaweidrive (Huawei Drive).

#### --huaweidrive-client-id

Huawei OAuth Client Id.

Leave blank normally.

Properties:

- Config:      client_id
- Env Var:     RCLONE_HUAWEIDRIVE_CLIENT_ID
- Type:        string
- Required:    false

#### --huaweidrive-client-secret

Huawei OAuth Client Secret.

Leave blank normally.

Properties:

- Config:      client_secret
- Env Var:     RCLONE_HUAWEIDRIVE_CLIENT_SECRET
- Type:        string
- Required:    false

### Advanced options

Here are the Advanced options specific to huaweidrive (Huawei Drive).

#### --huaweidrive-token

OAuth Access Token as a JSON blob.

Properties:

- Config:      token
- Env Var:     RCLONE_HUAWEIDRIVE_TOKEN
- Type:        string
- Required:    false

#### --huaweidrive-auth-url

Auth server URL.

Leave blank to use the provider defaults.

Properties:

- Config:      auth_url
- Env Var:     RCLONE_HUAWEIDRIVE_AUTH_URL
- Type:        string
- Required:    false

#### --huaweidrive-token-url

Token server url.

Leave blank to use the provider defaults.

Properties:

- Config:      token_url
- Env Var:     RCLONE_HUAWEIDRIVE_TOKEN_URL
- Type:        string
- Required:    false

#### --huaweidrive-chunk-size

Upload chunk size.

Must be a power of 2 >= 256k and <= 64MB.

Properties:

- Config:      chunk_size
- Env Var:     RCLONE_HUAWEIDRIVE_CHUNK_SIZE
- Type:        SizeSuffix
- Default:     8Mi

#### --huaweidrive-list-chunk

Size of listing chunk 1-1000.

Properties:

- Config:      list_chunk
- Env Var:     RCLONE_HUAWEIDRIVE_LIST_CHUNK
- Type:        int
- Default:     1000

#### --huaweidrive-upload-cutoff

Cutoff for switching to multipart upload.

Any files larger than this will be uploaded in chunks of chunk_size.
The minimum is 0 and the maximum is 5 GiB.

Properties:

- Config:      upload_cutoff
- Env Var:     RCLONE_HUAWEIDRIVE_UPLOAD_CUTOFF
- Type:        SizeSuffix
- Default:     20Mi

#### --huaweidrive-encoding

The encoding for the backend.

See the [encoding section in the overview](/overview/#encoding) for more info.

Properties:

- Config:      encoding
- Env Var:     RCLONE_HUAWEIDRIVE_ENCODING
- Type:        Encoding
- Default:     BackSlash,InvalidUtf8,RightSpace

{{< rem autogenerated options stop >}}

## Limitations

Huawei Drive has some rate limiting which may cause rclone to slow down transfers to avoid hitting the limits.

### File name restrictions

- File names cannot contain: `< > | : " * ? / \`
- File names cannot be exactly `. ..`
- Maximum file name length is 250 bytes
- Some unicode characters including emojis are not supported

### API rate limits  

Huawei Drive imposes API rate limits. Rclone automatically handles these by backing off when limits are hit.

### OAuth2 token expiry

- Access tokens expire after 1 hour
- Refresh tokens expire after 180 days  
- Rclone will automatically refresh tokens as needed

### File size limits

- Maximum file size is 50 GiB
- Files larger than this cannot be uploaded

### Upload behavior

- Files smaller than `upload_cutoff` (default 20MB) are uploaded in a single request using content or multipart method
- Larger files are uploaded using resumable uploads with configurable chunk size
- Maximum single chunk size for resumable uploads is 64MB
- Resumable uploads support automatic retry on network errors
- SHA256 hash verification is supported for uploaded files

### Directory operations

- Huawei Drive supports creating empty directories
- Directory modification times are not preserved
- Recursive directory operations may be slow for large directory trees due to API limitations
- Huawei Drive has a special "applicationData" folder for app-specific data (not accessible via rclone's default configuration)

## Error handling

Common errors and their meanings:

- `401 Unauthorized`: Invalid credentials or expired token
- `403 Forbidden`: Usually indicates insufficient permissions or API quota exceeded
- `404 Not Found`: File or directory does not exist
- `409 Conflict`: Directory already exists
- `429 Too Many Requests`: API rate limit exceeded, rclone will automatically retry

## Performance tuning

For better performance:

- Increase `--transfers` for concurrent operations (default is 4)
- Adjust `chunk_size` based on your network speed and file sizes
- Use `--fast-list` for operations on directories with many files (currently not implemented)

## Making backups

Huawei Drive is suitable for backups with some considerations:

- Use `rclone sync` rather than `rclone copy` to maintain exact directory structure
- Consider using `--backup-dir` to preserve deleted files
- Verify transfers with `--checksum` flag for critical data
- Be aware of the 50 GiB file size limit for individual files

## Integration with other services

Huawei Drive can be used as a backend for various rclone operations:

- Mount as a local filesystem using `rclone mount`
- Serve over HTTP using `rclone serve http`
- Serve over FTP using `rclone serve ftp`
- Serve over WebDAV using `rclone serve webdav`

Note that some operations may be slower than local storage due to API limitations and network latency.