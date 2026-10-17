// Package model defines the versioned result shared by the CLI and local report.
package model

type Report struct {
	SchemaVersion  int              `json:"schema_version"`
	Purpose        string           `json:"purpose,omitempty"` // preview; omitted for approved-base checks
	Version        string           `json:"version"`
	Repository     string           `json:"repository"`
	Base           string           `json:"base"`
	Candidate      string           `json:"candidate"`
	InputDigest    string           `json:"input_digest"`
	ContractPath   string           `json:"contract_path"`
	ContractHash   string           `json:"contract_hash"`
	GeneratedAt    string           `json:"generated_at"`
	Status         string           `json:"status"` // passed, crossed, unverified
	ExitCode       int              `json:"exit_code"`
	Changes        []ContractChange `json:"contract_changes"`
	RuleChanges    []RuleChange     `json:"rule_changes,omitempty"`
	RuleChangeNote string           `json:"rule_change_note,omitempty"`
	Checks         []Check          `json:"checks"`
	Diagnostics    []string         `json:"diagnostics"`
}

// RuleChange records definitions, not inferred strengthening or weakening.
type RuleChange struct {
	ID          string   `json:"id"`
	Change      string   `json:"change"`
	Before      string   `json:"before,omitempty"`
	After       string   `json:"after,omitempty"`
	PolicyFiles []string `json:"policy_files,omitempty"`
}

type ContractChange struct {
	Path   string `json:"path"`
	Change string `json:"change"` // added, modified, removed
	Detail string `json:"detail"`
}

type Check struct {
	ID            string    `json:"id"`
	Kind          string    `json:"kind"`
	Reason        string    `json:"reason"`
	Required      bool      `json:"required"`
	Status        string    `json:"status"`   // passed, crossed, unverified
	Evidence      string    `json:"evidence"` // native, structured, command
	Scope         []string  `json:"scope"`
	Specification string    `json:"specification,omitempty"`
	Files         int       `json:"files"`
	DurationMS    int64     `json:"duration_ms"`
	Findings      []Finding `json:"findings"`
	Diagnostics   []string  `json:"diagnostics"`
}

type Finding struct {
	Fingerprint string `json:"fingerprint"`
	Path        string `json:"path"`
	Line        int    `json:"line,omitempty"`
	Target      string `json:"target,omitempty"`
	Message     string `json:"message"`
	Remediation string `json:"remediation,omitempty"`
	Evolution   string `json:"evolution"` // new, existing, resolved, not_comparable
}
