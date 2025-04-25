# Changelog

This file documents all notable changes made to this project since it was
split out of runc repository.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

[Unreleased]: https://github.com/opencontainers/cgroups/compare/v0.0.2...HEAD

## [0.0.2] - 2005-04-25

### Added
* CPU burst stats. (#11)
* CI infrastructure. (#3, #6)
* CI: add nolintlint linter. (#10)

### Changed
* Mark some fields with `omitempty` JSON attribute. (#9)
* Modernize code by using new Go features. (#13)
* CI: switch to golangci-lint v2. (#12)

### Fixed
* systemd: write rounded CPU quota to cgroupfs. (#4)

[0.0.2]: https://github.com/opencontainers/cgroups/compare/v0.0.1...v0.0.2

## 0.0.1 - 2025-02-28

### Added

* This is an initial release of the code after splitting it from the runc repository,
  according to the [proposal].

[proposal]: https://github.com/opencontainers/tob/blob/main/proposals/cgroups.md
