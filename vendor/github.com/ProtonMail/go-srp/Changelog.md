# Changelog for SRP

## v0.0.7 (2023-03-22)

### Changed

- Update `github.com/ProtonMail/go-crypto` to latest version.

## v0.0.6 (2023-03-17)

### Changed

- Changed API for challenge solvers to use int64 instead of uint64 for go-mobile

## v0.0.5 (2022-05-10)

### Fixed

- Removed usage of time API that were incompatible with golang < 1.17

## v0.0.4 (2022-05-05)

### Added
* New function `Argon2PreimageChallenge` to solve new hash preimage challenges.

### Changed
* Update `github.com/cronokirby/saferith` dependency to v0.33.0. Adds assembly routines support for more platforms
* Update `ECDLPChallenge` to the new specification.
* Validate that 2 is a generator for SRP moduli
* Removed support for building with glide

## v0.0.3 (2021-12-15)

## Fixed
* Fix the hash generation for users with SRP legacy authentication version 0.

## v0.0.2 (2021-10-05)

### Added
* Add a `Version` property to `Auth` struct to let the client access the version number used
for authentication.

### Changed
* Changed the return type of `ECDLPChallenge` from uint64 to int64 to be supported
by gomobile.
* Use `github.com/ProtonMail/bcrypt` directly instead of relying on replace statements for 
`github.com/jameskeane/bcrypt`.

## Fixed
* Use the `$2y$` version of `bcrypt` internally directly instead of using a workaround
with `$2a$`.
* Update the `github.com/cronokirby/saferith` arithmetic library to v0.32.0 to fix
issues for devices with 32bits architectures.

## v0.0.1 (2021-09-29)

### Added
* New function `ECDLPChallenge` to solve Elliptic Curve Discrete Logaritm Problem challenges.

### Changed
* Use `github.com/cronokirby/saferith.Nat` instead of `math/big.Int` for sensitive operations, to avoid side channel attacks.
* Updated `saferith` to `v0.31.0`
### Removed 
* Deleted `srp_openpgp.go` and `srp_openpgp_test.go` as it was redundant and unused. 

## 2021-05-04
### Added
* Add `NewServerWithSecret` function to create a custom server instance.

## 2021-05-03
### Changed 
* Updated ProtonMail/go-crypto to the latest version and reverted to use standard library packages in some cases.

## 2021-04-21
### Changed
* Changed all interfaces taking a password to use []byte instead of string

## 2019-01-14

### Added
* tests

## 2019-01-03

### Added
* pmapi#27 modulus pubkey and verification
* password hash functions for different versions
