package policy_test

import (
	"strings"
	"testing"

	"github.com/GhandyP/enterprise-policy-procedures/internal/policy"
)

// inForceJSON is a corpus whose selected version is internally consistent:
// every rule applies at the effective date, and the only rule that requires an
// approval has a gate for it. Index must report no blocker at all.
const inForceJSON = `{
  "policyID": "HR-EXPENSE-CLAIMS",
  "effectiveDate": "2026-04-01",
  "versions": [
    {
      "version": "2.2",
      "sections": [
        {
          "id": "S1",
          "title": "Submission",
          "rules": [
            {
              "id": "C1",
              "text": "Claims must be submitted within 30 days of the expense.",
              "owner": "Finance Controller",
              "effectiveFrom": "2025-01-01"
            },
            {
              "id": "C2",
              "text": "Claims above 5000 EUR need a second approver.",
              "owner": "Finance Controller",
              "effectiveFrom": "2026-04-01",
              "requiresApproval": true
            }
          ]
        }
      ],
      "approvals": [
        {
          "id": "A1",
          "ruleID": "C2",
          "approver": "Head of Finance",
          "role": "Finance Controller"
        }
      ]
    }
  ]
}`

// conflictingJSON supplies the same rule id twice with different text. C1 moved
// from 30 to 45 days, and nothing in the document says which one is the truth.
const conflictingJSON = `{
  "policyID": "HR-EXPENSE-CLAIMS",
  "effectiveDate": "2026-04-01",
  "versions": [
    {
      "version": "2.1",
      "sections": [
        {"id": "S1", "title": "Submission", "rules": [
          {"id": "C1", "text": "Claims must be submitted within 30 days of the expense.", "owner": "Finance Controller", "effectiveFrom": "2025-01-01"}
        ]}
      ]
    },
    {
      "version": "2.2",
      "sections": [
        {"id": "S1", "title": "Submission", "rules": [
          {"id": "C1", "text": "Claims must be submitted within 45 days of the expense.", "owner": "Finance Controller", "effectiveFrom": "2026-04-01"}
        ]}
      ]
    }
  ]
}`

func indexOf(t *testing.T, corpus policy.Corpus, version string) (policy.Index, []policy.Issue) {
	t.Helper()
	return corpus.Index(version)
}

func ruleFactWithID(t *testing.T, facts []policy.RuleFact, id string) policy.RuleFact {
	t.Helper()

	for _, fact := range facts {
		if fact.Rule.ID == id {
			return fact
		}
	}
	t.Fatalf("no rule fact with id %q in %+v", id, facts)
	return policy.RuleFact{}
}

func issuesWithCode(t *testing.T, issues []policy.Issue, code string) []policy.Issue {
	t.Helper()

	var found []policy.Issue
	for _, issue := range issues {
		if issue.Code == code {
			found = append(found, issue)
		}
	}
	return found
}

// TestIndexReportsNothingForAConsistentVersion is the control for every other
// test in this file: a version whose rules all apply and whose approvals are
// all satisfied must produce no blocker.
func TestIndexReportsNothingForAConsistentVersion(t *testing.T) {
	index, issues := indexOf(t, mustParse(t, inForceJSON), "2.2")
	if len(issues) != 0 {
		t.Fatalf("Index reported blockers for a consistent version: %+v", issues)
	}

	if got, want := index.PolicyID, "HR-EXPENSE-CLAIMS"; got != want {
		t.Errorf("PolicyID = %q, want %q", got, want)
	}
	if got, want := index.Version, "2.2"; got != want {
		t.Errorf("Version = %q, want %q", got, want)
	}
	if got, want := index.EffectiveDate.Format("2006-01-02"), "2026-04-01"; got != want {
		t.Errorf("EffectiveDate = %q, want %q", got, want)
	}
	if got, want := len(index.Rules), 2; got != want {
		t.Fatalf("len(Rules) = %d, want %d", got, want)
	}
}

// TestIndexCarriesTheSectionEachRuleCameFrom pins that a step can name where
// its rule lives, which the citations of S9 need.
func TestIndexCarriesTheSectionEachRuleCameFrom(t *testing.T) {
	index, issues := indexOf(t, mustParse(t, inForceJSON), "2.2")
	if len(issues) != 0 {
		t.Fatalf("Index reported blockers: %+v", issues)
	}

	fact := ruleFactWithID(t, index.Rules, "C1")
	if got, want := fact.SectionID, "S1"; got != want {
		t.Errorf("SectionID = %q, want %q", got, want)
	}
	if got, want := fact.Rule.Owner, "Finance Controller"; got != want {
		t.Errorf("Rule.Owner = %q, want %q", got, want)
	}
}

// TestIndexAttachesTheApprovalGateToItsRule pins that a rule needing approval
// and its gate are joined by the rule id, so a later service can decide whether
// the gate is satisfied without re-walking the document.
func TestIndexAttachesTheApprovalGateToItsRule(t *testing.T) {
	index, issues := indexOf(t, mustParse(t, inForceJSON), "2.2")
	if len(issues) != 0 {
		t.Fatalf("Index reported blockers: %+v", issues)
	}

	gated := ruleFactWithID(t, index.Rules, "C2")
	if gated.Approval == nil {
		t.Fatal("C2 requires approval but no gate was attached")
	}
	if got, want := gated.Approval.Approver, "Head of Finance"; got != want {
		t.Errorf("Approval.Approver = %q, want %q", got, want)
	}

	plain := ruleFactWithID(t, index.Rules, "C1")
	if plain.Approval != nil {
		t.Errorf("C1 does not require approval but a gate was attached: %+v", plain.Approval)
	}
}

// TestIndexMarksARuleObsoleteAtTheEffectiveDate pins A5 for a rule whose window
// closed before the compile's effective date. The shared corpus carries such a
// rule on purpose: R4 was retired on 2025-12-31.
func TestIndexMarksARuleObsoleteAtTheEffectiveDate(t *testing.T) {
	index, issues := indexOf(t, mustParse(t, corpusJSON), "3.1")

	retired := ruleFactWithID(t, index.Rules, "R4")
	if !retired.Obsolete {
		t.Error("R4 was retired on 2025-12-31 but is not marked obsolete at the 2026-03-01 effective date")
	}

	inForce := ruleFactWithID(t, index.Rules, "R2")
	if inForce.Obsolete {
		t.Error("R2 has no effectiveTo and must not be marked obsolete")
	}

	var obsoleteRules []string
	for _, issue := range issuesWithCode(t, issues, policy.CodeObsoleteRule) {
		obsoleteRules = append(obsoleteRules, issue.RuleID)
	}
	if len(obsoleteRules) != 1 || obsoleteRules[0] != "R4" {
		t.Errorf("obsolete_rule issues = %v, want exactly [R4]", obsoleteRules)
	}

	for _, issue := range issuesWithCode(t, issues, policy.CodeObsoleteRule) {
		if !strings.Contains(issue.Path, "rules[3]") {
			t.Errorf("issue %+v does not point at the retired rule", issue)
		}
	}
}

// TestIndexReportsAVersionConflict pins A5's first case: the same rule id with
// different text in two supplied versions cannot be resolved by the compiler.
func TestIndexReportsAVersionConflict(t *testing.T) {
	_, issues := indexOf(t, mustParse(t, conflictingJSON), "2.2")

	conflicts := issuesWithCode(t, issues, policy.CodeVersionConflict)
	if len(conflicts) != 1 {
		t.Fatalf("version_conflict issues = %+v, want exactly one", conflicts)
	}
	if got, want := conflicts[0].RuleID, "C1"; got != want {
		t.Errorf("RuleID = %q, want %q", got, want)
	}
	for _, want := range []string{"2.1", "2.2", "30 days", "45 days"} {
		if !strings.Contains(conflicts[0].Message, want) {
			t.Errorf("message %q does not name %q, so the policy owner cannot see both texts", conflicts[0].Message, want)
		}
	}
}

// TestIndexAcceptsTheSameTextInTwoVersions pins the other half of A5: a rule id
// repeated across versions is the ordinary case and only a differing text is a
// conflict.
func TestIndexAcceptsTheSameTextInTwoVersions(t *testing.T) {
	_, issues := policy.Parse([]byte(`{
      "policyID": "P",
      "effectiveDate": "2026-01-01",
      "versions": [
        {"version": "1", "sections": [{"id": "S1", "title": "T", "rules": [
          {"id": "R1", "text": "same text", "owner": "O", "effectiveFrom": "2025-01-01"}
        ]}]},
        {"version": "2", "sections": [{"id": "S1", "title": "T", "rules": [
          {"id": "R1", "text": "same text", "owner": "O", "effectiveFrom": "2026-01-01"}
        ]}]}
      ]
    }`))
	if len(issues) != 0 {
		t.Fatalf("Parse reported issues: %+v", issues)
	}

	// Parse cannot see a conflict; Index is what detects it. Re-read the same
	// document through Index to prove identical text is not reported.
	corpus, _ := policy.Parse([]byte(`{
      "policyID": "P",
      "effectiveDate": "2026-01-01",
      "versions": [
        {"version": "1", "sections": [{"id": "S1", "title": "T", "rules": [
          {"id": "R1", "text": "same text", "owner": "O", "effectiveFrom": "2025-01-01"}
        ]}]},
        {"version": "2", "sections": [{"id": "S1", "title": "T", "rules": [
          {"id": "R1", "text": "same text", "owner": "O", "effectiveFrom": "2026-01-01"}
        ]}]}
      ]
    }`))
	if _, issues := corpus.Index("2"); len(issues) != 0 {
		t.Errorf("identical text in two versions was reported as a problem: %+v", issues)
	}
}

// TestIndexRejectsAMalformedDate pins that the effective-date service refuses a
// date it cannot read instead of guessing a layout.
func TestIndexRejectsAMalformedDate(t *testing.T) {
	tests := []struct {
		name      string
		document  string
		wantPaths int
		wantIn    string
	}{
		{
			name: "document effective date",
			document: `{"policyID":"P","effectiveDate":"01/03/2026","versions":[
              {"version":"1","sections":[{"id":"S1","title":"T","rules":[
                {"id":"R1","text":"t","owner":"O","effectiveFrom":"2026-01-01"}]}]}]}`,
			wantPaths: 1,
			wantIn:    "effectiveDate",
		},
		{
			name: "rule effectiveFrom",
			document: `{"policyID":"P","effectiveDate":"2026-01-01","versions":[
              {"version":"1","sections":[{"id":"S1","title":"T","rules":[
                {"id":"R1","text":"t","owner":"O","effectiveFrom":"March 1"}]}]}]}`,
			wantPaths: 1,
			wantIn:    "versions[0].sections[0].rules[0].effectiveFrom",
		},
		{
			name: "rule effectiveTo",
			document: `{"policyID":"P","effectiveDate":"2026-01-01","versions":[
              {"version":"1","sections":[{"id":"S1","title":"T","rules":[
                {"id":"R1","text":"t","owner":"O","effectiveFrom":"2026-01-01","effectiveTo":"2025-13-01"}]}]}]}`,
			wantPaths: 1,
			wantIn:    "versions[0].sections[0].rules[0].effectiveTo",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			corpus, issues := policy.Parse([]byte(test.document))
			if len(issues) != 0 {
				t.Fatalf("Parse rejected the document structurally: %+v", issues)
			}

			_, issues = corpus.Index("1")
			invalid := issuesWithCode(t, issues, policy.CodeInvalidDate)
			if len(invalid) != test.wantPaths {
				t.Fatalf("invalid_date issues = %+v, want %d", invalid, test.wantPaths)
			}
			if got := invalid[0].Path; got != test.wantIn {
				t.Errorf("Path = %q, want %q", got, test.wantIn)
			}
		})
	}
}

// TestIndexDoesNotReportADateIssueForACleanVersion keeps the invalid_date rule
// from firing on a well-formed document.
func TestIndexDoesNotReportADateIssueForACleanVersion(t *testing.T) {
	_, issues := indexOf(t, mustParse(t, inForceJSON), "2.2")
	if len(issuesWithCode(t, issues, policy.CodeInvalidDate)) != 0 {
		t.Errorf("valid dates reported as invalid_date: %+v", issues)
	}
}

// TestApprovalValidationRejectsAnOrphanGate pins that a gate naming a rule the
// version does not define is reported instead of being silently ignored: it
// would otherwise look like an approved rule that is never actually gated.
func TestApprovalValidationRejectsAnOrphanGate(t *testing.T) {
	corpus, issues := policy.Parse([]byte(`{
      "policyID": "P",
      "effectiveDate": "2026-01-01",
      "versions": [
        {
          "version": "1",
          "sections": [{"id": "S1", "title": "T", "rules": [
            {"id": "R1", "text": "t", "owner": "O", "effectiveFrom": "2026-01-01"}
          ]}],
          "approvals": [
            {"id": "A1", "ruleID": "NOPE", "approver": "Someone"}
          ]
        }
      ]
    }`))
	if len(issues) != 0 {
		t.Fatalf("Parse rejected the document structurally: %+v", issues)
	}

	_, issues = corpus.Index("1")
	orphans := issuesWithCode(t, issues, policy.CodeOrphanApproval)
	if len(orphans) != 1 {
		t.Fatalf("orphan_approval issues = %+v, want exactly one", orphans)
	}
	if got, want := orphans[0].RuleID, "NOPE"; got != want {
		t.Errorf("RuleID = %q, want %q", got, want)
	}
	if got, want := orphans[0].Path, "versions[0].approvals[0].ruleID"; got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}

// TestParseRejectsAnApprovalMissingItsRequiredFields pins the required shape of
// a gate, since a gate with no approver would claim an approval that no person
// gave.
func TestParseRejectsAnApprovalMissingItsRequiredFields(t *testing.T) {
	tests := []struct {
		name     string
		approval string
		wantPath string
	}{
		{
			name:     "no id",
			approval: `{"ruleID":"R1","approver":"Someone"}`,
			wantPath: "versions[0].approvals[0].id",
		},
		{
			name:     "no ruleID",
			approval: `{"id":"A1","approver":"Someone"}`,
			wantPath: "versions[0].approvals[0].ruleID",
		},
		{
			name:     "no approver",
			approval: `{"id":"A1","ruleID":"R1"}`,
			wantPath: "versions[0].approvals[0].approver",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, issues := policy.Parse([]byte(`{
              "policyID": "P",
              "effectiveDate": "2026-01-01",
              "versions": [
                {
                  "version": "1",
                  "sections": [{"id": "S1", "title": "T", "rules": [
                    {"id": "R1", "text": "t", "owner": "O", "effectiveFrom": "2026-01-01"}
                  ]}],
                  "approvals": [` + test.approval + `]
                }
              ]
            }`))
			if len(issues) != 1 {
				t.Fatalf("Parse reported %d issues, want exactly 1: %+v", len(issues), issues)
			}
			if got, want := issues[0].Code, policy.CodeInvalidInput; got != want {
				t.Errorf("Code = %q, want %q", got, want)
			}
			if got, want := issues[0].Path, test.wantPath; got != want {
				t.Errorf("Path = %q, want %q", got, want)
			}
		})
	}
}

// TestIndexRejectsAnUnknownVersion proves the compile cannot proceed on a
// version the document does not supply, and returns no rules when it refuses.
func TestIndexRejectsAnUnknownVersion(t *testing.T) {
	index, issues := indexOf(t, mustParse(t, inForceJSON), "9.9")
	if len(issues) == 0 {
		t.Fatal("Index accepted a version the document does not supply")
	}
	if got, want := issues[0].Code, policy.CodeUnknownVersion; got != want {
		t.Errorf("Code = %q, want %q", got, want)
	}
	if len(index.Rules) != 0 {
		t.Errorf("Index returned rules on failure: %+v", index.Rules)
	}
}
