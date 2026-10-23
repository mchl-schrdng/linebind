<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/assets/wordmark-dark.svg">
    <img src="docs/assets/wordmark.svg" width="280" alt="Linebind">
  </picture>
</p>

<p align="center">
  <a href="https://github.com/mchl-schrdng/linebind/actions/workflows/ci.yml?query=branch%3Amain+event%3Apush"><img src="https://github.com/mchl-schrdng/linebind/actions/workflows/ci.yml/badge.svg?branch=main&amp;event=push" alt="CI — main branch"></a>
  <a href="https://github.com/mchl-schrdng/linebind/actions/workflows/codeql.yml"><img src="https://github.com/mchl-schrdng/linebind/actions/workflows/codeql.yml/badge.svg" alt="CodeQL analysis"></a>
  <a href="https://github.com/mchl-schrdng/linebind/actions/workflows/contract.yml?query=event%3Apull_request"><img src="https://github.com/mchl-schrdng/linebind/actions/workflows/contract.yml/badge.svg?event=pull_request" alt="Repository contract — latest pull request run"></a>
  <a href="https://github.com/mchl-schrdng/linebind/actions/workflows/release.yml?query=event%3Apush"><img src="https://github.com/mchl-schrdng/linebind/actions/workflows/release.yml/badge.svg?event=push" alt="Release — latest tag run"></a>
</p>

<p align="center">
  <a href="https://github.com/mchl-schrdng/linebind/releases/latest"><img src="https://img.shields.io/github/v/release/mchl-schrdng/linebind?label=version&amp;logo=github&amp;color=171830" alt="Latest stable release"></a>
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/mchl-schrdng/linebind?label=Go%20source&amp;logo=go&amp;logoColor=white&amp;color=171830" alt="Minimum Go version for building from source"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/mchl-schrdng/linebind?color=171830" alt="License"></a>
  <a href="docs/installation.md"><img src="https://img.shields.io/badge/platforms-macOS%20%7C%20Linux%20%7C%20Windows-171830" alt="Release binaries for macOS, Linux and Windows"></a>
</p>

<p align="center">
  <a href="#get-started">Get started</a> ·
  <a href="#what-can-a-contract-check">What it checks</a> ·
  <a href="#tested-in-ci">CI</a> ·
  <a href="#with-your-coding-agent">Coding agents</a> ·
  <a href="docs/configuration.md">Configuration</a>
</p>

## Why I built Linebind

I built Linebind for my personal projects. As I delegated more implementation to
coding agents, I kept running into a problem: a change could pass the existing
CI checks while adding complexity or drifting away from a decision I wanted to
keep. Prompts and `AGENTS.md` helped communicate those decisions, but I still
had to check whether the resulting code respected them.

I wanted a few explicit, executable rules I could carry from one task to the
next. **Linebind turns selected repository decisions into a versioned contract,
checks changes against it, and reports the evidence.** Its bundled skill helps
an agent write and use that contract.

This is an experiment I am sharing for feedback. It does not judge whether a
design is good, enforce SOLID, or replace tests and code review. I would like to
know which decisions are worth checking this way, where the setup is too much
work, and where an existing tool already does the job better.

## One decision, one check

Suppose a project allows React and React DOM as runtime packages and wants new
packages reviewed explicitly. A contract can record that decision:

```yaml
version: 1
name: Frontend runtime packages
rules:
  - id: runtime-dependencies
    kind: dependencies
    reason: Review new runtime packages; checks root manifest declarations, not imports or transitive dependencies.
    remediation: Remove an accidental addition or propose a reviewed policy change.
    scope: [package.json]
    dependencies:
      format: npm
      sections: [dependencies]
      allow: [react, react-dom]
```

Adding another package to that section produces a finding, even if the
application tests pass. With `check`, adding it to the candidate's allowlist
also leaves a policy change for review: the selected base's rule still applies.
With `preview`, you can evaluate the proposed rule before adopting it.

This example checks declared names in one manifest. It says nothing about
package safety, bundle size or the quality of the implementation.
**[Run the disposable example](docs/try-it.md)**, or inspect the
[Go example with passing tests and a reported dependency change](examples/dependency-policy/README.md),
which also includes a case the rule misses.

## Get started

The CLI is **free and MIT-licensed**. Native checks need Linebind and Git, with
no account, model API or project dependency installation. Download the
[latest release](https://github.com/mchl-schrdng/linebind/releases/latest) for
macOS, Linux or Windows and follow the [installation guide](docs/installation.md).

In a Git repository with at least one commit, a `README.md`, and no contract yet:

```sh
BASE_SHA=$(git rev-parse --verify 'HEAD^{commit}')
linebind init
linebind validate
linebind preview --base "$BASE_SHA" --html .linebind/report.html
```

These commands use a POSIX shell. The starter checks only that `README.md`
exists. Edit `.linebind.yml` to reflect your own decisions, then validate and
preview again. The saved commit is a comparison baseline; your repository's
review process adopts the contract.

## With your coding agent

```sh
linebind setup --agent all
```

This installs the bundled skill for Codex, Claude Code and Antigravity, offline,
while preserving existing instructions. Select one with `--agent codex`,
`claude` or `antigravity`; add `--dry-run` to inspect the changes first. Reload
skills or start a new agent session if needed, then ask:

> Prepare a small Linebind contract from our documented decisions. Explain
> each rule's scope and limits, demonstrate a permitted change and a violation,
> and identify what our existing tests and tools should keep checking.

The skill guides three tasks: creating a contract, repairing unintended
violations, and proposing an authorized policy change. It tells the agent not
to weaken a rule just to obtain a passing check. **Those are instructions;
the CLI and required CI checks provide the independent verification.**
Installation does not ensure that an agent loads or follows the skill.

The [agent guide](docs/agents.md) covers setup, upgrades and compatibility.
The [canonical skill](internal/agentsetup/skill/SKILL.md) is bundled at release
time; a skill edit on `main` does not update an installed executable.

## What can a contract check?

| Rule | Example decision | What it checks |
| --- | --- | --- |
| [`require-files`](docs/configuration.md#require-files) | Keep a license and architecture document | Required paths and glob matches |
| [`forbid-text`](docs/configuration.md#forbid-text) | Keep debug markers out of shipped source | Go/RE2 patterns in selected text |
| [`dependencies`](docs/configuration.md#dependency-names) | Review new runtime packages | Declared names in npm, Go, Python and Cargo manifests |
| [`document`](docs/configuration.md#json-and-yaml-values) | Preserve an API operation or deployment setting | Selected JSON/YAML values |
| [`co-change`](docs/configuration.md#co-change) | Ship notes with a migration | Accompanying content edits between base and candidate |
| [`command`](docs/configuration.md#project-commands) | Use an existing architectural or policy verifier | Exit codes, dependency-cruiser or Conftest results |

Each rule has a reason, inputs and optional remediation. Supported checks can
use `compare: new` to distinguish new, existing and resolved findings.
See [configuration](docs/configuration.md) for scope, advisory rules and limits,
including [repository organization](docs/configuration.md#documentation-and-repository-organization).

Native checks inspect captured files without executing project code. External
commands need installed tools and `--allow-exec`; their temporary execution
copy **is not a sandbox**.

## Check adopted rules. Preview proposed changes.

| | `check` | `preview` |
| --- | --- | --- |
| Policy source | Selected base | Candidate |
| Purpose | Enforce adopted rules | Evaluate proposed rules |
| Policy edits alone | Exit `1`; review required | Visible in the report; can pass |

Once your repository has adopted a contract, set `BASE_SHA` to its trusted
base commit's full SHA and keep it fixed:

```sh
linebind check --base "$BASE_SHA" --html .linebind/check.html
linebind preview --base "$BASE_SHA" --html .linebind/preview.html
```

Both inspect the working tree by default. Add `--head "$HEAD_SHA"` for a
committed candidate. Preview never approves a proposal; your review process
does. Enforcing rules at merge time requires a required CI check and trusted
control of its base, verifier and invocation. [Trust model](docs/trust-model.md)

## See the evidence

The terminal shows a summary, next action and detailed findings. JSON and a
standalone offline HTML report retain the evaluated rules and captured inputs.
A result distinguishes a violation, incomplete verification and a policy change.
[Statuses and exit codes](docs/configuration.md#status-and-command-line-reference)

<p align="center">
  <a href="docs/assets/report-v0.4.0/mac/line-mac.png">
    <img src="docs/assets/report-v0.4.0/mac/line-mac.png" alt="Linebind report: four checks pass and an undeclared dependency crosses one required rule.">
  </a>
</p>

<p align="center"><sub>Real v0.4.0 synthetic-fixture evidence in a decorative Mac-style frame. <a href="docs/assets/README.md">Capture notes and reproduction</a></sub></p>

Select a check to read its reason, evidence and next step. Reports describe the
captured inputs, not later edits. [Report guide](docs/report.md) ·
[Policy preview example](docs/assets/report-v0.4.0/mac/preview-mac.png)

## Tested in CI

[CI](.github/workflows/ci.yml) runs Go tests and vet on Linux, macOS and Windows,
checks release archives, and exercises examples, real adapters and report
interactions. [Release](.github/workflows/release.yml) reuses that suite;
[CodeQL](.github/workflows/codeql.yml) analyzes Go code.

Our [contract gate](.github/workflows/contract.yml) uses `preview` with fixed PR
SHAs and a checksum-pinned released verifier. Merging after required checks pass
adopts the reviewed proposal. The setup Action and gate remain independently
pinned to v0.5.0. [CI setup and adoption](docs/ci.md) ·
[Contributor workflow](CONTRIBUTING.md)

## Where it helps, and where it does not

For one simple rule, a CI script may be enough. Linebind offers a common contract,
comparison and report for several explicit decisions. It runs inside CI;
it does not replace it. Keep specialist tools where they check the property
you need, and use a command rule only when connecting their evidence is useful.

A passing result covers only the declared checks. Linebind does not infer the
right architecture, detect all unnecessary complexity or prove that an agent
has followed every instruction. A narrow scope can miss a real violation;
a poor rule can block a useful change. The examples are controlled experiments,
not evidence of production adoption or measured productivity gains.

[Linebind Cloud](https://linebind-cloud.vercel.app/) adds a GitHub App and shared
PR reports, free for public and private repositories. Its check is informational
and currently supports native rules only. Local checks and the skill work
without Cloud. [Cloud setup and limitations](docs/cloud-pilot.md)

If you try it, I would especially value a concrete decision, the change you
expected it to accept or reject, and what actually happened. Suggestions for
simpler alternatives and cases where Linebind adds no value are welcome.
[Open an issue](https://github.com/mchl-schrdng/linebind/issues).

## Documentation and contributing

[Installation](docs/installation.md) · [Walkthrough](docs/try-it.md) ·
[Configuration](docs/configuration.md) · [Examples](examples/README.md) ·
[Agent guide](docs/agents.md) · [CI](docs/ci.md) ·
[Trust model](docs/trust-model.md) · [Contributing](CONTRIBUTING.md)

The CLI is written in Go. Run `go test ./...` and `go vet ./...` for local checks.
Reading this from an archive? The
[online README](https://github.com/mchl-schrdng/linebind#readme) includes the
linked guides and images.

---

<p align="center">
  Made by <a href="https://github.com/mchl-schrdng">mchl-schrdng</a> ·
  <a href="LICENSE">MIT</a> ·
  <a href="SECURITY.md">Security</a>
</p>
