---
title: "Seafile"
description: "Seafile"
date: "2020-05-02"
---

<i class="fa fa-server"></i>Seafile
----------------------------------------

This is a backend for the [Seafile](https://www.seafile.com/) storage service.
It works with both the free community edition, or the professional edition.
Seafile versions 6.x and 7.x are all supported.
Encrypted libraries are also supported.

### Root mode vs Library mode ###

There are two distinct modes you can setup your remote:
- you point your remote to the **root of the server**, meaning you don't specify a library during the configuration:
Paths are specified as `remote:library`. You may put subdirectories in too, eg `remote:library/path/to/dir`.
- you point your remote to a specific library during the configuration:
Paths are specified as `remote:path/to/dir`. **This is the recommended mode when using encrypted libraries**.

### Configuration in root mode ###

Here is an example of making a seafile configuration.  First run

    rclone config

This will guide you through an interactive setup process. To authenticate
you will need the URL of your server, your email (or username) and your password.

```
No remotes found - make a new one
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
User name
Enter a string value. Press Enter for the default ("").
user> me@example.com
Password
y) Yes type in my own password
g) Generate random password
y/g> y
Enter the password:
password:
Confirm the password:
password:
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
--------------------
[seafile]
type = seafile
url = http://my.seafile.server/
user = me@example.com
password = *** ENCRYPTED ***
--------------------
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d> y
```

This remote is called `seafile`. It's pointing to the root of your seafile server and can now be used like this

See all libraries

    rclone lsd seafile:

Create a new library

    rclone mkdir seafile:library

List the contents of a library

    rclone ls seafile:library

Sync `/home/local/directory` to the remote library, deleting any
excess files in the library.

    rclone sync /home/local/directory seafile:library

### Configuration in library mode ###

```
No remotes found - make a new one
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
User name
Enter a string value. Press Enter for the default ("").
user> me@example.com
Password
y) Yes type in my own password
g) Generate random password
y/g> y
Enter the password:
password:
Confirm the password:
password:
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
--------------------
[seafile]
type = seafile
url = http://my.seafile.server/
user = me@example.com
password = *** ENCRYPTED ***
library = My Library
--------------------
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d> y
```

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

    rclone sync /home/local/directory seafile:


### --fast-list ###

Seafile version 7+ supports `--fast-list` which allows you to use fewer
transactions in exchange for more memory. See the [rclone
docs](/docs/#fast-list) for more details.
Please note this is not supported on seafile server version 6.x


#### Restricted filename characters

In addition to the [default restricted characters set](/overview/#restricted-characters)
the following characters are also replaced:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| /         | 0x2F  | ／          |
| "         | 0x22  | ＂          |
| \         | 0x5C  | ＼           |

Invalid UTF-8 bytes will also be [replaced](/overview/#invalid-utf8),
as they can't be used in JSON strings.

### Seafile and rclone link ###

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

### Compatibility ###

It has been actively tested using the [seafile docker image](https://github.com/haiwen/seafile-docker) of these versions:
- 6.3.4 community edition
- 7.0.5 community edition
- 7.1.3 community edition

Versions below 6.0 are not supported.
Versions between 6.0 and 6.3 haven't been tested and might not work properly.

<!--- autogenerated options start - DO NOT EDIT, instead edit fs.RegInfo in backend/seafile/seafile.go then run make backenddocs -->

<!--- autogenerated options stop -->

