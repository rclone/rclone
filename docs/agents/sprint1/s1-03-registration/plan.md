---
type: plan
story: S1-03
scope: "tests only"
---

# S1-03 Plan (tests only) — Registration, docs, test stubs

Tests ONLY. For S1-03 the "tests" are the verification commands and the integration
`TestIntegration` stubs themselves — there is no new production logic, only wiring.
Each checkbox is one verification, one line. The grep/build/yaml checks are run as a
single `gcalfs/gmailfs` wiring test (or directly by the validator); the
`TestIntegration` items are real Go tests that must SKIP without credentials.

## T01 — blank imports in all.go

- [ ] `wiring_test (grep) ::AllGoHasBothImports` — `grep -c 'backend/gmailfs\|backend/gcalfs' backend/all/all.go` returns 2.
- [ ] `build ::GoBuildAllSucceeds` — `go build ./...` exits 0 (proves blank imports resolve to real, compiling packages — depends on S1-01 + S1-02).
- [ ] `wiring_test (git) ::AllGoNoReorderOrRemoval` — `git diff backend/all/all.go` shows only two added lines, zero removals.

## T02 — gmailfs doc page

- [ ] `doc_test ::GmailfsDocExists` — `docs/content/gmailfs.md` exists.
- [ ] `doc_test ::GmailfsDocStatesRequiredCredentials` — page mentions both `client_id` and `client_secret` as required with no default credential.
- [ ] `doc_test ::GmailfsDocHasLimitationsReadOnlyNoHash` — page has a Limitations section noting read-only and no hash support.
- [ ] `doc_test ::GmailfsDocDocumentsStartYear` — page documents the `start_year` option.
- [ ] `doc_test ::GmailfsDocNoWriteCapability` — page contains no statement that uploads/writes work.

## T03 — gcalfs doc page

- [ ] `doc_test ::GcalfsDocExists` — `docs/content/gcalfs.md` exists.
- [ ] `doc_test ::GcalfsDocStatesRequiredCredentials` — page mentions both `client_id` and `client_secret` as required with no default credential.
- [ ] `doc_test ::GcalfsDocHasLimitationsReadOnlyNoHash` — page has a Limitations section noting read-only and no hash support.
- [ ] `doc_test ::GcalfsDocDocumentsStartYear` — page documents the `start_year` option.
- [ ] `doc_test ::GcalfsDocNoWriteCapability` — page contains no statement that writes work.

## T04 — integration test stubs

- [ ] `backend/gmailfs/gmailfs_test.go::TestIntegration` — calls `fstest.Initialise()`, defaults `*fstest.RemoteName` to `"TestGmailFs:"`, `t.Skipf`s on `fs.ErrorNotFoundInConfigFile`, and SKIPs cleanly without credentials.
- [ ] `backend/gcalfs/gcalfs_test.go::TestIntegration` — same pattern defaulting to `"TestGcalFs:"`; SKIPs cleanly without credentials.
- [ ] `backend/gmailfs/gmailfs_test.go::TestIntegration/WriteOpsReturnError` — when configured, every write op returns an error (sub-test; guarded behind the skip).
- [ ] `test-skip ::BothIntegrationTestsSkipNoFail` — `go test ./backend/gmailfs/ ./backend/gcalfs/ -run TestIntegration -v` shows SKIP and no `--- FAIL` lines without credentials.

## T05 — config.yaml entries

- [ ] `config_test ::ConfigYamlHasBothRemotes` — `grep -c 'TestGmailFs:\|TestGcalFs:' fstest/test_all/config.yaml` returns 2.
- [ ] `config_test ::ConfigYamlHasBothBackendKeys` — `grep -c '"gmailfs"\|"gcalfs"' fstest/test_all/config.yaml` returns 2.
- [ ] `config_test ::ConfigYamlStillParses` — `python3 -c "import yaml; yaml.safe_load(open('fstest/test_all/config.yaml'))"` exits 0.
- [ ] `config_test ::ConfigYamlNoDuplicateEntries` — neither remote prefix appears more than once.

## T06 — backend data files, site navigation, index entries

- [ ] `backend/gmailfs/integration_test.go::TestRegistrationFilePresence` — grep all 7 registration files for both backend names; assert count ≥ 1 per backend per file.
