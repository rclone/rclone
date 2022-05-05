This RClone Fork contains a rudementary implementation of Real-Debrid.
Using this version, The RealDebrid /downloads "history" directory can be served as a read-only DLNA server. 

A potential use-case for this is serving the /downloads directory over plex, allowing you to build a media library truly unlimted(\*) in size.

- (\*) There are no server-side traffic limitations.
- (\*) At the moment only the first page of the download history is served.
- (\*) Very old downloads may not work, since realdebrid seems to deactivate the generated direct links after some (yet to be determined) amount of time.

Installation:

The realdebrid backend is implemented by overwriting the premiumize backend (for now):

1. Download the project files.
2. Enter your Realdebrid API Key (https://real-debrid.com/apitoken) into the 'Default' Field in '/backend/premiumizeme/premiumizeme.go' : Line 92.
3. (Build the project)
4. create a new remote using: 'rclone config'
5. choose 'premiumizeme' ('46')
6. chose 'no advanced config'
7. choose 'remote setup' (this is done to circumvent any actual oauth calls)
8. enter a random string, does not matter
9. your RealDebrid remote is now set up.
10. Launch the DLNA server: 'rclone serve dlna your-remote:'
11. Enjoy
