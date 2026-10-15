package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreviewBootstrapsWithoutApprovingTheFirstContract(t *testing.T) {
	dir := t.TempDir()
	testGit(t, dir, "init", "-q")
	put(t, dir, "src/code.txt", "allowed\n")
	testGit(t, dir, "add", ".")
	testGit(t, dir, "commit", "-qm", "before the contract")
	opt := Options{Repo: dir, Base: "HEAD", Head: "worktree", Contract: ".linebind.yml", Version: "test"}
	put(t, dir, ".linebind.yml", testContract)
	before := testGit(t, dir, "status", "--porcelain=v1")
	r := Preview(opt)
	if r.Purpose != "preview" || r.ExitCode != 0 || r.Status != "passed" || len(r.Changes) != 1 || r.Changes[0].Change != "added" {
		t.Fatalf("first contract could not be previewed: %+v", r)
	}
	if r.ContractHash != digest([]byte(testContract)) || r.Base == "HEAD" || r.InputDigest == "" {
		t.Fatalf("preview lacks recorded inputs: %+v", r)
	}
	if current := Check(opt); current.ExitCode != 2 || current.Purpose != "" {
		t.Fatalf("preview approved the first contract: %+v", current)
	}
	if testGit(t, dir, "status", "--porcelain=v1") != before {
		t.Fatal("preview modified the checkout")
	}
}

func TestPreviewCannotReplaceTheApprovedGate(t *testing.T) {
	dir, opt := checkRepo(t, testContract, "allowed\n")
	proposed := strings.ReplaceAll(testContract, "pattern: banned", "pattern: permitted")
	put(t, dir, ".linebind.yml", proposed)
	put(t, dir, "src/code.txt", "banned\n")
	current := Check(opt)
	preview := Preview(opt)
	if current.ExitCode != 1 || len(current.Checks[0].Findings) != 1 || preview.ExitCode != 0 || len(preview.Changes) != 1 {
		t.Fatalf("check and preview lost their distinct policy sources: check=%+v preview=%+v", current, preview)
	}
	if preview.Purpose != "preview" || preview.ContractHash != digest([]byte(proposed)) || current.ContractHash == preview.ContractHash {
		t.Fatalf("proposed policy provenance is missing: %+v", preview)
	}
	for _, raw := range []string{"invalid: [", ""} {
		if raw == "" {
			if err := os.Remove(filepath.Join(dir, ".linebind.yml")); err != nil {
				t.Fatal(err)
			}
		} else {
			put(t, dir, ".linebind.yml", raw)
		}
		if r := Preview(opt); r.Purpose != "preview" || r.ExitCode != 2 || r.Status != "unverified" || len(r.Checks) != 0 {
			t.Fatalf("invalid proposed contract produced evidence: %+v", r)
		}
		if r := Check(opt); r.ExitCode != 1 {
			t.Fatalf("invalid candidate policy bypassed check: %+v", r)
		}
	}
	opt.Base = "missing-base"
	if r := Preview(opt); r.Purpose != "preview" || r.ExitCode != 2 {
		t.Fatalf("failed capture lost preview identity: %+v", r)
	}
}

func TestPreviewUsesProposedRuleForBothSidesOfNewComparison(t *testing.T) {
	dir, opt := checkRepo(t, testContract, "new-pattern\n")
	put(t, dir, ".linebind.yml", strings.ReplaceAll(testContract, "pattern: banned", "pattern: new-pattern"))
	r := Preview(opt)
	if r.ExitCode != 0 || len(r.Checks[0].Findings) != 1 || r.Checks[0].Findings[0].Evolution != "existing" {
		t.Fatalf("proposed baseline used the old rule: %+v", r)
	}
	put(t, dir, "src/code.txt", "new-pattern\nnew-pattern\n")
	r = Preview(opt)
	if r.ExitCode != 1 || r.Status != "crossed" || len(r.Checks[0].Findings) != 2 || r.Checks[0].Findings[1].Evolution != "new" {
		t.Fatalf("preview compared the candidate to itself: %+v", r)
	}
	put(t, dir, "src/unreadable.txt", string([]byte{0xff}))
	if r = Preview(opt); r.ExitCode != 2 || r.Status != "unverified" {
		t.Fatalf("incomplete proposed analysis passed: %+v", r)
	}
}

func previewCommandContract(t *testing.T, action, format, compare, policyFile string, extra ...string) string {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	argv := append([]string{executable, "-test.run=^TestCommandProcess$", "--", action}, extra...)
	encoded, err := json.Marshal(argv)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf(`version: 1
name: Proposed command policy
rules:
  - id: command
    kind: command
    reason: Verify the selected policy source.
    scope: [src/**]
    compare: %s
    command:
      argv: %s
      format: %s
      policy-files: [%s]
`, compare, encoded, format, policyFile)
}

func TestPreviewCommandsUseCandidatePolicyOnlyWithExplicitExecution(t *testing.T) {
	policy := previewCommandContract(t, "policy", "exit", "strict", "policy.txt")
	dir, opt := checkRepo(t, policy, "allowed\n")
	put(t, dir, "policy.txt", "old policy")
	testGit(t, dir, "add", ".")
	testGit(t, dir, "commit", "-qm", "old command policy")
	opt.Base = "HEAD"
	put(t, dir, "policy.txt", "approved") // Expected by the test subprocess; still proposed to Linebind.
	if r := Preview(opt); r.ExitCode != 2 || !strings.Contains(strings.Join(r.Checks[0].Diagnostics, " "), "not executed") {
		t.Fatalf("preview executed without opt-in: %+v", r)
	}
	opt.AllowExec = true
	if r := Preview(opt); r.ExitCode != 0 || len(r.Changes) != 1 || r.Changes[0].Path != "policy.txt" {
		t.Fatalf("preview did not use candidate policy: %+v", r)
	}
	if r := Check(opt); r.ExitCode != 1 || r.Checks[0].Status != "crossed" {
		t.Fatalf("check adopted proposed command policy: %+v", r)
	}
	put(t, dir, "policy.txt", "fails the command")
	if r := Preview(opt); r.ExitCode != 1 || r.Checks[0].Status != "crossed" {
		t.Fatalf("preview ignored command failure: %+v", r)
	}
	if err := os.Remove(filepath.Join(dir, "policy.txt")); err != nil {
		t.Fatal(err)
	}
	if r := Preview(opt); r.ExitCode != 2 || !strings.Contains(strings.Join(r.Diagnostics, " "), "proposed policy file") {
		t.Fatalf("missing proposed policy was accepted: %+v", r)
	}
	put(t, dir, ".linebind.yml", testContract)
	r := Preview(opt)
	if r.ExitCode != 0 || len(r.Changes) != 2 || r.Changes[1].Path != "policy.txt" || r.Changes[1].Change != "removed" {
		t.Fatalf("removed old policy disappeared from review: %+v", r)
	}
}

func TestPreviewStructuredComparisonOverlaysProposedPolicyOnBothSnapshots(t *testing.T) {
	policy := previewCommandContract(t, "json", "conftest", "new", "result.json", "0")
	dir, opt := checkRepo(t, policy, "allowed\n")
	put(t, dir, "result.json", `[{"filename":"src/code.txt","namespace":"main","successes":1}]`)
	testGit(t, dir, "add", ".")
	testGit(t, dir, "commit", "-qm", "old structured policy")
	opt.Base, opt.AllowExec = "HEAD", true
	put(t, dir, "result.json", `[{"filename":"src/code.txt","namespace":"main","successes":0,"failures":[{"msg":"Newly declared rule"}]}]`)
	r := Preview(opt)
	if r.ExitCode != 0 || len(r.Checks[0].Findings) != 1 || r.Checks[0].Findings[0].Evolution != "existing" {
		t.Fatalf("proposed command policy was not shared by both sides: %+v", r)
	}
}
