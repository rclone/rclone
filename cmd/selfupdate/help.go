// +build !noselfupdate

package selfupdate

// Note: "|" will be replaced by backticks in the help string below
var selfUpdateHelp string = `
This command downloads the latest release of rclone and replaces
the currently running binary. The download is verified with a hashsum
and cryptographically signed signature.

If used without flags (or with implied |--stable| flag), this command
will install the latest stable release. However, some issues may be fixed
(or features added) only in the latest beta release. In such cases you should
run the command with the |--beta| flag, i.e. |rclone selfupdate --beta|.
You can check in advance what version would be installed by adding the
|--check| flag, then repeat the command without it when you are satisfied.

Sometimes the rclone team may recommend you a concrete beta or stable
rclone release to troubleshoot your issue or add a bleeding edge feature.
The |--version VER| flag, if given, will update to the concrete version
instead of the latest one. If you omit micro version from |VER| (for
example |1.53|), the latest matching micro version will be used.

Upon successful update rclone will print a message that contains a previous
version number. You will need it if you later decide to revert your update
for some reason. Then you'll have to note the previous version and run the
following command: |rclone selfupdate [--beta] OLDVER|.
If the old version contains only dots and digits (for example |v1.54.0|)
then it's a stable release so you won't need the |--beta| flag. Beta releases
have an additional information similar to |v1.54.0-beta.5111.06f1c0c61|.
(if you are a developer and use a locally built rclone, the version number
will end with |-DEV|, you will have to rebuild it as it obviously can't
be distributed).

If you previously installed rclone via a package manager, the package may
include local documentation or configure services. You may wish to update
with the flag |--package deb| or |--package rpm| (whichever is correct for
your OS) to update these too. This command with the default |--package zip|
will update only the rclone executable so the local manual may become
inaccurate after it.

The |rclone mount| command (https://rclone.org/commands/rclone_mount/) may
or may not support extended FUSE options depending on the build and OS.
|selfupdate| will refuse to update if the capability would be discarded.

Note: Windows forbids deletion of a currently running executable so this
command will rename the old executable to 'rclone.old.exe' upon success.

Please note that this command was not available before rclone version 1.55.
If it fails for you with the message |unknown command "selfupdate"| then
you will need to update manually following the install instructions located
at https://rclone.org/install/
`
