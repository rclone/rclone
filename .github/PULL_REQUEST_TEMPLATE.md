<!--
Thank you for contributing to rclone!

Please discuss anything more than a trivial fix in an issue FIRST - see the
"Linked issue" section below. Then fill in the questions to help us review.
-->

#### What does this change do?

<!-- Describe the change here. -->

#### Linked issue

<!--
Put `Fixes #1234` here if this closes an issue, or `#1234` to link without closing.

IMPORTANT: Anything beyond a trivial fix (typo, doc tweak, small obvious bug fix)
should be discussed and agreed in an issue BEFORE you open a pull request.
Pull requests for larger changes that have not been discussed in an issue first
may be closed. This saves everyone's time if the change needs a different approach
or isn't a good fit.
-->

Fixes #

#### For new or changed backends

<!--
Backend pull requests are often sent without the integration tests having been run.
We cannot merge a backend change until it passes the integration tests.

To be merged, a backend change REQUIRES:
  1. A clean run of `go run ./fstest/test_all -backends <remote>` (summarise the
     result below or paste a screenshot of the test_all output in the web browser).
  2. A test account so the maintainers can run the integration tests daily against
     the backend on the integration test server.

If the tests aren't quite passing yet and you need help getting them
to pass, then submit the PR and we can help getting you over the line.

See https://github.com/rclone/rclone/blob/master/CONTRIBUTING.md#writing-a-new-backend
and https://github.com/rclone/rclone/blob/master/CONTRIBUTING.md#integration-tests
-->

#### Checklist

- [ ] This change is trivial **OR** it has been discussed and agreed in the linked issue.
- [ ] I have read the [contribution guidelines](https://github.com/rclone/rclone/blob/master/CONTRIBUTING.md#submitting-a-new-feature-or-bug-fix).
- [ ] **(If I used AI tools to help write this code)** I have read and understood the [AI-assisted contributions guidance](https://github.com/rclone/rclone/blob/master/CONTRIBUTING.md#ai-assisted-contributions), and I have tested and take ownership of this change myself.
- [ ] I have added tests for all changes in this PR if appropriate.
- [ ] I have added documentation for the changes if appropriate.
- [ ] All commit messages are in [house style](https://github.com/rclone/rclone/blob/master/CONTRIBUTING.md#commit-messages).
- [ ] **(Backend changes only)** `test_all` passes for this backend and if submitting a new backend can provide a test account for the integration tester - see [CONTRIBUTING.md](https://github.com/rclone/rclone/blob/master/CONTRIBUTING.md#integration-tests).
- [ ] This Pull Request is ready for review.
