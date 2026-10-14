package policy_test

import (
	"strings"
	"testing"

	"github.com/GhandyP/enterprise-policy-procedures/internal/policy"
)

// corpusJSON is a minimal but complete supplied policy: two versions of one
// policy, where the newer version adds a rule that requires an approval, an
// exception of that rule with a condition, and a rule already retired.
const corpusJSON = `{
  "policyID": "IT-ACCEPTABLE-USE",
  "effectiveDate": "2026-03-01",
  "versions": [
    {
      "version": "3.0",
      "sections": [
        {
          "id": "S1",
          "title": "Scope",
          "rules": [
            {
              "id": "R1",
              "text": "Applies to every employee.",
              "owner": "IT",
              "effectiveFrom": "2025-01-01"
            }
          ]
        }
      ]
    },
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
            },
            {
              "id": "R2",
              "text": "Devices must be enrolled before use.",
              "owner": "IT Security",
              "effectiveFrom": "2026-03-01",
              "requiresApproval": true
            },
            {
              "id": "R3",
              "text": "Personal devices of contractors are excluded.",
              "owner": "IT Security",
              "effectiveFrom": "2026-03-01",
              "exceptionOf": "R2",
              "condition": "deviceOwner == contractor"
            },
            {
              "id": "R4",
              "text": "The legacy VPN was retired.",
              "owner": "Network",
              "effectiveFrom": "2024-01-01",
              "effectiveTo": "2025-12-31"
            }
          ]
        }
      ]
    }
  ]
}`

func mustParse(t *testing.T, document string) policy.Corpus {
	t.Helper()

	corpus, issues := policy.Parse([]byte(document))
	if len(issues) != 0 {
		t.Fatalf("Parse returned issues for a valid document: %+v", issues)
	}
	return corpus
}

func ruleWithID(t *testing.T, rules []policy.Rule, id string) policy.Rule {
	t.Helper()

	for _, rule := range rules {
		if rule.ID == id {
			return rule
		}
	}
	t.Fatalf("no rule with id %q in %+v", id, rules)
	return policy.Rule{}
}

// TestParseDecodesEverySuppliedField pins the contract in S5 and the rule fields
// the compiler's later decisions read: id, text, owner, effectiveFrom, the
// optional effectiveTo, condition, exceptionOf and requiresApproval.
func TestParseDecodesEverySuppliedField(t *testing.T) {
	corpus := mustParse(t, corpusJSON)

	if got, want := corpus.PolicyID, "IT-ACCEPTABLE-USE"; got != want {
		t.Errorf("PolicyID = %q, want %q", got, want)
	}
	if got, want := corpus.EffectiveDate, "2026-03-01"; got != want {
		t.Errorf("EffectiveDate = %q, want %q", got, want)
	}
	if got, want := len(corpus.Versions), 2; got != want {
		t.Fatalf("len(Versions) = %d, want %d", got, want)
	}
	if got, want := corpus.Versions[0].Version, "3.0"; got != want {
		t.Errorf("Versions[0].Version = %q, want %q", got, want)
	}
	if got, want := corpus.Versions[1].Version, "3.1"; got != want {
		t.Errorf("Versions[1].Version = %q, want %q", got, want)
	}

	section := corpus.Versions[1].Sections[0]
	if got, want := section.ID, "S1"; got != want {
		t.Errorf("Sections[0].ID = %q, want %q", got, want)
	}
	if got, want := section.Title, "Scope"; got != want {
		t.Errorf("Sections[0].Title = %q, want %q", got, want)
	}
	if got, want := len(section.Rules), 4; got != want {
		t.Fatalf("len(Sections[0].Rules) = %d, want %d", got, want)
	}

	enrolment := ruleWithID(t, section.Rules, "R2")
	if got, want := enrolment.Text, "Devices must be enrolled before use."; got != want {
		t.Errorf("R2.Text = %q, want %q", got, want)
	}
	if got, want := enrolment.Owner, "IT Security"; got != want {
		t.Errorf("R2.Owner = %q, want %q", got, want)
	}
	if !enrolment.RequiresApproval {
		t.Error("R2.RequiresApproval = false, want true")
	}

	exception := ruleWithID(t, section.Rules, "R3")
	if got, want := exception.ExceptionOf, "R2"; got != want {
		t.Errorf("R3.ExceptionOf = %q, want %q", got, want)
	}
	if got, want := exception.Condition, "deviceOwner == contractor"; got != want {
		t.Errorf("R3.Condition = %q, want %q", got, want)
	}

	retired := ruleWithID(t, section.Rules, "R4")
	if got, want := retired.EffectiveTo, "2025-12-31"; got != want {
		t.Errorf("R4.EffectiveTo = %q, want %q", got, want)
	}
	if got, want := retired.Owner, "Network"; got != want {
		t.Errorf("R4.Owner = %q, want %q", got, want)
	}
}

// TestParseAcceptsTheMinimumDocument proves nothing beyond the documented
// required fields is demanded of a policy owner.
func TestParseAcceptsTheMinimumDocument(t *testing.T) {
	corpus, issues := policy.Parse([]byte(`{
      "policyID": "P",
      "effectiveDate": "2026-01-01",
      "versions": [
        {
          "version": "1",
          "sections": [
            {
              "id": "S1",
              "title": "Only",
              "rules": [
                {"id": "R1", "text": "T", "owner": "O", "effectiveFrom": "2026-01-01"}
              ]
            }
          ]
        }
      ]
    }`))
	if len(issues) != 0 {
		t.Fatalf("Parse rejected the minimum document: %+v", issues)
	}

	rule := corpus.Versions[0].Sections[0].Rules[0]
	if rule.RequiresApproval || rule.Condition != "" || rule.ExceptionOf != "" || rule.EffectiveTo != "" {
		t.Errorf("optional fields were not left empty: %+v", rule)
	}
}

// TestSelectReturnsTheContractOfS5 pins the PolicyInput shape S5 names and
// proves the sections returned belong to the selected version alone.
func TestSelectReturnsTheContractOfS5(t *testing.T) {
	corpus := mustParse(t, corpusJSON)

	selected, issues := corpus.Select("3.1")
	if len(issues) != 0 {
		t.Fatalf("Select(3.1) returned issues: %+v", issues)
	}
	if got, want := selected.PolicyID, "IT-ACCEPTABLE-USE"; got != want {
		t.Errorf("PolicyID = %q, want %q", got, want)
	}
	if got, want := selected.Version, "3.1"; got != want {
		t.Errorf("Version = %q, want %q", got, want)
	}
	if got, want := len(selected.Sections), 1; got != want {
		t.Fatalf("len(Sections) = %d, want %d", got, want)
	}
	if got, want := len(selected.Sections[0].Rules), 4; got != want {
		t.Errorf("len(Sections[0].Rules) = %d, want %d", got, want)
	}

	older, issues := corpus.Select("3.0")
	if len(issues) != 0 {
		t.Fatalf("Select(3.0) returned issues: %+v", issues)
	}
	if got, want := len(older.Sections[0].Rules), 1; got != want {
		t.Errorf("Select(3.0) returned %d rules, want %d: the sections must come from the selected version, not from every supplied version", got, want)
	}
}

// TestSelectRejectsAnUnknownVersion pins the failure a policy owner sees when
// the requested version is not in the supplied document.
func TestSelectRejectsAnUnknownVersion(t *testing.T) {
	corpus := mustParse(t, corpusJSON)

	selected, issues := corpus.Select("9.9")
	if len(issues) == 0 {
		t.Fatal("Select(9.9) accepted a version the document does not supply")
	}
	if got, want := issues[0].Code, policy.CodeUnknownVersion; got != want {
		t.Errorf("issues[0].Code = %q, want %q", got, want)
	}
	if len(selected.Sections) != 0 {
		t.Errorf("Select returned sections on failure: %+v", selected)
	}

	message := issues[0].Message
	for _, want := range []string{"9.9", "3.0", "3.1"} {
		if !strings.Contains(message, want) {
			t.Errorf("message %q does not name %q", message, want)
		}
	}
}

// TestParseRejectsMalformedJSON covers the first thing a policy owner can get
// wrong: a file that is not the documented shape at all.
func TestParseRejectsMalformedJSON(t *testing.T) {
	_, issues := policy.Parse([]byte(`{"policyID": `))
	if len(issues) == 0 {
		t.Fatal("Parse accepted malformed JSON")
	}
	if got, want := issues[0].Code, policy.CodeInvalidInput; got != want {
		t.Errorf("issues[0].Code = %q, want %q", got, want)
	}
}

// TestValidateRejectsMissingRequiredFields pins, field by field, that every
// required element of S5 is demanded and that the message names the path.
func TestValidateRejectsMissingRequiredFields(t *testing.T) {
	const valid = `{"policyID":"P","effectiveDate":"2026-01-01","versions":[{"version":"1","sections":[{"id":"S1","title":"T","rules":[{"id":"R1","text":"T","owner":"O","effectiveFrom":"2026-01-01"}]}]}]}`

	tests := []struct {
		name     string
		document string
		wantPath string
	}{
		{
			name:     "policyID",
			document: `{"effectiveDate":"2026-01-01","versions":[{"version":"1","sections":[]}]}`,
			wantPath: "policyID",
		},
		{
			name:     "effectiveDate",
			document: `{"policyID":"P","versions":[{"version":"1","sections":[]}]}`,
			wantPath: "effectiveDate",
		},
		{
			name:     "no versions",
			document: `{"policyID":"P","effectiveDate":"2026-01-01"}`,
			wantPath: "versions",
		},
		{
			name:     "version string",
			document: `{"policyID":"P","effectiveDate":"2026-01-01","versions":[{"sections":[]}]}`,
			wantPath: "versions[0].version",
		},
		{
			name:     "section id",
			document: `{"policyID":"P","effectiveDate":"2026-01-01","versions":[{"version":"1","sections":[{"title":"T","rules":[]}]}]}`,
			wantPath: "versions[0].sections[0].id",
		},
		{
			name:     "section title",
			document: `{"policyID":"P","effectiveDate":"2026-01-01","versions":[{"version":"1","sections":[{"id":"S1","rules":[]}]}]}`,
			wantPath: "versions[0].sections[0].title",
		},
		{
			name:     "rule text",
			document: `{"policyID":"P","effectiveDate":"2026-01-01","versions":[{"version":"1","sections":[{"id":"S1","title":"T","rules":[{"id":"R1","owner":"O","effectiveFrom":"2026-01-01"}]}]}]}`,
			wantPath: "versions[0].sections[0].rules[0].text",
		},
		{
			name:     "rule owner",
			document: `{"policyID":"P","effectiveDate":"2026-01-01","versions":[{"version":"1","sections":[{"id":"S1","title":"T","rules":[{"id":"R1","text":"T","effectiveFrom":"2026-01-01"}]}]}]}`,
			wantPath: "versions[0].sections[0].rules[0].owner",
		},
		{
			name:     "rule effectiveFrom",
			document: `{"policyID":"P","effectiveDate":"2026-01-01","versions":[{"version":"1","sections":[{"id":"S1","title":"T","rules":[{"id":"R1","text":"T","owner":"O"}]}]}]}`,
			wantPath: "versions[0].sections[0].rules[0].effectiveFrom",
		},
		{
			name:     "rule id",
			document: `{"policyID":"P","effectiveDate":"2026-01-01","versions":[{"version":"1","sections":[{"id":"S1","title":"T","rules":[{"text":"T","owner":"O","effectiveFrom":"2026-01-01"}]}]}]}`,
			wantPath: "versions[0].sections[0].rules[0].id",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// The valid document is the control: it must produce no issue, so a
			// failure below cannot be blamed on the shared scaffold.
			if _, issues := policy.Parse([]byte(valid)); len(issues) != 0 {
				t.Fatalf("the control document reported issues: %+v", issues)
			}

			_, issues := policy.Parse([]byte(test.document))
			if len(issues) != 1 {
				t.Fatalf("Parse reported %d issues, want exactly 1: %+v", len(issues), issues)
			}
			if got, want := issues[0].Code, policy.CodeInvalidInput; got != want {
				t.Errorf("issues[0].Code = %q, want %q", got, want)
			}
			if got, want := issues[0].Path, test.wantPath; got != want {
				t.Errorf("issues[0].Path = %q, want %q (message: %s)", got, want, issues[0].Message)
			}
		})
	}
}

// TestValidateRejectsDuplicateRuleIDsInsideOneVersion pins that one version may
// not define the same rule id twice. The same id in two different versions is
// the ordinary case and is checked by the version-conflict rule instead.
func TestValidateRejectsDuplicateRuleIDsInsideOneVersion(t *testing.T) {
	_, issues := policy.Parse([]byte(`{
      "policyID": "P",
      "effectiveDate": "2026-01-01",
      "versions": [
        {
          "version": "1",
          "sections": [
            {"id": "S1", "title": "A", "rules": [
              {"id": "R1", "text": "first", "owner": "O", "effectiveFrom": "2026-01-01"},
              {"id": "R1", "text": "second", "owner": "O", "effectiveFrom": "2026-01-01"}
            ]},
            {"id": "S2", "title": "B", "rules": [
              {"id": "R1", "text": "third", "owner": "O", "effectiveFrom": "2026-01-01"}
            ]}
          ]
        }
      ]
    }`))
	if len(issues) != 1 {
		t.Fatalf("Parse reported %d issues, want exactly 1: %+v", len(issues), issues)
	}
	if got, want := issues[0].Code, policy.CodeDuplicateRuleID; got != want {
		t.Errorf("issues[0].Code = %q, want %q", got, want)
	}
	if !strings.Contains(issues[0].Message, "R1") {
		t.Errorf("message %q does not name the duplicated rule id", issues[0].Message)
	}

	// The same rule id in two versions is not a duplicate: it is expected.
	if _, issues := policy.Parse([]byte(corpusJSON)); len(issues) != 0 {
		t.Errorf("the shared two-version corpus reported issues: %+v", issues)
	}
}
