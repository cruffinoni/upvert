# Releasing upvert

Upvert releases are driven by git tags. A release tag is published only after
the tag, the commit, CI, and a GoReleaser snapshot have been validated.

## Tag format

Release tags use semantic versions:

- Stable: `vX.Y.Z`
- Prerelease: `vX.Y.Z-<label>[.<counter>]`

Prerelease identifiers may contain letters, digits, and hyphens and may be
separated by dots. Build metadata (`+...`) is not supported because release
asset names do not encode it.

## Creating a release

Upvert itself has no static version declaration, so it uses git tags as its
version source. From a clean `master` checkout:

```sh
upvert patch --push      # updates the changelog, commits, tags, and pushes
```

Or commit and tag locally, then push:

```sh
upvert patch --commit
git push origin HEAD
git push origin vX.Y.Z
```

For the first release, there is no previous tag, so upvert starts from
`0.0.0`. Choose the first stable or prerelease tag deliberately before
publishing.

## Prerelease arithmetic

Upvert preserves its original semver arithmetic: a bump advances the numeric
core and then applies the prerelease label.

```text
1.0.0 -> 1.0.1-alpha.1
1.0.1-alpha.1 -> 1.0.2-alpha.2
1.0.2-alpha.2 -> 1.1.0-beta.1   (minor)
```

This means upvert does not promote `v1.0.1-rc.1` to `v1.0.1` with another
bump; it would advance the core. If a same-core stable tag is required, create
and push that tag explicitly after reviewing the release. Do not reuse a
published tag.

## CI and publishing

- `.github/workflows/ci.yml` runs on pull requests, pushes to `master`, and
  manual dispatch. It checks formatting, tests/vet/builds on Linux, macOS, and
  Windows, and runs the race detector.
- `.github/workflows/release.yml` runs on `v*.*.*` tag pushes. It validates a
  newly created, semver-valid tag on `master`, waits for the tagged commit's
  successful CI run, runs `goreleaser check` and a snapshot release, and
  smoke-tests the extracted Linux binary.
- GoReleaser publishes the archives and `checksums.txt`; prerelease tags are
  published as GitHub prereleases and are not made latest.

## Release assets and the updater contract

GoReleaser produces:

- `upvert_<version-without-v>_<os>_<arch>.tar.gz`
- `upvert_<version-without-v>_windows_<arch>.zip`
- `upvert` / `upvert.exe` inside the archive
- `checksums.txt`

The updater resolves the latest stable release from GitHub's `/releases/latest`
redirect, or uses an explicit `--version` tag. It downloads the matching
archive and checksums, verifies SHA-256, extracts the binary, and replaces the
installed executable.

Any change to archive naming, the Windows archive format, or the checksum asset
name must be made in both `.goreleaser.yaml` and `internal/update/release.go`.

## Install directories

- Unix: `$XDG_BIN_HOME` when set, otherwise `$HOME/.local/bin`.
- Windows: `%LOCALAPPDATA%\Programs\upvert`.

`upvert update` installs into that directory by default; use `--bin-dir` to
choose another location. The installers use the same defaults.

## Limitations

- Release signing and notarization are not configured; checksums are the
  integrity boundary.
- The updater does not automatically check for updates on every command.
- Prereleases are installed only when explicitly requested with `--version`.
