# Try a contract in a disposable frontend repository

This is a synthetic dependency-manifest example, not a running React app.
It needs only Git and an [installed Linebind binary](installation.md), with
`linebind` on `PATH`. No Go, Node.js, package installation or model API key.
The commands below use a POSIX shell (macOS, Linux or Git Bash on Windows).
They create a temporary repository and do not change your existing projects.

## Start with one decision

```sh
DEMO_DIR=$(mktemp -d "${TMPDIR:-/tmp}/linebind-demo.XXXXXX")
cd "$DEMO_DIR"
git init -q
printf '/.linebind/\n' > .gitignore
cat > package.json <<'JSON'
{"private":true,"dependencies":{"react":"19.0.0","react-dom":"19.0.0"}}
JSON
cat > .linebind.yml <<'YAML'
version: 1
name: Frontend runtime dependencies
rules:
  - id: runtime-dependencies
    kind: dependencies
    reason: Keep runtime packages deliberate and explain each new dependency.
    remediation: Remove the accidental package or propose a contract change with its reason.
    scope: [package.json]
    dependencies:
      format: npm
      sections: [dependencies]
      allow: [react, react-dom]
YAML
git add .
git -c user.name='Linebind demo' -c user.email='demo@example.invalid' \
  -c commit.gpgsign=false commit -qm 'Record the demo decision'
BASE_SHA=$(git rev-parse HEAD)
linebind preview --base "$BASE_SHA" --html .linebind/01-passed.html
```

Exit `0`: both declared runtime dependencies are allowed. The commit establishes
this demo's comparison point; creating a commit is not a substitute for your
real repository's review process.

## Introduce an accidental dependency

```sh
cat > package.json <<'JSON'
{"private":true,"dependencies":{"react":"19.0.0","react-dom":"19.0.0","jquery":"3.7.1"}}
JSON
linebind preview --base "$BASE_SHA" --html .linebind/02-crossed.html
```

Exit `1`: jQuery is outside the rule. Open `.linebind/02-crossed.html` in your
browser. It shows the dependency, the decision's reason and the next action.
The report is an offline snapshot; regenerate it after edits.

## Correct the unintended change

```sh
cat > package.json <<'JSON'
{"private":true,"dependencies":{"react":"19.0.0","react-dom":"19.0.0"}}
JSON
linebind preview --base "$BASE_SHA" --html .linebind/03-fixed.html
```

Exit `0`. The implementation respects the same rule again.

## Distinguish policy review from a code violation

Add only a comment to the contract, leaving the manifest compliant:

```sh
printf '\n# This rule covers declarations in the root manifest.\n' >> .linebind.yml
linebind check --base "$BASE_SHA" --agent
```

Exit `1`: the policy file changed. Starting with v0.5.2, the text report includes:

```text
Summary: required checks: 1 passed, 0 crossed, 0 unverified; policy files changed: 1.
Next action: Review policy changes separately; use preview with the same base to evaluate the proposal, not approve it.
```

There is no rule violation to repair. Even a comment changes the policy file;
the report's rule-definition comparison distinguishes it from a changed rule.
Evaluate the proposal separately:

```sh
linebind preview --base "$BASE_SHA" --agent
```

Exit `0`, explicitly marked **PREVIEW / NOT APPROVED**. The comparison base
remains fixed; your repository's review process governs adoption. Do not change
the base or weaken a rule just to make the earlier `check` return `0`.

If a required violation and incomplete verification coexist, the summary lists
both even though exit `1` takes precedence. Advisory findings are counted
separately and do not block the exit code. Read the detailed findings and
diagnostics to decide the actual correction.

## Deliberately evolve the decision

Suppose the maintainer now wants Zod for form validation. Change the manifest
and explain that decision in the contract:

```sh
cat > package.json <<'JSON'
{"private":true,"dependencies":{"react":"19.0.0","react-dom":"19.0.0","zod":"4.0.0"}}
JSON
cat > .linebind.yml <<'YAML'
version: 1
name: Frontend runtime dependencies
rules:
  - id: runtime-dependencies
    kind: dependencies
    reason: React renders the UI; Zod validates form inputs. Keep other runtime additions explicit.
    remediation: Remove the accidental package or propose a contract change with its reason.
    scope: [package.json]
    dependencies:
      format: npm
      sections: [dependencies]
      allow: [react, react-dom, zod]
YAML
linebind preview --base "$BASE_SHA" --html .linebind/04-proposed.html
```

Exit `0`, with the policy difference still visible. Our PR workflow uses this
validation: the maintainer reviews and merges the passing proposal to adopt it.
The software cannot judge whether allowing a new package was a good decision.
To see what the original contract would reject, run separately:

```sh
linebind check --base "$BASE_SHA" --html .linebind/05-old-policy.html
printf 'Demo files: %s\n' "$DEMO_DIR"
```

That last check returns `1`: the old policy still rejects Zod and records the
contract edit. It is comparison evidence, not the proposed-policy CI result.
The example checks dependency **names**, not those versions' compatibility,
safety, runtime behavior or bundle size. These package versions are fixture
values, not installation recommendations.
