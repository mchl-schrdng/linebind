#!/usr/bin/env bash
set -euo pipefail
binary=${1:?Absolute path to the trusted Linebind v0.4.0 executable}
output=${2:?Absolute output directory}
case "$binary:$output" in /*:/*) ;; *) exit 2 ;; esac
mkdir -p "$output"
fixture_root=$(mktemp -d)
trap 'rm -rf "$fixture_root"' EXIT
mkdir "$fixture_root/react-storefront"
cd "$fixture_root/react-storefront"
git init -q -b main
cat > .linebind.yml <<'YAML'
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
YAML
printf '/.linebind/\n' > .gitignore
printf '{"dependencies":{"react":"19.0.0","react-dom":"19.0.0"}}\n' > package.json
git add .
GIT_AUTHOR_DATE=2026-10-17T00:00:00Z GIT_COMMITTER_DATE=2026-10-17T00:00:00Z \
  git -c user.name='Linebind fixture' -c user.email='fixture@example.invalid' \
  -c commit.gpgsign=false commit -qm 'Declare README dependency fixture'
"$binary" check --base main > /dev/null
printf '{"dependencies":{"react":"19.0.0","react-dom":"19.0.0","jquery":"3.7.1"}}\n' > package.json
printf 'linebind check --base main --html .linebind/report.html\n' > "$output/cli-command.txt"
status=0
"$binary" check --base main --html .linebind/report.html > "$output/cli.txt" || status=$?
test "$status" -eq 1
printf '%s\n' "$status" > "$output/cli-exit.txt"
cp .linebind/report.html "$output/cli.html"
status=0
"$binary" check --base main --format json --output "$output/cli.json" || status=$?
test "$status" -eq 1
