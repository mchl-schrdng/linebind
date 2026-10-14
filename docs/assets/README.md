# Artwork and evidence

The Linebind symbol and wordmark are original vector artwork, supplied in light
and dark variants under this repository's MIT license. The brackets represent
an explicit boundary; the line ties the two sides together.

`report-desktop.svg` is an editorial illustration of the report on a
macOS-style desktop, not a browser or operating-system screenshot. The window
layout is adapted from the actual report template. Its finding, rule, reason,
scope, counts and remediation come from a real CLI run of the included
`examples/frontend` fixture with its `crossed` mutation. No user desktop or
private project was captured. The original JSON evidence is
`frontend-crossed.json`; its development version and timestamp are retained.

The illustrated rule is strict, so its finding correctly reads “Not comparable”.
The candidate reference is shortened to “working tree” for the composition;
the JSON keeps its full identity. The PNG is a raster export of this SVG.

Reproduce the source scenario with the installed CLI:

```sh
go run ./scripts/pilots -binary /absolute/path/to/linebind
```

This produces fresh JSON and actual standalone HTML reports. The illustration
does not establish browser rendering or interaction compatibility. DOM-based
tests and native report tests are separate from visual browser QA.
