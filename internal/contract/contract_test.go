package contract

import (
	"strconv"
	"strings"
	"testing"

	"github.com/mchl-schrdng/linebind/internal/model"
	"go.yaml.in/yaml/v3"
)

const textContract = `version: 1
name: Example
rules:
  - id: no-raw-fetch
    kind: forbid-text
    reason: Use the reviewed HTTP client.
    scope: ["src/**/*.ts"]
    text:
      pattern: '\bfetch\s*\('
`

func mustLoad(t *testing.T, data string) *Contract {
	t.Helper()
	c, err := Load([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func oneCheck(t *testing.T, config string, files map[string][]byte) model.Check {
	t.Helper()
	checks := Evaluate(mustLoad(t, config), files)
	if len(checks) != 1 {
		t.Fatalf("got %d checks", len(checks))
	}
	return checks[0]
}

func TestLoadStrictAndDefaults(t *testing.T) {
	c := mustLoad(t, textContract)
	if !c.Rules[0].Required || c.Rules[0].Compare != "strict" {
		t.Fatalf("missing defaults: %+v", c.Rules[0])
	}
	c = mustLoad(t, strings.Replace(textContract, "    text:", "    required: false\n    text:", 1))
	if c.Rules[0].Required {
		t.Fatal("explicit false was lost")
	}
	cases := map[string]string{
		"unknown root":           textContract + "typo: true\n",
		"unknown nested":         strings.Replace(textContract, "pattern:", "pattren:", 1),
		"duplicate root":         textContract + "name: Other\n",
		"duplicate nested":       textContract + "      pattern: other\n",
		"multiple docs":          textContract + "---\nversion: 1\n",
		"unsupported version":    strings.Replace(textContract, "version: 1", "version: 2", 1),
		"empty rules":            "version: 1\nname: Test\nrules: []\n",
		"empty scope":            strings.Replace(textContract, `scope: ["src/**/*.ts"]`, "scope: []", 1),
		"outside scope":          strings.Replace(textContract, "src/**/*.ts", "../secret/*.ts", 1),
		"invalid glob":           strings.Replace(textContract, "src/**/*.ts", "src/[.ts", 1),
		"bad regex":              strings.Replace(textContract, `'\bfetch\s*\('`, "'(?=no)'", 1),
		"custom tag":             strings.Replace(textContract, "name: Example", "name: !evil Example", 1),
		"anchor":                 strings.Replace(textContract, "name: Example", "name: &name Example", 1),
		"wrong block":            strings.Replace(textContract, "kind: forbid-text", "kind: dependencies", 1),
		"second block":           textContract + "    files:\n      require: [README.md]\n",
		"legacy boolean":         strings.Replace(textContract, "    text:", "    required: on\n    text:", 1),
		"unsupported comparison": strings.Replace(textContract, "    text:", "    compare: anything\n    text:", 1),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load([]byte(input)); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
}

func TestLexicalChecksAndStableIdentity(t *testing.T) {
	files := map[string][]byte{"src/a.ts": []byte("// fetch('/comments-are-text')\nfetch('/x')\n"), "README.md": []byte("fetch('/outside-scope')")}
	before := oneCheck(t, textContract, files)
	if before.Status != "crossed" || before.Files != 1 || len(before.Findings) != 2 {
		t.Fatalf("unexpected check: %+v", before)
	}
	files["src/a.ts"] = []byte("\n// fetch('/comments-are-text')\nfetch('/x')\n")
	after := oneCheck(t, textContract, files)
	for i := range before.Findings {
		if before.Findings[i].Fingerprint != after.Findings[i].Fingerprint || before.Findings[i].Line+1 != after.Findings[i].Line {
			t.Fatal("line movement changed semantic identity")
		}
	}
	files["src/a.ts"] = []byte("fetch('/x')\nfetch('/x')\n")
	duplicates := oneCheck(t, textContract, files)
	if len(duplicates.Findings) != 2 || duplicates.Findings[0].Fingerprint == duplicates.Findings[1].Fingerprint {
		t.Fatal("duplicate matches cannot be distinguished")
	}
	changed := strings.Replace(textContract, `'\bfetch\s*\('`, "'fetch'", 1)
	if oneCheck(t, changed, files).Findings[0].Fingerprint == duplicates.Findings[0].Fingerprint {
		t.Fatal("changed predicate reused fingerprint")
	}
	commentOnly := strings.Replace(textContract, "Use the reviewed HTTP client.", "Updated explanation.", 1)
	if oneCheck(t, commentOnly, files).Findings[0].Fingerprint != duplicates.Findings[0].Fingerprint {
		t.Fatal("explanation changed fingerprint")
	}
	files["src/a.ts"] = []byte("httpClient.get('/x')")
	if got := oneCheck(t, textContract, files); got.Status != "passed" {
		t.Fatalf("fixed source: %+v", got)
	}
}

func TestTextMatchLocationsAcrossLines(t *testing.T) {
	for _, tc := range []struct {
		pattern, text string
		lines         []int
	}{
		{"banned", "banned banned\n\nbanned", []int{1, 1, 3}},
		{"(?s)begin.*?end", "\nbegin\nend begin\nend\nbegin end", []int{2, 3, 5}},
		{"(?m)^", "first\nsecond\n", []int{1, 2, 3}},
		{"banned", "banned\r\nbanned", []int{1, 2}},
	} {
		t.Run(tc.pattern+"/"+tc.text, func(t *testing.T) {
			config := strings.Replace(textContract, `'\bfetch\s*\('`, "'"+tc.pattern+"'", 1)
			got := oneCheck(t, config, map[string][]byte{"src/a.ts": []byte(tc.text)})
			if len(got.Findings) != len(tc.lines) {
				t.Fatal(got)
			}
			for i, line := range tc.lines {
				if got.Findings[i].Line != line {
					t.Fatalf("match %d: got line %d, want %d", i, got.Findings[i].Line, line)
				}
			}
		})
	}
}

func TestMissingScopeAndInvalidInputAreUnverified(t *testing.T) {
	for name, files := range map[string]map[string][]byte{
		"empty":     {},
		"unmatched": {"src/a.py": []byte("pass")},
		"outside":   {"../src/a.ts": []byte("fetch()")},
		"binary":    {"src/a.ts": {0xff, 0xfe}},
	} {
		t.Run(name, func(t *testing.T) {
			got := oneCheck(t, textContract, files)
			if got.Status != "unverified" || len(got.Diagnostics) == 0 {
				t.Fatalf("unexpected check: %+v", got)
			}
		})
	}
}

func TestRequiredFilesMissingIsViolation(t *testing.T) {
	config := `version: 1
name: Documentation
rules:
  - id: records
    kind: require-files
    reason: Keep decisions reviewable.
    files:
      require: [README.md, "docs/adr/*.md"]
`
	check := oneCheck(t, config, map[string][]byte{})
	if check.Status != "crossed" || len(check.Findings) != 2 {
		t.Fatalf("missing files: %+v", check)
	}
	check = oneCheck(t, config, map[string][]byte{"README.md": {}, "docs/adr/001.md": {}})
	if check.Status != "passed" || check.Files != 2 {
		t.Fatalf("present files: %+v", check)
	}
}

const documentContract = `version: 1
name: Kubernetes
rules:
  - id: no-privileged
    kind: document
    reason: Workloads must not run privileged.
    scope: ["infra/*.yaml"]
    document:
      selector: /spec/containers/*/securityContext/privileged
      forbidden-values: [true]
`

func TestDocumentsWildcardsMultipleDocumentsAndTypes(t *testing.T) {
	input := `apiVersion: v1
kind: Pod
spec:
  containers:
    - name: safe
      securityContext: {privileged: false}
    - name: dangerous
      securityContext: {privileged: true}
---
apiVersion: v1
kind: Pod
spec:
  containers:
    - name: also-safe
`
	check := oneCheck(t, documentContract, map[string][]byte{"infra/pods.yaml": []byte(input)})
	if check.Status != "crossed" || len(check.Findings) != 1 || check.Findings[0].Target != "document[1]/spec/containers/1/securityContext/privileged" {
		t.Fatalf("unexpected document result: %+v", check)
	}
	check = oneCheck(t, documentContract, map[string][]byte{"infra/pods.yaml": []byte(strings.ReplaceAll(input, "privileged: true", "privileged: false"))})
	if check.Status != "passed" {
		t.Fatalf("fixed manifest: %+v", check)
	}
	required := strings.Replace(documentContract, "forbidden-values: [true]", "required: true", 1)
	check = oneCheck(t, required, map[string][]byte{"infra/pods.yaml": []byte(input)})
	if len(check.Findings) != 1 || !strings.HasPrefix(check.Findings[0].Target, "document[2]") {
		t.Fatalf("wildcard required didn't catch missing child: %+v", check)
	}
	equals := strings.Replace(documentContract, "forbidden-values: [true]", "equals: false", 1)
	check = oneCheck(t, equals, map[string][]byte{"infra/pods.yaml": []byte(input)})
	if len(check.Findings) != 2 {
		t.Fatalf("equals must reject missing and true: %+v", check)
	}
}

func TestDocumentErrorsNeverPass(t *testing.T) {
	for name, input := range map[string]string{
		"broken":          "spec: [\n",
		"duplicate":       "spec: {}\nspec: {}\n",
		"custom-tag":      "spec: !execute evil\n",
		"recursive-alias": "spec: &x {ref: *x}\n",
		"empty":           "",
		"scalar":          "not-an-object",
	} {
		t.Run(name, func(t *testing.T) {
			got := oneCheck(t, documentContract, map[string][]byte{"infra/pods.yaml": []byte(input)})
			if got.Status != "unverified" {
				t.Fatalf("unexpected success: %+v", got)
			}
		})
	}
	jsonConfig := strings.Replace(documentContract, "infra/*.yaml", "infra/*.json", 1)
	got := oneCheck(t, jsonConfig, map[string][]byte{"infra/pod.json": []byte("spec: {}")})
	if got.Status != "unverified" {
		t.Fatal("YAML accepted in a JSON file")
	}
}

func TestDocumentNullAndPointerEscaping(t *testing.T) {
	config := strings.Replace(documentContract, "selector: /spec/containers/*/securityContext/privileged", "selector: /metadata/labels/example.com~1owner", 1)
	config = strings.Replace(config, "forbidden-values: [true]", "equals: null", 1)
	got := oneCheck(t, config, map[string][]byte{"infra/pod.yaml": []byte("metadata:\n  labels:\n    example.com/owner: null\n")})
	if got.Status != "passed" {
		t.Fatalf("null/pointer mismatch: %+v", got)
	}
	for _, operator := range []string{"equals: .nan", "equals: {}", "forbidden-values: []", "required: false", "equals: false\n      required: true"} {
		bad := strings.Replace(documentContract, "forbidden-values: [true]", operator, 1)
		if _, err := Load([]byte(bad)); err == nil {
			t.Errorf("accepted bad operator %q", operator)
		}
	}
}

const dependenciesContract = `version: 1
name: Packages
rules:
  - id: no-axios
    kind: dependencies
    reason: Use the standard HTTP client.
    scope: [package.json]
    dependencies:
      format: npm
      deny: [axios]
`

func TestNPMDependenciesAndAliases(t *testing.T) {
	for name, input := range map[string]string{
		"direct": `{"dependencies":{"axios":"^1.0.0"}}`,
		"alias":  `{"dependencies":{"http":"npm:axios@^1.0.0"}}`,
		"dev":    `{"devDependencies":{"axios":"latest"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			got := oneCheck(t, dependenciesContract, map[string][]byte{"package.json": []byte(input)})
			if got.Status != "crossed" || len(got.Findings) != 1 || got.Findings[0].Target != "axios" {
				t.Fatalf("unexpected deps: %+v", got)
			}
		})
	}
	for _, input := range []string{`{}`, `{"dependencies":{"zod":"^3"}}`} {
		if got := oneCheck(t, dependenciesContract, map[string][]byte{"package.json": []byte(input)}); got.Status != "passed" {
			t.Fatalf("allowed dependency: %+v", got)
		}
	}
	allowNone := strings.Replace(dependenciesContract, "deny: [axios]", "allow: []", 1)
	if got := oneCheck(t, allowNone, map[string][]byte{"package.json": []byte(`{"dependencies":{"zod":"^3"}}`)}); got.Status != "crossed" {
		t.Fatal("empty allowlist allowed a dependency")
	}
}

func TestInvalidAndNonRegistryDependenciesUnverified(t *testing.T) {
	for _, input := range []string{`{`, `[]`, `null`, `{"dependencies":null}`, `{"dependencies":{"x":false}}`, `{"dependencies":{},"dependencies":{"axios":"1"}}`, `{"dependencies":{"safe":"https://example.com/unverified.tgz"}}`, `{"dependencies":{"safe":"npm:"}}`} {
		got := oneCheck(t, dependenciesContract, map[string][]byte{"package.json": []byte(input)})
		if got.Status != "unverified" {
			t.Fatalf("%s: %+v", input, got)
		}
	}
}

func TestNPMAliasIdentityEndsBeforeVersionRange(t *testing.T) {
	for _, target := range []string{"axios", "@scope/blocked"} {
		config := strings.Replace(dependenciesContract, "deny: [axios]", "deny: ["+strconv.Quote(target)+"]", 1)
		for _, version := range []string{"", "@^1.0.0", "@>=1.0.0||@1.0.0"} {
			input := `{"dependencies":{"safe":` + strconv.Quote("npm:"+target+version) + `}}`
			got := oneCheck(t, config, map[string][]byte{"package.json": []byte(input)})
			if got.Status != "crossed" || len(got.Findings) != 1 || got.Findings[0].Target != target {
				t.Fatalf("alias %s%s hid its target: %+v", target, version, got)
			}
		}
	}
}

func TestLocalNPMSourceSpecificationsCannotPassAsRegistryNames(t *testing.T) {
	config := strings.Replace(dependenciesContract, "deny: [axios]", "allow: [safe]", 1)
	for _, spec := range []string{"blocked.tgz", "blocked.tar.gz", "blocked.tar", "BLOCKED.TGZ", ".local-package", "..", `vendor\package`} {
		input := `{"dependencies":{"safe":` + strconv.Quote(spec) + `}}`
		got := oneCheck(t, config, map[string][]byte{"package.json": []byte(input)})
		if got.Status != "unverified" || len(got.Diagnostics) == 0 {
			t.Fatalf("local source %q passed its declared name: %+v", spec, got)
		}
	}
	for _, spec := range []string{"1.2.3", "^1.2.3", ">=1 <2", "latest", "beta"} {
		input := `{"dependencies":{"safe":` + strconv.Quote(spec) + `}}`
		if got := oneCheck(t, config, map[string][]byte{"package.json": []byte(input)}); got.Status != "passed" {
			t.Fatalf("registry specification %q rejected: %+v", spec, got)
		}
	}
}

func TestGoModulesDirectAndReplacement(t *testing.T) {
	config := strings.ReplaceAll(dependenciesContract, "package.json", "go.mod")
	config = strings.Replace(config, "format: npm", "format: go", 1)
	config = strings.Replace(config, "deny: [axios]", "deny: [example.com/blocked]", 1)
	input := "module example.com/app\n\ngo 1.26.0\n\nrequire (\n example.com/allowed v1.0.0\n example.com/blocked v1.0.0 // indirect\n)\n"
	if got := oneCheck(t, config, map[string][]byte{"go.mod": []byte(input)}); got.Status != "passed" {
		t.Fatalf("indirect is out of scope: %+v", got)
	}
	input += "replace example.com/allowed => example.com/blocked v1.1.0\n"
	if got := oneCheck(t, config, map[string][]byte{"go.mod": []byte(input)}); got.Status != "crossed" {
		t.Fatalf("replacement bypass: %+v", got)
	}
	input = strings.Replace(input, "example.com/blocked v1.1.0", "../local", 1)
	if got := oneCheck(t, config, map[string][]byte{"go.mod": []byte(input)}); got.Status != "unverified" {
		t.Fatalf("local replacement unverifiable: %+v", got)
	}
}

const commandContract = `version: 1
name: Commands
rules:
  - id: existing-check
    kind: command
    reason: Run the project contract test.
    scope: ["src/**"]
    command:
      argv: [python, -m, unittest]
      policy-files: ["tests/test_contract.py"]
`

func TestCommandSchemaDefaultsAndNoExecution(t *testing.T) {
	c := mustLoad(t, commandContract)
	x := c.Rules[0].Command
	if x.Timeout != 30 || x.Format != "exit" || len(x.SuccessCodes) != 1 || x.SuccessCodes[0] != 0 || x.FailureCodes[0] != 1 {
		t.Fatalf("unexpected defaults: %+v", x)
	}
	check := Evaluate(c, map[string][]byte{"src/a.py": []byte("pass")})[0]
	if check.Status != "unverified" || check.Evidence != "command" {
		t.Fatalf("command executed or trusted implicitly: %+v", check)
	}
	for name, suffix := range map[string]string{
		"timeout":        "      timeout: 301\n",
		"zero timeout":   "      timeout: 0\n",
		"overlap":        "      failure-codes: [0]\n",
		"invalid code":   "      success-codes: [256]\n",
		"unknown format": "      format: magical\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Load([]byte(commandContract + suffix)); err == nil {
				t.Fatal("invalid command accepted")
			}
		})
	}
	for _, path := range []string{"../secret.py", "/secret.py", "C:/secret.py", "C:secret.py", "tests/*.py"} {
		if _, err := Load([]byte(strings.Replace(commandContract, "tests/test_contract.py", path, 1))); err == nil {
			t.Fatalf("unsafe policy path accepted: %s", path)
		}
	}
	newComparison := strings.Replace(commandContract, "    command:", "    compare: new\n    command:", 1)
	if _, err := Load([]byte(newComparison)); err == nil {
		t.Fatal("opaque command accepted a finding baseline")
	}
	if _, err := Load([]byte(newComparison + "      format: conftest\n")); err != nil {
		t.Fatal(err)
	}
}

func TestDocumentFingerprintTracksValueNotFormatting(t *testing.T) {
	config := strings.Replace(documentContract, "/spec/containers/*/securityContext/privileged", "/mode", 1)
	config = strings.Replace(config, "forbidden-values: [true]", "forbidden-values: [unsafe, dangerous]", 1)
	before := oneCheck(t, config, map[string][]byte{"infra/x.yaml": []byte("mode: unsafe\n")})
	moved := oneCheck(t, config, map[string][]byte{"infra/x.yaml": []byte("# comment\n\nmode: unsafe\n")})
	changed := oneCheck(t, config, map[string][]byte{"infra/x.yaml": []byte("mode: dangerous\n")})
	if before.Findings[0].Fingerprint != moved.Findings[0].Fingerprint {
		t.Fatal("document fingerprint changed with formatting")
	}
	if before.Findings[0].Fingerprint == changed.Findings[0].Fingerprint {
		t.Fatal("different violation reused the old fingerprint")
	}
}

func TestDeterministicSelectionAndCombinedFailure(t *testing.T) {
	files := map[string][]byte{"src/z.ts": []byte("fetch()"), "src/a.ts": {0xff}, "src/nested/b.ts": []byte("fetch()")}
	got := oneCheck(t, textContract, files)
	if got.Status != "unverified" || len(got.Diagnostics) != 1 || len(got.Findings) != 2 || got.Findings[0].Path != "src/nested/b.ts" {
		t.Fatalf("lost failure or incomplete evidence: %+v", got)
	}
	selected, err := Select([]string{"src/**/*.ts", "src/**"}, files)
	if err != nil || len(selected) != 3 || selected[0] != "src/a.ts" {
		t.Fatalf("selection not sorted/deduplicated: %v %v", selected, err)
	}
}

func TestDocumentIdentitiesStayBoundedForNestedValues(t *testing.T) {
	for _, depth := range []int{8, 64} {
		input := strings.Repeat(`{"nested":`, depth) + `"invalid"` + strings.Repeat("}", depth)
		docs, err := parseDocuments([]byte(input))
		if err != nil {
			t.Fatal(err)
		}
		if size := len(documentIdentity(docs[0])); size > 1024 {
			t.Fatalf("%d levels expanded %d input bytes into a %d-byte identity", depth, len(input), size)
		}
	}
}

func TestDocumentIdentitiesNormalizeValuesWithoutLosingTypes(t *testing.T) {
	identity := func(input string) string {
		t.Helper()
		docs, err := parseDocuments([]byte(input))
		if err != nil {
			t.Fatal(err)
		}
		return documentIdentity(docs[0])
	}
	if identity("{a: 0x50, b: false}") != identity("{b: false, a: 80}") {
		t.Fatal("map order or integer formatting changed the identity")
	}
	for _, pair := range [][2]string{{"80", "81"}, {"false", `"false"`}, {"80", `"80"`}, {"null", `"null"`}} {
		if identity(pair[0]) == identity(pair[1]) {
			t.Fatalf("different typed values share an identity: %v", pair)
		}
	}
}

func TestDocumentIdentityDoesNotRoundFloatingValues(t *testing.T) {
	for _, pair := range [][2]string{
		{"18446744073709551616", "18446744073709551617"},
		{"0.100000000000000001", "0.100000000000000002"},
	} {
		first := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: pair[0]}
		second := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: pair[1]}
		if documentIdentity(first) == documentIdentity(second) {
			t.Fatalf("distinct numeric literals share an identity: %v", pair)
		}
	}
}

func TestNumericPrecisionLimitsApplyToContractsAndDocuments(t *testing.T) {
	config := strings.Replace(documentContract, "/spec/containers/*/securityContext/privileged", "/mode", 1)
	config = strings.Replace(config, "forbidden-values: [true]", "equals: safe", 1)
	for _, literal := range []string{
		"18446744073709551616", "18446744073709551617",
		"0.100000000000000001", "0.100000000000000002",
		"4e-324", "1e-9999", "1e400", "0e401", ".nan", ".inf",
		"1." + strings.Repeat("0", 1024),
	} {
		got := oneCheck(t, config, map[string][]byte{"infra/value.yaml": []byte("mode: " + literal + "\n")})
		if got.Status != "unverified" || len(got.Diagnostics) == 0 {
			t.Errorf("numeric literal %q must be incomplete, got status=%s diagnostics=%v", literal, got.Status, got.Diagnostics)
		}
		if _, err := Load([]byte(strings.Replace(config, "equals: safe", "equals: "+literal, 1))); err == nil {
			t.Errorf("contract accepted numeric literal %q without an exact comparison", literal)
		}
	}
	for _, literal := range []string{"0.1", "1.0", "1e-6", "0.10", "1_000.0", "18446744073709551615"} {
		if _, err := parseDocuments([]byte("mode: " + literal)); err != nil {
			t.Errorf("representable numeric literal %q rejected: %v", literal, err)
		}
	}
}

func TestOverflowingJSONNumberCannotBecomeAString(t *testing.T) {
	config := strings.Replace(documentContract, "/spec/containers/*/securityContext/privileged", "/mode", 1)
	config = strings.Replace(config, "infra/*.yaml", "infra/*.json", 1)
	config = strings.Replace(config, "forbidden-values: [true]", `equals: "1e400"`, 1)
	for _, tc := range []struct{ input, want string }{
		{`{"mode":1e400}`, "unverified"},
		{`{"mode":"1e400"}`, "passed"},
	} {
		got := oneCheck(t, config, map[string][]byte{"infra/value.json": []byte(tc.input)})
		if got.Status != tc.want {
			t.Errorf("%s: want %s, got %+v", tc.input, tc.want, got)
		}
	}
}

func TestNonDecimalFloatsCannotReachExactParsing(t *testing.T) {
	for _, literal := range []string{"0x1p-999999", "1/2"} {
		if err := validateFloat(&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: literal}); err == nil {
			t.Fatalf("non-decimal literal %q accepted", literal)
		}
	}
}

func TestRequiredFilesStopsAtFindingLimit(t *testing.T) {
	config := "version: 1\nname: Required entries\nrules:\n  - id: missing\n    kind: require-files\n    reason: Keep each entry available.\n    files:\n      require:\n"
	for i := 0; i < maxFindings+1; i++ {
		config += "        - missing-" + strconv.Itoa(i) + ".md\n"
	}
	got := oneCheck(t, config, map[string][]byte{})
	if got.Status != "unverified" || len(got.Findings) != maxFindings || len(got.Diagnostics) == 0 {
		t.Fatalf("required-file findings escaped their bound: status=%s findings=%d diagnostics=%v", got.Status, len(got.Findings), got.Diagnostics)
	}
}
