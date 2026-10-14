package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixturePath is the repository-relative path of one supplied policy document.
func fixturePath(name string) string {
	return filepath.Join("testdata", name)
}

// mustRun runs the command with the given arguments and stdin, and returns the
// exit code together with everything it wrote.
func mustRun(t *testing.T, args []string, stdin string) (code int, stdout, stderr string) {
	t.Helper()

	var out, errOut strings.Builder
	code = run(args, strings.NewReader(stdin), &out, &errOut)
	return code, out.String(), errOut.String()
}

// TestRunRejectsAnUnknownFlag pins that a misspelled flag is a usage error, not
// a silent run with the default input.
func TestRunRejectsAnUnknownFlag(t *testing.T) {
	code, stdout, stderr := mustRun(t, []string{"-nope"}, "")
	if code != exitUsage {
		t.Errorf("exit code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(strings.ToLower(stderr), "usage") {
		t.Errorf("stderr = %q, want the usage", stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}

// TestRunRequiresAVersionWhenTheDocumentSuppliesSeveral pins that the program
// never guesses which version to compile when the document offers a choice.
func TestRunRequiresAVersionWhenTheDocumentSuppliesSeveral(t *testing.T) {
	t.Setenv(APIKeyEnv, "")

	code, _, stderr := mustRun(t, []string{"-in", fixturePath("policies_ok.json"), "-dry-run"}, "")
	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitUsage, stderr)
	}
	for _, want := range []string{"3.0", "3.1"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr = %q, does not name the supplied version %s", stderr, want)
		}
	}
}

// TestRunUsesTheOnlyVersionWhenNoneIsGiven pins the convenience the fixtures
// rely on: a document with one version needs no -version.
func TestRunUsesTheOnlyVersionWhenNoneIsGiven(t *testing.T) {
	t.Setenv(APIKeyEnv, "")

	code, stdout, stderr := mustRun(t, []string{"-in", fixturePath("policies_exceptions.json"), "-dry-run"}, "")
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitOK, stderr)
	}
	if !strings.Contains(stdout, "DPR-DATA-RETENTION") {
		t.Errorf("stdout does not name the policy it compiled:\n%s", stdout)
	}
}

// TestRunReadsTheDocumentFromStdin pins that the input can be piped, which is
// how another program would drive this one.
func TestRunReadsTheDocumentFromStdin(t *testing.T) {
	t.Setenv(APIKeyEnv, "")

	document, err := os.ReadFile(fixturePath("policies_exceptions.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	code, stdout, stderr := mustRun(t, []string{"-version", "5.0", "-dry-run"}, string(document))
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitOK, stderr)
	}
	if !strings.Contains(stdout, "DR-1") {
		t.Errorf("stdout does not trace the rules it read from stdin:\n%s", stdout)
	}
}

// TestRunReportsAnUnreadableDocument pins the first thing a policy owner can get
// wrong, and that the message goes to stderr rather than into the procedure.
func TestRunReportsAnUnreadableDocument(t *testing.T) {
	t.Setenv(APIKeyEnv, "")

	code, stdout, stderr := mustRun(t, []string{"-dry-run"}, `{"policyID": `)
	if code != exitFailure {
		t.Errorf("exit code = %d, want %d", code, exitFailure)
	}
	if !strings.Contains(stderr, "invalid_input") {
		t.Errorf("stderr = %q, want the invalid_input code", stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty: a rejected document produces no procedure", stdout)
	}
}

// TestRunWritesNothingWhenTheCompileIsBlocked pins A8 and S12 through the only
// path that could publish anything: the output file must not be created when a
// blocker stopped the compile.
func TestRunWritesNothingWhenTheCompileIsBlocked(t *testing.T) {
	t.Setenv(APIKeyEnv, "")

	out := filepath.Join(t.TempDir(), "procedure.json")
	code, stdout, stderr := mustRun(t, []string{
		"-in", fixturePath("policies_version_conflict.json"),
		"-version", "2.2",
		"-dry-run",
		"-out", out,
	}, "")
	if code != exitFailure {
		t.Errorf("exit code = %d, want %d (stderr: %s)", code, exitFailure, stderr)
	}
	if !strings.Contains(stderr, "version_conflict") {
		t.Errorf("stderr = %q, want the version_conflict code", stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("the output file exists after a blocked compile (stat error: %v)", err)
	}
}

// TestRunRefusesToCallTheModelWithoutCredentials pins that a run without a key
// fails before the call and leaves no half-written procedure behind.
func TestRunRefusesToCallTheModelWithoutCredentials(t *testing.T) {
	t.Setenv(APIKeyEnv, "")

	out := filepath.Join(t.TempDir(), "procedure.json")
	code, stdout, stderr := mustRun(t, []string{
		"-in", fixturePath("policies_ok.json"),
		"-version", "3.1",
		"-out", out,
	}, "")
	if code != exitFailure {
		t.Errorf("exit code = %d, want %d", code, exitFailure)
	}
	if !strings.Contains(stderr, APIKeyEnv) {
		t.Errorf("stderr = %q, does not name %s", stderr, APIKeyEnv)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("the output file exists after a failed run (stat error: %v)", err)
	}
}

// TestRunDryRunPrintsTheTraceAndThePayload pins the documented dry run: it needs
// no credentials, it names the rules it traced, and it prints the exact payload
// the model would receive, so a policy owner can review the prompt before
// spending a call.
func TestRunDryRunPrintsTheTraceAndThePayload(t *testing.T) {
	t.Setenv(APIKeyEnv, "")

	code, stdout, stderr := mustRun(t, []string{
		"-in", fixturePath("policies_ok.json"),
		"-version", "3.1",
		"-dry-run",
	}, "")
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitOK, stderr)
	}

	for _, want := range []string{
		"AU-1",
		"AU-2",
		"AU-3",
		"AU-4",
		"3.1",
		"enrolled in device management",
		"ruleID",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("the dry run does not show %q:\n%s", want, stdout)
		}
	}
}

// TestRunWritesTheProcedureToTheOutputFile pins the successful path: an answer
// replayed through the validator, written out as a draft procedure.
func TestRunWritesTheProcedureToTheOutputFile(t *testing.T) {
	t.Setenv(APIKeyEnv, "")

	dir := t.TempDir()
	out := filepath.Join(dir, "procedure.json")
	answer := writeAnswer(t, dir, `{"steps":[{"text":"Enrol the device first.","ruleIDs":["AU-2"]}]}`)

	code, _, stderr := mustRun(t, []string{
		"-in", fixturePath("policies_ok.json"),
		"-version", "3.1",
		"-answer", answer,
		"-out", out,
	}, "")
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitOK, stderr)
	}

	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read the written procedure: %v", err)
	}

	// A citation is an object, not the "R2@3.1" form: the String form is for a
	// reader, and the emitted procedure keeps the rule id and the version
	// separable for a program.
	for _, want := range []string{
		`"status": "draft"`,
		`"approvedBy": ""`,
		`"ruleID": "AU-2"`,
		`"version": "3.1"`,
		"Enrol the device first.",
	} {
		if !strings.Contains(string(written), want) {
			t.Errorf("the written procedure does not contain %q:\n%s", want, written)
		}
	}
}

// TestRunRejectsAnAnswerThatCitesOutsideTheTrace pins A3 at the command line:
// the program refuses the answer and writes nothing.
func TestRunRejectsAnAnswerThatCitesOutsideTheTrace(t *testing.T) {
	t.Setenv(APIKeyEnv, "")

	dir := t.TempDir()
	out := filepath.Join(dir, "procedure.json")
	answer := writeAnswer(t, dir, `{"steps":[{"text":"Do something else entirely.","ruleIDs":["NOPE"]}]}`)

	code, _, stderr := mustRun(t, []string{
		"-in", fixturePath("policies_ok.json"),
		"-version", "3.1",
		"-answer", answer,
		"-out", out,
	}, "")
	if code != exitFailure {
		t.Errorf("exit code = %d, want %d", code, exitFailure)
	}
	if !strings.Contains(stderr, "unknown_citation") {
		t.Errorf("stderr = %q, want the unknown_citation code", stderr)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("the output file exists after a rejected answer (stat error: %v)", err)
	}
}
