# How to release a new version

## When to release a new version

New version should be released when we are ready to make changes generally available.

New version should not be released if we want to test our latest changes or to make them available to a limited number of users. This can be achieved without releasing a new version. Go modules allow to point to latest master or specific Git commit. However, commits from master are not suitable for use in production unless they are a release tag. Do not use non-release commits in downstream projects in production.

Consider releasing a new Release Candidate version to make changes available to a larger group of users, if we are not ready to make them available to everyone yet.

Under no circumstances may releases be done during the weekend or the wee hours of the night.

## Version numbers

We follow the rules for semantic versioning, but prefixed with the letter `v`.

Examples of official releases:
- `v1.0.0`
- `v1.0.3`
- `v1.2.3`
- `v2.1.7`

Examples of Release Candidates:
- `v1.0.0-rc.4`
- `v2.1.0-rc.1`

## Step-by-step release process

1. Clone or fetch latest master of these Git repositories:
  - https://github.com/storj/storj
  - https://github.com/storj/gateway
  - https://github.com/storj/uplink-c
  - https://github.com/storj/linksharing
2. For each of them update, `go.mod` and `testsuite/go.mod` to latest master (or the specific Git commit that will be tagged as a new version) of `storj.io/uplink`. Makefile target `bump-dependencies` (available in all listed repositories) can bo used to do this automatically.
3. Use `go mod tidy` to update the respective `go.sum` files.
4. Push a change to Gerrit with the updated `go.mod` and `go.sum` files for each of the Git repositories.
5. Wait for the build to finish. If the build fails for any of the Git repositories, abort the release process. Investigate the issue, fix it, and start over the release process.
6. If all builds are successful, do not merge the changes yet.
7. If you haven't done this yet, announce your intention to make a new release to the #libuplink Slack channel.
8. Wait for a confirmation by at least one maintainer of this project (storj/uplink) before proceeding with the next step.
9. Create a new release from the Github web interface:
  - Go to https://github.com/storj/uplink/releases.
  - Click the `Draft a new release` button.
  - Enter `Tag version` following the rules for the version number, e.g. `v1.2.3`.
  - Enter the same value as `Release title`, e.g. `v1.2.3`.
  - If there are new commits in master since you executed step 1, do not include them in the release. Change the `Target` from `master` to the specific Git commit used in step 1.
  - Describe the changes since the previous release in a human-readable way. Only those changes that affect users. No need to describe refactorings, etc.
  - If you are releasing a new Release Candidate, select the `This is a pre-release` checkbox.
  - Click the `Publish release` button.
10. Update the Gerrit changes from step 1 with the new official version number.
11. Wait for the build to finish again and merge them.
