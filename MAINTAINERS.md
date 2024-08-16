# Maintainers guide for rclone #

Current active maintainers of rclone are:

| Name             | GitHub ID         | Specific Responsibilities    |
| :--------------- | :---------------- | :--------------------------  |
| Nick Craig-Wood  | @ncw              | overall project health       |
| Stefan Breunig   | @breunigs         |                              |
| Ishuah Kariuki   | @ishuah           |                              |
| Remus Bunduc     | @remusb           | cache backend                |
| Fabian Möller    | @B4dM4n           |                              |
| Alex Chen        | @Cnly             | onedrive backend             |
| Sandeep Ummadi   | @sandeepkru       | azureblob backend            |
| Sebastian Bünger | @buengese         | jottacloud, yandex & compress backends |
| Ivan Andreev     | @ivandeex         | chunker & mailru backends    |
| Max Sum          | @Max-Sum          | union backend                |
| Fred             | @creativeprojects | seafile backend              |
| Caleb Case       | @calebcase        | storj backend                |
| wiserain         | @wiserain         | pikpak backend               |
| albertony        | @albertony        |                              |
| Chun-Hung Tseng  | @henrybear327     | Proton Drive Backend         |
| Hideo Aoyama     | @boukendesho      | snap packaging               |
| nielash          | @nielash          | bisync                       |
| Dan McArdle      | @dmcardle         | gitannex                     |
| Sam Harrison     | @childish-sambino | filescom                     |

**This is a work in progress Draft**

This is a guide for how to be an rclone maintainer.  This is mostly a write-up of what I (@ncw) attempt to do.

## Triaging Tickets ##

When a ticket comes in it should be triaged.  This means it should be classified by adding labels and placed into a milestone. Quite a lot of tickets need a bit of back and forth to determine whether it is a valid ticket so tickets may remain without labels or milestone for a while.

Rclone uses the labels like this:

* `bug` - a definitely verified bug
* `can't reproduce` - a problem which we can't reproduce
* `doc fix` - a bug in the documentation - if users need help understanding the docs add this label
* `duplicate` - normally close these and ask the user to subscribe to the original
* `enhancement: new remote` - a new rclone backend
* `enhancement` - a new feature
* `FUSE` - to do with `rclone mount` command
* `good first issue` - mark these if you find a small self-contained issue - these get shown to new visitors to the project
* `help` wanted - mark these if you find a self-contained issue - these get shown to new visitors to the project
* `IMPORTANT` - note to maintainers not to forget to fix this for the release
* `maintenance` - internal enhancement, code re-organisation, etc.
* `Needs Go 1.XX` - waiting for that version of Go to be released
* `question` - not a `bug` or `enhancement` - direct to the forum for next time
* `Remote: XXX` - which rclone backend this affects
* `thinking` - not decided on the course of action yet

If it turns out to be a bug or an enhancement it should be tagged as such, with the appropriate other tags.  Don't forget the "good first issue" tag to give new contributors something easy to do to get going.

When a ticket is tagged it should be added to a milestone, either the next release, the one after, Soon or Help Wanted.  Bugs can be added to the "Known Bugs" milestone if they aren't planned to be fixed or need to wait for something (e.g. the next go release).

The milestones have these meanings:

* v1.XX - stuff we would like to fit into this release
* v1.XX+1 - stuff we are leaving until the next release
* Soon - stuff we think is a good idea - waiting to be scheduled for a release
* Help wanted - blue sky stuff that might get moved up, or someone could help with
* Known bugs - bugs waiting on external factors or we aren't going to fix for the moment

Tickets [with no milestone](https://github.com/rclone/rclone/issues?utf8=✓&q=is%3Aissue%20is%3Aopen%20no%3Amile) are good candidates for ones that have slipped between the gaps and need following up.

## Closing Tickets ##

Close tickets as soon as you can - make sure they are tagged with a release.  Post a link to a beta in the ticket with the fix in, asking for feedback.

## Pull requests ##

Try to process pull requests promptly!

Merging pull requests on GitHub itself works quite well nowadays so you can squash and rebase or rebase pull requests.  rclone doesn't use merge commits.  Use the squash and rebase option if you need to edit the commit message.

After merging the commit, in your local master branch, do `git pull` then run `bin/update-authors.py` to update the authors file then `git push`.

Sometimes pull requests need to be left open for a while - this especially true of contributions of new backends which take a long time to get right.

## Merges ##

If you are merging a branch locally then do `git merge --ff-only branch-name` to avoid a merge commit.  You'll need to rebase the branch if it doesn't merge cleanly.

## Release cycle ##

Rclone aims for a 6-8 week release cycle.  Sometimes release cycles take longer if there is something big to merge that didn't stabilize properly or for personal reasons.

High impact regressions should be fixed before the next release.

Near the start of the release cycle, the dependencies should be updated with `make update` to give time for bugs to surface.

Towards the end of the release cycle try not to merge anything too big so let things settle down.

Follow the instructions in RELEASE.md for making the release. Note that the testing part is the most time-consuming often needing several rounds of test and fix depending on exactly how many new features rclone has gained.

## Mailing list ##

There is now an invite-only mailing list for rclone developers `rclone-dev` on google groups.

## TODO ##

I should probably make a dev@rclone.org to register with cloud providers.
