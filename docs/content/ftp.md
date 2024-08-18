---
title: "FTP"
description: "Rclone docs for FTP backend"
versionIntroduced: "v1.37"
---

# {{< icon "fa fa-file" >}} FTP

FTP is the File Transfer Protocol. Rclone FTP support is provided using the
[github.com/jlaffaye/ftp](https://godoc.org/github.com/jlaffaye/ftp)
package.

[Limitations of Rclone's FTP backend](#limitations)

Paths are specified as `remote:path`. If the path does not begin with
a `/` it is relative to the home directory of the user.  An empty path
`remote:` refers to the user's home directory.

## Configuration

To create an FTP configuration named `remote`, run

    rclone config

Rclone config guides you through an interactive setup process. A minimal
rclone FTP remote definition only requires host, username and password.
For an anonymous FTP server, see [below](#anonymous-ftp).

```
No remotes found, make a new one?
n) New remote
r) Rename remote
c) Copy remote
s) Set configuration password
q) Quit config
n/r/c/s/q> n
name> remote
Type of storage to configure.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
[snip]
XX / FTP
   \ "ftp"
[snip]
Storage> ftp
** See help for ftp backend at: https://rclone.org/ftp/ **

FTP host to connect to
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
 1 / Connect to ftp.example.com
   \ "ftp.example.com"
host> ftp.example.com
FTP username
Enter a string value. Press Enter for the default ("$USER").
user> 
FTP port number
Enter a signed integer. Press Enter for the default (21).
port> 
FTP password
y) Yes type in my own password
g) Generate random password
y/g> y
Enter the password:
password:
Confirm the password:
password:
Use FTP over TLS (Implicit)
Enter a boolean value (true or false). Press Enter for the default ("false").
tls> 
Use FTP over TLS (Explicit)
Enter a boolean value (true or false). Press Enter for the default ("false").
explicit_tls> 
Remote config
Configuration complete.
Options:
- type: ftp
- host: ftp.example.com
- pass: *** ENCRYPTED ***
Keep this "remote" remote?
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

To see all directories in the home directory of `remote`

    rclone lsd remote:

Make a new directory

    rclone mkdir remote:path/to/directory

List the contents of a directory

    rclone ls remote:path/to/directory

Sync `/home/local/directory` to the remote directory, deleting any
excess files in the directory.

    rclone sync --interactive /home/local/directory remote:directory

### Anonymous FTP

When connecting to a FTP server that allows anonymous login, you can use the
special "anonymous" username. Traditionally, this user account accepts any
string as a password, although it is common to use either the password
"anonymous" or "guest". Some servers require the use of a valid e-mail
address as password.

Using [on-the-fly](#backend-path-to-dir) or
[connection string](/docs/#connection-strings) remotes makes it easy to access
such servers, without requiring any configuration in advance. The following
are examples of that:

    rclone lsf :ftp: --ftp-host=speedtest.tele2.net --ftp-user=anonymous --ftp-pass=$(rclone obscure dummy)
    rclone lsf :ftp,host=speedtest.tele2.net,user=anonymous,pass=$(rclone obscure dummy):

The above examples work in Linux shells and in PowerShell, but not Windows
Command Prompt. They execute the [rclone obscure](/commands/rclone_obscure/)
command to create a password string in the format required by the
[pass](#ftp-pass) option. The following examples are exactly the same, except use
an already obscured string representation of the same password "dummy", and
therefore works even in Windows Command Prompt:

    rclone lsf :ftp: --ftp-host=speedtest.tele2.net --ftp-user=anonymous --ftp-pass=IXs2wc8OJOz7SYLBk47Ji1rHTmxM
    rclone lsf :ftp,host=speedtest.tele2.net,user=anonymous,pass=IXs2wc8OJOz7SYLBk47Ji1rHTmxM:

### Implicit TLS

Rlone FTP supports implicit FTP over TLS servers (FTPS). This has to
be enabled in the FTP backend config for the remote, or with
[`--ftp-tls`](#ftp-tls). The default FTPS port is `990`, not `21` and
can be set with [`--ftp-port`](#ftp-port).

### Restricted filename characters

In addition to the [default restricted characters set](/overview/#restricted-characters)
the following characters are also replaced:

File names cannot end with the following characters. Replacement is
limited to the last character in a file name:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| SP        | 0x20  | ␠           |

Not all FTP servers can have all characters in file names, for example:

| FTP Server| Forbidden characters |
| --------- |:--------------------:|
| proftpd   | `*`                  |
| pureftpd  | `\ [ ]`              |

This backend's interactive configuration wizard provides a selection of
sensible encoding settings for major FTP servers: ProFTPd, PureFTPd, VsFTPd.
Just hit a selection number when prompted.

{{< rem autogenerated options start" - DO NOT EDIT - instead edit fs.RegInfo in backend/ftp/ftp.go then run make backenddocs" >}}
### Standard options

Here are the Standard options specific to ftp (FTP).

#### --ftp-host

FTP host to connect to.

E.g. "ftp.example.com".

Properties:

- Config:      host
- Env Var:     RCLONE_FTP_HOST
- Type:        string
- Required:    true

#### --ftp-user

FTP username.

Properties:

- Config:      user
- Env Var:     RCLONE_FTP_USER
- Type:        string
- Default:     "$USER"

#### --ftp-port

FTP port number.

Properties:

- Config:      port
- Env Var:     RCLONE_FTP_PORT
- Type:        int
- Default:     21

#### --ftp-pass

FTP password.

**NB** Input to this must be obscured - see [rclone obscure](/commands/rclone_obscure/).

Properties:

- Config:      pass
- Env Var:     RCLONE_FTP_PASS
- Type:        string
- Required:    false

#### --ftp-tls

Use Implicit FTPS (FTP over TLS).

When using implicit FTP over TLS the client connects using TLS
right from the start which breaks compatibility with
non-TLS-aware servers. This is usually served over port 990 rather
than port 21. Cannot be used in combination with explicit FTPS.

Properties:

- Config:      tls
- Env Var:     RCLONE_FTP_TLS
- Type:        bool
- Default:     false

#### --ftp-explicit-tls

Use Explicit FTPS (FTP over TLS).

When using explicit FTP over TLS the client explicitly requests
security from the server in order to upgrade a plain text connection
to an encrypted one. Cannot be used in combination with implicit FTPS.

Properties:

- Config:      explicit_tls
- Env Var:     RCLONE_FTP_EXPLICIT_TLS
- Type:        bool
- Default:     false

### Advanced options

Here are the Advanced options specific to ftp (FTP).

#### --ftp-concurrency

Maximum number of FTP simultaneous connections, 0 for unlimited.

Note that setting this is very likely to cause deadlocks so it should
be used with care.

If you are doing a sync or copy then make sure concurrency is one more
than the sum of `--transfers` and `--checkers`.

If you use `--check-first` then it just needs to be one more than the
maximum of `--checkers` and `--transfers`.

So for `concurrency 3` you'd use `--checkers 2 --transfers 2
--check-first` or `--checkers 1 --transfers 1`.



Properties:

- Config:      concurrency
- Env Var:     RCLONE_FTP_CONCURRENCY
- Type:        int
- Default:     0

#### --ftp-no-check-certificate

Do not verify the TLS certificate of the server.

Properties:

- Config:      no_check_certificate
- Env Var:     RCLONE_FTP_NO_CHECK_CERTIFICATE
- Type:        bool
- Default:     false

#### --ftp-disable-epsv

Disable using EPSV even if server advertises support.

Properties:

- Config:      disable_epsv
- Env Var:     RCLONE_FTP_DISABLE_EPSV
- Type:        bool
- Default:     false

#### --ftp-disable-mlsd

Disable using MLSD even if server advertises support.

Properties:

- Config:      disable_mlsd
- Env Var:     RCLONE_FTP_DISABLE_MLSD
- Type:        bool
- Default:     false

#### --ftp-disable-utf8

Disable using UTF-8 even if server advertises support.

Properties:

- Config:      disable_utf8
- Env Var:     RCLONE_FTP_DISABLE_UTF8
- Type:        bool
- Default:     false

#### --ftp-writing-mdtm

Use MDTM to set modification time (VsFtpd quirk)

Properties:

- Config:      writing_mdtm
- Env Var:     RCLONE_FTP_WRITING_MDTM
- Type:        bool
- Default:     false

#### --ftp-force-list-hidden

Use LIST -a to force listing of hidden files and folders. This will disable the use of MLSD.

Properties:

- Config:      force_list_hidden
- Env Var:     RCLONE_FTP_FORCE_LIST_HIDDEN
- Type:        bool
- Default:     false

#### --ftp-idle-timeout

Max time before closing idle connections.

If no connections have been returned to the connection pool in the time
given, rclone will empty the connection pool.

Set to 0 to keep connections indefinitely.


Properties:

- Config:      idle_timeout
- Env Var:     RCLONE_FTP_IDLE_TIMEOUT
- Type:        Duration
- Default:     1m0s

#### --ftp-close-timeout

Maximum time to wait for a response to close.

Properties:

- Config:      close_timeout
- Env Var:     RCLONE_FTP_CLOSE_TIMEOUT
- Type:        Duration
- Default:     1m0s

#### --ftp-tls-cache-size

Size of TLS session cache for all control and data connections.

TLS cache allows to resume TLS sessions and reuse PSK between connections.
Increase if default size is not enough resulting in TLS resumption errors.
Enabled by default. Use 0 to disable.

Properties:

- Config:      tls_cache_size
- Env Var:     RCLONE_FTP_TLS_CACHE_SIZE
- Type:        int
- Default:     32

#### --ftp-disable-tls13

Disable TLS 1.3 (workaround for FTP servers with buggy TLS)

Properties:

- Config:      disable_tls13
- Env Var:     RCLONE_FTP_DISABLE_TLS13
- Type:        bool
- Default:     false

#### --ftp-shut-timeout

Maximum time to wait for data connection closing status.

Properties:

- Config:      shut_timeout
- Env Var:     RCLONE_FTP_SHUT_TIMEOUT
- Type:        Duration
- Default:     1m0s

#### --ftp-ask-password

Allow asking for FTP password when needed.

If this is set and no password is supplied then rclone will ask for a password


Properties:

- Config:      ask_password
- Env Var:     RCLONE_FTP_ASK_PASSWORD
- Type:        bool
- Default:     false

#### --ftp-socks-proxy

Socks 5 proxy host.
		
		Supports the format user:pass@host:port, user@host:port, host:port.
		
		Example:
		
			myUser:myPass@localhost:9005
		

Properties:

- Config:      socks_proxy
- Env Var:     RCLONE_FTP_SOCKS_PROXY
- Type:        string
- Required:    false

#### --ftp-encoding

The encoding for the backend.

See the [encoding section in the overview](/overview/#encoding) for more info.

Properties:

- Config:      encoding
- Env Var:     RCLONE_FTP_ENCODING
- Type:        Encoding
- Default:     Slash,Del,Ctl,RightSpace,Dot
- Examples:
    - "Asterisk,Ctl,Dot,Slash"
        - ProFTPd can't handle '*' in file names
    - "BackSlash,Ctl,Del,Dot,RightSpace,Slash,SquareBracket"
        - PureFTPd can't handle '[]' or '*' in file names
    - "Ctl,LeftPeriod,Slash"
        - VsFTPd can't handle file names starting with dot

#### --ftp-description

Description of the remote.

Properties:

- Config:      description
- Env Var:     RCLONE_FTP_DESCRIPTION
- Type:        string
- Required:    false

{{< rem autogenerated options stop >}}

## Limitations

FTP servers acting as rclone remotes must support `passive` mode.
The mode cannot be configured as `passive` is the only supported one.
Rclone's FTP implementation is not compatible with `active` mode
as [the library it uses doesn't support it](https://github.com/jlaffaye/ftp/issues/29).
This will likely never be supported due to security concerns.

Rclone's FTP backend does not support any checksums but can compare
file sizes.

`rclone about` is not supported by the FTP backend. Backends without
this capability cannot determine free space for an rclone mount or
use policy `mfs` (most free space) as a member of an rclone union
remote.

See [List of backends that do not support rclone about](https://rclone.org/overview/#optional-features) and [rclone about](https://rclone.org/commands/rclone_about/)

The implementation of : `--dump headers`,
`--dump bodies`, `--dump auth` for debugging isn't the same as
for rclone HTTP based backends - it has less fine grained control.

`--timeout` isn't supported (but `--contimeout` is).

`--bind` isn't supported.

Rclone's FTP backend could support server-side move but does not
at present.

The `ftp_proxy` environment variable is not currently supported.

### Modification times

File modification time (timestamps) is supported to 1 second resolution
for major FTP servers: ProFTPd, PureFTPd, VsFTPd, and FileZilla FTP server.
The `VsFTPd` server has non-standard implementation of time related protocol
commands and needs a special configuration setting: `writing_mdtm = true`.

Support for precise file time with other FTP servers varies depending on what
protocol extensions they advertise. If all the `MLSD`, `MDTM` and `MFTM`
extensions are present, rclone will use them together to provide precise time.
Otherwise the times you see on the FTP server through rclone are those of the
last file upload.

You can use the following command to check whether rclone can use precise time
with your FTP server: `rclone backend features your_ftp_remote:` (the trailing
colon is important). Look for the number in the line tagged by `Precision`
designating the remote time precision expressed as nanoseconds. A value of
`1000000000` means that file time precision of 1 second is available.
A value of `3153600000000000000` (or another large number) means "unsupported".
