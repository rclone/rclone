Rclone's gitannex subcommand enables git-annex to store and retrieve content
from an rclone remote. It expects to be run by git-annex, not directly by users.
It is an "external special remote program" as defined by git-annex.

Installation on Linux
---------------------

1. Create a symlink and ensure it's on your PATH. For example:

        ln -s "$(realpath rclone)" "$HOME/bin/git-annex-remote-rclone-builtin"

2. Add a new external remote to your git-annex repo.

   The new remote's type should be "rclone-builtin". When git-annex interacts
   with remotes of this type, it will try to run a command named
   "git-annex-remote-rclone-builtin", so the symlink from the previous step
   should be on your PATH.

   * NOTE: If you are porting a remote from git-annex-remote-rclone, first
     change the externaltype from "rclone" to "rclone-builtin". Watch out, our
     configs have slightly different names:

      | config for "rclone" | config for "rclone-builtin" |
      |:--------------------|:----------------------------|
      | `prefix`            | `rcloneprefix`              |
      | `target`            | `rcloneremotename`          |
      | `rclone_layout`     | `rclonelayout`              |

   The following example creates a new git-annex remote named "MyRemote" that
   will use the rclone remote named "SomeRcloneRemote". This rclone remote must
   be configured in your rclone.conf file, wherever that is located on your
   system. The rcloneprefix value ensures that content is only written into the
   rclone remote underneath the "git-annex-content" directory.

        git annex initremote MyRemote         \
            type=external                     \
            externaltype=rclone-builtin       \
            encryption=none                   \
            rcloneremotename=SomeRcloneRemote \
            rcloneprefix=git-annex-content    \
            rclonelayout=nodir

3. Before you trust this command with your precious data, be sure to **test the
   remote**. This command is very new and has not been tested on many rclone
   backends. Caveat emptor!

        git annex testremote my-rclone-remote

Happy annexing!
