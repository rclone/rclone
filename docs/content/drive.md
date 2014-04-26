---
title: "Google drive"
description: "Rclone docs for Google drive"
date: "2014-04-26"
---

Paths are specified as `drive:path`

Drive paths may be as deep as required, eg
`drive:directory/subdirectory`.

The initial setup for drive involves getting a token from Google drive
which you need to do in your browser.  `rclone config` walks you
through it.

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
n) New remote
d) Delete remote
q) Quit config
e/n/d/q> n
name> remote
What type of source is it?
Choose a number from below
 1) swift
 2) s3
 3) local
 4) drive
type> 4
Google Application Client Id - leave blank to use rclone's.
client_id> 
Google Application Client Secret - leave blank to use rclone's.
client_secret> 
Remote config
Go to the following link in your browser
https://accounts.google.com/o/oauth2/auth?access_type=&approval_prompt=&client_id=XXXXXXXXXXXX.apps.googleusercontent.com&redirect_uri=urn%3XXXXX%3Awg%3Aoauth%3XX.0%3Aoob&response_type=code&scope=https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fdrive&state=state
Log in, then type paste the token that is returned in the browser here
Enter verification code> X/XXXXXXXXXXXXXXXXXX-XXXXXXXXX.XXXXXXXXX-XXXXX_XXXXXXX_XXXXXXX
--------------------
[remote]
client_id = 
client_secret = 
token = {"AccessToken":"xxxx.x.xxxxx_xxxxxxxxxxx_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx","RefreshToken":"1/xxxxxxxxxxxxxxxx_xxxxxxxxxxxxxxxxxxxxxxxxxx","Expiry":"2014-03-16T13:57:58.955387075Z","Extra":null}
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

You can then use it like this,

List directories in top level of your drive

    rclone lsd remote:

List all the files in your drive

    rclone ls remote:

To copy a local directory to a drive directory called backup

    rclone copy /home/source remote:backup

Modified time
-------------

Google drive stores modification times accurate to 1 ms.
