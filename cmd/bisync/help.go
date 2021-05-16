package bisync

import (
	"strconv"
	"strings"
)

func makeHelp(help string) string {
	replacer := strings.NewReplacer(
		"|", "`",
		"{MAXDELETE}", strconv.Itoa(DefaultMaxDelete),
		"{CHECKFILE}", DefaultCheckFilename,
		"{WORKDIR}", DefaultWorkdir,
	)
	return replacer.Replace(help)
}

var shortHelp = `Perform bidirectonal synchronization between two paths.`

var rcHelp = makeHelp(`This takes the following parameters

- path1 - a remote directory string e.g. |drive:path1|
- path2 - a remote directory string e.g. |drive:path2|
- dryRun - dry-run mode
- resync - performs the resync run
- checkAccess - abort if {CHECKFILE} files are not found on both filesystems
- checkFilename - file name for checkAccess (default: {CHECKFILE})
- maxDelete - abort sync if percentage of deleted files is above
  this threshold (default: {MAXDELETE})
- force - maxDelete safety check and run the sync
- checkSync - |true| by default, |false| disables comparison of final listings,
              |only| will skip sync, only compare listings from the last run
- removeEmptyDirs - remove empty directories at the final cleanup step
- filtersFile - read filtering patterns from a file
- workdir - server directory for history files (default: {WORKDIR})
- noCleanup - retain working files

See [bisync command help](https://rclone.org/commands/rclone_bisync/)
and [full bisync description](https://rclone.org/bisync/)
for more information.`)

var longHelp = shortHelp + makeHelp(`

[Bisync](https://rclone.org/bisync/) provides a
bidirectional cloud sync solution in rclone.
It retains the Path1 and Path2 filesystem listings from the prior run.
On each successive run it will:
- list files on Path1 and Path2, and check for changes on each side.
  Changes include |New|, |Newer|, |Older|, and |Deleted| files.
- Propagate changes on Path1 to Path2, and vice-versa.

See [full bisync description](https://rclone.org/bisync/) for details.
`)
