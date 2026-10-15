package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mchl-schrdng/linebind/internal/contract"
	"github.com/mchl-schrdng/linebind/internal/model"
	"github.com/mchl-schrdng/linebind/internal/repository"
)

const testContract = `version: 1
name: test
rules:
  - id: no-banned
    kind: forbid-text
    reason: Only the approved API is allowed.
    scope: ["src/**/*.txt"]
    compare: new
    text:
      pattern: banned
`

func testGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Linebind Test", "GIT_AUTHOR_EMAIL=test@example.invalid", "GIT_COMMITTER_NAME=Linebind Test", "GIT_COMMITTER_EMAIL=test@example.invalid")
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, b)
	}
	return strings.TrimSpace(string(b))
}
func put(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}
func checkRepo(t *testing.T, policy, source string) (string, Options) {
	t.Helper()
	dir := t.TempDir()
	testGit(t, dir, "init", "-q")
	put(t, dir, ".linebind.yml", policy)
	put(t, dir, "src/code.txt", source)
	testGit(t, dir, "add", ".")
	testGit(t, dir, "commit", "-qm", "approved base")
	return dir, Options{Repo: dir, Base: testGit(t, dir, "rev-parse", "HEAD"), Head: "worktree", Contract: ".linebind.yml", Version: "test"}
}
func TestApprovedPolicyCannotAuthorizeItsOwnRemoval(t *testing.T) {
	for _, candidatePolicy := range []string{"", strings.ReplaceAll(testContract, "pattern: banned", "pattern: permitted"), "invalid: ["} {
		t.Run(candidatePolicy, func(t *testing.T) {
			dir, opt := checkRepo(t, testContract, "allowed\n")
			put(t, dir, "src/code.txt", "banned\n")
			if candidatePolicy == "" {
				if err := os.Remove(filepath.Join(dir, ".linebind.yml")); err != nil {
					t.Fatal(err)
				}
			} else {
				put(t, dir, ".linebind.yml", candidatePolicy)
			}
			before := testGit(t, dir, "status", "--porcelain=v1")
			r := Check(opt)
			if r.ExitCode != 1 || r.Status != "crossed" || len(r.Changes) != 1 || len(r.Checks) != 1 || len(r.Checks[0].Findings) != 1 {
				t.Fatalf("candidate policy bypass: %+v", r)
			}
			if testGit(t, dir, "status", "--porcelain=v1") != before {
				t.Fatal("check modified checkout")
			}
		})
	}
}
func TestNewComparisonEvolutionAndReintroduction(t *testing.T) {
	dir, opt := checkRepo(t, testContract, "banned\n")
	put(t, dir, "src/code.txt", "\n\nbanned\n")
	r := Check(opt)
	if r.ExitCode != 0 || r.Checks[0].Findings[0].Evolution != "existing" {
		t.Fatalf("moved line not existing: %+v", r)
	}
	put(t, dir, "src/code.txt", "banned\nbanned\n")
	r = Check(opt)
	if r.ExitCode != 1 || len(r.Checks[0].Findings) != 2 || r.Checks[0].Findings[1].Evolution != "new" {
		t.Fatalf("duplicate hidden: %+v", r)
	}
	put(t, dir, "src/code.txt", "allowed\n")
	r = Check(opt)
	if r.ExitCode != 0 || r.Checks[0].Findings[0].Evolution != "resolved" {
		t.Fatalf("resolution wrong: %+v", r)
	}
	testGit(t, dir, "add", ".")
	testGit(t, dir, "commit", "-qm", "fix")
	opt.Base = testGit(t, dir, "rev-parse", "HEAD")
	put(t, dir, "src/code.txt", "banned\n")
	r = Check(opt)
	if r.ExitCode != 1 || r.Checks[0].Findings[0].Evolution != "new" {
		t.Fatalf("reintroduction hidden: %+v", r)
	}
}
func TestIncompleteNativeEvidenceCannotPassBaseline(t *testing.T) {
	dir, opt := checkRepo(t, testContract, "banned\n")
	put(t, dir, "src/unreadable.txt", string([]byte{0xff}))
	r := Check(opt)
	if r.ExitCode != 2 || r.Status != "unverified" || len(r.Checks[0].Findings) != 1 || r.Checks[0].Findings[0].Evolution != "not_comparable" {
		t.Fatalf("incomplete scan became green: %+v", r)
	}
}
func TestMissingBaseScopeAndUntrustedExecution(t *testing.T) {
	dir, opt := checkRepo(t, testContract, "okay\n")
	opt.Base = "does-not-exist"
	if r := Check(opt); r.ExitCode != 2 || len(r.Diagnostics) == 0 {
		t.Fatal(r)
	}
	opt.Base = "HEAD"
	if err := os.Remove(filepath.Join(dir, "src/code.txt")); err != nil {
		t.Fatal(err)
	}
	if r := Check(opt); r.ExitCode != 2 {
		t.Fatalf("empty scope passed: %+v", r)
	}
	policy := `version: 1
name: external
rules:
  - id: tool
    kind: command
    reason: Check with the approved tool.
    scope: ["src/*.txt"]
    command:
      argv: [definitely-missing-linebind-test-tool]
`
	_, opt = checkRepo(t, policy, "okay\n")
	if r := Check(opt); r.ExitCode != 2 || !strings.Contains(r.Checks[0].Diagnostics[0], "--allow-exec") {
		t.Fatalf("command did not require opt-in: %+v", r)
	}
	opt.AllowExec = true
	if r := Check(opt); r.ExitCode != 2 {
		t.Fatalf("missing executable passed: %+v", r)
	}
}
func TestOmittedDirectoryCannotHideScopedSources(t *testing.T) {
	rule := contract.Rule{Scope: []string{"src/**/*.txt"}}
	c := model.Check{Status: "passed"}
	s := &repository.Snapshot{Omitted: map[string]string{"src/hidden": "symlinks and submodules are not inspected"}}
	markOmissions(rule, s, &c)
	if c.Status != "unverified" {
		t.Fatal(c)
	}
	c = model.Check{Status: "passed"}
	s.Omitted = map[string]string{"docs/link": "symlinks and submodules are not inspected"}
	markOmissions(rule, s, &c)
	if c.Status != "passed" {
		t.Fatal("unrelated omission blocks native scope")
	}
}
func TestAggregateGateKeepsAdvisoryAndRequiredDistinct(t *testing.T) {
	for _, tc := range []struct {
		checks []model.Check
		status string
		exit   int
	}{
		{[]model.Check{{Status: "passed", Required: true}, {Status: "crossed"}}, "crossed", 0},
		{[]model.Check{{Status: "unverified", Required: true}, {Status: "crossed", Required: true}}, "crossed", 1},
		{[]model.Check{{Status: "crossed"}, {Status: "unverified", Required: true}}, "crossed", 2},
	} {
		r := model.Report{Checks: tc.checks}
		finalize(&r)
		if r.ExitCode != tc.exit || r.Status != tc.status {
			t.Fatal(r)
		}
	}
}

func TestCheckoutLineEndingsDoNotMoveThePolicy(t *testing.T) {
	dir, opt := checkRepo(t, testContract, "okay\n")
	put(t, dir, ".linebind.yml", strings.ReplaceAll(testContract, "\n", "\r\n"))
	if r := Check(opt); r.ExitCode != 0 || len(r.Changes) != 0 {
		t.Fatalf("CRLF checkout changed policy: %+v", r)
	}
	put(t, dir, ".linebind.yml", testContract+"# reviewed separately\n")
	if r := Check(opt); r.ExitCode != 1 || len(r.Changes) != 1 {
		t.Fatalf("comment change hidden: %+v", r)
	}
}

func TestRequiredFileOmissionsRespectExplicitScope(t *testing.T) {
	rule := contract.Rule{Scope: []string{"public/*.md"}, Files: &contract.FilesRule{Require: []string{"**/*.md"}}}
	for _, tc := range []struct{ omitted, want string }{
		{"private/link.md", "passed"},
		{"private", "passed"},
		{"public/link.md", "unverified"},
		{"public", "unverified"},
	} {
		t.Run(tc.omitted, func(t *testing.T) {
			c := model.Check{Status: "passed", Files: 1}
			s := &repository.Snapshot{Omitted: map[string]string{tc.omitted: "symlinks and submodules are not inspected"}}
			markOmissions(rule, s, &c)
			if c.Status != tc.want {
				t.Fatalf("omitted %s: want %s, got %+v", tc.omitted, tc.want, c)
			}
		})
	}
	rule.Files.Require = []string{"public/README.md"}
	c := model.Check{Status: "passed", Files: 1}
	markOmissions(rule, &repository.Snapshot{Omitted: map[string]string{"public/optional.md": "file exceeds capture limit"}}, &c)
	if c.Status != "passed" {
		t.Fatalf("unrequired file affected presence evidence: %+v", c)
	}
}

func TestDocumentNewComparisonIgnoresIntegerFormatting(t *testing.T) {
	policy := `version: 1
name: Values
rules:
  - id: value
    kind: document
    reason: Do not add forbidden values.
    scope: [config.yaml]
    compare: new
    document:
      selector: /mode
      forbidden-values: [80, 81, "80"]
`
	dir, opt := checkRepo(t, policy, "allowed\n")
	put(t, dir, "config.yaml", "mode: 0x50\n")
	testGit(t, dir, "add", ".")
	testGit(t, dir, "commit", "-qm", "existing forbidden value")
	opt.Base = "HEAD"
	for _, tc := range []struct{ value, evolution string }{{"80", "existing"}, {"81", "new"}, {`"80"`, "new"}} {
		put(t, dir, "config.yaml", "mode: "+tc.value+"\n")
		r := Check(opt)
		if len(r.Checks) != 1 || len(r.Checks[0].Findings) == 0 || r.Checks[0].Findings[0].Evolution != tc.evolution {
			t.Fatalf("value %s: %+v", tc.value, r)
		}
		if (r.ExitCode == 0) != (tc.evolution == "existing") {
			t.Fatalf("value %s produced the wrong gate: %+v", tc.value, r)
		}
	}
}

func TestRoundedDocumentValuesCannotPassNewComparison(t *testing.T) {
	policy := `version: 1
name: Numeric comparison
rules:
  - id: value
    kind: document
    reason: Values must remain explicit.
    scope: [config.json]
    compare: new
    document:
      selector: /mode
      equals: safe
`
	for _, pair := range [][2]string{
		{"18446744073709551616", "18446744073709551617"},
		{"0.100000000000000001", "0.100000000000000002"},
	} {
		dir, opt := checkRepo(t, policy, "allowed\n")
		put(t, dir, "config.json", `{"mode":`+pair[0]+`}`)
		testGit(t, dir, "add", ".")
		testGit(t, dir, "commit", "-qm", "numeric baseline")
		opt.Base = "HEAD"
		put(t, dir, "config.json", `{"mode":`+pair[1]+`}`)
		for _, evaluate := range []func(Options) model.Report{Check, Preview} {
			if got := evaluate(opt); got.ExitCode != 2 || got.Status != "unverified" {
				t.Errorf("rounded %v must not establish comparison: %+v", pair, got)
			}
		}
	}
}
