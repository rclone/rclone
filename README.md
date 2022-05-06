This RClone Fork contains a rudementary implementation of Real-Debrid.
Using this version, the entire RealDebrid /downloads "history" directory can be served as a read-only virtual drive. 

A potential use-case for this is serving the /downloads directory over plex, allowing you to build a media library truly unlimted(\*) in size.

- (\*) There are no server-side traffic limitations.
- (\*) Very old downloads may not work, since realdebrid seems to deactivate the generated direct links after some (yet to be determined) amount of time. This might be fixable by catching the error and re-generate the link, look for future versions.
- (\*) There is a server-side connection limit, which I believe is 16 parallel connections.
- (\*) The amount of people able to stream in parallel is obviously limited by your network connection.

Capabilities are limited to reading files and deleting them. 



**Installation:**

download the latest pre-built release from here: https://github.com/itsToggle/rclone_RD/releases/tag/v1.58.1-rd.1

**Setting up the remote:**

The realdebrid backend is implemented by overwriting the premiumize backend (for now).

1. create a new remote using: 'rclone config'
2. choose 'premiumizeme' ('46')
3. enter your realdebrid api key (https://real-debrid.com/apitoken)
4. choose 'no advanced config'
6. your RealDebrid remote is now set up.
7. Mount the remote 'rclone cmount your-remote: your-destination:'
8. Enjoy

**Building it yourself**

I really do suggest downloading the pre-built release. But if you want to tinker a bit and built it yourself, here are the steps:
- Download the project files. 
- To build the project, you need to have MinGW or a different gcc adaptation installed.
- install WinFsp.
- If you dont want to mount the remote as a virtual drive but rather as a dlna server or silimar, use 'go build' to build the project.
- If you do want to mount the remote as a virtual drive, continue:
- Build the project using 'go build -tags cmount'. 
- if that fails on 'fatal error: fuse_common.h missing', you need to do the following steps:
- Locate this folder: C:\Program Files (x86)\WinFsp\inc\fuse - inside you will find the missing files.
- Copy all files to the directory that they are missing from. For me that was: C:\Users\BigSchlong\go\pkg\mod\github.com\winfsp\cgofuse@v1.5.1-0.20220421173602-ce7e5a65cac7\fuse
- Try to build it again

