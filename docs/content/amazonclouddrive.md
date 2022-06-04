---
title: "Amazon Drive"
description: "Rclone docs for Amazon Drive"
---

# {{< icon "fab fa-amazon" >}} Amazon Drive

Amazon Drive, formerly known as Amazon Cloud Drive, is a cloud storage
service run by Amazon for consumers.

## Status

**Important:** rclone supports Amazon Drive only if you have your own
set of API keys. Unfortunately the [Amazon Drive developer
program](https://developer.amazon.com/amazon-drive) is now closed to
new entries so if you don't already have your own set of keys you will
not be able to use rclone with Amazon Drive.

For the history on why rclone no longer has a set of Amazon Drive API
keys see [the forum](https://forum.rclone.org/t/rclone-has-been-banned-from-amazon-drive/2314).

If you happen to know anyone who works at Amazon then please ask them
to re-instate rclone into the Amazon Drive developer program - thanks!

## Configuration

The initial setup for Amazon Drive involves getting a token from
Amazon which you need to do in your browser.  `rclone config` walks
you through it.

The configuration process for Amazon Drive may involve using an [oauth
proxy](https://github.com/ncw/oauthproxy). This is used to keep the
Amazon credentials out of the source code.  The proxy runs in Google's
very secure App Engine environment and doesn't store any credentials
which pass through it.

Since rclone doesn't currently have its own Amazon Drive credentials
so you will either need to have your own `client_id` and
`client_secret` with Amazon Drive, or use a third-party oauth proxy
in which case you will need to enter `client_id`, `client_secret`,
`auth_url` and `token_url`.

Note also if you are not using Amazon's `auth_url` and `token_url`,
(ie you filled in something for those) then if setting up on a remote
machine you can only use the [copying the config method of
configuration](https://rclone.org/remote_setup/#configuring-by-copying-the-config-file)
- `rclone authorize` will not work.

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
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
XX / Amazon Drive
   \ "amazon cloud drive"
[snip]
Storage> amazon cloud drive
Amazon Application Client Id - required.
client_id> your client ID goes here
Amazon Application Client Secret - required.
client_secret> your client secret goes here
Auth server URL - leave blank to use Amazon's.
auth_url> Optional auth URL
Token server url - leave blank to use Amazon's.
token_url> Optional token URL
Remote config
Make sure your Redirect URL is set to "http://127.0.0.1:53682/" in your custom config.
Use auto config?
 * Say Y if not sure
 * Say N if you are working on a remote or headless machine
y) Yes
n) No
y/n> y
If your browser doesn't open automatically go to the following link: http://127.0.0.1:53682/auth
Log in and authorize rclone for access
Waiting for code...
Got code
--------------------
[remote]
client_id = your client ID goes here
client_secret = your client secret goes here
auth_url = Optional auth URL
token_url = Optional token URL
token = {"access_token":"xxxxxxxxxxxxxxxxxxxxxxx","token_type":"bearer","refresh_token":"xxxxxxxxxxxxxxxxxx","expiry":"2015-09-06T16:07:39.658438471+01:00"}
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

See the [remote setup docs](/remote_setup/) for how to set it up on a
machine with no Internet browser available.

Note that rclone runs a webserver on your local machine to collect the
token as returned from Amazon. This only runs from the moment it
opens your browser to the moment you get back the verification
code.  This is on `http://127.0.0.1:53682/` and this it may require
you to unblock it temporarily if you are running a host firewall.

Once configured you can then use `rclone` like this,

List directories in top level of your Amazon Drive

    rclone lsd remote:

List all the files in your Amazon Drive

    rclone ls remote:

To copy a local directory to an Amazon Drive directory called backup

    rclone copy /home/source remote:backup

### Modified time and MD5SUMs

Amazon Drive doesn't allow modification times to be changed via
the API so these won't be accurate or used for syncing.

It does store MD5SUMs so for a more accurate sync, you can use the
`--checksum` flag.

### Restricted filename characters

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| NUL       | 0x00  | ␀           |
| /         | 0x2F  | ／          |

Invalid UTF-8 bytes will also be [replaced](/overview/#invalid-utf8),
as they can't be used in JSON strings.

### Deleting files

Any files you delete with rclone will end up in the trash.  Amazon
don't provide an API to permanently delete files, nor to empty the
trash, so you will have to do that with one of Amazon's apps or via
the Amazon Drive website. As of November 17, 2016, files are
automatically deleted by Amazon from the trash after 30 days.

### Using with non `.com` Amazon accounts

Let's say you usually use `amazon.co.uk`. When you authenticate with
rclone it will take you to an `amazon.com` page to log in.  Your
`amazon.co.uk` email and password should work here just fine.

{{< rem autogenerated options start" - DO NOT EDIT - instead edit fs.RegInfo in backend/amazonclouddrive/amazonclouddrive.go then run make backenddocs" >}}
### Standard options

Here are the standard options specific to amazon cloud drive (Amazon Drive).

#### --acd-client-id

OAuth Client Id.

Leave blank normally.

Properties:

- Config:      client_id
- Env Var:     RCLONE_ACD_CLIENT_ID
- Type:        string
- Required:    false

#### --acd-client-secret

OAuth Client Secret.

Leave blank normally.

Properties:

- Config:      client_secret
- Env Var:     RCLONE_ACD_CLIENT_SECRET
- Type:        string
- Required:    false

### Advanced options

Here are the advanced options specific to amazon cloud drive (Amazon Drive).

#### --acd-token

OAuth Access Token as a JSON blob.

Properties:

- Config:      token
- Env Var:     RCLONE_ACD_TOKEN
- Type:        string
- Required:    false

#### --acd-auth-url

Auth server URL.

Leave blank to use the provider defaults.

Properties:

- Config:      auth_url
- Env Var:     RCLONE_ACD_AUTH_URL
- Type:        string
- Required:    false

#### --acd-token-url

Token server url.

Leave blank to use the provider defaults.

Properties:

- Config:      token_url
- Env Var:     RCLONE_ACD_TOKEN_URL
- Type:        string
- Required:    false

#### --acd-checkpoint

Checkpoint for internal polling (debug).

Properties:

- Config:      checkpoint
- Env Var:     RCLONE_ACD_CHECKPOINT
- Type:        string
- Required:    false

#### --acd-upload-wait-per-gb

Additional time per GiB to wait after a failed complete upload to see if it appears.

Sometimes Amazon Drive gives an error when a file has been fully
uploaded but the file appears anyway after a little while.  This
happens sometimes for files over 1 GiB in size and nearly every time for
files bigger than 10 GiB. This parameter controls the time rclone waits
for the file to appear.

The default value for this parameter is 3 minutes per GiB, so by
default it will wait 3 minutes for every GiB uploaded to see if the
file appears.

You can disable this feature by setting it to 0. This may cause
conflict errors as rclone retries the failed upload but the file will
most likely appear correctly eventually.

These values were determined empirically by observing lots of uploads
of big files for a range of file sizes.

Upload with the "-v" flag to see more info about what rclone is doing
in this situation.

Properties:

- Config:      upload_wait_per_gb
- Env Var:     RCLONE_ACD_UPLOAD_WAIT_PER_GB
- Type:        Duration
- Default:     3m0s

#### --acd-templink-threshold

Files >= this size will be downloaded via their tempLink.

Files this size or more will be downloaded via their "tempLink". This
is to work around a problem with Amazon Drive which blocks downloads
of files bigger than about 10 GiB. The default for this is 9 GiB which
shouldn't need to be changed.

To download files above this threshold, rclone requests a "tempLink"
which downloads the file through a temporary URL directly from the
underlying S3 storage.

Properties:

- Config:      templink_threshold
- Env Var:     RCLONE_ACD_TEMPLINK_THRESHOLD
- Type:        SizeSuffix
- Default:     9Gi

#### --acd-encoding

The encoding for the backend.

See the [encoding section in the overview](/overview/#encoding) for more info.

Properties:

- Config:      encoding
- Env Var:     RCLONE_ACD_ENCODING
- Type:        MultiEncoder
- Default:     Slash,InvalidUtf8,Dot

{{< rem autogenerated options stop >}}

## Limitations

Note that Amazon Drive is case insensitive so you can't have a
file called "Hello.doc" and one called "hello.doc".

Amazon Drive has rate limiting so you may notice errors in the
sync (429 errors).  rclone will automatically retry the sync up to 3
times by default (see `--retries` flag) which should hopefully work
around this problem.

Amazon Drive has an internal limit of file sizes that can be uploaded
to the service. This limit is not officially published, but all files
larger than this will fail.

At the time of writing (Jan 2016) is in the area of 50 GiB per file.
This means that larger files are likely to fail.

Unfortunately there is no way for rclone to see that this failure is
because of file size, so it will retry the operation, as any other
failure. To avoid this problem, use `--max-size 50000M` option to limit
the maximum size of uploaded files. Note that `--max-size` does not split
files into segments, it only ignores files over this size.

`rclone about` is not supported by the Amazon Drive backend. Backends without
this capability cannot determine free space for an rclone mount or
use policy `mfs` (most free space) as a member of an rclone union
remote.

See [List of backends that do not support rclone about](https://rclone.org/overview/#optional-features) and [rclone about](https://rclone.org/commands/rclone_about/)

