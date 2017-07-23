# Maintainers guide for rclone #

Current active maintainers of rclone are

  * Nick Craig-Wood
  * Stefan Breunig

**This is a work in progress Draft**

This is a guide for how to be an rclone maintainer.

## Triaging Tickets ##

***FIXME*** this section needs some work!

When a ticket comes in it should be triaged.  This means it should be classified into a bug or an enhancement or a request for support.

Quite a lot of tickets need a bit of back and forth to determine whether it is a valid ticket.

If it turns out to be a bug or an enhancement it should be tagged as such, with the appropriate other tags.  Don't forget the "quickie" tag to give new contributors something easy to do to get going.

When a ticket is tagged it should be added to a milestone, either the next release, the one after, Soon or Unplanned.  Bugs can be added to the "Known Bugs" milestone if they aren't planned to be fixed or need to wait for something (eg the next go release).

***FIXME*** I don't think I've quite got the milestone thing sorted yet. I was wondering about classifying them into priority, or what?

Tickets [with no milestone](https://github.com/ncw/rclone/issues?utf8=✓&q=is%3Aissue%20is%3Aopen%20no%3Amile) are good candidates for ones that have slipped between the gaps and need following up.

## Closing Tickets ##

Close tickets as soon as you can - make sure they are tagged with a release.  Post a link to a beta in the ticket with the fix in, asking for feedback.

## Pull requests ##

Try to process pull requests promptly!

Merging pull requests on Github itself works quite well now-a-days so you can squash and rebase or rebase pull requests.  rclone doesn't use merge commits.  Use the squash and rebase option if you need to edit the commit message.

After merging the commit, in your local master branch, do `git pull` then run `bin/update-authors.py` to update the authors file then `git push`.

Sometimes pull requests need to be left open for a while - this especially true of contributions of new backends which take a long time to get right.

## Merges ##

If you are merging a branch locally then do `git merge --ff-only branch-name` to avoid a merge commit.  You'll need to rebase the branch if it doesn't merge cleanly.

## Release cycle ##

Rclone aims for a 6-8 week release cycle.  Sometimes release cycles take longer if there is something big to merge that didn't stabilize properly or for personal reasons.

High impact regressions should be fixed before the next release.

Near the start of the release cycle the dependencies should be updated with `make update` to give time for bugs to surface.

Towards the end of the release cycle try not to merge anything too big so let things settle down.

Follow the instructions in RELEASE.md for making the release. Note that the testing part is the most time consuming often needing several rounds of test and fix depending on exactly how many new features rclone has gained.

## TODO ##

I should probably make a mailing list for maintainers or at least an rclone-dev list, and I should probably make a dev@rclone.org to register with cloud providers.
