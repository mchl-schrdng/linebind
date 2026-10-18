# Connect a repository to the Cloud pilot

Linebind Cloud is an invite-only, free pilot at
<https://linebind-cloud.vercel.app>. Its GitHub check links to authenticated
adopted and proposed contract evidence. The check is informational: do not make
it a required merge gate during the pilot.

## Enrol the repository

The maintainer must enrol the repository owner's immutable GitHub account ID
and allowlist the exact reusable-workflow commit in Cloud. Install the
`linebind-cloud` GitHub App on the selected repository. A public workflow does
not enrol an account or give the App access to a repository by itself.

The workflow is distributed here so private and public repositories can call
it without access to the private Cloud application repository. It needs no
Cloud, database, Stripe or GitHub App secrets in the caller repository.

## Add the caller on the default branch

The maintainer supplies a reviewed, full 40-character commit SHA containing
`.github/workflows/cloud-pilot.yml`. Replace `REVIEWED_WORKFLOW_SHA` below with
that enrolled SHA; do not replace it with a branch or mutable tag.

```yaml
name: Linebind Cloud pilot
on:
  pull_request_target:
    types: [opened, synchronize, reopened, edited]
    branches: [main]
permissions:
  contents: read
  id-token: write
jobs:
  analyze:
    if: vars.LINEBIND_CLOUD_ENABLED == 'true'
    uses: mchl-schrdng/linebind/.github/workflows/cloud-pilot.yml@REVIEWED_WORKFLOW_SHA
```

Commit the caller to the default branch before opening a test PR. Adapt `main`
if the repository uses a different default branch. Enable the repository
variable `LINEBIND_CLOUD_ENABLED=true` after enrolment and App installation.
Create and review a `.linebind.yml` with explicit project decisions; see
[configuration](configuration.md) and [contract adoption](ci.md#bootstrap-and-policy-changes).

## Trust and results

The pinned reusable job runs on a GitHub-hosted runner. It checks out the
comparison base, fetches the candidate as Git objects, and runs the
checksum-verified v0.4.0 release. Reports include the evaluated rule definitions
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

The pilot supports native checks only. A required command check remains
unverified. Merge queues, pending-state invalidation, concurrent publication
and sustained outage recovery need further work before this can be a required
merge gate. Reports retain captured evidence; they do not continuously update.

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
