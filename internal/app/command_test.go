package app

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mchl-schrdng/linebind/internal/contract"
	"github.com/mchl-schrdng/linebind/internal/model"
	"github.com/mchl-schrdng/linebind/internal/repository"
)

func commandFixture(t *testing.T, action string) (contract.Rule, *repository.Snapshot) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	rule := contract.Rule{ID: "command", Kind: "command", Reason: "Test runner", Required: true, Compare: "strict", Scope: []string{"src/**"}, Command: &contract.CommandRule{Argv: []string{executable, "-test.run=^TestCommandProcess$", "--", action}, Format: "exit", Timeout: 3, SuccessCodes: []int{0}, FailureCodes: []int{1}}}
	snapshot := &repository.Snapshot{Files: map[string][]byte{"src/input.txt": []byte("original")}, Executable: map[string]bool{}, Omitted: map[string]string{}}
	return rule, snapshot
}

// This subprocess is intentionally outside the normal test control flow.
func TestCommandProcess(t *testing.T) {
	separator := -1
	for i, arg := range os.Args {
		if arg == "--" {
			separator = i
			break
		}
	}
	if separator < 0 || len(os.Args) <= separator+1 {
		return
	}
	args := os.Args[separator+1:]
	switch args[0] {
	case "success":
		time.Sleep(20 * time.Millisecond)
	case "failure":
		fmt.Fprintln(os.Stderr, "declared check failed")
		os.Exit(1)
	case "unexpected":
		os.Exit(7)
	case "modify":
		_ = os.WriteFile("src/input.txt", []byte("changed!"), 0600)
	case "policy":
		data, err := os.ReadFile("policy.txt")
		if err != nil || string(data) != "approved" {
			os.Exit(1)
		}
	case "environment":
		if os.Getenv("LINEBIND_TEST_SECRET") != "" || os.Getenv("HOME") == "" || os.Getenv("PATH") == "" {
			os.Exit(1)
		}
	case "overflow":
		fmt.Fprint(os.Stderr, strings.Repeat("x", 70<<10))
	case "timeout":
		time.Sleep(10 * time.Second)
	case "json":
		data, err := os.ReadFile("result.json")
		if err != nil {
			os.Exit(9)
		}
		fmt.Print(string(data))
		code, _ := strconv.Atoi(args[1])
		os.Exit(code)
	case "child":
		time.Sleep(1600 * time.Millisecond)
		_ = os.WriteFile(args[1], []byte("escaped"), 0600)
	case "spawn":
		executable, _ := os.Executable()
		child := exec.Command(executable, "-test.run=^TestCommandProcess$", "--", "child", args[1])
		if child.Start() != nil {
			os.Exit(9)
		}
		time.Sleep(10 * time.Second)
	default:
		os.Exit(9)
	}
	os.Exit(0)
}

func TestRunCommandOutcomesAndTrust(t *testing.T) {
	for _, tc := range []struct{ action, status, diagnostic string }{
		{"success", "passed", "not a sandbox"}, {"failure", "crossed", "exit code"}, {"unexpected", "unverified", "Unexpected command exit"},
		{"modify", "unverified", "modified a source input"}, {"overflow", "unverified", "output exceeded"}, {"environment", "passed", "not independently verified"},
	} {
		t.Run(tc.action, func(t *testing.T) {
			t.Setenv("LINEBIND_TEST_SECRET", "must-not-inherit")
			rule, snapshot := commandFixture(t, tc.action)
			got := runCommand(rule, snapshot, snapshot, true)
			if got.Status != tc.status || !strings.Contains(strings.Join(got.Diagnostics, "\n"), tc.diagnostic) {
				t.Fatalf("unexpected command result: %+v", got)
			}
			if got.DurationMS <= 0 {
				t.Fatal("execution duration was lost")
			}
			if string(snapshot.Files["src/input.txt"]) != "original" {
				t.Fatal("snapshot mutated")
			}
		})
	}
	rule, snapshot := commandFixture(t, "modify")
	if got := runCommand(rule, snapshot, snapshot, false); got.Status != "unverified" || !strings.Contains(strings.Join(got.Diagnostics, " "), "--allow-exec") {
		t.Fatalf("executed without explicit trust: %+v", got)
	}
	snapshot.Omitted["ignored.dat"] = "oversized"
	if got := runCommand(rule, snapshot, snapshot, true); got.Status != "unverified" {
		t.Fatalf("incomplete snapshot accepted: %+v", got)
	}
}

func TestRunCommandUsesApprovedPolicy(t *testing.T) {
	rule, snapshot := commandFixture(t, "policy")
	rule.Command.PolicyFiles = []string{"policy.txt"}
	snapshot.Files["policy.txt"] = []byte("candidate")
	base := &repository.Snapshot{Files: map[string][]byte{"policy.txt": []byte("approved")}}
	if got := runCommand(rule, snapshot, base, true); got.Status != "passed" {
		t.Fatalf("approved policy was not overlaid: %+v", got)
	}
	delete(base.Files, "policy.txt")
	if got := runCommand(rule, snapshot, base, true); got.Status != "unverified" {
		t.Fatal("missing approved policy accepted")
	}
}

func TestRunCommandTimeoutAndProcessTree(t *testing.T) {
	rule, snapshot := commandFixture(t, "timeout")
	rule.Command.Timeout = 1
	marker := filepath.Join(t.TempDir(), "child-survived")
	if runtime.GOOS != "windows" {
		rule.Command.Argv[len(rule.Command.Argv)-1] = "spawn"
		rule.Command.Argv = append(rule.Command.Argv, marker)
	}
	start := time.Now()
	got := runCommand(rule, snapshot, snapshot, true)
	if got.Status != "unverified" || !strings.Contains(strings.Join(got.Diagnostics, " "), "timed out") || time.Since(start) > 6*time.Second {
		t.Fatalf("timeout failed: %+v", got)
	}
	if runtime.GOOS != "windows" {
		time.Sleep(800 * time.Millisecond)
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatal("descendant survived process-group cancellation")
		}
	}
}

func TestRunCommandStructuredFailureKeepsEvidence(t *testing.T) {
	rule, snapshot := commandFixture(t, "json")
	rule.Command.Argv = append(rule.Command.Argv, "0")
	rule.Command.Format = "conftest"
	snapshot.Files["result.json"] = []byte(`[{"filename":"src/input.txt","namespace":"main","successes":0,"failures":[{"msg":"violation"}],"skipped":[{"msg":"incomplete"}]}]`)
	got := runCommand(rule, snapshot, snapshot, true)
	if got.Status != "unverified" || len(got.Findings) != 1 || got.Evidence != "structured" {
		t.Fatalf("incomplete analysis lost evidence or passed: %+v", got)
	}
	snapshot.Files["result.json"] = []byte(`[{"filename":"src/input.txt","namespace":"main","successes":1}]`)
	rule.Command.Argv[len(rule.Command.Argv)-1] = "1"
	if got := runCommand(rule, snapshot, snapshot, true); got.Status != "unverified" {
		t.Fatal("failure exit without findings passed")
	}
}

func TestRealConftest(t *testing.T) {
	bin := os.Getenv("LINEBIND_CONFTEST")
	if bin == "" {
		t.Skip("set LINEBIND_CONFTEST to pinned Conftest 0.62.0 executable")
	}
	version, err := exec.Command(bin, "--version").Output()
	if err != nil || !strings.Contains(string(version), "Conftest: 0.62.0") {
		t.Fatalf("expected Conftest 0.62.0: %s %v", version, err)
	}
	rule, snapshot := commandFixture(t, "success")
	rule.Scope = []string{"data/*.yaml"}
	rule.Command.Argv = []string{bin, "test", "data/input.yaml", "--policy", "policy", "--output", "json"}
	rule.Command.Format = "conftest"
	rule.Command.PolicyFiles = []string{"policy/main.rego"}
	snapshot.Files = map[string][]byte{"data/input.yaml": []byte("debug: true\n"), "policy/main.rego": []byte("package main\ndeny contains \"debug must be false\" if { input.debug == true }\n")}
	for _, tc := range []struct{ input, status string }{{"debug: true\n", "crossed"}, {"debug: false\n", "passed"}, {"debug: [\n", "unverified"}} {
		snapshot.Files["data/input.yaml"] = []byte(tc.input)
		if got := runCommand(rule, snapshot, snapshot, true); got.Status != tc.status {
			t.Fatalf("real conftest %q: %+v", tc.input, got)
		}
	}
	snapshot.Files["data/input.yaml"] = []byte("debug: false\n")
	snapshot.Files["policy/main.rego"] = []byte("package main\nunused := true\n")
	if got := runCommand(rule, snapshot, snapshot, true); got.Status != "unverified" {
		t.Fatalf("real zero-rule Conftest passed: %+v", got)
	}
}

func TestRealDependencyCruiser(t *testing.T) {
	bin := os.Getenv("LINEBIND_DEPCRUISE")
	if bin == "" {
		t.Skip("set LINEBIND_DEPCRUISE to pinned dependency-cruiser 17.3.8 executable")
	}
	version, err := exec.Command(bin, "--version").Output()
	if err != nil || strings.TrimSpace(string(version)) != "17.3.8" {
		t.Fatalf("expected dependency-cruiser 17.3.8: %s %v", version, err)
	}
	rule, snapshot := commandFixture(t, "success")
	rule.Scope = []string{"src/*.js"}
	rule.Command.Argv = []string{bin, "--config", "rules.cjs", "--output-type", "json", "src"}
	rule.Command.Format = "dependency-cruiser"
	rule.Command.PolicyFiles = []string{"rules.cjs"}
	snapshot.Files = map[string][]byte{"src/a.js": []byte("import {b} from './b.js'; import fs from 'node:fs'; export const a=b;\n"), "src/b.js": []byte("export const b=1;\n"), "rules.cjs": []byte(`module.exports={forbidden:[{name:"no-b",severity:"error",from:{path:"^src/a.js$"},to:{path:"^src/b.js$"}}]};`)}
	for _, tc := range []struct{ input, status string }{{"import {b} from './b.js'; import fs from 'node:fs'; export const a=b;", "crossed"}, {"import fs from 'node:fs'; export const a=1;", "passed"}, {"import {b} from './missing.js'; export const a=b;", "unverified"}, {"export const = ;", "passed"}} {
		snapshot.Files["src/a.js"] = []byte(tc.input)
		if got := runCommand(rule, snapshot, snapshot, true); got.Status != tc.status || !strings.Contains(strings.Join(got.Diagnostics, " "), "syntax validity requires") {
			t.Fatalf("real depcruise %q: %+v", tc.input, got)
		}
	}
}

func TestRealToolExamples(t *testing.T) {
	bins := map[string]string{"dependency-cruiser": os.Getenv("LINEBIND_DEPCRUISE"), "conftest": os.Getenv("LINEBIND_CONFTEST")}
	for name, bin := range bins {
		t.Run(name, func(t *testing.T) {
			if bin == "" {
				t.Skip("set the LINEBIND tool path to test committed examples")
			}
			read := func(directory string, snapshot *repository.Snapshot) {
				t.Helper()
				err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, walkErr error) error {
					if walkErr != nil {
						return walkErr
					}
					if entry.IsDir() {
						return nil
					}
					rel, err := filepath.Rel(directory, path)
					if err != nil {
						return err
					}
					data, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					snapshot.Files[filepath.ToSlash(rel)] = data
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			base := &repository.Snapshot{Files: map[string][]byte{}}
			directory := filepath.Join("..", "..", "examples", name)
			read(filepath.Join(directory, "base"), base)
			cases := []struct {
				scenario string
				exit     int
			}{{"base", 0}, {"crossed", 1}, {"unverified", 2}}
			if name == "dependency-cruiser" {
				cases = append(cases, struct {
					scenario string
					exit     int
				}{"invalid-syntax", 1})
			}
			for _, tc := range cases {
				t.Run(tc.scenario, func(t *testing.T) {
					candidate := &repository.Snapshot{Files: map[string][]byte{}}
					for path, data := range base.Files {
						candidate.Files[path] = data
					}
					if tc.scenario != "base" {
						read(filepath.Join(directory, tc.scenario), candidate)
					}
					config, err := contract.Load(base.Files[".linebind.yml"])
					if err != nil {
						t.Fatal(err)
					}
					var report model.Report
					for _, rule := range config.Rules {
						if rule.Command.Format == name {
							rule.Command.Argv[0] = bin
						}
						report.Checks = append(report.Checks, runCommand(rule, candidate, base, true))
					}
					finalize(&report)
					if report.ExitCode != tc.exit {
						t.Fatalf("example %s: %+v", tc.scenario, report)
					}
				})
			}
		})
	}
}
