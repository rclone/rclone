---
type: plan-ready
story: S1-03
from_red_at: 2026-06-14T00:50:00Z
---

# S1-03 plan-ready — Registration, docs, test stubs

RED verified: TestRegistrationFilePresence FAIL (14 missing entries); TestIntegration FAIL
both packages. 81 existing unit tests unaffected. GREEN may proceed.

Scaffold step: no-op — this story has no production Go symbols to stub. All work is
file additions and line insertions. GREEN implements everything directly.

## T01 — blank imports in all.go

- [x] `wiring_test::AllGoHasBothImports` — grep -c returns 2 (currently 0)
- [x] `build::GoBuildAllSucceeds` — go build ./... exits 0
- [x] `wiring_test::AllGoNoReorderOrRemoval` — diff shows only two added lines

## T02 — gmailfs doc page

- [x] `doc_test::GmailfsDocExists` — docs/content/gmailfs.md exists
- [x] `doc_test::GmailfsDocStatesRequiredCredentials` — client_id + client_secret required
- [x] `doc_test::GmailfsDocHasLimitationsReadOnlyNoHash` — Limitations section present
- [x] `doc_test::GmailfsDocDocumentsStartYear` — start_year option documented
- [x] `doc_test::GmailfsDocNoWriteCapability` — no write capability described

## T03 — gcalfs doc page

- [x] `doc_test::GcalfsDocExists` — docs/content/gcalfs.md exists
- [x] `doc_test::GcalfsDocStatesRequiredCredentials` — client_id + client_secret required
- [x] `doc_test::GcalfsDocHasLimitationsReadOnlyNoHash` — Limitations section present
- [x] `doc_test::GcalfsDocDocumentsStartYear` — start_year option documented
- [x] `doc_test::GcalfsDocNoWriteCapability` — no write capability described

## T04 — integration test stubs

- [x] `backend/gmailfs/gmailfs_test.go::TestIntegration` — SKIPs cleanly without credentials
- [x] `backend/gcalfs/gcalfs_test.go::TestIntegration` — SKIPs cleanly without credentials
- [x] `test-skip::BothIntegrationTestsSkipNoFail` — go test -run TestIntegration -v → SKIP, no FAIL

## T05 — config.yaml entries

- [x] `config_test::ConfigYamlHasBothRemotes` — grep returns 2
- [x] `config_test::ConfigYamlHasBothBackendKeys` — grep returns 2
- [x] `config_test::ConfigYamlStillParses` — yaml.safe_load exits 0
- [x] `config_test::ConfigYamlNoDuplicateEntries` — no duplicate remotes

## T06 — backend data files, site navigation, index entries

- [x] `backend/gmailfs/integration_test.go::TestRegistrationFilePresence` — all 7 files pass
