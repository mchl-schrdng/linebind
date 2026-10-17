#!/usr/bin/env bash
# Rebuild the README's disposable fixture reports with a trusted v0.4.0 binary.
set -euo pipefail

binary=${1:?Pass the absolute path to a trusted Linebind v0.4.0 executable}
output=${2:?Pass an absolute output directory}
case "$binary:$output" in /*:/*) ;; *) exit 2 ;; esac
"$binary" --version
mkdir -p "$output"
fixture_root=$(mktemp -d)
trap 'rm -rf "$fixture_root"' EXIT
mkdir -p "$fixture_root/react-storefront/src" "$fixture_root/react-storefront/deploy"
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
  - id: esm-modules
    kind: document
    reason: Tooling and imports rely on ECMAScript modules.
    scope: [package.json]
    document:
      selector: /type
      equals: module
  - id: project-readme
    kind: require-files
    reason: Keep the project entry point available to contributors.
    files:
      require: [README.md]
  - id: no-debug-markers
    kind: forbid-text
    reason: Temporary debug markers must not reach shipped source.
    scope: [src/**/*.js]
    text:
      pattern: DEBUG_ONLY
  - id: deployment-environment
    kind: document
    reason: Deployment configuration must name a supported environment.
    scope: [deploy/config.json]
    document:
      selector: /environment
      allowed-values: [staging, prod]
YAML

write_manifest() {
  cat > package.json <<'JSON'
{
  "name": "react-storefront",
  "private": true,
  "type": "module",
  "dependencies": {
JSON
  if [ -n "$1" ]; then
    printf '    "%s": "%s",\n' "$1" "$2" >> package.json
  fi
  cat >> package.json <<'JSON'
    "react": "19.0.0",
    "react-dom": "19.0.0"
  }
}
JSON
}

write_manifest '' ''
printf '# React storefront\n\nSynthetic README capture fixture; not a production application.\n' > README.md
printf 'export const title = "Storefront";\n' > src/app.js
printf '{"environment":"prod"}\n' > deploy/config.json
git add .
# Fixed fixture metadata makes the comparison commit reproducible.
GIT_AUTHOR_DATE=2026-10-17T00:00:00Z GIT_COMMITTER_DATE=2026-10-17T00:00:00Z \
  git -c user.name='Linebind fixture' -c user.email='fixture@example.invalid' \
  -c commit.gpgsign=false commit -qm 'Declare frontend fixture decisions'
base=$(git rev-parse HEAD)
"$binary" validate
"$binary" check --base "$base"

write_manifest jquery 3.7.1
status=0
"$binary" check --base "$base" --format json \
  --output "$output/report-crossed.json" --html "$output/report-crossed.html" || status=$?
test "$status" -eq 1

write_manifest zod 4.0.0
sed 's/allow: \[react, react-dom\]/allow: [react, react-dom, zod]/' .linebind.yml > proposed.yml
mv proposed.yml .linebind.yml
status=0
"$binary" check --base "$base" --format json \
  --output "$output/proposal-check.json" || status=$?
test "$status" -eq 1
"$binary" preview --base "$base" --format json \
  --output "$output/report-preview.json" --html "$output/report-preview.html"
printf 'Captured crossed check (exit 1), old-policy proposal check (exit 1), and preview (exit 0).\n'
