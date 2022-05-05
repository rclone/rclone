This RClone Fork contains a rudementary implementation of Real-Debrid.
Using this version, The RealDebrid /downloads "history" directory can be served as a read-only virtual drive. 

A potential use-case for this is serving the /downloads directory over plex, allowing you to build a media library truly unlimted(\*) in size. Since all traffic will be routed through the host running rclone, an unlimited(\*\*) amount of people in different locations can access the files.

- (\*) There are no server-side traffic limitations.
- (\*) At the moment only the first page of the download history is served.
- (\*) Very old downloads may not work, since realdebrid seems to deactivate the generated direct links after some (yet to be determined) amount of time.

- (\*\*) There are no server-side traffic limitations.
- (\*\*) There is however a server-side connection limit, which I believe is 16 parallel connections.
- (\*\*) The amount of people able to stream in parallel is obviously limited by your network connection.

Installation:

The realdebrid backend is implemented by overwriting the premiumize backend (for now):

1. Download the project files.
2. Enter your Realdebrid API Key (https://real-debrid.com/apitoken) into the 'Default' Field in '/backend/premiumizeme/premiumizeme.go' : Line 92.
3. Build the project using 'go build -tags cmount' (for now, I will release a prebuilt version soon)
4. create a new remote using: 'rclone config'
5. choose 'premiumizeme' ('46')
6. chose 'no advanced config'
7. choose 'remote setup' (this is done to circumvent any actual oauth calls)
8. enter a random string, does not matter
9. your RealDebrid remote is now set up.
10. Mount the remote 'rclone cmount your-remote: your-destination:'
11. Enjoy
