// Package trace turns the deterministic reading of one policy version into the
// trace a procedure may be built from: the rules that are in force and may be
// cited, the branches their conditions declare, and every reason the compile has
// to stop before a model is ever called.
//
// Like internal/policy, nothing here calls a model.
package trace

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/GhandyP/enterprise-policy-procedures/internal/policy"
)

// Issue codes reported by the trace.
const (
	// CodeMissingApproval marks a rule that requires approval and has no gate.
	CodeMissingApproval = "missing_approval"
	// CodeDanglingException marks an exception to a rule the version does not
	// define in force.
	CodeDanglingException = "dangling_exception"
	// CodeUnrelatedException marks an exception that branches a different
	// condition field than the rule it claims to except.
	CodeUnrelatedException = "unrelated_exception"
	// CodeInvalidCondition marks a condition that is not a predicate this
	// compiler can read.
	CodeInvalidCondition = "invalid_condition"
)

// Operators is the complete set of condition operators a policy document may
// use. Anything else is reported rather than handed to a model as if something
// else had evaluated it.
const (
	OperatorEquals    = "=="
	OperatorNotEquals = "!="
)

// Citation is the pair a procedure step must carry: the rule it came from and
// the policy version that rule belongs to.
type Citation struct {
	RuleID  string
	Version string
}

// String renders a citation the way a reader checks it against the policy.
func (c Citation) String() string {
	return c.RuleID + "@" + c.Version
}

// Branch is the condition under which one rule applies, read out of the
// predicate the policy declares. A rule with no condition has the zero Branch.
type Branch struct {
	RuleID    string
	Field     string
	Operator  string
	Value     string
	Exception bool
	ExceptOf  string
}

// Decision is one condition field together with every branch the version
// declares for it, in document order.
type Decision struct {
	Field    string
	Branches []Branch
}

// Rule is one citable rule: a rule of the selected version that is in force at
// the effective date, with the citation it must be printed with.
type Rule struct {
	policy.Rule
	SectionID string
	Citation  Citation
	Approval  *policy.Approval
	Branch    Branch
}

// Trace is the deterministic rule trace of one version. Nothing in it came from
// a model, so every field can be asserted in a test.
type Trace struct {
	PolicyID      string
	Version       string
	EffectiveDate time.Time
	Rules         []Rule
	Decisions     []Decision
}

// Cites reports whether a rule id is one the trace produced. This is what makes
// a citation acceptable: the model may cite the traced rules and nothing else.
func (t Trace) Cites(ruleID string) bool {
	for _, rule := range t.Rules {
		if rule.Rule.ID == ruleID {
			return true
		}
	}
	return false
}

// Build reads a policy index and returns the trace of its in-force rules,
// together with every blocker that must stop the compile: a rule needing
// approval without a gate, an exception to a rule that is not defined in force,
// an exception that branches something else, and a condition this compiler
// cannot read. Every blocker is reported in one pass.
func Build(index policy.Index) (Trace, []policy.Issue) {
	var issues []policy.Issue

	rules := make([]Rule, 0, len(index.Rules))
	for _, fact := range index.Rules {
		// A rule that is not in force is never traced, so it can never be
		// cited and can never reach the model in the first place.
		if fact.Obsolete {
			continue
		}

		rule := Rule{
			Rule:      fact.Rule,
			SectionID: fact.SectionID,
			Citation:  Citation{RuleID: fact.Rule.ID, Version: index.Version},
			Approval:  fact.Approval,
		}

		if fact.Rule.RequiresApproval && fact.Approval == nil {
			issues = append(issues, policy.Issue{
				Code:   CodeMissingApproval,
				RuleID: fact.Rule.ID,
				Message: fmt.Sprintf("rule %q requires approval and version %q attaches no approval gate to it, so no step may be drafted from it",
					fact.Rule.ID, index.Version),
			})
		}

		if fact.Rule.Condition != "" {
			field, operator, value, ok := ParseCondition(fact.Rule.Condition)
			if !ok {
				issues = append(issues, policy.Issue{
					Code:   CodeInvalidCondition,
					RuleID: fact.Rule.ID,
					Message: fmt.Sprintf("condition %q is not a %s predicate; write \"field %s value\" with a single value and no spaces",
						fact.Rule.Condition, "field == value", OperatorEquals),
				})
			} else {
				rule.Branch = Branch{RuleID: fact.Rule.ID, Field: field, Operator: operator, Value: value}
			}
		}

		if fact.Rule.ExceptionOf != "" {
			rule.Branch.Exception = true
			rule.Branch.ExceptOf = fact.Rule.ExceptionOf
		}

		rules = append(rules, rule)
	}

	issues = append(issues, validateExceptions(rules)...)

	return Trace{
		PolicyID:      index.PolicyID,
		Version:       index.Version,
		EffectiveDate: index.EffectiveDate,
		Rules:         rules,
		Decisions:     decisions(rules),
	}, issues
}

// validateExceptions checks that an exception names a rule the version defines
// in force, and that it branches the same condition field as the rule it
// excepts. An exception to an unconditional rule is allowed: it introduces the
// branch that rule never had.
func validateExceptions(rules []Rule) []policy.Issue {
	byID := map[string]Rule{}
	for _, rule := range rules {
		byID[rule.Rule.ID] = rule
	}

	var issues []policy.Issue
	for _, rule := range rules {
		if rule.Rule.ExceptionOf == "" {
			continue
		}

		excepted, defined := byID[rule.Rule.ExceptionOf]
		if !defined {
			issues = append(issues, policy.Issue{
				Code:   CodeDanglingException,
				RuleID: rule.Rule.ID,
				Message: fmt.Sprintf("rule %q excepts %q, which this version does not define as a rule in force, so it excepts nothing",
					rule.Rule.ID, rule.Rule.ExceptionOf),
			})
			continue
		}

		if excepted.Branch.Field == "" || excepted.Branch.Field == rule.Branch.Field {
			continue
		}

		issues = append(issues, policy.Issue{
			Code:   CodeUnrelatedException,
			RuleID: rule.Rule.ID,
			Message: fmt.Sprintf("rule %q excepts %q but branches field %q while %q branches %q, so it narrows something other than the rule it claims to except",
				rule.Rule.ID, excepted.Rule.ID, rule.Branch.Field, excepted.Rule.ID, excepted.Branch.Field),
		})
	}

	return issues
}

// decisions groups the parsed branches by the field they branch, keeping the
// order the document declared them in.
func decisions(rules []Rule) []Decision {
	order := []string{}
	branchesByField := map[string][]Branch{}

	for _, rule := range rules {
		field := rule.Branch.Field
		if field == "" {
			continue
		}
		if _, known := branchesByField[field]; !known {
			order = append(order, field)
		}
		branchesByField[field] = append(branchesByField[field], rule.Branch)
	}

	decisions := make([]Decision, 0, len(order))
	for _, field := range order {
		decisions = append(decisions, Decision{Field: field, Branches: branchesByField[field]})
	}
	return decisions
}

// ParseCondition reads the documented predicate grammar: an identifier, one of
// the supported operators, and a single value with no whitespace in it. The
// grammar is pinned rather than guessed, so a policy owner who writes prose into
// a condition is told instead of that prose travelling on as a branch.
func ParseCondition(condition string) (field, operator, value string, ok bool) {
	index := -1
	separator := ""

	if i := strings.Index(condition, OperatorEquals); i >= 0 {
		index, separator = i, OperatorEquals
	}
	if i := strings.Index(condition, OperatorNotEquals); i >= 0 && (index < 0 || i < index) {
		index, separator = i, OperatorNotEquals
	}
	if index < 0 {
		return "", "", "", false
	}

	field = strings.TrimSpace(condition[:index])
	value = strings.TrimSpace(condition[index+len(separator):])

	if !isIdentifier(field) || value == "" || strings.ContainsAny(value, " \t") {
		return "", "", "", false
	}

	return field, separator, value, true
}

func isIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '.' {
			return false
		}
	}
	return true
}
