---
title: "Google Calendar"
description: "Rclone docs for Google Calendar"
versionIntroduced: "v1.70"
---

# Google Calendar

The rclone Google Calendar backend (`gcalfs:`) provides read-only access to your
Google Calendar events as `.ics` files in a virtual filesystem.

**NB** Read-only. No upload, delete, or modify support.

**NB** You must supply your own OAuth2 credentials (`client_id` and `client_secret`).

## Configuration

The initial setup for the Google Calendar backend requires you to create your
own Google OAuth2 client and supply its `client_id` and `client_secret`. There
is no default credential bundled with rclone for Google Calendar.

1. Go to the [Google Cloud Console](https://console.cloud.google.com/),
   create (or select) a project, and enable the **Google Calendar API** for it.
2. Configure the OAuth consent screen and create an **OAuth client ID** of
   type *Desktop app*. Note down the generated `client_id` and `client_secret`.

Then run `rclone config` and answer the prompts:

```
No remotes found, make a new one?
n) New remote
n/s/q> n

name> gcal

Type of storage to configure.
Storage> gcalfs

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
gcalfs:
  └── {CalendarName}/
      └── {Year}/
          └── {YYYY-MM}/
              └── {YYYY-MM-DD}/
                  └── {eventId} — {Summary}.ics
```

Each calendar is exposed as a top-level directory containing years, months, and
days. Inside a day directory each event is exposed as a single `.ics` file.

## Limitations

- **Read-only**: no upload, delete, or modify support
- **No hash support**
- **Credentials required**: `client_id` and `client_secret` must be supplied
- `start_year` limits how far back the year list extends (default: 2000)
- Calendar names with duplicates are disambiguated with an 8-char ID suffix
- All-day events use `VALUE=DATE` in the `.ics` output
