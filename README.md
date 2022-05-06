This RClone Fork contains a rudementary implementation of Real-Debrid.
Using this version, the entire RealDebrid /downloads "history" directory can be served as a read-only virtual drive. 

A potential use-case for this is serving the /downloads directory over plex, allowing you to build a media library truly unlimted(\*) in size.

- (\*) There are no server-side traffic limitations.
- (\*) Very old downloads may not work, since realdebrid seems to deactivate the generated direct links after some (yet to be determined) amount of time. This might be fixable by catching the error and re-generate the link, look for future versions.
- (\*) There is a server-side connection limit, which I believe is 16 parallel connections.
- (\*) The amount of people able to stream in parallel is obviously limited by your network connection.

Capabilities are limited to reading files and deleting them. 



**Installation:**

Download the project files. Build the project using 'go build -tags cmount'

-or-

download the latest built release

****Setting up the remote:****

The realdebrid backend is implemented by overwriting the premiumize backend (for now).

1. create a new remote using: 'rclone config'
2. choose 'premiumizeme' ('46')
3. enter your realdebrid api key (https://real-debrid.com/apitoken)
4. choose 'no advanced config'
6. your RealDebrid remote is now set up.
7. Mount the remote 'rclone cmount your-remote: your-destination:'
8. Enjoy
