---
title: "SFTP"
description: "SFTP"
---

{{< icon "fa fa-server" >}} SFTP
----------------------------------------

SFTP is the [Secure (or SSH) File Transfer
Protocol](https://en.wikipedia.org/wiki/SSH_File_Transfer_Protocol).

The SFTP backend can be used with a number of different providers:

{{< provider_list >}}
{{< provider name="C14" home="https://www.online.net/en/storage/c14-cold-storage" config="/sftp/#c14">}}
{{< provider name="rsync.net" home="https://rsync.net/products/rclone.html" config="/sftp/#rsync-net">}}
{{< /provider_list >}}

SFTP runs over SSH v2 and is installed as standard with most modern
SSH installations.

Paths are specified as `remote:path`. If the path does not begin with
a `/` it is relative to the home directory of the user.  An empty path
`remote:` refers to the user's home directory.

"Note that some SFTP servers will need the leading / - Synology is a
good example of this. rsync.net, on the other hand, requires users to
OMIT the leading /.

Here is an example of making an SFTP configuration.  First run

    rclone config

This will guide you through an interactive setup process.

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / SSH/SFTP Connection
   \ "sftp"
[snip]
Storage> sftp
SSH host to connect to
Choose a number from below, or type in your own value
 1 / Connect to example.com
   \ "example.com"
host> example.com
SSH username, leave blank for current username, $USER
user> sftpuser
SSH port, leave blank to use default (22)
port>
SSH password, leave blank to use ssh-agent.
y) Yes type in my own password
g) Generate random password
n) No leave this optional password blank
y/g/n> n
Path to unencrypted PEM-encoded private key file, leave blank to use ssh-agent.
key_file>
Remote config
--------------------
[remote]
host = example.com
user = sftpuser
port =
pass =
key_file =
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

This remote is called `remote` and can now be used like this:

See all directories in the home directory

    rclone lsd remote:

Make a new directory

    rclone mkdir remote:path/to/directory

List the contents of a directory

    rclone ls remote:path/to/directory

Sync `/home/local/directory` to the remote directory, deleting any
excess files in the directory.

    rclone sync -i /home/local/directory remote:directory

### SSH Authentication ###

The SFTP remote supports three authentication methods:

  * Password
  * Key file, including certificate signed keys
  * ssh-agent

Key files should be PEM-encoded private key files. For instance `/home/$USER/.ssh/id_rsa`.
Only unencrypted OpenSSH or PEM encrypted files are supported.

The key file can be specified in either an external file (key_file) or contained within the 
rclone config file (key_pem).  If using key_pem in the config file, the entry should be on a
single line with new line ('\n' or '\r\n') separating lines.  i.e. 

key_pem = -----BEGIN RSA PRIVATE KEY-----\nMaMbaIXtE\n0gAMbMbaSsd\nMbaass\n-----END RSA PRIVATE KEY-----

This will generate it correctly for key_pem for use in the config:  

    awk '{printf "%s\\n", $0}' < ~/.ssh/id_rsa

If you don't specify `pass`, `key_file`, or `key_pem` then rclone will attempt to contact an ssh-agent.

You can also specify `key_use_agent` to force the usage of an ssh-agent. In this case
`key_file` or `key_pem` can also be specified to force the usage of a specific key in the ssh-agent.

Using an ssh-agent is the only way to load encrypted OpenSSH keys at the moment.

If you set the `--sftp-ask-password` option, rclone will prompt for a
password when needed and no password has been configured.

If you have a certificate then you can provide the path to the public key that contains the certificate.  For example:

```
[remote]
type = sftp
host = example.com
user = sftpuser
key_file = ~/id_rsa
pubkey_file = ~/id_rsa-cert.pub
````

If you concatenate a cert with a private key then you can specify the
merged file in both places.

Note: the cert must come first in the file.  e.g.

```
cat id_rsa-cert.pub id_rsa > merged_key
```

### Host key validation ###

By default rclone will not check the server's host key for validation.  This
can allow an attacker to replace a server with their own and if you use
password authentication then this can lead to that password being exposed.

Host key matching, using standard `known_hosts` files can be turned on by
enabling the `known_hosts_file` option.  This can point to the file maintained
by `OpenSSH` or can point to a unique file.

e.g.

```
[remote]
type = sftp
host = example.com
user = sftpuser
pass = 
known_hosts_file = ~/.ssh/known_hosts
````

There are some limitations:

* `rclone` will not _manage_ this file for you.  If the key is missing or
wrong then the connection will be refused.
* If the server is set up for a certificate host key then the entry in
the `known_hosts` file _must_ be the `@cert-authority` entry for the CA
* Unlike `OpenSSH`, the libraries used by `rclone` do not permit (at time
of writing) multiple host keys to be listed for a server.  Only the first
entry is used.

If the host key provided by the server does not match the one in the
file (or is missing) then the connection will be aborted and an error
returned such as

    NewFs: couldn't connect SSH: ssh: handshake failed: knownhosts: key mismatch

or

    NewFs: couldn't connect SSH: ssh: handshake failed: knownhosts: key is unknown

If you see an error such as

    NewFs: couldn't connect SSH: ssh: handshake failed: ssh: no authorities for hostname: example.com:22

then it is likely the server has presented a CA signed host certificate
and you will need to add the appropriate `@cert-authority` entry.

The `known_hosts_file` setting can be set during `rclone config` as an
advanced option.

### ssh-agent on macOS ###

Note that there seem to be various problems with using an ssh-agent on
macOS due to recent changes in the OS.  The most effective work-around
seems to be to start an ssh-agent in each session, e.g.

    eval `ssh-agent -s` && ssh-add -A

And then at the end of the session

    eval `ssh-agent -k`

These commands can be used in scripts of course.

### Modified time ###

Modified times are stored on the server to 1 second precision.

Modified times are used in syncing and are fully supported.

Some SFTP servers disable setting/modifying the file modification time after
upload (for example, certain configurations of ProFTPd with mod_sftp). If you
are using one of these servers, you can set the option `set_modtime = false` in
your RClone backend configuration to disable this behaviour.

{{< rem autogenerated options start" - DO NOT EDIT - instead edit fs.RegInfo in backend/sftp/sftp.go then run make backenddocs" >}}
### Standard Options

Here are the standard options specific to sftp (SSH/SFTP Connection).

#### --sftp-host

SSH host to connect to

- Config:      host
- Env Var:     RCLONE_SFTP_HOST
- Type:        string
- Default:     ""
- Examples:
    - "example.com"
        - Connect to example.com

#### --sftp-user

SSH username, leave blank for current username, $USER

- Config:      user
- Env Var:     RCLONE_SFTP_USER
- Type:        string
- Default:     ""

#### --sftp-port

SSH port, leave blank to use default (22)

- Config:      port
- Env Var:     RCLONE_SFTP_PORT
- Type:        string
- Default:     ""

#### --sftp-pass

SSH password, leave blank to use ssh-agent.

**NB** Input to this must be obscured - see [rclone obscure](/commands/rclone_obscure/).

- Config:      pass
- Env Var:     RCLONE_SFTP_PASS
- Type:        string
- Default:     ""

#### --sftp-key-pem

Raw PEM-encoded private key, If specified, will override key_file parameter.

- Config:      key_pem
- Env Var:     RCLONE_SFTP_KEY_PEM
- Type:        string
- Default:     ""

#### --sftp-key-file

Path to PEM-encoded private key file, leave blank or set key-use-agent to use ssh-agent.

Leading `~` will be expanded in the file name as will environment variables such as `${RCLONE_CONFIG_DIR}`.


- Config:      key_file
- Env Var:     RCLONE_SFTP_KEY_FILE
- Type:        string
- Default:     ""

#### --sftp-key-file-pass

The passphrase to decrypt the PEM-encoded private key file.

Only PEM encrypted key files (old OpenSSH format) are supported. Encrypted keys
in the new OpenSSH format can't be used.

**NB** Input to this must be obscured - see [rclone obscure](/commands/rclone_obscure/).

- Config:      key_file_pass
- Env Var:     RCLONE_SFTP_KEY_FILE_PASS
- Type:        string
- Default:     ""

#### --sftp-pubkey-file

Optional path to public key file.

Set this if you have a signed certificate you want to use for authentication.

Leading `~` will be expanded in the file name as will environment variables such as `${RCLONE_CONFIG_DIR}`.


- Config:      pubkey_file
- Env Var:     RCLONE_SFTP_PUBKEY_FILE
- Type:        string
- Default:     ""

#### --sftp-key-use-agent

When set forces the usage of the ssh-agent.

When key-file is also set, the ".pub" file of the specified key-file is read and only the associated key is
requested from the ssh-agent. This allows to avoid `Too many authentication failures for *username*` errors
when the ssh-agent contains many keys.

- Config:      key_use_agent
- Env Var:     RCLONE_SFTP_KEY_USE_AGENT
- Type:        bool
- Default:     false

#### --sftp-use-insecure-cipher

Enable the use of insecure ciphers and key exchange methods. 

This enables the use of the following insecure ciphers and key exchange methods:

- aes128-cbc
- aes192-cbc
- aes256-cbc
- 3des-cbc
- diffie-hellman-group-exchange-sha256
- diffie-hellman-group-exchange-sha1

Those algorithms are insecure and may allow plaintext data to be recovered by an attacker.

- Config:      use_insecure_cipher
- Env Var:     RCLONE_SFTP_USE_INSECURE_CIPHER
- Type:        bool
- Default:     false
- Examples:
    - "false"
        - Use default Cipher list.
    - "true"
        - Enables the use of the aes128-cbc cipher and diffie-hellman-group-exchange-sha256, diffie-hellman-group-exchange-sha1 key exchange.

#### --sftp-disable-hashcheck

Disable the execution of SSH commands to determine if remote file hashing is available.
Leave blank or set to false to enable hashing (recommended), set to true to disable hashing.

- Config:      disable_hashcheck
- Env Var:     RCLONE_SFTP_DISABLE_HASHCHECK
- Type:        bool
- Default:     false

### Advanced Options

Here are the advanced options specific to sftp (SSH/SFTP Connection).

#### --sftp-known-hosts-file

Optional path to known_hosts file.

Set this value to enable server host key validation.

Leading `~` will be expanded in the file name as will environment variables such as `${RCLONE_CONFIG_DIR}`.


- Config:      known_hosts_file
- Env Var:     RCLONE_SFTP_KNOWN_HOSTS_FILE
- Type:        string
- Default:     ""
- Examples:
    - "~/.ssh/known_hosts"
        - Use OpenSSH's known_hosts file

#### --sftp-ask-password

Allow asking for SFTP password when needed.

If this is set and no password is supplied then rclone will:
- ask for a password
- not contact the ssh agent


- Config:      ask_password
- Env Var:     RCLONE_SFTP_ASK_PASSWORD
- Type:        bool
- Default:     false

#### --sftp-path-override

Override path used by SSH connection.

This allows checksum calculation when SFTP and SSH paths are
different. This issue affects among others Synology NAS boxes.

Shared folders can be found in directories representing volumes

    rclone sync /home/local/directory remote:/directory --ssh-path-override /volume2/directory

Home directory can be found in a shared folder called "home"

    rclone sync /home/local/directory remote:/home/directory --ssh-path-override /volume1/homes/USER/directory

- Config:      path_override
- Env Var:     RCLONE_SFTP_PATH_OVERRIDE
- Type:        string
- Default:     ""

#### --sftp-set-modtime

Set the modified time on the remote if set.

- Config:      set_modtime
- Env Var:     RCLONE_SFTP_SET_MODTIME
- Type:        bool
- Default:     true

#### --sftp-md5sum-command

The command used to read md5 hashes. Leave blank for autodetect.

- Config:      md5sum_command
- Env Var:     RCLONE_SFTP_MD5SUM_COMMAND
- Type:        string
- Default:     ""

#### --sftp-sha1sum-command

The command used to read sha1 hashes. Leave blank for autodetect.

- Config:      sha1sum_command
- Env Var:     RCLONE_SFTP_SHA1SUM_COMMAND
- Type:        string
- Default:     ""

#### --sftp-skip-links

Set to skip any symlinks and any other non regular files.

- Config:      skip_links
- Env Var:     RCLONE_SFTP_SKIP_LINKS
- Type:        bool
- Default:     false

#### --sftp-subsystem

Specifies the SSH2 subsystem on the remote host.

- Config:      subsystem
- Env Var:     RCLONE_SFTP_SUBSYSTEM
- Type:        string
- Default:     "sftp"

#### --sftp-server-command

Specifies the path or command to run a sftp server on the remote host.

The subsystem option is ignored when server_command is defined.

- Config:      server_command
- Env Var:     RCLONE_SFTP_SERVER_COMMAND
- Type:        string
- Default:     ""

#### --sftp-use-fstat

If set use fstat instead of stat

Some servers limit the amount of open files and calling Stat after opening
the file will throw an error from the server. Setting this flag will call
Fstat instead of Stat which is called on an already open file handle.

It has been found that this helps with IBM Sterling SFTP servers which have
"extractability" level set to 1 which means only 1 file can be opened at
any given time.


- Config:      use_fstat
- Env Var:     RCLONE_SFTP_USE_FSTAT
- Type:        bool
- Default:     false

#### --sftp-disable-concurrent-reads

If set don't use concurrent reads

Normally concurrent reads are safe to use and not using them will
degrade performance, so this option is disabled by default.

Some servers limit the amount number of times a file can be
downloaded. Using concurrent reads can trigger this limit, so if you
have a server which returns

    Failed to copy: file does not exist

Then you may need to enable this flag.

If concurrent reads are disabled, the use_fstat option is ignored.


- Config:      disable_concurrent_reads
- Env Var:     RCLONE_SFTP_DISABLE_CONCURRENT_READS
- Type:        bool
- Default:     false

#### --sftp-idle-timeout

Max time before closing idle connections

If no connections have been returned to the connection pool in the time
given, rclone will empty the connection pool.

Set to 0 to keep connections indefinitely.


- Config:      idle_timeout
- Env Var:     RCLONE_SFTP_IDLE_TIMEOUT
- Type:        Duration
- Default:     1m0s

{{< rem autogenerated options stop >}}

### Limitations ###

SFTP supports checksums if the same login has shell access and `md5sum`
or `sha1sum` as well as `echo` are in the remote's PATH.
This remote checksumming (file hashing) is recommended and enabled by default.
Disabling the checksumming may be required if you are connecting to SFTP servers
which are not under your control, and to which the execution of remote commands
is prohibited.  Set the configuration option `disable_hashcheck` to `true` to
disable checksumming.

SFTP also supports `about` if the same login has shell
access and `df` are in the remote's PATH. `about` will
return the total space, free space, and used space on the remote
for the disk of the specified path on the remote or, if not set,
the disk of the root on the remote.
`about` will fail if it does not have shell
access or if `df` is not in the remote's PATH.

Note that some SFTP servers (e.g. Synology) the paths are different for
SSH and SFTP so the hashes can't be calculated properly.  For them
using `disable_hashcheck` is a good idea.

The only ssh agent supported under Windows is Putty's pageant.

The Go SSH library disables the use of the aes128-cbc cipher by
default, due to security concerns. This can be re-enabled on a
per-connection basis by setting the `use_insecure_cipher` setting in
the configuration file to `true`. Further details on the insecurity of
this cipher can be found
[in this paper](http://www.isg.rhul.ac.uk/~kp/SandPfinal.pdf).

SFTP isn't supported under plan9 until [this
issue](https://github.com/pkg/sftp/issues/156) is fixed.

Note that since SFTP isn't HTTP based the following flags don't work
with it: `--dump-headers`, `--dump-bodies`, `--dump-auth`

Note that `--timeout` isn't supported (but `--contimeout` is).


## C14 {#c14}

C14 is supported through the SFTP backend.

See [C14's documentation](https://www.online.net/en/storage/c14-cold-storage)

## rsync.net {#rsync-net}

rsync.net is supported through the SFTP backend.

See [rsync.net's documentation of rclone examples](https://www.rsync.net/products/rclone.html).
