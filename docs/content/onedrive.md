---
title: "Microsoft OneDrive"
description: "Rclone docs for Microsoft OneDrive"
date: "2015-10-14"
---

<i class="fa fa-windows"></i> Microsoft OneDrive
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
...
17 / Microsoft OneDrive
   \ "onedrive"
...
Storage> 17
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

rclone uses a pair of Client ID and Key shared by all rclone users when performing requests by default.
If you are having problems with them (E.g., seeing a lot of throttling), you can get your own
Client ID and Key by following the steps below:

1. Open https://apps.dev.microsoft.com/#/appList, then click `Add an app` (Choose `Converged applications` if applicable)
2. Enter a name for your app, and click continue. Copy and keep the `Application Id` under the app name for later use.
3. Under section `Application Secrets`, click `Generate New Password`. Copy and keep that password for later use.
4. Under section `Platforms`, click `Add platform`, then `Web`. Enter `http://localhost:53682/` in
`Redirect URLs`.
5. Under section `Microsoft Graph Permissions`, `Add` these `delegated permissions`:
`Files.Read`, `Files.ReadWrite`, `Files.Read.All`, `Files.ReadWrite.All`, `offline_access`, `User.Read`.
6. Scroll to the bottom and click `Save`.

Now the application is complete. Run `rclone config` to create or edit a OneDrive remote.
Supply the app ID and password as Client ID and Secret, respectively. rclone will walk you through the remaining steps.

### Modified time and hashes ###

OneDrive allows modification times to be set on objects accurate to 1
second.  These will be used to detect whether objects need syncing or
not.

OneDrive personal supports SHA1 type hashes. OneDrive for business and
Sharepoint Server support
[QuickXorHash](https://docs.microsoft.com/en-us/onedrive/developer/code-snippets/quickxorhash).

For all types of OneDrive you can use the `--checksum` flag.

### Deleting files ###

Any files you delete with rclone will end up in the trash.  Microsoft
doesn't provide an API to permanently delete files, nor to empty the
trash, so you will have to do that with one of Microsoft's apps or via
the OneDrive website.

<!--- autogenerated options start - DO NOT EDIT, instead edit fs.RegInfo in backend/onedrive/onedrive.go then run make backenddocs -->
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

Chunk size to upload files with - must be multiple of 320k.

Above this size files will be chunked - must be multiple of 320k. Note
that the chunks will be buffered into memory.

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

<!--- autogenerated options stop -->

### Limitations ###

Note that OneDrive is case insensitive so you can't have a
file called "Hello.doc" and one called "hello.doc".

There are quite a few characters that can't be in OneDrive file
names.  These can't occur on Windows platforms, but on non-Windows
platforms they are common.  Rclone will map these names to and from an
identical looking unicode equivalent.  For example if a file has a `?`
in it will be mapped to `？` instead.

The largest allowed file sizes are 15GB for OneDrive for Business and 35GB for OneDrive Personal (Updated 4 Jan 2019).

The entire path, including the file name, must contain fewer than 400 characters for OneDrive, OneDrive for Business and SharePoint Online. If you are encrypting file and folder names with rclone, you may want to pay attention to this limitation because the encrypted names are typically longer than the original ones.

OneDrive seems to be OK with at least 50,000 files in a folder, but at
100,000 rclone will get errors listing the directory like `couldn’t
list files: UnknownError:`.  See
[#2707](https://github.com/ncw/rclone/issues/2707) for more info.

An official document about the limitations for different types of OneDrive can be found [here](https://support.office.com/en-us/article/invalid-file-names-and-file-types-in-onedrive-onedrive-for-business-and-sharepoint-64883a5d-228e-48f5-b3d2-eb39e07630fa). 

### Versioning issue ###

Every change in OneDrive causes the service to create a new version.
This counts against a users quota.
For example changing the modification time of a file creates a second
version, so the file is using twice the space.

The `copy` is the only rclone command affected by this as we copy
the file and then afterwards set the modification time to match the
source file.

**Note**: Starting October 2018, users will no longer be able to disable versioning by default. This is because Microsoft has brought an [update](https://techcommunity.microsoft.com/t5/Microsoft-OneDrive-Blog/New-Updates-to-OneDrive-and-SharePoint-Team-Site-Versioning/ba-p/204390) to the mechanism. To change this new default setting, a PowerShell command is required to be run by a SharePoint admin. If you are an admin, you can run these commands in PowerShell to change that setting:

1. `Install-Module -Name Microsoft.Online.SharePoint.PowerShell` (in case you haven't installed this already)
1. `Import-Module Microsoft.Online.SharePoint.PowerShell -DisableNameChecking`
1. `Connect-SPOService -Url https://YOURSITE-admin.sharepoint.com -Credential YOU@YOURSITE.COM` (replacing `YOURSITE`, `YOU`, `YOURSITE.COM` with the actual values; this will prompt for your credentials)
1. `Set-SPOTenant -EnableMinimumVersionRequirement $False`
1. `Disconnect-SPOService` (to disconnect from the server)

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

### Troubleshooting ###

```
Error: access_denied
Code: AADSTS65005
Description: Using application 'rclone' is currently not supported for your organization [YOUR_ORGANIZATION] because it is in an unmanaged state. An administrator needs to claim ownership of the company by DNS validation of [YOUR_ORGANIZATION] before the application rclone can be provisioned.
```

This means that rclone can't use the OneDrive for Business API with your account. You can't do much about it, maybe write an email to your admins.

However, there are other ways to interact with your OneDrive account. Have a look at the webdav backend: https://rclone.org/webdav/#sharepoint