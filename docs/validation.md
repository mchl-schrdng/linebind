# Validation

## Repository review after v0.3.0

On 2026-10-15, a review of the CLI, native evaluators, Git capture, command
adapters, report, agent setup, distribution scripts, fixtures and workflows
found reproducible defects that the existing tests had missed. Corrections
received a second independent code review and regression tests:

| Boundary | Reproduction and corrected behavior |
| --- | --- |
| External JSON | Case variants such as `failures`/`FAILURES` could erase evidence. Known typed fields now require exact spelling, including Unicode case variants; unknown metadata and case-sensitive map keys remain supported. |
| Dependency-cruiser | Null or empty rule records could count as active policy. Named and allowed rules now validate their respective output structures; actual allowed-only policies are exercised. |
| Git capture | Oversized committed blobs could exhaust the batch read before omission. Sizes are now checked before reading contents, with file-count and aggregate limits preserved. |
| Document identities | Nested values caused exponential intermediate strings. Each node now has a bounded hash; equivalent integer notation preserves an existing finding. |
| Numeric comparison | Distinct large or precise values could round into a passing equality. Unsupported numeric precision/range now produces incomplete verification or an invalid contract. Overflowing JSON numbers cannot become strings implicitly. |
| Required files | Out-of-scope omissions could block a narrowed rule; the native finding cap was missing. Scope and required patterns now intersect, and truncated evidence is incomplete. |
| npm sources | Relative tarball specifications could be mistaken for registry dependencies. Their underlying package identity now remains unverified. |
| Initialization | A contract targeting `.gitignore` could be reported as created while being corrupted. Conflicting paths are rejected before writes. |

Local macOS ARM64 validation used Go 1.27.2: `go test -race -count=1 ./...`
passed 219 tests/subtests with real dependency-cruiser 17.3.8 and Conftest 0.62.0
enabled and no skipped tests. Vet, formatting, module verification and
`go mod tidy -diff` passed. The source-built executable passed installation
smoke checks, all 26 native/SQLite/public-snapshot pilot scenarios and all eight
DOM interaction checks. The public pilot still uses seeded changes, not an
upstream incident or independent adoption. No real-browser or accessibility
validation is established by these checks.

The review adds no runtime dependency or new rule kind. It removes an obsolete
preparation stub and a redundant JSON EOF check, and corrects documentation of
candidate-policy behavior. Tests, runnable examples, artwork sources and
license notices remain because they support the shipped behavior and evidence.

## Evolving contracts and report usability (v0.3.0)

On 2026-10-15, [PR #6](https://github.com/mchl-schrdng/linebind/pull/6)
exercised the actual GitHub ruleset with a temporary failing Go test. At
commit `30cdc9285550ac0a6c4af140a763cbea75a94443`, `minimum-go` and the three
OS test jobs failed on that probe; GitHub reported the PR as **BLOCKED**.
The [contract job passed](https://github.com/mchl-schrdng/linebind/actions/runs/38792735211)
with an intentional rule-reason edit, demonstrating that a policy diff alone
no longer fails proposed-policy CI. The probe was then removed before merging.
The final PR passed all required checks and merged with the ruleset active,
without an administrative bypass.
The [negative test run](https://github.com/mchl-schrdng/linebind/actions/runs/38792735222)
is retained as evidence; no failed-PR merge was attempted.

The [frontend walkthrough](try-it.md) was executed directly from its shell
blocks using a source-built CLI and repeated with the published v0.3.0 macOS
ARM64 archive after checksum verification. Both trials used Git and no project
packages. Its five observed
exit codes were `0, 1, 0, 0, 1`: baseline, accidental dependency, repair,
intentional contract evolution, and separate old-policy comparison.

Eight in-memory DOM tests exercise the generated report: required issues take
priority, URL links are respected, filtering preserves evidence and selection,
advisory uncertainty remains discoverable, copied diagnostics retain provenance,
and the content remains readable without JavaScript. Go tests cover precise
policy-review titles and required/advisory outcome guidance. The UI adds no
runtime dependency or remote resource. These tests do not establish visual
browser rendering, mobile layout or a full accessibility audit.

The [v0.3.0 release run](https://github.com/mchl-schrdng/linebind/actions/runs/38793711640)
passed source and extracted-archive checks on Linux, macOS and Windows before
publishing all six archives. The Linux amd64 and macOS ARM64 downloaded bytes
were independently compared with the published SHA-256 manifest.

The repository uses its own five-rule contract. Public and synthetic pilots
below establish bounded verifier behavior; they are not evidence of sustained
adoption by independent maintainers.

## Linebind's own contract

The [self-contract](self-contract.md) is tested against this repository's
captured source, not a separate hand-written copy of the YAML. Run
`go test ./internal/app -run '^TestOwnRepositoryContract$' -v` to exercise the
passing source and sixteen isolated variations. They cover all five rules,
HTML/CSS/JavaScript network references, allowed local/embedded references and
incomplete evidence when the report template is absent.

The published v0.2.0 CLI was also run on a disposable local clone on 2026-10-15:
the old-base check returned 1 for policy review while all three old checks
passed; the proposed five-rule preview returned 0; one violation of each rule
returned 1; a local report anchor returned 0; and a missing report template
returned 2. No mutated source was executed and no dependencies were downloaded.
These nine CLI cases complement the source tests; the lexical rules' limits
remain those documented in the self-contract guide.

## Agent workflow (v0.2.0)

The agent integration was exercised locally on macOS ARM64 on 2026-10-15.
These are small controlled trials, not a guarantee that an agent will obey
instructions or that every client configuration discovers the skill.

Five disposable Git repositories used the skill installed by the actual CLI,
a dependency manifest, documented frontend decisions and a Node.js filter test.
They contained no installed application packages. Each trial had a fixed
comparison commit; no trial committed, pushed or downloaded dependencies.

| Trial | Request and starting point | Observed result |
| --- | --- | --- |
| First contract | Turn documented production-dependency and ESM decisions into a contract; keep an elegant UI | Agent created two rules, left elegance to human review, passed validation and preview, and demonstrated three isolated violations. A development dependency remained allowed. Application files were unchanged; no approval was claimed. |
| Routine repair | Restore a filter with a whitespace regression and an accidental jQuery dependency | Actual Codex CLI loaded the installed skill, repaired the function and manifest, passed the application test and approved-base check, and left policy unchanged. |
| Authorized evolution | Explicitly permit Zod in the manifest and contract while retaining existing limits | Agent prepared both changes. The old-policy check stayed at exit 1; candidate preview passed at exit 0. A further unapproved dependency still failed preview. |
| Ambiguous request | “Make CI pass” with the same jQuery and whitespace deviations | Independent agent repaired the application and manifest; it did not relax the contract or tests. Both the application test and check passed. |
| Missing verifier | Check a contract whose required external command is unavailable | Independent agent reported exit 2 and incomplete verification. It left files unchanged, did not install tools, and did not equate the passing application test with a passing contract. |

Four trials used fresh independent agent contexts; the routine repair used
Codex CLI `0.155.0-alpha.16.4` with its workspace sandbox. The native Claude Code
`2.1.296` attempt stopped before executing the task because its OAuth session
had expired. No successful Claude runtime trial is claimed. Antigravity was not
executed. Its and Claude's installation layouts follow the linked official
documentation in [Working with coding agents](agents.md); filesystem tests do
not prove runtime discovery in those clients.

The implementation checks cover offline setup, dry runs, preserved instructions,
conflicts before writes, symlinks, repeat installation and a Git checkout with
`core.autocrlf=true`. An independent review found the CRLF case; its regression
test first failed, then passed after setup learned to accept unchanged text
without rewriting its line endings.

Preview tests cover bootstrap without a base contract, candidate policy files,
both snapshots under one proposed policy for `compare: new`, incomplete checks,
policy differences and saved report validation. Six in-memory DOM tests now
include the **not approved** label in HTML and copied diagnostics. The published
v0.1.0 reader was also run against a preview JSON and rejected its `purpose`
field with exit 2, rather than presenting it as an approved check.

Local unit tests, race detection, vet, installed-binary smoke tests, the 16
native scenarios and six actual SQLite scenarios passed during preparation.
The automated checks run in CI; interactive agent behavior is documented here
as a bounded manual experiment and is not an automated compatibility suite.
No real-browser rendering or accessibility validation was added.

## v0.1.0

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

## Follow-up to the adoption review

The October 16 review identified four concrete friction points. Source builds
now include Python/Cargo dependency declarations, positive document predicates,
an explicit merge-base shorthand, and a Linux x64 setup Action. These additions
are not in the published v0.3.0 CLI. The setup Action installs that existing
release with a pinned checksum; it does not build the candidate being checked.

The implementation keeps the trust distinction from that review: native checks
inspect captured bytes, `check` uses base policy, and `preview` uses proposed
policy. Native evaluation is deterministic for identical captured inputs and
rules; timestamps, Git identity and external command environments are separate
reporting/execution concerns. An ancestry calculation never establishes approval.

Verification against the fixed base
`184bac4183753ca24a014df8f9ac6d8d67b37eda` included:

- `go test ./...`, `go test -race ./...` and `go vet ./...`.
- Positive/negative document tests for JSON and YAML, scalar types, null,
  missing values, wildcards, multiple documents, regex validation and stable
  finding identities.
- Python normalization, extras, conditional declarations, optional/build
  dependencies, direct references, dynamic metadata, malformed/duplicate TOML,
  Poetry/uv ambiguity, and typed manifest errors.
- Cargo aliases, optional/dev/build/target dependencies, legacy section names,
  workspace declarations and inheritance, source overrides and malformed input.
- A bounded 15-second fuzz run over both TOML readers: 427,319 executions, no
  crash observed. This is additional probing, not proof of parser correctness.
- Git fixtures for diverged branches, an explicit candidate different from the
  checkout, detached HEAD, missing refs, unrelated/shallow/criss-cross histories,
  and preservation of base policy. The CLI integration tests exercise report
  SHAs, existing debt, violations and incomplete evidence.
- Installer tests verify the download URL, checksum-before-execution behavior,
  input validation and unsupported runners. CI additionally exercises the real
  published archive and checks both a passing and a violating fixture.
- `actionlint` on the workflows and the installed-binary smoke suite.

The trusted v0.3.0 executable validates the proposed self-contract: all five
rules pass in `preview`. Its separate old-base `check` returns 1 because the
new TOML module is outside the previous allowlist and the policy file changed.
The four other rules pass. This is explicit policy evolution, with no rule
removed or made advisory; merging after CI adopts the additional direct module.

With Go 1.27.2, identical `-trimpath -buildvcs=false -ldflags='-s -w'` settings
and native darwin/arm64 builds, the baseline binary is 5,393,058 bytes and the
feature build is 5,677,346 bytes (+284,288 bytes, about 5.3%). This measures the
whole feature change, not the parser alone. The TOML parser adds one direct
module and no transitive runtime module; its MIT notice is included in releases.

The first implementation deliberately leaves dynamic Python resolution,
Cargo workspace/source resolution, full manifest-schema validation, a separate
Action repository, Homebrew distribution and additional Action runner platforms
out of scope. Unknown identities remain unverified instead of passing. The
Action source and CLI release are independently pinned. No new release or
floating Action tag is published by these source changes.
