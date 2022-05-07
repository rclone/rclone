# RClone_RD

This RClone Fork contains a rudementary implementation of Real-Debrid.
Using this version, the entire RealDebrid /downloads "history" directory can be served as a read-only virtual drive. 

A potential use-case for this is serving the /downloads directory over plex, allowing you to build a media library truly unlimted in size.

- There are no server-side traffic limitations.
- Very old downloads may not work, since realdebrid seems to deactivate the generated direct links after some (yet to be determined) amount of time. This might be fixable by catching the error and re-generate the link, look for future versions.
- There is a server-side connection limit, which I believe is 16 parallel connections.
- The amount of people able to stream in parallel is obviously limited by your network connection.

Capabilities are limited to reading files and deleting them. 



## Installation:

download the latest pre-built release from here: https://github.com/itsToggle/rclone_RD/releases

## Setting up the remote:

The realdebrid backend is implemented by overwriting the premiumize backend (for now).

1. create a new remote using: 'rclone config'
2. choose a name for your remote e.g. 'your-remote'
3. choose 'premiumizeme' ('46')
4. enter your realdebrid api key (https://real-debrid.com/apitoken)
5. choose 'no advanced config'
6. your RealDebrid remote is now set up.
7. Mount the remote 'rclone cmount your-remote: your-destination:' - replace 'your-remote' with your remotes name. replace 'your-destination:' with a drive letter, e.g. 'X:'
8. Enjoy!

### Recommended Tags when mounting:

1. It is recommended to use the tag '--dir-cache-time 30s' when mounting, to regulary refresh the directory.
2. It is recommended to use the tag '--vfs-cache-mode full' when mounting. Apperantly this speeds things up a bit.

an example mounting command would look like this: 'rclone cmount rdtest: X: --dir-cache-time 10s --vfs-cache-mode full'

## Building it yourself (Windows)

I really do suggest downloading the pre-built release. But if you want to tinker a bit and built it yourself, here are the steps:
- Download the project files. 
- Install Golang
- To build the project, you need to have MinGW or a different gcc adaptation installed.
- install WinFsp.
- If you dont want to mount the remote as a virtual drive but rather as a dlna server or silimar, use 'go build' to build the project.
- If you do want to mount the remote as a virtual drive, continue:
- Build the project using 'go build -tags cmount'. 
- if that fails on 'fatal error: fuse_common.h missing', you need to do the following steps:
- Locate this folder: C:\Program Files (x86)\WinFsp\inc\fuse - inside you will find the missing files.
- Copy all files to the directory that they are missing from. For me that was: C:\Users\BigSchlong\go\pkg\mod\github.com\winfsp\cgofuse@v1.5.1-0.20220421173602-ce7e5a65cac7\fuse
- Try to build it again

## Building it yourself (Mac/Linux)

I don't have the means to compile a release for Mac or Linux, so you will have to build it yourself.
- Download the project files
- Install Golang 
- Run a terminal in the root directory of the project files
- use 'go build -tags cmount' to build the project
- If anything fails, Check the official rclone Channels for Help.

