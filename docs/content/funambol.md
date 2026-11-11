---
title: "Funambol / OneMediaHub"
description: "Rclone docs for Funambol / OneMediaHub personal cloud (O2 Cloud, Movistar Cloud and other telco tenants)"
versionIntroduced: "v1.76"
---

# {{< icon "fa fa-cloud" >}} Funambol / OneMediaHub

This backend talks to the [Funambol](https://www.funambol.com/) /
**OneMediaHub** ("SAPI") personal-cloud platform.  The platform is
white-labelled by a number of mobile operators, so a single backend works for
many providers.  The default endpoint is **O2 Cloud** in Spain
(`https://cloud.o2online.es`); point `endpoint` at another tenant to use a
different operator, e.g. `https://micloud.movistar.es` for **Movistar Cloud**.

Paths are specified as `remote:path`

Paths may be as deep as required, e.g. `remote:directory/subdirectory`.

HTTPS SAPI requests use a Chrome TLS handshake and HTTP/2 to work with the
provider's browser checks. HTTP/1.1-only tenants are detected automatically.
The global `--disable-http2` flag selects the standard HTTP/1.1 transport.

## Configuration

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found, make a new one?
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / Funambol / OneMediaHub (O2 Cloud and other telco tenants)
   \ "funambol"
[snip]
Storage> funambol
Username for the Funambol / OneMediaHub account.
user> me@example.com
Password for the Funambol / OneMediaHub account.
y) Yes type in my own password
g) Generate random password
y/g> y
Enter the password:
password:
Confirm the password:
password:
Edit advanced config? (y/n)
y/n> n
--------------------
[remote]
type = funambol
user = me@example.com
pass = *** ENCRYPTED ***
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

Once configured you can use it like any other remote, for example:

List directories in the top level of your account

    rclone lsd remote:

List all the files in your account

    rclone ls remote:

Copy a local directory to the remote

    rclone copy /home/source remote:backup

### Authentication

Setup is interactive (`rclone config`).  You provide your username and, if
the account has one, password; rclone generates a device id, registers itself
as a new device and completes a one-time login, sending a verification code if
your account has two-factor authentication enabled.  The resulting session is
stored in the config - no browser fingerprint or manual `device_id` is needed.

O2 Spain accounts that sign in via Mi O2 have no password: leave `pass` empty
and set `user` to the mobile number. At the default O2 endpoint, leave
`client_id`, `client_secret` and `oauth_platform` blank. Rclone supplies the
O2 Cloud macOS 4.0.0 application's credentials and matching `macos` platform.
The initial SMS login obtains an OAuth refresh token. Rclone then renews
sessions and tokens automatically while the provider accepts these credentials.
The config must be writable so rotated credentials survive a restart.

These bundled credentials belong to the O2 desktop application, rather than a
dedicated rclone registration. They apply only to passwordless login at the
default O2 endpoint. A custom `client_id` does not inherit the bundled secret;
supply its matching `client_secret` if required, and set `oauth_platform` to
that application's platform (`windows` by default for custom applications).

Movistar Cloud accounts log in the same way, with the endpoint set to
`https://micloud.movistar.es`, but in a browser: rclone uses the Movistar Cloud
Windows application's credentials and shows the URL of Telefónica's login
page. Open it, log in with the code sent to your phone, then paste the address
of the page the browser ends up on (it contains `/ui/html/clientoauth.html`)
back into rclone. For example:

    rclone config create remote funambol user=612345678 endpoint=https://micloud.movistar.es

Existing remotes containing only session cookies need one
`rclone config reconnect remote:` with the OAuth settings above to obtain a
refresh token. Clear a previously saved `oauth_platform = windows` when using
the built-in macOS credentials, or set it explicitly to `macos`. Cookies cannot
be converted into a refresh token. If the provider revokes the saved refresh
credentials, reconnect again.

Password logins use the persistent-login cookie to obtain fresh validation
keys. Other passwordless configurations without OAuth application credentials
use temporary web SMS sessions, which may require another code when they expire.

### Modification times and hashes

The remote stores modification times to the nearest second.  Modification
times are set on upload but cannot be changed afterwards without re-uploading.

No content hash is exposed.  The item ETag looks like a digest but is assigned
per upload (two uploads of identical bytes get different ETags), so it is not a
usable hash; `rclone check` therefore relies on size and modification time.

### Restrictions

Files are stored as generic items (no server-side transcoding), so any file
type round-trips byte-for-byte. The service refuses filenames ending in
`.tmp` (case-insensitive); rclone reports these without retrying.

Uploads and folder creations are not immediately listable: the service indexes
new items a few seconds after they are written, so a listing taken right after
a write may not yet show it.

Existing generic files are overwritten in place, preserving their item IDs.
Same-name duplicates and existing picture, video or audio items are removed
before uploading a replacement as a generic file.

<!-- autogenerated options start - DO NOT EDIT - instead edit fs.RegInfo in backend/funambol/funambol.go and run make backenddocs to verify --> <!-- markdownlint-disable-line line-length -->
### Standard options

Here are the Standard options specific to funambol (Funambol / OneMediaHub (O2 Cloud and other telco tenants)).

#### --funambol-user

Username for the Funambol / OneMediaHub account.

Usually the email address or mobile number (MSISDN) you log in with.
For SMS login (no password) this must be the mobile number.

Properties:

- Config:      user
- Env Var:     RCLONE_FUNAMBOL_USER
- Type:        string
- Required:    true

#### --funambol-pass

Password for the Funambol / OneMediaHub account.

Leave empty for accounts without one (O2 Spain accounts that sign in
via Mi O2): rclone then logs in with an SMS code sent to the mobile
number in user.

**NB** Input to this must be obscured - see [rclone obscure](/commands/rclone_obscure/).

Properties:

- Config:      pass
- Env Var:     RCLONE_FUNAMBOL_PASS
- Type:        string
- Required:    false

### Advanced options

Here are the Advanced options specific to funambol (Funambol / OneMediaHub (O2 Cloud and other telco tenants)).

#### --funambol-cookies

Persisted session cookies (set automatically by rclone config).

This holds the logged-in session established during config (after any
two-factor step) so it can be reused.  When it expires, reconnect with
"rclone config reconnect remote:".

Properties:

- Config:      cookies
- Env Var:     RCLONE_FUNAMBOL_COOKIES
- Type:        string
- Required:    false

#### --funambol-device-id

Device id reported to the service (auto-generated if left blank).

This identifies the rclone "device" to the service and appears in your
account's device list.  Leave it empty: rclone generates a stable
"web-<hex>" id on first login and saves it here, registering itself just
like the official app.  Set it only to reuse a specific existing id.

Properties:

- Config:      device_id
- Env Var:     RCLONE_FUNAMBOL_DEVICE_ID
- Type:        string
- Required:    false

#### --funambol-endpoint

SAPI endpoint of the provider hosting the service.

Funambol / OneMediaHub is white-labelled by several operators.  Leave
this set to the default for O2 Spain, or point it at another tenant,
e.g. https://www.cloud.example.com

Properties:

- Config:      endpoint
- Env Var:     RCLONE_FUNAMBOL_ENDPOINT
- Type:        string
- Default:     "https://cloud.o2online.es"

#### --funambol-encoding

The encoding for the backend.

See the [encoding section in the overview](/overview/#encoding) for more info.

Properties:

- Config:      encoding
- Env Var:     RCLONE_FUNAMBOL_ENCODING
- Type:        Encoding
- Default:     Slash,DoubleQuote,BackSlash,Del,Ctl,InvalidUtf8,Dot

#### --funambol-description

Description of the remote.

Properties:

- Config:      description
- Env Var:     RCLONE_FUNAMBOL_DESCRIPTION
- Type:        string
- Required:    false

<!-- autogenerated options stop -->

## Limitations

Server-side file move/rename is supported on the same configured remote, for
generic `file` items, which include all files uploaded by this backend, and for
existing picture, video and audio items. Server-side copy is supported for
generic `file` items only; pictures, videos and audio use rclone's
download/upload fallback. Directory move/rename is also supported.

Native file rename preserves the original extension on O2. When the requested
extension changes, rclone uses server-side copy followed by deletion instead.
File operations wait up to two minutes for server-side validation and metadata
to become available. Existing destinations are removed before replacement.

The server limits the size of one file (4 GiB on O2 Spain). The backend reads
the limit from the server and refuses a bigger file before the upload starts.
To store bigger files, wrap the backend with [chunker](/chunker/).

The download server mishandles a range request that starts at 2,000,000,000
bytes or later: it answers with two zero bytes instead of the data. Reads
that start earlier are fine, including ones that continue past that offset.
To work around this, rclone reads from byte 1,999,999,999 and discards the
extra bytes, so seeking far into a very large file downloads (and throws away)
up to the difference. Multi-thread downloads are disabled for the same reason.

The backend can be wrapped by [chunker](/chunker/) and then [crypt](/crypt/).
Server-side copy and move do not encrypt existing plaintext files; migrating
those files to crypt still requires downloading and uploading their contents.

This backend was developed against the O2 Spain tenant; other Funambol /
OneMediaHub operators should work by setting `endpoint`, but have not all been
tested.
