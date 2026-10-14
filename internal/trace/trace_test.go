package trace_test

import (
	"testing"

	"github.com/GhandyP/enterprise-policy-procedures/internal/policy"
	"github.com/GhandyP/enterprise-policy-procedures/internal/trace"
)

// traceJSON is a version whose rules are all in force and whose single approval
// gate is satisfied. Two of its rules branch on the same condition field, so it
// also exercises a decision with more than one branch.
const traceJSON = `{
  "policyID": "IT-ACCEPTABLE-USE",
  "effectiveDate": "2026-03-01",
  "versions": [
    {
      "version": "3.1",
      "sections": [
        {
          "id": "S1",
          "title": "Scope",
          "rules": [
            {
              "id": "R1",
              "text": "Applies to every employee and contractor.",
              "owner": "IT",
              "effectiveFrom": "2026-03-01"
            }
          ]
        },
        {
          "id": "S2",
          "title": "Devices",
          "rules": [
            {
              "id": "R2",
              "text": "Devices must be enrolled before use.",
              "owner": "IT Security",
              "effectiveFrom": "2026-03-01",
              "requiresApproval": true
            },
            {
              "id": "R3",
              "text": "Contractor personal devices are excluded.",
              "owner": "IT Security",
              "effectiveFrom": "2026-03-01",
              "exceptionOf": "R2",
              "condition": "deviceOwner == contractor"
            },
            {
              "id": "R4",
              "text": "Employee devices must be managed by IT.",
              "owner": "IT Security",
              "effectiveFrom": "2026-03-01",
              "condition": "deviceOwner == employee"
            }
          ]
        }
      ],
      "approvals": [
        {
          "id": "A1",
          "ruleID": "R2",
          "approver": "Head of IT Security",
          "role": "IT Security"
        }
      ]
    }
  ]
}`

// build runs the documented pipeline: parse, index one version, and trace it.
// Parse and Build must both be clean, because the tests that call build are
// about the trace itself. The index is deliberately not asserted on: what it
// reports for a rule that is not in force is internal/policy's business and is
// pinned by its own tests.
func build(t *testing.T, document, version string) trace.Trace {
	t.Helper()

	corpus, issues := policy.Parse([]byte(document))
	if len(issues) != 0 {
		t.Fatalf("Parse reported issues: %+v", issues)
	}
	index, _ := corpus.Index(version)

	tr, issues := trace.Build(index)
	if len(issues) != 0 {
		t.Fatalf("Build reported issues for a clean version: %+v", issues)
	}
	return tr
}

func buildIssues(t *testing.T, document, version string) []policy.Issue {
	t.Helper()

	corpus, issues := policy.Parse([]byte(document))
	if len(issues) != 0 {
		t.Fatalf("Parse reported issues: %+v", issues)
	}
	index, _ := corpus.Index(version)

	_, issues = trace.Build(index)
	return issues
}

func withCode(issues []policy.Issue, code string) []policy.Issue {
	var found []policy.Issue
	for _, issue := range issues {
		if issue.Code == code {
			found = append(found, issue)
		}
	}
	return found
}

func ruleWithID(t *testing.T, rules []trace.Rule, id string) trace.Rule {
	t.Helper()

	for _, rule := range rules {
		if rule.Rule.ID == id {
			return rule
		}
	}
	t.Fatalf("no traced rule with id %q in %+v", id, rules)
	return trace.Rule{}
}

// TestBuildTracesEveryRuleInForce pins A2's raw material: the trace is the set
// of rules a step may cite, each already carrying the citation it must print.
func TestBuildTracesEveryRuleInForce(t *testing.T) {
	tr := build(t, traceJSON, "3.1")

	if got, want := tr.PolicyID, "IT-ACCEPTABLE-USE"; got != want {
		t.Errorf("PolicyID = %q, want %q", got, want)
	}
	if got, want := tr.Version, "3.1"; got != want {
		t.Errorf("Version = %q, want %q", got, want)
	}
	if got, want := len(tr.Rules), 4; got != want {
		t.Fatalf("len(Rules) = %d, want %d: every rule in force must be citable", got, want)
	}

	enrolment := ruleWithID(t, tr.Rules, "R2")
	if got, want := enrolment.Citation.RuleID, "R2"; got != want {
		t.Errorf("Citation.RuleID = %q, want %q", got, want)
	}
	if got, want := enrolment.Citation.Version, "3.1"; got != want {
		t.Errorf("Citation.Version = %q, want %q", got, want)
	}
	if got, want := enrolment.Citation.String(), "R2@3.1"; got != want {
		t.Errorf("Citation.String() = %q, want %q: a step must be able to print the rule and its version", got, want)
	}
	if got, want := enrolment.SectionID, "S2"; got != want {
		t.Errorf("SectionID = %q, want %q", got, want)
	}
	if enrolment.Approval == nil {
		t.Error("R2 requires approval but the trace carries no gate for it")
	}
}

// TestBuildCitesOnlyTheTracedRules pins the check A3 leans on: membership in the
// trace is what makes a citation acceptable.
func TestBuildCitesOnlyTheTracedRules(t *testing.T) {
	tr := build(t, traceJSON, "3.1")

	for _, citable := range []string{"R1", "R2", "R3", "R4"} {
		if !tr.Cites(citable) {
			t.Errorf("Cites(%q) = false, want true", citable)
		}
	}
	for _, foreign := range []string{"R9", "", "r2", "R2@3.0"} {
		if tr.Cites(foreign) {
			t.Errorf("Cites(%q) = true, want false: only the traced rule ids are citable", foreign)
		}
	}
}

// TestBuildLeavesOutARuleThatIsNotInForce pins A3's obsolete case: a rule the
// index did not find in force never reaches the model, so the model cannot cite
// it and the answer can never contain it.
func TestBuildLeavesOutARuleThatIsNotInForce(t *testing.T) {
	tr := build(t, `{
      "policyID": "NET-RETIREMENTS",
      "effectiveDate": "2026-03-01",
      "versions": [
        {"version": "1.0", "sections": [{"id": "S1", "title": "Access", "rules": [
          {"id": "N1", "text": "Legacy VPN access is granted on request.", "owner": "Network", "effectiveFrom": "2024-01-01"},
          {"id": "N2", "text": "The legacy VPN was retired.", "owner": "Network", "effectiveFrom": "2024-01-01", "effectiveTo": "2025-12-31"}
        ]}]}
      ]
    }`, "1.0")

	if got, want := len(tr.Rules), 1; got != want {
		t.Fatalf("len(Rules) = %d, want %d: only the rule in force may be cited", got, want)
	}
	if got, want := tr.Rules[0].Rule.ID, "N1"; got != want {
		t.Errorf("Rules[0].Rule.ID = %q, want %q", got, want)
	}
	if tr.Cites("N2") {
		t.Error("Cites(N2) = true, but N2 stopped applying before the effective date")
	}
}

// TestBuildBlocksARuleNeedingApprovalWithNoGate pins A4: the compile stops and
// names the rule, instead of drafting a step nobody is authorised to approve.
func TestBuildBlocksARuleNeedingApprovalWithNoGate(t *testing.T) {
	issues := buildIssues(t, `{
      "policyID": "HR-EXPENSE-CLAIMS",
      "effectiveDate": "2026-04-01",
      "versions": [
        {"version": "2.2", "sections": [{"id": "S1", "title": "Submission", "rules": [
          {"id": "C1", "text": "Claims need a second approver above 5000 EUR.", "owner": "Finance Controller", "effectiveFrom": "2026-04-01", "requiresApproval": true}
        ]}]}
      ]
    }`, "2.2")

	missing := withCode(issues, trace.CodeMissingApproval)
	if len(missing) != 1 {
		t.Fatalf("missing_approval issues = %+v, want exactly one", missing)
	}
	if got, want := missing[0].RuleID, "C1"; got != want {
		t.Errorf("RuleID = %q, want %q", got, want)
	}
	if missing[0].Message == "" {
		t.Error("the blocker carries no message for the policy owner")
	}
}

// TestBuildAcceptsARuleWhoseGateExists is the control for A4.
func TestBuildAcceptsARuleWhoseGateExists(t *testing.T) {
	issues := buildIssues(t, traceJSON, "3.1")
	if len(withCode(issues, trace.CodeMissingApproval)) != 0 {
		t.Errorf("a satisfied gate was reported as missing: %+v", issues)
	}
}

// TestBuildRejectsAnExceptionOfARuleTheVersionDoesNotDefine pins A6's validation
// of exceptions: an exception to a rule that does not exist excepts nothing.
func TestBuildRejectsAnExceptionOfARuleTheVersionDoesNotDefine(t *testing.T) {
	issues := buildIssues(t, `{
      "policyID": "IT-ACCEPTABLE-USE",
      "effectiveDate": "2026-03-01",
      "versions": [
        {"version": "3.1", "sections": [{"id": "S1", "title": "Devices", "rules": [
          {"id": "R1", "text": "Devices must be enrolled.", "owner": "IT Security", "effectiveFrom": "2026-03-01"},
          {"id": "R2", "text": "Warehouse scanners are excluded.", "owner": "IT Security", "effectiveFrom": "2026-03-01", "exceptionOf": "NOPE", "condition": "deviceKind == scanner"}
        ]}]}
      ]
    }`, "3.1")

	dangling := withCode(issues, trace.CodeDanglingException)
	if len(dangling) != 1 {
		t.Fatalf("dangling_exception issues = %+v, want exactly one", dangling)
	}
	if got, want := dangling[0].RuleID, "R2"; got != want {
		t.Errorf("RuleID = %q, want %q", got, want)
	}
}

// TestBuildRejectsAnExceptionThatBranchesADifferentField pins the other half of
// A6: an exception has to branch the same condition as the rule it excepts,
// otherwise it narrows something other than the rule it claims to except.
func TestBuildRejectsAnExceptionThatBranchesADifferentField(t *testing.T) {
	issues := buildIssues(t, `{
      "policyID": "IT-ACCEPTABLE-USE",
      "effectiveDate": "2026-03-01",
      "versions": [
        {"version": "3.1", "sections": [{"id": "S1", "title": "Devices", "rules": [
          {"id": "R1", "text": "Devices must be enrolled.", "owner": "IT Security", "effectiveFrom": "2026-03-01", "condition": "deviceOwner == employee"},
          {"id": "R2", "text": "Warehouse scanners are excluded.", "owner": "IT Security", "effectiveFrom": "2026-03-01", "exceptionOf": "R1", "condition": "site == warehouse"}
        ]}]}
      ]
    }`, "3.1")

	unrelated := withCode(issues, trace.CodeUnrelatedException)
	if len(unrelated) != 1 {
		t.Fatalf("unrelated_exception issues = %+v, want exactly one", unrelated)
	}
	if got, want := unrelated[0].RuleID, "R2"; got != want {
		t.Errorf("RuleID = %q, want %q", got, want)
	}
}

// TestBuildParsesConditionsIntoBranches pins S20 step 3: the branch a condition
// describes is derived in Go, not read out of model prose.
func TestBuildParsesConditionsIntoBranches(t *testing.T) {
	tr := build(t, traceJSON, "3.1")

	exception := ruleWithID(t, tr.Rules, "R3")
	if !exception.Branch.Exception {
		t.Error("R3 excepts R2 but its branch is not marked as an exception")
	}
	if got, want := exception.Branch.ExceptOf, "R2"; got != want {
		t.Errorf("Branch.ExceptOf = %q, want %q", got, want)
	}
	if got, want := exception.Branch.Field, "deviceOwner"; got != want {
		t.Errorf("Branch.Field = %q, want %q", got, want)
	}
	if got, want := exception.Branch.Operator, "=="; got != want {
		t.Errorf("Branch.Operator = %q, want %q", got, want)
	}
	if got, want := exception.Branch.Value, "contractor"; got != want {
		t.Errorf("Branch.Value = %q, want %q", got, want)
	}

	plain := ruleWithID(t, tr.Rules, "R1")
	if plain.Branch.Exception {
		t.Error("R1 has no condition but was given an exception branch")
	}
	if plain.Branch.Field != "" {
		t.Errorf("R1 has no condition but Branch.Field = %q", plain.Branch.Field)
	}
}

// TestBuildGroupsBranchesIntoDecisions pins the decisions S5 asks the procedure
// to carry, derived from the conditions the policy actually declares.
func TestBuildGroupsBranchesIntoDecisions(t *testing.T) {
	tr := build(t, traceJSON, "3.1")

	if got, want := len(tr.Decisions), 1; got != want {
		t.Fatalf("len(Decisions) = %d, want %d: %+v", got, want, tr.Decisions)
	}
	decision := tr.Decisions[0]
	if got, want := decision.Field, "deviceOwner"; got != want {
		t.Errorf("Decision.Field = %q, want %q", got, want)
	}
	if got, want := len(decision.Branches), 2; got != want {
		t.Fatalf("len(Decision.Branches) = %d, want %d: %+v", got, want, decision.Branches)
	}

	values := []string{decision.Branches[0].Value, decision.Branches[1].Value}
	if values[0] != "contractor" || values[1] != "employee" {
		t.Errorf("branch values = %v, want [contractor employee] in document order", values)
	}
}

// TestBuildRejectsAConditionItCannotRead pins the documented grammar. A policy
// owner who writes prose into a condition gets told, instead of that prose being
// passed to the model as if it were evaluated somewhere.
func TestBuildRejectsAConditionItCannotRead(t *testing.T) {
	tests := []struct {
		name      string
		condition string
	}{
		{name: "no operator", condition: "deviceOwner"},
		{name: "unsupported operator", condition: "amount > 5000"},
		{name: "missing value", condition: "deviceOwner =="},
		{name: "missing field", condition: "== contractor"},
		{name: "value with spaces", condition: "deviceOwner == a contractor"},
		{name: "field is not an identifier", condition: "device owner == contractor"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			issues := buildIssues(t, `{
              "policyID": "P",
              "effectiveDate": "2026-03-01",
              "versions": [
                {"version": "1", "sections": [{"id": "S1", "title": "T", "rules": [
                  {"id": "R1", "text": "t", "owner": "O", "effectiveFrom": "2026-03-01", "condition": "`+test.condition+`"}
                ]}]}
              ]
            }`, "1")

			invalid := withCode(issues, trace.CodeInvalidCondition)
			if len(invalid) != 1 {
				t.Fatalf("invalid_condition issues = %+v, want exactly one for condition %q", invalid, test.condition)
			}
			if got, want := invalid[0].RuleID, "R1"; got != want {
				t.Errorf("RuleID = %q, want %q", got, want)
			}
		})
	}
}

// TestBuildReportsEveryBlockerInOnePass pins the decision that a policy owner
// fixes one list instead of rediscovering the next blocker after each fix.
func TestBuildReportsEveryBlockerInOnePass(t *testing.T) {
	issues := buildIssues(t, `{
      "policyID": "PROC-PURCHASING",
      "effectiveDate": "2026-03-01",
      "versions": [
        {"version": "1.2", "sections": [{"id": "S1", "title": "Approvals", "rules": [
          {"id": "P1", "text": "Purchases above 50000 EUR need a second approver.", "owner": "Procurement Lead", "effectiveFrom": "2026-03-01", "requiresApproval": true},
          {"id": "P2", "text": "Framework suppliers are excluded.", "owner": "Procurement Lead", "effectiveFrom": "2026-03-01", "exceptionOf": "GONE", "condition": "supplier == framed"},
          {"id": "P3", "text": "Bulk orders need a quote.", "owner": "Procurement Lead", "effectiveFrom": "2026-03-01", "condition": "amount above threshold"}
        ]}]}
      ]
    }`, "1.2")

	wantCodes := []string{trace.CodeMissingApproval, trace.CodeDanglingException, trace.CodeInvalidCondition}
	for _, code := range wantCodes {
		if len(withCode(issues, code)) != 1 {
			t.Errorf("code %q reported %d times, want exactly 1: %+v", code, len(withCode(issues, code)), issues)
		}
	}
	if got, want := len(issues), len(wantCodes); got != want {
		t.Errorf("issues = %d, want %d: %+v", got, want, issues)
	}
}
