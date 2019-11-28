---
title: "Jottacloud"
description: "Rclone docs for Jottacloud"
date: "2018-08-07"
---

<i class="fa fa-cloud"></i> Jottacloud
-----------------------------------------

Paths are specified as `remote:path`

Paths may be as deep as required, eg `remote:directory/subdirectory`.

To configure Jottacloud you will need to generate a personal security token in the Jottacloud web inteface. You will the option to do in your [account security settings](https://www.jottacloud.com/web/secure). Note that the web inteface may refer to this token as a JottaCli token.

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> jotta
Type of storage to configure.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
[snip]
XX / JottaCloud
   \ "jottacloud"
[snip]
Storage> jottacloud
** See help for jottacloud backend at: https://rclone.org/jottacloud/ **

Edit advanced config? (y/n)
y) Yes
n) No
y/n> n
Remote config

Generate a personal login token here: https://www.jottacloud.com/web/secure
Login Token> <your token here>

Do you want to use a non standard device/mountpoint e.g. for accessing files uploaded using the official Jottacloud client?

y) Yes
n) No
y/n> y
Please select the device to use. Normally this will be Jotta
Choose a number from below, or type in an existing value
 1 > DESKTOP-3H31129
 2 > fla1
 3 > Jotta
Devices> 3
Please select the mountpoint to user. Normally this will be Archive
Choose a number from below, or type in an existing value
 1 > Archive
 2 > Shared
 3 > Sync
Mountpoints> 1
--------------------
[jotta]
type = jottacloud
user = 0xC4KE@gmail.com
token = {........}
device = Jotta
mountpoint = Archive
configVersion = 1
--------------------
y) Yes this is OK
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

### Devices and Mountpoints ###

The official Jottacloud client registers a device for each computer you install it on and then creates a mountpoint for each folder you select for Backup.
The web interface uses a special device called Jotta for the Archive, Sync and Shared mountpoints. In most cases you'll want to use the Jotta/Archive device/mounpoint however if you want to access files uploaded by any of the official clients rclone provides the option to select other devices and mountpoints during config.

### --fast-list ###

This remote supports `--fast-list` which allows you to use fewer
transactions in exchange for more memory. See the [rclone
docs](/docs/#fast-list) for more details.

Note that the implementation in Jottacloud always uses only a single
API request to get the entire list, so for large folders this could
lead to long wait time before the first results are shown.

### Modified time and hashes ###

Jottacloud allows modification times to be set on objects accurate to 1
second.  These will be used to detect whether objects need syncing or
not.

Jottacloud supports MD5 type hashes, so you can use the `--checksum`
flag.

Note that Jottacloud requires the MD5 hash before upload so if the
source does not have an MD5 checksum then the file will be cached
temporarily on disk (wherever the `TMPDIR` environment variable points
to) before it is uploaded.  Small files will be cached in memory - see
the `--jottacloud-md5-memory-limit` flag.

#### Restricted filename characters

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

### Deleting files ###

By default rclone will send all files to the trash when deleting files.
Due to a lack of API documentation emptying the trash is currently
only possible via the Jottacloud website. If deleting permanently
is required then use the `--jottacloud-hard-delete` flag,
or set the equivalent environment variable.

### Versions ###

Jottacloud supports file versioning. When rclone uploads a new version of a file it creates a new version of it. Currently rclone only supports retrieving the current version but older versions can be accessed via the Jottacloud Website.

### Quota information ###

To view your current quota you can use the `rclone about remote:`
command which will display your usage limit (unless it is unlimited)
and the current usage.

### Device IDs ###

Jottacloud requires each 'device' to be registered. Rclone brings such a registration to easily access your account but if you want to use Jottacloud together with rclone on multiple machines you NEED to create a seperate deviceID/deviceSecrect on each machine. You will asked during setting up the remote. Please be aware that this also means that copying the rclone config from one machine to another does NOT work with Jottacloud accounts. You have to create it on each machine.

<!--- autogenerated options start - DO NOT EDIT, instead edit fs.RegInfo in backend/jottacloud/jottacloud.go then run make backenddocs -->
### Advanced Options

Here are the advanced options specific to jottacloud (JottaCloud).

#### --jottacloud-md5-memory-limit

Files bigger than this will be cached on disk to calculate the MD5 if required.

- Config:      md5_memory_limit
- Env Var:     RCLONE_JOTTACLOUD_MD5_MEMORY_LIMIT
- Type:        SizeSuffix
- Default:     10M

#### --jottacloud-hard-delete

Delete files permanently rather than putting them into the trash.

- Config:      hard_delete
- Env Var:     RCLONE_JOTTACLOUD_HARD_DELETE
- Type:        bool
- Default:     false

#### --jottacloud-unlink

Remove existing public link to file/folder with link command rather than creating.
Default is false, meaning link command will create or retrieve public link.

- Config:      unlink
- Env Var:     RCLONE_JOTTACLOUD_UNLINK
- Type:        bool
- Default:     false

#### --jottacloud-upload-resume-limit

Files bigger than this can be resumed if the upload fail's.

- Config:      upload_resume_limit
- Env Var:     RCLONE_JOTTACLOUD_UPLOAD_RESUME_LIMIT
- Type:        SizeSuffix
- Default:     10M

<!--- autogenerated options stop -->

### Limitations ###

Note that Jottacloud is case insensitive so you can't have a file called
"Hello.doc" and one called "hello.doc".

There are quite a few characters that can't be in Jottacloud file names. Rclone will map these names to and from an identical looking unicode equivalent. For example if a file has a ? in it will be mapped to ？ instead.

Jottacloud only supports filenames up to 255 characters in length.

### Troubleshooting ###

Jottacloud exhibits some inconsistent behaviours regarding deleted files and folders which may cause Copy, Move and DirMove operations to previously deleted paths to fail. Emptying the trash should help in such cases.
