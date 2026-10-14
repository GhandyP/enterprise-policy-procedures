// Package procedure drafts the prose of a procedure from the deterministic rule
// trace, and validates every citation the model returns against it.
//
// Only the step text and the rule ids come from the model. The decisions,
// exceptions, approval gates, citation index and draft marker are Go's output,
// so a model that misunderstands the policy has nothing to be wrong about except
// the prose.
package procedure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync/atomic"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/genai"

	"github.com/GhandyP/enterprise-policy-procedures/internal/policy"
	"github.com/GhandyP/enterprise-policy-procedures/internal/trace"
)

const (
	// DefaultModel is the Gemini model this program documents.
	DefaultModel = "gemini-flash-latest"
	// ModelEnv overrides DefaultModel.
	ModelEnv = "ADK_MODEL"
	// APIKeyEnv is the environment variable the model client reads.
	APIKeyEnv = "GOOGLE_API_KEY"
	// AppName identifies the ADK application that owns the sessions.
	AppName   = "enterprise-policy-procedures"
	agentName = "procedure_drafter"
	userID    = "enterprise-policy-procedures-cli"
)

// ErrMissingAPIKey reports a model call attempted without credentials.
var ErrMissingAPIKey = fmt.Errorf("%s is required to call the model", APIKeyEnv)

// ErrNoAgentOutput reports an agent turn that returned nothing at all.
var ErrNoAgentOutput = errors.New("the agent finished without an answer")

// instruction is the whole contract handed to the model.
const instruction = `You draft the steps of ONE procedure from a versioned policy, for the policy owner who has to review it.

Rules:
- Cite only the supplied ruleIDs. Every step needs at least one ruleIDs entry, copied exactly as given.
- Never invent a rule, a ruleID, a version, an owner, an approval, a date or a legal requirement.
  If the policy does not say it, it does not go in the procedure.
- Never resolve a decision or an exception yourself: the policy already declares them, and they are
  printed from the policy, not from your answer. You may refer to a rule's condition in words.
- Never execute a step, and never describe anything as done, sent, applied, published or enforced.
  You are drafting, not running the procedure.
- Write the procedure a person can follow: numbered in effect by order, one action per step, short
  plain sentences, no headings, no tables, no markdown, no commentary about yourself.
- The procedure stays a draft until a named human approves it. Never write that it was approved.
- If a rule cannot be turned into an actionable step, say what a human must settle in the step text.
`

// Instruction returns the agent instruction. Tests pin the trace boundary, the
// no-invention and no-execution rules, and the draft marker on it.
func Instruction() string { return instruction }

// OutputSchema describes the only thing the model may return: the steps and the
// rules each cites. Decisions, exceptions, approvals, the status and the
// approver are deliberately absent, because the program supplies them and the
// model therefore cannot get them wrong.
func OutputSchema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"steps": {
				Type:        genai.TypeArray,
				Description: "The steps of the procedure, in the order a person would follow them.",
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"text": {
							Type:        genai.TypeString,
							Description: "One actionable step, in plain sentences.",
						},
						"ruleIDs": {
							Type:        genai.TypeArray,
							Description: "Ids of the supplied rules this step came from, copied exactly. At least one.",
							Items:       &genai.Schema{Type: genai.TypeString},
						},
					},
					Required: []string{"text", "ruleIDs"},
				},
			},
		},
		Required: []string{"steps"},
	}
}

// ModelName reports the model this program will ask for: ADK_MODEL when it is
// set, and DefaultModel otherwise.
func ModelName() string {
	return firstNonEmpty(os.Getenv(ModelEnv), DefaultModel)
}

// Config selects the model and the credentials. Both default to the environment.
type Config struct {
	// ModelName defaults to ADK_MODEL and then to DefaultModel.
	ModelName string
	// APIKey defaults to APIKeyEnv.
	APIKey string
}

// Agent drafts one procedure from one rule trace.
type Agent struct {
	runner *runner.Runner
	runs   atomic.Uint64
}

// NewAgent builds the single-agent procedure drafter. It fails before any
// network call when no credentials are available, and it binds no tools, so the
// agent cannot read or write anything beyond the trace it is given.
func NewAgent(ctx context.Context, cfg Config) (*Agent, error) {
	modelName := firstNonEmpty(cfg.ModelName, os.Getenv(ModelEnv), DefaultModel)
	apiKey := firstNonEmpty(cfg.APIKey, os.Getenv(APIKeyEnv))
	if apiKey == "" {
		return nil, ErrMissingAPIKey
	}

	model, err := gemini.NewModel(ctx, modelName, &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		return nil, fmt.Errorf("create model %q: %w", modelName, err)
	}

	drafter, err := llmagent.New(llmagent.Config{
		Name:         agentName,
		Model:        model,
		Description:  "Drafts the steps of one procedure from a versioned policy, citing the rules it was given.",
		Instruction:  Instruction(),
		OutputSchema: OutputSchema(),
	})
	if err != nil {
		return nil, fmt.Errorf("create agent: %w", err)
	}

	run, err := runner.NewInMemory(AppName, drafter)
	if err != nil {
		return nil, fmt.Errorf("create runner: %w", err)
	}

	return &Agent{runner: run}, nil
}

// Draft renders the prompt from the trace, asks the model for the steps, and
// validates every citation against the same trace.
//
// A non-empty issue list means the answer was rejected and the returned
// procedure carries only its deterministic half, with no steps. A non-nil error
// means the call itself could not be made or completed.
func (a *Agent) Draft(ctx context.Context, tr trace.Trace) (Procedure, []policy.Issue, error) {
	// One session per run, keyed by the policy version, keeps one version's
	// answer out of the next version's prompt.
	sessionID := fmt.Sprintf("%s-%d", tr.Version, a.runs.Add(1))
	prompt := genai.NewContentFromText(RenderPrompt(tr), genai.RoleUser)

	var raw string
	for event, err := range a.runner.Run(ctx, userID, sessionID, prompt, agent.RunConfig{}) {
		if err != nil {
			return Procedure{}, nil, fmt.Errorf("run agent: %w", err)
		}
		if event == nil {
			continue
		}
		// ADK stores the value parsed against OutputSchema on the event. Read
		// it wherever it appears and keep the last one, so a differently marked
		// final event cannot make the answer disappear.
		if event.Output != nil {
			encoded, err := json.Marshal(event.Output)
			if err != nil {
				return Procedure{}, nil, fmt.Errorf("encode agent output: %w", err)
			}
			raw = string(encoded)
		}
	}

	if raw == "" {
		return Procedure{}, nil, ErrNoAgentOutput
	}

	built, issues := ParseAnswer(tr, []byte(raw))
	return built, issues, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
