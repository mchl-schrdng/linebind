package app

import (
	"maps"
	"testing"

	"github.com/mchl-schrdng/linebind/internal/contract"
	"github.com/mchl-schrdng/linebind/internal/repository"
)

func TestOwnRepositoryContract(t *testing.T) {
	repo, err := repository.Open("../..")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := repo.WorkingTree()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Omitted) != 0 {
		t.Fatalf("incomplete source fixture: %v", snapshot.Omitted)
	}
	policy, err := contract.Load(snapshot.Files[".linebind.yml"])
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range contract.Evaluate(policy, snapshot.Files) {
		if check.Status != "passed" {
			t.Fatalf("repository does not satisfy its proposed contract: %+v", check)
		}
	}

	for _, fixture := range []struct {
		name, path, addition, rule, status string
	}{
		{"missing trust document", "docs/trust-model.md", "", "public-entry-points", "crossed"},
		{"unapproved direct module", "go.mod", "\nrequire example.invalid/unapproved v1.0.0\n", "deliberate-direct-dependencies", "crossed"},
		{"native process call", "internal/contract/unsafe.go", "package contract\nimport \"os/exec\"\nfunc unsafe() { exec.Command(\"unexpected\") }\n", "pure-native-contract-package", "crossed"},
		{"missing bundled reference", "internal/agentsetup/skill/references/authoring.md", "", "bundled-agent-guidance", "crossed"},
		{"remote script", "internal/report/report.html", `<script src="https://example.invalid/script.js"></script>`, "report-network-references", "crossed"},
		{"protocol-relative stylesheet", "internal/report/report.html", `<link href='//example.invalid/style.css'>`, "report-network-references", "crossed"},
		{"unquoted image", "internal/report/report.html", `<img src=https://example.invalid/image.png>`, "report-network-references", "crossed"},
		{"css import", "internal/report/report.html", `@import "theme.css";`, "report-network-references", "crossed"},
		{"remote css image", "internal/report/report.html", `background: url("https://example.invalid/image.png")`, "report-network-references", "crossed"},
		{"fetch", "internal/report/report.html", `fetch("/telemetry")`, "report-network-references", "crossed"},
		{"socket", "internal/report/report.html", `new WebSocket("wss://example.invalid")`, "report-network-references", "crossed"},
		{"event stream", "internal/report/report.html", `new EventSource("/events")`, "report-network-references", "crossed"},
		{"xhr", "internal/report/report.html", `new XMLHttpRequest()`, "report-network-references", "crossed"},
		{"local anchor", "internal/report/report.html", `<a href="#report">Report</a>`, "report-network-references", "passed"},
		{"embedded image", "internal/report/report.html", `<img src="data:image/png;base64,AA==">`, "report-network-references", "passed"},
		{"missing report", "internal/report/report.html", "", "report-network-references", "unverified"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			files := maps.Clone(snapshot.Files)
			if fixture.addition == "" {
				delete(files, fixture.path)
			} else {
				files[fixture.path] = []byte(string(files[fixture.path]) + "\n" + fixture.addition)
			}
			found := false
			for _, check := range contract.Evaluate(policy, files) {
				want := "passed"
				if check.ID == fixture.rule {
					found = true
					want = fixture.status
				}
				if check.Status != want {
					t.Fatalf("want %s: %+v", want, check)
				}
			}
			if !found {
				t.Fatalf("contract no longer contains rule %s", fixture.rule)
			}
		})
	}
}
