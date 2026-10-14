# v0.1.0 validation

The checks below exercise the declared behavior of this release. They are not
evidence of production adoption or a claim that arbitrary repositories are safe.
The runnable commands live in [Contributing](../CONTRIBUTING.md) and
[Examples](examples.md).

| Layer | Evidence |
| --- | --- |
| Native engine | Unit tests for strict configuration, globs, dependency policies, JSON/YAML selectors, lexical findings and deterministic fingerprints |
| Git and comparison | Temporary Git repositories exercise base-controlled policy, working trees, immutable commits, missing inputs, symlinks, ignored parent links, resource limits and new/existing/resolved findings |
| Commands | Tests cover opt-in, timeout, bounded output, environment isolation, input mutation and malformed or contradictory structured results |
| Actual specialist tools | dependency-cruiser 17.3.8 and Conftest 0.62.0 run against committed example fixtures; an additional syntax check covers dependency-cruiser's parser recovery behavior |
| Installed CLI | Sixteen native scenarios cover frontend, backend, data and infrastructure, with passing, crossed, unverified and removed-policy cases |
| Actual data rows | Six Python/SQLite cases exercise unique keys, duplicate keys, null keys, malformed SQL, rejected writes and disabled execution |
| Public source snapshot | Four scenarios use a pinned p-limit commit with a Linebind-authored policy and seeded mutations; upstream project code is not executed |
| Saved reports | CLI round trips preserve the generated HTML; invalid schemas, contradictory verdicts and incomplete successful reports are rejected |
| Report interaction | Five in-memory DOM tests cover navigation, hostile text, clipboard paths, keyboard dismissal and readable content without JavaScript |
| Distribution | Six archives are cross-compiled with explicit version/commit metadata, fixed archive timestamps, licenses and SHA-256 checksums |

The local validation used macOS ARM64. `go test -race ./...`, `go vet ./...`,
the real external-tool examples and the 26 combined native/data/public scenarios
passed during release preparation. Rebuilding the same frozen source with the
same toolchain produced byte-identical archives.

The [CI workflow](../.github/workflows/ci.yml) requires source tests, vet, an
installed-binary smoke test and the native scenarios against extracted release
archives on Linux, macOS and Windows. It also tests the supported Go 1.26 series,
real external tools, SQLite cases and DOM interactions. The
[release workflow](../.github/workflows/release.yml) publishes only after these
jobs succeed, using the archives produced and tested in that run. Execution on
those three runners does not establish native execution of all six architectures.

Native report tests and DOM tests do not establish real-browser rendering,
clipboard permissions or assistive-technology behavior. Visual browser and
accessibility review remain unverified for v0.1.0. The README's desktop image is
an explicitly labeled [illustration with real fixture evidence](assets/README.md).

Other boundaries are deliberate: lexical rules do not resolve imports,
document rules do not validate running services, and host commands are not
sandboxed. Findings are as useful as the declared contract and the evidence its
checks can obtain. See the [trust model](trust-model.md) for the complete limits.
