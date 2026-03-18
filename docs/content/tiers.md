---
title: "Backend Support Tiers"
description: "A complete list of supported backends and their stability tiers."
---

# Tiers

Rclone backends are divided into tiers to give users an idea of the stability of each backend.

| Tier   | Label         | Intended meaning |
|--------|---------------|------------------|
| {{< tier tier="Tier 1" >}} | Core          | Production-grade, first-class |
| {{< tier tier="Tier 2" >}} | Stable        | Well-supported, minor gaps |
| {{< tier tier="Tier 3" >}} | Supported     | Works for many uses; known caveats |
| {{< tier tier="Tier 4" >}} | Experimental  | Use with care; expect gaps/changes |
| {{< tier tier="Tier 5" >}} | Deprecated    | No longer maintained or supported |

## Overview

Here is a summary of all backends:

{{< tiers-table >}}

## Scoring

Here is how the backends are scored.

### Features

These are useful optional features a backend should have in rough
order of importance. Each one of these scores a point for the Features
column.

- F1: Hash(es)
- F2: Modtime
- F3: Stream upload
- F4: Copy/Move
- F5: DirMove	
- F6: Metadata
- F7: MultipartUpload


### Tier

The tier is decided after determining these attributes. Some discretion is allowed in tiering as some of these attributes are more important than others.

| Attr | T1: Core | T2: Stable | T3: Supported | T4: Experimental | T5: Incubator |
|------|----------|------------|---------------|------------------|---------------|
| Maintainers | >=2 | >=1 | >=1 | >=0 | >=0 |
| API source | Official | Official | Either | Either | Either |
| Features (F1-F7) | >=5/7 | >=4/7 | >=3/7 | >=2/7 | N/A |
| Integration tests | All Green | All green | Nearly all green | Some Flaky | N/A |
| Error handling | Pacer | Pacer | Retries | Retries | N/A |
| Data integrity | Hashes, alt, modtime | Hashes or alt | Hash OR modtime | Best-effort | N/A |
| Perf baseline | Bench within 2x S3 | Bench doc | Anecdotal OK | Optional | N/A |
| Adoption | widely used | often used | some use | N/A | N/A |
| Docs completeness | Full | Full | Basic | Minimal | Minimal |
| Security | Principle-of-least-privilege | Reasonable scopes | Basic auth | Works | Works |
