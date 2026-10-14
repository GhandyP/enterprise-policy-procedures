// Command enterprise-policy-procedures compiles one version of a supplied
// policy into a procedure whose every step cites the rule and version it came
// from, or refuses to produce anything and names why.
//
// The rules, the branches, the exceptions, the approval gates, the citations
// and the draft marker are decided in Go. The model writes the prose of the
// steps and nothing else.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/GhandyP/enterprise-policy-procedures/internal/policy"
	"github.com/GhandyP/enterprise-policy-procedures/internal/procedure"
	"github.com/GhandyP/enterprise-policy-procedures/internal/trace"
)

// Exit codes. A blocked compile is a failure, because the program produced no
// procedure; a misuse of the command line is a usage error.
const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

// APIKeyEnv is the environment variable a live run needs, re-exported so the
// help text and the tests name it from one place.
const APIKeyEnv = procedure.APIKeyEnv

const usage = `enterprise-policy-procedures compiles one version of a policy into a procedure
whose every step cites the rule and the version it came from.

usage:
  enterprise-policy-procedures -version <version> [options]

options:
  -in <path>       policy document to read; omit or use - to read stdin
  -version <v>     version to compile; required when the document supplies more
                   than one, and the only version is used when it supplies one
  -dry-run         print the rule trace and the exact model payload, and call
                   no model; needs no credentials
  -answer <path>   replay a stored model answer through the validator instead
                   of calling the model
  -out <path>      write the procedure here; omit to write stdout

environment:
  ` + APIKeyEnv + `   required only for a live run
  ` + procedure.ModelEnv + `        overrides the model, default ` + procedure.DefaultModel + `

exit codes:
  0  the procedure was written
  1  the compile was blocked or the run failed; nothing was written
  2  the command line was misused

Nothing this command writes is ever approved: every procedure is a draft for a
policy owner to review, and there is no publisher, distributor or workflow
adapter anywhere in this program.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run is the whole program, with its streams passed in, so every path can be
// exercised by a test with no credentials and no process to spawn.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("enterprise-policy-procedures", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { fmt.Fprint(stderr, usage) }

	var (
		inPath     = flags.String("in", "", "policy document to read; omit to read stdin")
		version    = flags.String("version", "", "version to compile")
		outPath    = flags.String("out", "", "where to write the procedure; omit to write stdout")
		dryRun     = flags.Bool("dry-run", false, "print the rule trace and the model payload without calling the model")
		answerPath = flags.String("answer", "", "replay a stored model answer through the validator")
	)

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument %q\n\n", flags.Arg(0))
		fmt.Fprint(stderr, usage)
		return exitUsage
	}

	document, err := readInput(*inPath, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "read the policy document: %v\n", err)
		return exitFailure
	}

	corpus, issues := policy.Parse(document)
	if len(issues) != 0 {
		report(stderr, issues)
		return exitFailure
	}

	selected, ok := chooseVersion(corpus, *version)
	if !ok {
		fmt.Fprintf(stderr, "this document supplies %d versions, so -version is required; it supplies: %s\n\n",
			len(corpus.Versions), suppliedVersions(corpus))
		fmt.Fprint(stderr, usage)
		return exitUsage
	}

	index, issues := corpus.Index(selected)
	if len(issues) != 0 {
		report(stderr, issues)
		return exitFailure
	}

	compiled, issues := trace.Build(index)
	if len(issues) != 0 {
		report(stderr, issues)
		return exitFailure
	}

	if *dryRun {
		fmt.Fprint(stdout, traceReport(compiled))
		fmt.Fprintf(stdout, "\n--- payload the model would receive (%s) ---\n\n", procedure.ModelName())
		fmt.Fprint(stdout, procedure.RenderPrompt(compiled))
		return exitOK
	}

	built, code := draft(compiled, *answerPath, stderr)
	if code != exitOK {
		return code
	}

	if err := writeProcedure(built, *outPath, stdout); err != nil {
		fmt.Fprintf(stderr, "write the procedure: %v\n", err)
		return exitFailure
	}
	return exitOK
}

// draft obtains the answer, either from a stored file or from the model, and
// validates it against the same trace the prompt was rendered from.
func draft(compiled trace.Trace, answerPath string, stderr io.Writer) (procedure.Procedure, int) {
	if answerPath != "" {
		raw, err := os.ReadFile(answerPath)
		if err != nil {
			fmt.Fprintf(stderr, "read the stored answer: %v\n", err)
			return procedure.Procedure{}, exitFailure
		}

		built, issues := procedure.ParseAnswer(compiled, raw)
		if len(issues) != 0 {
			report(stderr, issues)
			return procedure.Procedure{}, exitFailure
		}
		return built, exitOK
	}

	ctx := context.Background()

	// NewAgent checks the credentials before it creates anything, so a run
	// without a key fails here rather than halfway through a call.
	agent, err := procedure.NewAgent(ctx, procedure.Config{})
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return procedure.Procedure{}, exitFailure
	}

	built, issues, err := agent.Draft(ctx, compiled)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return procedure.Procedure{}, exitFailure
	}
	if len(issues) != 0 {
		report(stderr, issues)
		return procedure.Procedure{}, exitFailure
	}
	return built, exitOK
}

// writeProcedure encodes the draft and sends it to the output file or stdout. It
// is only ever called once the answer has been accepted, so nothing
// half-validated can reach a file.
func writeProcedure(built procedure.Procedure, outPath string, stdout io.Writer) error {
	encoded, err := json.MarshalIndent(built, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')

	if outPath == "" || outPath == "-" {
		_, err := stdout.Write(encoded)
		return err
	}
	return os.WriteFile(outPath, encoded, 0o600)
}

func readInput(path string, stdin io.Reader) ([]byte, error) {
	if path == "" || path == "-" {
		return io.ReadAll(stdin)
	}
	return os.ReadFile(path)
}

// chooseVersion honours the requested version, or the only version when the
// document supplies exactly one. It never guesses between several.
func chooseVersion(corpus policy.Corpus, requested string) (string, bool) {
	if requested != "" {
		return requested, true
	}
	if len(corpus.Versions) == 1 {
		return corpus.Versions[0].Version, true
	}
	return "", false
}

func suppliedVersions(corpus policy.Corpus) string {
	versions := make([]string, 0, len(corpus.Versions))
	for _, version := range corpus.Versions {
		versions = append(versions, version.Version)
	}
	return strings.Join(versions, ", ")
}

// report writes every blocker in one pass, each with the path that locates it,
// so a policy owner fixes one list instead of rediscovering the next blocker
// after every fix.
func report(w io.Writer, issues []policy.Issue) {
	fmt.Fprintf(w, "the compile stopped with %d blocker(s); no procedure was drafted:\n", len(issues))
	for _, issue := range issues {
		fmt.Fprintf(w, "  %s\n", issue.String())
	}
}

// traceReport prints what Go decided, before any model is involved.
func traceReport(compiled trace.Trace) string {
	var report strings.Builder

	fmt.Fprintf(&report, "Policy:         %s\n", compiled.PolicyID)
	fmt.Fprintf(&report, "Version:        %s\n", compiled.Version)
	fmt.Fprintf(&report, "Effective date: %s\n", compiled.EffectiveDate.Format(policy.DateLayout))
	fmt.Fprintf(&report, "Rules in force: %d\n", len(compiled.Rules))
	fmt.Fprintf(&report, "Blockers:       none\n\n")

	for _, rule := range compiled.Rules {
		fmt.Fprintf(&report, "  %-12s %s\n", rule.Citation.String(), rule.Rule.Text)
		if rule.Rule.ExceptionOf != "" {
			fmt.Fprintf(&report, "  %-12s excepts %s\n", "", rule.Rule.ExceptionOf)
		}
		if rule.Approval != nil {
			fmt.Fprintf(&report, "  %-12s gate: %s\n", "", rule.Approval.Approver)
		}
	}

	for _, decision := range compiled.Decisions {
		fmt.Fprintf(&report, "\nDecision %s:\n", decision.Field)
		for _, branch := range decision.Branches {
			fmt.Fprintf(&report, "  %-12s %s %s %s\n", branch.RuleID, branch.Field, branch.Operator, branch.Value)
		}
	}

	return report.String()
}
