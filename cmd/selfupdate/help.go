package selfupdate

// Note: "|" will be replaced by backticks in the help string below
var selfUpdateHelp string = `
This command downloads the latest release of rclone and replaces
the currently running binary. The download is verified with a hashsum
and cryptographically signed signature.

The |--version VER| flag, if given, will update to a concrete version
instead of the latest one. If you omit micro version from |VER| (for
example |1.53|), the latest matching micro version will be used.

If you previously installed rclone via a package manager, the package may
include local documentation or configure services. You may wish to update
with the flag |--package deb| or |--package rpm| (whichever is correct for
your OS) to update these too. This command with the default |--package zip|
will update only the rclone executable so the local manual may become
inaccurate after it.

Note: Windows forbids deletion of a currently running executable so this
command will rename the old executable to 'rclone.old.exe' upon success.
`
