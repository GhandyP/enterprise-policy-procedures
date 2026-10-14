# Policy fixtures

Each file here is one policy document written the way a policy owner would submit
it, plus exactly one defect. They exist so that every rule the compiler enforces
can be pointed at a real document instead of an invented one, and so a reviewer
can see the rule fail on the document that triggers it.

| File | What it demonstrates | What the compiler reports |
| --- | --- | --- |
| `policies_ok.json` | An acceptable-use policy with two versions, a satisfied approval gate, an exception and a rule added in the newer version. | Nothing. This is the control: it must compile clean. |
| `policies_version_conflict.json` | The expense claim window is 30 days in version 2.1 and 45 days in version 2.2. | `version_conflict` on `EC-1`: the compiler will not choose between the two texts. |
| `policies_obsolete_rule.json` | The legacy VPN client was retired on 31 December 2025, and the policy is compiled for 1 March 2026. | `obsolete_rule` on `RA-2`: a rule that no longer applies cannot be cited. |
| `policies_missing_approval.json` | Capital expenditure above 50000 EUR requires the finance committee, and no approval gate is recorded for it. | `missing_approval` on `CE-1`: no step may be drafted from a rule nobody is recorded as approving. |
| `policies_exceptions.json` | Three retention branches on `recordType`, an exception to the supplier rule, and a deletion run that needs sign-off. | Nothing. It shows exceptions, branches and a gate surviving into the procedure. |
| `policies_conditional_branches.json` | One decision, `purchaseSize`, with three branches and a different approver for each. | Nothing. It shows one condition field becoming one decision with three branches. |
| `policies_invalid.json` | A rule with no text, and the identifier `DC-2` used twice in one version. | `invalid_input` for the missing text and `duplicate_rule_id` for `DC-2`. |

## Reading a fixture

Every field in these files is one of the documented ones: `policyID`,
`effectiveDate` and `versions` at the top; `version`, `sections` and `approvals`
in a version; `id`, `title` and `rules` in a section; `id`, `text`, `owner` and
`effectiveFrom` on a rule, plus the optional `effectiveTo`, `condition`,
`exceptionOf` and `requiresApproval`; and `id`, `ruleID`, `approver` and `role`
on an approval gate.

Dates are always `YYYY-MM-DD`. A condition is always one field, `==` or `!=`, and
one value with no spaces in it — `deviceOwner == contractor`. Anything else is
reported rather than interpreted.

These fixtures are synthetic and describe no real company, product or
regulation. They are test data, not policy advice.
