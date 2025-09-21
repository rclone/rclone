# {{< icon "fa fa-moon" >}} Shade

This is a backend for the [Shade](https://shade.inc/) platform

## About Shade

[Shade](https://shade.inc/) is an AI-powered cloud NAS that makes your cloud files behave like a local drive, optimized for media and creative workflows. It provides fast, secure access with natural-language search, easy sharing, and scalable cloud storage.


## Accounts & Pricing

To use this backend, you need to [create a free account](https://app.shade.inc/) on Shade. You can start with a free account and get 20GB of storage for free.

## Configuration

Here is an example of making a Shade configuration.

First, create a [create a free account](https://app.shade.inc/) account and choose a plan.

You will need to log in and get the `API Key` and `Drive ID` for your account from the settings section of your account and created drive respectively.

Now run

`rclone config`

Follow this interactive process:

```text
e) Edit existing remote
n) New remote
d) Delete remote
r) Rename remote
c) Copy remote
s) Set configuration password
q) Quit config
e/n/d/r/c/s/q> n

Enter name for new remote.
name> Shade

Option Storage.
Type of storage to configure.
Choose a number from below, or type in your own value.
[OTHER OPTIONS]
xx / Shade FS
   \ (shade)
[OTHER OPTIONS]
Storage> xx

Option drive_id.
The ID of your drive, see this in the drive settings. Individual rclone configs must be made per drive.
Enter a value.
drive_id> [YOUR_ID]

Option api_key.
An API key for your account.
Enter a value.
api_key> [YOUR_API_KEY]

Edit advanced config?
y) Yes
n) No (default)
y/n> n

Configuration complete.
Options:
- type: shade
- drive_id: [YOUR_ID]
- api_key: [YOUR_API_KEY]
Keep this "Shade" remote?
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d> y

```

### Standard options

Here are the Standard options specific to shade (Shade FS).

#### --shade-drive-id

The ID of your drive, see this in the drive settings. Individual rclone configs must be made per drive.

Properties:

- Config:      drive_id
- Env Var:     RCLONE_SHADE_DRIVE_ID
- Type:        string
- Required:    true

#### --shade-api-key

An API key for your account. You can find this under Settings > API Keys

Properties:

- Config:      api_key
- Env Var:     RCLONE_SHADE_API_KEY
- Type:        string
- Required:    true

### Advanced options

Here are the Advanced options specific to shade (Shade FS).

#### --shade-endpoint

Endpoint for the service.

Leave blank normally.

Properties:

- Config:      endpoint
- Env Var:     RCLONE_SHADE_ENDPOINT
- Type:        string
- Required:    false

#### --shade-chunk-size

Chunk size to use for uploading.

Any files larger than this will be uploaded in chunks of this size.

Note that this is stored in memory per transfer, so increasing it will
increase memory usage.

Minimum is 5MB, maximum is 5GB.

Properties:

- Config:      chunk_size
- Env Var:     RCLONE_SHADE_CHUNK_SIZE
- Type:        SizeSuffix
- Default:     64Mi

#### --shade-encoding

The encoding for the backend.

See the [encoding section in the overview](/overview/#encoding) for more info.

Properties:

- Config:      encoding
- Env Var:     RCLONE_SHADE_ENCODING
- Type:        Encoding
- Default:     Slash,BackSlash,Del,Ctl,InvalidUtf8,Dot

#### --shade-description

Description of the remote.

Properties:

- Config:      description
- Env Var:     RCLONE_SHADE_DESCRIPTION
- Type:        string
- Required:    false

## Backend commands

Here are the commands specific to the shade backend.

Run them with

    rclone backend COMMAND remote:

The help below will explain what arguments each command takes.

See the [backend](/commands/rclone_backend/) command for more
info on how to pass options and arguments.

These can be run on a running backend using the rc command
[backend/command](/rc/#backend-command).


