# Connect a repository to Linebind Cloud

Linebind Cloud is the hosted companion at <https://linebind-cloud.vercel.app>.
Its GitHub check links to authenticated adopted and proposed contract evidence.
The check is informational: do not make it a required merge gate.

The CLI is free and open source. Cloud is free for connected public and private
repositories. Local checks and the agent skill work without Cloud.

## Connect GitHub

Connect [Linebind Cloud through GitHub Marketplace](https://github.com/marketplace/linebind-cloud),
then install the App on the selected repositories. An organization may require its owner's approval.
GitHub returns you to your [workspace](https://linebind-cloud.vercel.app/dashboard).
Sign in with GitHub; no separate Linebind account is needed.
The App reads contents, pull requests and Actions metadata, and writes checks.
It cannot push code or change repository settings. Your own repository permissions
are checked whenever you open a private report.

The workflow is public so both private and public repositories can call it.
No Cloud, database or GitHub App secrets are needed in your repository.

## Add the contract and workflow

If the repository does not yet have a contract, run `linebind init`, edit
`.linebind.yml` to reflect explicit project decisions, then run `linebind validate`.
Review and commit it to the default branch. See
[configuration](configuration.md) and [contract adoption](ci.md#bootstrap-and-policy-changes).

Save this exact caller as `.github/workflows/linebind.yml` on the default branch:

```yaml
name: Linebind
on:
  pull_request_target:
    types: [opened, synchronize, reopened, edited]
permissions:
  contents: read
  id-token: write
jobs:
  analyze:
    uses: mchl-schrdng/linebind/.github/workflows/cloud-pilot.yml@85d5a32f40de4dfb2da35ad587867cef8b5396a4
```

This reviewed workflow is pinned to a full commit SHA. Do not substitute a branch
or mutable tag, add untrusted steps, or pass repository secrets. No activation
variable is needed for this caller. Existing callers that use
`LINEBIND_CLOUD_ENABLED` can keep it set to `true` or adopt the caller above.

After both setup files are on the default branch, open a pull request or push a
new commit to an existing one. **Linebind / Analysis** links to its report.
Sign in with GitHub to read it or open your [workspace](https://linebind-cloud.vercel.app/dashboard).

If no report appears, check the App's repository selection, that Actions is
enabled, and that both files are on the default branch. Organization policies
may require an administrator to allow the reusable workflow. Inspect the Linebind
run in Actions and rerun a failed job after fixing its cause. If the captured
revisions have changed, push a new commit for fresh evidence. Reopening an old
pull request may retain the old comparison base in GitHub's event.

## Trust and results

The pinned reusable job runs on a GitHub-hosted runner. It checks out the
comparison base, fetches the candidate as Git objects, and runs the
checksum-verified v0.5.0 release. Reports include the evaluated rule definitions
and added, removed or modified rules, so the hosted review shows the same
evidence as the CLI. It does not execute candidate code, install
project dependencies, or enable command checks. Do not add untrusted steps
to this privileged `pull_request_target` job.

Only the fixed Cloud destination receives the short-lived GitHub Actions OIDC
token. Cloud verifies the workflow version, owner, repository, run attempt,
current open PR and both commit SHAs before publishing a check. The App's
installation access never substitutes for a report visitor's GitHub access.

An Actions job succeeding means evidence was submitted. Inspect the separate
**Linebind / Analysis** check for the proposed contract's result: a required
violation or incomplete required verification produces a failed analysis.
Policy changes can pass the proposed view while the adopted view still records
the old-policy findings. A preview does not approve a contract change.

Cloud supports native checks only. A required command check remains
unverified. Merge queues, pending-state invalidation and a timely recovery
schedule need further work before this can be a required merge gate. Reports retain captured evidence; they do not continuously update.

Uninstalling the App revokes access to its reports. A new installation starts a
new report history; it does not reconnect links to the former installation.
See the [privacy page](https://linebind-cloud.vercel.app/privacy) for stored data
and retention details.

## Exercise the integration

Use a disposable repository with a reviewed baseline contract and no real
customer data. Open independent PRs for an allowed change, a required-rule
violation, invalid input and an explicit contract change. Confirm each check's
conclusion and open its report. Correct a failing PR and verify the new head is
analysed. Re-running its old run after that change must reject stale evidence.

Also verify anonymous sign-in, permission denial for a different GitHub user
without repository access, and report access after App access is removed.
Record the PR, base/head, run, report URL and observed result for each case.
Keep intentional failing PRs unmerged.
