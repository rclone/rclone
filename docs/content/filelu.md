
---
title: "FileLu"
description: "Rclone docs for FileLu"
versionIntroduced: "v1.0"
---

# {{< icon "fa fa-archive" >}} FileLu

[FileLu](https://filelu.com/) is a cloud storage and file hosting service offering features such as secure file uploads, downloads, and sharing capabilities. FileLu supports high storage limits and allows seamless integration with rclone for managing files in the cloud.

Paths are specified as `remote:path`.

Paths may be as deep as required, e.g., `remote:directory/subdirectory`.

## Configuration

Here is an example of how to make a remote called `filelu`. First, run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found, make a new one?
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> filelu
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
16 / FileLu
   \ "filelu"
[snip]
Storage> filelu
Enter your FileLu Rclone Key:
Rclone Key> YOUR_FILELU_RCLONE_KEY
Configuration complete.
Options:
- type: filelu
- Rclone Key: *** ENCRYPTED ***
Keep this "filelu" remote?
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

### Example Commands

Create a new folder named `foldername`:

    rclone mkdir filelu:foldername

Delete a folder on FileLu (`368791` is an example folder_id):

    rclone rmdir filelu:368791

Delete a file on FileLu (`5fuoz8emuunp` is a sample file_code):

    rclone delete filelu:5fuoz8emuunp

List files from your FileLu account:

    rclone ls filelu:

List all folders:

    rclone lsd filelu:

Copy a specific file to the FileLu root:

    rclone copy D:\\hello.txt filelu:

Copy files from a local directory to a FileLu directory (directory id `366238`):

    rclone copy D:/local-folder filelu:366238

Download a file from FileLu into a local directory:

    rclone copy filelu:5fuoz8emuunp D:/local-folder

Move files from a local directory to a FileLu directory:

    rclone move D:\\local-folder filelu:366238

Sync files from a local directory to a FileLu directory (directory id `366238`):

    rclone sync D:/local-folder filelu:366238
    
Mount remote to local Linux:

rclone mount filelu: /root/mnt --vfs-cache-mode writes

Mount remote to local Windows:

rclone mount filelu: D:/local_mnt --vfs-cache-mode writes


Get storage info about the FileLu account:

    rclone about filelu:

### Modification Times and Hashes

FileLu supports modification times but does not currently support hashes.

### Restricted Filename Characters

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| NUL       | 0x00  | ␀           |
| /         | 0x2F  | ／          |

Invalid UTF-8 bytes will also be [replaced](/overview/#invalid-utf8), as they cannot be used in JSON strings.

### Duplicated Files

FileLu can have two files with exactly the same name and path (unlike a normal file system).

Duplicated files cause problems with syncing, and you will see messages in the log about duplicates.

Use `rclone dedupe` to fix duplicated files.

### Failure to Log In

#### Invalid Credentials

If you are connecting to your FileLu remote for the first time and receive an error such as:

```
Failed to create file system for "my-filelu-remote:": couldn't login: Invalid credentials
```

Ensure your Rclone Key is correct.

### Process `killed`

Accounts with large files or extensive metadata may experience significant memory usage during list/sync operations. Ensure the system running `rclone` has sufficient memory and CPU to handle these operations.

## Limitations

This backend uses a custom library implementing the FileLu API. While it supports file transfers, some advanced features may not yet be available. Please report any issues to the [rclone forum](https://forum.rclone.org/) for troubleshooting and updates.

### Standard Options

Here are the standard options specific to FileLu:

#### --filelu-user

User name. 

- **Note:** This is not required if using an Rclone Key.

#### --filelu-pass

Password.

- **NB:** Input to this must be obscured - see [rclone obscure](/commands/rclone_obscure/).

#### --filelu-debug

Output more debug information.

Properties:

- Config:      debug
- Env Var:     RCLONE_FILELU_DEBUG
- Type:        bool
- Default:     false

#### --filelu-use-https

Use HTTPS for transfers.

Properties:

- Config:      use_https
- Env Var:     RCLONE_FILELU_USE_HTTPS
- Type:        bool
- Default:     true

---

For further information, visit [FileLu's website](https://filelu.com/).
"""

# Write the content to a markdown file
with open("FileLu_rclone_docs.md", "w") as file:
    file.write(content)
