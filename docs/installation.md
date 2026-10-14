# Installation

Linebind ships as one executable. Git must be available on `PATH`; Go, Node.js
and Python are not needed for native checks or the offline report. A declared
external command requires its own tools and explicit `--allow-exec` permission.

Download the archive matching your computer, plus `SHA256SUMS`, from the
[GitHub release](https://github.com/mchl-schrdng/linebind/releases).

| Platform | Architecture | v0.1.0 archive |
| --- | --- | --- |
| macOS | Apple silicon | `linebind_0.1.0_darwin_arm64.tar.gz` |
| macOS | Intel | `linebind_0.1.0_darwin_amd64.tar.gz` |
| Linux | x86-64 | `linebind_0.1.0_linux_amd64.tar.gz` |
| Linux | ARM64 | `linebind_0.1.0_linux_arm64.tar.gz` |
| Windows | x86-64 | `linebind_0.1.0_windows_amd64.zip` |
| Windows | ARM64 | `linebind_0.1.0_windows_arm64.zip` |

On macOS, calculate the downloaded archive's digest with:

```sh
shasum -a 256 linebind_0.1.0_darwin_arm64.tar.gz
```

On Linux use `sha256sum ARCHIVE`; on Windows use
`Get-FileHash ARCHIVE -Algorithm SHA256` in PowerShell. Compare the result with
the matching filename in `SHA256SUMS` before extracting. Checksums detect an
altered download; v0.1.0 archives are not signed or notarized.

Extract the archive, then move the executable into a directory on your `PATH`,
or run it by its full path. For example, on an Apple silicon Mac:

```sh
tar -xzf linebind_0.1.0_darwin_arm64.tar.gz
./linebind --version
```

On Windows, extract the ZIP using Explorer or `Expand-Archive`, then run
`.\linebind.exe --version`. Each archive also includes the README, license and
third-party notices.

Run `linebind init` inside the Git repository you want to check. Review and
commit the generated contract before using that commit as `--base`. The
[README](../README.md#get-started) walks through the first check.

To build from source, use Git and Go 1.26 or newer:

```sh
git clone https://github.com/mchl-schrdng/linebind.git
cd linebind
go build -o ./bin/linebind ./cmd/linebind
```

A source build reports `dev` unless version metadata is explicitly supplied.
Release builds use the pinned toolchain and metadata described in
[Contributing](../CONTRIBUTING.md#release-packaging).
