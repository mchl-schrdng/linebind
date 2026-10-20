package contract

import (
	"strings"
	"testing"
)

const coChangeContract = `version: 1
name: Migration delivery
rules:
  - id: migration-guide
    kind: co-change
    reason: Ship migration instructions with schema changes.
    scope: ["migrations/**"]
    co-change:
      require: [docs/migrations.md]
    remediation: Update the migration guide.
`

func TestCoChangeValidation(t *testing.T) {
	c := mustLoad(t, coChangeContract)
	if !strings.Contains(c.Rules[0].Specification, "co-change:") {
		t.Fatal("report lost the rule")
	}
	if _, err := Load([]byte("version: 1\nname: Roundtrip\nrules:\n" + indentRule(c.Rules[0].Specification))); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{
		strings.Replace(coChangeContract, "require: [docs/migrations.md]", "require: []", 1),
		strings.Replace(coChangeContract, "require: [docs/migrations.md]", "require: [../secret]", 1),
		strings.Replace(coChangeContract, "    co-change:", "    compare: new\n    co-change:", 1),
		strings.Replace(coChangeContract, "scope: [\"migrations/**\"]", "scope: []", 1),
		coChangeContract + "    files:\n      require: [README.md]\n",
		strings.Replace(coChangeContract, "kind: co-change", "kind: forbid-text", 1),
	} {
		if _, err := Load([]byte(input)); err == nil {
			t.Fatalf("accepted invalid co-change rule:\n%s", input)
		}
	}
	if got := Evaluate(c, map[string][]byte{}); got[0].Status != "unverified" {
		t.Fatal("one snapshot cannot establish co-change")
	}
}

func indentRule(spec string) string {
	return "  - " + strings.ReplaceAll(strings.TrimSuffix(spec, "\n"), "\n", "\n    ") + "\n"
}

func TestCoChangeEndpoints(t *testing.T) {
	rule := mustLoad(t, coChangeContract).Rules[0]
	base := map[string][]byte{"migrations/001.sql": []byte("old"), "docs/migrations.md": []byte("old")}
	for _, tc := range []struct {
		name, status string
		candidate    map[string][]byte
	}{
		{"unchanged", "passed", base},
		{"unrelated", "passed", map[string][]byte{"migrations/001.sql": []byte("old"), "docs/migrations.md": []byte("old"), "README.md": []byte("new")}},
		{"changed without guide", "crossed", map[string][]byte{"migrations/001.sql": []byte("new"), "docs/migrations.md": []byte("old")}},
		{"changed with guide", "passed", map[string][]byte{"migrations/001.sql": []byte("new"), "docs/migrations.md": []byte("new")}},
		{"deleted trigger", "crossed", map[string][]byte{"docs/migrations.md": []byte("old")}},
		{"deleted companion", "crossed", map[string][]byte{"migrations/001.sql": []byte("new")}},
		{"renamed trigger", "crossed", map[string][]byte{"migrations/002.sql": []byte("old"), "docs/migrations.md": []byte("old")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := EvaluateChanges(rule, base, tc.candidate)
			if got.Status != tc.status {
				t.Fatalf("want %s: %+v", tc.status, got)
			}
			if got.Status == "crossed" && (len(got.Findings) != 1 || got.Findings[0].Target != "docs/migrations.md" || got.Findings[0].Remediation == "") {
				t.Fatalf("incomplete evidence: %+v", got)
			}
		})
	}
	for _, tc := range []struct {
		name, status string
		base, after  map[string][]byte
	}{
		{"empty unchanged", "passed", map[string][]byte{}, map[string][]byte{}},
		{"empty first trigger", "crossed", map[string][]byte{}, map[string][]byte{"migrations/001.sql": {}}},
		{"new empty companion counts as addition", "passed", map[string][]byte{}, map[string][]byte{"migrations/001.sql": {}, "docs/migrations.md": {}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := EvaluateChanges(rule, tc.base, tc.after); got.Status != tc.status {
				t.Fatal(got)
			}
		})
	}
}

func TestCoChangeRequiresEveryPatternAndKeepsStableOrder(t *testing.T) {
	rule := mustLoad(t, strings.Replace(coChangeContract, "[docs/migrations.md]", "[docs/migrations.md, \"tests/*.sql\"]", 1)).Rules[0]
	before := map[string][]byte{"migrations/001.sql": []byte("old")}
	after := map[string][]byte{"migrations/001.sql": []byte("new"), "docs/migrations.md": []byte("new")}
	got := EvaluateChanges(rule, before, after)
	if got.Status != "crossed" || len(got.Findings) != 1 || got.Findings[0].Target != "tests/*.sql" {
		t.Fatal(got)
	}
	after["tests/001.sql"] = []byte("new")
	if got := EvaluateChanges(rule, before, after); got.Status != "passed" {
		t.Fatal(got)
	}
	rule.CoChange.Require = []string{"missing/*.md"}
	first := EvaluateChanges(rule, before, after).Findings[0].Fingerprint
	rule.CoChange.Require = []string{"other/*.md"}
	if EvaluateChanges(rule, before, after).Findings[0].Fingerprint == first {
		t.Fatal("predicate change reused identity")
	}
}

func TestCoChangeDoesNotCountCheckoutLineEndings(t *testing.T) {
	rule := mustLoad(t, coChangeContract).Rules[0]
	base := map[string][]byte{"migrations/001.sql": []byte("old\n"), "docs/migrations.md": []byte("old\n")}
	after := map[string][]byte{"migrations/001.sql": []byte("old\r\n"), "docs/migrations.md": []byte("old\n")}
	if got := EvaluateChanges(rule, base, after); got.Status != "passed" {
		t.Fatalf("checkout conversion triggered a change: %+v", got)
	}
	after["migrations/001.sql"] = []byte("new\n")
	after["docs/migrations.md"] = []byte("old\r\n")
	if got := EvaluateChanges(rule, base, after); got.Status != "crossed" {
		t.Fatalf("checkout conversion satisfied companion edit: %+v", got)
	}
	for _, contents := range [][2][]byte{
		{[]byte("old\n"), []byte("old \n")},
		{{0xff, '\n'}, {0xff, '\r', '\n'}},
		{{0, '\n'}, {0, '\r', '\n'}},
	} {
		if SameContent(contents[0], contents[1]) {
			t.Fatal("meaningful or binary content change was normalized away")
		}
	}
}
