package contract

import (
	"strings"
	"testing"
)

func TestPositiveDocumentPredicates(t *testing.T) {
	for _, tc := range []struct{ predicate, input, status string }{
		{`allowed-values: [staging, prod, true, 3, null]`, `{"env":"prod"}`, "passed"},
		{`allowed-values: [staging, prod, true, 3, null]`, `{"env":null}`, "passed"},
		{`allowed-values: [staging, prod, true, 3, null]`, `{"env":true}`, "passed"},
		{`allowed-values: [staging, prod, true, 3, null]`, `{"env":"true"}`, "crossed"},
		{`allowed-values: [staging, prod, true, 3, null]`, `{"env":3}`, "passed"},
		{`allowed-values: [staging, prod, true, 3, null]`, `{"env":"3"}`, "crossed"},
		{`allowed-values: [staging, prod, true, 3, null]`, `{}`, "crossed"},
		{`allowed-values: [staging, prod, true, 3, null]`, `{"env":[]}`, "crossed"},
		{`allowed-values: [staging, prod]`, `{"env":"dev"}`, "crossed"},
		{`pattern: '^(staging|prod)$'`, `{"env":"prod"}`, "passed"},
		{`pattern: '^(staging|prod)$'`, `{"env":"production"}`, "crossed"},
		{`pattern: 'prod'`, `{"env":"production"}`, "passed"},
		{`pattern: '.*'`, `{"env":null}`, "crossed"},
		{`pattern: '.*'`, `{"env":true}`, "crossed"},
		{`pattern: '.*'`, `{"env":3}`, "crossed"},
		{`pattern: '.*'`, `{"env":{}}`, "crossed"},
		{`pattern: '.*'`, `{}`, "crossed"},
		{`pattern: '.*'`, `{"env":""}`, "passed"},
	} {
		t.Run(tc.predicate+tc.input, func(t *testing.T) {
			config := strings.Replace(documentContract, "infra/*.yaml", "config.json", 1)
			config = strings.Replace(config, "/spec/containers/*/securityContext/privileged", "/env", 1)
			config = strings.Replace(config, "forbidden-values: [true]", tc.predicate, 1)
			got := oneCheck(t, config, map[string][]byte{"config.json": []byte(tc.input)})
			if got.Status != tc.status {
				t.Fatalf("want %s: %+v", tc.status, got)
			}
		})
	}
	for _, predicate := range []string{`allowed-values: []`, `allowed-values: null`, `allowed-values: [{}]`, `allowed-values: [.inf]`, `pattern: ''`, `pattern: null`, `pattern: true`, `pattern: '(?=x)'`, "pattern: x\n      equals: x", "allowed-values: [x]\n      required: true"} {
		if _, err := Load([]byte(strings.Replace(documentContract, "forbidden-values: [true]", predicate, 1))); err == nil {
			t.Errorf("accepted %s", predicate)
		}
	}
}

func TestDocumentPatternDoesNotCoerceYAMLTimestamp(t *testing.T) {
	config := strings.Replace(documentContract, "/spec/containers/*/securityContext/privileged", "/date", 1)
	config = strings.Replace(config, "forbidden-values: [true]", "pattern: '.*'", 1)
	for _, input := range []string{"date: 2026-10-16", "date: true", "date: 123"} {
		if got := oneCheck(t, config, map[string][]byte{"infra/test.yaml": []byte(input)}); got.Status != "crossed" {
			t.Fatal(got)
		}
	}
}

func TestPositivePredicatesWildcardsAndIdentity(t *testing.T) {
	for _, predicate := range []string{`allowed-values: [safe]`, `pattern: '^safe$'`} {
		config := strings.Replace(documentContract, "/spec/containers/*/securityContext/privileged", "/items/*/env", 1)
		config = strings.Replace(config, "forbidden-values: [true]", predicate, 1)
		first := oneCheck(t, config, map[string][]byte{"infra/test.yaml": []byte("items: [{env: safe}, {env: bad}, {}]\n---\nitems: [{env: safe}]\n")})
		if first.Status != "crossed" || len(first.Findings) != 2 {
			t.Fatal(first)
		}
		moved := oneCheck(t, config, map[string][]byte{"infra/test.yaml": []byte("# comment\nitems: [{env: safe}, {env: bad}, {}]\n---\nitems: [{env: safe}]\n")})
		if moved.Findings[0].Fingerprint != first.Findings[0].Fingerprint {
			t.Fatal("line movement changes identity")
		}
		changed := oneCheck(t, strings.Replace(config, predicate, strings.Replace(predicate, "safe", "good", 1), 1), map[string][]byte{"infra/test.yaml": []byte("items: [{env: good}, {env: bad}, {}]")})
		if changed.Findings[0].Fingerprint == first.Findings[0].Fingerprint {
			t.Fatal("changed predicate reuses identity")
		}
	}
}
