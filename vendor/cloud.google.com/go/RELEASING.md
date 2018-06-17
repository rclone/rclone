# How to Release this Repo

1. Determine the current release version with `git tag -l`. It should look
   something like `vX.Y.Z`. We'll call the current
   version `$CV` and the new version `$NV`.
1. On master, run `git log $CV..` to list all the changes since the last
   release.
1. Edit the News section of `README.md` to include a summary of the changes.
1. Mail the CL containing the `README.md` changes. When the CL is approved, submit it.
1. Without submitting any other CLs:
   a. Switch to master.
   b. Tag the repo with the next version: `git tag $NV`.
   c. Push the tag: `git push origin $NV`.
