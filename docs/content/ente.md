---
title: "Ente.io"
description: "Rclone docs for Ente.io"
---

# Ente.io

[Ente.io](https://ente.io) is an end-to-end encrypted cloud storage platform for photos, videos, and files.

## Configuration

To create a new Ente remote, run:

```bash
rclone config
```

Select `n` for a new remote and choose `ente`.

### Options

#### email
Email associated with your Ente.io account.

#### password
Password for your Ente.io account.

#### session_token
Optional session authentication token (`X-Auth-Token`).

#### account_key
Optional master encryption key (hex encoded).

#### endpoint
Ente API server URL. Default is `https://api.ente.com`. For self-hosted instances, set this to your server address (e.g. `http://localhost:8080`).

#### app
Ente app scope (`photos`, `drive`, `secrets`). Default is `photos`.

## Features
- End-to-end encrypted transfers
- Support for self-hosted Ente Museum servers
- Support for collections and file uploads/downloads
