# 0019. Shared execution identity

- **Status:** Proposed
- **Date:** 2026-11-15

## Context

The same TR can run with different plan variants and in different scenarios.
Reconstructing its identity independently across orchestration and reporting can
mix results or associate artifacts with the wrong execution.

## Decision

Identify an execution by the structured tuple `(TR, variant, scenario)`, using
`core.ExecutionKey` for execution-keyed comparisons and maps. Empty variant and
scenario values are valid identities. Display labels are for presentation and
must not serve as identity keys, because distinct tuples can produce the same label.

Continuation, report merging, storage selection, and JUnit artifact attribution
retain all three fields. Verdict identity additionally includes the item;
measurement identity additionally includes name and percentile. Within progress
tracking and measurement selection, which are scoped to a single run, scenario
is implicit and the local key leaves it empty. These local keys must not be used
to compare executions across scenarios.

This refines the scenario and variant reporting decisions in
[ADR-0015](0015-scenario-runs-continue-from-merge-and-interactive-attestations.md)
and [ADR-0016](0016-named-plan-variants.md).

## Decision notes

### Options considered

- **Shared structured key (chosen):** makes the identity fields explicit and comparable.
- **Formatted labels or separate keys:** can collide or diverge across consumers.

### Consequences

Consumers must preserve the tuple when crossing scenario boundaries and extend it
when identifying individual verdicts or measurements. Single-run consumers can
omit the implicit scenario only within that scope.
