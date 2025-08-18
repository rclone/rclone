---
title: "FileJump"
description: "Rclone docs for FileJump"
versionIntroduced: "v1.71"
status: Beta
---

# {{< icon "fa fa-cloud" >}} FileJump

**⚠️ WARNING: The FileJump backend is new and experimental. While it can be tested and used, it should not be used for important data or production environments. Please use with caution and ensure you have proper backups.**

Paths are specified as `remote:path`

Paths may be as deep as required, e.g. `remote:directory/subdirectory`.

The initial setup for FileJump involves getting an API access token from your FileJump account settings. `rclone config` walks you through it.

## Configuration

Here is an example of how to make a remote called `remote`. First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found, make a new one?
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / FileJump
   \ "filejump"
[snip]
Storage> filejump
** See help for filejump backend at: https://rclone.org/filejump/ **

You should create an API access token on your FileJump server in Account Settings (top right) → Developers → API access tokens. For example:
- https://drive.filejump.com/account-settings
- https://app.filejump.com/account-settings
- https://eu.filejump.com/account-settings
Enter a string value. Press Enter for the default ("").
access_token> your_api_token_here
Enter the domain of your FileJump server (check your browser's address bar when logged in)
Choose a number from below, or type in your own value.
 1 > drive.filejump.com
 2 > app.filejump.com
 3 > eu.filejump.com
api_domain> 3
Edit advanced config? (y/n)
y) Yes
n) No (default)
y/n> n
Remote config
--------------------
[remote]
- type: filejump
- access_token: your_api_token_here
- api_domain: eu.filejump.com
--------------------
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d> y
```

You can then use it like this:

Test with

    rclone lsd remote:

List directories in top level of your FileJump

    rclone ls remote:

List all the files in your FileJump

    rclone copy /home/local/directory remote:directory

Copy `/home/local/directory` to the remote directory called `directory`.

### Testing status per FileJump server

- drive.filejump.com: tested during development; account available.
- eu.filejump.com: tested during development; account available.
- app.filejump.com: not tested due to lack of an account. FileJump first launched drive.filejump.com, then app.filejump.com, and the newest deployment is eu.filejump.com. If you have an app.filejump.com account you don't need, testing rclone on this server would be helpful so we can verify and, if necessary, fix the backend. Please open an issue or PR.

### Getting your API access token

To configure rclone with FileJump you will need to get an API access token.

1. Login to your FileJump account at your FileJump domain (e.g. https://drive.filejump.com, https://app.filejump.com, or https://eu.filejump.com)
2. Go to Account Settings by clicking on your profile in the top right corner
3. Navigate to the API section
4. Create a new API access token
5. Copy the token and paste it into rclone when prompted

### Workspaces

FileJump supports workspaces for organizing your files. By default, rclone will use your personal workspace (`workspace_id = 0`). If you want to use a different workspace, you need to provide its ID.

To get a list of your available workspaces and their IDs, you can temporarily set `workspace_id = -1` in your rclone configuration. When you next run an rclone command (e.g., `rclone lsd remote:`), rclone will print a list of all available workspaces and their corresponding IDs, then abort. You can then update your configuration with the correct ID.

### Root folder ID

You can set the `root_folder_id` to point to a non-root folder.

When you do this, rclone won't be able to see any files in the root folder or any folders above the one you specified.

### Invalid refresh token

If you get an error about an invalid refresh token, this is usually because you have revoked the API access token in your FileJump account settings. You will need to create a new token and update your rclone config.

### Modification times

FileJump doesn't support setting custom modification times via the API. The modification time will be set to the upload time by the server. You may want to use the `--update` flag for a more accurate sync.

### Duplicates

FileJump allows duplicate file names in the same directory. When rclone uploads a file, it will first remove any existing files with the same name to avoid duplicates.

### Restricted filenames

FileJump has a minimum filename length requirement of 3 characters. Rclone will automatically pad shorter filenames with spaces on upload to meet this requirement. It will also remove the spaces on download. This means if you happen to have a 1 or 2 character file name ending in a space then syncing may go wrong.

### Trash and file deletion

By default, when rclone deletes files from FileJump, they are moved to the FileJump trash instead of being permanently deleted. This provides a safety net in case files are accidentally deleted.

If you want to permanently delete files instead of moving them to trash, you can set the `hard_delete` option to `true` in your configuration:

```
rclone config set remote hard_delete true
```

To empty the FileJump trash, you can use the cleanup command:

```
rclone cleanup remote:
```

This will permanently delete all files currently in the FileJump trash. Due to a bug in the FileJump API which can cause errors when deleting a very large number of files at once, rclone performs the cleanup by deleting files in batches of 100. This makes the process more reliable.

{{< rem autogenerated options start" - DO NOT EDIT - instead edit fs.RegInfo in backend/filejump/filejump.go then run make backenddocs" >}}
### Standard options

Here are the Standard options specific to filejump (FileJump).

#### --filejump-access-token

You should create an API access token in your Account Settings (top right) -> Developers -> API access tokens

Properties:

- Config:      access_token
- Env Var:     RCLONE_FILEJUMP_ACCESS_TOKEN
- Type:        string
- Required:    true

#### --filejump-api-domain

Enter the domain of your FileJump server (check your browser's address bar when logged in)

Properties:

- Config:      api_domain
- Env Var:     RCLONE_FILEJUMP_API_DOMAIN
- Type:        string
- Required:    true
- Examples:
    - "drive.filejump.com"
        - 
    - "app.filejump.com"
        - 
    - "eu.filejump.com"
        - 

### Advanced options

Here are the Advanced options specific to filejump (FileJump).

#### --filejump-workspace-id

The ID of the workspace to use. Defaults to personal workspace (0). Set to -1 to list available workspaces on first use.

Properties:

- Config:      workspace_id
- Env Var:     RCLONE_FILEJUMP_WORKSPACE_ID
- Type:        string
- Default:     "0"

#### --filejump-upload-cutoff

Cutoff for switching to multipart upload (>= 50 MiB).

Properties:

- Config:      upload_cutoff
- Env Var:     RCLONE_FILEJUMP_UPLOAD_CUTOFF
- Type:        SizeSuffix
- Default:     50Mi

#### --filejump-hard-delete

Delete files permanently instead of moving them to trash.

Properties:

- Config:      hard_delete
- Env Var:     RCLONE_FILEJUMP_HARD_DELETE
- Type:        bool
- Default:     false

#### --filejump-encoding

The encoding for the backend.

See the [encoding section in the overview](/overview/#encoding) for more info.

Properties:

- Config:      encoding
- Env Var:     RCLONE_FILEJUMP_ENCODING
- Type:        Encoding
- Default:     Slash,Del,Ctl,LeftSpace,RightSpace,InvalidUtf8,Dot

#### --filejump-description

Description of the remote.

Properties:

- Config:      description
- Env Var:     RCLONE_FILEJUMP_DESCRIPTION
- Type:        string
- Required:    false

{{< rem autogenerated options stop >}}

### Specific Backend Commands

Here are the specific backend commands for `filejump`.

Run `rclone help backend filejump` to see the full list of commands.

| Command | Arguments | Description |
|---|---|---|
| about | | Gets the space usage of the drive. |
| publiclink | `remote:path/to/file` | Generates a public link to a file. |

## Limitations

FileJump has some limitations that affect rclone usage:

### File operations

- **Copy**: ✅ Supported via server-side copy operations
- **Move**: ✅ Supported via server-side move operations
- **Directory Move**: ✅ Supported via server-side move operations
- **Cleanup**: ✅ Supported - empties the FileJump trash

### Filename restrictions

- Minimum filename length: 3 characters (automatically padded by rclone)
- Standard filename encoding restrictions apply

### Modification times

- FileJump doesn't support setting custom modification times
- Upload time is used as the modification time
- Using the --update flag can improve sync accuracy

### Hash support

Note that FileJump does not currently support standard file hashes (e.g. MD5, SHA1). The hash value returned by the API appears to be a placeholder (a base64 encoding of the file/folder ID with a `|pa` suffix) and should not be relied upon for integrity checking. This may be subject to change in the future.
Integrity checking relies on file size only.

### Range requests

- For `eu.filejump.com`, native server-side HTTP range reads are unfortunately not supported. Therefore, rclone simulates range reads by downloading the full file once and serving the requested range from that stream. This can increase bandwidth usage for random-access workloads.
- For other domains, range requests are handled by the server via signed URLs where available.

### Workspace limitations

- Each workspace is isolated
- Files cannot be moved between workspaces via rclone
- Workspace access depends on your FileJump account permissions

### Upload mechanism

The choice of `api_domain` affects how rclone uploads and downloads files:
- `eu.filejump.com`: uploads use the TUS protocol; downloads go through a direct API endpoint. This difference is necessary because the server-side protocol changed between older and newer deployments.
- `drive.filejump.com` and `app.filejump.com`: uploads use S3 presigned URLs; downloads may redirect to a signed URL.