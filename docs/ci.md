# Use Linebind as a PR gate

For the optional hosted report integration, see the
[Cloud pilot setup](cloud-pilot.md). Its informational GitHub App check is a
separate integration from the required CLI gate described here.

[The repository's workflow](../.github/workflows/contract.yml) is a complete,
copyable GitHub Actions integration. It validates the PR's code under the
**candidate contract** using `preview`. Rules may evolve in the same PR as code:
a contract edit alone does not fail CI. Merging adopts the reviewed proposal.

It downloads the published Linux amd64 binary from
[v0.5.0](https://github.com/mchl-schrdng/linebind/releases/tag/v0.5.0), verifies its
pinned SHA-256 and uses explicit PR base/head SHAs. The verifier is a previously
published release, not built from the PR. Updating it requires reviewing both
the version and checksum.

The job checks out the base and fetches the candidate as Git objects. It does
not install project dependencies, check out the candidate or enable command
checks. A required command rule therefore remains unverified. Use a separately
designed isolated execution environment before enabling those rules;
`--allow-exec` alone does not provide isolation.

## Install with the setup Action

The Linux x64 setup Action downloads a published verifier, checks its pinned
SHA-256 before extraction/execution, adds it to `PATH` and exposes an absolute
`binary` output. Pin the Action source independently of the installed CLI.
Supply both `version` and its reviewed `sha256` when selecting another release;
there is no automatic `latest` update.

After the base checkout and candidate-fetch steps from the full workflow:

```yaml
- uses: mchl-schrdng/linebind@7a721b149c7f37a9458b876d8280a7bbad2db766 # reviewed Action source
  with:
    version: v0.5.0
    sha256: eaa31c3379a9031c6d2178125bb341b591600f6b150e19d8e6bde50c831cb1dc
- name: Validate the proposal
  env:
    BASE_SHA: ${{ github.event.pull_request.base.sha }}
    HEAD_SHA: ${{ github.event.pull_request.head.sha }}
  run: |
    mkdir -p "$RUNNER_TEMP/linebind-report"
    linebind preview --base "$BASE_SHA" --head "$HEAD_SHA" \
      --format json --output "$RUNNER_TEMP/linebind-report/result.json" \
      --html "$RUNNER_TEMP/linebind-report/report.html"
```

Use `check` instead of `preview` when your workflow intentionally enforces
base policy. Keep artifact upload in an `if: always()` step. macOS, Windows
and Linux ARM64 users can use the [release archives](installation.md); the
initial Action deliberately targets the runner used by the existing PR gate.

## The command and its evidence

Use full base/head SHAs from the PR event. Preserve the command's exit status;
if piping through `tee`, enable `pipefail`. The repository workflow saves JSON
and standalone HTML together.

| Exit | Effect on this job |
| --- | --- |
| `0` | Required proposed checks passed; policy edits and advisory findings remain visible |
| `1` | Failed: a required proposed rule was crossed |
| `2` | Failed: required verification was incomplete, the contract was invalid/missing, or an operation failed |

An artifact named `linebind-pr-<number>-<head-sha>` keeps both reports for seven
days, including failures, via `always()`. Fetch/download failures can occur
before a report exists; runner cancellation or an upload outage can prevent
retention. Uploading evidence does not turn a failed job into a pass.

Download and extract the artifact, then open `report.html` locally. Reports
contain rule text, paths and findings, so review them before sharing publicly.
Reports retain **preview / not approved**: passing proposal validation is not
approval. The maintainer's merge is the adoption step.

## Make the check required

Workflow YAML alone does not prevent merging. Configure a branch ruleset to
require **Linebind contract** and the project's test/build checks, with GitHub
Actions as the expected source. Keep check names unique. See GitHub's
[required status checks](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches#require-status-checks-before-merging).

This repository's `main` uses **Main requires passing CI** with:

- Pull requests required and branches up to date before merging.
- Seven required checks: `package`, `minimum-go`, `external-tools`,
  `verify (ubuntu-latest)`, `verify (macos-latest)`, `verify (windows-latest)`
  and `Linebind contract`.
- No bypass actors, force pushes or branch deletion.
- Zero required approving reviews: the sole maintainer can merge after CI passes.

The separate [CodeQL workflow](../.github/workflows/codeql.yml) analyzes Go
packages built on Linux for PRs and pushes to `main`, and uploads findings to
GitHub code scanning. Its `CodeQL (Go)` job is not currently one of the seven
required checks. A passing workflow means analysis completed; review the
findings themselves in the Security tab.

A failed required test blocks merging, including for the owner and coding agents
under these rules. Administrators can still change repository settings; the
ruleset does not remove that administrative power. Never disable protections
or use an administrative merge to finish a task.

This is a `pull_request` workflow, so a PR can change its definition. Review
contract, test and workflow changes together: a green check cannot decide
whether removing a useful rule was legitimate. The job has only `contents: read`;
its automatic token fetches Git objects and is not persisted in Git config.
It needs no repository secrets. `pull_request_target` has different permissions
and event semantics and is not a drop-in replacement. See GitHub's
[event definitions](https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows#pull_request).

## Bootstrap and policy changes

An initial commit is needed for comparison; the base need not contain a
contract. Create the first `.linebind.yml` in a PR, validate its checks and merge
once required CI passes. Later code and contract changes use the same flow.
For example, deliberately adding Zod to both a dependency rule and the manifest
can pass, while adding an undeclared dependency still fails.

`check` remains useful as separate evidence: it evaluates the base contract and
returns exit `1` on policy edits. It does not supply this repository's merge
decision. The [agent skill](agents.md) retains that evidence alongside `preview`
when evolving a decision, without changing the comparison base to get green.

Teams that intentionally want to freeze the adopted policy can instead require
`check`, but must define their own policy-adoption process; otherwise legitimate
policy edits remain blocked. Neither command implements approval labels or a
local approval flag. See [the trust model](trust-model.md#evolving-the-contract).

## Builds, deployment and merge queues

A build job in the same workflow can use `needs: contract` to wait for the gate.
A similarly named job in another workflow does not create that dependency.
Builds execute candidate code and belong on disposable runners without
deployment secrets; the supplied Linebind gate remains native-only.

Keep deployment in your trusted-branch pipeline, verify its exact revision and
make deployment depend on verification and the build. This PR workflow does not
validate a later merge commit or authorize production deployment. Configure any
required environment approvals in repository settings.

The gate evaluates the event's PR head/base pair, not a synthetic merge result.
Require the branch to be up to date and obtain a fresh run when the base changes.
Merge queues need a separately implemented and tested `merge_group` trigger and
revision selection; this PR-only example is insufficient as their sole required
check. See GitHub's
[`needs`](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#jobsjob_idneeds)
and [`merge_group`](https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows#merge_group)
references.
