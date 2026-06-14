---
title: "MCP"
description: "Rclone docs for MCP"
versionIntroduced: "v1.75"
---

# MCP (Model Context Protocol)

The rclone MCP backend (`mcp:`) makes rclone an [MCP](https://modelcontextprotocol.io/)
*client*: it connects to an MCP server and presents that server's capabilities
as a mostly read-only directory tree.

The server may be run locally over its stdin/stdout (`stdio` transport) or
reached over the network using the streamable HTTP (`http`) or HTTP+SSE (`sse`)
transports.

**NB** This backend is **read-only**. Uploading, deleting, or modifying files is
not supported. See the [Limitations](#limitations) section.

## Configuration

Run `rclone config` and answer the prompts:

```
No remotes found, make a new one?
n) New remote
n/s/q> n

name> mcp

Type of storage to configure.
Storage> mcp

Transport used to talk to the MCP server.
transport> auto

Command to run the MCP server for the stdio transport.
command> npx -y @modelcontextprotocol/server-everything
```

For a `stdio` server set `command` to the command (and arguments) that starts
the MCP server. For an `http` or `sse` server leave `command` empty and set
`url` to the server's endpoint, e.g. `https://example.com/mcp`. With the default
`transport` of `auto`, rclone picks `stdio` when a `command` is configured and
`http` when a `url` is configured.

### Options

- `transport` — transport used to talk to the MCP server. One of `auto`
  (default), `stdio` (run `command` and talk over its stdin/stdout), `http`
  (connect to `url` with the streamable HTTP transport) or `sse` (connect to
  `url` with the HTTP+SSE transport).
- `command` — command to run the MCP server for the `stdio` transport. A space
  separated list of the command and its arguments; CSV encoding may be used to
  include spaces in an argument.
- `url` — URL of the MCP server for the `http` or `sse` transport, e.g.
  `https://example.com/mcp`.
- `headers` — extra HTTP headers to send for the `http` and `sse` transports, as
  a comma separated list of key,value pairs (advanced).
- `bearer_token` — bearer token to send in the `Authorization` header for the
  `http` and `sse` transports (advanced).
- `sockets` — bind a Unix socket per tool and expose it as a `.sock` symlink for
  use under `rclone mount` (advanced).
- `socket_dir` — directory to bind the per-tool Unix sockets in; implies
  `sockets`. If unset, a temporary directory is used and removed on exit
  (advanced).

## Filesystem layout

```
mcp:
  ├── README.md            server overview, built from the initialize result
  ├── config.json          connection info and advertised capabilities
  ├── logs.txt             a snapshot of this session's activity log
  ├── tools/               one directory per tool
  ├── resources/           the server's resources, exposed as readable files
  └── prompts/             one markdown file per prompt template
```

`tools/`, `resources/` and `prompts/` each contain a `schema.json` describing
every advertised item, plus per-item entries. Tools and prompts are invoked with
the backend command, e.g.

```
rclone backend call   mcp: <tool> '<json-arguments>'
rclone backend prompt mcp: <prompt> arg=value
```

## Limitations

- **Read-only**: no upload, delete, or modify support
- **No hash support**: checksums are not available
- The available files and directories depend entirely on the connected MCP
  server and the capabilities it advertises
