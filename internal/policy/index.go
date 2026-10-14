package policy

import (
	"fmt"
	"strings"
	"time"
)

// DateLayout is the only date format a policy document may use. A policy owner
// writing 01/03/2026 means two different days depending on the country, so the
// format is pinned instead of guessed.
const DateLayout = "2006-01-02"

// Issue codes reported by the effective-date, conflict and approval services.
const (
	// CodeInvalidDate marks a date the compiler cannot read.
	CodeInvalidDate = "invalid_date"
	// CodeObsoleteRule marks a rule that is not in force at the effective date:
	// its window closed before it, or has not opened yet.
	CodeObsoleteRule = "obsolete_rule"
	// CodeVersionConflict marks one rule id that two supplied versions define
	// with different text.
	CodeVersionConflict = "version_conflict"
	// CodeOrphanApproval marks a gate attached to a rule the version does not
	// define.
	CodeOrphanApproval = "orphan_approval"
)

// Index is the deterministic reading of one selected version: the facts the
// trace and the model are built from, and nothing the model produced.
type Index struct {
	PolicyID      string
	Version       string
	EffectiveDate time.Time
	Rules         []RuleFact
	Approvals     []Approval
}

// RuleFact is one rule of the selected version together with what the compiler
// derived from it. Obsolete is true when the rule's window does not cover the
// effective date, so the rule may never be cited. Approval is the gate that
// covers the rule, or nil when the version attaches none.
type RuleFact struct {
	Rule      Rule
	SectionID string
	Obsolete  bool
	Approval  *Approval
}

// Index reads one version of the corpus and reports every blocker the document
// contains: dates it cannot read, rules that are not in force, rule ids two
// versions disagree about, and approval gates attached to nothing.
//
// The approvals are read from the corpus rather than from the selected view,
// because PolicyInput keeps the three fields S5 names and nothing else.
func (c Corpus) Index(version string) (Index, []Issue) {
	selected, issues := c.Select(version)
	if len(issues) != 0 {
		return Index{}, issues
	}

	selectedIndex := c.indexOf(version)
	versionPath := fmt.Sprintf("versions[%d]", selectedIndex)
	approvals := c.Versions[selectedIndex].Approvals

	effectiveDate, dateIssues := c.effectiveDate()
	issues = append(issues, dateIssues...)

	facts, factIssues := ruleFacts(selected.Sections, approvals, versionPath, effectiveDate, c.dateIsUsable())
	issues = append(issues, factIssues...)
	issues = append(issues, orphanApprovals(approvals, versionPath, facts)...)
	issues = append(issues, c.conflicts()...)

	return Index{
		PolicyID:      selected.PolicyID,
		Version:       selected.Version,
		EffectiveDate: effectiveDate,
		Rules:         facts,
		Approvals:     approvals,
	}, issues
}

// dateIsUsable reports whether the effective date could be read, which decides
// whether a rule's window can be compared against it at all.
func (c Corpus) dateIsUsable() bool {
	_, err := time.Parse(DateLayout, c.EffectiveDate)
	return err == nil
}

func (c Corpus) effectiveDate() (time.Time, []Issue) {
	date, err := time.Parse(DateLayout, c.EffectiveDate)
	if err != nil {
		return time.Time{}, []Issue{{
			Code:    CodeInvalidDate,
			Path:    "effectiveDate",
			Message: fmt.Sprintf("effectiveDate %q is not a date in %s form", c.EffectiveDate, DateLayout),
		}}
	}
	return date, nil
}

// indexOf locates the selected version in the document so a reported path can
// point at the version the reader is looking at.
func (c Corpus) indexOf(version string) int {
	for i, candidate := range c.Versions {
		if candidate.Version == version {
			return i
		}
	}
	return -1
}

// ruleFacts folds the selected version's sections into one ordered slice of
// rules, deriving the window fact of each and attaching its approval gate.
func ruleFacts(sections []Section, approvals []Approval, versionPath string, effectiveDate time.Time, dateIsUsable bool) ([]RuleFact, []Issue) {
	var issues []Issue

	gates := map[string]*Approval{}
	for i := range approvals {
		gate := approvals[i]
		if _, taken := gates[gate.RuleID]; !taken {
			gates[gate.RuleID] = &gate
		}
	}

	var facts []RuleFact
	for s, section := range sections {
		for r, rule := range section.Rules {
			rulePath := fmt.Sprintf("%s.sections[%d].rules[%d]", versionPath, s, r)

			from, fromOK := parseRuleDate(rule.EffectiveFrom, rulePath+".effectiveFrom", rule.ID, &issues)
			to, toOK := parseRuleDate(rule.EffectiveTo, rulePath+".effectiveTo", rule.ID, &issues)

			fact := RuleFact{
				Rule:      rule,
				SectionID: section.ID,
				Approval:  gates[rule.ID],
			}

			// A window can only be judged when every date it is made of was
			// readable; otherwise the date errors already reported stand alone.
			if dateIsUsable && fromOK && toOK {
				fact.Obsolete = !covers(from, to, effectiveDate)
				if fact.Obsolete {
					issues = append(issues, Issue{
						Code:    CodeObsoleteRule,
						Path:    rulePath,
						RuleID:  rule.ID,
						Message: outsideWindow(rule.ID, from, to, effectiveDate),
					})
				}
			}

			facts = append(facts, fact)
		}
	}

	return facts, issues
}

// parseRuleDate reads an optional rule date. An empty value is the open end of a
// window and is not an error.
func parseRuleDate(value, path, ruleID string, issues *[]Issue) (time.Time, bool) {
	if value == "" {
		return time.Time{}, true
	}

	date, err := time.Parse(DateLayout, value)
	if err != nil {
		*issues = append(*issues, Issue{
			Code:    CodeInvalidDate,
			Path:    path,
			RuleID:  ruleID,
			Message: fmt.Sprintf("%q is not a date in %s form", value, DateLayout),
		})
		return time.Time{}, false
	}

	return date, true
}

// covers reports whether the window from and to contains the effective date. A
// zero from is unbounded in the past and a zero to is unbounded in the future.
func covers(from, to, effectiveDate time.Time) bool {
	if !from.IsZero() && effectiveDate.Before(from) {
		return false
	}
	if !to.IsZero() && effectiveDate.After(to) {
		return false
	}
	return true
}

func outsideWindow(ruleID string, from, to, effectiveDate time.Time) string {
	switch {
	case !to.IsZero() && effectiveDate.After(to):
		return fmt.Sprintf("rule %q stopped applying on %s, before the %s effective date, so it may not be cited",
			ruleID, to.Format(DateLayout), effectiveDate.Format(DateLayout))
	case !from.IsZero():
		return fmt.Sprintf("rule %q only starts applying on %s, after the %s effective date, so it may not be cited",
			ruleID, from.Format(DateLayout), effectiveDate.Format(DateLayout))
	default:
		return fmt.Sprintf("rule %q does not apply on the %s effective date", ruleID, effectiveDate.Format(DateLayout))
	}
}

// orphanApprovals reports gates attached to a rule the selected version does not
// define. Silently ignoring one would let a document look approved while the
// rule it was meant to gate stays ungated.
func orphanApprovals(approvals []Approval, versionPath string, facts []RuleFact) []Issue {
	defined := map[string]bool{}
	for _, fact := range facts {
		defined[fact.Rule.ID] = true
	}

	var issues []Issue
	for a, gate := range approvals {
		if defined[gate.RuleID] {
			continue
		}
		issues = append(issues, Issue{
			Code:   CodeOrphanApproval,
			Path:   fmt.Sprintf("%s.approvals[%d].ruleID", versionPath, a),
			RuleID: gate.RuleID,
			Message: fmt.Sprintf("approval gate %q names rule %q, which this version does not define",
				gate.ID, gate.RuleID),
		})
	}

	return issues
}

// conflicts reports every rule id that two supplied versions define with
// different text. It is deliberately not resolved: the compiler has no basis to
// prefer either text, so the policy owner has to say which one is the truth.
func (c Corpus) conflicts() []Issue {
	type definition struct {
		version string
		text    string
	}

	order := []string{}
	byRule := map[string][]definition{}

	for _, version := range c.Versions {
		seenInVersion := map[string]bool{}
		for _, section := range version.Sections {
			for _, rule := range section.Rules {
				if rule.ID == "" || seenInVersion[rule.ID] {
					// A rule id twice inside one version is a duplicate, which
					// Parse already reports, not a disagreement between versions.
					continue
				}
				seenInVersion[rule.ID] = true

				if _, known := byRule[rule.ID]; !known {
					order = append(order, rule.ID)
				}
				byRule[rule.ID] = append(byRule[rule.ID], definition{version: version.Version, text: rule.Text})
			}
		}
	}

	var issues []Issue
	for _, ruleID := range order {
		definitions := byRule[ruleID]

		texts := map[string]bool{}
		for _, definition := range definitions {
			texts[definition.text] = true
		}
		if len(texts) < 2 {
			continue
		}

		parts := make([]string, 0, len(definitions))
		for _, definition := range definitions {
			parts = append(parts, fmt.Sprintf("%s: %q", definition.version, definition.text))
		}

		issues = append(issues, Issue{
			Code:   CodeVersionConflict,
			RuleID: ruleID,
			Message: fmt.Sprintf("rule %q is defined with different text by supplied versions (%s); state which text is the approved one",
				ruleID, strings.Join(parts, "; ")),
		})
	}

	return issues
}
