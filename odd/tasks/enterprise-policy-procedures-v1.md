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
| T0 | Bootstrap: isolated repository, MIT LICENSE, `.gitignore`, `go.mod` + ADK Go v2, CI workflow, this document | S1, S3, A9 | inline | done — see L7, L8 |
| T1 | Versioned policy contract + validation with RED tests (`internal/policy`) | A1, A5, S5, S6 | inline | pending |
| T2 | Rule parser, effective-date service and version-conflict detection (`internal/policy`) | A3, A5, S9, S13 | inline | pending |
| T3 | Rule trace + condition/branch validation as pure services (`internal/trace`) | A2, A4, A6, S20.3 | inline | pending |
| T4 | ADK agent: policy-grounded instruction, response schema, keyed session, citation validation (`internal/procedure`) | A2, A3, S16, S17 | inline | pending |
| T5 | CLI (`main.go`), fixtures (`testdata/`), README rewrite | A7, A8, A9, S4 | inline | pending |
| T6 | Independent verification of the candidate | A1-A9 | native assess decides | pending |
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
