---
title: "Akamai Netstorage"
description: "Rclone docs for Akamai NetStorage"
versionIntroduced: "v1.58"
---

# {{< icon "fas fa-database" >}} Akamai NetStorage

Paths are specified as `remote:`
You may put subdirectories in too, e.g. `remote:/path/to/dir`.
If you have a CP code you can use that as the folder after the domain such as \<domain>\/\<cpcode>\/\<internal directories within cpcode>.

For example, this is commonly configured with or without a CP code:
* **With a CP code**. `[your-domain-prefix]-nsu.akamaihd.net/123456/subdirectory/`
* **Without a CP code**. `[your-domain-prefix]-nsu.akamaihd.net`


See all buckets
   rclone lsd remote:
The initial setup for Netstorage involves getting an account and secret. Use `rclone config` to walk you through the setup process.

## Configuration

Here's an example of how to make a remote called `ns1`.

1. To begin the interactive configuration process, enter this command:

```
rclone config
```

2. Type `n` to create a new remote.

```
n) New remote
d) Delete remote
q) Quit config
e/n/d/q> n
```

3. For this example, enter `ns1` when you reach the name> prompt.

```
name> ns1
```

4. Enter `netstorage` as the type of storage to configure.

```
Type of storage to configure.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
XX / NetStorage
   \ "netstorage"
Storage> netstorage
```

5. Select between the HTTP or HTTPS protocol. Most users should choose HTTPS, which is the default. HTTP is provided primarily for debugging purposes.


```
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
 1 / HTTP protocol
   \ "http"
 2 / HTTPS protocol
   \ "https"
protocol> 1
```

6. Specify your NetStorage host, CP code, and any necessary content paths using this format: `<domain>/<cpcode>/<content>/`

```
Enter a string value. Press Enter for the default ("").
host> baseball-nsu.akamaihd.net/123456/content/
```

7. Set the netstorage account name
```
Enter a string value. Press Enter for the default ("").
account> username
```

8. Set the Netstorage account secret/G2O key which will be used for authentication purposes. Select the `y` option to set your own password then enter your secret.
Note: The secret is stored in the `rclone.conf` file with hex-encoded encryption.

```
y) Yes type in my own password
g) Generate random password
y/g> y
Enter the password:
password:
Confirm the password:
password:
```

9. View the summary and confirm your remote configuration.

```
[ns1]
type = netstorage
protocol = http
host = baseball-nsu.akamaihd.net/123456/content/
account = username
secret = *** ENCRYPTED ***
--------------------
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d> y
```

This remote is called `ns1` and can now be used.

## Example operations

Get started with rclone and NetStorage with these examples. For additional rclone commands, visit https://rclone.org/commands/.

### See contents of a directory in your project

    rclone lsd ns1:/974012/testing/

### Sync the contents local with remote

    rclone sync . ns1:/974012/testing/

### Upload local content to remote
    rclone copy notes.txt ns1:/974012/testing/

### Delete content on remote
    rclone delete ns1:/974012/testing/notes.txt

### Move or copy content between CP codes.

Your credentials must have access to two CP codes on the same remote. You can't perform operations between different remotes.

    rclone move ns1:/974012/testing/notes.txt ns1:/974450/testing2/

## Features

### Symlink Support

The Netstorage backend changes the rclone `--links, -l` behavior. When uploading, instead of creating the .rclonelink file, use the "symlink" API in order to create the corresponding symlink on the remote. The .rclonelink file will not be created, the upload will be intercepted and only the symlink file that matches the source file name with no suffix will be created on the remote.

This will effectively allow commands like copy/copyto, move/moveto and sync to upload from local to remote and download from remote to local directories with symlinks. Due to internal rclone limitations, it is not possible to upload an individual symlink file to any remote backend. You can always use the "backend symlink" command to create a symlink on the NetStorage server, refer to "symlink" section below.

Individual symlink files on the remote can be used with the commands like "cat" to print the destination name, or "delete" to delete symlink, or copy, copy/to and move/moveto to download from the remote to local. Note: individual symlink files on the remote should be specified including the suffix .rclonelink.

**Note**: No file with the suffix .rclonelink should ever exist on the server since it is not possible to actually upload/create a file with .rclonelink suffix with rclone, it can only exist if it is manually created through a non-rclone method on the remote.

### Implicit vs. Explicit Directories

With NetStorage, directories can exist in one of two forms:

1. **Explicit Directory**. This is an actual, physical directory that you have created in a storage group.
2. **Implicit Directory**. This refers to a directory within a path that has not been physically created. For example, during upload of a file, nonexistent subdirectories can be specified in the target path. NetStorage creates these as "implicit." While the directories aren't physically created, they exist implicitly and the noted path is connected with the uploaded file.

Rclone will intercept all file uploads and mkdir commands for the NetStorage remote and will explicitly issue the mkdir command for each directory in the uploading path. This will help with the interoperability with the other Akamai services such as SFTP and the Content Management Shell (CMShell). Rclone will not guarantee correctness of operations with implicit directories which might have been created as a result of using an upload API directly.

### `--fast-list` / ListR support

NetStorage remote supports the ListR feature by using the "list" NetStorage API action to return a lexicographical list of all objects within the specified CP code, recursing into subdirectories as they're encountered.

* **Rclone will use the ListR method for some commands by default**. Commands such as `lsf -R` will use ListR by default. To disable this, include the `--disable listR` option to use the non-recursive method of listing objects.

* **Rclone will not use the ListR method for some commands**. Commands such as `sync` don't use ListR by default. To force using the ListR method, include the  `--fast-list` option.

There are pros and cons of using the ListR method, refer to [rclone documentation](https://rclone.org/docs/#fast-list). In general, the sync command over an existing deep tree on the remote will run faster with the "--fast-list" flag but with extra memory usage as a side effect. It might also result in higher CPU utilization but the whole task can be completed faster.

**Note**: There is a known limitation that "lsf -R" will display number of files in the directory and directory size as -1 when ListR method is used. The workaround is to pass "--disable listR" flag if these numbers are important in the output.

### Purge

NetStorage remote supports the purge feature by using the "quick-delete" NetStorage API action. The quick-delete action is disabled by default for security reasons and can be enabled for the account through the Akamai portal. Rclone will first try to use quick-delete action for the purge command and if this functionality is disabled then will fall back to a standard delete method.

**Note**: Read the [NetStorage Usage API](https://learn.akamai.com/en-us/webhelp/netstorage/netstorage-http-api-developer-guide/GUID-15836617-9F50-405A-833C-EA2556756A30.html) for considerations when using "quick-delete". In general, using quick-delete method will not delete the tree immediately and objects targeted for quick-delete may still be accessible.

{{< rem autogenerated options start" - DO NOT EDIT - instead edit fs.RegInfo in backend/netstorage/netstorage.go then run make backenddocs" >}}
### Standard options

Here are the Standard options specific to netstorage (Akamai NetStorage).

#### --netstorage-host

Domain+path of NetStorage host to connect to.

Format should be `<domain>/<internal folders>`

Properties:

- Config:      host
- Env Var:     RCLONE_NETSTORAGE_HOST
- Type:        string
- Required:    true

#### --netstorage-account

Set the NetStorage account name

Properties:

- Config:      account
- Env Var:     RCLONE_NETSTORAGE_ACCOUNT
- Type:        string
- Required:    true

#### --netstorage-secret

Set the NetStorage account secret/G2O key for authentication.

Please choose the 'y' option to set your own password then enter your secret.

**NB** Input to this must be obscured - see [rclone obscure](/commands/rclone_obscure/).

Properties:

- Config:      secret
- Env Var:     RCLONE_NETSTORAGE_SECRET
- Type:        string
- Required:    true

### Advanced options

Here are the Advanced options specific to netstorage (Akamai NetStorage).

#### --netstorage-protocol

Select between HTTP or HTTPS protocol.

Most users should choose HTTPS, which is the default.
HTTP is provided primarily for debugging purposes.

Properties:

- Config:      protocol
- Env Var:     RCLONE_NETSTORAGE_PROTOCOL
- Type:        string
- Default:     "https"
- Examples:
    - "http"
        - HTTP protocol
    - "https"
        - HTTPS protocol

#### --netstorage-description

Description of the remote.

Properties:

- Config:      description
- Env Var:     RCLONE_NETSTORAGE_DESCRIPTION
- Type:        string
- Required:    false

## Backend commands

Here are the commands specific to the netstorage backend.

Run them with

    rclone backend COMMAND remote:

The help below will explain what arguments each command takes.

See the [backend](/commands/rclone_backend/) command for more
info on how to pass options and arguments.

These can be run on a running backend using the rc command
[backend/command](/rc/#backend-command).

### du

Return disk usage information for a specified directory

    rclone backend du remote: [options] [<arguments>+]

The usage information returned, includes the targeted directory as well as all
files stored in any sub-directories that may exist.

### symlink

You can create a symbolic link in ObjectStore with the symlink action.

    rclone backend symlink remote: [options] [<arguments>+]

The desired path location (including applicable sub-directories) ending in
the object that will be the target of the symlink (for example, /links/mylink).
Include the file extension for the object, if applicable.
`rclone backend symlink <src> <path>`

{{< rem autogenerated options stop >}}
