// Package contract loads declarative rules and evaluates in-memory snapshots.
// It never reads project files or executes project code.
package contract

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"math/big"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

type Contract struct {
	Version int    `yaml:"version" json:"version"`
	Name    string `yaml:"name" json:"name"`
	Rules   []Rule `yaml:"rules" json:"rules"`
}

type Rule struct {
	Specification string            `yaml:"-" json:"-"`
	ID            string            `yaml:"id" json:"id"`
	Kind          string            `yaml:"kind" json:"kind"`
	Reason        string            `yaml:"reason" json:"reason"`
	Scope         []string          `yaml:"scope" json:"scope"`
	Required      bool              `yaml:"required" json:"required"`
	Compare       string            `yaml:"compare" json:"compare"`
	Remediation   string            `yaml:"remediation,omitempty" json:"remediation,omitempty"`
	Text          *TextRule         `yaml:"text,omitempty" json:"text,omitempty"`
	Files         *FilesRule        `yaml:"files,omitempty" json:"files,omitempty"`
	Dependencies  *DependenciesRule `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
	Document      *DocumentRule     `yaml:"document,omitempty" json:"document,omitempty"`
	Command       *CommandRule      `yaml:"command,omitempty" json:"command,omitempty"`
	CoChange      *CoChangeRule     `yaml:"co-change,omitempty" json:"co-change,omitempty"`
}

type TextRule struct {
	Pattern string `yaml:"pattern" json:"pattern"`
}

type FilesRule struct {
	Require []string `yaml:"require" json:"require"`
}

type CoChangeRule struct {
	Require []string `yaml:"require" json:"require"`
}

type DependenciesRule struct {
	Format   string   `yaml:"format" json:"format"`
	Allow    []string `yaml:"allow,omitempty" json:"allow,omitempty"`
	Deny     []string `yaml:"deny,omitempty" json:"deny,omitempty"`
	Sections []string `yaml:"sections,omitempty" json:"sections,omitempty"`
}

type DocumentRule struct {
	Selector        string `yaml:"selector" json:"selector"`
	Equals          any    `yaml:"equals,omitempty" json:"equals,omitempty"`
	ForbiddenValues []any  `yaml:"forbidden-values,omitempty" json:"forbidden-values,omitempty"`
	AllowedValues   []any  `yaml:"allowed-values,omitempty" json:"allowed-values,omitempty"`
	Pattern         string `yaml:"pattern,omitempty" json:"pattern,omitempty"`
	Required        bool   `yaml:"required,omitempty" json:"required,omitempty"`
	equalsSet       bool
	pattern         *regexp.Regexp
}

type CommandRule struct {
	Argv         []string `yaml:"argv" json:"argv"`
	Timeout      int      `yaml:"timeout,omitempty" json:"timeout"`
	SuccessCodes []int    `yaml:"success-codes,omitempty" json:"success_codes"`
	FailureCodes []int    `yaml:"failure-codes,omitempty" json:"failure_codes"`
	Format       string   `yaml:"format,omitempty" json:"format"`
	PolicyFiles  []string `yaml:"policy-files,omitempty" json:"policy_files,omitempty"`
}

// Load accepts exactly one YAML document with no aliases, custom tags, duplicate
// keys, unknown fields, or unused kind-specific options.
func Load(data []byte) (*Contract, error) {
	if len(data) > 1<<20 {
		return nil, fmt.Errorf("contract exceeds 1 MiB")
	}
	docs, err := parseDocuments(data)
	if err != nil {
		return nil, fmt.Errorf("contract: %w", err)
	}
	if len(docs) != 1 || docs[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("contract must contain exactly one mapping document")
	}
	var c Contract
	d := yaml.NewDecoder(bytes.NewReader(data))
	d.KnownFields(true)
	if err := d.Decode(&c); err != nil {
		return nil, fmt.Errorf("contract: %w", err)
	}
	if c.Version != 1 {
		return nil, fmt.Errorf("unsupported contract version %d; expected 1", c.Version)
	}
	if strings.TrimSpace(c.Name) == "" || len(c.Name) > 200 {
		return nil, fmt.Errorf("name must contain 1..200 characters")
	}
	if len(c.Rules) == 0 || len(c.Rules) > 256 {
		return nil, fmt.Errorf("rules must contain 1..256 entries")
	}
	rawRules := field(docs[0], "rules")
	seen := map[string]bool{}
	for i := range c.Rules {
		r := &c.Rules[i]
		n := rawRules.Content[i]
		if field(n, "required") == nil {
			r.Required = true
		} else if err := strictBool(field(n, "required")); err != nil {
			return nil, fmt.Errorf("rule %q required: %w", r.ID, err)
		}
		if r.Compare == "" {
			r.Compare = "strict"
		}
		if r.Document != nil {
			docNode := field(n, "document")
			r.Document.equalsSet = field(docNode, "equals") != nil
			if pattern := field(docNode, "pattern"); pattern != nil {
				if pattern.Tag != "!!str" || pattern.Value == "" {
					return nil, fmt.Errorf("rule %q: document.pattern must be a nonempty string", r.ID)
				}
			}
			if allowed := field(docNode, "allowed-values"); allowed != nil && allowed.Kind != yaml.SequenceNode {
				return nil, fmt.Errorf("rule %q: allowed-values must be a nonempty list", r.ID)
			}
			if required := field(docNode, "required"); required != nil {
				if err := strictBool(required); err != nil {
					return nil, fmt.Errorf("rule %q document.required: %w", r.ID, err)
				}
			}
		}
		if r.Command != nil {
			if timeout := field(field(n, "command"), "timeout"); timeout != nil && r.Command.Timeout == 0 {
				return nil, fmt.Errorf("rule %q: explicit command.timeout must be 1..300 seconds", r.ID)
			}
		}
		if seen[r.ID] {
			return nil, fmt.Errorf("duplicate rule id %q", r.ID)
		}
		seen[r.ID] = true
		if err := validateRule(r); err != nil {
			return nil, fmt.Errorf("rule %q: %w", r.ID, err)
		}
		r.Specification, err = specification(*r)
		if err != nil {
			return nil, fmt.Errorf("rule %q specification: %w", r.ID, err)
		}
	}
	return &c, nil
}

func validateRule(r *Rule) error {
	if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`).MatchString(r.ID) {
		return fmt.Errorf("id must be 1..128 letters, digits, dots, underscores or hyphens")
	}
	if strings.TrimSpace(r.Reason) == "" {
		return fmt.Errorf("reason is required")
	}
	if r.Compare != "strict" && r.Compare != "new" {
		return fmt.Errorf("compare must be strict or new")
	}
	blocks := 0
	for _, present := range []bool{r.Text != nil, r.Files != nil, r.Dependencies != nil, r.Document != nil, r.Command != nil, r.CoChange != nil} {
		if present {
			blocks++
		}
	}
	if blocks != 1 {
		return fmt.Errorf("exactly one kind-specific configuration is required")
	}
	if r.Kind != "require-files" && len(r.Scope) == 0 {
		return fmt.Errorf("scope must not be empty")
	}
	for _, p := range r.Scope {
		if err := validatePattern(p); err != nil {
			return err
		}
	}
	switch r.Kind {
	case "co-change":
		if r.CoChange == nil || len(r.CoChange.Require) == 0 {
			return fmt.Errorf("co-change requires co-change.require patterns")
		}
		if r.Compare != "strict" {
			return fmt.Errorf("co-change already compares base and candidate; compare must be strict")
		}
		for _, p := range r.CoChange.Require {
			if err := validatePattern(p); err != nil {
				return err
			}
		}
	case "forbid-text":
		if r.Text == nil {
			return fmt.Errorf("forbid-text requires text")
		}
		if r.Text.Pattern == "" {
			return fmt.Errorf("text.pattern must not be empty")
		}
		if _, err := regexp.Compile(r.Text.Pattern); err != nil {
			return fmt.Errorf("invalid RE2 pattern: %w", err)
		}
	case "require-files":
		if r.Files == nil || len(r.Files.Require) == 0 {
			return fmt.Errorf("require-files requires files.require patterns")
		}
		for _, p := range r.Files.Require {
			if err := validatePattern(p); err != nil {
				return err
			}
		}
	case "dependencies":
		if r.Dependencies == nil {
			return fmt.Errorf("dependencies configuration is required")
		}
		x := r.Dependencies
		if !slices.Contains([]string{"npm", "go", "python", "cargo"}, x.Format) {
			return fmt.Errorf("dependencies.format must be npm, go, python or cargo")
		}
		if (x.Allow == nil) == (x.Deny == nil) {
			return fmt.Errorf("specify exactly one of dependencies.allow or dependencies.deny")
		}
		if x.Deny != nil && len(x.Deny) == 0 {
			return fmt.Errorf("dependencies.deny must not be empty")
		}
		for _, name := range append(slices.Clone(x.Allow), x.Deny...) {
			if strings.TrimSpace(name) == "" || strings.ContainsAny(name, " \t\r\n\x00") {
				return fmt.Errorf("dependency names must be nonempty exact names")
			}
		}
		if x.Format == "go" && len(x.Sections) != 0 {
			return fmt.Errorf("sections are unsupported for go")
		}
		var supported []string
		switch x.Format {
		case "npm":
			supported = []string{"dependencies", "devDependencies", "optionalDependencies", "peerDependencies"}
		case "python":
			supported = []string{"project.dependencies", "project.optional-dependencies", "build-system.requires"}
			for _, names := range [][]string{x.Allow, x.Deny} {
				for i, name := range names {
					if !pythonPackageName.MatchString(name) {
						return fmt.Errorf("invalid Python package name %q", name)
					}
					names[i] = normalizePythonName(name)
				}
			}
		case "cargo":
			supported = []string{"dependencies", "dev-dependencies", "build-dependencies"}
		}
		if supported != nil {
			if len(x.Sections) == 0 {
				x.Sections = supported
			}
			for _, section := range x.Sections {
				if !slices.Contains(supported, section) {
					return fmt.Errorf("unsupported %s section %q", x.Format, section)
				}
			}
		}
	case "document":
		if r.Document == nil {
			return fmt.Errorf("document configuration is required")
		}
		x := r.Document
		if _, err := selectorParts(x.Selector); err != nil {
			return err
		}
		ops := 0
		if x.equalsSet {
			ops++
			if !isScalar(x.Equals) {
				return fmt.Errorf("document.equals must be a scalar or null")
			}
		}
		if x.ForbiddenValues != nil {
			ops++
			if len(x.ForbiddenValues) == 0 {
				return fmt.Errorf("forbidden-values must not be empty")
			}
			for _, v := range x.ForbiddenValues {
				if !isScalar(v) {
					return fmt.Errorf("forbidden-values must contain scalars or null")
				}
			}
		}
		if x.AllowedValues != nil {
			ops++
			if len(x.AllowedValues) == 0 {
				return fmt.Errorf("allowed-values must not be empty")
			}
			for _, v := range x.AllowedValues {
				if !isScalar(v) {
					return fmt.Errorf("allowed-values must contain scalars or null")
				}
			}
		}
		if x.Pattern != "" {
			ops++
			var err error
			x.pattern, err = regexp.Compile(x.Pattern)
			if err != nil {
				return fmt.Errorf("invalid document RE2 pattern: %w", err)
			}
		}
		if x.Required {
			ops++
		}
		if ops != 1 {
			return fmt.Errorf("document requires exactly one of equals, forbidden-values, allowed-values, pattern, or required: true")
		}
	case "command":
		if r.Command == nil {
			return fmt.Errorf("command configuration is required")
		}
		x := r.Command
		if len(x.Argv) == 0 || len(x.Argv) > 128 || strings.TrimSpace(x.Argv[0]) == "" {
			return fmt.Errorf("command.argv must contain an executable and at most 128 arguments")
		}
		for _, a := range x.Argv {
			if strings.ContainsRune(a, 0) || len(a) > 16384 {
				return fmt.Errorf("invalid command argument")
			}
		}
		if x.Timeout == 0 {
			x.Timeout = 30
		}
		if x.Timeout < 1 || x.Timeout > 300 {
			return fmt.Errorf("command.timeout must be 1..300 seconds")
		}
		if x.Format == "" {
			x.Format = "exit"
		}
		if !slices.Contains([]string{"exit", "dependency-cruiser", "conftest"}, x.Format) {
			return fmt.Errorf("unsupported command.format %q", x.Format)
		}
		if r.Compare == "new" && x.Format == "exit" {
			return fmt.Errorf("compare: new requires structured findings; exit-code commands are strict only")
		}
		if x.SuccessCodes == nil {
			x.SuccessCodes = []int{0}
		}
		if x.FailureCodes == nil {
			x.FailureCodes = []int{1}
		}
		if len(x.SuccessCodes) == 0 {
			return fmt.Errorf("success-codes must not be empty")
		}
		codes := map[int]bool{}
		for _, code := range append(slices.Clone(x.SuccessCodes), x.FailureCodes...) {
			if code < 0 || code > 255 || codes[code] {
				return fmt.Errorf("exit codes must be distinct and between 0 and 255")
			}
			codes[code] = true
		}
		for _, p := range x.PolicyFiles {
			if !safePath(p) {
				return fmt.Errorf("policy-files must be repository-relative file paths: %q", p)
			}
			if strings.ContainsAny(p, "*?[]{}") {
				return fmt.Errorf("policy-files do not support globs: %q", p)
			}
		}
	default:
		return fmt.Errorf("unsupported kind %q", r.Kind)
	}
	return nil
}

func strictBool(n *yaml.Node) error {
	if n.Kind != yaml.ScalarNode || n.Tag != "!!bool" || (n.Value != "true" && n.Value != "false") {
		return fmt.Errorf("use true or false")
	}
	return nil
}

func field(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

func parseDocuments(data []byte) ([]*yaml.Node, error) {
	if len(data) > 16<<20 {
		return nil, fmt.Errorf("document exceeds 16 MiB")
	}
	d := yaml.NewDecoder(bytes.NewReader(data))
	var docs []*yaml.Node
	for {
		var n yaml.Node
		if err := d.Decode(&n); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		if len(n.Content) == 0 {
			continue
		}
		if err := validateNode(n.Content[0], 0); err != nil {
			return nil, err
		}
		docs = append(docs, n.Content[0])
		if len(docs) > 1000 {
			return nil, fmt.Errorf("more than 1000 YAML documents")
		}
	}
	return docs, nil
}

func validateNode(n *yaml.Node, depth int) error {
	if depth > 128 {
		return fmt.Errorf("document nesting exceeds 128 levels")
	}
	if n.Kind == yaml.AliasNode || n.Anchor != "" {
		return fmt.Errorf("YAML anchors and aliases are unsupported (line %d)", n.Line)
	}
	if !slices.Contains([]string{"!!map", "!!seq", "!!str", "!!bool", "!!int", "!!float", "!!null", "!!timestamp"}, n.Tag) {
		return fmt.Errorf("unsupported YAML tag %q (line %d)", n.Tag, n.Line)
	}
	if n.Tag == "!!float" {
		if err := validateFloat(n); err != nil {
			return fmt.Errorf("unsupported number at line %d: %w", n.Line, err)
		}
	}
	// The YAML resolver falls back to a string when a plain decimal overflows.
	// Keep JSON numbers and unquoted YAML numbers from silently changing type.
	if n.Tag == "!!str" && n.Style == 0 && decimalLiteral.MatchString(strings.ReplaceAll(n.Value, "_", "")) {
		return fmt.Errorf("numeric literal exceeds the supported range (line %d); quote an intended string", n.Line)
	}
	if n.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i < len(n.Content); i += 2 {
			key := n.Content[i]
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
				return fmt.Errorf("mapping keys must be strings (line %d)", key.Line)
			}
			if seen[key.Value] {
				return fmt.Errorf("duplicate key %q (line %d)", key.Value, key.Line)
			}
			seen[key.Value] = true
		}
	}
	for _, child := range n.Content {
		if err := validateNode(child, depth+1); err != nil {
			return err
		}
	}
	return nil
}

var decimalLiteral = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)

func validateFloat(n *yaml.Node) error {
	if len(n.Value) > 1024 {
		return fmt.Errorf("floating-point literal exceeds 1 KiB")
	}
	literal := strings.ReplaceAll(n.Value, "_", "")
	if !decimalLiteral.MatchString(literal) {
		return fmt.Errorf("unsupported floating-point literal")
	}
	if i := strings.LastIndexAny(literal, "eE"); i >= 0 {
		exponent, err := strconv.Atoi(literal[i+1:])
		if err != nil || exponent < -400 || exponent > 400 {
			return fmt.Errorf("decimal exponent must be between -400 and 400")
		}
	}
	var value float64
	if err := n.Decode(&value); err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return fmt.Errorf("number must decode to a finite value")
	}
	// Bound the input before exact rational parsing. Comparing decimal text
	// avoids promising exact binary representation for ordinary values like 0.1.
	original, ok := new(big.Rat).SetString(literal)
	if !ok {
		return fmt.Errorf("unsupported floating-point literal")
	}
	canonical, ok := new(big.Rat).SetString(strconv.FormatFloat(value, 'g', -1, 64))
	if !ok || original.Cmp(canonical) != 0 {
		return fmt.Errorf("number loses precision when decoded")
	}
	return nil
}

func isScalar(v any) bool {
	switch v.(type) {
	case nil, string, bool, int, int64, uint64:
		return true
	case float64:
		return !math.IsInf(v.(float64), 0) && !math.IsNaN(v.(float64))
	default:
		return false
	}
}
