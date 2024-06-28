---
title: "iCloud Drive"
description: "Rclone docs for iCloud Drive"
versionIntroduced: "v1.66"
---

# {{< icon "fa fa-cloud" >}} iCloud


## Configuration

The initial setup for an iCloud Drive backend involves getting a session.
`rclone config` walks you through it.

Here is an example of how to make a remote called `iclouddrive`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found, make a new one?
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> iclouddrive
Option Storage.
Type of storage to configure.
Choose a number from below, or type in your own value.
[snip]
55 / iCloud Drive
   \ (iclouddrive)
[snip]
Storage> iclouddrive
Option apple_id.
Apple ID.
Enter a value.
apple_id> APPLEID  
Option password.
Your password for rclone (generate one at https://app.koofr.net/app/admin/preferences/password).
Choose an alternative below.
y) Yes, type in my own password
g) Generate random password
y/g> y
Enter the password:
password:
Confirm the password:
password:
Edit advanced config?
y) Yes
n) No (default)
y/n> n
Option config_2fa.
Two-factor authentication: please enter your 2FA code
Enter a value.
config_2fa> 2FACODE
Remote config
--------------------
[koofr]
- type: iclouddrive
- apple_id: APPLEID
- password: *** ENCRYPTED ***
- cookies: ****************************
- trust_token: ****************************
--------------------
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d> y
```

## Advanced Data Protection

ADP is currently unsupported and need to be disabled
