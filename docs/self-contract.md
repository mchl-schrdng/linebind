# Linebind checks its own repository

The committed [contract](../.linebind.yml) captures five existing product
decisions. The published CLI checks it locally and in our
[pull-request workflow](../.github/workflows/contract.yml).

| Rule | Decision and evidence | Boundary |
| --- | --- | --- |
| `public-entry-points` | Keep licensing, security, configuration, trust and agent documentation present | Presence does not establish content quality |
| `deliberate-direct-dependencies` | Keep the three approved direct Go modules explicit | Names from `go.mod`, not versions, vulnerabilities, transitives or an import graph |
| `pure-native-contract-package` | Reject selected process, network and filesystem calls in the native evaluator | Lexical detection, including comments and tests; aliases and other APIs can escape it |
| `bundled-agent-guidance` | Keep the embedded skill and both operating/authoring references | Presence does not prove skill discovery or agent compliance |
| `report-network-references` | Reject literal remote URLs in HTML attributes/CSS imports and selected JavaScript network calls in the report template | Lexical detection, not a browser/network sandbox; constructed URLs and other APIs are outside its coverage |

The normal Go tests, actual external-tool trials and report interaction tests
remain separate CI checks. We do not express UI elegance, overall security or
semantic architecture correctness as assertions these five rules can prove.
`internal/app/self_contract_test.go` exercises this actual contract against
captured source and isolated mutations, including missing inputs and allowed
local report links. The dependency declarations must correctly distinguish
direct and indirect modules; native checks do not derive that distinction from
Go imports.

## Run the workflow here

Use a trusted published executable. In this maintainer repository, `origin/main`
is the designated approved branch. Resolve it once at the beginning of a task;
fetch it through the normal Git workflow first when the local ref is stale.
The example assumes the executable is installed at `./bin/linebind`:

```sh
BASE_SHA=$(git rev-parse --verify 'origin/main^{commit}')
./bin/linebind validate
./bin/linebind preview --base "$BASE_SHA" --agent --html .linebind/self-preview.html
```

This validates the current working tree under its proposed contract, matching
our CI mode. For contract evolution, also keep this old-policy comparison:

```sh
./bin/linebind check --base "$BASE_SHA" --format json \
  --output .linebind/self-check.json --html .linebind/self-check.html
```

A policy change is expected to leave `check` at exit 1 even when every old rule
passes. This is old-policy evidence, not this repository's merge gate. Our CI
uses `preview` to validate the proposed contract, alongside the ordinary tests.
A contract edit alone can pass. Invalid contracts, incomplete required checks
and required violations fail.

Keep the fixed-base report and proposed policy diff available for review.
The maintainer adopts the proposal by merging once all required CI checks pass.
No external reviewer is mandatory; no actor bypasses those checks. Never change
the gate, weaken a rule or move the comparison base just to obtain green.
After adoption into `main`, a new task can resolve that approved revision. See [CI integration](ci.md#bootstrap-and-policy-changes).

The source repository's `AGENTS.md` and `CLAUDE.md` point to the canonical
bundled skill instead of maintaining two generated copies of that same source.
Consuming projects use `linebind setup --agent all` as documented in the
[agent guide](agents.md).
