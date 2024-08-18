---
title: "Seafile"
description: "Seafile"
versionIntroduced: "v1.52"
---

# {{< icon "fa fa-server" >}} Seafile

This is a backend for the [Seafile](https://www.seafile.com/) storage service:
- It works with both the free community edition or the professional edition.
- Seafile versions 6.x, 7.x, 8.x and 9.x are all supported.
- Encrypted libraries are also supported.
- It supports 2FA enabled users
- Using a Library API Token is **not** supported

## Configuration

There are two distinct modes you can setup your remote:
- you point your remote to the **root of the server**, meaning you don't specify a library during the configuration:
Paths are specified as `remote:library`. You may put subdirectories in too, e.g. `remote:library/path/to/dir`.
- you point your remote to a specific library during the configuration:
Paths are specified as `remote:path/to/dir`. **This is the recommended mode when using encrypted libraries**. (_This mode is possibly slightly faster than the root mode_)

### Configuration in root mode

Here is an example of making a seafile configuration for a user with **no** two-factor authentication.  First run

    rclone config

This will guide you through an interactive setup process. To authenticate
you will need the URL of your server, your email (or username) and your password.

```
No remotes found, make a new one?
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> seafile
Type of storage to configure.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
[snip]
XX / Seafile
   \ "seafile"
[snip]
Storage> seafile
** See help for seafile backend at: https://rclone.org/seafile/ **

URL of seafile host to connect to
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
 1 / Connect to cloud.seafile.com
   \ "https://cloud.seafile.com/"
url> http://my.seafile.server/
User name (usually email address)
Enter a string value. Press Enter for the default ("").
user> me@example.com
Password
y) Yes type in my own password
g) Generate random password
n) No leave this optional password blank (default)
y/g> y
Enter the password:
password:
Confirm the password:
password:
Two-factor authentication ('true' if the account has 2FA enabled)
Enter a boolean value (true or false). Press Enter for the default ("false").
2fa> false
Name of the library. Leave blank to access all non-encrypted libraries.
Enter a string value. Press Enter for the default ("").
library>
Library password (for encrypted libraries only). Leave blank if you pass it through the command line.
y) Yes type in my own password
g) Generate random password
n) No leave this optional password blank (default)
y/g/n> n
Edit advanced config? (y/n)
y) Yes
n) No (default)
y/n> n
Remote config
Two-factor authentication is not enabled on this account.
--------------------
[seafile]
type = seafile
url = http://my.seafile.server/
user = me@example.com
pass = *** ENCRYPTED ***
2fa = false
--------------------
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d> y
```

This remote is called `seafile`. It's pointing to the root of your seafile server and can now be used like this:

See all libraries

    rclone lsd seafile:

Create a new library

    rclone mkdir seafile:library

List the contents of a library

    rclone ls seafile:library

Sync `/home/local/directory` to the remote library, deleting any
excess files in the library.

    rclone sync --interactive /home/local/directory seafile:library

### Configuration in library mode

Here's an example of a configuration in library mode with a user that has the two-factor authentication enabled. Your 2FA code will be asked at the end of the configuration, and will attempt to authenticate you:

```
No remotes found, make a new one?
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> seafile
Type of storage to configure.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
[snip]
XX / Seafile
   \ "seafile"
[snip]
Storage> seafile
** See help for seafile backend at: https://rclone.org/seafile/ **

URL of seafile host to connect to
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
 1 / Connect to cloud.seafile.com
   \ "https://cloud.seafile.com/"
url> http://my.seafile.server/
User name (usually email address)
Enter a string value. Press Enter for the default ("").
user> me@example.com
Password
y) Yes type in my own password
g) Generate random password
n) No leave this optional password blank (default)
y/g> y
Enter the password:
password:
Confirm the password:
password:
Two-factor authentication ('true' if the account has 2FA enabled)
Enter a boolean value (true or false). Press Enter for the default ("false").
2fa> true
Name of the library. Leave blank to access all non-encrypted libraries.
Enter a string value. Press Enter for the default ("").
library> My Library
Library password (for encrypted libraries only). Leave blank if you pass it through the command line.
y) Yes type in my own password
g) Generate random password
n) No leave this optional password blank (default)
y/g/n> n
Edit advanced config? (y/n)
y) Yes
n) No (default)
y/n> n
Remote config
Two-factor authentication: please enter your 2FA code
2fa code> 123456
Authenticating...
Success!
--------------------
[seafile]
type = seafile
url = http://my.seafile.server/
user = me@example.com
pass = 
2fa = true
library = My Library
--------------------
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d> y
```

You'll notice your password is blank in the configuration. It's because we only need the password to authenticate you once.

You specified `My Library` during the configuration. The root of the remote is pointing at the
root of the library `My Library`:

See all files in the library:

    rclone lsd seafile:

Create a new directory inside the library

    rclone mkdir seafile:directory

List the contents of a directory

    rclone ls seafile:directory

Sync `/home/local/directory` to the remote library, deleting any
excess files in the library.

    rclone sync --interactive /home/local/directory seafile:


### --fast-list

Seafile version 7+ supports `--fast-list` which allows you to use fewer
transactions in exchange for more memory. See the [rclone
docs](/docs/#fast-list) for more details.
Please note this is not supported on seafile server version 6.x


### Restricted filename characters

In addition to the [default restricted characters set](/overview/#restricted-characters)
the following characters are also replaced:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| /         | 0x2F  | ／          |
| "         | 0x22  | ＂          |
| \         | 0x5C  | ＼           |

Invalid UTF-8 bytes will also be [replaced](/overview/#invalid-utf8),
as they can't be used in JSON strings.

### Seafile and rclone link

Rclone supports generating share links for non-encrypted libraries only.
They can either be for a file or a directory:

```
rclone link seafile:seafile-tutorial.doc
http://my.seafile.server/f/fdcd8a2f93f84b8b90f4/

```

or if run on a directory you will get:

```
rclone link seafile:dir
http://my.seafile.server/d/9ea2455f6f55478bbb0d/
```

Please note a share link is unique for each file or directory. If you run a link command on a file/dir
that has already been shared, you will get the exact same link.

### Compatibility

It has been actively developed using the [seafile docker image](https://github.com/haiwen/seafile-docker) of these versions:
- 6.3.4 community edition
- 7.0.5 community edition
- 7.1.3 community edition
- 9.0.10 community edition

Versions below 6.0 are not supported.
Versions between 6.0 and 6.3 haven't been tested and might not work properly.

Each new version of `rclone` is automatically tested against the [latest docker image](https://hub.docker.com/r/seafileltd/seafile-mc/) of the seafile community server.

{{< rem autogenerated options start" - DO NOT EDIT - instead edit fs.RegInfo in backend/seafile/seafile.go then run make backenddocs" >}}
### Standard options

Here are the Standard options specific to seafile (seafile).

#### --seafile-url

URL of seafile host to connect to.

Properties:

- Config:      url
- Env Var:     RCLONE_SEAFILE_URL
- Type:        string
- Required:    true
- Examples:
    - "https://cloud.seafile.com/"
        - Connect to cloud.seafile.com.

#### --seafile-user

User name (usually email address).

Properties:

- Config:      user
- Env Var:     RCLONE_SEAFILE_USER
- Type:        string
- Required:    true

#### --seafile-pass

Password.

**NB** Input to this must be obscured - see [rclone obscure](/commands/rclone_obscure/).

Properties:

- Config:      pass
- Env Var:     RCLONE_SEAFILE_PASS
- Type:        string
- Required:    false

#### --seafile-2fa

Two-factor authentication ('true' if the account has 2FA enabled).

Properties:

- Config:      2fa
- Env Var:     RCLONE_SEAFILE_2FA
- Type:        bool
- Default:     false

#### --seafile-library

Name of the library.

Leave blank to access all non-encrypted libraries.

Properties:

- Config:      library
- Env Var:     RCLONE_SEAFILE_LIBRARY
- Type:        string
- Required:    false

#### --seafile-library-key

Library password (for encrypted libraries only).

Leave blank if you pass it through the command line.

**NB** Input to this must be obscured - see [rclone obscure](/commands/rclone_obscure/).

Properties:

- Config:      library_key
- Env Var:     RCLONE_SEAFILE_LIBRARY_KEY
- Type:        string
- Required:    false

#### --seafile-auth-token

Authentication token.

Properties:

- Config:      auth_token
- Env Var:     RCLONE_SEAFILE_AUTH_TOKEN
- Type:        string
- Required:    false

### Advanced options

Here are the Advanced options specific to seafile (seafile).

#### --seafile-create-library

Should rclone create a library if it doesn't exist.

Properties:

- Config:      create_library
- Env Var:     RCLONE_SEAFILE_CREATE_LIBRARY
- Type:        bool
- Default:     false

#### --seafile-encoding

The encoding for the backend.

See the [encoding section in the overview](/overview/#encoding) for more info.

Properties:

- Config:      encoding
- Env Var:     RCLONE_SEAFILE_ENCODING
- Type:        Encoding
- Default:     Slash,DoubleQuote,BackSlash,Ctl,InvalidUtf8

#### --seafile-description

Description of the remote.

Properties:

- Config:      description
- Env Var:     RCLONE_SEAFILE_DESCRIPTION
- Type:        string
- Required:    false

{{< rem autogenerated options stop >}}

