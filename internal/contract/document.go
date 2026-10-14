package contract

import (
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/mchl-schrdng/linebind/internal/model"
	"go.yaml.in/yaml/v3"
)

func selectorParts(selector string) ([]string, error) {
	if selector == "" || !strings.HasPrefix(selector, "/") {
		return nil, fmt.Errorf("document.selector must be a nonempty JSON pointer beginning with /; * selects object or array children")
	}
	parts := strings.Split(selector[1:], "/")
	for i, s := range parts {
		for j := 0; j < len(s); j++ {
			if s[j] == '~' {
				if j+1 == len(s) || (s[j+1] != '0' && s[j+1] != '1') {
					return nil, fmt.Errorf("invalid JSON pointer escape in selector")
				}
				j++
			}
		}
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(s, "~1", "/"), "~0", "~")
	}
	return parts, nil
}

type selectedValue struct {
	node    *yaml.Node
	pointer string
}

func selectValues(n *yaml.Node, parts []string, pointer string) []selectedValue {
	if len(parts) == 0 {
		return []selectedValue{{n, pointer}}
	}
	part := parts[0]
	if n != nil && part == "*" {
		var matches []selectedValue
		switch n.Kind {
		case yaml.MappingNode:
			for i := 0; i < len(n.Content); i += 2 {
				matches = append(matches, selectValues(n.Content[i+1], parts[1:], pointer+"/"+escapePointer(n.Content[i].Value))...)
			}
		case yaml.SequenceNode:
			for i, child := range n.Content {
				matches = append(matches, selectValues(child, parts[1:], pointer+"/"+strconv.Itoa(i))...)
			}
		}
		if len(matches) > 0 {
			return matches
		}
	}
	var child *yaml.Node
	if n != nil {
		switch n.Kind {
		case yaml.MappingNode:
			child = field(n, part)
		case yaml.SequenceNode:
			if index, err := strconv.Atoi(part); err == nil && index >= 0 && index < len(n.Content) && strconv.Itoa(index) == part {
				child = n.Content[index]
			}
		}
	}
	return selectValues(child, parts[1:], pointer+"/"+escapePointer(part))
}

func escapePointer(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}

func evaluateDocuments(r Rule, selected []string, files map[string][]byte, check *model.Check) {
	parts, _ := selectorParts(r.Document.Selector)
	for _, name := range selected {
		data := files[name]
		ext := strings.ToLower(path.Ext(name))
		if ext != ".json" && ext != ".yaml" && ext != ".yml" {
			check.Diagnostics = append(check.Diagnostics, name+": document rules support JSON and YAML files only")
			continue
		}
		if ext == ".json" && !json.Valid(data) {
			check.Diagnostics = append(check.Diagnostics, name+": invalid JSON")
			continue
		}
		docs, err := parseDocuments(data)
		if err != nil {
			check.Diagnostics = append(check.Diagnostics, name+": "+err.Error())
			continue
		}
		if len(docs) == 0 {
			check.Diagnostics = append(check.Diagnostics, name+": no document to verify")
			continue
		}
		for index, doc := range docs {
			if doc.Kind != yaml.MappingNode && doc.Kind != yaml.SequenceNode {
				check.Diagnostics = append(check.Diagnostics, fmt.Sprintf("%s: document %d must be an object or array", name, index+1))
				continue
			}
			for _, value := range selectValues(doc, parts, "") {
				bad, why := documentViolation(r.Document, value.node)
				if !bad {
					continue
				}
				if len(check.Findings) == maxFindings {
					check.Diagnostics = append(check.Diagnostics, "finding limit reached; remaining document values were not reported")
					return
				}
				target := fmt.Sprintf("document[%d]%s", index+1, value.pointer)
				f := finding(r, name, target, why, documentIdentity(value.node))
				if value.node != nil {
					f.Line = value.node.Line
				}
				check.Findings = append(check.Findings, f)
			}
		}
	}
}

// Document identities ignore formatting, comments and line movement, but keep
// the violating value so replacing one forbidden value with another is new.
func documentIdentity(n *yaml.Node) string {
	if n == nil {
		return "missing"
	}
	parts := []string{n.Tag, n.Value}
	if n.Kind == yaml.MappingNode {
		var entries []string
		for i := 0; i < len(n.Content); i += 2 {
			entry, _ := json.Marshal([]string{n.Content[i].Value, documentIdentity(n.Content[i+1])})
			entries = append(entries, string(entry))
		}
		sort.Strings(entries)
		parts = append(parts, entries...)
	} else {
		for _, child := range n.Content {
			parts = append(parts, documentIdentity(child))
		}
	}
	data, _ := json.Marshal(parts)
	return string(data)
}

func documentViolation(rule *DocumentRule, n *yaml.Node) (bool, string) {
	if rule.Required {
		return n == nil || n.Tag == "!!null", "Required document value is missing or null."
	}
	if rule.equalsSet {
		if n == nil {
			return true, "Document value is missing; equality could not be satisfied."
		}
		return !scalarEqual(n, rule.Equals), "Document value differs from the required scalar."
	}
	if n == nil {
		return false, ""
	}
	for _, forbidden := range rule.ForbiddenValues {
		if scalarEqual(n, forbidden) {
			return true, "Document value is forbidden by this rule."
		}
	}
	return false, ""
}

func scalarEqual(n *yaml.Node, expected any) bool {
	if n.Kind != yaml.ScalarNode {
		return false
	}
	var actual any
	if n.Tag == "!!timestamp" {
		actual = n.Value
	} else if err := n.Decode(&actual); err != nil {
		return false
	}
	a, err := json.Marshal(actual)
	if err != nil {
		return false
	}
	b, err := json.Marshal(expected)
	return err == nil && string(a) == string(b)
}
