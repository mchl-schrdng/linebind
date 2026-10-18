# Installation

Linebind ships as one executable. Git must be available on `PATH`; Go, Node.js
and Python are not needed for native checks or the offline report. A declared
external command requires its own tools and explicit `--allow-exec` permission.

Download the archive matching your computer, plus `SHA256SUMS`, from the
[GitHub release](https://github.com/mchl-schrdng/linebind/releases).

| Platform | Architecture | v0.4.2 archive |
| --- | --- | --- |
| macOS | Apple silicon | `linebind_0.4.2_darwin_arm64.tar.gz` |
| macOS | Intel | `linebind_0.4.2_darwin_amd64.tar.gz` |
| Linux | x86-64 | `linebind_0.4.2_linux_amd64.tar.gz` |
| Linux | ARM64 | `linebind_0.4.2_linux_arm64.tar.gz` |
| Windows | x86-64 | `linebind_0.4.2_windows_amd64.zip` |
| Windows | ARM64 | `linebind_0.4.2_windows_arm64.zip` |

On macOS, calculate the downloaded archive's digest with:

```sh
shasum -a 256 linebind_0.4.2_darwin_arm64.tar.gz
```

On Linux use `sha256sum ARCHIVE`; on Windows use
`Get-FileHash ARCHIVE -Algorithm SHA256` in PowerShell. Compare the result with
the matching filename in `SHA256SUMS` before extracting. Checksums detect an
altered download; v0.4.2 archives are not signed or notarized.

Extract the archive, then move the executable into a directory on your `PATH`,
or run it by its full path. For example, on an Apple silicon Mac:

```sh
tar -xzf linebind_0.4.2_darwin_arm64.tar.gz
./linebind --version
```

On Windows, extract the ZIP using Explorer or `Expand-Archive`, then run
`.\linebind.exe --version`. Each archive also includes the README, license and
third-party notices.

Run `linebind init` inside the Git repository you want to check, then validate
and preview the draft against an existing comparison commit. The
[README](../README.md#get-started) walks through that first preview. Your
repository's review process adopts the contract; preview does not approve it.

## GitHub Actions

The repository includes a [setup Action](../action.yml) for Linux x64 runners.
It installs the pinned v0.4.1 release, verifies its SHA-256 and adds it to `PATH`.
See the [CI guide](ci.md#install-with-the-setup-action) for commit pinning,
base/head selection and report retention. No Homebrew tap is published.

## Build from source

To build from source, use Git and Go 1.26 or newer:

```sh
git clone https://github.com/mchl-schrdng/linebind.git
cd linebind
go build -o ./bin/linebind ./cmd/linebind
```

A source build reports `dev` unless version metadata is explicitly supplied.
Release builds use the pinned toolchain and metadata described in
[Contributing](../CONTRIBUTING.md#release-packaging).

## Coding-agent integration

Starting with v0.2.0, Linebind bundles a skill for Codex, Claude Code and
Antigravity. After installing that executable, run
`linebind setup --agent all` in your project. You do not need to copy this
repository or install Go in the consuming project. See the
[agent workflow](agents.md) for version availability, installation targets,
first-contract setup and deliberate policy changes.
