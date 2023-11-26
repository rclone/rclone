---
title: "Microsoft OneDrive"
description: "Rclone docs for Microsoft OneDrive"
versionIntroduced: "v1.24"
---

# {{< icon "fab fa-windows" >}} Microsoft OneDrive

Paths are specified as `remote:path`

Paths may be as deep as required, e.g. `remote:directory/subdirectory`.

## Configuration

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
Use web browser to automatically authenticate rclone with remote?
 * Say Y if the machine running rclone has a web browser you can use
 * Say N if running rclone on a (remote) machine without web browser access
If not sure try Y. If Y failed, try N.
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

### Getting your own Client ID and Key

rclone uses a default Client ID when talking to OneDrive, unless a custom `client_id` is specified in the config.
The default Client ID and Key are shared by all rclone users when performing requests.

You may choose to create and use your own Client ID, in case the default one does not work well for you. 
For example, you might see throttling.

#### Creating Client ID for OneDrive Personal

To create your own Client ID, please follow these steps:

1. Open https://portal.azure.com/#blade/Microsoft_AAD_RegisteredApps/ApplicationsListBlade and then click `New registration`.
2. Enter a name for your app, choose account type `Accounts in any organizational directory (Any Azure AD directory - Multitenant) and personal Microsoft accounts (e.g. Skype, Xbox)`, select `Web` in `Redirect URI`, then type (do not copy and paste) `http://localhost:53682/` and click Register. Copy and keep the `Application (client) ID` under the app name for later use.
3. Under `manage` select `Certificates & secrets`, click `New client secret`. Enter a description (can be anything) and set `Expires` to 24 months. Copy and keep that secret _Value_ for later use (you _won't_ be able to see this value afterwards).
4. Under `manage` select `API permissions`, click `Add a permission` and select `Microsoft Graph` then select `delegated permissions`.
5. Search and select the following permissions: `Files.Read`, `Files.ReadWrite`, `Files.Read.All`, `Files.ReadWrite.All`, `offline_access`, `User.Read` and `Sites.Read.All` (if custom access scopes are configured, select the permissions accordingly). Once selected click `Add permissions` at the bottom.

Now the application is complete. Run `rclone config` to create or edit a OneDrive remote.
Supply the app ID and password as Client ID and Secret, respectively. rclone will walk you through the remaining steps.

The access_scopes option allows you to configure the permissions requested by rclone.
See [Microsoft Docs](https://docs.microsoft.com/en-us/graph/permissions-reference#files-permissions) for more information about the different scopes.

The `Sites.Read.All` permission is required if you need to [search SharePoint sites when configuring the remote](https://github.com/rclone/rclone/pull/5883). However, if that permission is not assigned, you need to exclude `Sites.Read.All` from your access scopes or set `disable_site_permission` option to true in the advanced options.

#### Creating Client ID for OneDrive Business

The steps for OneDrive Personal may or may not work for OneDrive Business, depending on the security settings of the organization.
A common error is that the publisher of the App is not verified.

You may try to [verify you account](https://docs.microsoft.com/en-us/azure/active-directory/develop/publisher-verification-overview), or try to limit the App to your organization only, as shown below.

1. Make sure to create the App with your business account.
2. Follow the steps above to create an App. However, we need a different account type here: `Accounts in this organizational directory only (*** - Single tenant)`. Note that you can also change the account type after creating the App.
3. Find the [tenant ID](https://docs.microsoft.com/en-us/azure/active-directory/fundamentals/active-directory-how-to-find-tenant) of your organization.
4. In the rclone config, set `auth_url` to `https://login.microsoftonline.com/YOUR_TENANT_ID/oauth2/v2.0/authorize`.
5. In the rclone config, set `token_url` to `https://login.microsoftonline.com/YOUR_TENANT_ID/oauth2/v2.0/token`.

Note: If you have a special region, you may need a different host in step 4 and 5. Here are [some hints](https://github.com/rclone/rclone/blob/bc23bf11db1c78c6ebbf8ea538fbebf7058b4176/backend/onedrive/onedrive.go#L86).


### Modification times and hashes

OneDrive allows modification times to be set on objects accurate to 1
second.  These will be used to detect whether objects need syncing or
not.

OneDrive Personal, OneDrive for Business and Sharepoint Server support
[QuickXorHash](https://docs.microsoft.com/en-us/onedrive/developer/code-snippets/quickxorhash).

Before rclone 1.62 the default hash for Onedrive Personal was `SHA1`.
For rclone 1.62 and above the default for all Onedrive backends is
`QuickXorHash`.

Starting from July 2023 `SHA1` support is being phased out in Onedrive
Personal in favour of `QuickXorHash`. If necessary the
`--onedrive-hash-type` flag (or `hash_type` config option) can be used
to select `SHA1` during the transition period if this is important
your workflow.

For all types of OneDrive you can use the `--checksum` flag.

### --fast-list

This remote supports `--fast-list` which allows you to use fewer
transactions in exchange for more memory. See the [rclone
docs](/docs/#fast-list) for more details.

This must be enabled with the `--onedrive-delta` flag (or `delta =
true` in the config file) as it can cause performance degradation.

It does this by using the delta listing facilities of OneDrive which
returns all the files in the remote very efficiently. This is much
more efficient than listing directories recursively and is Microsoft's
recommended way of reading all the file information from a drive.

This can be useful with `rclone mount` and [rclone rc vfs/refresh
recursive=true](/rc/#vfs-refresh)) to very quickly fill the mount with
information about all the files.

The API used for the recursive listing (`ListR`) only supports listing
from the root of the drive. This will become increasingly inefficient
the further away you get from the root as rclone will have to discard
files outside of the directory you are using.

Some commands (like `rclone lsf -R`) will use `ListR` by default - you
can turn this off with `--disable ListR` if you need to.

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
| \         | 0x5C  | ＼          |
| \|        | 0x7C  | ｜          |

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

### Deleting files

Any files you delete with rclone will end up in the trash.  Microsoft
doesn't provide an API to permanently delete files, nor to empty the
trash, so you will have to do that with one of Microsoft's apps or via
the OneDrive website.

{{< rem autogenerated options start" - DO NOT EDIT - instead edit fs.RegInfo in backend/onedrive/onedrive.go then run make backenddocs" >}}
### Standard options

Here are the Standard options specific to onedrive (Microsoft OneDrive).

#### --onedrive-client-id

OAuth Client Id.

Leave blank normally.

Properties:

- Config:      client_id
- Env Var:     RCLONE_ONEDRIVE_CLIENT_ID
- Type:        string
- Required:    false

#### --onedrive-client-secret

OAuth Client Secret.

Leave blank normally.

Properties:

- Config:      client_secret
- Env Var:     RCLONE_ONEDRIVE_CLIENT_SECRET
- Type:        string
- Required:    false

#### --onedrive-region

Choose national cloud region for OneDrive.

Properties:

- Config:      region
- Env Var:     RCLONE_ONEDRIVE_REGION
- Type:        string
- Default:     "global"
- Examples:
    - "global"
        - Microsoft Cloud Global
    - "us"
        - Microsoft Cloud for US Government
    - "de"
        - Microsoft Cloud Germany
    - "cn"
        - Azure and Office 365 operated by Vnet Group in China

### Advanced options

Here are the Advanced options specific to onedrive (Microsoft OneDrive).

#### --onedrive-token

OAuth Access Token as a JSON blob.

Properties:

- Config:      token
- Env Var:     RCLONE_ONEDRIVE_TOKEN
- Type:        string
- Required:    false

#### --onedrive-auth-url

Auth server URL.

Leave blank to use the provider defaults.

Properties:

- Config:      auth_url
- Env Var:     RCLONE_ONEDRIVE_AUTH_URL
- Type:        string
- Required:    false

#### --onedrive-token-url

Token server url.

Leave blank to use the provider defaults.

Properties:

- Config:      token_url
- Env Var:     RCLONE_ONEDRIVE_TOKEN_URL
- Type:        string
- Required:    false

#### --onedrive-chunk-size

Chunk size to upload files with - must be multiple of 320k (327,680 bytes).

Above this size files will be chunked - must be multiple of 320k (327,680 bytes) and
should not exceed 250M (262,144,000 bytes) else you may encounter \"Microsoft.SharePoint.Client.InvalidClientQueryException: The request message is too big.\"
Note that the chunks will be buffered into memory.

Properties:

- Config:      chunk_size
- Env Var:     RCLONE_ONEDRIVE_CHUNK_SIZE
- Type:        SizeSuffix
- Default:     10Mi

#### --onedrive-drive-id

The ID of the drive to use.

Properties:

- Config:      drive_id
- Env Var:     RCLONE_ONEDRIVE_DRIVE_ID
- Type:        string
- Required:    false

#### --onedrive-drive-type

The type of the drive (personal | business | documentLibrary).

Properties:

- Config:      drive_type
- Env Var:     RCLONE_ONEDRIVE_DRIVE_TYPE
- Type:        string
- Required:    false

#### --onedrive-root-folder-id

ID of the root folder.

This isn't normally needed, but in special circumstances you might
know the folder ID that you wish to access but not be able to get
there through a path traversal.


Properties:

- Config:      root_folder_id
- Env Var:     RCLONE_ONEDRIVE_ROOT_FOLDER_ID
- Type:        string
- Required:    false

#### --onedrive-access-scopes

Set scopes to be requested by rclone.

Choose or manually enter a custom space separated list with all scopes, that rclone should request.


Properties:

- Config:      access_scopes
- Env Var:     RCLONE_ONEDRIVE_ACCESS_SCOPES
- Type:        SpaceSepList
- Default:     Files.Read Files.ReadWrite Files.Read.All Files.ReadWrite.All Sites.Read.All offline_access
- Examples:
    - "Files.Read Files.ReadWrite Files.Read.All Files.ReadWrite.All Sites.Read.All offline_access"
        - Read and write access to all resources
    - "Files.Read Files.Read.All Sites.Read.All offline_access"
        - Read only access to all resources
    - "Files.Read Files.ReadWrite Files.Read.All Files.ReadWrite.All offline_access"
        - Read and write access to all resources, without the ability to browse SharePoint sites. 
        - Same as if disable_site_permission was set to true

#### --onedrive-disable-site-permission

Disable the request for Sites.Read.All permission.

If set to true, you will no longer be able to search for a SharePoint site when
configuring drive ID, because rclone will not request Sites.Read.All permission.
Set it to true if your organization didn't assign Sites.Read.All permission to the
application, and your organization disallows users to consent app permission
request on their own.

Properties:

- Config:      disable_site_permission
- Env Var:     RCLONE_ONEDRIVE_DISABLE_SITE_PERMISSION
- Type:        bool
- Default:     false

#### --onedrive-expose-onenote-files

Set to make OneNote files show up in directory listings.

By default, rclone will hide OneNote files in directory listings because
operations like "Open" and "Update" won't work on them.  But this
behaviour may also prevent you from deleting them.  If you want to
delete OneNote files or otherwise want them to show up in directory
listing, set this option.

Properties:

- Config:      expose_onenote_files
- Env Var:     RCLONE_ONEDRIVE_EXPOSE_ONENOTE_FILES
- Type:        bool
- Default:     false

#### --onedrive-server-side-across-configs

Deprecated: use --server-side-across-configs instead.

Allow server-side operations (e.g. copy) to work across different onedrive configs.

This will only work if you are copying between two OneDrive *Personal* drives AND
the files to copy are already shared between them.  In other cases, rclone will
fall back to normal copy (which will be slightly slower).

Properties:

- Config:      server_side_across_configs
- Env Var:     RCLONE_ONEDRIVE_SERVER_SIDE_ACROSS_CONFIGS
- Type:        bool
- Default:     false

#### --onedrive-list-chunk

Size of listing chunk.

Properties:

- Config:      list_chunk
- Env Var:     RCLONE_ONEDRIVE_LIST_CHUNK
- Type:        int
- Default:     1000

#### --onedrive-no-versions

Remove all versions on modifying operations.

Onedrive for business creates versions when rclone uploads new files
overwriting an existing one and when it sets the modification time.

These versions take up space out of the quota.

This flag checks for versions after file upload and setting
modification time and removes all but the last version.

**NB** Onedrive personal can't currently delete versions so don't use
this flag there.


Properties:

- Config:      no_versions
- Env Var:     RCLONE_ONEDRIVE_NO_VERSIONS
- Type:        bool
- Default:     false

#### --onedrive-link-scope

Set the scope of the links created by the link command.

Properties:

- Config:      link_scope
- Env Var:     RCLONE_ONEDRIVE_LINK_SCOPE
- Type:        string
- Default:     "anonymous"
- Examples:
    - "anonymous"
        - Anyone with the link has access, without needing to sign in.
        - This may include people outside of your organization.
        - Anonymous link support may be disabled by an administrator.
    - "organization"
        - Anyone signed into your organization (tenant) can use the link to get access.
        - Only available in OneDrive for Business and SharePoint.

#### --onedrive-link-type

Set the type of the links created by the link command.

Properties:

- Config:      link_type
- Env Var:     RCLONE_ONEDRIVE_LINK_TYPE
- Type:        string
- Default:     "view"
- Examples:
    - "view"
        - Creates a read-only link to the item.
    - "edit"
        - Creates a read-write link to the item.
    - "embed"
        - Creates an embeddable link to the item.

#### --onedrive-link-password

Set the password for links created by the link command.

At the time of writing this only works with OneDrive personal paid accounts.


Properties:

- Config:      link_password
- Env Var:     RCLONE_ONEDRIVE_LINK_PASSWORD
- Type:        string
- Required:    false

#### --onedrive-hash-type

Specify the hash in use for the backend.

This specifies the hash type in use. If set to "auto" it will use the
default hash which is QuickXorHash.

Before rclone 1.62 an SHA1 hash was used by default for Onedrive
Personal. For 1.62 and later the default is to use a QuickXorHash for
all onedrive types. If an SHA1 hash is desired then set this option
accordingly.

From July 2023 QuickXorHash will be the only available hash for
both OneDrive for Business and OneDriver Personal.

This can be set to "none" to not use any hashes.

If the hash requested does not exist on the object, it will be
returned as an empty string which is treated as a missing hash by
rclone.


Properties:

- Config:      hash_type
- Env Var:     RCLONE_ONEDRIVE_HASH_TYPE
- Type:        string
- Default:     "auto"
- Examples:
    - "auto"
        - Rclone chooses the best hash
    - "quickxor"
        - QuickXor
    - "sha1"
        - SHA1
    - "sha256"
        - SHA256
    - "crc32"
        - CRC32
    - "none"
        - None - don't use any hashes

#### --onedrive-av-override

Allows download of files the server thinks has a virus.

The onedrive/sharepoint server may check files uploaded with an Anti
Virus checker. If it detects any potential viruses or malware it will
block download of the file.

In this case you will see a message like this

    server reports this file is infected with a virus - use --onedrive-av-override to download anyway: Infected (name of virus): 403 Forbidden: 

If you are 100% sure you want to download this file anyway then use
the --onedrive-av-override flag, or av_override = true in the config
file.


Properties:

- Config:      av_override
- Env Var:     RCLONE_ONEDRIVE_AV_OVERRIDE
- Type:        bool
- Default:     false

#### --onedrive-delta

If set rclone will use delta listing to implement recursive listings.

If this flag is set the the onedrive backend will advertise `ListR`
support for recursive listings.

Setting this flag speeds up these things greatly:

    rclone lsf -R onedrive:
    rclone size onedrive:
    rclone rc vfs/refresh recursive=true

**However** the delta listing API **only** works at the root of the
drive. If you use it not at the root then it recurses from the root
and discards all the data that is not under the directory you asked
for. So it will be correct but may not be very efficient.

This is why this flag is not set as the default.

As a rule of thumb if nearly all of your data is under rclone's root
directory (the `root/directory` in `onedrive:root/directory`) then
using this flag will be be a big performance win. If your data is
mostly not under the root then using this flag will be a big
performance loss.

It is recommended if you are mounting your onedrive at the root
(or near the root when using crypt) and using rclone `rc vfs/refresh`.


Properties:

- Config:      delta
- Env Var:     RCLONE_ONEDRIVE_DELTA
- Type:        bool
- Default:     false

#### --onedrive-encoding

The encoding for the backend.

See the [encoding section in the overview](/overview/#encoding) for more info.

Properties:

- Config:      encoding
- Env Var:     RCLONE_ONEDRIVE_ENCODING
- Type:        Encoding
- Default:     Slash,LtGt,DoubleQuote,Colon,Question,Asterisk,Pipe,BackSlash,Del,Ctl,LeftSpace,LeftTilde,RightSpace,RightPeriod,InvalidUtf8,Dot

{{< rem autogenerated options stop >}}

## Limitations

If you don't use rclone for 90 days the refresh token will
expire. This will result in authorization problems. This is easy to
fix by running the `rclone config reconnect remote:` command to get a
new token and refresh token.

### Naming

Note that OneDrive is case insensitive so you can't have a
file called "Hello.doc" and one called "hello.doc".

There are quite a few characters that can't be in OneDrive file
names.  These can't occur on Windows platforms, but on non-Windows
platforms they are common.  Rclone will map these names to and from an
identical looking unicode equivalent.  For example if a file has a `?`
in it will be mapped to `？` instead.

### File sizes

The largest allowed file size is 250 GiB for both OneDrive Personal and OneDrive for Business [(Updated 13 Jan 2021)](https://support.microsoft.com/en-us/office/invalid-file-names-and-file-types-in-onedrive-and-sharepoint-64883a5d-228e-48f5-b3d2-eb39e07630fa?ui=en-us&rs=en-us&ad=us#individualfilesize).

### Path length

The entire path, including the file name, must contain fewer than 400 characters for OneDrive, OneDrive for Business and SharePoint Online. If you are encrypting file and folder names with rclone, you may want to pay attention to this limitation because the encrypted names are typically longer than the original ones.

### Number of files

OneDrive seems to be OK with at least 50,000 files in a folder, but at
100,000 rclone will get errors listing the directory like `couldn’t
list files: UnknownError:`.  See
[#2707](https://github.com/rclone/rclone/issues/2707) for more info.

An official document about the limitations for different types of OneDrive can be found [here](https://support.office.com/en-us/article/invalid-file-names-and-file-types-in-onedrive-onedrive-for-business-and-sharepoint-64883a5d-228e-48f5-b3d2-eb39e07630fa).

## Versions

Every change in a file OneDrive causes the service to create a new
version of the file.  This counts against a users quota.  For
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

## Cleanup

OneDrive supports `rclone cleanup` which causes rclone to look through
every file under the path supplied and delete all version but the
current version. Because this involves traversing all the files, then
querying each file for versions it can be quite slow. Rclone does
`--checkers` tests in parallel. The command also supports `--interactive`/`i`
or `--dry-run` which is a great way to see what it would do.

    rclone cleanup --interactive remote:path/subdir # interactively remove all old version for path/subdir
    rclone cleanup remote:path/subdir               # unconditionally remove all old version for path/subdir

**NB** Onedrive personal can't currently delete versions

## Troubleshooting ##

### Excessive throttling or blocked on SharePoint

If you experience excessive throttling or is being blocked on SharePoint then it may help to set the user agent explicitly with a flag like this: `--user-agent "ISV|rclone.org|rclone/v1.55.1"`

The specific details can be found in the Microsoft document: [Avoid getting throttled or blocked in SharePoint Online](https://docs.microsoft.com/en-us/sharepoint/dev/general-development/how-to-avoid-getting-throttled-or-blocked-in-sharepoint-online#how-to-decorate-your-http-traffic-to-avoid-throttling)

### Unexpected file size/hash differences on Sharepoint ####

It is a
[known](https://github.com/OneDrive/onedrive-api-docs/issues/935#issuecomment-441741631)
issue that Sharepoint (not OneDrive or OneDrive for Business) silently modifies
uploaded files, mainly Office files (.docx, .xlsx, etc.), causing file size and
hash checks to fail. There are also other situations that will cause OneDrive to
report inconsistent file sizes. To use rclone with such
affected files on Sharepoint, you
may disable these checks with the following command line arguments:

```
--ignore-checksum --ignore-size
```

Alternatively, if you have write access to the OneDrive files, it may be possible
to fix this problem for certain files, by attempting the steps below.
Open the web interface for [OneDrive](https://onedrive.live.com) and find the
affected files (which will be in the error messages/log for rclone). Simply click on
each of these files, causing OneDrive to open them on the web. This will cause each
file to be converted in place to a format that is functionally equivalent
but which will no longer trigger the size discrepancy. Once all problematic files
are converted you will no longer need the ignore options above.

### Replacing/deleting existing files on Sharepoint gets "item not found" ####

It is a [known](https://github.com/OneDrive/onedrive-api-docs/issues/1068) issue
that Sharepoint (not OneDrive or OneDrive for Business) may return "item not
found" errors when users try to replace or delete uploaded files; this seems to
mainly affect Office files (.docx, .xlsx, etc.) and web files (.html, .aspx, etc.). As a workaround, you may use
the `--backup-dir <BACKUP_DIR>` command line argument so rclone moves the
files to be replaced/deleted into a given backup directory (instead of directly
replacing/deleting them). For example, to instruct rclone to move the files into
the directory `rclone-backup-dir` on backend `mysharepoint`, you may use:

```
--backup-dir mysharepoint:rclone-backup-dir
```

### access\_denied (AADSTS65005) ####

```
Error: access_denied
Code: AADSTS65005
Description: Using application 'rclone' is currently not supported for your organization [YOUR_ORGANIZATION] because it is in an unmanaged state. An administrator needs to claim ownership of the company by DNS validation of [YOUR_ORGANIZATION] before the application rclone can be provisioned.
```

This means that rclone can't use the OneDrive for Business API with your account. You can't do much about it, maybe write an email to your admins.

However, there are other ways to interact with your OneDrive account. Have a look at the WebDAV backend: https://rclone.org/webdav/#sharepoint

### invalid\_grant (AADSTS50076) ####

```
Error: invalid_grant
Code: AADSTS50076
Description: Due to a configuration change made by your administrator, or because you moved to a new location, you must use multi-factor authentication to access '...'.
```

If you see the error above after enabling multi-factor authentication for your account, you can fix it by refreshing your OAuth refresh token. To do that, run `rclone config`, and choose to edit your OneDrive backend. Then, you don't need to actually make any changes until you reach this question: `Already have a token - refresh?`. For this question, answer `y` and go through the process to refresh your token, just like the first time the backend is configured. After this, rclone should work again for this backend.

### Invalid request when making public links ####

On Sharepoint and OneDrive for Business, `rclone link` may return an "Invalid
request" error. A possible cause is that the organisation admin didn't allow
public links to be made for the organisation/sharepoint library. To fix the
permissions as an admin, take a look at the docs:
[1](https://docs.microsoft.com/en-us/sharepoint/turn-external-sharing-on-or-off),
[2](https://support.microsoft.com/en-us/office/set-up-and-manage-access-requests-94b26e0b-2822-49d4-929a-8455698654b3).

### Can not access `Shared` with me files

Shared with me files is not supported by rclone [currently](https://github.com/rclone/rclone/issues/4062), but there is a workaround:

1. Visit [https://onedrive.live.com](https://onedrive.live.com/)
2. Right click a item in `Shared`, then click `Add shortcut to My files` in the context
    ![make_shortcut](https://user-images.githubusercontent.com/60313789/206118040-7e762b3b-aa61-41a1-8649-cc18889f3572.png "Screenshot (Shared with me)")
3. The shortcut will appear in `My files`, you can access it with rclone, it behaves like a normal folder/file.
    ![in_my_files](https://i.imgur.com/0S8H3li.png "Screenshot (My Files)")
    ![rclone_mount](https://i.imgur.com/2Iq66sW.png "Screenshot (rclone mount)")

### Live Photos uploaded from iOS (small video clips in .heic files)

The iOS OneDrive app introduced [upload and storage](https://techcommunity.microsoft.com/t5/microsoft-onedrive-blog/live-photos-come-to-onedrive/ba-p/1953452) 
of [Live Photos](https://support.apple.com/en-gb/HT207310) in 2020. 
The usage and download of these uploaded Live Photos is unfortunately still work-in-progress 
and this introduces several issues when copying, synchronising and mounting – both in rclone and in the native OneDrive client on Windows.

The root cause can easily be seen if you locate one of your Live Photos in the OneDrive web interface. 
Then download the photo from the web interface. You will then see that the size of downloaded .heic file is smaller than the size displayed in the web interface. 
The downloaded file is smaller because it only contains a single frame (still photo) extracted from the Live Photo (movie) stored in OneDrive.

The different sizes will cause `rclone copy/sync` to repeatedly recopy unmodified photos something like this:

    DEBUG : 20230203_123826234_iOS.heic: Sizes differ (src 4470314 vs dst 1298667)
    DEBUG : 20230203_123826234_iOS.heic: sha1 = fc2edde7863b7a7c93ca6771498ac797f8460750 OK
    INFO  : 20230203_123826234_iOS.heic: Copied (replaced existing)

These recopies can be worked around by adding `--ignore-size`. Please note that this workaround only syncs the still-picture not the movie clip, 
and relies on modification dates being correctly updated on all files in all situations.

The different sizes will also cause `rclone check` to report size errors something like this:

    ERROR : 20230203_123826234_iOS.heic: sizes differ

These check errors can be suppressed by adding `--ignore-size`.

The different sizes will also cause `rclone mount` to fail downloading with an error something like this:

    ERROR : 20230203_123826234_iOS.heic: ReadFileHandle.Read error: low level retry 1/10: unexpected EOF

or like this when using `--cache-mode=full`:

    INFO  : 20230203_123826234_iOS.heic: vfs cache: downloader: error count now 1: vfs reader: failed to write to cache file: 416 Requested Range Not Satisfiable:
    ERROR : 20230203_123826234_iOS.heic: vfs cache: failed to download: vfs reader: failed to write to cache file: 416 Requested Range Not Satisfiable:
