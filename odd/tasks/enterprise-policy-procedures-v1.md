# enterprise-policy-procedures — v1 rule-traceable procedures from a versioned policy

Public Go project, built from scratch: one Google ADK (Go v2) agent turns a supplied, versioned
enterprise policy into the prose of a procedure whose steps exist only because Go matched them to a
rule of an approved policy version. A step that cites no rule, or cites a rule that is unknown,
obsolete at the effective date, or contradicted by another supplied version, rejects the whole
answer. Policy storage systems, workflow engines and procedure publication stay out of v1.

Repo: `github.com/GhandyP/enterprise-policy-procedures` (public, MIT). Branch: `feature/enterprise-policy-procedures`.
Delivery: `chain_strategy=feature-branch-chain` — work-unit commits on the feature branch, one PR to `main` at slice close (push and PR require the user's explicit OK).
Audience: policy owners and compliance teams in mid-size and large companies (S2).
Test runner: `go test ./... -count=1` (plus `gofmt -l .`, `go vet ./...`, `go build ./...`).

## Specs

S1 — user (verbatim): "1 el reconocimiento y explorar , crear el repo de github con git y gh cli , crear un plan odd para crear
 desde cero en este repo el proyecto completo y lista para mi portafolio en github"

S2 — user (verbatim, audience statement given as the answer to the repository-name question): "Es para empresas en general y que sea ayuda alos encargados de roles de de politicas en las empresas medianas y grandes"

S3 — user decisions (verbatim labels): scope "Caso 16 standalone (recomendado)"; repository name "enterprise-policy-procedures"; v1 "Núcleo determinista en Go + citas (recomendado)"; visibility "Público (recomendado, como tus otros 11)".

S4 — README "Goal" (verbatim): "Draft a clear procedure from supplied policy text while preserving rules, exceptions, approvals, and version information."

S5 — README "Missing elements" → Contract (verbatim): "**Contract:** Define `PolicyInput{policyID, version, sections[]}` -> `Procedure{steps[], decisions[], exceptions[], approvals[], citations[]}`."

S6 — README "Missing elements" → Domain boundary (verbatim): "**Domain boundary:** Add versioned policy retrieval, rule parser, effective-date service, and procedure repository adapters."

S7 — README "Missing elements" → Fixtures and validation (verbatim): "**Fixtures and validation:** Cover conflicting versions, exceptions, missing approvals, conditional branches, and obsolete rules with rule-trace tests."

S8 — README "Missing elements" → Safety (verbatim): "**Safety:** Never execute a procedure from generated text; require policy-owner review before publishing or using it for compliance decisions."

S9 — README "Done when" (verbatim): "Each procedure step cites the applicable rule and version."

S10 — README "Done when" (verbatim): "Exceptions and approval gates are preserved and validated."

S11 — README "Done when" (verbatim): "Fixtures cover branching and conflicting policy versions."

S12 — README "Done when" (verbatim): "No generated procedure triggers an operational action without human sign-off."

S13 — README "Next improvement" (verbatim): "Build a versioned policy fixture and make every generated step carry a rule citation."

S14 — README "What exists now" (verbatim): "The output is untyped prose; rule IDs, conditions, exceptions, approvals, and citations are not checked."

S15 — README "What exists now" (verbatim): "This is intentionally text-only: it does not retrieve an approved policy version or execute a procedure."

S16 — README "Missing elements" → ADK layer (verbatim): "**ADK layer:** Add a policy-grounded instruction, response schema, and session keyed by policy version; a future `check_policy` tool must be read-only."

S17 — README sample prompt (verbatim): "Turn this policy excerpt into a numbered procedure with exceptions."

S18 — README "Target structure" (verbatim): "├── schema.go (next)\n├── procedure.go (next)\n├── adapters/policy_store.go (next)\n├── testdata/policies.json (next)\n└── main_test.go (next)"

S19 — README "Run the current prototype" (verbatim): "`internal/adkhelper.Run` uses ADK Go v2.2.0, Gemini, and `gemini-flash-latest`; set `ADK_MODEL` to override the model."

S20 — README "Implementation order" (verbatim): "1. Model policy versions, rules, exceptions, approvals, and citations.\n2. Add version-aware retrieval and schema-constrained procedure drafting.\n3. Implement branch/condition validation as a pure service.\n4. Test obsolete versions, conflicts, missing approvals, and exceptions.\n5. Add reviewed publication only after the dry-run procedure is traceable."

**Assumptions** (marked, not user requirements): module path `github.com/GhandyP/enterprise-policy-procedures`; dependencies `google.golang.org/adk/v2 v2.6.0` and `google.golang.org/genai v1.74.0`, pinning the newest ADK Go v2 release exactly as `executive-report-agent` does, while S19 documents v2.2.0 inside the cookbook; model `gemini-flash-latest` with `ADK_MODEL` override and `GOOGLE_API_KEY` credentials; the input contract is read as JSON from a file or stdin; S6 is read as "retrieve the supplied, identifiable policy version", not as a document-management backend; S20 step 5 (reviewed publication) is deferred, since S12 only forbids publishing without sign-off and no publication feature is requested; the procedure is always `status: "draft"` with an empty `approvedBy`, because S8 requires policy-owner review before publication.

### Acceptance criteria (v1)

- A1 (S5, S6): the input contract accepts `policyID`, a policy `version` and `sections[]`, and produces `Procedure{steps[], decisions[], exceptions[], approvals[], citations[]}`.
- A2 (S9, S13): every step in an accepted procedure carries at least one citation naming a rule id and the policy version; a step caught without a citation rejects the answer.
- A3 (S9, S14, S15): the only rule ids the model may cite are the ones the deterministic rule trace produced from the selected version; a citation to an unknown id, to an id owned by another version, or to a rule obsolete at the effective date rejects the answer.
- A4 (S12, S13): a rule marked as requiring approval, with no approval gate attached to it, blocks the compile and no procedure is printed.
- A5 (S11, S13): two supplied versions that define the same rule id with different text, or a rule that is obsolete at the effective date, are reported as conflicts that block the compile.
- A6 (S10, S20.3): exceptions and approval gates survive into the procedure and are validated; condition and branch validation is a pure function with its own tests.
- A7 (S11): fixtures cover conflicting versions, exceptions, missing approvals, conditional branches and obsolete rules, and each case is pinned by a test.
- A8 (S8, S12): the program never executes a procedure step, prints every procedure as `draft`, and offers no publisher, distributor or workflow adapter.
- A9 (S3): the code is a standalone Go module in this repository with LICENSE, CI, README and this ODD document.

**Non-goals (v1)**: the read-only `check_policy` tool that S16 lists as future work; a policy management system, document store or retrieval backend (S6 is reduced to the supplied document); the procedure repository adapter and the reviewed publication of S20 step 5; standard-specific rule vocabularies (SOX, ISO 27001); multi-agent graphs; PDF/DOCX rendering; execution of an approval workflow beyond the draft marker and the approver field; natural-language rule extraction from free-form prose (rules arrive structured, with ids).

## Tasks

| ID | Task | Specs | Route | Status / evidence |
| --- | --- | --- | --- | --- |
| T0 | Bootstrap: isolated repository, MIT LICENSE, `.gitignore`, `go.mod` + ADK Go v2, CI workflow, this document | S1, S3, A9 | inline | done — `6658d3a`, `22ed537`; CI `38488173848` `success`, L7-L9 |
| T1 | Versioned policy contract + validation with RED tests (`internal/policy`) | A1, A5, S5, S6 | inline | done — RED `no non-test Go files in internal/policy` (exit 1), GREEN 17 tests, `528a31e`, L11 |
| T2 | Rule parser, effective-date service and version-conflict detection (`internal/policy`) | A3, A5, S9, S13 | inline | done — RED undefined `Index`, `RuleFact`, `CodeObsoleteRule`, `CodeVersionConflict`, `CodeInvalidDate`, `CodeOrphanApproval`; GREEN 34 tests, `845f501`, L14 |
| T3 | Rule trace + condition/branch validation as pure services (`internal/trace`) | A2, A4, A6, S20.3 | inline | done — RED `no non-test Go files in internal/trace` (exit 1), GREEN 17 tests, `4b406f6`, L15 |
| T4 | ADK agent: policy-grounded instruction, response schema, keyed session, citation validation (`internal/procedure`) | A2, A3, S16, S17 | inline | done — RED `no non-test Go files in internal/procedure` (exit 1), GREEN 25 tests, `16aefb1`, L18 |
| T5 | CLI (`main.go`), fixtures (`testdata/`), README rewrite | A7, A8, A9, S4 | inline | done — RED undefined `run`, `exitOK`, `exitFailure`, `APIKeyEnv`; GREEN 21 tests in the root package and 97 in the module, `1b7629f`, L19 |
| T6 | Verification of the candidate | A1-A9 | native assess decides | done — native assess over `22ed537..1b7629f`: `medium`, 25 paths / 4358 changed lines, `reviewDue false` (`under_budget`); plan: writer self-verification stands, no separate verifier. Verdict per criterion in L20 |
| T7 | Delivery: push the feature branch and open the PR to `main` | S3 | after explicit user OK | pending |

## Log

L1 — user (verbatim): "1 el reconocimiento y explorar , crear el repo de github con git y gh cli , crear un plan odd para crear
 desde cero en este repo el proyecto completo y lista para mi portafolio en github"

L2 — user decisions (verbatim labels): scope "Caso 16 standalone (recomendado)"; v1 "Núcleo determinista en Go + citas (recomendado)"; visibility "Público (recomendado, como tus otros 11)". Repository name chosen by the user from four proposed options: "enterprise-policy-procedures".

L3 — user (verbatim, audience statement, delivered where a repository name was expected): "Es para empresas en general y que sea ayuda alos encargados de roles de de politicas en las empresas medianas y grandes". Effect: the audience is fixed as policy owners and compliance teams in mid-size and large companies (S2), and it is what the README headline, the repository description and the topics address. It does not change the technical scope of S4-S20.

L4 — environment evidence (reconnaissance, read-only). This directory had no local `.git`; `git rev-parse --show-toplevel` resolved to `/home/ghandy`, the home directory itself, whose index already stages unrelated files from `~/Desktop` and `~/odd/tasks`. Every Git command issued here before T0 therefore addressed the wrong repository boundary. Fix: an isolated `git init -b main` in this directory, verified afterwards (`--show-toplevel` and `--git-dir` both resolve here); `/home/ghandy/.git` was left untouched and none of its staged files were modified. `gh` is authenticated as `GhandyP` with `repo` and `workflow` scopes. `gh repo create GhandyP/enterprise-policy-procedures --public` created the empty public repository, and its description and topics were set with `gh repo edit`. No repository with a policy, procedure or compliance name existed before this one (87 repositories inspected).

L5 — convention evidence. The sibling portfolio repositories `executive-report-agent`, `source-grounded-faq-generation` and `adk-meeting-summary` establish the house pattern followed here: MIT LICENSE, `.gitignore`, `go.mod` with `adk/v2`, validation concentrated in pure Go packages under `internal/` with `testdata/` fixtures, `odd/tasks/<feature>.md`, and a CI workflow running `gofmt -l`, `go vet`, `go build` and `go test`. In those repositories only the model call requires credentials, which is why every rule of this feature is designed as a pure function that `go test` can exercise with no API key.

L6 — decision: the semantics S5-S13 leave open are fixed here, all deterministic and testable. (1) A rule is the unit of trace: it carries `id`, `text`, `owner`, `effectiveFrom`, optional `effectiveTo`, optional `condition` and optional `exceptionOf`. (2) Two supplied versions that give the same rule id different text is a `version_conflict`: not a merge, and not a silent last-writer-wins. (3) A rule whose `effectiveTo` precedes the compile's effective date is `obsolete_rule` and may not be cited. (4) A rule carrying `requiresApproval: true` with no approval gate in the selected version is `missing_approval` and blocks the compile. (5) The model receives only the rule ids that survived validation and may cite only those; a citation outside that set rejects the entire answer — the same membership rule that `executive-report-agent` applies to numbers. (6) `-dry-run` prints the deterministic trace and the exact model payload without credentials. (7) Every conflict, obsolete rule and missing approval is reported together in one compile report, so a policy owner fixes one list instead of discovering blockers one at a time.

L7 — decision and evidence: the cookbook stub `main.go` is not carried into this module, because it imports `github.com/GhandyP/adk-go-enterprise-cookbook/internal/adkhelper`, a package that exists only inside the cookbook, so `go build ./...` failed with `no required module provides package .../internal/adkhelper`. Recorded verbatim for provenance:

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/GhandyP/adk-go-enterprise-cookbook/internal/adkhelper"
)

func main() {
	if err := adkhelper.Run(
		context.Background(),
		os.Args[1:],
		"case_16_policies_and_procedures",
		"A text-only starting point for policies and procedures.",
		"Draft a clear procedure from policy text supplied by the user. Preserve rules and exceptions, identify owners or missing owners, and do not invent policy.",
	); err != nil {
		log.Fatal(err)
	}
}
```

The cookbook `README.md` stays in the tree as the requirements source that S4-S20 quote, and T5 replaces it with the portfolio README; the authoritative copy of the cookbook text remains `/home/ghandy/Documents/P1_proyecticos/adk-go-enterprise-cookbook`.

L8 — bootstrap evidence (T0). `git init -b main` created the isolated repository and `origin` points at `https://github.com/GhandyP/enterprise-policy-procedures.git`. `go mod init github.com/GhandyP/enterprise-policy-procedures` plus `go get google.golang.org/adk/v2@v2.6.0 google.golang.org/genai@v1.74.0` resolved from `proxy.golang.org` and produced `go.mod` with `go 1.26.6` — the local toolchain is `go1.26.4` with `GOTOOLCHAIN=auto`, so the 1.26.6 toolchain is fetched from the module proxy. Observed state of the bootstrap commit: `gofmt -l .` prints nothing, `go build ./...` exits 0, `go vet ./...` exits 1 with `no packages to vet` and `go test ./... -count=1` exits 1 with `no packages to test`. Those last two are the expected behaviour of a module that has no Go packages yet, and they are exactly what broke CI in `executive-report-agent` (its L19): the workflow therefore carries the `if: ${{ hashFiles('**/*.go') != '' }}` guard on `go vet` and `go test` from the first commit, while `gofmt` and `go build` run unconditionally.

L9 — CI and delivery evidence (T0 close). Pushing `6658d3a` to `main` triggered run `38488173848`, completed `success` in 15s (`test`, job `115606773956`): `gofmt` and `go build` ran unconditionally and passed, while `go vet` and `go test` were skipped by the `hashFiles` guard exactly as designed — the same module state that failed CI on `executive-report-agent`'s bootstrap commit (its L19) is green here on the first push, because the guard was copied from the start instead of after the failure. `actions/setup-go` resolved `go 1.26.6` from `go.mod`. The single annotation is informational: the `ubuntu-latest` label migrates to Ubuntu 26 from 2026-10-19, the same annotation `executive-report-agent` records, needing no action now. Remote state after T0: `refs/heads/main` = `6658d3a` and no other branch; the repository is `PUBLIC`, license `mit`, default branch `main`, description set, topics `adk, compliance, enterprise, gemini, golang, governance, policy-management`. The feature branch is deliberately not created at T0: it would be identical to `main` until T1 produces the first work unit, and a branch with no diff cannot become the PR the delivery step promises.

L10 — ENGRAM MIRROR PENDING: the local document is authoritative and current. `mem_save` for this feature timed out twice against `http://127.0.0.1:7437` (15s limit), and `mem_search` and `mem_doctor` then failed the same way with `the server accepted the connection but did not respond`, so no claim is made that either write landed and neither was retried blindly, to avoid duplicating a write that may already have been applied. This blocks no check: the document above is complete and committed, and the mirror must be synchronized once the Engram server answers again.

L11 — evidence (T1): commit `528a31e` on `feature/enterprise-policy-procedures`, the first work unit on this branch. RED observed as the house pattern does it: `github.com/GhandyP/enterprise-policy-procedures/internal/policy: no non-test Go files` with exit 1. GREEN as `ok ... internal/policy` with 17 tests, `gofmt -l internal/` printing nothing, and `go vet ./...` and `go build ./...` exiting 0. The tests live in the external `policy_test` package on purpose, so they can only reach the exported API. Two design points landed with it: `PolicyInput` keeps S5's three fields exactly and `Select` returns the sections of the selected version rather than the union of every supplied version — without that, a citation to a rule owned by a sibling version would be undetectable, and A3 depends on detecting it — and `Parse` reports every issue in one pass, each carrying the `path` that locates it, instead of stopping at the first defect.

L12 — gap found while implementing T1, closed by decision. A4 requires that a rule marked as requiring approval, with no approval gate attached, blocks the compile, and S5's `Procedure` carries `approvals[]`; but the contract committed in T1 has no place in the *input* for an approval gate, so A4 had nothing to read. Decision: a gate is attached to the version it belongs to, and its shape is frozen here so the fixture work can start against it without waiting for the Go type:

```json
"approvals": [
  { "id": "A1", "ruleID": "R2", "approver": "Name or role", "role": "IT Security" }
]
```

So `VersionedPolicy` gains `approvals[]`, each gate naming the `ruleID` it gates, the `approver` and the `role`. T2 adds the type and its validation; A4 is implemented in T3. The omission is recorded rather than rewritten, because T1's commit is already the reviewable unit for S5's shape and no requirement changed.

L13 — ENGRAM MIRROR STILL PENDING, re-checked when the listener answered again: `http://127.0.0.1:7437/` returns 404 quickly, but the API path still hangs, so `mem_search` timed out again with `the server accepted the connection but did not respond` after the earlier double `mem_save` timeout. The write state of those first two attempts remains unknown, so neither was retried blindly. The repository document stays authoritative and complete.

L14 — evidence (T2): commit `845f501`. RED observed as undefined `Index`, `RuleFact`, `CodeObsoleteRule`, `CodeVersionConflict`, `CodeInvalidDate` and `CodeOrphanApproval` (exit 1); GREEN as 34 tests in `internal/policy`, with `gofmt -l internal/` printing nothing and `go vet ./...` and `go build ./...` exiting 0. Two design points landed. (1) `Index` reads approval gates from `c.Versions[i]` rather than from `PolicyInput`, so the contract keeps exactly the three fields S5 names instead of widening to carry them. (2) A rule's window is judged in both directions: a rule whose `effectiveTo` has passed is `obsolete_rule` per A5, and so is a rule whose `effectiveFrom` has not arrived, which A3's "may not be cited" already implies. Judging only the closing edge would have left a not-yet-effective rule citable. One test defect was mine, not the code's: the T1 helper `mustParse` asserts a document is structurally clean, which is right for T1 and wrong as a precondition for a conflict fixture.

L15 — evidence (T3): commit `4b406f6`. RED observed as `no non-test Go files in internal/trace` (exit 1); GREEN as 17 tests in the package, 11 of them top level, with `gofmt`, `go vet ./...` and `go build ./...` clean. `Build` folds the index into the trace and reports every blocker in one pass: `missing_approval`, `dangling_exception`, `unrelated_exception` and `invalid_condition`. A rule that is not in force is never traced, so A3's obsolete case is closed by construction rather than by a check on the answer. The condition grammar is pinned in code — identifier, `==` or `!=`, and one value with no whitespace — so prose in a condition is reported instead of travelling on as a branch. One test defect was mine again: the `build` helper originally required `Index` to report nothing, which is wrong for a fixture that deliberately carries a retired rule; the helper now asserts `Parse` and `Build` and leaves the index's own reporting to the package that owns it. **Correction recorded**: the first version of this commit's message claimed 25 tests in the package; the real count is 17, and the message was amended to the counted number before the branch was pushed. No count in this document is estimated.

L16 — SUBAGENT DELEGATION UNAVAILABLE, recorded instead of retried. `gentle-ai-worker` was launched twice for the `testdata/` fixtures and rejected both times with `Writer session Git authority changed before admission`; the second attempt was made only after committing the pending document so the tree was clean, and it failed identically, so the refusal is not a dirty-tree race. `gentle-ai-explore` was then launched read-only to map the ADK Go v2 surface needed by T4 and died with `process cleanup unconfirmed after 1000ms; capacity quarantined`. Conclusion: neither a delegated writer nor a delegated explorer is available in this session, and a third attempt would be ceremony rather than engineering. Consequences, both stated rather than hidden: the fixtures return to T5, which is where this plan put them before the parallel split was attempted, and T4's ADK pattern was read inline from `executive-report-agent` instead of from a handoff. No parallel writer ever ran, so no two writers touched overlapping surfaces at any point.

L17 — decision, T4's separation: the model returns only `steps[{text, ruleIDs}]`. Decisions, exceptions, approval gates, the citation index, `status` and `approvedBy` are Go's output, derived from the trace the compiler already validated, so a model that misunderstands the policy has no field to be wrong about except the prose of a step. This is the strongest available reading of A2 together with A8 and S12, and it makes S5's `decisions[]`, `exceptions[]` and `approvals[]` true by construction instead of by validation. `initial go.sum` did not satisfy the ADK import graph: the first build of `internal/procedure` failed with missing go.sum entries for `github.com/google/uuid`, `github.com/google/jsonschema-go/jsonschema`, `github.com/google/safehtml/template`, `rsc.io/omap` and `rsc.io/ordered`, the same five modules `executive-report-agent` records in its L12, resolved the same way with `go mod tidy`.

L18 — evidence (T4): commit `16aefb1`. RED observed as `no non-test Go files in internal/procedure` (exit 1). Two defects were mine and both were fixed without weakening a rule: `Draft` returned two values where its signature promised three, and the schema test looked for a step's properties on the array schema instead of on its `items`. GREEN as 25 tests in the package and 76 across the module. `NewAgent` checks the credentials before it creates anything, which is what lets a test prove the no-key path without reaching the network, and the ADK session is keyed by the policy version, which is what S16 asks for.

L19 — evidence (T5): commit `1b7629f`. RED observed as undefined `run`, `exitOK`, `exitFailure` and `APIKeyEnv` (exit 1); GREEN as 21 tests in the root package and 97 across the module, with `gofmt -l .` printing nothing and `go vet ./...` and `go build ./...` exiting 0. Seven fixtures landed under `testdata/` with `testdata/README.md` indexing the rule each one pins, and `TestFixtureSetIsExactlyTheDocumentedSeven` holds the index and the files to each other in both directions. One real defect surfaced here and was fixed rather than shipped: `trace.Citation` and `trace.Branch` carried no JSON tags, so the emitted procedure mixed `"ruleID"` with `"RuleID"` and `"field"` with `"Field"` — an inconsistent contract for exactly the file a consumer reads. The assertion that caught it now pins the object form of a citation instead of the `R2@3.1` form, which appears in prose and never in JSON. Functional evidence, every command with `GOOGLE_API_KEY` unset: the dry run on `policies_ok.json` exits 0 and traces AU-1 to AU-4 with the AU-3 exception and the AU-2 gate; `version_conflict`, `obsolete_rule`, `missing_approval`, `invalid_input` with `duplicate_rule_id`, and `unknown_citation` each exit 1 with their code named in stderr and no output file created; a live run with no key exits 1 naming `GOOGLE_API_KEY` and creates no file; the stored-answer replay exits 0 and writes a procedure whose `status` is `draft` with an empty `approvedBy`.

L20 — verification record (T6). Native `assess` over the committed range `22ed537..1b7629f` returned `risk: medium`, `nativeRisk: medium`, 25 changed paths and 4358 changed lines, `reviewDue: false` with reason `under_budget`, `outcome_source: unknown`, writer profile `large` from `runtime`, and the plan `{writerSelfVerification: true, structuralReadbackOnly: false, independentVerifier: false}`: the writer's self-verification stands and no separate verifier is required, which is the risk-gated path for a medium tier under RDD with an unknown native review outcome. No escalation was requested, because no Task Size high-risk item is touched: the program writes only the file it is asked for, reads `GOOGLE_API_KEY` and nothing else, has no consumer of its contract yet, has no concurrency, and every rule it implements has a test. Verdict per acceptance criterion, each from an observed command on the frozen candidate `1b7629f` with a clean tree. **A1** verified: the contract tests in `internal/policy` and the replay run emitting all five of S5's arrays. **A2** verified: `TestParseAnswerRejectsAnUncitedStep`, whose message names `step 2`, and the emitted `"ruleID": "AU-2"` beside `"version": "3.1"`. **A3** verified: `TestParseAnswerRejectsACitationOutsideTheTrace` over an unknown id, a sibling-version id, a rule that is not in force and a mixed step, plus the functional run where citing `RA-2` exits 1 with `unknown_citation` and creates no file. **A4** verified: `TestBuildBlocksARuleNeedingApprovalWithNoGate` and the `missing_approval` blocker on `CE-1`. **A5** verified: `TestIndexReportsAVersionConflict` and `TestIndexMarksARuleObsoleteAtTheEffectiveDate`, and both fixtures exiting 1 with their codes. **A6** verified: the `internal/trace` tests for dangling and unrelated exceptions, parsed branches, grouped decisions and the six rejected condition forms, plus the dry run showing `exception to: DR-2` and `approval gate: Data Protection Officer`. **A7** verified: seven fixtures, `TestFixturesBehaveAsDocumented` running all seven, and the index held to the files in both directions. **A8** verified structurally as well as by test: zero occurrences of `os/exec`, `net/http` or `os.Remove` anywhere in the module, the only `os.WriteFile` in the program is the `-out` write in `main.go`, `StatusDraft` is the only status the code constructs, and every failure path is observed to create no file. **A9** verified: LICENSE, `.gitignore`, CI workflow, README, `testdata/README.md` and this document are all present, and the module is standalone at `github.com/GhandyP/enterprise-policy-procedures`. **Not verified, and stated rather than claimed**: the live Gemini call. No API key exists in this environment, so no model call was ever executed; every rule above is pinned by a pure function and the `-dry-run` output is the reviewable surface of the prompt. Also not verified: whether a real policy owner finds the blocker messages useful, which no test can answer.
