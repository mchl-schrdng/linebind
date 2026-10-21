<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/assets/wordmark-dark.svg">
    <img src="docs/assets/wordmark.svg" width="280" alt="Linebind">
  </picture>
</p>

<p align="center"><strong>Keep project decisions from drifting.</strong></p>
<p align="center">A free, open-source CLI for explicit repository rules.<br>Check a change against its trusted base. See what crossed the line.</p>

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

A PR can pass its tests and still add an unwanted dependency, change a deployment
setting or ship a migration without its notes. Linebind checks those explicit
project decisions against a YAML contract stored in Git.

**`check` reads the contract from the selected trusted base.** A PR cannot make
itself pass by weakening that contract. Proposed rule changes remain visible
for review; `preview` lets you evaluate them before adoption.

The CLI is **free and MIT-licensed**. Native checks need only Linebind and Git:
no account, dependency installation or model API. Results are available in the
terminal, JSON and an offline HTML report.

The bundled [agent skill](#with-your-coding-agent) guides your coding agent from
project decisions to a verifiable contract, then through repairs and deliberate
rule changes. The CLI performs the checks; the skill helps the agent use their
evidence. [Linebind Cloud](https://linebind-cloud.vercel.app/) adds a GitHub App
and shared PR reports with a Free Marketplace plan. Local checks and the skill
work without a Cloud account.

<p align="center">
  <img src="docs/assets/mecha-banner.webp" width="100%" alt="Original panoramic Linebind illustration: an ivory mecha and its articulated hand hold a coral line across a midnight-blue background.">
</p>

## Get started

Download the [latest release](https://github.com/mchl-schrdng/linebind/releases/latest)
for macOS, Linux or Windows. Verify the archive, extract it and put `linebind`
on your `PATH`. [Installation and checksums](docs/installation.md)

In an existing Git repository with at least one commit, a `README.md`, and no
contract yet, run these commands in a POSIX shell:

```sh
BASE_SHA=$(git rev-parse --verify 'HEAD^{commit}')
linebind init
linebind validate
linebind preview --base "$BASE_SHA" --html .linebind/report.html
```

Open `.linebind/report.html` in your browser. The starter checks only that
`README.md` exists. Edit `.linebind.yml` to describe decisions your project
actually needs, then validate and preview again.

Here, the saved commit is a comparison baseline. Preview evaluates the draft;
your repository's review and merge process adopts it.

**[Try the disposable demo](docs/try-it.md)** to introduce an undeclared
dependency, fix it, then propose a deliberate contract change. Only Git and
Linebind are required.

## A frontend change in practice

A feature adds jQuery to `package.json`. The project has agreed to review new
runtime packages, so its contract identifies the dependency and explains what
to do next. Here is the dependency rule from the report as a complete contract:

```yaml
version: 1
name: Frontend decisions
rules:
  - id: runtime-dependencies
    kind: dependencies
    reason: Keep runtime packages deliberate and explain each new dependency.
    scope: [package.json]
    dependencies:
      format: npm
      sections: [dependencies]
      allow: [react, react-dom]
    remediation: Remove the accidental package or propose a contract change.
```

The rule checks declared package names. It does not judge versions,
vulnerabilities or application behavior. Its allowlist is a project decision,
not a universal frontend recommendation.

Remove an accidental dependency and rerun the check. If the project deliberately
needs a new package, propose the dependency and contract change together. The
report keeps that policy change visible for review.

<p align="center">
  <a href="docs/assets/report-v0.4.0/mac/line-mac.png">
    <img src="docs/assets/report-v0.4.0/mac/line-mac.png" alt="The contract line in a Mac-style window on a vivid desktop: one required dependency rule crosses the boundary, four checks pass.">
  </a>
</p>

<p align="center"><sub>Four checks pass. One dependency crosses the boundary. Real v0.4.0 evidence in a Mac-style presentation. <a href="docs/assets/README.md">Capture notes</a></sub></p>

In the HTML report, each mark represents a declared check. Select a mark to
read its reason, evaluated rule, evidence and next step. Required failures rise
above the line; advisory failures sit below it. Hollow marks mean verification
is incomplete.

## What can a contract check?

| Rule | Example decision | What it checks |
| --- | --- | --- |
| [`require-files`](docs/configuration.md#require-files) | Keep a license and architecture document | Required paths and glob matches |
| [`forbid-text`](docs/configuration.md#forbid-text) | Keep debug markers out of shipped source | Go/RE2 patterns in selected text |
| [`dependencies`](docs/configuration.md#dependency-names) | Review new runtime packages | Declared names in npm, Go, Python and Cargo manifests |
| [`document`](docs/configuration.md#json-and-yaml-values) | Preserve an API operation or deployment setting | Selected JSON/YAML values |
| [`co-change`](docs/configuration.md#co-change) | Ship notes with a migration | Accompanying content edits between base and candidate |
| [`command`](docs/configuration.md#project-commands) | Run an existing architectural or policy verifier | Exit codes, dependency-cruiser or Conftest results |

Each rule records a reason, its inputs and optional remediation. Use
`required: false` for an advisory rule. Use `compare: new` on supported checks
to distinguish new, existing and resolved findings while blocking new debt;
`strict` is the default. [Configuration reference](docs/configuration.md)

Native checks inspect captured files without executing project code. They do
not infer architecture, resolve complete dependency graphs or establish runtime
correctness. External commands require installed tools and `--allow-exec`;
their disposable execution copy **is not a sandbox**.

## Check adopted rules. Preview proposed changes.

| | `check` | `preview` |
| --- | --- | --- |
| Contract source | Selected base | Candidate |
| Purpose | Enforce adopted policy | Validate proposed policy |
| Policy changes alone | Exit `1`; review required | Visible in the report; can pass |
| Adoption | Depends on the base your repository trusts | Never established by preview |

Once a contract has been adopted, set `BASE_SHA` to its reviewed base commit's
full SHA and keep it fixed while evaluating your change:

```sh
linebind check --base "$BASE_SHA" --html .linebind/check.html
linebind preview --base "$BASE_SHA" --html .linebind/preview.html
```

Both commands inspect the working tree by default. Add `--head "$HEAD_SHA"`
to evaluate a committed candidate. Editing the candidate allowlist does not
change the rules used by `check`.

A proposed allowlist can pass `preview` while `check` against the original base
still reports the policy change. Review and merge adopt the proposal; preview
never approves it. [Example report](docs/assets/report-v0.4.0/mac/preview-mac.png) ·
[Trust model](docs/trust-model.md)

## Tested in CI

[CI](.github/workflows/ci.yml) runs Go tests and vet on Linux, macOS and Windows,
checks release archives and installed binaries, and exercises examples, real
adapters and report interactions. [Release](.github/workflows/release.yml)
reuses that suite; [CodeQL](.github/workflows/codeql.yml) analyzes Go code.

The [repository contract gate](.github/workflows/contract.yml) uses `preview`
with explicit PR SHAs and a checksum-pinned released verifier. Required checks
must pass before merging. [Contributor checks and workflow roles](CONTRIBUTING.md#ci-and-merging)

The Linux x64 setup Action and contract gate are independently pinned to
**v0.5.0**. Select a version and matching checksum explicitly when upgrading.
[Set up the Action](docs/ci.md#install-with-the-setup-action) ·
[Our own contract](CONTRIBUTING.md#repository-contract)

## With your coding agent

Turn a decision from your conversation into a rule the next change can check.
The bundled skill gives your existing coding agent a workflow for three tasks:

- **Create a contract.** Start from decisions you have actually made, explain
  what each rule protects, and demonstrate a violation in an isolated fixture.
  Subjective goals without a meaningful verifier stay in human review.
- **Develop within it.** Run the checks, read their findings, and repair
  unintended deviations in the code. The instructions explicitly forbid
  weakening a rule just to make a check pass.
- **Evolve a decision.** When you authorize a change, prepare the code and
  contract together. Keep the old-policy result and proposed preview visible
  for the repository's review and adoption process.

For example, if the contract permits React and React DOM, adding Zod is a
reported deviation. The skill guides a repair if that addition was accidental,
or a narrow contract proposal if you have decided to adopt Zod.

After installing the CLI, run this in your project:

```sh
linebind setup --agent all
```

Install the bundled skill for **Codex, Claude Code and Antigravity**, or select
one with `--agent codex`, `claude` or `antigravity`. Add `--dry-run` to inspect
the changes first.

Installation copies the bundled instructions into the repository, offline,
and preserves existing guidance. It does not create or approve a contract.
Reload skills or start a new agent session if needed, then ask:

> Prepare a Linebind contract from our documented decisions. Explain what each
> rule protects and what remains unchecked. Demonstrate a permitted change and
> a violation in an isolated fixture.

Linebind adds no model account or API key. The skill guides the agent; the CLI
evaluates declared checks, and CI can run them independently. Installing a skill
does not guarantee that an agent session loads or follows it.

[Agent setup, everyday workflow and compatibility limits](docs/agents.md)

## Read the result correctly

`passed` covers the declared check; `crossed` reports a violation; `unverified`
means a complete result could not be established. Required results control the
exit status. A green result does not certify the whole repository.
[Exit codes](docs/configuration.md#status-and-command-line-reference) ·
[Report guide](docs/report.md) · [Trust model](docs/trust-model.md)

## Documentation and contributing

[Installation](docs/installation.md) · [Walkthrough](docs/try-it.md) ·
[Configuration](docs/configuration.md) · [Examples](examples/README.md) ·
[CI integration](docs/ci.md) ·
[Contributing](CONTRIBUTING.md)

The CLI is written in Go. Run `go test ./...` and `go vet ./...` for local
checks; see the contributing guide for packaging and release validation.

Reading this from a downloaded archive? The
[online README](https://github.com/mchl-schrdng/linebind#readme) includes the
linked guides and images.

---

<p align="center">
  Made by <a href="https://github.com/mchl-schrdng">mchl-schrdng</a> ·
  <a href="LICENSE">MIT</a> ·
  <a href="SECURITY.md">Security</a>
</p>
