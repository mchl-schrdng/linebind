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
go build -o ./bin/linebind-dev ./cmd/linebind
go run ./scripts/smoke -binary ./bin/linebind-dev
go run ./scripts/pilots -binary ./bin/linebind-dev
```

The smoke check uses a temporary Git repository outside this checkout, including
a path with spaces. It checks version, help, initialization and validation.
Add regression tests for changed behavior. Keep native diagnostics and distinguish
a failed check from a check that could not run.

The [example runner](examples/README.md) can also exercise a trusted Python/SQLite
checker with `-data-command`. This is optional for local native checks.

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
job executes only the project's own integration fixtures.

Report interaction tests use Node.js and a test-only dependency outside the checkout:

```sh
npm install --prefix /tmp/linebind-ui-test --no-save --ignore-scripts --no-audit --no-fund jsdom@26.1.0
NODE_PATH=/tmp/linebind-ui-test/node_modules node scripts/ui-test.mjs ./bin/linebind-dev
```

They exercise the generated report in an in-memory DOM with network loading
disabled. They do not establish real-browser layout, clipboard permissions or
assistive-technology behavior.

## Repository contract

[`AGENTS.md`](AGENTS.md) is this repository's agent entry point and directs agents
to the canonical [bundled skill](internal/agentsetup/skill/SKILL.md). Provide it
explicitly to agents that do not load `AGENTS.md` automatically. Consuming projects
use [`linebind setup`](docs/agents.md), which still supports each agent's own
instruction files.

Our [five-rule contract](.linebind.yml) preserves these decisions:

| Rule | Coverage and limit |
| --- | --- |
| `public-entry-points` | Required licensing, security and contributor/user guides; presence does not prove quality |
| `deliberate-direct-dependencies` | The four allowed direct declarations in `go.mod`; not versions, indirect dependencies or source imports |
| `pure-native-contract-package` | Selected process/network/filesystem spellings in the evaluator; a lexical check, including comments, not an effect analysis |
| `bundled-agent-guidance` | The embedded skill and both references exist; not proof of agent discovery or compliance |
| `report-network-references` | Selected network references in the HTML template; constructed URLs and other APIs can escape this lexical check |

Use a trusted released executable at `./bin/linebind`, separate from the
`linebind-dev` build above. Resolve the freshly fetched `origin/main` once at the
start of a task and keep that full SHA fixed:

```sh
git fetch origin main
BASE_SHA=$(git rev-parse --verify 'origin/main^{commit}')
./bin/linebind validate
./bin/linebind preview --base "$BASE_SHA" --agent --html .linebind/self-preview.html
```

This matches our candidate-policy CI. For policy evolution, also run the old
gate separately and retain its report:

```sh
./bin/linebind check --base "$BASE_SHA" --format json \
  --output .linebind/self-check.json --html .linebind/self-check.html
```

An expected policy-review exit `1` must not prevent the preview from running.
Preview validates the proposal; merging after required checks pass adopts it.
Do not change the base or weaken a rule to obtain a pass. The Go test
`TestOwnRepositoryContract` exercises the actual YAML against isolated permitted,
violating and missing-input cases. Application tests remain separate.

## CI and merging

Each workflow has a distinct responsibility:

| Workflow | Purpose |
| --- | --- |
| [CI](.github/workflows/ci.yml) | Build six archives; run tests, vet, source-install and archive smoke checks on Linux/macOS/Windows; exercise the minimum Go version, real adapters, SQLite, report interactions and the setup Action |
| [Repository contract](.github/workflows/contract.yml) | Preview the proposed contract with fixed PR SHAs and a checksum-pinned released verifier |
| [CodeQL](.github/workflows/codeql.yml) | Analyze Go code and upload security findings; successful analysis is not a clean-security verdict |
| [Release](.github/workflows/release.yml) | Reuse CI for a stable version tag and publish the exact tested archives |
| [Trusted Linebind analysis](.github/workflows/cloud-pilot.yml) | Public reusable workflow called by Cloud-connected repositories; submit native reports using OIDC |

The Cloud workflow is a public integration, despite its historical filename.
Keep its path and reviewed revisions available to callers. Its hosted check is
informational and separate from the required repository contract gate.

All [scripts](scripts/) have callers: CI/release packaging, installation smoke
checks, the [example suite](examples/README.md), report interaction tests, and
the public [setup Action](action.yml). The Action source and the CLI version it
installs are independently pinned; see [CI integration](docs/ci.md).

All seven required CI checks must pass before merging into `main`, including
contract validation. No bypass or second reviewer is configured. Contract edits
are normal: CI validates the proposed rules with `preview`, and the maintainer
reviews the diff before merging. Use temporary feature branches and delete them
after merging; `main` is the only long-lived branch. See
[required checks](docs/ci.md#make-the-check-required).

## Release packaging

Maintainers use Go 1.27.2 for release builds. This command creates six archives
and `SHA256SUMS` in a new directory; it does not publish anything:

```sh
go run ./scripts/package -version v0.5.0 -out dist
```

The commit defaults to `git rev-parse HEAD`. Each archive contains the binary,
README, MIT license and dependency notices. Build from a clean checkout with the release toolchain
to compare hashes. The helper fixes archive timestamps and builds with
`CGO_ENABLED=0`, `-trimpath` and explicit version/commit metadata.

CI tests the source and extracted packages on Linux, macOS and Windows. All six
OS/architecture combinations are cross-compiled; this does not mean every
architecture has been executed on native hardware. A separate CI job tests the
supported Go 1.26 series.

Pushing an exact stable tag such as `v0.5.0` runs the same checks and publishes
the tested archives as a GitHub release. Only the final publishing job has
repository write permission. Action versions are pinned to upstream commit SHAs.

Contributions are accepted under the project's [MIT license](LICENSE).
