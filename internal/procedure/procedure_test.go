package procedure_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/GhandyP/enterprise-policy-procedures/internal/policy"
	"github.com/GhandyP/enterprise-policy-procedures/internal/procedure"
	"github.com/GhandyP/enterprise-policy-procedures/internal/trace"
)

// versionJSON is the version every test here compiles. It carries a rule that
// requires approval with a gate for it, an exception, a second branch on the
// same condition field, and a rule that stopped applying before the effective
// date so the trace can be shown to exclude it.
const versionJSON = `{
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
            },
            {
              "id": "R9",
              "text": "The legacy VPN was retired.",
              "owner": "Network",
              "effectiveFrom": "2024-01-01",
              "effectiveTo": "2025-12-31"
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

func versionTrace(t *testing.T) trace.Trace {
	t.Helper()

	corpus, issues := policy.Parse([]byte(versionJSON))
	if len(issues) != 0 {
		t.Fatalf("Parse reported issues: %+v", issues)
	}
	index, _ := corpus.Index("3.1")

	tr, issues := trace.Build(index)
	if len(issues) != 0 {
		t.Fatalf("Build reported issues: %+v", issues)
	}
	return tr
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

// TestRenderPromptCarriesOnlyTheTracedRules pins that the model is handed the
// rules it is allowed to cite and nothing else. A rule that is not in force must
// not appear in the prompt at all, so no citation to it can be manufactured.
func TestRenderPromptCarriesOnlyTheTracedRules(t *testing.T) {
	tr := versionTrace(t)
	prompt := procedure.RenderPrompt(tr)

	for _, want := range []string{"R1", "R2", "R3", "R4", "3.1", "IT-ACCEPTABLE-USE"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt does not mention %q", want)
		}
	}
	if strings.Contains(prompt, "R9") {
		t.Error("prompt mentions R9, which is not in force at the effective date")
	}
	if !strings.Contains(prompt, "Devices must be enrolled before use.") {
		t.Error("prompt does not carry the rule text the step has to be drafted from")
	}
}

// TestOutputSchemaAsksOnlyForStepsAndCitations pins that the model is allowed to
// produce prose and citations, and nothing that Go already decided.
func TestOutputSchemaAsksOnlyForStepsAndCitations(t *testing.T) {
	schema := procedure.OutputSchema()

	encoded, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}

	var decoded struct {
		Type       string                     `json:"type"`
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}

	if got, want := len(decoded.Properties), 1; got != want {
		t.Fatalf("schema asks for %d properties, want %d: the model may only return steps", got, want)
	}
	steps, ok := decoded.Properties["steps"]
	if !ok {
		t.Fatal("schema has no steps property")
	}
	if got, want := decoded.Required, []string{"steps"}; len(got) != len(want) || got[0] != want[0] {
		t.Errorf("required = %v, want %v", got, want)
	}

	// The steps property is an array schema, so the shape of one step lives in
	// its items schema, not on the array itself.
	var step struct {
		Items struct {
			Properties map[string]json.RawMessage `json:"properties"`
			Required   []string                   `json:"required"`
		} `json:"items"`
	}
	if err := json.Unmarshal(steps, &step); err != nil {
		t.Fatalf("unmarshal steps: %v", err)
	}
	if got, want := len(step.Items.Properties), 2; got != want {
		t.Fatalf("a step asks for %d properties, want %d: text and ruleIDs", got, want)
	}
	for _, name := range []string{"text", "ruleIDs"} {
		if _, ok := step.Items.Properties[name]; !ok {
			t.Errorf("a step has no %q property", name)
		}
	}
	if len(step.Items.Required) != 2 {
		t.Errorf("a step requires %v, want both text and ruleIDs", step.Items.Required)
	}
}

// TestInstructionStatesTheTraceBoundary pins the rules the instruction must
// carry: cite only the supplied ids, invent no rule, and execute nothing.
func TestInstructionStatesTheTraceBoundary(t *testing.T) {
	instruction := strings.ToLower(procedure.Instruction())

	for _, want := range []string{"ruleids", "cite", "never invent", "never execute", "draft"} {
		if !strings.Contains(instruction, want) {
			t.Errorf("instruction does not state %q", want)
		}
	}
}

// TestParseAnswerBuildsTheProcedureFromTheTrace pins that a valid answer becomes
// a Procedure carrying the rules Go already traced.
func TestParseAnswerBuildsTheProcedureFromTheTrace(t *testing.T) {
	tr := versionTrace(t)

	built, issues := procedure.ParseAnswer(tr, []byte(`{
      "steps": [
        {"text": "Enrol every device before it is used.", "ruleIDs": ["R2"]},
        {"text": "Confirm who owns the device.", "ruleIDs": ["R1", "R4"]}
      ]
    }`))
	if len(issues) != 0 {
		t.Fatalf("ParseAnswer rejected a valid answer: %+v", issues)
	}

	if got, want := built.PolicyID, "IT-ACCEPTABLE-USE"; got != want {
		t.Errorf("PolicyID = %q, want %q", got, want)
	}
	if got, want := built.Version, "3.1"; got != want {
		t.Errorf("Version = %q, want %q", got, want)
	}
	if got, want := len(built.Steps), 2; got != want {
		t.Fatalf("len(Steps) = %d, want %d", got, want)
	}
	if got, want := built.Steps[0].Text, "Enrol every device before it is used."; got != want {
		t.Errorf("Steps[0].Text = %q, want %q", got, want)
	}
	if got, want := len(built.Steps[0].Citations), 1; got != want {
		t.Fatalf("len(Steps[0].Citations) = %d, want %d", got, want)
	}
	if got, want := built.Steps[0].Citations[0].String(), "R2@3.1"; got != want {
		t.Errorf("Steps[0].Citations[0] = %q, want %q", got, want)
	}
	if got, want := len(built.Steps[1].Citations), 2; got != want {
		t.Errorf("len(Steps[1].Citations) = %d, want %d", got, want)
	}
}

// TestParseAnswerFillsTheRestFromTheTrace pins the separation this program
// exists for: decisions, exceptions, approvals and the citation index are Go's
// output, never the model's.
func TestParseAnswerFillsTheRestFromTheTrace(t *testing.T) {
	tr := versionTrace(t)

	built, issues := procedure.ParseAnswer(tr, []byte(`{
      "steps": [{"text": "Enrol every device.", "ruleIDs": ["R2"]}]
    }`))
	if len(issues) != 0 {
		t.Fatalf("ParseAnswer rejected a valid answer: %+v", issues)
	}

	if got, want := len(built.Decisions), 1; got != want {
		t.Fatalf("len(Decisions) = %d, want %d from the trace", got, want)
	}
	if got, want := built.Decisions[0].Field, "deviceOwner"; got != want {
		t.Errorf("Decisions[0].Field = %q, want %q", got, want)
	}
	if got, want := len(built.Decisions[0].Branches), 2; got != want {
		t.Errorf("len(Decisions[0].Branches) = %d, want %d", got, want)
	}

	if got, want := len(built.Exceptions), 1; got != want {
		t.Fatalf("len(Exceptions) = %d, want %d", got, want)
	}
	if got, want := built.Exceptions[0].RuleID, "R3"; got != want {
		t.Errorf("Exceptions[0].RuleID = %q, want %q", got, want)
	}
	if got, want := built.Exceptions[0].ExceptOf, "R2"; got != want {
		t.Errorf("Exceptions[0].ExceptOf = %q, want %q", got, want)
	}

	if got, want := len(built.Approvals), 1; got != want {
		t.Fatalf("len(Approvals) = %d, want %d", got, want)
	}
	if got, want := built.Approvals[0].Approver, "Head of IT Security"; got != want {
		t.Errorf("Approvals[0].Approver = %q, want %q", got, want)
	}
	if got, want := built.Approvals[0].Citation.String(), "R2@3.1"; got != want {
		t.Errorf("Approvals[0].Citation = %q, want %q", got, want)
	}

	if got, want := len(built.Citations), 1; got != want {
		t.Fatalf("len(Citations) = %d, want %d: the index carries the citations the steps used", got, want)
	}
	if got, want := built.Citations[0].Version, "3.1"; got != want {
		t.Errorf("Citations[0].Version = %q, want %q", got, want)
	}
}

// TestProcedureNeverLeavesDraft pins A8 and S12: nothing this program produces
// is approved, and no field claims a human signed it off.
func TestProcedureNeverLeavesDraft(t *testing.T) {
	tr := versionTrace(t)

	built, issues := procedure.ParseAnswer(tr, []byte(`{
      "steps": [{"text": "Enrol every device.", "ruleIDs": ["R2"]}]
    }`))
	if len(issues) != 0 {
		t.Fatalf("ParseAnswer rejected a valid answer: %+v", issues)
	}

	if got, want := built.Status, "draft"; got != want {
		t.Errorf("Status = %q, want %q", got, want)
	}
	if built.ApprovedBy != "" {
		t.Errorf("ApprovedBy = %q, want empty: only a human signs a procedure off", built.ApprovedBy)
	}
}

// TestParseAnswerRejectsAnUncitedStep pins A2: every step carries at least one
// citation or the whole answer is rejected.
func TestParseAnswerRejectsAnUncitedStep(t *testing.T) {
	tr := versionTrace(t)

	_, issues := procedure.ParseAnswer(tr, []byte(`{
      "steps": [
        {"text": "Enrol every device.", "ruleIDs": ["R2"]},
        {"text": "Be reasonable about it.", "ruleIDs": []}
      ]
    }`))

	uncited := withCode(issues, procedure.CodeUncitedStep)
	if len(uncited) != 1 {
		t.Fatalf("uncited_step issues = %+v, want exactly one", uncited)
	}
	if !strings.Contains(uncited[0].Message, "step 2") {
		t.Errorf("message %q does not name which step was uncited", uncited[0].Message)
	}
}

// TestParseAnswerRejectsACitationOutsideTheTrace pins A3 for the two ways a
// citation can be outside the trace: an id the version does not define, and an
// id that belongs to a sibling version.
func TestParseAnswerRejectsACitationOutsideTheTrace(t *testing.T) {
	tr := versionTrace(t)

	tests := []struct {
		name   string
		answer string
		ruleID string
	}{
		{
			name:   "unknown rule id",
			answer: `{"steps":[{"text":"Do the thing.","ruleIDs":["NOPE"]}]}`,
			ruleID: "NOPE",
		},
		{
			name:   "rule id from a sibling version",
			answer: `{"steps":[{"text":"Do the thing.","ruleIDs":["R2@3.0"]}]}`,
			ruleID: "R2@3.0",
		},
		{
			name:   "rule that is not in force",
			answer: `{"steps":[{"text":"Use the legacy VPN.","ruleIDs":["R9"]}]}`,
			ruleID: "R9",
		},
		{
			name:   "one good and one foreign citation",
			answer: `{"steps":[{"text":"Do the thing.","ruleIDs":["R2","NOPE"]}]}`,
			ruleID: "NOPE",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, issues := procedure.ParseAnswer(tr, []byte(test.answer))

			unknown := withCode(issues, procedure.CodeUnknownCitation)
			if len(unknown) != 1 {
				t.Fatalf("unknown_citation issues = %+v, want exactly one", unknown)
			}
			if got, want := unknown[0].RuleID, test.ruleID; got != want {
				t.Errorf("RuleID = %q, want %q", got, want)
			}
		})
	}
}

// TestParseAnswerAcceptsEveryTracedCitation is the control for A3: every id the
// trace produced must be citable, including the ones on an exception.
func TestParseAnswerAcceptsEveryTracedCitation(t *testing.T) {
	tr := versionTrace(t)

	for _, rule := range tr.Rules {
		id := rule.Rule.ID
		t.Run(id, func(t *testing.T) {
			_, issues := procedure.ParseAnswer(tr, []byte(`{"steps":[{"text":"Do the thing.","ruleIDs":["`+id+`"]}]}`))
			if len(issues) != 0 {
				t.Errorf("the trace produced %q but citing it was rejected: %+v", id, issues)
			}
		})
	}
}

// TestParseAnswerRejectsAnUnreadableAnswer pins the two shapes a malformed
// answer takes: something that is not JSON at all, and JSON with no steps.
func TestParseAnswerRejectsAnUnreadableAnswer(t *testing.T) {
	tr := versionTrace(t)

	tests := []struct {
		name   string
		answer string
	}{
		{name: "not JSON", answer: `I have written the procedure for you.`},
		{name: "truncated JSON", answer: `{"steps": [`},
		{name: "no steps", answer: `{}`},
		{name: "empty steps", answer: `{"steps": []}`},
		{name: "empty text", answer: `{"steps":[{"text":"","ruleIDs":["R2"]}]}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			built, issues := procedure.ParseAnswer(tr, []byte(test.answer))

			if len(withCode(issues, procedure.CodeUnreadableAnswer)) != 1 {
				t.Fatalf("unreadable_answer issues = %+v, want exactly one", issues)
			}
			if len(built.Steps) != 0 {
				t.Errorf("a rejected answer produced %d steps", len(built.Steps))
			}
			if got, want := built.Status, "draft"; got != want {
				t.Errorf("Status = %q on a rejected answer, want %q", got, want)
			}
		})
	}
}

// TestNewAgentRefusesToStartWithoutCredentials pins that the credential check
// happens before anything is created, so no test here can reach the network and
// no run without a key can fail halfway.
func TestNewAgentRefusesToStartWithoutCredentials(t *testing.T) {
	t.Setenv(procedure.APIKeyEnv, "")

	_, err := procedure.NewAgent(context.Background(), procedure.Config{})
	if err == nil {
		t.Fatal("NewAgent started with no credentials")
	}
	if !strings.Contains(err.Error(), procedure.APIKeyEnv) {
		t.Errorf("error %q does not name %s", err, procedure.APIKeyEnv)
	}
}

// TestModelSelectionHonoursTheEnvironment pins the documented override, without
// starting an agent.
func TestModelSelectionHonoursTheEnvironment(t *testing.T) {
	t.Setenv(procedure.ModelEnv, "gemini-custom")

	if got, want := procedure.ModelName(), "gemini-custom"; got != want {
		t.Errorf("ModelName() = %q, want %q", got, want)
	}

	t.Setenv(procedure.ModelEnv, "")
	if got, want := procedure.ModelName(), procedure.DefaultModel; got != want {
		t.Errorf("ModelName() = %q, want the documented default %q", got, want)
	}
}
