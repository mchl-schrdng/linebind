# Use Linebind as a PR gate

[The repository's workflow](../.github/workflows/contract.yml) is a complete,
copyable GitHub Actions integration. Save it as `.github/workflows/contract.yml`
in a repository whose approved base already contains `.linebind.yml`.

It downloads the published Linux amd64 binary from
[v0.1.0](https://github.com/mchl-schrdng/linebind/releases/tag/v0.1.0), checks its
pinned SHA-256, and evaluates the PR head with the contract from the explicit PR
base SHA. The hash was checked against the published `SHA256SUMS` and archive
bytes. Updating the version also requires reviewing and updating that hash.

The job checks out the base and fetches the candidate as Git objects. It does
not build Linebind from the PR, install project dependencies, check out the
candidate, or enable command checks. A required command rule therefore produces
an unverified result in this workflow. Use a separately designed execution
environment before opting into those rules; adding `--allow-exec` alone does
not provide isolation.

## The command and its evidence

The workflow runs this command with full SHAs supplied by the PR event:

```bash
mkdir -p "$RUNNER_TEMP/linebind-report"
set -o pipefail
"$RUNNER_TEMP/linebind-bin/linebind" check \
  --repo "$GITHUB_WORKSPACE" \
  --base "$BASE_SHA" --head "$HEAD_SHA" \
  --format json --html "$RUNNER_TEMP/linebind-report/report.html" \
  | tee "$RUNNER_TEMP/linebind-report/result.json"
```

JSON is visible in the job log and saved beside the standalone HTML report.
`pipefail` preserves a failing verifier exit code through `tee`. There is no
`continue-on-error` or success fallback:

| Exit | Effect on this job |
| --- | --- |
| `0` | Required checks passed; advisory results remain visible |
| `1` | Failed: a required rule was crossed or a policy change needs review |
| `2` | Failed: required verification was incomplete or an operation failed |

An artifact named `linebind-pr-<number>-<head-sha>` keeps both reports for seven
days. The upload step uses `always()`, including after exits `1` and `2`. A
download/fetch failure can occur before a report exists, and runner cancellation
or an upload outage can still prevent retention. Uploading reports cannot turn
a failing gate into a pass.

Download and extract the artifact, then open `report.html` locally. Reports can
contain repository paths, rule text and findings; the repository's access and
artifact policies should fit that content.

## Make the check required

Adding the YAML does not change branch protection. After a real PR has run the
workflow, configure the target branch's ruleset or branch protection to require
the check named **Linebind contract**. Its job ID is `contract`. Keep that check
name unique across workflows, and select GitHub Actions as its expected source
where that setting is available. GitHub documents these settings under
[required status checks](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches#require-status-checks-before-merging).

Use required reviews for changes to `.github/workflows/`, `.github/CODEOWNERS`,
`.linebind.yml` and any declared policy files. For example, adapt these entries
to a real team with write access and enable required code-owner review:

```text
/.github/workflows/ @your-org/maintainers
/.github/CODEOWNERS @your-org/maintainers
/.linebind.yml @your-org/maintainers
/policy/ @your-org/maintainers
```

`CODEOWNERS` alone does not require approval. GitHub uses the base branch's file
for PR ownership, and recommends protecting that file itself. See
[GitHub's CODEOWNERS documentation](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners).

This is a `pull_request` workflow. Its workflow definition can be changed by a
PR, so review of the enforcement workflow and its action/binary pins is part of
the trust boundary. A required check does not establish that its own definition
was left intact. The scanner cannot protect a verification process an authorized
maintainer can replace. This example grants only `contents: read`; its automatic
GitHub token is used for the fetch, including private repositories, and is not
persisted in Git configuration. It requires no repository secrets.

`pull_request_target` is not a drop-in replacement: it runs in the base context,
with a different `GITHUB_SHA` and permission model. This example avoids a custom
status publisher and uses the ordinary PR check context. See GitHub's
[event definitions](https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows#pull_request).

## Bootstrap and policy changes

Review and merge the first contract into the trusted branch before making this
check required. A selected base without a valid contract returns exit `2`.
The workflow does not approve a contract by finding it in a PR.

A later contract or declared policy-file edit returns exit `1` even after human
review; the run still uses the old base policy. v0.1.0 has no approval label,
same-PR approval API or automatic bypass. Before enabling a required gate,
establish your separately controlled maintainer process for incorporating
approved policy changes. If that process has no authorized exception path,
the policy-change PR will remain blocked. Keep this process auditable and
separate from ordinary code approval. Never set `--base` to the unreviewed PR
head to manufacture a passing result. See the
[contract evolution rules](trust-model.md#evolving-the-contract).

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
the approved base explicitly there too; `--base "$GITHUB_SHA"` would accept any
contract change already present in that candidate. This PR workflow does not
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
