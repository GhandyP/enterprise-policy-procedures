package procedure

import (
	"fmt"
	"strings"

	"github.com/GhandyP/enterprise-policy-procedures/internal/policy"
	"github.com/GhandyP/enterprise-policy-procedures/internal/trace"
)

// RenderPrompt writes the whole payload handed to the model: the policy it is
// drafting for, and the rules the deterministic trace produced. Only traced
// rules are rendered, so a rule that is not in force at the effective date never
// reaches the model and no citation to it can be manufactured.
func RenderPrompt(tr trace.Trace) string {
	var prompt strings.Builder

	fmt.Fprintf(&prompt, "Policy: %s\nVersion: %s\nEffective date: %s\n\n",
		tr.PolicyID, tr.Version, tr.EffectiveDate.Format(policy.DateLayout))

	if len(tr.Rules) == 0 {
		prompt.WriteString("This version has no rule in force, so no procedure can be drafted from it.\n")
		return prompt.String()
	}

	prompt.WriteString("Rules in force. Every step must cite at least one of these ruleIDs, copied exactly:\n\n")
	for _, rule := range tr.Rules {
		fmt.Fprintf(&prompt, "- ruleID: %s\n  section: %s\n  owner: %s\n  text: %s\n",
			rule.Rule.ID, rule.SectionID, rule.Rule.Owner, rule.Rule.Text)

		if rule.Branch.Field != "" {
			fmt.Fprintf(&prompt, "  condition: %s %s %s\n", rule.Branch.Field, rule.Branch.Operator, rule.Branch.Value)
		}
		if rule.Rule.ExceptionOf != "" {
			fmt.Fprintf(&prompt, "  exception to: %s\n", rule.Rule.ExceptionOf)
		}
		if rule.Approval != nil {
			fmt.Fprintf(&prompt, "  approval gate: %s (%s)\n", rule.Approval.Approver, rule.Approval.Role)
		}
	}

	if len(tr.Decisions) > 0 {
		prompt.WriteString("\nDecisions this policy already declares. Do not invent others, and do not resolve them yourself:\n\n")
		for _, decision := range tr.Decisions {
			branches := make([]string, 0, len(decision.Branches))
			for _, branch := range decision.Branches {
				branches = append(branches, fmt.Sprintf("%s %s %s (%s)", branch.Field, branch.Operator, branch.Value, branch.RuleID))
			}
			fmt.Fprintf(&prompt, "- %s: %s\n", decision.Field, strings.Join(branches, "; "))
		}
	}

	return prompt.String()
}
