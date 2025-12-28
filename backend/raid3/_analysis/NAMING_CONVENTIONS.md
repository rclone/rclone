# Naming Conventions - RAID3 Backend

This document outlines the naming conventions used in the raid3 backend to ensure consistency and maintainability.

## General Principles

1. **Follow Go naming conventions** - Exported names start with uppercase, unexported names start with lowercase
2. **Be descriptive** - Names should clearly indicate purpose
3. **Be consistent** - Use the same naming pattern for similar concepts
4. **Follow rclone patterns** - Match conventions used in other rclone backends where applicable

## Package-Level Constants

### Timeout Constants
- Pattern: `{mode}InitTimeout`, `{operation}Timeout`
- Examples: `aggressiveInitTimeout`, `balancedInitTimeout`, `standardInitTimeout`, `healthCheckTimeout`
- All unexported (lowercase) as they are internal implementation details

### Size Constants
- Pattern: `{type}ChunkSize`, `min{Type}ChunkSize`, `default{Type}ChunkSize`
- Examples: `defaultChunkSize`, `minChunkSize`, `minReadChunkSize`
- All unexported (lowercase) as they are internal implementation details

### Worker Constants
- Pattern: `{operation}Workers`, `default{Type}Workers`
- Examples: `defaultUploadWorkers`, `rebuildWorkers`
- All unexported (lowercase) as they are internal implementation details

### Queue Constants
- Pattern: `default{Type}QueueSize`
- Examples: `defaultUploadQueueSize`
- All unexported (lowercase) as they are internal implementation details

## Type Names

### Struct Types
- Pattern: `{purpose}{Type}` (camelCase, unexported) or `{Purpose}{Type}` (PascalCase, exported)
- Examples: `uploadQueue`, `uploadJob`, `particleObjectInfo`, `moveState`, `moveResult`
- Unexported types use lowercase, exported types use PascalCase

### Interface Types
- Follow Go conventions: exported interfaces use PascalCase
- Examples: `fs.Fs`, `fs.Object`, `fs.ObjectInfo`

## Function Names

### Exported Functions (Public API)
- Pattern: PascalCase
- Examples: `NewFs`, `SplitBytes`, `MergeBytes`, `CalculateParity`

### Unexported Functions (Internal)
- Pattern: camelCase
- Examples: `putBuffered`, `putStreaming`, `openBuffered`, `openStreaming`, `updateBuffered`, `updateStreaming`

### Helper Functions
- Pattern: `{verb}{Noun}` or `{operation}{Type}`
- Examples: `validateRemote`, `validateContext`, `formatOperationError`, `formatParticleError`, `formatBackendError`
- All unexported (lowercase) as they are internal helpers

## Variable Names

### Backend References
- Pattern: `{backendType}` (lowercase, descriptive)
- Examples: `even`, `odd`, `parity`, `evenFs`, `oddFs`, `parityFs`

### Error Variables
- Pattern: `err{Type}` or `{operation}Err`
- Examples: `errEven`, `errOdd`, `errParity`, `err`, `copyErr`, `moveErr`

### Object Variables
- Pattern: `{type}Obj` or `{type}Object`
- Examples: `evenObj`, `oddObj`, `parityObj`, `srcObj`, `dstObj`

### Boolean Variables
- Pattern: `{condition}Exists`, `is{Condition}`, `has{Feature}`
- Examples: `evenExists`, `oddExists`, `parityExists`, `isOddLength`, `isCrossRemote`, `hasMkdirMetadata`

### Counter Variables
- Pattern: `{type}Count`, `{operation}Count`
- Examples: `evenCount`, `oddCount`, `parityCount`, `rebuiltCount`, `cleanedCount`

### Size Variables
- Pattern: `{type}Size`, `{operation}Size`
- Examples: `evenSize`, `oddSize`, `paritySize`, `totalSize`, `rebuiltSize`

## Channel Names

- Pattern: `{purpose}Ch` or `{purpose}Chan`
- Examples: `listCh`, `results`, `jobs` (when context is clear)

## Map and Slice Names

- Pattern: `{type}s` or `{type}List` or `{type}Map`
- Examples: `uploadedParticles`, `tempParticles`, `brokenObjects`, `entries`, `moves`

## Context Variables

- Pattern: `ctx`, `gCtx` (for errgroup context)
- Examples: `ctx`, `gCtx`, `uploadCtx`

## Backend-Specific Naming

### Particle Types
- Use lowercase strings: `"even"`, `"odd"`, `"parity"`
- Variable names: `particleType`, `targetType`

### Remote Paths
- Pattern: `{purpose}Remote` or `{type}Remote`
- Examples: `remote`, `srcRemote`, `dstRemote`, `tempRemote`, `parityRemote`

### Parity Filenames
- Pattern: `{base}ParityName` or `parityName`
- Examples: `parityName`, `srcParityName`, `dstParityName`, `oldParityName`

## Error Message Formatting

Error messages use standardized formatting functions:
- `formatOperationError(operation, context, err)` - General operation errors
- `formatParticleError(backend, particleType, operation, context, err)` - Particle-specific errors
- `formatBackendError(backend, operation, context, err)` - Backend-specific errors
- `formatNotFoundError(backend, resource, context, err)` - Not found errors

Format: `"{backend}: {operation} failed: {context}: {error}"`

## File Naming

- Go files: `{purpose}.go` (lowercase, descriptive)
- Examples: `raid3.go`, `object.go`, `operations.go`, `list.go`, `health.go`, `metadata.go`, `commands.go`, `particles.go`, `helpers.go`, `heal.go`, `constants.go`

## Test Naming

- Pattern: `Test{Feature}` or `Test{Operation}{Scenario}`
- Examples: `TestStandard`, `TestConcurrentOperations`, `TestPutFailsWithUnavailableBackend`, `TestHealCommandSingleFile`

## Summary

- **Constants**: lowercase, descriptive (e.g., `defaultChunkSize`)
- **Types**: lowercase for unexported (e.g., `uploadQueue`), PascalCase for exported
- **Functions**: camelCase for unexported (e.g., `putBuffered`), PascalCase for exported
- **Variables**: camelCase, descriptive (e.g., `evenObj`, `errEven`)
- **Booleans**: `{condition}Exists` or `is{Condition}` (e.g., `evenExists`, `isOddLength`)
- **Errors**: `err{Type}` (e.g., `errEven`, `err`)
- **Consistency**: Use the same pattern for similar concepts throughout the codebase

