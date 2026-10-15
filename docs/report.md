# The offline report

`linebind check --base REF --html report.html` writes the result of that run.
`linebind report result.json --output report.html` renders a saved JSON result
without repeating the analysis. Open the resulting HTML file locally; it needs
no server, account, external font or network resource.

`linebind preview --base REF --html preview.html` produces an explicitly
labelled **PREVIEW / NOT APPROVED** report using the candidate's proposed policy.
Its JSON has `"purpose": "preview"`, and `linebind report` preserves that label
when rendering it later. An earlier v0.1.0 binary rejects preview JSON as an
unknown format field. A repository can use preview to validate its proposed
contract in CI; a passing result does not approve that contract or establish
compliance with the base policy. The maintainer reviews the contract diff and
adopts it through the repository's merge process.
See the [agent workflow](agents.md#what-preview-establishes).

Each check retains its reason, required or advisory setting, verification
method, declared scope, findings and diagnostics. New, existing and resolved
findings remain distinct. Contract changes appear separately from check
results. A short explanation distinguishes blocking required results from
advisory results; incomplete verification stays visible. The capture time is
shown near the summary so a saved report is not mistaken for live status.
In an approved-base `check`, a contract-only change is titled **Contract review
required**, rather than suggesting that a rule was violated. Coexisting
violations or incomplete checks remain visible in the headline and evidence.
The provenance section records the revisions, input digest and
contract hash; these are traceability information, not an authentication seal.

With JavaScript enabled, the first required check needing attention opens by
default, followed by advisory issues if all required checks passed. Select any
check to read its details. **Needs attention** filters the navigation to crossed
and unverified checks, including advisory results. **All checks** restores the
full list. Direct links such as `#check-1` take precedence and reveal their check
even when it was filtered out. Without JavaScript, every check remains readable
in the page. Printing also includes every check.

The copy action always copies the complete plain-text diagnostic, including
checks hidden by the filter. If clipboard access is unavailable, it selects
the text for manual copying.
Press Escape to close that selection.

## Interaction checks

The Go renderer tests check escaping, evidence preservation and text output.
An optional DOM test generates a real report through the CLI, then checks its
script against an in-memory DOM with resource and network loading disabled.
It covers initial issue selection, filtering, hash changes, clipboard success
and failure, keyboard dismissal, focus restoration and content without
JavaScript. Renderer tests also distinguish required, advisory and unknown
outcomes without treating incomplete checks as passing.

With Node.js and npm available, these POSIX shell commands keep the test-only
dependency outside the checkout:

```sh
npm install --prefix /tmp/linebind-ui-test --no-save --ignore-scripts --no-audit --no-fund jsdom@26.1.0
go build -o ./bin/linebind ./cmd/linebind
NODE_PATH=/tmp/linebind-ui-test/node_modules node scripts/ui-test.mjs ./bin/linebind
```

These tests do not establish visual layout, real-browser clipboard permissions
or assistive-technology behavior. Browser and accessibility review remain
separate checks.

## Artwork and report images

The Linebind symbol and outlined wordmark in `docs/assets/` are original SVG
artwork shipped under the repository license. They use no external font or
third-party image. The report uses system fonts.

When adding a report image, capture an actual CLI-generated report and retain
its JSON/HTML, invocation, source revision and input digest with the capture
notes. Record the browser and viewport used. Identify a seeded example as a
fixture; do not present its findings as findings from a production repository.
An illustration or design study should be labeled as such, rather than as a
report screenshot. Redact sensitive inputs before generating shareable reports.
