# Contributing

Small, focused pull requests are welcome. Describe the repository decision you
want to verify, the current behavior, and the expected result. For a new check,
include a minimal passing case, a failing case, and a case that cannot be verified.
Discuss changes to the contract or report format in an issue before implementing
a broad redesign.

You need Git and Go 1.26 or newer. From the repository root:

```sh
go test ./...
go vet ./...
gofmt -w cmd internal scripts
go build -o linebind ./cmd/linebind
go run ./scripts/smoke -binary ./linebind
go run ./scripts/pilots -binary ./linebind
```

The smoke check uses a temporary Git repository outside this checkout, including
a path with spaces. It checks version, help, initialization and validation.
Add regression tests for changed behavior. Keep native diagnostics and distinguish
a failed check from a check that could not run.

The [example runner](docs/examples.md) can also exercise a trusted Python/SQLite
checker with `-data-command`, or fetch one pinned public repository with
`-public`. Neither is required to run the native examples.

CI additionally exercises the real dependency-cruiser 17.3.8 and conftest 0.62.0
adapters on Linux. To run those tests locally, install those exact tools and pass
absolute executable paths:

```sh
LINEBIND_DEPCRUISE=/path/to/depcruise LINEBIND_CONFTEST=/path/to/conftest \
  go test -race ./internal/app -run '^TestReal' -v
```

The race detector requires a supported C toolchain. CI disables npm lifecycle
scripts and verifies the conftest release archive against its pinned upstream
SHA-256 before execution. These integration fixtures belong to Linebind; the
job does not execute commands from the public p-limit pilot.

## Release packaging

Maintainers use Go 1.27.2 for release builds. This command creates six archives
and `SHA256SUMS` in a new directory; it does not publish anything:

```sh
go run ./scripts/package -version v0.2.0 -out dist
```

The commit defaults to `git rev-parse HEAD`. Each archive contains the binary,
README, MIT license and dependency notices. Build from a clean checkout with the release toolchain
to compare hashes. The helper fixes archive timestamps and builds with
`CGO_ENABLED=0`, `-trimpath` and explicit version/commit metadata.

CI tests the source and extracted packages on Linux, macOS and Windows. All six
OS/architecture combinations are cross-compiled; this does not mean every
architecture has been executed on native hardware. A separate CI job tests the
supported Go 1.26 series.

Pushing an exact stable tag such as `v0.2.0` runs the same checks and publishes
the tested archives as a GitHub release. Only the final publishing job has
repository write permission. Action versions are pinned to upstream commit SHAs.

Contributions are accepted under the project's [MIT license](LICENSE).
