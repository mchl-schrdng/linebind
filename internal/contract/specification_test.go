package contract

import (
	"strings"
	"testing"
)

func TestSpecificationPreservesEffectiveOperandsAndDefaults(t *testing.T) {
	for _, tt := range []struct{ kind, block, want string }{
		{"dependencies", "dependencies: {format: npm, allow: []}", "allow: []"},
		{"dependencies", "dependencies: {format: python, allow: []}", "allow: []"},
		{"dependencies", "dependencies: {format: cargo, allow: []}", "allow: []"},
		{"document", "document: {selector: /x, allowed-values: [null, false, 0, '']}", "allowed-values:"},
		{"document", "document: {selector: /x, pattern: '^safe$'}", "pattern: ^safe$"},
		{"document", "document: {selector: /x, required: true}", "required: true"},
		{"document", "document: {selector: /x, equals: null}", "equals: null"},
		{"document", "document: {selector: /x, equals: false}", "equals: false"},
		{"document", "document: {selector: /x, equals: 0}", "equals: 0"},
		{"document", "document: {selector: /x, equals: ''}", "equals: \"\""},
		{"command", "command: {argv: [tool], failure-codes: []}", "failure-codes: []"},
	} {
		t.Run(tt.block, func(t *testing.T) {
			wrap := func(rule string) []byte {
				return []byte("version: 1\nname: test\nrules:\n  - " + strings.ReplaceAll(strings.TrimSpace(rule), "\n", "\n    ") + "\n")
			}
			c, err := Load(wrap("id: x\nkind: " + tt.kind + "\nreason: Keep the decision.\nscope: [src/**]\n" + tt.block))
			if err != nil {
				t.Fatal(err)
			}
			spec := c.Rules[0].Specification
			for _, want := range []string{tt.want, "required: true", "compare: strict"} {
				if !strings.Contains(spec, want) {
					t.Fatalf("lost operand/default %q: %s", want, spec)
				}
			}
			again, err := Load(wrap(spec))
			if err != nil || again.Rules[0].Specification != spec {
				t.Fatalf("specification does not round-trip: %v\n%s", err, spec)
			}
			if tt.kind == "dependencies" && again.Rules[0].Dependencies.Format == "npm" && len(again.Rules[0].Dependencies.Sections) != 4 {
				t.Fatal("lost default npm sections")
			}
			if tt.kind == "command" && again.Rules[0].Command.Timeout != 30 {
				t.Fatal("lost effective timeout")
			}
		})
	}
}
