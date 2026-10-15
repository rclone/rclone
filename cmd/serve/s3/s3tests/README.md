# s3-tests for rclone serve s3

This runs the [ceph/s3-tests](https://github.com/ceph/s3-tests) S3
compatibility suite against `rclone serve s3`.

It needs `git`, `go` and `python3` (with the `venv` module). Run it with

```sh
cmd/serve/s3/s3tests/run.sh
```

This fetches s3-tests at a pinned commit, makes a Python venv with the
pinned `requirements.txt` (both cached in `~/.cache/rclone-s3tests`),
builds rclone from this tree and serves a temporary local directory
with the three users s3-tests needs. A full run takes about 10 minutes.

To run a subset, pass test ids or pytest options

```sh
cmd/serve/s3/s3tests/run.sh s3tests/functional/test_s3.py::test_bucket_list_empty
cmd/serve/s3/s3tests/run.sh -k multipart
```

To test a different remote or flags

```sh
RCLONE_REMOTE=:memory: cmd/serve/s3/s3tests/run.sh
RCLONE_ARGS="--vfs-cache-mode full" cmd/serve/s3/s3tests/run.sh
```

See the top of `run.sh` for the other environment variables.

## Results

Results go to a temporary directory (or `$S3TESTS_OUT`) which has

- `pytest.log` - the full pytest output, with tracebacks
- `junit.xml` - machine readable results
- `rclone.log` - the log from `rclone serve s3`
- `report.txt` - the summary printed at the end

The summary shows pass/fail/skip counts by feature area and the
commonest failures.

Many tests fail because they test features `serve s3` doesn't
implement (ACLs, versioning, lifecycle, bucket policies, encryption,
object lock, etc), so rather than tracking the failures,
`expected-pass.txt` lists the tests which should pass. The run fails
if any of them don't, and lists any other tests which now pass. After
fixing something, add the newly passing tests with

```sh
cmd/serve/s3/s3tests/run.sh --update
```

and commit the change to `expected-pass.txt` with the fix.

## Files

- `run.sh` - the test runner
- `report.py` - summarises the results and checks them against `expected-pass.txt`
- `rclone_shim.py` - pytest plugin adapting the s3-tests cleanup to
  `serve s3` (it doesn't change what the tests test)
- `s3tests.conf.in` - the s3-tests config
- `requirements.txt` - pinned Python dependencies
- `expected-pass.txt` - the tests which should pass

The s3-tests suite has moved into the ceph repository
(`src/test/rgw/s3-tests` in [ceph/ceph](https://github.com/ceph/ceph)).
`run.sh` uses the final commit of the standalone repository.
