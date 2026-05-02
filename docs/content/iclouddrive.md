---
title: "iCloud Drive and Photos"
description: "Rclone docs for iCloud Drive and Photos"
versionIntroduced: "v1.69"
---

# iCloud Drive and Photos

## Configuration

The initial setup for an iCloud backend involves getting a trust token/session.
This uses your regular Apple ID password plus 2FA, either from a trusted
device prompt or an SMS code sent to a trusted phone number.

**IMPORTANT**: App-specific passwords are not accepted. Only use your
regular Apple ID password and 2FA.

This backend serves two Apple services:

- `drive` - iCloud Drive (default)
- `photos` - iCloud Photos

`rclone config` walks you through the token creation. The trust token is valid
for 30 days. After which you will have to reauthenticate with `rclone reconnect`
or `rclone config`.

## Authentication

rclone authenticates with Apple using [SRP (Secure Remote Password)](https://en.wikipedia.org/wiki/Secure_Remote_Password_protocol),
the same protocol used by the iCloud web interface. Your password is never
sent to Apple's servers -- instead, a cryptographic proof is exchanged that
verifies you know the password without revealing it.

The authentication flow is:

1. rclone initiates a session with Apple's identity service
2. An SRP key exchange takes place (your password is used locally to derive a key)
3. Apple sends a 2FA prompt to your trusted devices, or lets you request an
   SMS code
4. After you enter the 2FA code, rclone receives a trust token for future sessions

Here is an example of how to make a Photos remote called `icloudphotos`.
For iCloud Drive, leave `service` at its default `drive` value. First run:

```console
rclone config
```

This will guide you through an interactive setup process:

```text
No remotes found, make a new one?
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> icloudphotos
Option Storage.
Type of storage to configure.
Choose a number from below, or type in your own value.
[snip]
XX / iCloud Drive
   \ (iclouddrive)
[snip]
Storage> iclouddrive
Option service.
iCloud service to use.
Choose a number from below, or type in your own value of type string.
Press Enter for the default (drive).
 1 / iCloud Drive
   \ (drive)
 2 / iCloud Photos
   \ (photos)
service> 2
Option apple_id.
Apple ID.
Enter a value.
apple_id> APPLEID  
Option password.
Password.
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
Two-factor authentication: enter your 2FA code or type 'sms' for a text message
Enter a value.
config_2fa> 2FACODE
Remote config
--------------------
[icloudphotos]
- type: iclouddrive
- service: photos
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

## iCloud Photos

The iCloud Drive backend also supports accessing iCloud Photos by setting the
`service` option to `photos`:

```console
rclone lsd iclouddrive: --iclouddrive-service photos
```

This presents a read-only hierarchy rooted at photo libraries:

- **Level 1**: Photo libraries — your personal library (`PrimarySync`) and
  any Shared Photo Library (`SharedSync-XXXX`)
- **Level 2+**: Albums and folders within a library, nested recursively as in
  Apple Photos
- **Leaf level**: Photos/videos within an album, including Live Photo `.MOV`
  companions

Examples:

```console
# List libraries
rclone lsd iclouddrive: --iclouddrive-service photos

# List albums in your primary library
rclone lsd iclouddrive:PrimarySync/ --iclouddrive-service photos

# List photos in an album
rclone ls iclouddrive:PrimarySync/All\ Photos/ --iclouddrive-service photos

# Download a photo
rclone copy iclouddrive:PrimarySync/Favorites/IMG_0001.HEIC /tmp/ --iclouddrive-service photos
```

You can either:

- set `service = photos` in `rclone config` for a dedicated Photos remote
- keep `service = drive` and pass `--iclouddrive-service photos` when needed

### Metadata

With `--metadata`, Photos entries expose these read-only metadata keys:

- `width`
- `height`
- `added-time`
- `favorite`
- `hidden`

These metadata keys are only available when `service = photos`.

### Caching

iCloud Photos caches album listings to disk for fast subsequent access.
On the first run, listing a large album uses parallel `startRank`
partitions to fetch pages concurrently. After that, a lightweight change
check (~200ms) determines whether the cache is still valid.

Cache location: `~/.cache/rclone/iclouddrive-photos/<remote>/<zone>/`

To clear the cache: delete that directory or run `rclone config reconnect`.

### FUSE mounts

For mounting iCloud Photos via `rclone mount`, the following flags
are recommended:

    rclone mount remote: /mnt/photos \
        --iclouddrive-service photos \
        --vfs-refresh \
        --dir-cache-time 1h \
        --vfs-cache-mode full \
        --attr-timeout 1m \
        --read-only

- `--vfs-refresh` pre-warms directory caches in the background on mount
  start so that albums are ready when you browse them
- `--dir-cache-time 1h` extends the in-memory cache lifetime beyond the
  default 5 minutes (change detection is fast, so this is safe)
- `--vfs-cache-mode full` caches downloaded photos and videos to local
  disk for fast repeated access
- `--attr-timeout 1m` reduces kernel attribute lookups (safe because
  the backend is read-only)
- `--read-only` prevents confusing write errors

The first listing of a very large album (e.g. 75,000 items in "All
Photos") can take several minutes due to API pagination limits. This
happens once — subsequent listings use the disk cache.

### Limitations

iCloud Photos is read-only. Upload, delete, rename, and move operations
are not supported.

## Advanced Data Protection

Advanced Data Protection is supported.

On iPhone, Settings `>` Apple Account `>` iCloud `>` 'Access iCloud Data on the Web'
must be ON.

If ADP is enabled on your account, rclone requests PCS cookies after 2FA.
Apple may require approval on a trusted device before those cookies are issued.

## Troubleshooting

### PCS cookie errors with ADP

If you see `Missing PCS cookies from the request` or a `requestPCS:` error,
the ADP approval flow did not complete successfully.

Check that 'Access iCloud Data on the Web' is enabled and approve any prompt
on your trusted device.

Then run `rclone reconnect remote:`.

If the remote still has stale auth state, clear the `cookies` and
`trust_token` fields in the config, or delete and recreate the remote.

<!-- autogenerated options start - DO NOT EDIT - instead edit fs.RegInfo in backend/iclouddrive/iclouddrive.go and run make backenddocs to verify --> <!-- markdownlint-disable-line line-length -->
### Standard options

Here are the Standard options specific to iclouddrive (iCloud Drive and Photos).

#### --iclouddrive-service

iCloud service to use.

Properties:

- Config:      service
- Env Var:     RCLONE_ICLOUDDRIVE_SERVICE
- Type:        string
- Default:     "drive"
- Examples:
  - "drive"
    - iCloud Drive
  - "photos"
    - iCloud Photos

#### --iclouddrive-apple-id

Apple ID.

Properties:

- Config:      apple_id
- Env Var:     RCLONE_ICLOUDDRIVE_APPLE_ID
- Type:        string
- Required:    true

#### --iclouddrive-password

Password.

**NB** Input to this must be obscured - see [rclone obscure](/commands/rclone_obscure/).

Properties:

- Config:      password
- Env Var:     RCLONE_ICLOUDDRIVE_PASSWORD
- Type:        string
- Required:    true

#### --iclouddrive-trust-token

Trust token for session authentication.

Properties:

- Config:      trust_token
- Env Var:     RCLONE_ICLOUDDRIVE_TRUST_TOKEN
- Type:        string
- Required:    false

#### --iclouddrive-cookies

Session cookies.

Properties:

- Config:      cookies
- Env Var:     RCLONE_ICLOUDDRIVE_COOKIES
- Type:        string
- Required:    false

### Advanced options

Here are the Advanced options specific to iclouddrive (iCloud Drive and Photos).

#### --iclouddrive-client-id

Client ID for iCloud API access.

Properties:

- Config:      client_id
- Env Var:     RCLONE_ICLOUDDRIVE_CLIENT_ID
- Type:        string
- Default:     "d39ba9916b7251055b22c7f910e2ea796ee65e98b2ddecea8f5dde8d9d1a815d"

#### --iclouddrive-encoding

The encoding for the backend.

See the [encoding section in the overview](/overview/#encoding) for more info.

Properties:

- Config:      encoding
- Env Var:     RCLONE_ICLOUDDRIVE_ENCODING
- Type:        Encoding
- Default:     Slash,BackSlash,Del,Ctl,InvalidUtf8,Dot

#### --iclouddrive-description

Description of the remote.

Properties:

- Config:      description
- Env Var:     RCLONE_ICLOUDDRIVE_DESCRIPTION
- Type:        string
- Required:    false

### Metadata

Metadata is read-only and available for the Photos service only.

Here are the possible system metadata items for the iclouddrive backend.

| Name | Help | Type | Example | Read Only |
|------|------|------|---------|-----------|
| added-time | Time the item was added to the iCloud library | RFC 3339 | 2006-01-02T15:04:05Z | **Y** |
| favorite | Whether the item is marked as favorite | bool |  | **Y** |
| height | Image height in pixels | int |  | **Y** |
| hidden | Whether the item is hidden | bool |  | **Y** |
| width | Image width in pixels | int |  | **Y** |

See the [metadata](/docs/#metadata) docs for more info.

<!-- autogenerated options stop -->
