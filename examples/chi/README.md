# Chi: passing tests, a changed dependency decision

On 2026-10-21 we tested a controlled mutation against
[Chi at `9f8adb1aabaa8a91180a821111d3f8e668d1c0bf`](https://github.com/go-chi/chi/tree/9f8adb1aabaa8a91180a821111d3f8e668d1c0bf).
Its README says [“No external dependencies”](https://github.com/go-chi/chi/blob/9f8adb1aabaa8a91180a821111d3f8e668d1c0bf/README.md#L38).
We wrote a proposed Linebind rule for direct declarations in the root module,
then deliberately added a dependency and a small file importing it.

This is a local experiment, not an upstream bug, an autonomously invented agent
workaround, or evidence of adoption by Chi. The added function is compiled but
is not called by the router or its existing tests.

## Reproduce

Use Git, Go (tested with 1.27.2) and the released Linebind v0.5.1 executable on your
`PATH`. The commands below use a POSIX shell, create a disposable checkout,
download the pinned Go dependency and execute Chi's tests.

```sh
CHI_TRIAL=$(mktemp -d "${TMPDIR:-/tmp}/linebind-chi.XXXXXX")
CHI_BASE=9f8adb1aabaa8a91180a821111d3f8e668d1c0bf
git clone https://github.com/go-chi/chi.git "$CHI_TRIAL/chi"
cd "$CHI_TRIAL/chi"
git checkout --detach "$CHI_BASE"

cat > .linebind.yml <<'YAML'
version: 1
name: Chi root module declared dependencies
rules:
  - id: root-direct-dependencies
    kind: dependencies
    reason: 'README.md promises no external dependencies. This rule checks direct require declarations in the root module only; imports, indirect requirements and separate example modules are not verified.'
    scope: [go.mod]
    dependencies:
      format: go
      allow: []
    remediation: Remove the new direct requirement or explicitly review the project decision.
YAML

# The proposed rule passes before the mutation.
linebind preview --base "$CHI_BASE"

cat > linebind_probe.go <<'GO'
package chi

import "github.com/bmatcuk/doublestar/v4"

func linebindProbeMatch(pattern, name string) (bool, error) {
    return doublestar.Match(pattern, name)
}
GO

go mod edit -require=github.com/bmatcuk/doublestar/v4@v4.10.3
go mod download github.com/bmatcuk/doublestar/v4

go test -mod=readonly -count=1 ./...
go mod tidy -diff
linebind preview --base "$CHI_BASE"
# Expected: exit 1, with the finding below.
```

```text
Direct dependency is not permitted: github.com/bmatcuk/doublestar/v4
```

Observed with Go 1.27.2 and Linebind v0.5.1: both Go commands exit `0`;
Linebind exits `1`. The module has a real import and consistent `go.sum` entries.
This says nothing about whether the added helper implements a useful feature.
We use `preview` because this is our proposed contract; Chi has not adopted it.

## A limit the same experiment exposes

In the disposable copy, mark the requirement as indirect:

```sh
sed 's|^require github\.com/bmatcuk/doublestar/v4 v4\.10\.3$|& // indirect|' go.mod > go.mod.tmp
mv go.mod.tmp go.mod

go test -mod=readonly -count=1 ./...
go mod tidy -diff
linebind preview --base "$CHI_BASE"
```

The Go tests still pass. **Linebind now passes too**: its Go dependency parser
checks direct declarations and excludes `// indirect` requirements. But
`go mod tidy -diff` exits `1`, showing that this imported dependency should be
direct. The two checks catch different properties; retain both.

The rule also does not inspect imports or nested modules. These exclusions are
written into the contract itself. See the
[dependency parser](../../internal/contract/dependencies.go) for its implementation.
A dedicated CI script could enforce this single decision too; this example
shows the distinction between existing checks and an explicit project rule,
not a detection capability unique to Linebind.

If the decision itself needs to change, propose that change for review.
[The trusted-base tests](../../internal/app/check_test.go) and
[preview tests](../../internal/app/preview_test.go) cover why modifying a candidate
contract cannot replace the policy used by `check`, while `preview` can evaluate
the new proposal. This experiment does not test agent compliance or CI security.
