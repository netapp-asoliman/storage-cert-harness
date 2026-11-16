# Checkup JUnit profile

`report.junit.flat.xml` exposes Harness verdicts to consumers that read only the
first JUnit suite, including the OpenShift Virtualization Checkup. It is emitted
alongside the grouped `report.junit.xml` and the detailed JSON and Markdown reports.
The grouped report remains the general JUnit report and is not changed by this profile.

## Version 1

The root `<testsuites>` has a `harness.checkup_junit_profile_version` property
with value `1` and contains exactly one `<testsuite>`. A consumer should check
this property before relying on the profile; the generic Checkup parser does not
currently perform this check. The existing `harness.junit_profile_version`
property has a separate meaning and does not identify the Checkup profile.

The suite contains one testcase per Harness verdict, including each SLA or
functional check independently. Informational measurements, environment facts,
and attestations do not become testcases. One test requirement can therefore
produce multiple testcases. The suite name is descriptive; consumers must not
use it to identify the profile.

The testcase `classname` is the test requirement execution label. Its `name`
contains the requirement ID, optional variant, criterion and optional scenario:
`TR-ID [variant] / criterion [scenario]`, with absent parts omitted. The name
is for display and must not be split to recover identity: its component strings
can contain separators. Case properties carry `harness.tr_id`, `harness.item`,
and, when applicable, `harness.variant` and `scenario`. The XML encoder escapes
special characters. The generic Checkup summary uses testcase names for failure
lists and does not preserve these properties; detailed reports remain authoritative.

Each verdict maps to exactly one outcome:

| Verdict | JUnit representation |
| --- | --- |
| pass | No `failure`, `error`, or `skipped` child. |
| fail | A `failure` child, with the reason when available. |
| error | An `error` child, with the reason when available. |
| skip | A `skipped` child, with the reason when available. |

Both root and suite counts describe verdict cases. `tests` includes skipped
cases; `failures`, `errors` and `skipped` count their respective outcomes.
Unsupported outcomes or verdicts missing an ID or criterion fail export instead
of being represented as passes. Expected and actual values are testcase
properties when available; the existing generic Checkup parser does not display
them.

## Completion and compatibility

Harness writes reports at checkpoints as well as at completion. The presence of
a flat JUnit file alone does not establish that a run completed, nor that it was
certifiable. Consumers must account for the Harness exit status and report
lifecycle (`harness.status`, health and incidents) as well as verdicts. A
Checkup adapter is responsible for detecting incomplete runs and failed setup,
and for retaining available reports and logs. A process that cannot start or is
forcibly terminated may be unable to publish final artifacts.

Version 1 permits extra properties and other additive details that do not change
the required fields or their meaning. Changes to required identity, case
granularity, outcomes, counts or layout require a new profile version. Consumers
should reject unsupported versions instead of assuming they are compatible.
