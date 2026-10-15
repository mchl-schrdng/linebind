# The offline report

`linebind check --base REF --html report.html` writes the result of that run.
`linebind report result.json --output report.html` renders a saved JSON result
without repeating the analysis. Open the resulting HTML file locally; it needs
no server, account, external font or network resource.

`linebind preview --base REF --html preview.html` produces an explicitly
labelled **PREVIEW / NOT APPROVED** report using the candidate's proposed policy.
Its JSON has `"purpose": "preview"`, and `linebind report` preserves that label
when rendering it later. An earlier v0.1.0 binary rejects preview JSON as an
unknown format field. A passing preview is not a passing approved-policy gate.
See the [agent workflow](agents.md#what-preview-establishes).

Each check retains its reason, required or advisory setting, verification
method, declared scope, findings and diagnostics. New, existing and resolved
findings remain distinct. Contract changes appear separately from check
results. The provenance section records the revisions, input digest and
contract hash; these are traceability information, not an authentication seal.

With JavaScript enabled, select a check to read its details. Direct links such
as `#check-1` select the corresponding check. Without JavaScript, every check
remains readable in the page. The copy action copies plain-text diagnostics;
if clipboard access is unavailable, it selects the text for manual copying.
Press Escape to close that selection.

## Interaction checks

The Go renderer tests check escaping, evidence preservation and text output.
An optional DOM test generates a real report through the CLI, then checks its
script against an in-memory DOM with resource and network loading disabled.
It covers check navigation, hash changes, clipboard success and failure,
keyboard dismissal, focus restoration and content without JavaScript.

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
