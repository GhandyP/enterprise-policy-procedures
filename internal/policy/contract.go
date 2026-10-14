// Package policy holds the deterministic half of the compiler: the versioned
// policy contract, its validation, and the rule facts derived from it. Nothing
// in this package calls a model, so every rule here is exercised by go test
// with no API key.
package policy

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Issue codes this package reports. They are stable identifiers: a caller may
// match on them, and the tests assert them exactly.
const (
	// CodeInvalidInput marks a document that is not the documented shape.
	CodeInvalidInput = "invalid_input"
	// CodeUnknownVersion marks a version the document does not supply.
	CodeUnknownVersion = "unknown_version"
	// CodeDuplicateRuleID marks one version that defines a rule id twice.
	CodeDuplicateRuleID = "duplicate_rule_id"
)

// Corpus is the supplied document: one policy and every version of it that is
// available to the compile. A compile always has more than one version in
// reach, because a citation to a rule that belongs to a sibling version is one
// of the failures the compiler must detect.
type Corpus struct {
	PolicyID      string            `json:"policyID"`
	EffectiveDate string            `json:"effectiveDate"`
	Versions      []VersionedPolicy `json:"versions"`
}

// VersionedPolicy is one version of a policy.
type VersionedPolicy struct {
	Version   string     `json:"version"`
	Sections  []Section  `json:"sections"`
	Approvals []Approval `json:"approvals,omitempty"`
}

// Approval is the human gate a rule that requires approval must have before its
// step can leave the draft. ID, RuleID and Approver are required; Role is
// optional and describes the function the approver holds in the policy.
type Approval struct {
	ID       string `json:"id"`
	RuleID   string `json:"ruleID"`
	Approver string `json:"approver"`
	Role     string `json:"role,omitempty"`
}

// Section groups the rules a policy owner filed together.
type Section struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Rules []Rule `json:"rules"`
}

// Rule is the unit of trace: the thing a procedure step must cite. Only ID,
// Text, Owner and EffectiveFrom are required; the rest describe when a rule
// stops applying, under what condition it applies, which rule it excepts, and
// whether it needs a human approval gate before it can be published.
type Rule struct {
	ID               string `json:"id"`
	Text             string `json:"text"`
	Owner            string `json:"owner"`
	EffectiveFrom    string `json:"effectiveFrom"`
	EffectiveTo      string `json:"effectiveTo,omitempty"`
	Condition        string `json:"condition,omitempty"`
	ExceptionOf      string `json:"exceptionOf,omitempty"`
	RequiresApproval bool   `json:"requiresApproval,omitempty"`
}

// PolicyInput is the contract of S5: the selected view of a corpus, carrying
// the policy id, the single version being compiled and the sections of that
// version alone.
type PolicyInput struct {
	PolicyID string    `json:"policyID"`
	Version  string    `json:"version"`
	Sections []Section `json:"sections"`
}

// Issue is one reason the compile cannot continue. Path locates the problem in
// the supplied document so a policy owner can fix the right line.
type Issue struct {
	Code    string `json:"code"`
	Path    string `json:"path,omitempty"`
	RuleID  string `json:"ruleID,omitempty"`
	Message string `json:"message"`
}

// String renders one issue for a human reader.
func (i Issue) String() string {
	if i.Path == "" {
		return fmt.Sprintf("%s: %s", i.Code, i.Message)
	}
	return fmt.Sprintf("%s: %s: %s", i.Path, i.Code, i.Message)
}

// Parse decodes a supplied policy document and also validates it. The corpus is
// returned even when issues are found, so a caller can report every problem
// from one pass instead of making the policy owner resubmit once per defect.
func Parse(data []byte) (Corpus, []Issue) {
	var corpus Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		return Corpus{}, []Issue{{
			Code:    CodeInvalidInput,
			Message: fmt.Sprintf("the document is not valid policy JSON: %v", err),
		}}
	}
	return corpus, corpus.Validate()
}

// Validate reports every structural problem in the corpus.
func (c Corpus) Validate() []Issue {
	var issues []Issue

	if c.PolicyID == "" {
		issues = append(issues, Issue{
			Code:    CodeInvalidInput,
			Path:    "policyID",
			Message: "the policy must be identified by a policyID",
		})
	}
	if c.EffectiveDate == "" {
		issues = append(issues, Issue{
			Code:    CodeInvalidInput,
			Path:    "effectiveDate",
			Message: "the document must state the effectiveDate the compile is run for",
		})
	}
	if len(c.Versions) == 0 {
		issues = append(issues, Issue{
			Code:    CodeInvalidInput,
			Path:    "versions",
			Message: "the document must supply at least one version",
		})
	}

	for v, version := range c.Versions {
		versionPath := fmt.Sprintf("versions[%d]", v)
		if version.Version == "" {
			issues = append(issues, Issue{
				Code:    CodeInvalidInput,
				Path:    versionPath + ".version",
				Message: "every supplied version must be identified by a version",
			})
		}
		issues = append(issues, version.validate(versionPath)...)
	}

	return issues
}

func (v VersionedPolicy) validate(versionPath string) []Issue {
	var issues []Issue
	seen := map[string]int{}

	for s, section := range v.Sections {
		sectionPath := fmt.Sprintf("%s.sections[%d]", versionPath, s)
		if section.ID == "" {
			issues = append(issues, Issue{
				Code:    CodeInvalidInput,
				Path:    sectionPath + ".id",
				Message: "every section must be identified by an id",
			})
		}
		if section.Title == "" {
			issues = append(issues, Issue{
				Code:    CodeInvalidInput,
				Path:    sectionPath + ".title",
				Message: "every section must carry a title",
			})
		}

		for r, rule := range section.Rules {
			rulePath := fmt.Sprintf("%s.rules[%d]", sectionPath, r)
			issues = append(issues, rule.validate(rulePath)...)

			// An empty id is already reported as a missing field, so it is not
			// counted here: only a real, repeated id is a duplicate.
			if rule.ID == "" {
				continue
			}
			seen[rule.ID]++
			if seen[rule.ID] == 2 {
				issues = append(issues, Issue{
					Code:    CodeDuplicateRuleID,
					Path:    rulePath + ".id",
					RuleID:  rule.ID,
					Message: fmt.Sprintf("rule id %q is defined more than once inside version %q; one version may define a rule id only once", rule.ID, v.Version),
				})
			}
		}
	}

	for a, approval := range v.Approvals {
		issues = append(issues, approval.validate(fmt.Sprintf("%s.approvals[%d]", versionPath, a))...)
	}

	return issues
}

func (a Approval) validate(approvalPath string) []Issue {
	var issues []Issue

	required := []struct {
		field string
		value string
		why   string
	}{
		{"id", a.ID, "every approval gate must be identified by an id"},
		{"ruleID", a.RuleID, "an approval gate must name the rule it gates"},
		{"approver", a.Approver, "an approval gate with no approver claims an approval no person gave"},
	}

	for _, field := range required {
		if field.value == "" {
			issues = append(issues, Issue{
				Code:    CodeInvalidInput,
				Path:    approvalPath + "." + field.field,
				RuleID:  a.RuleID,
				Message: field.why,
			})
		}
	}

	return issues
}

func (r Rule) validate(rulePath string) []Issue {
	var issues []Issue

	required := []struct {
		field string
		value string
		why   string
	}{
		{"id", r.ID, "a procedure step cannot cite a rule that has no id"},
		{"text", r.Text, "a rule without text cannot be drafted into a procedure step"},
		{"owner", r.Owner, "a rule without an owner has no one accountable for it"},
		{"effectiveFrom", r.EffectiveFrom, "a rule must state when it starts applying"},
	}

	for _, field := range required {
		if field.value == "" {
			issues = append(issues, Issue{
				Code:    CodeInvalidInput,
				Path:    rulePath + "." + field.field,
				RuleID:  r.ID,
				Message: field.why,
			})
		}
	}

	return issues
}

// Select returns the view S5 defines for one supplied version.
func (c Corpus) Select(version string) (PolicyInput, []Issue) {
	for _, candidate := range c.Versions {
		if candidate.Version == version {
			return PolicyInput{
				PolicyID: c.PolicyID,
				Version:  candidate.Version,
				Sections: candidate.Sections,
			}, nil
		}
	}

	return PolicyInput{}, []Issue{{
		Code:    CodeUnknownVersion,
		Path:    "version",
		Message: fmt.Sprintf("version %q is not supplied by this document; supplied versions: %s", version, strings.Join(c.versions(), ", ")),
	}}
}

// versions lists the version identifiers the corpus supplies, in the order the
// document declared them.
func (c Corpus) versions() []string {
	list := make([]string, 0, len(c.Versions))
	for _, version := range c.Versions {
		list = append(list, version.Version)
	}
	return list
}
