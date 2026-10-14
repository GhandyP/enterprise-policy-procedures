# 16 — Policies and procedures

## Goal

Draft a clear procedure from supplied policy text while preserving rules, exceptions, approvals, and version information.

## Run the current prototype

From this case directory:

```bash
export GOOGLE_API_KEY="your-key"
go run .
```

`internal/adkhelper.Run` uses ADK Go v2.2.0, Gemini, and `gemini-flash-latest`; set `ADK_MODEL` to override the model.

Sample prompt:

```text
Turn this policy excerpt into a numbered procedure with exceptions.
```

## What exists now

- `main.go (exists)` sends policy text to one focused Gemini ADK agent.
- The output is untyped prose; rule IDs, conditions, exceptions, approvals, and citations are not checked.
- This is intentionally text-only: it does not retrieve an approved policy version or execute a procedure.

## Missing elements

- **Contract:** Define `PolicyInput{policyID, version, sections[]}` -> `Procedure{steps[], decisions[], exceptions[], approvals[], citations[]}`.
- **ADK layer:** Add a policy-grounded instruction, response schema, and session keyed by policy version; a future `check_policy` tool must be read-only.
- **Domain boundary:** Add versioned policy retrieval, rule parser, effective-date service, and procedure repository adapters.
- **Fixtures and validation:** Cover conflicting versions, exceptions, missing approvals, conditional branches, and obsolete rules with rule-trace tests.
- **Safety:** Never execute a procedure from generated text; require policy-owner review before publishing or using it for compliance decisions.

## Target structure

```text
16-policies-and-procedures/
├── main.go (exists)
├── README.md (exists)
├── schema.go (next)
├── procedure.go (next)
├── adapters/policy_store.go (next)
├── testdata/policies.json (next)
└── main_test.go (next)
```

## Implementation order

1. Model policy versions, rules, exceptions, approvals, and citations.
2. Add version-aware retrieval and schema-constrained procedure drafting.
3. Implement branch/condition validation as a pure service.
4. Test obsolete versions, conflicts, missing approvals, and exceptions.
5. Add reviewed publication only after the dry-run procedure is traceable.

## Done when

- [ ] Each procedure step cites the applicable rule and version.
- [ ] Exceptions and approval gates are preserved and validated.
- [ ] Fixtures cover branching and conflicting policy versions.
- [ ] No generated procedure triggers an operational action without human sign-off.

## Next improvement

Build a versioned policy fixture and make every generated step carry a rule citation.
