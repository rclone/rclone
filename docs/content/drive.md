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
[snip]
10 / Google Drive
   \ "drive"
[snip]
Storage> drive
Google Application Client Id - leave blank normally.
client_id>
Google Application Client Secret - leave blank normally.
client_secret>
Scope that rclone should use when requesting access from drive.
Choose a number from below, or type in your own value
 1 / Full access all files, excluding Application Data Folder.
   \ "drive"
 2 / Read-only access to file metadata and file contents.
   \ "drive.readonly"
   / Access to files created by rclone only.
 3 | These are visible in the drive website.
   | File authorization is revoked when the user deauthorizes the app.
   \ "drive.file"
   / Allows read and write access to the Application Data folder.
 4 | This is not visible in the drive website.
   \ "drive.appfolder"
   / Allows read-only access to file metadata but
 5 | does not allow any access to read or download file content.
   \ "drive.metadata.readonly"
scope> 1
ID of the root folder - leave blank normally.  Fill in to access "Computers" folders. (see docs).
root_folder_id> 
Service Account Credentials JSON file path - needed only if you want use SA instead of interactive login.
service_account_file>
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
scope = drive
root_folder_id = 
service_account_file =
token = {"access_token":"XXX","token_type":"Bearer","refresh_token":"XXX","expiry":"2014-03-16T13:57:58.955387075Z"}
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

### Scopes ###

Rclone allows you to select which scope you would like for rclone to
use.  This changes what type of token is granted to rclone.  [The
scopes are defined
here.](https://developers.google.com/drive/v3/web/about-auth).

The scope are

#### drive ####

This is the default scope and allows full access to all files, except
for the Application Data Folder (see below).

Choose this one if you aren't sure.

#### drive.readonly ####

This allows read only access to all files.  Files may be listed and
downloaded but not uploaded, renamed or deleted.

#### drive.file ####

With this scope rclone can read/view/modify only those files and
folders it creates.

So if you uploaded files to drive via the web interface (or any other
means) they will not be visible to rclone.

This can be useful if you are using rclone to backup data and you want
to be sure confidential data on your drive is not visible to rclone.

Files created with this scope are visible in the web interface.

#### drive.appfolder ####

This gives rclone its own private area to store files.  Rclone will
not be able to see any other files on your drive and you won't be able
to see rclone's files from the web interface either.

#### drive.metadata.readonly ####

This allows read only access to file names only.  It does not allow
rclone to download or upload data, or rename or delete files or
directories.

### Root folder ID ###

You can set the `root_folder_id` for rclone.  This is the directory
(identified by its `Folder ID`) that rclone considers to be a the root
of your drive.

Normally you will leave this blank and rclone will determine the
correct root to use itself.

However you can set this to restrict rclone to a specific folder
hierarchy or to access data within the "Computers" tab on the drive
web interface (where files from Google's Backup and Sync desktop
program go).

In order to do this you will have to find the `Folder ID` of the
directory you wish rclone to display.  This will be the last segment
of the URL when you open the relevant folder in the drive web
interface.

So if the folder you want rclone to use has a URL which looks like
`https://drive.google.com/drive/folders/1XyfxxxxxxxxxxxxxxxxxxxxxxxxxKHCh`
in the browser, then you use `1XyfxxxxxxxxxxxxxxxxxxxxxxxxxKHCh` as
the `root_folder_id` in the config.

**NB** folders under the "Computers" tab seem to be read only (drive
gives a 500 error) when using rclone.

There doesn't appear to be an API to discover the folder IDs of the
"Computers" tab - please contact us if you know otherwise!

Note also that rclone can't access any data under the "Backups" tab on
the google drive web interface yet.

### Service Account support ###

You can set up rclone with Google Drive in an unattended mode,
i.e. not tied to a specific end-user Google account. This is useful
when you want to synchronise files onto machines that don't have
actively logged-in users, for example build machines.

To use a Service Account instead of OAuth2 token flow, enter the path
to your Service Account credentials at the `service_account_file`
prompt during `rclone config` and rclone won't use the browser based
authentication flow.

#### Use case - Google Apps/G-suite account and individual Drive ####

Let's say that you are the administrator of a Google Apps (old) or
G-suite account.
The goal is to store data on an individual's Drive account, who IS
a member of the domain.
We'll call the domain **example.com**, and the user
**foo@example.com**.

There's a few steps we need to go through to accomplish this:

##### 1. Create a service account for example.com #####
  - To create a service account and obtain its credentials, go to the
[Google Developer Console](https://console.developers.google.com).
  - You must have a project - create one if you don't.
  - Then go to "IAM & admin" -> "Service Accounts".
  - Use the "Create Credentials" button. Fill in "Service account name"
with something that identifies your client. "Role" can be empty.
  - Tick "Furnish a new private key" - select "Key type JSON".
  - Tick "Enable G Suite Domain-wide Delegation". This option makes
"impersonation" possible, as documented here:
[Delegating domain-wide authority to the service account](https://developers.google.com/identity/protocols/OAuth2ServiceAccount#delegatingauthority)
  - These credentials are what rclone will use for authentication.
If you ever need to remove access, press the "Delete service
account key" button.

##### 2. Allowing API access to example.com Google Drive #####
  - Go to example.com's admin console
  - Go into "Security" (or use the search bar)
  - Select "Show more" and then "Advanced settings"
  - Select "Manage API client access" in the "Authentication" section
  - In the "Client Name" field enter the service account's
"Client ID" - this can be found in the Developer Console under
"IAM & Admin" -> "Service Accounts", then "View Client ID" for
the newly created service account.
It is a ~21 character numerical string.
  - In the next field, "One or More API Scopes", enter
`https://www.googleapis.com/auth/drive`
to grant access to Google Drive specifically.

##### 3. Configure rclone, assuming a new install #####

```
rclone config

n/s/q> n         # New
name>gdrive      # Gdrive is an example name
Storage>         # Select the number shown for Google Drive
client_id>       # Can be left blank
client_secret>   # Can be left blank
scope>           # Select your scope, 1 for example
root_folder_id>  # Can be left blank
service_account_file> /home/foo/myJSONfile.json # This is where the JSON file goes!
y/n>             # Auto config, y

```

##### 4. Verify that it's working #####
  - `rclone -v --drive-impersonate foo@example.com lsf gdrive:backup`
  - The arguments do:
    - `-v` - verbose logging
    - `--drive-impersonate foo@example.com` - this is what does
the magic, pretending to be user foo.
    - `lsf` - list files in a parsing friendly way
    - `gdrive:backup` - use the remote called gdrive, work in
the folder named backup.

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

By default rclone will send all files to the trash when deleting
files.  If deleting them permanently is required then use the
`--drive-use-trash=false` flag, or set the equivalent environment
variable.

### Emptying trash ###

If you wish to empty your trash you can use the `rclone cleanup remote:`
command which will permanently delete all your trashed files. This command
does not take any path arguments.

### Quota information ###

To view your current quota you can use the `rclone about remote:`
command which will display your usage limit (quota), the usage in Google
Drive, the size of all files in the Trash and the space used by other
Google services such as Gmail. This command does not take any path
arguments.

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

#### --drive-impersonate user ####

When using a service account, this instructs rclone to impersonate the user passed in.

#### --drive-list-chunk int ####

Size of listing chunk 100-1000. 0 to disable. (default 1000)

#### --drive-shared-with-me ####

Instructs rclone to operate on your "Shared with me" folder (where
Google Drive lets you access the files and folders others have shared
with you).

This works both with the "list" (lsd, lsl, etc) and the "copy"
commands (copy, sync, etc), and with all other commands too.

#### --drive-skip-gdocs ####

Skip google documents in all listings. If given, gdocs practically become invisible to rclone.

#### --drive-trashed-only ####

Only show files that are in the trash.  This will show trashed files
in their original directory structure.

#### --drive-upload-cutoff=SIZE ####

File size cutoff for switching to chunked upload.  Default is 8 MB.

#### --drive-use-trash ####

Controls whether files are sent to the trash or deleted
permanently. Defaults to true, namely sending files to the trash.  Use
`--drive-use-trash=false` to delete files permanently instead.

#### --drive-use-created-date ####

Use the file creation date in place of the modification date. Defaults
to false.

Useful when downloading data and you want the creation date used in
place of the last modified date.

**WARNING**: This flag may have some unexpected consequences.

When uploading to your drive all files will be overwritten unless they
haven't been modified since their creation. And the inverse will occur
while downloading.  This side effect can be avoided by using the
`--checksum` flag.

This feature was implemented to retain photos capture date as recorded
by google photos. You will first need to check the "Create a Google
Photos folder" option in your google drive settings. You can then copy
or move the photos locally and use the date the image was taken
(created) set as the modification date.

### Limitations ###

Drive has quite a lot of rate limiting.  This causes rclone to be
limited to transferring about 2 files per second only.  Individual
files may be transferred much faster at 100s of MBytes/s but lots of
small files can take a long time.

Server side copies are also subject to a separate rate limit. If you
see User rate limit exceeded errors, wait at least 24 hours and retry.
You can disable server side copies with `--disable copy` to download
and upload the files if you prefer.

#### Limitations of Google Docs ####

Google docs will appear as size -1 in `rclone ls` and as size 0 in
anything which uses the VFS layer, eg `rclone mount`, `rclone serve`.

This is because rclone can't find out the size of the Google docs
without downloading them.

Google docs will transfer correctly with `rclone sync`, `rclone copy`
etc as rclone knows to ignore the size when doing the transfer.

However an unfortunate consequence of this is that you can't download
Google docs using `rclone mount` - you will get a 0 sized file.  If
you try again the doc may gain its correct size and be downloadable.

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

The most likely cause of this is the duplicated file issue above - run
`rclone dedupe` and check your logs for duplicate object or directory
messages.

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

3. Under "ENABLE APIS AND SERVICES" search for "Drive", and enable the
then "Google Drive API".

4. Click "Credentials" in the left-side panel (not "Create
credentials", which opens the wizard), then "Create credentials", then
"OAuth client ID".  It will prompt you to set the OAuth consent screen
product name, if you haven't set one already.

5. Choose an application type of "other", and click "Create". (the
default name is fine)

6. It will show you a client ID and client secret.  Use these values
in rclone config to add a new remote or edit an existing remote.

(Thanks to @balazer on github for these instructions.)
