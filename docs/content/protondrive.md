---
title: "Proton Drive"
description: "Rclone docs for Proton Drive"
versionIntroduced: "v1.64.0"
---

# {{< icon "fa fa-atom-simple" >}} Proton Drive

[Proton Drive](https://proton.me/drive) is an end-to-end encrypted Swiss vault
 for your files that protects your data.

This is a rclone backend for Proton Drive which supports the file transfer
features of Proton Drive using the same client-side encryption.

Due to the fact that Proton Drive doesn't publish its API documentation, this 
backend is implemented with the best effort by reading the open-sourced client 
source code and observing the Proton Drive traffic on the browser.

Paths are specified as `remote:path`

Paths may be as deep as required, e.g. `remote:directory/subdirectory`.

## Configurations

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
XX / Proton Drive
   \ "Proton Drive"
[snip]
Storage> protondrive
User name
user> you@protonmail.com
Password.
y) Yes type in my own password
g) Generate random password
n) No leave this optional password blank
y/g/n> y
Enter the password:
password:
Confirm the password:
password:
Option 2fa.
2FA code (if the account requires one)
Enter a value. Press Enter to leave empty.
2fa> 123456
Remote config
--------------------
[remote]
type = protondrive
user = you@protonmail.com
pass = *** ENCRYPTED ***
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

**NOTE:** The Proton Drive encryption keys need to have been already generated 
after a regular login via the browser, otherwise attempting to use the 
credentials in `rclone` will fail.

Once configured you can then use `rclone` like this,

List directories in top level of your Proton Drive

    rclone lsd remote:

List all the files in your Proton Drive

    rclone ls remote:

To copy a local directory to an Proton Drive directory called backup

    rclone copy /home/source remote:backup

### Modified time

Proton Drive Bridge does not support modification times yet.

### Restricted filename characters

Invalid UTF-8 bytes will be [replaced](/overview/#invalid-utf8), also left and 
right spaces will be removed ([code reference](https://github.com/ProtonMail/WebClients/blob/b4eba99d241af4fdae06ff7138bd651a40ef5d3c/applications/drive/src/app/store/_links/validation.ts#L51))

### Duplicated files

Proton Drive can not have two files with exactly the same name and path. If the 
conflict occurs, depending on the advanced config, the file might or might not 
be overwritten.

### Caching

The cache is currently built for the case when the rclone is the only instance 
performing operations to the mount point. The event system, which is the proton
API system that provides visibility of what has changed on the drive, is yet 
to be implemented, so updates from other clients won’t be reflected in the 
cache. Thus, if there are concurrent clients accessing the same mount point, 
then we might have a problem with caching the stale data.

## Limitations

This backend uses the 
[Proton-API-Bridge](https://github.com/henrybear327/Proton-API-Bridge), which 
is based on [go-proton-api](https://github.com/henrybear327/go-proton-api), a 
fork of the [official repo](https://github.com/ProtonMail/go-proton-api).

There is no official API documentation available from Proton Drive. But, thanks 
to Proton open sourcing [proton-go-api](https://github.com/ProtonMail/go-proton-api) 
and the web, iOS, and Android client codebases, we don't need to completely 
reverse engineer the APIs by observing the web client traffic! 

[proton-go-api](https://github.com/ProtonMail/go-proton-api) provides the basic 
building blocks of API calls and error handling, such as 429 exponential 
back-off, but it is pretty much just a barebone interface to the Proton API. 
For example, the encryption and decryption of the Proton Drive file are not 
provided in this library. 

The Proton-API-Bridge, attempts to bridge the gap, so rclone can be built on 
top of this quickly. This codebase handles the intricate tasks before and after 
calling Proton APIs, particularly the complex encryption scheme, allowing 
developers to implement features for other software on top of this codebase. 
There are likely quite a few errors in this library, as there isn't official 
documentation available. 