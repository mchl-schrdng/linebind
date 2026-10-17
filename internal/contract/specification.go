package contract

import "go.yaml.in/yaml/v3"

// Preserve meaningful empty/null operands that omitempty would otherwise lose.
// Defaults have already been applied by Load; comments and YAML styling are not policy.
func specification(rule Rule) (string, error) {
	var node yaml.Node
	if err := node.Encode(rule); err != nil {
		return "", err
	}
	if rule.Dependencies != nil && rule.Dependencies.Allow != nil && len(rule.Dependencies.Allow) == 0 {
		deps := field(&node, "dependencies")
		deps.Content = append(deps.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "allow"}, &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"})
	}
	if rule.Document != nil && rule.Document.equalsSet && rule.Document.Equals == nil {
		doc := field(&node, "document")
		doc.Content = append(doc.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "equals"}, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"})
	}
	if rule.Command != nil && rule.Command.FailureCodes != nil && len(rule.Command.FailureCodes) == 0 {
		cmd := field(&node, "command")
		cmd.Content = append(cmd.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "failure-codes"}, &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"})
	}
	data, err := yaml.Marshal(&node)
	return string(data), err
}
