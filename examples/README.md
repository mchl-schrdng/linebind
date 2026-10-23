# Reproducible examples

These are synthetic fixtures with seeded changes, not production applications
or evidence of adoption. For a first run needing only Git and the installed
CLI, use the [disposable walkthrough](../docs/try-it.md).

| Fixture | Declared decision | Limit |
| --- | --- | --- |
| [Frontend](frontend/base/.linebind.yml) | Allow React and React DOM runtime declarations | No import resolution, bundle behavior or package safety |
| [Backend](backend/base/.linebind.yml) | Keep the declared `GET /orders` response | No general API compatibility or actual server behavior |
| [Data](data/base/.linebind.yml) | Declare `order_id` non-null in the schema | No check of actual rows |
| [Infrastructure](infrastructure/base/.linebind.yml) | Request `runAsNonRoot: true` at pod level | No container override, image or live-cluster check |
| [Co-change](co-change/README.md) | Ship migration notes with SQL changes | Checks an accompanying edit, not its quality |
| [dependency-cruiser](dependency-cruiser/README.md) | Enforce declared module boundaries with a specialist tool | Requires the external tool and execution opt-in |
| [Conftest](conftest/README.md) | Evaluate declared infrastructure policy | Requires the external tool and execution opt-in |

For a standalone Go demonstration, see
[Passing tests, a changed dependency decision](dependency-policy/README.md).
It also demonstrates a case that Linebind misses and `go mod tidy` catches.

## Run the suite

From the Linebind checkout with Git and Go installed:

```sh
go build -o ./bin/linebind-dev ./cmd/linebind
go run ./scripts/pilots -binary ./bin/linebind-dev
```

Keep `./bin/linebind` for the trusted released verifier used by this repository's
contract workflow. On Windows, use `./bin/linebind-dev.exe`. To exercise a release archive, pass its
extracted executable instead. `-examples /path/to/linebind/examples` selects
the fixtures when running elsewhere. No project dependencies are installed.

The runner creates temporary Git repositories and checks passed (exit `0`),
crossed (`1`), incomplete (`2`) and removed-policy cases for each native fixture.
Co-change adds a repaired case: 21 native scenarios in total. It verifies base
identity, contract digest, JSON verdict and saved JSON-to-HTML round trips.
The removed-policy cases must still enforce the base's rules.

The printed evidence directory retains repositories, reports and `results.json`,
including binary version/hash and platform. `-out /path/to/new-directory` sets
its location and refuses an existing destination. CI runs these scenarios
against extracted archives on Linux, macOS and Windows. The specialist-tool
fixtures run separately in CI; see [Contributing](../CONTRIBUTING.md).

## Check actual SQLite rows

```sh
go run ./scripts/pilots -binary ./bin/linebind-dev -data-command
```

This adds six cases using Python's standard-library `sqlite3`; no pip package or
server is needed. The [trusted checker](data-command/base/checks/check_orders.py)
loads a fixed CSV and evaluates candidate `transform.sql`, requiring nonempty
results with unique, non-null `order_id` values. The checker and CSV are base
`policy-files`; the SQL is candidate input.

| Candidate | Expected result |
| --- | --- |
| Original transformation | Passed, `0` |
| Duplicate or null keys | Crossed, `1` |
| Malformed SQL or attempted write | Unverified, `2` |
| Original query without execution opt-in | Unverified, `2` |

The checker permits bounded read queries over the fixture and selected SQLite
functions; it rejects writes and attachments. Those restrictions belong to this
example, not an OS sandbox supplied by Linebind. The runner opts into execution
only for the five intended command cases and records the Python version.
Passing three fixture rows does not establish production data quality.

With `-data-command`, the suite runs 27 scenarios without fetching another repository.
