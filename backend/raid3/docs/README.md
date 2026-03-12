# RAID3 Backend - Detailed Documentation

This directory contains detailed design documents, implementation notes, test results, and research findings for the raid3 RAID 3 backend.

---

## 📚 Essential Documentation

Essential documentation: [../README.md](../README.md) for user-facing documentation and usage guide, [RAID3.md](RAID3.md) for technical RAID 3 specification and implementation details, [TESTING.md](TESTING.md) for complete testing guide (automated tests, manual testing, and bash integration tests).

---

## 📂 Documentation Organization

**Design & research**: Error handling and RAID 3 compliance ([ERROR_HANDLING.md](ERROR_HANDLING.md), [OPEN_QUESTIONS.md](OPEN_QUESTIONS.md)), timeout and performance ([TIMEOUT_MODE.md](TIMEOUT_MODE.md), [STRICT_WRITE_POLICY.md](STRICT_WRITE_POLICY.md)), heal and maintenance ([CLEAN_HEAL.md](CLEAN_HEAL.md)). **Testing**: [TESTING.md](TESTING.md) for complete testing guide and test results. **Contributing**: [PRE_COMMIT_CHECKLIST.md](PRE_COMMIT_CHECKLIST.md) for a detailed pre-commit checklist aligned with rclone CONTRIBUTING.

---

## 🎯 Quick Reference by Topic

To understand how the backend works: start with [../README.md](../README.md), read [RAID3.md](RAID3.md) for technical details. For error handling and design context: [ERROR_HANDLING.md](ERROR_HANDLING.md), [OPEN_QUESTIONS.md](OPEN_QUESTIONS.md) for pending and resolved questions. For bug fixes: [STRICT_WRITE_POLICY.md](STRICT_WRITE_POLICY.md) for critical corruption fix. For testing: [TESTING.md](TESTING.md) for complete testing guide. For S3/MinIO performance: [TIMEOUT_MODE.md](TIMEOUT_MODE.md) for timeout modes configuration.

---

