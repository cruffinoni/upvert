# upvert

`upvert` is a self-contained Go CLI for bumping semantic versions across
projects. It is a Go port of the original `bump-version.sh` workflow and keeps
its safety-first approach: **nothing is written until you pass `--write`,
`--commit`, or `--push`; rollback is preview-only until you pass `--apply`**.

It auto-detects Go, Node.js, Python, Rust, and repositories that mix several of
those ecosystems, then updates version declarations and the changelog. It can
also create the release commit and annotated tag, and optionally push them.

## Build and install

Requires Go 1.27 or newer.

```sh
go build -o upvert ./cmd/upvert
# or install the command from this checkout
go install ./cmd/upvert
```

Build from a checkout and replace the installed binary:

```sh
./install.sh --local
./install.sh --local --bin-dir "$HOME/.local/bin"
.\install.ps1 -Local
```

`--local` builds `./cmd/upvert` with Go, skips the release download and
checksum verification, and installs over the binary found on `PATH` (or the
default install directory). It does not modify `PATH`.

Install a published release:

```sh
curl -fsSL https://raw.githubusercontent.com/cruffinoni/upvert/master/install.sh | sh
# Windows PowerShell
irm https://raw.githubusercontent.com/cruffinoni/upvert/master/install.ps1 | iex
```

Then keep it current with:

```sh
upvert update
upvert update --version vX.Y.Z-rc.1
```

Run the test suite and the Go guidelines linter with:

```sh
go test ./...
go vet ./...
$HOME/dev/go-guidelines/gg-lint scan .
```

## Usage

```text
upvert <patch|minor|major|pre|rollback|update|version> [flags]
```

The default is a preview that prints a language-agnostic report of every change
it would make. Choose one action per run.

```sh
upvert patch                          # preview in the current directory
upvert patch --write                  # update files only
upvert minor --commit                 # update, commit, and tag
upvert patch --pre alpha --commit     # start an alpha on the next patch core
upvert pre alpha --commit             # continue the current alpha
upvert pre --release --commit         # promote the current prerelease
upvert minor --push                   # update, commit, tag, and push
upvert rollback --apply               # undo the last bump commit and tag
upvert patch -C ../other-repo         # operate on another repository
```

### Commands

| Command    | Description                                                             |
|------------|-------------------------------------------------------------------------|
| `patch`    | Increment the patch version (1.0.0 -> 1.0.1)                            |
| `minor`    | Increment the minor version, resetting patch (1.0.0 -> 1.1.0)           |
| `major`    | Increment the major version, resetting minor and patch (1.0.0 -> 2.0.0) |
| `pre`      | Continue, relabel, or promote the current prerelease                      |
| `rollback` | Remove the last bump tag and reset the work tree (use `--apply`)          |
| `update`   | Install a GitHub release, latest stable by default                      |
| `version`  | Print the upvert version, commit, and build time                        |

### Flags

Actions are mutually exclusive; the default is a preview.

| Flag       | Description                                     |
|------------|-------------------------------------------------|
| `--write`  | Update files without committing or tagging      |
| `--commit` | Update files, create a release commit, and tag  |
| `--push`   | Update files, commit, tag, and push to `origin` |
| `--apply`  | Perform a `rollback` (the default is a preview) |

`--apply` is only valid on `rollback`. `--write`, `--commit`, and `--push`
are valid on `patch`, `minor`, `major`, and `pre`.

Modifiers:

| Flag                      | Description                                                    |
|---------------------------|----------------------------------------------------------------|
| `--pre <label>`           | Start a prerelease on the bumped core                          |
| `--release`               | Promote the current prerelease to stable (`pre` only)          |
| `--version-file <path>`   | Explicit version file instead of auto-detection                |
| `--changelog-file <path>` | Changelog path (default `CHANGELOG.md`)                        |
| `--no-changelog`          | Skip the changelog update                                      |
| `--no-version-attr`       | Skip in-code versions (`__version__`, `VERSION`, Go `Version`) |
| `--message <template>`    | Commit message template containing `{version}`                 |
| `--pre-bump <cmd>`        | Shell command to run before bumping                            |
| `--post-bump <cmd>`       | Shell command to run after bumping                             |
| `--quiet`                 | Suppress step progress; the report is still printed            |
| `-C, --dir <path>`        | Run as if started in this directory                             |

```sh
upvert update [--version <tag>] [--bin-dir <dir>] [--no-path-update] [--quiet]
```

Update flags:

| Flag                | Description                                                        |
|---------------------|--------------------------------------------------------------------|
| `--version <tag>`   | Release tag to install; defaults to the latest stable release      |
| `--bin-dir <dir>`   | Install directory (default: `~/.local/bin` or the Windows location) |
| `--no-path-update`  | Windows only: do not add the install directory to the user PATH    |
| `--quiet`           | Suppress step progress; print only the final result and errors     |

upvert reads no configuration files and no `UPVERT_*` environment variables.
Every setting is a command-line flag. The defaults are the current directory,
`CHANGELOG.md`, and `chore: bump version to {version}`.

## Releases and self-update

Releases are published from `v*` tags by GoReleaser after CI validates the tag.
The release assets are `upvert_<version>_<os>_<arch>.tar.gz` (or `.zip` on
Windows) plus `checksums.txt`.

```sh
upvert update                         # latest stable release
upvert update --version v1.2.3-rc.1   # explicit prerelease
upvert update --bin-dir "$HOME/bin"   # custom install directory
upvert update --quiet
```

`upvert update` installs into `$XDG_BIN_HOME` or `$HOME/.local/bin` on Unix
and `%LOCALAPPDATA%\Programs\upvert` on Windows. Use `--bin-dir` to install
somewhere else. Prereleases are installed only when passed explicitly with
`--version`.

`upvert version` prints the injected release version, commit, and build time.
See `docs/release-versioning.md` for the tag, release, and updater contract.

## What a bump does

1. Detects the project type and the file(s) that declare the version.
2. Computes the next version and refuses to continue when the result is not
   greater than the latest `vX.Y.Z` tag.
3. For `--commit` and `--push`, validates the git environment (repository,
   clean work tree, version/tag consistency) before touching any file.
4. Updates the version declarations (tag-sourced projects are left untouched)
   and prepends a dated entry to the changelog.
5. `--write` stops after the files, changelog, and hooks. `--commit` also
   creates the changed-file commit and annotated tag. `--push` also pushes
   both to `origin`.

upvert never runs `go mod tidy` or any other language tooling. A bump only
edits the discovered version declarations and the changelog; it does not touch
`go.mod`, `go.sum`, lockfiles, or module caches.

### Output

Previews and real runs use the same report. The change list and release plan
stay identical; preview and commit differ only in the heading and the closing
line. `--write` uses the same layout but omits the release section because it
leaves git untouched.

```text
upvert minor (preview)
  Version   1.2.3 → 1.3.0
  Source    package.json

  Changes
    package.json
    CHANGELOG.md              entry for 1.3.0

  Release
    tag      v1.3.0
    commit   chore: bump version to 1.3.0

  Nothing was written. Re-run with --write, --commit, or --push.
```

`--write` and `--commit` print the same report with `(write)` or `(commit)`
in the heading and their own closing line. The report goes to standard output;
warnings, errors, and step progress stay on standard error.

Real runs also report each operation while it happens: writing the version
files, updating the changelog, committing, tagging, and pushing. On an
interactive terminal a single animated line names the current step, shows its
position in the run, and counts the seconds spent on it, so a slow hook or push
is immediately identifiable; the line is cleared before the report. Because the
line is painted as soon as a step starts, a fast run shows one brief line while
a stuck step keeps its label and elapsed time on screen. When standard error is
redirected, each step is a plain line, which keeps CI logs readable. A run that
fails after writing restores the files it touched, so the work tree is left as
it was. Preview runs print no progress, and `--quiet` suppresses it for real runs.

### Project detection

| Ecosystem | Detected from | Version source |
| --- | --- | --- |
| Go | `go.mod` | `Version = "x.y.z"` in a known version file or any `.go` file; git tags when no static declaration exists |
| Rust | `Cargo.toml` with `[package]` | `[package].version` |
| Python | `pyproject.toml`, `setup.cfg`, `setup.py` | packaging metadata, plus in-code `__version__` / `VERSION` attributes |
| Node.js | `package.json` | top-level `version` field (no `jq` required) |
| Multi-language | two or more of the above | **git tags are the source of truth** |

A repository with more than one ecosystem is treated as having one platform
version. Every discovered declaration is synchronized, including
`package-lock.json`, `[tool.poetry]`, `[workspace.package]`, and nested Python
modules. Generated trees such as `.git`, `node_modules`, `vendor`, `venv`,
`build`, `dist`, and `*.egg-info` are ignored, and inside a git work tree the
repository ignore rules decide what counts as a project file, so scratch
directories and local fixtures can never be mistaken for manifests. An
untagged multi-language repository starts from `0.0.0`, so the first `patch`
bump produces `0.0.1`.

Go projects without a static semantic `Version` declaration — common when
GoReleaser injects the version with `-ldflags -X` — use git tags as the version
source. `--write` is rejected for these projects because it would not create
the tag; use `--commit` or `--push`. Add a supported static declaration, or
point `--version-file` at a supported file, to opt into file updates.

### Pre-releases

```sh
upvert patch --pre alpha --commit    # 1.0.0 -> 1.0.1-alpha.1
upvert pre alpha --commit            # 1.0.1-alpha.1 -> 1.0.1-alpha.2
upvert pre beta --commit             # 1.0.1-alpha.2 -> 1.0.1-beta.1
upvert pre --release --commit        # 1.0.1-beta.1 -> 1.0.1
upvert minor --pre beta --commit     # 1.0.1 -> 1.1.0-beta.1
```

`--pre` always starts a prerelease at `.1` on the version produced by the
bump. `upvert pre <label>` continues or relabels the current prerelease
without changing its numeric core, and `upvert pre --release` promotes it to
stable. The `pre` command also accepts `--write`, `--commit`, `--push`, the
changelog and hook modifiers, and `-C`.

## Behavioural differences from the script

The port preserves the script's workflow, safety checks, file formats, and
messages. The intentional differences are:

- **Single binary, Go CLI.** Commands and flags are exposed through
  [cobra](https://github.com/spf13/cobra), use the usual `--flag` syntax, and
  errors exit with status 1. `--dir` / `-C` is new and makes the tool usable
  outside the current directory.
- **One explicit action per run.** `--write`, `--commit`, and `--push` are
  mutually exclusive; rollback uses `--apply` and defaults to a preview.
- **Hooks do not run during a preview.** The script ran `--pre-bump` even when
  it promised that "no changes will be made". Both hooks now run only with
  `--write`, `--commit`, or `--push`.
- **Prereleases have their own command.** `--pre` starts a prerelease on the
  bumped core; `upvert pre <label>` continues or relabels it without moving
  the core, and `upvert pre --release` promotes it. The original script
  advanced the core on every invocation.
- **No `jq` dependency.** `package.json` and `package-lock.json` are edited in
  place and keep their existing formatting and key order.
- **Rollback understands custom commit messages.** The rollback pattern is
  derived from `--message`; the script only recognised its default prefix.
  Rollback also works when the project type cannot be auto-detected.
- **Deterministic file discovery.** Files are processed in lexical path order
  rather than filesystem order.
- **Single-project `setup.cfg` writes work for unquoted versions**
  (`version = 1.2.3`), which the script could not update outside multi-language
  mode.
- **Pre-release labels are validated by the semver parser** before any file is
  written, so an invalid label cannot produce an invalid version string.
- **Go version declarations are resolved with `go/ast`.** The version may be a
  literal or a same-file variable reference; the literal is updated in place and
  indirection is reported. Multiple semantic-version declarations in one file are
  rejected. Declarations marked as build-injected (Go directives such as
  `//go:embed`, `-ldflags`, or notes like "injected at build time") cause the
  project to use git tags instead of being edited. Unparseable Go files are
  skipped with a warning during detection.
- **Colour is opt-in by terminal.** ANSI colours are used only when stderr is a
  terminal, `TERM` is not `dumb`, and `NO_COLOR` is unset.
- **No configuration files or environment overrides.** Every setting is a
  command-line flag, so a repository cannot change actions, targets, or hooks.
- **Running without a command prints help** and exits successfully, following
  the usual Go CLI convention; the script treated it as an error.

Preserved quirks worth knowing:

- Multi-language repositories use tags, not file contents, as the version
  source; file values are overwritten on the next bump. `--write` is rejected
  for these projects because it would leave the tag behind.
- Go projects without a static version declaration also use tags, so
  `--write` is rejected there for the same reason.
- Rollback requires `--apply` and runs `git reset --hard HEAD~1`; uncommitted
  work is discarded. The command warns when a force push may be needed.

## Layout

```text
cmd/upvert/main.go        entry point and signal handling
internal/cli              flag-only command tree
internal/bump             public facade and bump/rollback pipeline
internal/project          project detection registry and composite model
internal/ecosystem        language strategy contract
internal/ecosystem/golang Go version declarations (go/ast)
internal/ecosystem/node   package.json and package-lock.json
internal/ecosystem/python Python packaging metadata and in-code modules
internal/ecosystem/rust   Cargo.toml
internal/semver           semantic-version parsing and bump arithmetic
internal/fileio           filesystem queries and atomic content edits
internal/toml             format-preserving TOML edits and queries
internal/vcs              git command-line adapter
internal/gomod            go.mod maintenance
internal/changelog        changelog entry rendering
internal/ui               levelled terminal output
internal/buildinfo        injected release version metadata
internal/update           release download, checksum, and install logic
```

Release automation lives in `.github/workflows` and `.goreleaser.yaml`;
`install.sh` and `install.ps1` install published binaries. The updater contract
is documented in `docs/release-versioning.md`.

Each supported ecosystem implements `ecosystem.Language` (`Detect`, `Scan`,
`Read`, `Write`); `internal/project` wires the adapters into an ordered registry
and composes multi-language projects. Adding a language means adding one adapter
package and its registry entries; the bump pipeline and the CLI stay unchanged.

The plumbing is deliberately library-backed: cobra for commands,
Masterminds semver for version math, sjson/gjson for formatting-preserving JSON
edits, go-toml for TOML parsing and targeted edits, go-ini for setup.cfg
parsing, renameio for atomic writes, and fatih/color plus go-isatty for terminal
output. The self-updater adds golang.org/x/mod, golang.org/x/sync,
golang.org/x/term, and golang.org/x/sys; tests use testify. Git itself is still
invoked as the system git command so credential helpers and transports keep
working.
