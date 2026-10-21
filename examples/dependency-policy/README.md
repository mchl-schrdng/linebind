# Passing tests, a changed dependency decision

This small synthetic project demonstrates one decision: no dependencies outside
the root Go module. We move a working implementation into a second local module
and import it. The behavior tests still pass, but the dependency rule reports
the new requirement. No upstream repository is used or downloaded.

This is a deliberately constructed example, not an observed agent incident.

## Reproduce

Use Git, Go and the released Linebind executable on your `PATH`. The POSIX shell
commands create a disposable directory. Both modules use only the standard
library, so no package download is needed. Tested with Go 1.27.2 and Linebind
v0.5.1.

```sh
DEPENDENCY_TRIAL=$(mktemp -d "${TMPDIR:-/tmp}/linebind-dependency.XXXXXX")
mkdir "$DEPENDENCY_TRIAL/app" "$DEPENDENCY_TRIAL/glob"
cd "$DEPENDENCY_TRIAL/app"
git init -q
go mod init example.com/dependency-demo

cat > match.go <<'GO'
package demo

import "path"

func Match(pattern, name string) (bool, error) {
    return path.Match(pattern, name)
}
GO

cat > match_test.go <<'GO'
package demo

import "testing"

func TestMatch(t *testing.T) {
    for name, want := range map[string]bool{"main.go": true, "notes.txt": false} {
        got, err := Match("*.go", name)
        if err != nil || got != want {
            t.Fatalf("Match(%q): got %v, %v; want %v", name, got, err, want)
        }
    }
}
GO

git add go.mod match.go match_test.go
git -c user.name="Linebind example" -c user.email="example@example.invalid" \
    -c commit.gpgsign=false -c core.hooksPath=/dev/null commit -qm "Baseline"
BASE_SHA=$(git rev-parse --verify 'HEAD^{commit}')

cat > .linebind.yml <<'YAML'
version: 1
name: Root module dependency policy
rules:
  - id: root-direct-dependencies
    kind: dependencies
    reason: 'Keep this module free of external dependencies. This rule checks direct require declarations in the root go.mod only; imports, indirect requirements, replacements and dependency implementations are not verified.'
    scope: [go.mod]
    dependencies:
      format: go
      allow: []
    remediation: Remove the new direct requirement or explicitly review the project decision.
YAML

# Baseline: all three commands pass.
go test -mod=readonly -count=1 ./...
go mod tidy -diff
linebind preview --base "$BASE_SHA"

# Move the same implementation into a separate local module.
cd "$DEPENDENCY_TRIAL/glob"
go mod init example.com/glob
sed 's/^package demo$/package glob/' ../app/match.go > match.go
cd "$DEPENDENCY_TRIAL/app"

cat > match.go <<'GO'
package demo

import "example.com/glob"

func Match(pattern, name string) (bool, error) {
    return glob.Match(pattern, name)
}
GO

go mod edit -require=example.com/glob@v0.0.0 -replace=example.com/glob=../glob
go test -mod=readonly -count=1 ./...
go mod tidy -diff
linebind preview --base "$BASE_SHA"
# Expected: tests 0, tidy 0, preview 2 (Not verified).
```

The final preview reports the forbidden declaration, but returns **Not verified**
(exit `2`) because the local replacement's package identity is unverified:

```text
Direct dependency is not permitted: example.com/glob
```

This is not a `Crossed` result: the report explicitly preserves that uncertainty.
The added module is local, but it is still a dependency outside the root module.
The existing tests exercise it through `Match`; their assertions are unchanged.
This demonstrates a declared dependency boundary, not a behavior regression.
The saved base is a comparison baseline. `preview` evaluates our proposed
contract; neither the initial commit nor this preview adopts it.

## A limit the same example exposes

In the same disposable directory, mark the requirement as indirect:

```sh
sed 's|^require example\.com/glob v0\.0\.0$|& // indirect|' go.mod > go.mod.tmp
mv go.mod.tmp go.mod

go test -mod=readonly -count=1 ./...
go mod tidy -diff
linebind preview --base "$BASE_SHA"
# Expected: tests 0, tidy 1, preview 0.
```

The Go tests still pass. **Linebind now passes too**: its Go dependency parser
checks direct declarations and excludes `// indirect` requirements. But
`go mod tidy -diff` reports that the imported dependency should be direct.
Keep both checks; they test different properties.

The contract also does not inspect source imports, replacements or dependency
implementations. See the [dependency parser](../../internal/contract/dependencies.go).
A dedicated CI script could enforce this single decision too; the example does
not establish a detection capability unique to Linebind.

If the decision itself needs to change, propose that change for review.
[Trusted-base tests](../../internal/app/check_test.go) and
[preview tests](../../internal/app/preview_test.go) cover why a candidate contract
cannot replace the policy used by `check`, while `preview` can evaluate it.
This example does not test agent compliance or CI security.
