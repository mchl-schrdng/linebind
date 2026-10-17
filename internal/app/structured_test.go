package app

import (
	"encoding/json"
	"path/filepath"
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
		"zero rules": func(x map[string]any) { x["summary"].(map[string]any)["ruleSetUsed"] = map[string]any{} },
		"null rule": func(x map[string]any) {
			x["summary"].(map[string]any)["ruleSetUsed"] = map[string]any{"forbidden": []any{nil}}
		},
		"empty rule": func(x map[string]any) {
			x["summary"].(map[string]any)["ruleSetUsed"] = map[string]any{"forbidden": []any{map[string]any{}}}
		},
		"unsupported rule severity": func(x map[string]any) {
			x["summary"].(map[string]any)["ruleSetUsed"] = map[string]any{"forbidden": []any{map[string]any{"name": "rule", "severity": "unknown"}}}
		},
		"null allowed rule": func(x map[string]any) {
			x["summary"].(map[string]any)["ruleSetUsed"] = map[string]any{"allowed": []any{nil}, "allowedSeverity": "error"}
		},
		"empty allowed rule": func(x map[string]any) {
			x["summary"].(map[string]any)["ruleSetUsed"] = map[string]any{"allowed": []any{map[string]any{}}, "allowedSeverity": "error"}
		},
		"missing allowed severity": func(x map[string]any) {
			x["summary"].(map[string]any)["ruleSetUsed"] = map[string]any{"allowed": []any{map[string]any{"from": map[string]any{}, "to": map[string]any{}}}}
		},
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

func TestDependencyResultsCannotEraseInvalidityWithCaseAliases(t *testing.T) {
	data := strings.Replace(validCruise, `"valid":false`, `"valid":false,"VALID":true`, 1)
	data = strings.Replace(data, `"violations":[{"type":"dependency","from":"src/a.js","to":"src/b.js","rule":{"name":"no-b","severity":"error"}}]`, `"violations":[]`, 1)
	data = strings.Replace(data, `"error":1`, `"error":0`, 1)
	files := map[string][]byte{"src/a.js": {}, "src/b.js": {}}
	if _, err := dependencyFindings(adapterRule(), []byte(data), "/snapshot", files, []string{"src/a.js", "src/b.js"}); err == nil {
		t.Fatal("ambiguous dependency validity produced a verdict")
	}
}

func TestDependencyCycleEvidence(t *testing.T) {
	files := map[string][]byte{"src/a.js": {}, "src/b.js": {}, "src/c.js": {}}
	selected := []string{"src/a.js", "src/b.js"}
	cycleOutput := func() map[string]any {
		var output map[string]any
		if err := json.Unmarshal([]byte(validCruise), &output); err != nil {
			t.Fatal(err)
		}
		output["modules"].([]any)[1].(map[string]any)["dependencies"] = []any{
			map[string]any{"resolved": "src/a.js", "couldNotResolve": false, "valid": false},
		}
		violation := output["summary"].(map[string]any)["violations"].([]any)[0].(map[string]any)
		violation["type"] = "cycle"
		violation["cycle"] = []any{map[string]any{"name": "src/b.js"}, map[string]any{"name": "src/a.js"}}
		return output
	}
	data, _ := json.Marshal(cycleOutput())
	findings, err := dependencyFindings(adapterRule(), data, "/snapshot", files, selected)
	if err != nil || len(findings) != 1 || !strings.Contains(findings[0].Message, "src/a.js -> src/b.js -> src/a.js") {
		t.Fatalf("a single cycle summary must account for both edges: %+v %v", findings, err)
	}
	for name, edit := range map[string]func(map[string]any, map[string]any){
		"empty cycle": func(_ map[string]any, v map[string]any) { v["cycle"] = []any{} },
		"open cycle": func(_ map[string]any, v map[string]any) {
			v["cycle"] = []any{map[string]any{"name": "src/b.js"}}
		},
		"wrong first edge": func(_ map[string]any, v map[string]any) { v["to"] = "src/a.js" },
		"edge absent from graph": func(x map[string]any, _ map[string]any) {
			x["modules"].([]any)[1].(map[string]any)["dependencies"] = []any{}
		},
		"uncaptured hop": func(_ map[string]any, v map[string]any) {
			v["cycle"] = []any{map[string]any{"name": "src/b.js"}, map[string]any{"name": "src/c.js"}, map[string]any{"name": "src/a.js"}}
		},
		"escaping hop": func(_ map[string]any, v map[string]any) {
			v["cycle"] = []any{map[string]any{"name": "../escape.js"}, map[string]any{"name": "src/a.js"}}
		},
		"unrelated invalid edge": func(x map[string]any, _ map[string]any) {
			b := x["modules"].([]any)[1].(map[string]any)
			b["dependencies"] = append(b["dependencies"].([]any), map[string]any{"resolved": "src/b.js", "couldNotResolve": false, "valid": false})
		},
		"unrelated invalid module": func(x map[string]any, _ map[string]any) {
			x["modules"].([]any)[1].(map[string]any)["valid"] = false
		},
	} {
		t.Run(name, func(t *testing.T) {
			output := cycleOutput()
			v := output["summary"].(map[string]any)["violations"].([]any)[0].(map[string]any)
			edit(output, v)
			data, _ := json.Marshal(output)
			if _, err := dependencyFindings(adapterRule(), data, "/snapshot", files, selected); err == nil {
				t.Fatal("inconsistent cycle evidence accepted")
			}
		})
	}
	if _, err := dependencyFindings(adapterRule(), data, "/snapshot", files, selected[:1]); err == nil {
		t.Fatal("out-of-scope cycle edge accepted")
	}
	output := cycleOutput()
	modules := output["modules"].([]any)
	modules[1].(map[string]any)["dependencies"].([]any)[0].(map[string]any)["resolved"] = "src/c.js"
	output["modules"] = append(modules, map[string]any{"source": "src/c.js", "valid": true, "dependencies": []any{map[string]any{"resolved": "src/a.js", "couldNotResolve": false, "valid": false}}})
	summary := output["summary"].(map[string]any)
	summary["totalCruised"] = 3
	summary["violations"].([]any)[0].(map[string]any)["cycle"] = []any{map[string]any{"name": "src/b.js"}, map[string]any{"name": "src/c.js"}, map[string]any{"name": "src/a.js"}}
	data, _ = json.Marshal(output)
	other, err := dependencyFindings(adapterRule(), data, "/snapshot", files, append(selected, "src/c.js"))
	if err != nil || len(other) != 1 || other[0].Fingerprint == findings[0].Fingerprint {
		t.Fatalf("different cycle with same first edge must have its own identity: %+v %v", other, err)
	}
}

func TestConftestResultsRequireEvaluatedPolicies(t *testing.T) {
	rule := adapterRule()
	rule.Command.Format = "conftest"
	root := t.TempDir()
	absolutePath, err := json.Marshal(filepath.Join(root, "data.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	outsidePath, err := json.Marshal(filepath.Join(t.TempDir(), "data.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range []string{
		`[{"filename":"data.yaml","namespace":"main","successes":1}]`,
		`[{"filename":` + string(absolutePath) + `,"namespace":"main","successes":0,"failures":[{"msg":"forbidden","metadata":{"query":"data.main.deny"}}]}]`,
		`[{"filename":"./data.yaml","namespace":"main","successes":0,"warnings":[{"msg":"warning"}]}]`,
	} {
		if _, err := conftestFindings(rule, []byte(data), root, []string{"data.yaml"}); err != nil {
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
		`[{"filename":` + string(outsidePath) + `,"namespace":"main","successes":1}]`,
		`[{"filename":"data.yaml","namespace":"main","successes":1,"exceptions":[{"msg":"waived"}]}]`,
		`[{"filename":"data.yaml","namespace":"main","successes":1,"skipped":[{"msg":"unsupported"}]}]`,
		`[{"filename":"data.yaml","namespace":"main","successes":0,"failures":[{"msg":""}]}]`,
		`[{"filename":"data.yaml","namespace":"main","successes":1},{"filename":"data.yaml","namespace":"main","successes":1}]`,
	} {
		if _, err := conftestFindings(rule, []byte(data), root, []string{"data.yaml"}); err == nil {
			t.Errorf("incomplete result passed: %s", data)
		}
	}
	data := []byte(`[{"filename":"data.yaml","namespace":"main","successes":1}]`)
	if _, err := conftestFindings(rule, data, root, []string{"data.yaml", "other.yaml"}); err == nil {
		t.Fatal("unanalysed file omitted silently")
	}
}
