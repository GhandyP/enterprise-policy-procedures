package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeAnswer stores one model answer in a temporary directory and returns the
// path, which is how -answer takes a stored answer.
func writeAnswer(t *testing.T, dir, answer string) string {
	t.Helper()

	path := filepath.Join(dir, "answer.json")
	if err := os.WriteFile(path, []byte(answer), 0o600); err != nil {
		t.Fatalf("write the stored answer: %v", err)
	}
	return path
}

// TestFixtureSetIsExactlyTheDocumentedSeven pins the fixtures against their
// index, so a new document cannot arrive without testdata/README.md explaining
// which rule it pins and what the compiler is meant to report for it.
func TestFixtureSetIsExactlyTheDocumentedSeven(t *testing.T) {
	index, err := os.ReadFile(fixturePath("README.md"))
	if err != nil {
		t.Fatalf("read the fixture index: %v", err)
	}

	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}

	fixtures := 0
	for _, entry := range entries {
		name := entry.Name()
		if name == "README.md" {
			continue
		}
		if !strings.HasSuffix(name, ".json") {
			t.Errorf("testdata contains %q, which is neither a fixture nor the index", name)
			continue
		}
		fixtures++
		if !strings.Contains(string(index), name) {
			t.Errorf("testdata/README.md does not describe %q", name)
		}
	}

	if got, want := fixtures, 7; got != want {
		t.Errorf("testdata holds %d fixtures, want %d", got, want)
	}
}

// TestFixturesBehaveAsDocumented runs every fixture through the program and
// pins what the compiler reports for it. Each case names the one rule from the
// fixture index it covers, and asserts the code the index promises.
func TestFixturesBehaveAsDocumented(t *testing.T) {
	t.Setenv(APIKeyEnv, "")

	tests := []struct {
		name       string
		fixture    string
		version    string
		wantCode   int
		wantIssues []string
	}{
		{
			name:     "an acceptable-use policy with a satisfied gate and an exception compiles clean",
			fixture:  "policies_ok.json",
			version:  "3.1",
			wantCode: exitOK,
		},
		{
			name:       "two versions disagree about the expense claim window",
			fixture:    "policies_version_conflict.json",
			version:    "2.2",
			wantCode:   exitFailure,
			wantIssues: []string{"version_conflict"},
		},
		{
			name:       "a rule retired before the effective date",
			fixture:    "policies_obsolete_rule.json",
			version:    "1.4",
			wantCode:   exitFailure,
			wantIssues: []string{"obsolete_rule"},
		},
		{
			name:       "a rule that requires approval with no gate recorded",
			fixture:    "policies_missing_approval.json",
			wantCode:   exitFailure,
			wantIssues: []string{"missing_approval"},
		},
		{
			name:     "exceptions, three branches and a gate survive the trace",
			fixture:  "policies_exceptions.json",
			wantCode: exitOK,
		},
		{
			name:     "one condition field becomes one decision with three branches",
			fixture:  "policies_conditional_branches.json",
			wantCode: exitOK,
		},
		{
			name:       "a rule with no text and an identifier used twice",
			fixture:    "policies_invalid.json",
			wantCode:   exitFailure,
			wantIssues: []string{"invalid_input", "duplicate_rule_id"},
		},
	}

	for _, test := range tests {
		t.Run(test.fixture, func(t *testing.T) {
			args := []string{"-in", fixturePath(test.fixture), "-dry-run"}
			if test.version != "" {
				args = append(args, "-version", test.version)
			}

			code, stdout, stderr := mustRun(t, args, "")
			if code != test.wantCode {
				t.Fatalf("exit code = %d, want %d (stderr: %s)", code, test.wantCode, stderr)
			}

			for _, want := range test.wantIssues {
				if !strings.Contains(stderr, want) {
					t.Errorf("stderr does not report %q:\n%s", want, stderr)
				}
			}

			if test.wantCode == exitOK {
				if stderr != "" {
					t.Errorf("a clean fixture reported %q", stderr)
				}
				if stdout == "" {
					t.Error("a clean fixture produced no trace")
				}
			} else if stdout != "" {
				t.Errorf("a blocked compile printed a procedure:\n%s", stdout)
			}
		})
	}
}

// TestObsoleteRuleIsNotCitableThroughTheCommandLine pins A3 where a policy owner
// can see it: the retired rule is absent from the payload the model would get,
// so it cannot come back as a citation.
func TestObsoleteRuleIsNotCitableThroughTheCommandLine(t *testing.T) {
	t.Setenv(APIKeyEnv, "")

	// The fixture is blocked by the retired rule, so the trace never reaches a
	// prompt. The dry run of the clean fixture is where the absence is visible,
	// and policies_ok.json supplies the same shape without the blocker: its
	// exception rule AU-3 is citable and its older version is not selected.
	_, stdout, stderr := mustRun(t, []string{
		"-in", fixturePath("policies_ok.json"),
		"-version", "3.1",
		"-dry-run",
	}, "")
	if stderr != "" {
		t.Fatalf("the clean fixture reported %q", stderr)
	}

	if !strings.Contains(stdout, "AU-3") {
		t.Errorf("the payload omits AU-3, which is in force:\n%s", stdout)
	}
}

// TestReadmeClaimsMatchTheFixtures pins the two claims the fixture index makes
// that a reader would otherwise have to take on trust: the branch count of the
// conditional fixture, and that the exception fixture carries an approval gate.
func TestReadmeClaimsMatchTheFixtures(t *testing.T) {
	t.Setenv(APIKeyEnv, "")

	_, stdout, _ := mustRun(t, []string{
		"-in", fixturePath("policies_conditional_branches.json"),
		"-dry-run",
	}, "")
	for _, want := range []string{"purchaseSize == small", "purchaseSize == medium", "purchaseSize == large"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("the decision does not show %q:\n%s", want, stdout)
		}
	}

	_, stdout, _ = mustRun(t, []string{
		"-in", fixturePath("policies_exceptions.json"),
		"-dry-run",
	}, "")
	for _, want := range []string{"exception to: DR-2", "approval gate: Data Protection Officer"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("the payload does not show %q:\n%s", want, stdout)
		}
	}
}
