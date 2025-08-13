---
title: "piqlConnect"
description: "Rclone docs for piqlConnect"
versionIntroduced: "?"
---

# {{< icon "fa fa-film" >}} piqlConnect

## Configuration

Here is an example of how to make a remote called `remote`. First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found, make a new one?
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n

Enter name for new remote.
name> remote

Option Storage.
Type of storage to configure.
Choose a number from below, or type in your own value.
[snip]
XX / piqlConnect
   \ (piqlconnect)
[snip]
Storage> piqlconnect

Option api_key.
piqlConnect API key obtained from web interface
Enter a value.
api_key> ehu5f6wuxv7e3lt5x55yckihfiveosum

Option root_url.
piqlConnect API url
Enter a value of type string. Press Enter for the default (https://app.piql.com/api).
root_url>

Edit advanced config?
y) Yes
n) No (default)
y/n>

Configuration complete.
Options:
- type: piqlconnect
- api_key: ehu5f6wuxv7e3lt5x55yckihfiveosum
Keep this "remote" remote?
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
```
