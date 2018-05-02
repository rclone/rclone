# Changelog


**v1.1.0**

- `OptParse` function parses FUSE options.
- `fmt.Stringer` and `fmt.GoStringer` implementation for `fuse.Error`.


**v1.0.4**

- Implement BSD `flags`, `chflags`, `setcrtime`, `setchgtime`.
- Improve documentation.


**v1.0.3**

- Windows XP compatibility (eliminate `RegGetValueW`).


**v1.0.2**

- Windows XP compatibility (eliminate slim R/W lock).


**v1.0.1**

- Cross-compilation `Dockerfile`.
- CircleCI integration.
- Do not catch `SIGHUP`.
- Improve documentation.


**v1.0**

- Initial cgofuse release.
- The API is now **FROZEN**. Breaking API changes will receive a major version update (`2.0`). Incremental API changes will receive a minor version update (`1.x`).
