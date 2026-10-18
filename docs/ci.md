# Use Linebind as a PR gate

For the optional hosted report integration, see the
[Cloud pilot setup](cloud-pilot.md). Its informational GitHub App check is a
separate integration from the required CLI gate described here.

[The repository's workflow](../.github/workflows/contract.yml) is a complete,
copyable GitHub Actions integration. It validates the PR's code under the
**candidate contract** using `preview`. Rules may evolve in the same PR as code:
a contract edit alone does not fail CI. Merging adopts the reviewed proposal.

It downloads the published Linux amd64 binary from
[v0.4.1](https://github.com/mchl-schrdng/linebind/releases/tag/v0.4.1), verifies its
pinned SHA-256 and uses explicit PR base/head SHAs. The verifier is a previously
published release, not built from the PR. Updating it requires reviewing both
the version and checksum.

The job checks out the base and fetches the candidate as Git objects. It does
not install project dependencies, check out the candidate or enable command
checks. A required command rule therefore remains unverified. Use a separately
designed isolated execution environment before enabling those rules;
`--allow-exec` alone does not provide isolation.

## Install with the setup Action

The repository now includes a composite setup Action. On a Linux x64 runner,
replace the download/checksum/extract step with:

```yaml
- name: Install the published verifier
  uses: mchl-schrdng/linebind@7a721b149c7f37a9458b876d8280a7bbad2db766 # reviewed Action source
  with:
    version: v0.4.1
    sha256: ab9fddad704f6d28ee8d42315784166673336c8b780263bf9e4ff8b8cdd5827a
```

The example installs v0.4.1, verifies the archive against its pinned
SHA-256 before extracting or executing it, and adds `linebind` to `PATH`.
It also exposes an absolute `binary` output. It does not build the candidate,
install project dependencies, choose a base, change the checkout or execute
command rules. Keep the base/head selection, native check, artifact retention
and required-check settings from the full workflow above. For another published
release, supply both `version: vX.Y.Z` and its independently reviewed `sha256`.
A wrong checksum fails closed; there is no automatic `latest` update.

Pin the Action source independently of the CLI release it installs. The
examples supply both inputs because that immutable Action revision predates
the default-version update. The Action on `main` and this repository's contract
gate now default to v0.4.1; older pinned Action revisions retain their defaults.
Python/Cargo formats and the new document predicates require CLI v0.4.0 or newer.

For example, after the existing base checkout and candidate-fetch steps:

```yaml
- uses: mchl-schrdng/linebind@7a721b149c7f37a9458b876d8280a7bbad2db766 # reviewed Action source
  with:
    version: v0.4.1
    sha256: ab9fddad704f6d28ee8d42315784166673336c8b780263bf9e4ff8b8cdd5827a
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

The workflow runs this command with full SHAs supplied by the PR event:

```bash
mkdir -p "$RUNNER_TEMP/linebind-report"
set -o pipefail
"$RUNNER_TEMP/linebind-bin/linebind" preview \
  --repo "$GITHUB_WORKSPACE" \
  --base "$BASE_SHA" --head "$HEAD_SHA" \
  --format json --html "$RUNNER_TEMP/linebind-report/report.html" \
  | tee "$RUNNER_TEMP/linebind-report/result.json"
```

JSON is visible in the job log and saved beside the standalone HTML report.
`pipefail` preserves a failing exit through `tee`; there is no success fallback.

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

## Gate a frontend build

To prevent a build from starting when the contract fails, place the build job
in the **same workflow** and give it `needs: contract`. For a Node frontend
with a committed npm lockfile and a `build` script, add this under `jobs:`:

```yaml
  build:
    needs: contract
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          ref: ${{ github.event.pull_request.head.sha }}
          persist-credentials: false
      - uses: actions/setup-node@949feb2413d6458794dcd2491c4babbbce0c15c1 # v7.1.0
        with:
          node-version: '24'
          package-manager-cache: false
      - run: npm ci
      - run: npm run build
```

This separate build job intentionally executes candidate code, including npm
lifecycle scripts. Keep it on a disposable runner without deployment secrets.
The Linebind job itself remains native-only. A job in a different workflow does
not wait for this one merely because it has a similar name. GitHub's
[`needs` behavior](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#jobsjob_idneeds)
is the dependency that makes this example a build gate.

## Connect it to deployment

Keep production deployment in your existing trusted-branch pipeline. Require
the PR checks before merging, then verify the exact revision you will deploy
and make the deployment job depend on that verification and its build. Choose
the comparison revision and contract source explicitly there too. `preview`
validates the deploy candidate's policy; `check` enforces the selected base's policy. This PR workflow does not
itself validate a later merge commit or authorize a deployment.

For a deployment workflow that already has jobs named `contract`, `build` and
`deploy`, the relevant wiring is:

```yaml
  deploy:
    needs: [contract, build]
    environment: production
    # Keep the existing runs-on, permissions and deployment steps here.
```

This fragment is wiring, not a runnable deployment recipe: the deployment target,
credentials and approved production base belong to your project. Configure any
required approval on the `production` environment in repository settings; the
name alone does not create an approval rule. See GitHub's
[deployment controls](https://docs.github.com/en/actions/how-tos/deploy/configure-and-manage-deployments/control-deployments).

The supplied workflow evaluates the PR head against the base SHA recorded in
that event, not a synthetic merge result. Require the branch to be up to date
and obtain a fresh run when the base changes. There are no path filters that
silently omit documentation-only PRs. Merge queues need a separately implemented
and tested `merge_group` trigger and candidate/base selection; this PR-only
example is insufficient as their sole required check. GitHub explains that
additional event under
[`merge_group`](https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows#merge_group).
