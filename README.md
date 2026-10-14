# enterprise-policy-procedures

**Nothing leaves the draft until every step can be traced to a policy rule and the version that
rule came from.** For policy owners and compliance teams in mid-size and large companies.

A policy owner hands this program a versioned policy. Go reads the version, decides which rules are
actually in force on the effective date, and refuses to continue when the document contradicts
itself. If the document survives that, one Google ADK (Go v2) agent drafts the steps — and every
step is accepted only if it cites a rule id that Go already traced. A step that cites anything else
rejects the entire answer.

```
                       ┌─────────────────────────────────────┐
 policies.json ──────▶ │ internal/policy                     │
 (every version)       │  contract, validation,              │
                       │  effective dates, version conflicts, │
                       │  approval gates                     │
                       └───────────────┬─────────────────────┘
                                       │ the selected version, read into facts
                       ┌───────────────▼─────────────────────┐
                       │ internal/trace                      │
                       │  citable rules, conditions →        │
                       │  branches and decisions,            │
                       │  missing_approval, bad exceptions   │
                       └───────────────┬─────────────────────┘
                                       │ trace only: no rule that is not in force
                       ┌───────────────▼─────────────────────┐
                       │ internal/procedure                  │
                       │  one ADK agent writes the steps,    │
                       │  every citation checked against     │
                       │  the trace                          │
                       └───────────────┬─────────────────────┘
                                       │
                     procedure.json (steps + decisions + exceptions +
                                     approvals + citations, status: draft)
```

## Requirements

- Go 1.26 or later (the module pins `go 1.26.6`, which ADK Go v2 requires).
- A Gemini API key in `GOOGLE_API_KEY` **only** for a live run. `-dry-run` and `-answer` need no
  credentials, and neither does the whole test suite.

## Quick start

```bash
# See what Go decided and the exact payload the model would receive. No key needed.
go run . -in testdata/policies_ok.json -version 3.1 -dry-run

# A live run needs a key; it writes a draft procedure to stdout.
export GOOGLE_API_KEY="your-key"
go run . -in testdata/policies_ok.json -version 3.1 -out procedure.json
```

## What Go decides, and what the model decides

This separation is the point of the program.

| Go decides, before any model call | The model decides |
| --- | --- |
| Which version is compiled, and which rules are in force on the effective date | The prose of each step |
| Whether two supplied versions disagree about a rule's text | — |
| Whether a rule that requires approval has a gate | — |
| Whether an exception names a rule, and branches the same condition field | — |
| The branches each condition declares, and the decisions they form | — |
| Every citation, and whether a step's citation is one it traced | — |
| `status` (`draft`) and `approvedBy` (empty) | — |

The model returns `steps[{text, ruleIDs}]` and nothing else. Because the rest of the procedure is
filled from the trace, a model that misunderstands the policy has one field to be wrong about: the
prose of a step.

## The rules the compiler enforces

Every rule below has a fixture in `testdata/` that triggers it, and a test that pins it. Nothing
here is left to the model's judgement.

| Code | When it is reported |
| --- | --- |
| `invalid_input` | A required field is missing, the JSON is not the documented shape, or one version defines a rule id twice. |
| `duplicate_rule_id` | One version defines the same rule id more than once. |
| `unknown_version` | `-version` names a version the document does not supply. |
| `invalid_date` | A date is not `YYYY-MM-DD`. The format is pinned rather than guessed. |
| `version_conflict` | Two supplied versions define one rule id with different text. Reported, never resolved: the compiler will not choose between them, so the policy owner says which text is approved. |
| `obsolete_rule` | A rule's window does not cover the effective date, whether because it closed before it or has not opened yet. |
| `orphan_approval` | An approval gate names a rule the version does not define, which would make a document look approved while the rule it meant to gate stays ungated. |
| `missing_approval` | A rule requires approval and no gate is attached. The compile stops; no step is drafted from a rule nobody is recorded as approving. |
| `dangling_exception` | An exception names a rule the version does not define in force. |
| `unrelated_exception` | An exception branches a different condition field than the rule it claims to except, so it narrows something else. |
| `invalid_condition` | A condition is not `field == value` or `field != value` with a single value and no spaces. Prose in a condition is reported, not passed on. |
| `uncited_step` | A drafted step cites no rule. |
| `unknown_citation` | A step cites a rule the trace did not produce: an id the version does not define, an id from a sibling version, or a rule that is not in force. |
| `unreadable_answer` | The model returned something that is not the documented shape, or returned no steps. |

**All blockers are reported together, in one pass.** A policy owner fixes one list instead of
rediscovering the next blocker after every fix.

**A blocked compile writes nothing.** No output file is created, and no procedure is printed. A
rejected answer behaves the same way: the deterministic half is returned for the error message, and
the steps are dropped, so nothing half-validated can be printed as a procedure.

## Command line

```
enterprise-policy-procedures -version <version> [options]

  -in <path>       policy document to read; omit or use - to read stdin
  -version <v>     version to compile; required when the document supplies more
                   than one, and the only version is used when it supplies one
  -dry-run         print the rule trace and the exact model payload, and call no
                   model; needs no credentials
  -answer <path>   replay a stored model answer through the validator instead of
                   calling the model
  -out <path>      write the procedure here; omit to write stdout
```

`ADK_MODEL` overrides the model, which defaults to `gemini-flash-latest`.

Exit codes: `0` the procedure was written, `1` the compile was blocked or the run failed and nothing
was written, `2` the command line was misused.

## Input contract

A document is one policy and every version of it that you want the compiler to see. More than one
version is normal: it is what makes a `version_conflict` detectable.

```json
{
  "policyID": "IT-ACCEPTABLE-USE",
  "effectiveDate": "2026-03-01",
  "versions": [
    {
      "version": "3.1",
      "sections": [
        {
          "id": "S2",
          "title": "Device enrolment",
          "rules": [
            {
              "id": "AU-2",
              "text": "Every device used to access company systems must be enrolled in device management before first use.",
              "owner": "IT Security",
              "effectiveFrom": "2025-01-01",
              "requiresApproval": true
            }
          ]
        }
      ],
      "approvals": [
        { "id": "AP-1", "ruleID": "AU-2", "approver": "Head of IT Security", "role": "IT Security" }
      ]
    }
  ]
}
```

A rule carries the required `id`, `text`, `owner` and `effectiveFrom`, plus the optional
`effectiveTo`, `condition`, `exceptionOf` and `requiresApproval`. Dates are always `YYYY-MM-DD`. See
[`testdata/README.md`](testdata/README.md) for a fixture-by-fixture guide.

## Tests

```bash
gofmt -l .
go vet ./...
go build ./...
go test ./... -count=1
```

**No test calls a model.** Every rule above is a pure function, so the suite runs with no API key.
The one path that needs credentials, `NewAgent`, is covered by a test proving it refuses to start
without them and creates nothing when it does.

**What is not verified:** the live Gemini call. No API key exists in the environment where this was
built, so no model call was ever executed and nothing in this repository claims a live result. The
`-dry-run` output is the reviewable surface of the prompt.

## Out of scope, deliberately

- **No policy management system.** It reads the document it is given; it does not store, search or
  retrieve policies, and it has no document-management backend.
- **No publication, no distribution, no workflow.** There is no publisher, no sender, no approval
  workflow and no adapter that could act on a procedure. Every procedure is `draft` with an empty
  `approvedBy`, and a named human approves it outside this program.
- **No execution.** It never runs a step, never calls a tool, and the agent is built with no tools
  bound, so it cannot read or write anything beyond the trace.
- **No free-form rule extraction.** Rules arrive structured, with ids. Pulling rules out of prose is
  a different and much harder problem, and doing it here would create the very ambiguity this
  program exists to refuse.
- **No standard-specific vocabulary.** SOX, ISO 27001 and the rest each need their own rule model.
- The read-only `check_policy` tool the source case anticipated is not here either: this version
  gives the model the trace up front instead of letting it ask for rules mid-answer.

## Layout

```
main.go                     the command: flags, the pipeline, exit codes
fixtures_test.go            one case per fixture, and the fixture index
main_test.go                the command line, its streams and its exit codes
internal/policy/            the contract, validation, effective dates,
                            version conflicts, approval gates
internal/trace/             citable rules, conditions, branches, decisions
internal/procedure/         the ADK agent, the prompt, the answer validator
testdata/                   seven policy fixtures and their index
odd/tasks/                  the ODD plan of record for this branch
```

## License

MIT. See [LICENSE](LICENSE).

The fixtures are synthetic and describe no real company, product or regulation. They are test data,
not policy advice.
