# RClone_RD

This RClone Fork contains a rudementary implementation of Real-Debrid.
Using this version, the entire RealDebrid /downloads "history" directory can be served as a read-only virtual drive. 

A potential use-case for this is serving the /downloads directory over plex, allowing you to build a media library truly unlimted in size. Im working on a project that would allow plex to function the same way that Wako,Syncler and other streaming apps do. Keep an eye out for that ;)

### Capabilities and Limitations:

- Read/Write capabilities are limited to reading files and deleting them. 
- There are no server-side traffic limitations.
- Very old downloads may not work, since realdebrid seems to deactivate the generated direct links after some (yet to be determined) amount of time. This might be fixable by catching the error and re-generate the link, look for future versions.
- There is a server-side connection limit, which I believe is 16 parallel connections.

## Installation:

I only have the means to provide a pre-built release for Windows. 

For Linux and Mac OSX, I will be providing **comunity-built** releases, which I **cannot verify** - **use these at your own risk**.

### Windows:

download the latest pre-built 'rclone.exe' file from here: https://github.com/itsToggle/rclone_RD/releases

### Mac OSX (comunity build):

download the latest pre-built 'rclone' file from here: https://github.com/itsToggle/rclone_RD/releases

### Linux (comunity build):

No comunity build has been provided yet.

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
- To build the project, you need to have MinGW or a different cgo adaptation installed.
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

