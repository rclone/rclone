---
title: "Microsoft OneDrive"
description: "Rclone docs for Microsoft OneDrive"
---

{{< icon "fab fa-windows" >}} Microsoft OneDrive
-----------------------------------------

Paths are specified as `remote:path`

Paths may be as deep as required, eg `remote:directory/subdirectory`.

The initial setup for OneDrive involves getting a token from
Microsoft which you need to do in your browser.  `rclone config` walks
you through it.

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
e) Edit existing remote
n) New remote
d) Delete remote
r) Rename remote
c) Copy remote
s) Set configuration password
q) Quit config
e/n/d/r/c/s/q> n
name> remote
Type of storage to configure.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
[snip]
XX / Microsoft OneDrive
   \ "onedrive"
[snip]
Storage> onedrive
Microsoft App Client Id
Leave blank normally.
Enter a string value. Press Enter for the default ("").
client_id>
Microsoft App Client Secret
Leave blank normally.
Enter a string value. Press Enter for the default ("").
client_secret>
Edit advanced config? (y/n)
y) Yes
n) No
y/n> n
Remote config
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
Choose a number from below, or type in an existing value
 1 / OneDrive Personal or Business
   \ "onedrive"
 2 / Sharepoint site
   \ "sharepoint"
 3 / Type in driveID
   \ "driveid"
 4 / Type in SiteID
   \ "siteid"
 5 / Search a Sharepoint site
   \ "search"
Your choice> 1
Found 1 drives, please select the one you want to use:
0: OneDrive (business) id=b!Eqwertyuiopasdfghjklzxcvbnm-7mnbvcxzlkjhgfdsapoiuytrewqk
Chose drive to use:> 0
Found drive 'root' of type 'business', URL: https://org-my.sharepoint.com/personal/you/Documents
Is that okay?
y) Yes
n) No
y/n> y
--------------------
[remote]
type = onedrive
token = {"access_token":"youraccesstoken","token_type":"Bearer","refresh_token":"yourrefreshtoken","expiry":"2018-08-26T22:39:52.486512262+08:00"}
drive_id = b!Eqwertyuiopasdfghjklzxcvbnm-7mnbvcxzlkjhgfdsapoiuytrewqk
drive_type = business
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

See the [remote setup docs](/remote_setup/) for how to set it up on a
machine with no Internet browser available.

Note that rclone runs a webserver on your local machine to collect the
token as returned from Microsoft. This only runs from the moment it
opens your browser to the moment you get back the verification
code.  This is on `http://127.0.0.1:53682/` and this it may require
you to unblock it temporarily if you are running a host firewall.

Once configured you can then use `rclone` like this,

List directories in top level of your OneDrive

    rclone lsd remote:

List all the files in your OneDrive

    rclone ls remote:

To copy a local directory to an OneDrive directory called backup

    rclone copy /home/source remote:backup

### Getting your own Client ID and Key ###

You can use your own Client ID if the default (`client_id` left blank)
one doesn't work for you or you see lots of throttling. The default
Client ID and Key is shared by all rclone users when performing
requests.

If you are having problems with them (E.g., seeing a lot of throttling), you can get your own
Client ID and Key by following the steps below:

1. Open https://portal.azure.com/#blade/Microsoft_AAD_RegisteredApps/ApplicationsListBlade and then click `New registration`.
2. Enter a name for your app, choose account type `Accounts in any organizational directory (Any Azure AD directory - Multitenant) and personal Microsoft accounts (e.g. Skype, Xbox)`, select `Web` in `Redirect URI` Enter `http://localhost:53682/` and click Register. Copy and keep the `Application (client) ID` under the app name for later use.
3. Under `manage` select `Certificates & secrets`, click `New client secret`. Copy and keep that secret for later use.
4. Under `manage` select `API permissions`, click `Add a permission` and select `Microsoft Graph` then select `delegated permissions`.
5. Search and select the following permissions: `Files.Read`, `Files.ReadWrite`, `Files.Read.All`, `Files.ReadWrite.All`, `offline_access`, `User.Read`. Once selected click `Add permissions` at the bottom.

Now the application is complete. Run `rclone config` to create or edit a OneDrive remote.
Supply the app ID and password as Client ID and Secret, respectively. rclone will walk you through the remaining steps.

### Modification time and hashes ###

OneDrive allows modification times to be set on objects accurate to 1
second.  These will be used to detect whether objects need syncing or
not.

OneDrive personal supports SHA1 type hashes. OneDrive for business and
Sharepoint Server support
[QuickXorHash](https://docs.microsoft.com/en-us/onedrive/developer/code-snippets/quickxorhash).

For all types of OneDrive you can use the `--checksum` flag.

### Restricted filename characters ###

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
| \         | 0x5C  | ＼          |
| \|        | 0x7C  | ｜          |
| #         | 0x23  | ＃          |
| %         | 0x25  | ％          |

File names can also not end with the following characters.
These only get replaced if they are the last character in the name:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| SP        | 0x20  | ␠           |
| .         | 0x2E  | ．          |

File names can also not begin with the following characters.
These only get replaced if they are the first character in the name:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| SP        | 0x20  | ␠           |
| ~         | 0x7E  | ～          |

Invalid UTF-8 bytes will also be [replaced](/overview/#invalid-utf8),
as they can't be used in JSON strings.

### Deleting files ###

Any files you delete with rclone will end up in the trash.  Microsoft
doesn't provide an API to permanently delete files, nor to empty the
trash, so you will have to do that with one of Microsoft's apps or via
the OneDrive website.

{{< rem autogenerated options start" - DO NOT EDIT - instead edit fs.RegInfo in backend/onedrive/onedrive.go then run make backenddocs" >}}
### Standard Options

Here are the standard options specific to onedrive (Microsoft OneDrive).

#### --onedrive-client-id

Microsoft App Client Id
Leave blank normally.

- Config:      client_id
- Env Var:     RCLONE_ONEDRIVE_CLIENT_ID
- Type:        string
- Default:     ""

#### --onedrive-client-secret

Microsoft App Client Secret
Leave blank normally.

- Config:      client_secret
- Env Var:     RCLONE_ONEDRIVE_CLIENT_SECRET
- Type:        string
- Default:     ""

### Advanced Options

Here are the advanced options specific to onedrive (Microsoft OneDrive).

#### --onedrive-chunk-size

Chunk size to upload files with - must be multiple of 320k (327,680 bytes).

Above this size files will be chunked - must be multiple of 320k (327,680 bytes) and
should not exceed 250M (262,144,000 bytes) else you may encounter \"Microsoft.SharePoint.Client.InvalidClientQueryException: The request message is too big.\"
Note that the chunks will be buffered into memory.

- Config:      chunk_size
- Env Var:     RCLONE_ONEDRIVE_CHUNK_SIZE
- Type:        SizeSuffix
- Default:     10M

#### --onedrive-drive-id

The ID of the drive to use

- Config:      drive_id
- Env Var:     RCLONE_ONEDRIVE_DRIVE_ID
- Type:        string
- Default:     ""

#### --onedrive-drive-type

The type of the drive ( personal | business | documentLibrary )

- Config:      drive_type
- Env Var:     RCLONE_ONEDRIVE_DRIVE_TYPE
- Type:        string
- Default:     ""

#### --onedrive-expose-onenote-files

Set to make OneNote files show up in directory listings.

By default rclone will hide OneNote files in directory listings because
operations like "Open" and "Update" won't work on them.  But this
behaviour may also prevent you from deleting them.  If you want to
delete OneNote files or otherwise want them to show up in directory
listing, set this option.

- Config:      expose_onenote_files
- Env Var:     RCLONE_ONEDRIVE_EXPOSE_ONENOTE_FILES
- Type:        bool
- Default:     false

#### --onedrive-server-side-across-configs

Allow server side operations (eg copy) to work across different onedrive configs.

This can be useful if you wish to do a server side copy between two
different Onedrives.  Note that this isn't enabled by default
because it isn't easy to tell if it will work between any two
configurations.

- Config:      server_side_across_configs
- Env Var:     RCLONE_ONEDRIVE_SERVER_SIDE_ACROSS_CONFIGS
- Type:        bool
- Default:     false

#### --onedrive-encoding

This sets the encoding for the backend.

See: the [encoding section in the overview](/overview/#encoding) for more info.

- Config:      encoding
- Env Var:     RCLONE_ONEDRIVE_ENCODING
- Type:        MultiEncoder
- Default:     Slash,LtGt,DoubleQuote,Colon,Question,Asterisk,Pipe,Hash,Percent,BackSlash,Del,Ctl,LeftSpace,LeftTilde,RightSpace,RightPeriod,InvalidUtf8,Dot

{{< rem autogenerated options stop >}}

### Limitations

If you don't use rclone for 90 days the refresh token will
expire. This will result in authorization problems. This is easy to
fix by running the `rclone config reconnect remote:` command to get a
new token and refresh token.

#### Naming ####

Note that OneDrive is case insensitive so you can't have a
file called "Hello.doc" and one called "hello.doc".

There are quite a few characters that can't be in OneDrive file
names.  These can't occur on Windows platforms, but on non-Windows
platforms they are common.  Rclone will map these names to and from an
identical looking unicode equivalent.  For example if a file has a `?`
in it will be mapped to `？` instead.

#### File sizes ####

The largest allowed file size is 100GB for both OneDrive Personal and OneDrive for Business [(Updated 17 June 2020)](https://support.microsoft.com/en-us/office/invalid-file-names-and-file-types-in-onedrive-and-sharepoint-64883a5d-228e-48f5-b3d2-eb39e07630fa?ui=en-us&rs=en-us&ad=us#individualfilesize).

#### Path length ####

The entire path, including the file name, must contain fewer than 400 characters for OneDrive, OneDrive for Business and SharePoint Online. If you are encrypting file and folder names with rclone, you may want to pay attention to this limitation because the encrypted names are typically longer than the original ones.

#### Number of files ####

OneDrive seems to be OK with at least 50,000 files in a folder, but at
100,000 rclone will get errors listing the directory like `couldn’t
list files: UnknownError:`.  See
[#2707](https://github.com/rclone/rclone/issues/2707) for more info.

An official document about the limitations for different types of OneDrive can be found [here](https://support.office.com/en-us/article/invalid-file-names-and-file-types-in-onedrive-onedrive-for-business-and-sharepoint-64883a5d-228e-48f5-b3d2-eb39e07630fa).

### Versions

Every change in a file OneDrive causes the service to create a new
version of the the file.  This counts against a users quota.  For
example changing the modification time of a file creates a second
version, so the file apparently uses twice the space.

For example the `copy` command is affected by this as rclone copies
the file and then afterwards sets the modification time to match the
source file which uses another version.

You can use the `rclone cleanup` command (see below) to remove all old
versions.

Or you can set the `no_versions` parameter to `true` and rclone will
remove versions after operations which create new versions. This takes
extra transactions so only enable it if you need it.

**Note** At the time of writing Onedrive Personal creates versions
(but not for setting the modification time) but the API for removing
them returns "API not found" so cleanup and `no_versions` should not
be used on Onedrive Personal.

### Disabling versioning

Starting October 2018, users will no longer be able to
disable versioning by default. This is because Microsoft has brought
an
[update](https://techcommunity.microsoft.com/t5/Microsoft-OneDrive-Blog/New-Updates-to-OneDrive-and-SharePoint-Team-Site-Versioning/ba-p/204390)
to the mechanism. To change this new default setting, a PowerShell
command is required to be run by a SharePoint admin. If you are an
admin, you can run these commands in PowerShell to change that
setting:

1. `Install-Module -Name Microsoft.Online.SharePoint.PowerShell` (in case you haven't installed this already)
2. `Import-Module Microsoft.Online.SharePoint.PowerShell -DisableNameChecking`
3. `Connect-SPOService -Url https://YOURSITE-admin.sharepoint.com -Credential YOU@YOURSITE.COM` (replacing `YOURSITE`, `YOU`, `YOURSITE.COM` with the actual values; this will prompt for your credentials)
4. `Set-SPOTenant -EnableMinimumVersionRequirement $False`
5. `Disconnect-SPOService` (to disconnect from the server)

*Below are the steps for normal users to disable versioning. If you don't see the "No Versioning" option, make sure the above requirements are met.*  

User [Weropol](https://github.com/Weropol) has found a method to disable
versioning on OneDrive

1. Open the settings menu by clicking on the gear symbol at the top of the OneDrive Business page.
2. Click Site settings.
3. Once on the Site settings page, navigate to Site Administration > Site libraries and lists.
4. Click Customize "Documents".
5. Click General Settings > Versioning Settings.
6. Under Document Version History select the option No versioning.
Note: This will disable the creation of new file versions, but will not remove any previous versions. Your documents are safe.
7. Apply the changes by clicking OK.
8. Use rclone to upload or modify files. (I also use the --no-update-modtime flag)
9. Restore the versioning settings after using rclone. (Optional)

### Cleanup

OneDrive supports `rclone cleanup` which causes rclone to look through
every file under the path supplied and delete all version but the
current version. Because this involves traversing all the files, then
querying each file for versions it can be quite slow. Rclone does
`--checkers` tests in parallel. The command also supports `-i` which
is a great way to see what it would do.

    rclone cleanup -i remote:path/subdir # interactively remove all old version for path/subdir
    rclone cleanup remote:path/subdir    # unconditionally remove all old version for path/subdir

**NB** Onedrive personal can't currently delete versions

### Troubleshooting ###

#### Unexpected file size/hash differences on Sharepoint ####

It is a
[known](https://github.com/OneDrive/onedrive-api-docs/issues/935#issuecomment-441741631)
issue that Sharepoint (not OneDrive or OneDrive for Business) silently modifies
uploaded files, mainly Office files (.docx, .xlsx, etc.), causing file size and
hash checks to fail. To use rclone with such affected files on Sharepoint, you
may disable these checks with the following command line arguments:

```
--ignore-checksum --ignore-size
```

#### Replacing/deleting existing files on Sharepoint gets "item not found" ####

It is a [known](https://github.com/OneDrive/onedrive-api-docs/issues/1068) issue
that Sharepoint (not OneDrive or OneDrive for Business) may return "item not
found" errors when users try to replace or delete uploaded files; this seems to
mainly affect Office files (.docx, .xlsx, etc.). As a workaround, you may use
the `--backup-dir <BACKUP_DIR>` command line argument so rclone moves the
files to be replaced/deleted into a given backup directory (instead of directly
replacing/deleting them). For example, to instruct rclone to move the files into
the directory `rclone-backup-dir` on backend `mysharepoint`, you may use:

```
--backup-dir mysharepoint:rclone-backup-dir
```

#### access\_denied (AADSTS65005) ####

```
Error: access_denied
Code: AADSTS65005
Description: Using application 'rclone' is currently not supported for your organization [YOUR_ORGANIZATION] because it is in an unmanaged state. An administrator needs to claim ownership of the company by DNS validation of [YOUR_ORGANIZATION] before the application rclone can be provisioned.
```

This means that rclone can't use the OneDrive for Business API with your account. You can't do much about it, maybe write an email to your admins.

However, there are other ways to interact with your OneDrive account. Have a look at the webdav backend: https://rclone.org/webdav/#sharepoint

#### invalid\_grant (AADSTS50076) ####

```
Error: invalid_grant
Code: AADSTS50076
Description: Due to a configuration change made by your administrator, or because you moved to a new location, you must use multi-factor authentication to access '...'.
```

If you see the error above after enabling multi-factor authentication for your account, you can fix it by refreshing your OAuth refresh token. To do that, run `rclone config`, and choose to edit your OneDrive backend. Then, you don't need to actually make any changes until you reach this question: `Already have a token - refresh?`. For this question, answer `y` and go through the process to refresh your token, just like the first time the backend is configured. After this, rclone should work again for this backend.
