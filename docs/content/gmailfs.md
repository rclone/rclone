---
title: "Gmail"
description: "Rclone docs for Gmail"
versionIntroduced: "v1.70"
---

# Gmail

The rclone Gmail backend (`gmailfs:`) provides read-only access to your Gmail
messages as a virtual filesystem.

**NB** This backend is **read-only**. Uploading, deleting, or modifying messages
is not supported. See the [Limitations](#limitations) section.

**NB** You must supply your own OAuth2 credentials (`client_id` and
`client_secret`). Rclone does not bundle Gmail credentials.

## Configuration

The initial setup for the Gmail backend requires you to create your own Google
OAuth2 client and supply its `client_id` and `client_secret`. There is no
default credential bundled with rclone for Gmail.

1. Go to the [Google Cloud Console](https://console.cloud.google.com/),
   create (or select) a project, and enable the **Gmail API** for it.
2. Configure the OAuth consent screen and create an **OAuth client ID** of
   type *Desktop app*. Note down the generated `client_id` and `client_secret`.

Then run `rclone config` and answer the prompts:

```
No remotes found, make a new one?
n) New remote
n/s/q> n

name> gmail

Type of storage to configure.
Storage> gmailfs

Google Application Client Id - REQUIRED.
client_id> YOUR_CLIENT_ID

Google Application Client Secret - REQUIRED.
client_secret> YOUR_CLIENT_SECRET

Oldest year to include when listing the year directories.
start_year> 2000

Use web browser to automatically authenticate rclone?
y/n> y
```

After completing the browser-based OAuth flow rclone stores the resulting token
in your configuration.

### Options

- `client_id` — **required** Google application client id. No default is
  provided by rclone.
- `client_secret` — **required** Google application client secret. No default is
  provided by rclone.
- `start_year` — oldest year to include when generating the year directory list
  (default: 2000).

## Filesystem layout

```
gmailfs:
  └── {Year}/
      └── {YYYY-MM}/
          └── {YYYY-MM-DD}/
              └── {threadId} — {Subject}/
                  ├── {messageId} — {Subject}.eml
                  └── attachments/
                      └── {messageId} — {filename}
```

Each year directory contains months, each month contains days, and each day
contains one directory per thread. Inside a thread directory the individual
messages are exposed as `.eml` files, and any attachments are placed in an
`attachments/` subdirectory.

## Limitations

- **Read-only**: no upload, delete, or modify support
- **No hash support**: checksums are not available
- **Credentials required**: `client_id` and `client_secret` must be supplied;
  rclone provides no default Gmail credential
- `start_year` limits how far back the year list extends (default: 2000)
