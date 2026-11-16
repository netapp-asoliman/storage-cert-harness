# 0020. Checkup JUnit verdict profile

- **Status:** Proposed
- **Date:** 2026-11-15

## Context

OpenShift Virtualization Checkup consumes the first suite in a JUnit report.
The Harness grouped report includes environment suites before verdict suites,
so it cannot be consumed as a complete verdict summary by this parser.

## Decision

Alongside the existing grouped JUnit report, emit a versioned Checkup JUnit
profile containing one suite with one testcase per graded verdict. The profile
preserves the requirement identity, outcome and accurate suite counts. The
documented v1 contract is in [Checkup JUnit profile](../docs/checkup-junit-profile.md).
The grouped and JSON reports remain the detailed evidence sources. The Checkup
adapter must check profile compatibility and run completion before publishing a
summary; checkpoint JUnit alone does not prove a complete run.

This refines the reporting choices in [ADR 0014](0014-report-model-environment-attestations-and-exporters.md).

## Decision notes

### Options considered

- **Additional flat JUnit profile:** works with the existing Checkup parser and retains the grouped report.
- **Modify the Checkup parser:** could consume all grouped suites, but requires consumer changes and must distinguish informational cases from graded verdicts.
- **Convert Harness JSON in Checkup:** preserves structured detail but adds a separate converter and contract to the adapter.

### Consequences

The Harness produces an additional report and maintains a versioned profile.
Future incompatible profile changes require a new version; the adapter and
consumer integration tests are separate work.
