---
title: "Google drive"
description: "Rclone docs for Google drive"
date: "2016-04-12"
---

<i class="fa fa-google"></i> Google Drive
-----------------------------------------

Paths are specified as `drive:path`

Drive paths may be as deep as required, eg `drive:directory/subdirectory`.

The initial setup for drive involves getting a token from Google drive
which you need to do in your browser.  `rclone config` walks you
through it.

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found - make a new one
n) New remote
r) Rename remote
c) Copy remote
s) Set configuration password
q) Quit config
n/r/c/s/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
 1 / Amazon Drive
   \ "amazon cloud drive"
 2 / Amazon S3 (also Dreamhost, Ceph, Minio)
   \ "s3"
 3 / Backblaze B2
   \ "b2"
 4 / Dropbox
   \ "dropbox"
 5 / Encrypt/Decrypt a remote
   \ "crypt"
 6 / FTP Connection
   \ "ftp"
 7 / Google Cloud Storage (this is not Google Drive)
   \ "google cloud storage"
 8 / Google Drive
   \ "drive"
 9 / Hubic
   \ "hubic"
10 / Local Disk
   \ "local"
11 / Microsoft OneDrive
   \ "onedrive"
12 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
   \ "swift"
13 / SSH/SFTP Connection
   \ "sftp"
14 / Yandex Disk
   \ "yandex"
Storage> 8
Google Application Client Id - leave blank normally.
client_id> 
Google Application Client Secret - leave blank normally.
client_secret> 
Remote config
Use auto config?
 * Say Y if not sure
 * Say N if you are working on a remote or headless machine or Y didn't work
y) Yes
n) No
y/n> y
If your browser doesn't open automatically go to the following link: http://127.0.0.1:53682/auth
Log in and authorize rclone for access
Waiting for code...
Got code
Configure this as a team drive?
y) Yes
n) No
y/n> n
--------------------
[remote]
client_id = 
client_secret = 
token = {"AccessToken":"xxxx.x.xxxxx_xxxxxxxxxxx_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx","RefreshToken":"1/xxxxxxxxxxxxxxxx_xxxxxxxxxxxxxxxxxxxxxxxxxx","Expiry":"2014-03-16T13:57:58.955387075Z","Extra":null}
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

Note that rclone runs a webserver on your local machine to collect the
token as returned from Google if you use auto config mode. This only
runs from the moment it opens your browser to the moment you get back
the verification code.  This is on `http://127.0.0.1:53682/` and this
it may require you to unblock it temporarily if you are running a host
firewall, or use manual mode.

You can then use it like this,

List directories in top level of your drive

    rclone lsd remote:

List all the files in your drive

    rclone ls remote:

To copy a local directory to a drive directory called backup

    rclone copy /home/source remote:backup

### Team drives ###

If you want to configure the remote to point to a Google Team Drive
then answer `y` to the question `Configure this as a team drive?`.

This will fetch the list of Team Drives from google and allow you to
configure which one you want to use.  You can also type in a team
drive ID if you prefer.

For example:

```
Configure this as a team drive?
y) Yes
n) No
y/n> y
Fetching team drive list...
Choose a number from below, or type in your own value
 1 / Rclone Test
   \ "xxxxxxxxxxxxxxxxxxxx"
 2 / Rclone Test 2
   \ "yyyyyyyyyyyyyyyyyyyy"
 3 / Rclone Test 3
   \ "zzzzzzzzzzzzzzzzzzzz"
Enter a Team Drive ID> 1
--------------------
[remote]
client_id = 
client_secret = 
token = {"AccessToken":"xxxx.x.xxxxx_xxxxxxxxxxx_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx","RefreshToken":"1/xxxxxxxxxxxxxxxx_xxxxxxxxxxxxxxxxxxxxxxxxxx","Expiry":"2014-03-16T13:57:58.955387075Z","Extra":null}
team_drive = xxxxxxxxxxxxxxxxxxxx
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

### Modified time ###

Google drive stores modification times accurate to 1 ms.

### Revisions ###

Google drive stores revisions of files.  When you upload a change to
an existing file to google drive using rclone it will create a new
revision of that file.

Revisions follow the standard google policy which at time of writing
was

  * They are deleted after 30 days or 100 revisions (whatever comes first).
  * They do not count towards a user storage quota.

### Deleting files ###

By default rclone will delete files permanently when requested.  If
sending them to the trash is required instead then use the
`--drive-use-trash` flag.

### Specific options ###

Here are the command line options specific to this cloud storage
system.

#### --drive-auth-owner-only ####

Only consider files owned by the authenticated user.

#### --drive-chunk-size=SIZE ####

Upload chunk size. Must a power of 2 >= 256k. Default value is 8 MB.

Making this larger will improve performance, but note that each chunk
is buffered in memory one per transfer.

Reducing this will reduce memory usage but decrease performance.

#### --drive-auth-owner-only ####

Only consider files owned by the authenticated user.

#### --drive-formats ####

Google documents can only be exported from Google drive.  When rclone
downloads a Google doc it chooses a format to download depending upon
this setting.

By default the formats are `docx,xlsx,pptx,svg` which are a sensible
default for an editable document.

When choosing a format, rclone runs down the list provided in order
and chooses the first file format the doc can be exported as from the
list. If the file can't be exported to a format on the formats list,
then rclone will choose a format from the default list.

If you prefer an archive copy then you might use `--drive-formats
pdf`, or if you prefer openoffice/libreoffice formats you might use
`--drive-formats ods,odt,odp`.

Note that rclone adds the extension to the google doc, so if it is
calles `My Spreadsheet` on google docs, it will be exported as `My
Spreadsheet.xlsx` or `My Spreadsheet.pdf` etc.

Here are the possible extensions with their corresponding mime types.

| Extension | Mime Type | Description |
| --------- |-----------| ------------|
| csv  | text/csv | Standard CSV format for Spreadsheets |
| doc  | application/msword | Micosoft Office Document |
| docx | application/vnd.openxmlformats-officedocument.wordprocessingml.document | Microsoft Office Document |
| epub | application/epub+zip | E-book format |
| html | text/html | An HTML Document |
| jpg  | image/jpeg | A JPEG Image File |
| odp  | application/vnd.oasis.opendocument.presentation | Openoffice Presentation |
| ods  | application/vnd.oasis.opendocument.spreadsheet | Openoffice Spreadsheet |
| ods  | application/x-vnd.oasis.opendocument.spreadsheet | Openoffice Spreadsheet |
| odt  | application/vnd.oasis.opendocument.text | Openoffice Document |
| pdf  | application/pdf | Adobe PDF Format |
| png  | image/png | PNG Image Format|
| pptx | application/vnd.openxmlformats-officedocument.presentationml.presentation | Microsoft Office Powerpoint |
| rtf  | application/rtf | Rich Text Format |
| svg  | image/svg+xml | Scalable Vector Graphics Format |
| tsv  | text/tab-separated-values | Standard TSV format for spreadsheets |
| txt  | text/plain | Plain Text |
| xls  | application/vnd.ms-excel | Microsoft Office Spreadsheet |
| xlsx | application/vnd.openxmlformats-officedocument.spreadsheetml.sheet | Microsoft Office Spreadsheet |
| zip  | application/zip | A ZIP file of HTML, Images CSS |

#### --drive-list-chunk int ####

Size of listing chunk 100-1000. 0 to disable. (default 1000)

#### --drive-shared-with-me ####

Only show files that are shared with me

#### --drive-skip-gdocs ####

Skip google documents in all listings. If given, gdocs practically become invisible to rclone.

#### --drive-trashed-only ####

Only show files that are in the trash.  This will show trashed files
in their original directory structure.

#### --drive-upload-cutoff=SIZE ####

File size cutoff for switching to chunked upload.  Default is 8 MB.

#### --drive-use-trash ####

Send files to the trash instead of deleting permanently. Defaults to
off, namely deleting files permanently.

### Limitations ###

Drive has quite a lot of rate limiting.  This causes rclone to be
limited to transferring about 2 files per second only.  Individual
files may be transferred much faster at 100s of MBytes/s but lots of
small files can take a long time.

Server side copies are also subject to a separate rate limit. If you
see User rate limit exceeded errors, wait at least 24 hours and retry.
You can disable server side copies with `--disable copy` to download
and upload the files if you prefer.

### Duplicated files ###

Sometimes, for no reason I've been able to track down, drive will
duplicate a file that rclone uploads.  Drive unlike all the other
remotes can have duplicated files.

Duplicated files cause problems with the syncing and you will see
messages in the log about duplicates.

Use `rclone dedupe` to fix duplicated files.

Note that this isn't just a problem with rclone, even Google Photos on
Android duplicates files on drive sometimes.

### Rclone appears to be re-copying files it shouldn't ###

There are two possible reasons for rclone to recopy files which
haven't changed to Google Drive.

The first is the duplicated file issue above - run `rclone dedupe` and
check your logs for duplicate object or directory messages.

The second is that sometimes Google reports different sizes for the
Google Docs exports which will cause rclone to re-download Google Docs
for no apparent reason.  `--ignore-size` is a not very satisfactory
work-around for this if it is causing you a lot of problems.

### Google docs downloads sometimes fail with "Failed to copy: read X bytes expecting Y" ###

This is the same problem as above.  Google reports the google doc is
one size, but rclone downloads a different size.  Work-around with the
`--ignore-size` flag or wait for rclone to retry the download which it
will.

### Making your own client_id ###

When you use rclone with Google drive in its default configuration you
are using rclone's client_id.  This is shared between all the rclone
users.  There is a global rate limit on the number of queries per
second that each client_id can do set by Google.  rclone already has a
high quota and I will continue to make sure it is high enough by
contacting Google.

However you might find you get better performance making your own
client_id if you are a heavy user. Or you may not depending on exactly
how Google have been raising rclone's rate limit.

Here is how to create your own Google Drive client ID for rclone:

1. Log into the [Google API
Console](https://console.developers.google.com/) with your Google
account. It doesn't matter what Google account you use. (It need not
be the same account as the Google Drive you want to access)

2. Select a project or create a new project.

3. Under Overview, Google APIs, Google Apps APIs, click "Drive API",
then "Enable".

4. Click "Credentials" in the left-side panel (not "Go to
credentials", which opens the wizard), then "Create credentials", then
"OAuth client ID".  It will prompt you to set the OAuth consent screen
product name, if you haven't set one already.

5. Choose an application type of "other", and click "Create". (the
default name is fine)

6. It will show you a client ID and client secret.  Use these values
in rclone config to add a new remote or edit an existing remote.

(Thanks to @balazer on github for these instructions.)
