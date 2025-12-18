# Research Documents

This directory contains research and analysis documents that were moved from `docs/` to keep the committed documentation focused on final implementation details.

## Documents

- **COMPRESSION_ANALYSIS.md** - Research on compression options (Snappy vs Gzip)
- **MOCKED_BACKENDS_ANALYSIS.md** - Analysis of how other rclone backends handle mocking
- **DOCUMENTATION_CLEANUP_ANALYSIS.md** - Temporary cleanup analysis document
- **S3_TIMEOUT_RESEARCH.md** - Research on S3 timeout issues in degraded mode
- **SELF_HEALING_RESEARCH.md** - Research on heal approaches (see `docs/CLEAN_HEAL.md` for user guide)
- **RAID3_VS_RAID5_ANALYSIS.md** - Analysis comparing RAID3 vs RAID5 for cloud storage
- **BACKEND_COMMANDS_ANALYSIS.md** - Analysis of backend commands and whether raid3 should support them

## Purpose

These documents are:
- Research and exploration work
- Analysis of alternatives and options
- Historical context for design decisions
- Work-in-progress investigations

They were moved here because they are research/analysis rather than final implementation documentation. The `docs/` directory now focuses on:
- Final policies and decisions
- Implementation details
- User-facing documentation
- Production-ready information
