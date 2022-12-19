---
title: "Estuary"
description: "A user-friendly API to upload data to IPFS and Filecoin."
---

# {{< icon "fa-light fa-triangle" >}} Estuary

[Estuary](https://estuary.tech) is a user-friendly way API to onboard data 
to IPFS and Filecoin, and have it stored in a decentralized manner.

## Backend options

Estuary is a decentralized data storage service built on key d-web protocols such as IPFS and Filecoin.
It is relatively easy to set up a local estuary node following these [instructions](https://github.com/application-research/estuary/blob/dev/README.md).
Alternatively, estuary also provides a hosted service in the form of [estuary.tech](https://estuary.tech).



## Configuration

Here is an example of setting up estuary config.  First run

    rclone config

This will guide you through an interactive setup process.  To authenticate
you will either need an API Key. An API key may be generated either via the UI or using this [endpoint](https://docs.estuary.tech/user-key-add).

```
No remotes found, make a new one?
n) New remote
q) Quit config
n/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / Estuary based Filecoin/IPFS storage
   \ "estuary"
[snip]
Storage> estuary
Estuary API token
Enter a value.
token> EST-XXXX-ARY
Option url.
Estuary URL
Enter a string value. Press Enter for the default (https://api.estuary.tech)
url> 
Configuration complete.
Options:
- type: estuary
- token: EST-XXXX-ARY
Keep this "remote" remote?
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

This remote is called `remote` and can now be used like this

Make a directory if it doesn't already exist
    
    rclone mkdir remote:path

List all directories in the remote:path

    rclone lsd remote:path

List all directories in the remote:path

    rclone ls remote:path

Copy files from source to dest

    rclone copy source:path dest:path

Sync `/home/local/directory` to remote, deleting any
excess files.

    rclone sync -i /home/local/directory remote:path

### Modified time

The modified time is stored as metadata on the object as
`modTime` as milliseconds since 1970-01-01.  
Other tools should be able to use this as a modified time.

Modified times are used in syncing and are fully supported. Note that
if a modification time needs to be updated on an object then it will
create a new version of the object.



