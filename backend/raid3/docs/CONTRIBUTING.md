# Contributing to the RAID3 Backend

This document outlines how to contribute to the raid3 backend development. For general rclone contribution guidelines, see the main [CONTRIBUTING.md](../../CONTRIBUTING.md) in the rclone repository root.

---

## ⚠️ Important: Pre-MVP Status

The raid3 backend is currently in pre-MVP (pre-Minimum Viable Product) stage and is still under active development. Architecture, API, and behavior are still evolving. Because raid3 is still pre-MVP, all discussions, questions, feature suggestions, and general feedback should be posted on the [rclone forum](https://forum.rclone.org/). Use the forum for questions about how to use the backend, feature suggestions and ideas, general feedback and experiences, architectural discussions, use case discussions, and anything that isn't a clear bug report. This approach helps keep development-focused discussions in one place, gather community feedback before making architectural decisions, avoid fragmenting discussions across GitHub Issues, and maintain focus on reaching MVP status.

---

## 🐛 Bug Reports: Use GitHub Issues

GitHub Issues should only be used for clear, reproducible bug reports. When reporting bugs, please include: rclone version (output from `rclone version`), OS and architecture (e.g., Linux x86_64, macOS ARM64), configuration (raid3 backend config, anonymized if needed), command that triggered the issue, verbose log (output from `rclone -vv ...`), expected vs. actual behavior, and steps to reproduce the issue. If you're unsure whether something is a bug, please ask on the forum first.

---

## 📝 Code Contributions: Not Yet Accepted

Pull requests are not being accepted yet. This allows us to maintain focus on core functionality and stability, ensure architectural decisions are consistent, complete the MVP implementation before accepting code contributions, and avoid merge conflicts and architectural drift during active development. Once the backend reaches MVP status, we will accept pull requests for bug fixes and features, follow the standard rclone contribution guidelines, and review contributions according to rclone's code quality standards. We'll announce when we're ready to accept pull requests on the forum and in release notes.

---

## 🧪 Testing and Feedback

Note: This section will become more relevant as we approach MVP status. For now, the primary focus is on core functionality development. Once we're closer to MVP, we especially appreciate testing on different platforms and storage backends, bug reports with clear reproduction steps (via GitHub Issues), feedback on usability (via the forum), and documentation improvements and clarifications (via GitHub Issues for small fixes, forum for larger discussions). If you're testing the raid3 backend, see [`README.md`](../README.md) for setup and usage instructions, [`TESTING.md`](TESTING.md) for testing documentation, and [`integration/README.md`](../integration/README.md) for bash-based integration tests.

---

## Summary

Post questions, feature suggestions, general feedback, and discussions on the [rclone forum](https://forum.rclone.org/). Post bug reports on GitHub Issues. Pull requests are not accepted yet (pre-MVP). Thank you for your interest in contributing to the raid3 backend! We look forward to your feedback on the forum as we work toward MVP status.
