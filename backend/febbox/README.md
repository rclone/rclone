# Febbox backend for rclone

This backend provides read/write access to Febbox cloud storage from rclone.

Features
- List, read, upload and delete objects
- Recursive directory operations (copy, sync)
- Server-side renames where supported by the API
- Configurable API endpoint and chunk size

Prerequisites
- A Febbox account and an API key (or token) with appropriate permissions.

Remote name and path format
- Remote names use the usual rclone syntax. Example: `febbox:container/path`

Authentication
- The backend reads credentials from the rclone config. You can configure using `rclone config` or environment variable `FEBBOX_API_KEY`.

Interactive `rclone config` example

1. `rclone config`
2. `n` (new remote)
3. name: `myfeb` (or any name)
4. type: `febbox`
5. When prompted, supply your cookies and share key.

Non-interactive example

Set the cookies and share key in the environment and create a remote:


Basic usage examples

- List files:

```
rclone ls myfeb:bucket-name
```

- Copy local to Febbox:

```
rclone copy /path/to/local myfeb:bucket-name/path -P
```

- Sync Febbox to local:

```
rclone sync myfeb:bucket-name/path /path/to/local -P
```

Notes and limitations
- The backend implements common file operations but may not support every advanced server-side feature of other storage backends (e.g., object lifecycle rules or advanced ACLs) depending on the Febbox API.
- Performance characteristics depend on the chosen `chunk_size` and network latency.

Troubleshooting
- Authentication errors: confirm `api_key` is correct and has permissions.
- Connection errors: check `endpoint` and network connectivity.

Contribution
- If you find issues or have feature requests, please open an issue in the rclone repository and include details about the Febbox API responses and sample commands that reproduce the problem.

License
- This documentation is distributed under the same license as rclone.
