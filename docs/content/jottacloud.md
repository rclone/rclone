---
title: "Jottacloud"
description: "Rclone docs for Jottacloud"
versionIntroduced: "v1.43"
---

# {{< icon "fa fa-cloud" >}} Jottacloud

Jottacloud is a cloud storage service provider from a Norwegian company, using its own datacenters
in Norway. In addition to the official service at [jottacloud.com](https://www.jottacloud.com/),
it also provides white-label solutions to different companies, such as:
* Telia
  * Telia Cloud (cloud.telia.se)
  * Telia Sky (sky.telia.no)
* Tele2
  * Tele2 Cloud (mittcloud.tele2.se)
* Onlime
  * Onlime Cloud Storage (onlime.dk)
* Elkjøp (with subsidiaries):
  * Elkjøp Cloud (cloud.elkjop.no)
  * Elgiganten Sweden (cloud.elgiganten.se)
  * Elgiganten Denmark (cloud.elgiganten.dk)
  * Giganti Cloud  (cloud.gigantti.fi)
  * ELKO Cloud (cloud.elko.is)

Most of the white-label versions are supported by this backend, although may require different
authentication setup - described below.

Paths are specified as `remote:path`

Paths may be as deep as required, e.g. `remote:directory/subdirectory`.

## Authentication types

Some of the whitelabel versions uses a different authentication method than the official service,
and you have to choose the correct one when setting up the remote.

### Standard authentication

The standard authentication method used by the official service (jottacloud.com), as well as
some of the whitelabel services, requires you to generate a single-use personal login token
from the account security settings in the service's web interface. Log in to your account,
go to "Settings" and then "Security", or use the direct link presented to you by rclone when
configuring the remote: <https://www.jottacloud.com/web/secure>. Scroll down to the section
"Personal login token", and click the "Generate" button. Note that if you are using a
whitelabel service you probably can't use the direct link, you need to find the same page in
their dedicated web interface, and also it may be in a different location than described above.

To access your account from multiple instances of rclone, you need to configure each of them
with a separate personal login token. E.g. you create a Jottacloud remote with rclone in one
location, and copy the configuration file to a second location where you also want to run
rclone and access the same remote. Then you need to replace the token for one of them, using
the [config reconnect](https://rclone.org/commands/rclone_config_reconnect/) command, which
requires you to generate a new personal login token and supply as input. If you do not
do this, the token may easily end up being invalidated, resulting in both instances failing
with an error message something along the lines of:

    oauth2: cannot fetch token: 400 Bad Request
    Response: {"error":"invalid_grant","error_description":"Stale token"}

When this happens, you need to replace the token as described above to be able to use your
remote again.

All personal login tokens you have taken into use will be listed in the web interface under
"My logged in devices", and from the right side of that list you can click the "X" button to
revoke individual tokens.

### Legacy authentication

If you are using one of the whitelabel versions (e.g. from Elkjøp) you may not have the option
to generate a CLI token. In this case you'll have to use the legacy authentication. To do this select
yes when the setup asks for legacy authentication and enter your username and password.
The rest of the setup is identical to the default setup.

### Telia Cloud authentication

Similar to other whitelabel versions Telia Cloud doesn't offer the option of creating a CLI token, and
additionally uses a separate authentication flow where the username is generated internally. To setup
rclone to use Telia Cloud, choose Telia Cloud authentication in the setup. The rest of the setup is
identical to the default setup.

### Tele2 Cloud authentication

As Tele2-Com Hem merger was completed this authentication can be used for former Com Hem Cloud and
Tele2 Cloud customers as no support for creating a CLI token exists, and additionally uses a separate
authentication flow where the username is generated internally. To setup rclone to use Tele2 Cloud,
choose Tele2 Cloud authentication in the setup. The rest of the setup is identical to the default setup.

### Onlime Cloud Storage authentication

Onlime has sold access to Jottacloud proper, while providing localized support to Danish Customers, but
have recently set up their own hosting, transferring their customers from Jottacloud servers to their
own ones.

This, of course, necessitates using their servers for authentication, but otherwise functionality and
architecture seems equivalent to Jottacloud.

To setup rclone to use Onlime Cloud Storage, choose Onlime Cloud authentication in the setup. The rest
of the setup is identical to the default setup.

## Configuration

Here is an example of how to make a remote called `remote` with the default setup.  First run:

    rclone config

This will guide you through an interactive setup process:

```
No remotes found, make a new one?
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Option Storage.
Type of storage to configure.
Choose a number from below, or type in your own value.
[snip]
XX / Jottacloud
   \ (jottacloud)
[snip]
Storage> jottacloud
Edit advanced config?
y) Yes
n) No (default)
y/n> n
Option config_type.
Select authentication type.
Choose a number from below, or type in an existing string value.
Press Enter for the default (standard).
   / Standard authentication.
 1 | Use this if you're a normal Jottacloud user.
   \ (standard)
   / Legacy authentication.
 2 | This is only required for certain whitelabel versions of Jottacloud and not recommended for normal users.
   \ (legacy)
   / Telia Cloud authentication.
 3 | Use this if you are using Telia Cloud.
   \ (telia)
   / Tele2 Cloud authentication.
 4 | Use this if you are using Tele2 Cloud.
   \ (tele2)
   / Onlime Cloud authentication.
 5 | Use this if you are using Onlime Cloud.
   \ (onlime)
config_type> 1
Personal login token.
Generate here: https://www.jottacloud.com/web/secure
Login Token> <your token here>
Use a non-standard device/mountpoint?
Choosing no, the default, will let you access the storage used for the archive
section of the official Jottacloud client. If you instead want to access the
sync or the backup section, for example, you must choose yes.
y) Yes
n) No (default)
y/n> y
Option config_device.
The device to use. In standard setup the built-in Jotta device is used,
which contains predefined mountpoints for archive, sync etc. All other devices
are treated as backup devices by the official Jottacloud client. You may create
a new by entering a unique name.
Choose a number from below, or type in your own string value.
Press Enter for the default (DESKTOP-3H31129).
 1 > DESKTOP-3H31129
 2 > Jotta
config_device> 2
Option config_mountpoint.
The mountpoint to use for the built-in device Jotta.
The standard setup is to use the Archive mountpoint. Most other mountpoints
have very limited support in rclone and should generally be avoided.
Choose a number from below, or type in an existing string value.
Press Enter for the default (Archive).
 1 > Archive
 2 > Shared
 3 > Sync
config_mountpoint> 1
--------------------
[remote]
type = jottacloud
configVersion = 1
client_id = jottacli
client_secret =
tokenURL = https://id.jottacloud.com/auth/realms/jottacloud/protocol/openid-connect/token
token = {........}
username = 2940e57271a93d987d6f8a21
device = Jotta
mountpoint = Archive
--------------------
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d> y
```

Once configured you can then use `rclone` like this,

List directories in top level of your Jottacloud

    rclone lsd remote:

List all the files in your Jottacloud

    rclone ls remote:

To copy a local directory to an Jottacloud directory called backup

    rclone copy /home/source remote:backup

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
[--temp-dir](/docs/#temp-dir-dir)) before it is uploaded.
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

By default, rclone will send all files to the trash when deleting files. They will be permanently
deleted automatically after 30 days. You may bypass the trash and permanently delete files immediately
by using the [--jottacloud-hard-delete](#jottacloud-hard-delete) flag, or set the equivalent environment variable.
Emptying the trash is supported by the [cleanup](/commands/rclone_cleanup/) command.

### Versions

Jottacloud supports file versioning. When rclone uploads a new version of a file it creates a new version of it.
Currently rclone only supports retrieving the current version but older versions can be accessed via the Jottacloud Website.

Versioning can be disabled by `--jottacloud-no-versions` option. This is achieved by deleting the remote file prior to uploading
a new version. If the upload the fails no version of the file will be available in the remote.

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
