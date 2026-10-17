package app

import (
	"strings"
	"testing"

	"github.com/mchl-schrdng/linebind/internal/model"
	"github.com/mchl-schrdng/linebind/internal/repository"
)

func TestRuleDefinitionsTrackPolicySourceWithoutChangingGates(t *testing.T) {
	dir, opt := checkRepo(t, testContract, "allowed\n")
	proposed := strings.ReplaceAll(testContract, "pattern: banned", "pattern: permitted")
	put(t, dir, ".linebind.yml", proposed)
	put(t, dir, "src/code.txt", "banned\n")
	for _, tt := range []struct {
		run     func(Options) model.Report
		pattern string
		exit    int
	}{{Check, "pattern: banned", 1}, {Preview, "pattern: permitted", 0}} {
		r := tt.run(opt)
		if r.ExitCode != tt.exit || !strings.Contains(r.Checks[0].Specification, tt.pattern) {
			t.Fatalf("wrong policy/gate: %+v", r)
		}
		if len(r.RuleChanges) != 1 || r.RuleChanges[0].Change != "modified" || !strings.Contains(r.RuleChanges[0].Before, "pattern: banned") || !strings.Contains(r.RuleChanges[0].After, "pattern: permitted") {
			t.Fatalf("missing before/after: %+v", r.RuleChanges)
		}
	}
	put(t, dir, ".linebind.yml", testContract+"\n# reviewed comment\n")
	put(t, dir, "src/code.txt", "allowed\n")
	if r := Preview(opt); r.ExitCode != 0 || len(r.RuleChanges) != 0 || len(r.Changes) != 1 {
		t.Fatalf("comment became a rule modification: %+v", r)
	}
	put(t, dir, ".linebind.yml", "invalid: [")
	if r := Check(opt); r.ExitCode != 1 || r.RuleChangeNote == "" || len(r.RuleChanges) != 0 {
		t.Fatalf("invalid proposal hid uncertainty: %+v", r)
	}
}

func TestRuleReviewIncludesAddedRemovedAndChangedPolicyFiles(t *testing.T) {
	base := &repository.Snapshot{Files: map[string][]byte{".linebind.yml": []byte(testContract)}}
	candidate := &repository.Snapshot{Files: map[string][]byte{".linebind.yml": []byte(strings.ReplaceAll(testContract, "no-banned", "replacement"))}}
	changes, note := describeRuleChanges(base, candidate, ".linebind.yml")
	if note != "" || len(changes) != 2 || changes[0].Change != "removed" || changes[1].Change != "added" {
		t.Fatalf("rename must remain remove/add: %+v, %s", changes, note)
	}
	policy := []byte("version: 1\nname: tool\nrules:\n  - id: tool\n    kind: command\n    reason: Check behavior.\n    scope: [src/**]\n    command: {argv: [node, check.js], policy-files: [check.js]}\n")
	base.Files[".linebind.yml"], candidate.Files[".linebind.yml"] = policy, policy
	base.Files["check.js"], candidate.Files["check.js"] = []byte("assert(false)"), []byte("// removed assertion")
	changes, note = describeRuleChanges(base, candidate, ".linebind.yml")
	if note != "" || len(changes) != 1 || changes[0].Before != changes[0].After || len(changes[0].PolicyFiles) != 1 || changes[0].PolicyFiles[0] != "check.js" {
		t.Fatalf("external policy change disappeared: %+v", changes)
	}
	delete(base.Files, ".linebind.yml")
	changes, note = describeRuleChanges(base, candidate, ".linebind.yml")
	if note != "" || len(changes) != 1 || changes[0].Change != "added" {
		t.Fatalf("first adoption: %+v, %s", changes, note)
	}
}
