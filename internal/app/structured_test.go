package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mchl-schrdng/linebind/internal/contract"
)

// Minimal fields consumed from dependency-cruiser 17.3.8's actual JSON output.
const validCruise = `{"modules":[{"source":"src/a.js","valid":true,"dependencies":[{"resolved":"src/b.js","coreModule":false,"couldNotResolve":false,"valid":false}]},{"source":"src/b.js","valid":true,"dependencies":[]}],"summary":{"violations":[{"type":"dependency","from":"src/a.js","to":"src/b.js","rule":{"name":"no-b","severity":"error"}}],"error":1,"warn":0,"info":0,"totalCruised":2,"optionsUsed":{"outputType":"json"},"ruleSetUsed":{"forbidden":[{"name":"no-b","severity":"error"}]}}}`

func adapterRule() contract.Rule {
	return contract.Rule{ID: "boundary", Kind: "command", Required: true, Reason: "Reviewed dependency policy", Compare: "strict", Scope: []string{"src/*.js"}, Command: &contract.CommandRule{Format: "dependency-cruiser", Argv: []string{"depcruise"}}}
}

func TestDependencyResultsRequireCompleteCoverage(t *testing.T) {
	files := map[string][]byte{"src/a.js": {}, "src/b.js": {}, "other.js": {}}
	selected := []string{"src/a.js", "src/b.js"}
	findings, err := dependencyFindings(adapterRule(), []byte(validCruise), "/snapshot", files, selected)
	if err != nil || len(findings) != 1 || findings[0].Path != "src/a.js" || findings[0].Target != "src/b.js" {
		t.Fatalf("actual schema rejected: %+v %v", findings, err)
	}
	for name, edit := range map[string]func(map[string]any){
		"zero rules":            func(x map[string]any) { x["summary"].(map[string]any)["ruleSetUsed"] = map[string]any{} },
		"missing summary count": func(x map[string]any) { delete(x["summary"].(map[string]any), "error") },
		"inconsistent summary":  func(x map[string]any) { x["summary"].(map[string]any)["error"] = 0 },
		"hidden invalid dependency": func(x map[string]any) {
			x["summary"].(map[string]any)["error"] = 0
			x["summary"].(map[string]any)["violations"] = []any{}
		},
		"truncated modules": func(x map[string]any) {
			x["modules"] = x["modules"].([]any)[:1]
			x["summary"].(map[string]any)["totalCruised"] = 1
		},
		"duplicate modules": func(x map[string]any) {
			modules := x["modules"].([]any)
			x["modules"] = append(modules, modules[0])
			x["summary"].(map[string]any)["totalCruised"] = 3
		},
		"unresolved import": func(x map[string]any) {
			x["modules"].([]any)[0].(map[string]any)["dependencies"].([]any)[0].(map[string]any)["couldNotResolve"] = true
		},
		"missing analysis":  func(x map[string]any) { delete(x["modules"].([]any)[0].(map[string]any), "dependencies") },
		"unresolved module": func(x map[string]any) { x["modules"].([]any)[0].(map[string]any)["couldNotResolve"] = true },
		"unvisited module":  func(x map[string]any) { x["modules"].([]any)[0].(map[string]any)["matchesDoNotFollow"] = true },
		"collapsed module":  func(x map[string]any) { x["modules"].([]any)[0].(map[string]any)["consolidated"] = true },
		"filter": func(x map[string]any) {
			x["summary"].(map[string]any)["optionsUsed"].(map[string]any)["exclude"] = "secret"
		},
		"tool baseline": func(x map[string]any) {
			x["summary"].(map[string]any)["optionsUsed"].(map[string]any)["ignoreKnown"] = true
		},
		"outside snapshot": func(x map[string]any) { x["modules"].([]any)[0].(map[string]any)["source"] = "unknown.js" },
		"absolute escape":  func(x map[string]any) { x["modules"].([]any)[0].(map[string]any)["source"] = "/elsewhere/a.js" },
		"outside scope": func(x map[string]any) {
			x["summary"].(map[string]any)["violations"].([]any)[0].(map[string]any)["from"] = "other.js"
		},
		"bad target": func(x map[string]any) {
			x["summary"].(map[string]any)["violations"].([]any)[0].(map[string]any)["to"] = "../escape.js"
		},
		"unknown severity": func(x map[string]any) {
			x["summary"].(map[string]any)["violations"].([]any)[0].(map[string]any)["rule"].(map[string]any)["severity"] = "whatever"
		},
	} {
		t.Run(name, func(t *testing.T) {
			var output map[string]any
			if err := json.Unmarshal([]byte(validCruise), &output); err != nil {
				t.Fatal(err)
			}
			edit(output)
			data, _ := json.Marshal(output)
			if _, err := dependencyFindings(adapterRule(), data, "/snapshot", files, selected); err == nil {
				t.Fatal("incomplete or inconsistent output accepted")
			}
		})
	}
	for _, data := range []string{strings.Replace(validCruise, `"totalCruised":2`, `"totalCruised":2,"totalCruised":2`, 1), validCruise + "{}", `null`, `{"modules":[],"summary":{}}`} {
		if _, err := dependencyFindings(adapterRule(), []byte(data), "/snapshot", files, selected); err == nil {
			t.Fatal("ambiguous output accepted")
		}
	}
	unknown := strings.Replace(validCruise, `"couldNotResolve":false`, `"couldNotResolve":true`, 1)
	findings, err = dependencyFindings(adapterRule(), []byte(unknown), "/snapshot", files, selected)
	if err == nil || len(findings) != 1 {
		t.Fatal("incomplete graph must retain known violations")
	}
}

func TestConftestResultsRequireEvaluatedPolicies(t *testing.T) {
	rule := adapterRule()
	rule.Command.Format = "conftest"
	for _, data := range []string{
		`[{"filename":"data.yaml","namespace":"main","successes":1}]`,
		`[{"filename":"/snapshot/data.yaml","namespace":"main","successes":0,"failures":[{"msg":"forbidden","metadata":{"query":"data.main.deny"}}]}]`,
		`[{"filename":"./data.yaml","namespace":"main","successes":0,"warnings":[{"msg":"warning"}]}]`,
	} {
		if _, err := conftestFindings(rule, []byte(data), "/snapshot", []string{"data.yaml"}); err != nil {
			t.Fatalf("real result shape rejected: %s: %v", data, err)
		}
	}
	for _, data := range []string{
		`[]`, `null`, `{}`,
		`[{"filename":"data.yaml","namespace":"main","successes":0}]`,
		`[{"filename":"data.yaml","namespace":"main","successes":-1}]`,
		`[{"filename":"data.yaml","namespace":"main","successes":1,"successes":0}]`,
		`[{"filename":"other.yaml","namespace":"main","successes":1}]`,
		`[{"filename":"../data.yaml","namespace":"main","successes":1}]`,
		`[{"filename":"data.yaml","namespace":"main","successes":1,"exceptions":[{"msg":"waived"}]}]`,
		`[{"filename":"data.yaml","namespace":"main","successes":1,"skipped":[{"msg":"unsupported"}]}]`,
		`[{"filename":"data.yaml","namespace":"main","successes":0,"failures":[{"msg":""}]}]`,
		`[{"filename":"data.yaml","namespace":"main","successes":1},{"filename":"data.yaml","namespace":"main","successes":1}]`,
	} {
		if _, err := conftestFindings(rule, []byte(data), "/snapshot", []string{"data.yaml"}); err == nil {
			t.Errorf("incomplete result passed: %s", data)
		}
	}
	data := []byte(`[{"filename":"data.yaml","namespace":"main","successes":1}]`)
	if _, err := conftestFindings(rule, data, "/snapshot", []string{"data.yaml", "other.yaml"}); err == nil {
		t.Fatal("unanalysed file omitted silently")
	}
}
