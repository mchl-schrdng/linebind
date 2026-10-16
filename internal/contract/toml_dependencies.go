package contract

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const pythonName = `[A-Za-z0-9](?:[A-Za-z0-9._-]*[A-Za-z0-9])?`

var (
	pythonPackageName    = regexp.MustCompile(`^` + pythonName + `$`)
	pythonNameSeparators = regexp.MustCompile(`[-_.]+`)
	pythonRequirement    = regexp.MustCompile(`^[ \t]*(` + pythonName + `)[ \t]*(?:\[[ \t]*(?:` + pythonName + `(?:[ \t]*,[ \t]*` + pythonName + `)*)?[ \t]*\])?[ \t]*(.*)$`)
	pythonVersion        = regexp.MustCompile(`^[ \t]*(?:===|==|!=|<=|>=|~=|<|>)[ \t]*[A-Za-z0-9.*+!_-]+[ \t]*$`)
	markerOperand        = regexp.MustCompile(`^(?:'[^'\r\n\\]*'|"[^"\r\n\\]*"|python_version\b|python_full_version\b|os_name\b|sys_platform\b|platform_release\b|platform_system\b|platform_version\b|platform_machine\b|platform_python_implementation\b|implementation_name\b|implementation_version\b|extra\b)`)
	markerOperator       = regexp.MustCompile(`^(?:===|==|!=|<=|>=|~=|<|>|not[ \t]+in\b|in\b)`)
	cargoPackageName     = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_-]*$`)
)

func normalizePythonName(name string) string {
	return pythonNameSeparators.ReplaceAllString(strings.ToLower(name), "-")
}

func pythonDependencies(data []byte, sections []string) ([]string, []string, error) {
	var m struct {
		Project *struct {
			Dependencies []string            `toml:"dependencies"`
			Optional     map[string][]string `toml:"optional-dependencies"`
			Dynamic      []string            `toml:"dynamic"`
		} `toml:"project"`
		Build *struct {
			Requires []string `toml:"requires"`
		} `toml:"build-system"`
		Tool struct {
			Poetry map[string]any `toml:"poetry"`
			UV     struct {
				Sources map[string]any `toml:"sources"`
			} `toml:"uv"`
		} `toml:"tool"`
	}
	if err := toml.Unmarshal(data, &m); err != nil {
		return nil, nil, fmt.Errorf("invalid pyproject.toml: %w", err)
	}
	var specs, deps, diagnostics []string
	for _, section := range sections {
		if section == "build-system.requires" {
			if m.Build != nil {
				if m.Build.Requires == nil {
					return nil, nil, fmt.Errorf("build-system.requires must be a list of requirements")
				}
				specs = append(specs, m.Build.Requires...)
			}
			continue
		}
		if m.Project == nil {
			return nil, nil, fmt.Errorf("pyproject.toml has no static PEP 621 project table")
		}
		field := strings.TrimPrefix(section, "project.")
		if slices.Contains(m.Project.Dynamic, field) {
			diagnostics = append(diagnostics, section+" is dynamic; complete dependency names are unverified")
		}
		if field == "dependencies" {
			specs = append(specs, m.Project.Dependencies...)
		} else {
			for _, group := range slices.Sorted(maps.Keys(m.Project.Optional)) {
				specs = append(specs, m.Project.Optional[group]...)
			}
		}
	}
	for _, key := range []string{"dependencies", "dev-dependencies", "group"} {
		if _, ok := m.Tool.Poetry[key]; ok {
			diagnostics = append(diagnostics, "tool.poetry."+key+" is outside static PEP 621 dependency declarations")
		}
	}
	if len(m.Tool.UV.Sources) > 0 {
		diagnostics = append(diagnostics, "tool.uv.sources overrides package sources; package identity is unverified")
	}
	for _, spec := range specs {
		name, direct, err := pythonRequirementName(spec)
		if err != nil {
			return nil, nil, err
		}
		deps = append(deps, name)
		if direct {
			diagnostics = append(diagnostics, "dependency "+name+" uses a direct reference whose package identity is unverified")
		}
	}
	return deps, diagnostics, nil
}

// Parse declaration syntax, never evaluate markers against the host environment.
// Version ranges are not resolved or checked for satisfiability.
func pythonRequirementName(spec string) (string, bool, error) {
	invalid := func() (string, bool, error) {
		return "", false, fmt.Errorf("unsupported or invalid Python requirement %q", spec)
	}
	if strings.ContainsAny(spec, "\r\n\x00") {
		return invalid()
	}
	match := pythonRequirement.FindStringSubmatch(spec)
	if match == nil {
		return invalid()
	}
	name, rest := normalizePythonName(match[1]), strings.TrimSpace(match[2])
	if strings.HasPrefix(rest, "@") {
		// Direct references are always incomplete evidence, regardless of the URL.
		if strings.TrimSpace(rest[1:]) == "" {
			return invalid()
		}
		return name, true, nil
	}
	versions, marker, hasMarker := strings.Cut(rest, ";")
	versions = strings.TrimSpace(versions)
	if strings.HasPrefix(versions, "(") {
		if !strings.HasSuffix(versions, ")") {
			return invalid()
		}
		versions = strings.TrimSpace(versions[1 : len(versions)-1])
		if versions == "" {
			return invalid()
		}
	}
	if versions != "" {
		parts := strings.Split(versions, ",")
		if strings.TrimSpace(parts[len(parts)-1]) == "" {
			parts = parts[:len(parts)-1]
		}
		for _, version := range parts {
			if !pythonVersion.MatchString(version) {
				return invalid()
			}
		}
	}
	if hasMarker {
		tail, ok := pythonMarker(marker, 0)
		if !ok || strings.TrimSpace(tail) != "" {
			return invalid()
		}
	}
	return name, false, nil
}

func pythonMarker(s string, depth int) (string, bool) {
	if depth > 64 {
		return s, false
	}
	for {
		s = strings.TrimSpace(s)
		if strings.HasPrefix(s, "(") {
			tail, ok := pythonMarker(s[1:], depth+1)
			if !ok || !strings.HasPrefix(tail, ")") {
				return s, false
			}
			s = tail[1:]
		} else {
			for _, token := range []*regexp.Regexp{markerOperand, markerOperator, markerOperand} {
				s = strings.TrimSpace(s)
				part := token.FindString(s)
				if part == "" {
					return s, false
				}
				s = s[len(part):]
			}
		}
		s = strings.TrimSpace(s)
		if s == "" || s[0] == ')' {
			return s, true
		}
		if strings.HasPrefix(s, "and") {
			s = s[3:]
		} else if strings.HasPrefix(s, "or") {
			s = s[2:]
		} else {
			return s, false
		}
	}
}

func cargoDependencies(data []byte, sections []string) ([]string, []string, error) {
	var m map[string]any
	if err := toml.Unmarshal(data, &m); err != nil {
		return nil, nil, fmt.Errorf("invalid Cargo.toml: %w", err)
	}
	if _, pkg := m["package"].(map[string]any); !pkg {
		if _, workspace := m["workspace"].(map[string]any); !workspace {
			return nil, nil, fmt.Errorf("Cargo.toml requires a package or workspace table")
		}
	}
	var deps, diagnostics []string
	tables := []map[string]any{m}
	if targets, ok := m["target"]; ok {
		targetMap, ok := targets.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("Cargo target must be a table")
		}
		for _, target := range slices.Sorted(maps.Keys(targetMap)) {
			table, ok := targetMap[target].(map[string]any)
			if !ok {
				return nil, nil, fmt.Errorf("Cargo target %q must be a table", target)
			}
			tables = append(tables, table)
		}
	}
	if workspace, ok := m["workspace"].(map[string]any); ok && slices.Contains(sections, "dependencies") {
		tables = append(tables, workspace)
	}
	// Older Cargo editions accept underscore spellings of these sections.
	cargoSections := slices.Clone(sections)
	for _, section := range sections {
		if strings.Contains(section, "-") {
			cargoSections = append(cargoSections, strings.ReplaceAll(section, "-", "_"))
		}
	}
	for _, table := range tables {
		for _, section := range cargoSections {
			raw, present := table[section]
			if !present {
				continue
			}
			entries, ok := raw.(map[string]any)
			if !ok {
				return nil, nil, fmt.Errorf("Cargo %s must be a dependency table", section)
			}
			for _, name := range slices.Sorted(maps.Keys(entries)) {
				if !cargoPackageName.MatchString(name) {
					return nil, nil, fmt.Errorf("invalid Cargo dependency name %q", name)
				}
				deps = append(deps, name)
				switch spec := entries[name].(type) {
				case string:
					if strings.TrimSpace(spec) == "" {
						return nil, nil, fmt.Errorf("Cargo dependency %s has an empty specification", name)
					}
				case map[string]any:
					if alias, exists := spec["package"]; exists {
						actual, ok := alias.(string)
						if !ok || !cargoPackageName.MatchString(actual) {
							return nil, nil, fmt.Errorf("Cargo dependency %s has an invalid package alias", name)
						}
						deps = append(deps, actual)
					}
					source := false
					for _, key := range []string{"version", "path", "git", "registry", "registry-index"} {
						if value, exists := spec[key]; exists {
							text, ok := value.(string)
							if !ok || strings.TrimSpace(text) == "" {
								return nil, nil, fmt.Errorf("Cargo dependency %s has invalid %s", name, key)
							}
							if key == "version" || key == "path" || key == "git" {
								source = true
							}
							if key != "version" {
								diagnostics = append(diagnostics, "dependency "+name+" uses "+key+"; package identity is unverified")
							}
						}
					}
					if inherited, exists := spec["workspace"]; exists {
						if inherited != true {
							return nil, nil, fmt.Errorf("Cargo dependency %s requires workspace = true", name)
						}
						source = true
						// Manifest-local inspection cannot locate an external workspace root safely.
						diagnostics = append(diagnostics, "dependency "+name+" inherits workspace metadata; package identity is unverified")
					}
					if !source {
						return nil, nil, fmt.Errorf("Cargo dependency %s has no version, source or workspace declaration", name)
					}
				default:
					return nil, nil, fmt.Errorf("Cargo dependency %s must be a string or table", name)
				}
			}
		}
	}
	for _, override := range []string{"patch", "replace"} {
		if _, present := m[override]; present {
			diagnostics = append(diagnostics, "Cargo "+override+" can replace package sources; package identity is unverified")
		}
	}
	return deps, diagnostics, nil
}
