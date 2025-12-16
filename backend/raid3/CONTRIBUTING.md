# Contributing to the RAID3 Backend

## Purpose of This Document

This document outlines how to contribute to the **raid3 backend** development. For general rclone contribution guidelines, see the main [CONTRIBUTING.md](../../CONTRIBUTING.md) in the rclone repository root.

---

## ⚠️ Important: Pre-MVP Status

**The raid3 backend is currently in pre-MVP (pre-Minimum Viable Product) stage** and is still under active development. This means the architecture, API, and behavior are still evolving.

### 🗣️ **Discussions: Use the rclone Forum**

**Because raid3 is still pre-MVP, all discussions, questions, feature suggestions, and general feedback should be posted on the [rclone forum](https://forum.rclone.org/).**

Please use the forum for:
- **Questions** about how to use the backend
- **Feature suggestions** and ideas
- **General feedback** and experiences
- **Architectural discussions**
- **Use case discussions**
- **Anything that isn't a clear bug report**

This approach helps us:
- Keep development-focused discussions in one place
- Gather community feedback before making architectural decisions
- Avoid fragmenting discussions across GitHub Issues
- Maintain focus on reaching MVP status

---

## 🐛 Bug Reports: Use GitHub Issues

**GitHub Issues should only be used for clear, reproducible bug reports.**

When reporting bugs, please include:

- **Rclone version** (output from `rclone version`)
- **OS and architecture** (e.g., Linux x86_64, macOS ARM64)
- **Configuration** (raid3 backend config, anonymized if needed)
- **Command** that triggered the issue
- **Verbose log** (output from `rclone -vv ...`)
- **Expected vs. actual behavior**
- **Steps to reproduce** the issue

**If you're unsure whether something is a bug**, please ask on the forum first.

---

## 📝 Code Contributions: Not Yet Accepted

**Pull requests are not being accepted yet.**

This allows us to:
- Maintain focus on core functionality and stability
- Ensure architectural decisions are consistent
- Complete the MVP implementation before accepting code contributions
- Avoid merge conflicts and architectural drift during active development

### Future Contribution Process

Once the backend reaches MVP status, we will:
- Accept pull requests for bug fixes and features
- Follow the standard rclone contribution guidelines
- Review contributions according to rclone's code quality standards

We'll announce when we're ready to accept pull requests on the forum and in release notes.

---

## 🧪 Testing and Feedback

> **Note**: This section will become more relevant as we approach MVP status. For now, the primary focus is on core functionality development.

Once we're closer to MVP, we especially appreciate:
- **Testing** on different platforms and storage backends
- **Bug reports** with clear reproduction steps (via GitHub Issues)
- **Feedback** on usability (via the forum)
- **Documentation** improvements and clarifications (via GitHub Issues for small fixes, forum for larger discussions)

If you're testing the raid3 backend, see:
- [`README.md`](README.md) - Setup and usage instructions
- [`TESTING.md`](TESTING.md) - Testing documentation
- [`integration/README.md`](integration/README.md) - Bash-based integration tests

---

## Summary

| Type of Contribution | Where to Post |
|---------------------|---------------|
| **Questions** | [rclone forum](https://forum.rclone.org/) |
| **Feature suggestions** | [rclone forum](https://forum.rclone.org/) |
| **General feedback** | [rclone forum](https://forum.rclone.org/) |
| **Discussions** | [rclone forum](https://forum.rclone.org/) |
| **Bug reports** | GitHub Issues |
| **Pull requests** | Not accepted yet (pre-MVP) |

---

**Thank you for your interest in contributing to the raid3 backend!** 🎯

We look forward to your feedback on the forum as we work toward MVP status.
