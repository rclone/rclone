---
title: "Dropbox"
description: "Rclone docs for Dropbox"
versionIntroduced: "v1.02"
---

# {{< icon "fab fa-dropbox" >}} Dropbox

Paths are specified as `remote:path`

Dropbox paths may be as deep as required, e.g.
`remote:directory/subdirectory`.

## Configuration

The initial setup for dropbox involves getting a token from Dropbox
which you need to do in your browser.  `rclone config` walks you
through it.

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
n) New remote
d) Delete remote
q) Quit config
e/n/d/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / Dropbox
   \ "dropbox"
[snip]
Storage> dropbox
Dropbox App Key - leave blank normally.
app_key>
Dropbox App Secret - leave blank normally.
app_secret>
Remote config
Please visit:
https://www.dropbox.com/1/oauth2/authorize?client_id=XXXXXXXXXXXXXXX&response_type=code
Enter the code: XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX_XXXXXXXXXX
--------------------
[remote]
app_key =
app_secret =
token = XXXXXXXXXXXXXXXXXXXXXXXXXXXXX_XXXX_XXXXXXXXXXXXXXXXXXXXXXXXXXXXX
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

See the [remote setup docs](/remote_setup/) for how to set it up on a
machine with no Internet browser available.

Note that rclone runs a webserver on your local machine to collect the
token as returned from Dropbox. This only
runs from the moment it opens your browser to the moment you get back
the verification code.  This is on `http://127.0.0.1:53682/` and it
may require you to unblock it temporarily if you are running a host
firewall, or use manual mode.

You can then use it like this,

List directories in top level of your dropbox

    rclone lsd remote:

List all the files in your dropbox

    rclone ls remote:

To copy a local directory to a dropbox directory called backup

    rclone copy /home/source remote:backup

### Dropbox for business

Rclone supports Dropbox for business and Team Folders.

When using Dropbox for business `remote:` and `remote:path/to/file`
will refer to your personal folder.

If you wish to see Team Folders you must use a leading `/` in the
path, so `rclone lsd remote:/` will refer to the root and show you all
Team Folders and your User Folder.

You can then use team folders like this `remote:/TeamFolder` and
`remote:/TeamFolder/path/to/file`.

A leading `/` for a Dropbox personal account will do nothing, but it
will take an extra HTTP transaction so it should be avoided.

### Modification times and hashes

Dropbox supports modified times, but the only way to set a
modification time is to re-upload the file.

This means that if you uploaded your data with an older version of
rclone which didn't support the v2 API and modified times, rclone will
decide to upload all your old data to fix the modification times.  If
you don't want this to happen use `--size-only` or `--checksum` flag
to stop it.

Dropbox supports [its own hash
type](https://www.dropbox.com/developers/reference/content-hash) which
is checked for all transfers.

### Restricted filename characters

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| NUL       | 0x00  | ␀           |
| /         | 0x2F  | ／           |
| DEL       | 0x7F  | ␡           |
| \         | 0x5C  | ＼           |

File names can also not end with the following characters.
These only get replaced if they are the last character in the name:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| SP        | 0x20  | ␠           |

Invalid UTF-8 bytes will also be [replaced](/overview/#invalid-utf8),
as they can't be used in JSON strings.

### Batch mode uploads {#batch-mode}

Using batch mode uploads is very important for performance when using
the Dropbox API. See [the dropbox performance guide](https://developers.dropbox.com/dbx-performance-guide)
for more info.

There are 3 modes rclone can use for uploads.

#### --dropbox-batch-mode off

In this mode rclone will not use upload batching. This was the default
before rclone v1.55. It has the disadvantage that it is very likely to
encounter `too_many_requests` errors like this

    NOTICE: too_many_requests/.: Too many requests or write operations. Trying again in 15 seconds.

When rclone receives these it has to wait for 15s or sometimes 300s
before continuing which really slows down transfers.

This will happen especially if `--transfers` is large, so this mode
isn't recommended except for compatibility or investigating problems.

#### --dropbox-batch-mode sync

In this mode rclone will batch up uploads to the size specified by
`--dropbox-batch-size` and commit them together.

Using this mode means you can use a much higher `--transfers`
parameter (32 or 64 works fine) without receiving `too_many_requests`
errors.

This mode ensures full data integrity.

Note that there may be a pause when quitting rclone while rclone
finishes up the last batch using this mode.

#### --dropbox-batch-mode async

In this mode rclone will batch up uploads to the size specified by
`--dropbox-batch-size` and commit them together.

However it will not wait for the status of the batch to be returned to
the caller. This means rclone can use a much bigger batch size (much
bigger than `--transfers`), at the cost of not being able to check the
status of the upload.

This provides the maximum possible upload speed especially with lots
of small files, however rclone can't check the file got uploaded
properly using this mode.

If you are using this mode then using "rclone check" after the
transfer completes is recommended. Or you could do an initial transfer
with `--dropbox-batch-mode async` then do a final transfer with
`--dropbox-batch-mode sync` (the default).

Note that there may be a pause when quitting rclone while rclone
finishes up the last batch using this mode.


{{< rem autogenerated options start" - DO NOT EDIT - instead edit fs.RegInfo in backend/dropbox/dropbox.go then run make backenddocs" >}}
### Standard options

Here are the Standard options specific to dropbox (Dropbox).

#### --dropbox-client-id

OAuth Client Id.

Leave blank normally.

Properties:

- Config:      client_id
- Env Var:     RCLONE_DROPBOX_CLIENT_ID
- Type:        string
- Required:    false

#### --dropbox-client-secret

OAuth Client Secret.

Leave blank normally.

Properties:

- Config:      client_secret
- Env Var:     RCLONE_DROPBOX_CLIENT_SECRET
- Type:        string
- Required:    false

### Advanced options

Here are the Advanced options specific to dropbox (Dropbox).

#### --dropbox-token

OAuth Access Token as a JSON blob.

Properties:

- Config:      token
- Env Var:     RCLONE_DROPBOX_TOKEN
- Type:        string
- Required:    false

#### --dropbox-auth-url

Auth server URL.

Leave blank to use the provider defaults.

Properties:

- Config:      auth_url
- Env Var:     RCLONE_DROPBOX_AUTH_URL
- Type:        string
- Required:    false

#### --dropbox-token-url

Token server url.

Leave blank to use the provider defaults.

Properties:

- Config:      token_url
- Env Var:     RCLONE_DROPBOX_TOKEN_URL
- Type:        string
- Required:    false

#### --dropbox-chunk-size

Upload chunk size (< 150Mi).

Any files larger than this will be uploaded in chunks of this size.

Note that chunks are buffered in memory (one at a time) so rclone can
deal with retries.  Setting this larger will increase the speed
slightly (at most 10% for 128 MiB in tests) at the cost of using more
memory.  It can be set smaller if you are tight on memory.

Properties:

- Config:      chunk_size
- Env Var:     RCLONE_DROPBOX_CHUNK_SIZE
- Type:        SizeSuffix
- Default:     48Mi

#### --dropbox-impersonate

Impersonate this user when using a business account.

Note that if you want to use impersonate, you should make sure this
flag is set when running "rclone config" as this will cause rclone to
request the "members.read" scope which it won't normally. This is
needed to lookup a members email address into the internal ID that
dropbox uses in the API.

Using the "members.read" scope will require a Dropbox Team Admin
to approve during the OAuth flow.

You will have to use your own App (setting your own client_id and
client_secret) to use this option as currently rclone's default set of
permissions doesn't include "members.read". This can be added once
v1.55 or later is in use everywhere.


Properties:

- Config:      impersonate
- Env Var:     RCLONE_DROPBOX_IMPERSONATE
- Type:        string
- Required:    false

#### --dropbox-shared-files

Instructs rclone to work on individual shared files.

In this mode rclone's features are extremely limited - only list (ls, lsl, etc.) 
operations and read operations (e.g. downloading) are supported in this mode.
All other operations will be disabled.

Properties:

- Config:      shared_files
- Env Var:     RCLONE_DROPBOX_SHARED_FILES
- Type:        bool
- Default:     false

#### --dropbox-shared-folders

Instructs rclone to work on shared folders.
			
When this flag is used with no path only the List operation is supported and 
all available shared folders will be listed. If you specify a path the first part 
will be interpreted as the name of shared folder. Rclone will then try to mount this 
shared to the root namespace. On success shared folder rclone proceeds normally. 
The shared folder is now pretty much a normal folder and all normal operations 
are supported. 

Note that we don't unmount the shared folder afterwards so the 
--dropbox-shared-folders can be omitted after the first use of a particular 
shared folder.

Properties:

- Config:      shared_folders
- Env Var:     RCLONE_DROPBOX_SHARED_FOLDERS
- Type:        bool
- Default:     false

#### --dropbox-pacer-min-sleep

Minimum time to sleep between API calls.

Properties:

- Config:      pacer_min_sleep
- Env Var:     RCLONE_DROPBOX_PACER_MIN_SLEEP
- Type:        Duration
- Default:     10ms

#### --dropbox-encoding

The encoding for the backend.

See the [encoding section in the overview](/overview/#encoding) for more info.

Properties:

- Config:      encoding
- Env Var:     RCLONE_DROPBOX_ENCODING
- Type:        Encoding
- Default:     Slash,BackSlash,Del,RightSpace,InvalidUtf8,Dot

#### --dropbox-batch-mode

Upload file batching sync|async|off.

This sets the batch mode used by rclone.

For full info see [the main docs](https://rclone.org/dropbox/#batch-mode)

This has 3 possible values

- off - no batching
- sync - batch uploads and check completion (default)
- async - batch upload and don't check completion

Rclone will close any outstanding batches when it exits which may make
a delay on quit.


Properties:

- Config:      batch_mode
- Env Var:     RCLONE_DROPBOX_BATCH_MODE
- Type:        string
- Default:     "sync"

#### --dropbox-batch-size

Max number of files in upload batch.

This sets the batch size of files to upload. It has to be less than 1000.

By default this is 0 which means rclone which calculate the batch size
depending on the setting of batch_mode.

- batch_mode: async - default batch_size is 100
- batch_mode: sync - default batch_size is the same as --transfers
- batch_mode: off - not in use

Rclone will close any outstanding batches when it exits which may make
a delay on quit.

Setting this is a great idea if you are uploading lots of small files
as it will make them a lot quicker. You can use --transfers 32 to
maximise throughput.


Properties:

- Config:      batch_size
- Env Var:     RCLONE_DROPBOX_BATCH_SIZE
- Type:        int
- Default:     0

#### --dropbox-batch-timeout

Max time to allow an idle upload batch before uploading.

If an upload batch is idle for more than this long then it will be
uploaded.

The default for this is 0 which means rclone will choose a sensible
default based on the batch_mode in use.

- batch_mode: async - default batch_timeout is 10s
- batch_mode: sync - default batch_timeout is 500ms
- batch_mode: off - not in use


Properties:

- Config:      batch_timeout
- Env Var:     RCLONE_DROPBOX_BATCH_TIMEOUT
- Type:        Duration
- Default:     0s

#### --dropbox-batch-commit-timeout

Max time to wait for a batch to finish committing

Properties:

- Config:      batch_commit_timeout
- Env Var:     RCLONE_DROPBOX_BATCH_COMMIT_TIMEOUT
- Type:        Duration
- Default:     10m0s

{{< rem autogenerated options stop >}}

## Limitations

Note that Dropbox is case insensitive so you can't have a file called
"Hello.doc" and one called "hello.doc".

There are some file names such as `thumbs.db` which Dropbox can't
store.  There is a full list of them in the ["Ignored Files" section
of this document](https://www.dropbox.com/en/help/145).  Rclone will
issue an error message `File name disallowed - not uploading` if it
attempts to upload one of those file names, but the sync won't fail.

Some errors may occur if you try to sync copyright-protected files
because Dropbox has its own [copyright detector](https://techcrunch.com/2014/03/30/how-dropbox-knows-when-youre-sharing-copyrighted-stuff-without-actually-looking-at-your-stuff/) that
prevents this sort of file being downloaded. This will return the error `ERROR :
/path/to/your/file: Failed to copy: failed to open source object:
path/restricted_content/.`

If you have more than 10,000 files in a directory then `rclone purge
dropbox:dir` will return the error `Failed to purge: There are too
many files involved in this operation`.  As a work-around do an
`rclone delete dropbox:dir` followed by an `rclone rmdir dropbox:dir`.

When using `rclone link` you'll need to set `--expire` if using a
non-personal account otherwise the visibility may not be correct.
(Note that `--expire` isn't supported on personal accounts). See the
[forum discussion](https://forum.rclone.org/t/rclone-link-dropbox-permissions/23211) and the 
[dropbox SDK issue](https://github.com/dropbox/dropbox-sdk-go-unofficial/issues/75).

## Get your own Dropbox App ID

When you use rclone with Dropbox in its default configuration you are using rclone's App ID. This is shared between all the rclone users.

Here is how to create your own Dropbox App ID for rclone:

1. Log into the [Dropbox App console](https://www.dropbox.com/developers/apps/create) with your Dropbox Account (It need not
to be the same account as the Dropbox you want to access)

2. Choose an API => Usually this should be `Dropbox API`

3. Choose the type of access you want to use => `Full Dropbox` or `App Folder`. If you want to use Team Folders, `Full Dropbox` is required ([see here](https://www.dropboxforum.com/t5/Dropbox-API-Support-Feedback/How-to-create-team-folder-inside-my-app-s-folder/m-p/601005/highlight/true#M27911)).

4. Name your App. The app name is global, so you can't use `rclone` for example

5. Click the button `Create App`

6. Switch to the `Permissions` tab. Enable at least the following permissions: `account_info.read`, `files.metadata.write`, `files.content.write`, `files.content.read`, `sharing.write`. The `files.metadata.read` and `sharing.read` checkboxes will be marked too. Click `Submit`

7. Switch to the `Settings` tab. Fill `OAuth2 - Redirect URIs` as `http://localhost:53682/` and click on `Add`

8. Find the `App key` and `App secret` values on the `Settings` tab. Use these values in rclone config to add a new remote or edit an existing remote. The `App key` setting corresponds to `client_id` in rclone config, the `App secret` corresponds to `client_secret`
