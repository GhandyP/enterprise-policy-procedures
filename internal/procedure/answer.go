package procedure

import (
	"encoding/json"
	"fmt"

	"github.com/GhandyP/enterprise-policy-procedures/internal/policy"
	"github.com/GhandyP/enterprise-policy-procedures/internal/trace"
)

// Issue codes reported by the answer validator.
const (
	// CodeUnreadableAnswer marks an answer that is not the documented shape.
	CodeUnreadableAnswer = "unreadable_answer"
	// CodeUncitedStep marks a step that cites no rule, which A2 forbids.
	CodeUncitedStep = "uncited_step"
	// CodeUnknownCitation marks a citation outside the deterministic trace,
	// which A3 forbids.
	CodeUnknownCitation = "unknown_citation"
)

// StatusDraft is the only status this program ever produces. A procedure leaves
// the draft state when an identified human signs it off, and this program has no
// way to claim that happened.
const StatusDraft = "draft"

// Citation is the rule a step came from, together with the policy version that
// rule belongs to. It is trace.Citation, so a caller can print one either way.
type Citation = trace.Citation

// Step is one step of the procedure, with the citation S9 requires it to carry.
type Step struct {
	Text      string     `json:"text"`
	RuleIDs   []string   `json:"ruleIDs"`
	Citations []Citation `json:"citations"`
}

// Decision is a condition the policy declares, with every branch it declares for
// that condition. Both come from the trace, never from the model.
type Decision struct {
	Field    string         `json:"field"`
	Branches []trace.Branch `json:"branches"`
}

// Exception is an exception the policy declares, carrying the citation of the
// rule it excepts.
type Exception struct {
	RuleID    string   `json:"ruleID"`
	ExceptOf  string   `json:"exceptOf"`
	Condition string   `json:"condition"`
	Citation  Citation `json:"citation"`
}

// Approval is an approval gate the policy attaches to a rule, carrying the
// citation of the rule it gates.
type Approval struct {
	RuleID   string   `json:"ruleID"`
	Approver string   `json:"approver"`
	Role     string   `json:"role,omitempty"`
	Citation Citation `json:"citation"`
}

// Procedure is the contract of S5. Only Steps come from the model; decisions,
// exceptions, approvals, the citation index, the status and the approver are
// filled by Go from the trace the compiler already validated.
type Procedure struct {
	PolicyID   string      `json:"policyID"`
	Version    string      `json:"version"`
	Status     string      `json:"status"`
	ApprovedBy string      `json:"approvedBy"`
	Steps      []Step      `json:"steps"`
	Decisions  []Decision  `json:"decisions"`
	Exceptions []Exception `json:"exceptions"`
	Approvals  []Approval  `json:"approvals"`
	Citations  []Citation  `json:"citations"`
}

// answer is the only thing the model is asked to return.
type answer struct {
	Steps []struct {
		Text    string   `json:"text"`
		RuleIDs []string `json:"ruleIDs"`
	} `json:"steps"`
}

// ParseAnswer turns a raw model answer into a Procedure, or reports why it
// cannot. It accepts a step only when it carries at least one citation and every
// citation is one the trace produced; a single rejected step rejects the whole
// answer, and the returned procedure then carries the deterministic half with no
// steps at all, so nothing half-validated can be printed as a procedure.
func ParseAnswer(tr trace.Trace, raw []byte) (Procedure, []policy.Issue) {
	deterministic := deterministicHalf(tr)

	var decoded answer
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return deterministic, []policy.Issue{{
			Code:    CodeUnreadableAnswer,
			Message: fmt.Sprintf("the answer is not the documented JSON shape: %v", err),
		}}
	}
	if len(decoded.Steps) == 0 {
		return deterministic, []policy.Issue{{
			Code:    CodeUnreadableAnswer,
			Message: "the answer contains no steps",
		}}
	}

	var issues []policy.Issue
	for i, step := range decoded.Steps {
		position := i + 1

		if step.Text == "" {
			issues = append(issues, policy.Issue{
				Code:    CodeUnreadableAnswer,
				Message: fmt.Sprintf("step %d carries no text", position),
			})
			continue
		}
		if len(step.RuleIDs) == 0 {
			issues = append(issues, policy.Issue{
				Code: CodeUncitedStep,
				Message: fmt.Sprintf("step %d cites no rule, and every step must cite the rule and version it came from",
					position),
			})
			continue
		}
		for _, ruleID := range step.RuleIDs {
			if !tr.Cites(ruleID) {
				issues = append(issues, policy.Issue{
					Code:   CodeUnknownCitation,
					RuleID: ruleID,
					Message: fmt.Sprintf("step %d cites %q, which version %s does not define as a rule in force",
						position, ruleID, tr.Version),
				})
			}
		}
	}

	if len(issues) != 0 {
		return deterministic, issues
	}

	built := deterministic
	for _, step := range decoded.Steps {
		built.Steps = append(built.Steps, Step{
			Text:      step.Text,
			RuleIDs:   step.RuleIDs,
			Citations: citationsFor(tr, step.RuleIDs),
		})
	}
	built.Citations = usedCitations(built.Steps)

	return built, nil
}

// deterministicHalf is everything Go decides before the model is called, so a
// rejected answer still shows the policy owner what the policy says.
func deterministicHalf(tr trace.Trace) Procedure {
	built := Procedure{
		PolicyID:   tr.PolicyID,
		Version:    tr.Version,
		Status:     StatusDraft,
		ApprovedBy: "",
		Citations:  []Citation{},
	}

	for _, decision := range tr.Decisions {
		built.Decisions = append(built.Decisions, Decision{Field: decision.Field, Branches: decision.Branches})
	}
	for _, rule := range tr.Rules {
		if rule.Rule.ExceptionOf != "" {
			built.Exceptions = append(built.Exceptions, Exception{
				RuleID:    rule.Rule.ID,
				ExceptOf:  rule.Rule.ExceptionOf,
				Condition: rule.Rule.Condition,
				Citation:  rule.Citation,
			})
		}
		if rule.Approval != nil {
			built.Approvals = append(built.Approvals, Approval{
				RuleID:   rule.Rule.ID,
				Approver: rule.Approval.Approver,
				Role:     rule.Approval.Role,
				Citation: rule.Citation,
			})
		}
	}

	return built
}

// citationsFor resolves cited rule ids to the citations the trace produced.
func citationsFor(tr trace.Trace, ruleIDs []string) []Citation {
	citations := make([]Citation, 0, len(ruleIDs))
	for _, ruleID := range ruleIDs {
		for _, rule := range tr.Rules {
			if rule.Rule.ID == ruleID {
				citations = append(citations, rule.Citation)
				break
			}
		}
	}
	return citations
}

// usedCitations indexes the distinct citations the steps used, in first-use
// order, which is what a reader checks the procedure against.
func usedCitations(steps []Step) []Citation {
	seen := map[string]bool{}
	citations := []Citation{}

	for _, step := range steps {
		for _, citation := range step.Citations {
			if seen[citation.String()] {
				continue
			}
			seen[citation.String()] = true
			citations = append(citations, citation)
		}
	}

	return citations
}
