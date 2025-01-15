Rclone's `gitannex` subcommand enables [git-annex] to store and retrieve content
from an rclone remote. It is meant to be run by git-annex, not directly by
users.

[git-annex]: https://git-annex.branchable.com/

Installation on Linux
---------------------

1. Skip this step if your version of git-annex is [10.20240430] or newer.
   Otherwise, you must create a symlink somewhere on your PATH with a particular
   name. This symlink helps git-annex tell rclone it wants to run the "gitannex"
   subcommand.

   ```sh
   # Create the helper symlink in "$HOME/bin".
   ln -s "$(realpath rclone)" "$HOME/bin/git-annex-remote-rclone-builtin"

   # Verify the new symlink is on your PATH.
   which git-annex-remote-rclone-builtin
   ```

   [10.20240430]: https://git-annex.branchable.com/news/version_10.20240430/

2. Add a new remote to your git-annex repo. This new remote will connect
   git-annex with the `rclone gitannex` subcommand.

   Start by asking git-annex to describe the remote's available configuration
   parameters.

   ```sh
   # If you skipped step 1:
   git annex initremote MyRemote type=rclone --whatelse

   # If you created a symlink in step 1:
   git annex initremote MyRemote type=external externaltype=rclone-builtin --whatelse
    ```

   > **NOTE**: If you're porting an existing [git-annex-remote-rclone] remote to
   > use `rclone gitannex`, you can probably reuse the configuration parameters
   > verbatim without renaming them. Check parameter synonyms with `--whatelse`
   > as shown above.
   >
   > [git-annex-remote-rclone]: https://github.com/git-annex-remote-rclone/git-annex-remote-rclone

   The following example creates a new git-annex remote named "MyRemote" that
   will use the rclone remote named "SomeRcloneRemote". That rclone remote must
   be one configured in your rclone.conf file, which can be located with `rclone
   config file`.

   ```sh
   git annex initremote MyRemote         \
       type=external                     \
       externaltype=rclone-builtin       \
       encryption=none                   \
       rcloneremotename=SomeRcloneRemote \
       rcloneprefix=git-annex-content    \
       rclonelayout=nodir
   ```

3. Before you trust this command with your precious data, be sure to **test the
   remote**. This command is very new and has not been tested on many rclone
   backends. Caveat emptor!

   ```sh
   git annex testremote MyRemote
   ```

Happy annexing!
