package contract

import (
	"reflect"
	"strings"
	"testing"
)

func dependencyConfig(format, policy string) string {
	return "version: 1\nname: Names\nrules:\n  - id: deps\n    kind: dependencies\n    reason: Keep names explicit.\n    scope: ['*.toml']\n    dependencies:\n      format: " + format + "\n      " + policy + "\n"
}

func TestPythonDependencyNames(t *testing.T) {
	config := dependencyConfig("python", "deny: [unsafe-pkg]")
	for _, tc := range []struct {
		name, input, status string
		findings            int
	}{
		{"empty static project", "[project]\nname='app'", "passed", 0},
		{"normalized", "[project]\ndependencies=['Unsafe_Pkg[extra]>=1; python_version < \"3.9\"']", "crossed", 1},
		{"optional groups", "[project.optional-dependencies]\ntest=['unsafe.pkg']", "crossed", 1},
		{"build dependencies", "[project]\n[build-system]\nrequires=['unsafe-pkg']", "crossed", 1},
		{"dynamic", "[project]\ndynamic=['dependencies']", "unverified", 0},
		{"static plus dynamic", "[project]\ndependencies=['unsafe-pkg']\ndynamic=['dependencies']", "unverified", 1},
		{"direct reference", "[project]\ndependencies=['unsafe-pkg @ https://example.invalid/a.whl']", "unverified", 1},
		{"missing project", "[build-system]\nrequires=['setuptools']", "unverified", 0},
		{"poetry", "[project]\n[tool.poetry.dependencies]\nunsafe='1'", "unverified", 0},
		{"uv override", "[project]\n[tool.uv.sources]\nsafe={path='../unsafe'}", "unverified", 0},
		{"wrong type", "[project]\ndependencies=['safe',42]", "unverified", 0},
		{"wrong optional type", "[project.optional-dependencies]\ntest='safe'", "unverified", 0},
		{"wrong dynamic type", "[project]\ndynamic='dependencies'", "unverified", 0},
		{"duplicate", "[project]\ndependencies=[]\ndependencies=['unsafe-pkg']", "unverified", 0},
		{"duplicate unrelated key", "unused=1\nunused=2\n[project]", "unverified", 0},
		{"duplicate unrelated table", "[project]\n[tool.other]\nx=1\nx=2", "unverified", 0},
		{"invalid TOML", "[project", "unverified", 0},
		{"missing build requirements", "[project]\n[build-system]\nbuild-backend='setuptools.build_meta'", "unverified", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := oneCheck(t, config, map[string][]byte{"pyproject.toml": []byte(tc.input)})
			if got.Status != tc.status || len(got.Findings) != tc.findings {
				t.Fatalf("want %s/%d: %+v", tc.status, tc.findings, got)
			}
		})
	}
	allowed := dependencyConfig("python", "allow: [Safe_Pkg]")
	if got := oneCheck(t, allowed, map[string][]byte{"pyproject.toml": []byte("[project]\ndependencies=['safe.pkg', 'SAFE-PKG']")}); got.Status != "passed" {
		t.Fatal(got)
	}
	selected := config + "      sections: [project.dependencies]\n"
	if got := oneCheck(t, selected, map[string][]byte{"pyproject.toml": []byte("[project]\ndynamic=['optional-dependencies']\n[project.optional-dependencies]\ntest=['unsafe-pkg']")}); got.Status != "passed" {
		t.Fatal(got)
	}
	for _, format := range []string{"python", "cargo"} {
		if _, err := Load([]byte(dependencyConfig(format, "allow: []") + "      sections: [typo]\n")); err == nil {
			t.Fatal("invalid section accepted")
		}
	}
	if _, err := Load([]byte(dependencyConfig("python", "allow: [bad/name]"))); err == nil {
		t.Fatal("invalid Python name accepted")
	}
}

func TestPythonRequirementGrammar(t *testing.T) {
	for _, spec := range []string{"safe", "Safe_Pkg [x,y] >=1.0, <2", "safe (>=1,)", "safe; python_version < '3.8' or (sys_platform == 'win32' and extra in 'test')", "safe; 'win32' not in sys_platform", "safe==1.*"} {
		if name, direct, err := pythonRequirementName(spec); err != nil || name == "" || direct {
			t.Errorf("rejected %q: %s %v %v", spec, name, direct, err)
		}
	}
	for _, spec := range []string{"", "-bad", "safe-", "safe/bad", "safe[]junk", "safe=>1", "safe==", "safe ()", "safe (>=1", "safe;", "safe; arbitrary == 'x'", "safe; python_version <", "safe; (python_version < '3') garbage", "safe; " + strings.Repeat("(", 65) + "python_version < '3'" + strings.Repeat(")", 65)} {
		if _, _, err := pythonRequirementName(spec); err == nil {
			t.Errorf("accepted %q", spec)
		}
	}
}

func TestCargoDependencyNames(t *testing.T) {
	config := dependencyConfig("cargo", "deny: [unsafe]")
	for _, tc := range []struct {
		name, input, status string
		findings            int
	}{
		{"empty package", "[package]\nname='app'", "passed", 0},
		{"direct", "[package]\n[dependencies]\nunsafe='1'", "crossed", 1},
		{"renamed", "[package]\n[dependencies]\nsafe={package='unsafe',version='1'}", "crossed", 1},
		{"build", "[package]\n[build-dependencies]\nunsafe='1'", "crossed", 1},
		{"dev", "[package]\n[dev-dependencies]\nunsafe='1'", "crossed", 1},
		{"legacy dev", "[package]\n[dev_dependencies]\nunsafe='1'", "crossed", 1},
		{"legacy target build", "[package]\n[target.'cfg(windows)'.build_dependencies]\nunsafe='1'", "crossed", 1},
		{"target", "[package]\n[target.'cfg(windows)'.dependencies]\nunsafe='1'", "crossed", 1},
		{"optional", "[package]\n[dependencies]\nunsafe={version='1',optional=true}", "crossed", 1},
		{"workspace root", "[workspace.dependencies]\nsafe={package='unsafe',version='1'}", "crossed", 1},
		{"workspace inheritance", "[package]\n[dependencies]\nunsafe.workspace=true", "unverified", 1},
		{"path", "[package]\n[dependencies]\nunsafe={path='../other'}", "unverified", 1},
		{"git", "[package]\n[dependencies]\nunsafe={git='https://example.invalid/repo'}", "unverified", 1},
		{"registry", "[package]\n[dependencies]\nsafe={version='1',registry='private'}", "unverified", 0},
		{"patch", "[package]\n[patch.crates-io]\nsafe={path='../unsafe'}", "unverified", 0},
		{"replace", "[package]\n[replace]\n'safe:1.0.0'={path='../unsafe'}", "unverified", 0},
		{"no package", "[dependencies]\nunsafe='1'", "unverified", 0},
		{"duplicate", "[package]\n[dependencies]\nunsafe='1'\nunsafe='2'", "unverified", 0},
		{"invalid type", "[package]\n[dependencies]\nsafe=1", "unverified", 0},
		{"invalid section", "[package]\ndependencies=[]", "passed", 0},
		{"invalid top section", "dependencies=[]\n[package]", "unverified", 0},
		{"empty spec", "[package]\n[dependencies]\nsafe=''", "unverified", 0},
		{"empty table", "[package]\n[dependencies]\nsafe={}", "unverified", 0},
		{"bad alias", "[package]\n[dependencies]\nsafe={package=42,version='1'}", "unverified", 0},
		{"bad path", "[package]\n[dependencies]\nsafe={path=42}", "unverified", 0},
		{"bad workspace", "[package]\n[dependencies]\nsafe={workspace={}}", "unverified", 0},
		{"bad target", "target=[]\n[package]", "unverified", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := oneCheck(t, config, map[string][]byte{"Cargo.toml": []byte(tc.input)})
			if got.Status != tc.status || len(got.Findings) != tc.findings {
				t.Fatalf("want %s/%d: %+v", tc.status, tc.findings, got)
			}
		})
	}
	selected := config + "      sections: [build-dependencies]\n"
	if got := oneCheck(t, selected, map[string][]byte{"Cargo.toml": []byte("[package]\n[dev-dependencies]\nunsafe='1'")}); got.Status != "passed" {
		t.Fatal(got)
	}
	input := []byte("[package]\n[dependencies]\nz={path='../z'}\na={git='https://example.invalid/a'}")
	_, first, err := cargoDependencies(input, []string{"dependencies"})
	if err != nil {
		t.Fatal(err)
	}
	for range 20 {
		_, next, err := cargoDependencies(input, []string{"dependencies"})
		if err != nil || !reflect.DeepEqual(first, next) {
			t.Fatal("nondeterministic diagnostics")
		}
	}
}

func TestTOMLDepthBound(t *testing.T) {
	input := "unused=" + strings.Repeat("[", 10001) + "0" + strings.Repeat("]", 10001) + "\n[project]\n[package]\n"
	for _, format := range []string{"python", "cargo"} {
		got := oneCheck(t, dependencyConfig(format, "allow: []"), map[string][]byte{"manifest.toml": []byte(input)})
		if got.Status != "unverified" {
			t.Fatalf("%s deeply nested input passed: %+v", format, got)
		}
	}
}

func FuzzTOMLDependencyManifests(f *testing.F) {
	for _, seed := range []string{"[project]\ndependencies=['safe']", "[package]\n[dependencies]\nsafe={version='1'}", "[project]\ndynamic=['dependencies']", "[package]\n[dependencies]\nsafe={workspace=true}", "unused=1\nunused=2"} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 65536 {
			t.Skip()
		}
		pythonDependencies(data, []string{"project.dependencies", "project.optional-dependencies", "build-system.requires"})
		cargoDependencies(data, []string{"dependencies", "build-dependencies", "dev-dependencies"})
	})
}
