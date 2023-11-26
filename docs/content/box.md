---
title: "Box"
description: "Rclone docs for Box"
versionIntroduced: "v1.38"
---

# {{< icon "fa fa-archive" >}} Box

Paths are specified as `remote:path`

Paths may be as deep as required, e.g. `remote:directory/subdirectory`.

The initial setup for Box involves getting a token from Box which you
can do either in your browser, or with a config.json downloaded from Box
to use JWT authentication.  `rclone config` walks you through it.

## Configuration

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found, make a new one?
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / Box
   \ "box"
[snip]
Storage> box
Box App Client Id - leave blank normally.
client_id> 
Box App Client Secret - leave blank normally.
client_secret>
Box App config.json location
Leave blank normally.
Enter a string value. Press Enter for the default ("").
box_config_file>
Box App Primary Access Token
Leave blank normally.
Enter a string value. Press Enter for the default ("").
access_token>

Enter a string value. Press Enter for the default ("user").
Choose a number from below, or type in your own value
 1 / Rclone should act on behalf of a user
   \ "user"
 2 / Rclone should act on behalf of a service account
   \ "enterprise"
box_sub_type>
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
--------------------
[remote]
client_id = 
client_secret = 
token = {"access_token":"XXX","token_type":"bearer","refresh_token":"XXX","expiry":"XXX"}
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

See the [remote setup docs](/remote_setup/) for how to set it up on a
machine with no Internet browser available.

Note that rclone runs a webserver on your local machine to collect the
token as returned from Box. This only runs from the moment it opens
your browser to the moment you get back the verification code.  This
is on `http://127.0.0.1:53682/` and this it may require you to unblock
it temporarily if you are running a host firewall.

Once configured you can then use `rclone` like this,

List directories in top level of your Box

    rclone lsd remote:

List all the files in your Box

    rclone ls remote:

To copy a local directory to an Box directory called backup

    rclone copy /home/source remote:backup

### Using rclone with an Enterprise account with SSO

If you have an "Enterprise" account type with Box with single sign on
(SSO), you need to create a password to use Box with rclone. This can
be done at your Enterprise Box account by going to Settings, "Account"
Tab, and then set the password in the "Authentication" field.

Once you have done this, you can setup your Enterprise Box account
using the same procedure detailed above in the, using the password you
have just set.

### Invalid refresh token

According to the [box docs](https://developer.box.com/v2.0/docs/oauth-20#section-6-using-the-access-and-refresh-tokens):

> Each refresh_token is valid for one use in 60 days.

This means that if you

  * Don't use the box remote for 60 days
  * Copy the config file with a box refresh token in and use it in two places
  * Get an error on a token refresh

then rclone will return an error which includes the text `Invalid
refresh token`.

To fix this you will need to use oauth2 again to update the refresh
token.  You can use the methods in [the remote setup
docs](/remote_setup/), bearing in mind that if you use the copy the
config file method, you should not use that remote on the computer you
did the authentication on.

Here is how to do it.

```
$ rclone config
Current remotes:

Name                 Type
====                 ====
remote               box

e) Edit existing remote
n) New remote
d) Delete remote
r) Rename remote
c) Copy remote
s) Set configuration password
q) Quit config
e/n/d/r/c/s/q> e
Choose a number from below, or type in an existing value
 1 > remote
remote> remote
--------------------
[remote]
type = box
token = {"access_token":"XXX","token_type":"bearer","refresh_token":"XXX","expiry":"2017-07-08T23:40:08.059167677+01:00"}
--------------------
Edit remote
Value "client_id" = ""
Edit? (y/n)>
y) Yes
n) No
y/n> n
Value "client_secret" = ""
Edit? (y/n)>
y) Yes
n) No
y/n> n
Remote config
Already have a token - refresh?
y) Yes
n) No
y/n> y
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
--------------------
[remote]
type = box
token = {"access_token":"YYY","token_type":"bearer","refresh_token":"YYY","expiry":"2017-07-23T12:22:29.259137901+01:00"}
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

### Modification times and hashes

Box allows modification times to be set on objects accurate to 1
second.  These will be used to detect whether objects need syncing or
not.

Box supports SHA1 type hashes, so you can use the `--checksum`
flag.

### Restricted filename characters

In addition to the [default restricted characters set](/overview/#restricted-characters)
the following characters are also replaced:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| \         | 0x5C  | ＼           |

File names can also not end with the following characters.
These only get replaced if they are the last character in the name:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| SP        | 0x20  | ␠           |

Invalid UTF-8 bytes will also be [replaced](/overview/#invalid-utf8),
as they can't be used in JSON strings.

### Transfers

For files above 50 MiB rclone will use a chunked transfer.  Rclone will
upload up to `--transfers` chunks at the same time (shared among all
the multipart uploads).  Chunks are buffered in memory and are
normally 8 MiB so increasing `--transfers` will increase memory use.

### Deleting files

Depending on the enterprise settings for your user, the item will
either be actually deleted from Box or moved to the trash.

Emptying the trash is supported via the rclone however cleanup command
however this deletes every trashed file and folder individually so it
may take a very long time. 
Emptying the trash via the  WebUI does not have this limitation 
so it is advised to empty the trash via the WebUI.

### Root folder ID

You can set the `root_folder_id` for rclone.  This is the directory
(identified by its `Folder ID`) that rclone considers to be the root
of your Box drive.

Normally you will leave this blank and rclone will determine the
correct root to use itself.

However you can set this to restrict rclone to a specific folder
hierarchy.

In order to do this you will have to find the `Folder ID` of the
directory you wish rclone to display.  This will be the last segment
of the URL when you open the relevant folder in the Box web
interface.

So if the folder you want rclone to use has a URL which looks like
`https://app.box.com/folder/11xxxxxxxxx8`
in the browser, then you use `11xxxxxxxxx8` as
the `root_folder_id` in the config.

{{< rem autogenerated options start" - DO NOT EDIT - instead edit fs.RegInfo in backend/box/box.go then run make backenddocs" >}}
### Standard options

Here are the Standard options specific to box (Box).

#### --box-client-id

OAuth Client Id.

Leave blank normally.

Properties:

- Config:      client_id
- Env Var:     RCLONE_BOX_CLIENT_ID
- Type:        string
- Required:    false

#### --box-client-secret

OAuth Client Secret.

Leave blank normally.

Properties:

- Config:      client_secret
- Env Var:     RCLONE_BOX_CLIENT_SECRET
- Type:        string
- Required:    false

#### --box-box-config-file

Box App config.json location

Leave blank normally.

Leading `~` will be expanded in the file name as will environment variables such as `${RCLONE_CONFIG_DIR}`.

Properties:

- Config:      box_config_file
- Env Var:     RCLONE_BOX_BOX_CONFIG_FILE
- Type:        string
- Required:    false

#### --box-access-token

Box App Primary Access Token

Leave blank normally.

Properties:

- Config:      access_token
- Env Var:     RCLONE_BOX_ACCESS_TOKEN
- Type:        string
- Required:    false

#### --box-box-sub-type



Properties:

- Config:      box_sub_type
- Env Var:     RCLONE_BOX_BOX_SUB_TYPE
- Type:        string
- Default:     "user"
- Examples:
    - "user"
        - Rclone should act on behalf of a user.
    - "enterprise"
        - Rclone should act on behalf of a service account.

### Advanced options

Here are the Advanced options specific to box (Box).

#### --box-token

OAuth Access Token as a JSON blob.

Properties:

- Config:      token
- Env Var:     RCLONE_BOX_TOKEN
- Type:        string
- Required:    false

#### --box-auth-url

Auth server URL.

Leave blank to use the provider defaults.

Properties:

- Config:      auth_url
- Env Var:     RCLONE_BOX_AUTH_URL
- Type:        string
- Required:    false

#### --box-token-url

Token server url.

Leave blank to use the provider defaults.

Properties:

- Config:      token_url
- Env Var:     RCLONE_BOX_TOKEN_URL
- Type:        string
- Required:    false

#### --box-root-folder-id

Fill in for rclone to use a non root folder as its starting point.

Properties:

- Config:      root_folder_id
- Env Var:     RCLONE_BOX_ROOT_FOLDER_ID
- Type:        string
- Default:     "0"

#### --box-upload-cutoff

Cutoff for switching to multipart upload (>= 50 MiB).

Properties:

- Config:      upload_cutoff
- Env Var:     RCLONE_BOX_UPLOAD_CUTOFF
- Type:        SizeSuffix
- Default:     50Mi

#### --box-commit-retries

Max number of times to try committing a multipart file.

Properties:

- Config:      commit_retries
- Env Var:     RCLONE_BOX_COMMIT_RETRIES
- Type:        int
- Default:     100

#### --box-list-chunk

Size of listing chunk 1-1000.

Properties:

- Config:      list_chunk
- Env Var:     RCLONE_BOX_LIST_CHUNK
- Type:        int
- Default:     1000

#### --box-owned-by

Only show items owned by the login (email address) passed in.

Properties:

- Config:      owned_by
- Env Var:     RCLONE_BOX_OWNED_BY
- Type:        string
- Required:    false

#### --box-impersonate

Impersonate this user ID when using a service account.

Setting this flag allows rclone, when using a JWT service account, to
act on behalf of another user by setting the as-user header.

The user ID is the Box identifier for a user. User IDs can found for
any user via the GET /users endpoint, which is only available to
admins, or by calling the GET /users/me endpoint with an authenticated
user session.

See: https://developer.box.com/guides/authentication/jwt/as-user/


Properties:

- Config:      impersonate
- Env Var:     RCLONE_BOX_IMPERSONATE
- Type:        string
- Required:    false

#### --box-encoding

The encoding for the backend.

See the [encoding section in the overview](/overview/#encoding) for more info.

Properties:

- Config:      encoding
- Env Var:     RCLONE_BOX_ENCODING
- Type:        Encoding
- Default:     Slash,BackSlash,Del,Ctl,RightSpace,InvalidUtf8,Dot

{{< rem autogenerated options stop >}}

## Limitations

Note that Box is case insensitive so you can't have a file called
"Hello.doc" and one called "hello.doc".

Box file names can't have the `\` character in.  rclone maps this to
and from an identical looking unicode equivalent `＼` (U+FF3C Fullwidth
Reverse Solidus).

Box only supports filenames up to 255 characters in length.

Box has [API rate limits](https://developer.box.com/guides/api-calls/permissions-and-errors/rate-limits/) that sometimes reduce the speed of rclone.

`rclone about` is not supported by the Box backend. Backends without
this capability cannot determine free space for an rclone mount or
use policy `mfs` (most free space) as a member of an rclone union
remote.

See [List of backends that do not support rclone about](https://rclone.org/overview/#optional-features) and [rclone about](https://rclone.org/commands/rclone_about/)

## Get your own Box App ID

Here is how to create your own Box App ID for rclone:

1. Go to the [Box Developer Console](https://app.box.com/developers/console)
and login, then click `My Apps` on the sidebar. Click `Create New App`
and select `Custom App`.

2. In the first screen on the box that pops up, you can pretty much enter
whatever you want. The `App Name` can be whatever. For `Purpose` choose
automation to avoid having to fill out anything else. Click `Next`.

3. In the second screen of the creation screen, select
`User Authentication (OAuth 2.0)`. Then click `Create App`.

4. You should now be on the `Configuration` tab of your new app. If not,
click on it at the top of the webpage. Copy down `Client ID`
and `Client Secret`, you'll need those for rclone.

5. Under "OAuth 2.0 Redirect URI", add `http://127.0.0.1:53682/`

6. For `Application Scopes`, select `Read all files and folders stored in Box`
and `Write all files and folders stored in box` (assuming you want to do both).
Leave others unchecked. Click `Save Changes` at the top right.
