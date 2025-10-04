---
title: "Jottacloud"
description: "Rclone docs for Jottacloud"
versionIntroduced: "v1.43"
---

# {{< icon "fa fa-cloud" >}} Jottacloud

Jottacloud is a cloud storage service provider from a Norwegian company, using
its own datacenters in Norway.

In addition to the official service at [jottacloud.com](https://www.jottacloud.com/),
it also provides white-label solutions to different companies. The following
are currently supported by this backend, using a different authentication setup
as described [below](#whitelabel-authentication):

- Elkjøp (with subsidiaries):
  - Elkjøp Cloud (cloud.elkjop.no)
  - Elgiganten Cloud (cloud.elgiganten.dk)
  - Elgiganten Cloud (cloud.elgiganten.se)
  - ELKO Cloud (cloud.elko.is)
  - Gigantti Cloud (cloud.gigantti.fi)
- Telia
  - Telia Cloud (cloud.telia.se)
  - Telia Sky (sky.telia.no)
- Tele2
  - Tele2 Cloud (mittcloud.tele2.se)
- Onlime
  - Onlime (onlime.dk)
- MediaMarkt
  - Let's Go Cloud (letsgo.jotta.cloud)

Paths are specified as `remote:path`

Paths may be as deep as required, e.g. `remote:directory/subdirectory`.

## Authentication

Authentication in Jottacloud is in general based on OAuth 2.0 and OpenID
Connect (OIDC). There are different variants to choose from, described below.
Some of the variants are only supported by the official service and not
white-label services, so this must be taken into consideration when choosing.

To access your account from multiple instances of rclone, you need to configure
each of them separately. E.g. you create a Jottacloud remote with rclone in one
location, and copy the configuration file to a second location where you also
want to run rclone and access the same remote. Then you need to replace the
token for one of them, using the [config reconnect](https://rclone.org/commands/rclone_config_reconnect/)
command. For standard authentication (described below) this means you will have
to generate a new personal login token and supply as input. If you do not do
this, the token may easily end up being invalidated, resulting in both
instances failing with an error message something along the lines of:

```text
  oauth2: cannot fetch token: 400 Bad Request
  Response: {"error":"invalid_grant","error_description":"Stale token"}
```

The background for this is that OAuth tokens from Jottacloud normally have one
hour expiry, after which they will be automatically refreshed by rclone.
Jottacloud will then refresh not only the access token, but also the refresh
token. Any requests using a previous refresh token will be flagged, and lead
to the stale token error. When this happens, you need to replace the token as
described above to be able to use your remote again.

Each time you are granted access with a new token, it will listed in the web
interface under "My logged in devices". From the right side of that list you
can click the "X" button to revoke access. This will effectively disable the
refresh token, which means you will still have access using an existing access
token until that expires, but you will not be able to refresh it.

### Standard

This is an OAuth variant designed for command-line applications. It is
primarily supported by the official service (jottacloud.com), but may also be
supported by some of the white-label services. The specific domain name and
endpoint to connect to are found automatically (it is encoded into the supplied
login token, described next).

When configuring a remote, you are asked to enter a single-use personal login
token, which you must manually generate from the account security settings in
the service's web interface. You do not need a web browser on the same machine
like with traditional OAuth, but need to use a web browser somewhere, and be
able to be copy the generated string into your rclone configuration session.

Log in to your account, go to "Settings" and then "Security", or use the direct
link presented to you by rclone when configuring the remote:
<https://www.jottacloud.com/web/secure>. Scroll down to the section "Personal
login token", and click the "Generate" button. Note that if you are using a
white-label service you probably can't use the direct link, you need to find
the same page in their dedicated web interface, and also it may be in a
different location than described above.

When you have successfully authenticated using a personal login token, which
means you have received a proper OAuth token, there will be an entry in the
"My logged in devices" list in the web interface. It will be listed with
application name "Jottacloud CLI".

### Traditional

Jottacloud also supports a more traditional OAuth variant. Most of the
white-label services supports this, and often only this as they do not support
personal login tokens. This method relies on pre-defined domain names and
endpoints, and rclone must therefore explicitly add any white-label services
that should be supported.

When configuring a remote, you must interactively login to an OAuth
authorization web site, and a one-time authorization code are automatically
sent back to rclone, which it uses to request a token.

Note that when setting this up, you need to be on a machine with an
internet-connected web browser. If you need it on a machine where this is not
the case, then you will have to create the configuration on a different machine
and copy it from there. The jottacloud backend does not support the
`rclone authorize` command. See the [remote setup docs](/remote_setup) for
details.

When you have successfully authenticated, there will be an entry in the
"My logged in devices" list in the web interface. It will typically be listed
with application name "Jottacloud for Desktop" or similar (it depends on the
white-label service configuration).

### Legacy

Originally Jottacloud used an OAuth variant which required your account's
username and password to be specified. When Jottacloud migrated to the newer
methods, some white-label versions (those from Elkjøp) still used this legacy
method for a long time. Currently there are no known uses of this, it is still
supported by rclone, but the support will be removed in a future version.

## Configuration

Here is an example of how to make a remote called `remote` with the default setup.
First run:

```sh
rclone config
```

This will guide you through an interactive setup process:

```text
No remotes found, make a new one?
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n

Enter name for new remote.
name> remote

Option Storage.
Type of storage to configure.
Choose a number from below, or type in your own value.
[snip]
XX / Jottacloud
   \ (jottacloud)
[snip]
Storage> jottacloud

Option client_id.
OAuth Client Id.
Leave blank normally.
Enter a value. Press Enter to leave empty.
client_id>

Option client_secret.
OAuth Client Secret.
Leave blank normally.
Enter a value. Press Enter to leave empty.
client_secret>

Edit advanced config?
y) Yes
n) No (default)
y/n> n

Option config_type.
Type of authentication.
Choose a number from below, or type in an existing value of type string.
Press Enter for the default (standard).
   / Standard authentication.
   | This is primarily supported by the official service, but may also be supported
   | by some of the white-label services. It is designed for command-line
 1 | applications, and you will be asked to enter a single-use personal login token
   | which you must manually generate from the account security settings in the
   | web interface of your service.
   \ (standard)
   / Traditional authentication.
   | This is supported by the official service and most of the white-label
 2 | services, you will be asked which service to connect to. You need to be on
   | a machine with an internet-connected web browser.
   \ (traditional)
   / Legacy authentication.
 3 | This is no longer supported by any known services and not recommended used.
   | You will be asked for your account's username and password.
   \ (legacy)
config_type> 1

Option config_login_token.
Personal login token.
Generate it from the account security settings in the web interface of your
service, for the official service on https://www.jottacloud.com/web/secure.
Enter a value.
config_login_token> <your token here>

Use a non-standard device/mountpoint?
Choosing no, the default, will let you access the storage used for the archive
section of the official Jottacloud client. If you instead want to access the
sync or the backup section, for example, you must choose yes.
y) Yes
n) No (default)
y/n> n

Configuration complete.
Options:
- type: jottacloud
- configVersion: 1
- client_id: jottacli
- client_secret:
- tokenURL: https://id.jottacloud.com/auth/realms/jottacloud/protocol/openid-connect/token
- token: {........}
- username: 2940e57271a93d987d6f8a21
- device: Jotta
- mountpoint: Archive
Keep this "remote" remote?
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d> y
```

Once configured you can then use `rclone` like this,

List directories in top level of your Jottacloud

```sh
rclone lsd remote:
```

List all the files in your Jottacloud

```sh
rclone ls remote:
```

To copy a local directory to an Jottacloud directory called backup

```sh
rclone copy /home/source remote:backup
```

### Devices and Mountpoints

The official Jottacloud client registers a device for each computer you install
it on, and shows them in the backup section of the user interface. For each
folder you select for backup it will create a mountpoint within this device.
A built-in device called Jotta is special, and contains mountpoints Archive,
Sync and some others, used for corresponding features in official clients.

With rclone you'll want to use the standard Jotta/Archive device/mountpoint in
most cases. However, you may for example want to access files from the sync or
backup functionality provided by the official clients, and rclone therefore
provides the option to select other devices and mountpoints during config.

You are allowed to create new devices and mountpoints. All devices except the
built-in Jotta device are treated as backup devices by official Jottacloud
clients, and the mountpoints on them are individual backup sets.

With the built-in Jotta device, only existing, built-in, mountpoints can be
selected. In addition to the mentioned Archive and Sync, it may contain
several other mountpoints such as: Latest, Links, Shared and Trash. All of
these are special mountpoints with a different internal representation than
the "regular" mountpoints. Rclone will only to a very limited degree support
them. Generally you should avoid these, unless you know what you are doing.

### --fast-list

This backend supports `--fast-list` which allows you to use fewer
transactions in exchange for more memory. See the [rclone
docs](/docs/#fast-list) for more details.

Note that the implementation in Jottacloud always uses only a single
API request to get the entire list, so for large folders this could
lead to long wait time before the first results are shown.

Note also that with rclone version 1.58 and newer, information about
[MIME types](/overview/#mime-type) and metadata item [utime](#metadata)
are not available when using `--fast-list`.

### Modification times and hashes

Jottacloud allows modification times to be set on objects accurate to 1
second. These will be used to detect whether objects need syncing or
not.

Jottacloud supports MD5 type hashes, so you can use the `--checksum`
flag.

Note that Jottacloud requires the MD5 hash before upload so if the
source does not have an MD5 checksum then the file will be cached
temporarily on disk (in location given by
[--temp-dir](/docs/#temp-dir-string)) before it is uploaded.
Small files will be cached in memory - see the
[--jottacloud-md5-memory-limit](#jottacloud-md5-memory-limit) flag.
When uploading from local disk the source checksum is always available,
so this does not apply. Starting with rclone version 1.52 the same is
true for encrypted remotes (in older versions the crypt backend would not
calculate hashes for uploads from local disk, so the Jottacloud
backend had to do it as described above).

### Restricted filename characters

In addition to the [default restricted characters set](/overview/#restricted-characters)
the following characters are also replaced:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| "         | 0x22  | ＂          |
| *         | 0x2A  | ＊          |
| :         | 0x3A  | ：          |
| <         | 0x3C  | ＜          |
| >         | 0x3E  | ＞          |
| ?         | 0x3F  | ？          |
| \|        | 0x7C  | ｜          |

Invalid UTF-8 bytes will also be [replaced](/overview/#invalid-utf8),
as they can't be used in XML strings.

### Deleting files

By default, rclone will send all files to the trash when deleting files. They
will be permanently deleted automatically after 30 days. You may bypass the
trash and permanently delete files immediately by using the [--jottacloud-hard-delete](#jottacloud-hard-delete)
flag, or set the equivalent environment variable. Emptying the trash is
supported by the [cleanup](/commands/rclone_cleanup/) command.

### Versions

Jottacloud supports file versioning. When rclone uploads a new version of a
file it creates a new version of it. Currently rclone only supports retrieving
the current version but older versions can be accessed via the Jottacloud Website.

Versioning can be disabled by `--jottacloud-no-versions` option. This is
achieved by deleting the remote file prior to uploading a new version. If the
upload the fails no version of the file will be available in the remote.

### Quota information

To view your current quota you can use the `rclone about remote:`
command which will display your usage limit (unless it is unlimited)
and the current usage.

{{< rem autogenerated options start" - DO NOT EDIT - instead edit fs.RegInfo in backend/jottacloud/jottacloud.go then run make backenddocs" >}}
### Standard options

Here are the Standard options specific to jottacloud (Jottacloud).

#### --jottacloud-client-id

OAuth Client Id.

Leave blank normally.

Properties:

- Config:      client_id
- Env Var:     RCLONE_JOTTACLOUD_CLIENT_ID
- Type:        string
- Required:    false

#### --jottacloud-client-secret

OAuth Client Secret.

Leave blank normally.

Properties:

- Config:      client_secret
- Env Var:     RCLONE_JOTTACLOUD_CLIENT_SECRET
- Type:        string
- Required:    false

### Advanced options

Here are the Advanced options specific to jottacloud (Jottacloud).

#### --jottacloud-token

OAuth Access Token as a JSON blob.

Properties:

- Config:      token
- Env Var:     RCLONE_JOTTACLOUD_TOKEN
- Type:        string
- Required:    false

#### --jottacloud-auth-url

Auth server URL.

Leave blank to use the provider defaults.

Properties:

- Config:      auth_url
- Env Var:     RCLONE_JOTTACLOUD_AUTH_URL
- Type:        string
- Required:    false

#### --jottacloud-token-url

Token server url.

Leave blank to use the provider defaults.

Properties:

- Config:      token_url
- Env Var:     RCLONE_JOTTACLOUD_TOKEN_URL
- Type:        string
- Required:    false

#### --jottacloud-client-credentials

Use client credentials OAuth flow.

This will use the OAUTH2 client Credentials Flow as described in RFC 6749.

Note that this option is NOT supported by all backends.

Properties:

- Config:      client_credentials
- Env Var:     RCLONE_JOTTACLOUD_CLIENT_CREDENTIALS
- Type:        bool
- Default:     false

#### --jottacloud-md5-memory-limit

Files bigger than this will be cached on disk to calculate the MD5 if required.

Properties:

- Config:      md5_memory_limit
- Env Var:     RCLONE_JOTTACLOUD_MD5_MEMORY_LIMIT
- Type:        SizeSuffix
- Default:     10Mi

#### --jottacloud-trashed-only

Only show files that are in the trash.

This will show trashed files in their original directory structure.

Properties:

- Config:      trashed_only
- Env Var:     RCLONE_JOTTACLOUD_TRASHED_ONLY
- Type:        bool
- Default:     false

#### --jottacloud-hard-delete

Delete files permanently rather than putting them into the trash.

Properties:

- Config:      hard_delete
- Env Var:     RCLONE_JOTTACLOUD_HARD_DELETE
- Type:        bool
- Default:     false

#### --jottacloud-upload-resume-limit

Files bigger than this can be resumed if the upload fail's.

Properties:

- Config:      upload_resume_limit
- Env Var:     RCLONE_JOTTACLOUD_UPLOAD_RESUME_LIMIT
- Type:        SizeSuffix
- Default:     10Mi

#### --jottacloud-no-versions

Avoid server side versioning by deleting files and recreating files instead of overwriting them.

Properties:

- Config:      no_versions
- Env Var:     RCLONE_JOTTACLOUD_NO_VERSIONS
- Type:        bool
- Default:     false

#### --jottacloud-encoding

The encoding for the backend.

See the [encoding section in the overview](/overview/#encoding) for more info.

Properties:

- Config:      encoding
- Env Var:     RCLONE_JOTTACLOUD_ENCODING
- Type:        Encoding
- Default:     Slash,LtGt,DoubleQuote,Colon,Question,Asterisk,Pipe,Del,Ctl,InvalidUtf8,Dot

#### --jottacloud-description

Description of the remote.

Properties:

- Config:      description
- Env Var:     RCLONE_JOTTACLOUD_DESCRIPTION
- Type:        string
- Required:    false

### Metadata

Jottacloud has limited support for metadata, currently an extended set of timestamps.

Here are the possible system metadata items for the jottacloud backend.

| Name | Help | Type | Example | Read Only |
|------|------|------|---------|-----------|
| btime | Time of file birth (creation), read from rclone metadata | RFC 3339 | 2006-01-02T15:04:05.999999999Z07:00 | N |
| content-type | MIME type, also known as media type | string | text/plain | **Y** |
| mtime | Time of last modification, read from rclone metadata | RFC 3339 | 2006-01-02T15:04:05.999999999Z07:00 | N |
| utime | Time of last upload, when current revision was created, generated by backend | RFC 3339 | 2006-01-02T15:04:05.999999999Z07:00 | **Y** |

See the [metadata](/docs/#metadata) docs for more info.

{{< rem autogenerated options stop >}}

## Limitations

Note that Jottacloud is case insensitive so you can't have a file called
"Hello.doc" and one called "hello.doc".

There are quite a few characters that can't be in Jottacloud file names. Rclone will map these names to and from an identical
looking unicode equivalent. For example if a file has a ? in it will be mapped to ？ instead.

Jottacloud only supports filenames up to 255 characters in length.

## Troubleshooting

Jottacloud exhibits some inconsistent behaviours regarding deleted files and folders which may cause Copy, Move and DirMove
operations to previously deleted paths to fail. Emptying the trash should help in such cases.
