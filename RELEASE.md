Extra required software for making a release
  * [github-release](https://github.com/aktau/github-release) for uploading packages
  * pandoc for making the html and man pages

Making a release
  * git status - make sure everything is checked in
  * Check travis & appveyor builds are green
  * make check
  * make test # see integration test server or run locally
  * make tag
  * edit docs/content/changelog.md
  * make doc
  * git status - to check for new man pages - git add them
  * git commit -a -v -m "Version v1.XX"
  * make retag
  * git push --tags origin master
  * # Wait for the appveyor and travis builds to complete then...
  * make fetch_binaries
  * make tarball
  * make sign_upload
  * make check_sign
  * make upload
  * make upload_website
  * make upload_github
  * make startdev
  * # announce with forum post, twitter post, G+ post

Early in the next release cycle update the vendored dependencies
  * Review any pinned packages in go.mod and remove if possible
  * make update
  * git status
  * git add new files
  * git commit -a -v
