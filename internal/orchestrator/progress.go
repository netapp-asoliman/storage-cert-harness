package orchestrator

import (
	"log/slog"
	"sync"

	"github.com/openshift-storage-cert/storage-cert-harness/internal/core"
)

// progressTracker reports plan execution progress at the Test Requirement
// execution level. Named variants are separate executions, while a TR's
// individual SLA/check verdicts are intentionally not counted separately.
type progressTracker struct {
	logger        *slog.Logger
	onStateChange func(key core.ExecutionKey, state progressState, outcome core.Outcome)

	mu       sync.Mutex
	order    []core.ExecutionKey
	state    map[core.ExecutionKey]progressState
	outcomes map[core.ExecutionKey]core.Outcome
}

type progressState uint8

const (
	progressQueued progressState = iota
	progressRunning
	progressComplete
	progressAborted
)

func newProgressTracker(logger *slog.Logger, trs []core.TestRequirement) *progressTracker {
	p := &progressTracker{
		logger:   logger,
		order:    make([]core.ExecutionKey, 0, len(trs)),
		state:    make(map[core.ExecutionKey]progressState, len(trs)),
		outcomes: make(map[core.ExecutionKey]core.Outcome, len(trs)),
	}
	for _, tr := range trs {
		key := core.ExecutionKey{TR: tr.ID, Variant: tr.Variant}
		p.order = append(p.order, key)
		p.state[key] = progressQueued
	}
	p.loggerInfo("plan progress",
		"status", "started",
		"completed", 0,
		"total", len(p.order),
		"remaining", len(p.order),
		"queued", progressLabels(p.order),
	)
	return p
}

func (p *progressTracker) start(tr core.TestRequirement) {
	if p == nil {
		return
	}
	key := core.ExecutionKey{TR: tr.ID, Variant: tr.Variant}
	label := core.ExecutionLabel(tr.ID, tr.Variant)
	p.mu.Lock()
	if p.state[key] != progressQueued {
		p.mu.Unlock()
		return
	}
	p.state[key] = progressRunning
	completed, total := p.countsLocked()
	running, queued := p.listsLocked(progressRunning, progressQueued)
	p.loggerDebug("plan progress detail",
		"status", "running",
		"test", label,
		"completed", completed,
		"total", total,
		"remaining", total-completed,
		"running", running,
		"queued", queued,
	)
	p.mu.Unlock()
	if p.onStateChange != nil {
		p.onStateChange(key, progressRunning, "")
	}
}

func (p *progressTracker) complete(tr core.TestRequirement, outcome core.Outcome) {
	if p == nil {
		return
	}
	key := core.ExecutionKey{TR: tr.ID, Variant: tr.Variant}
	label := core.ExecutionLabel(tr.ID, tr.Variant)
	p.mu.Lock()
	if p.state[key] == progressComplete {
		p.mu.Unlock()
		return
	}
	p.state[key] = progressComplete
	p.outcomes[key] = outcome
	completed, total := p.countsLocked()
	running, queued := p.listsLocked(progressRunning, progressQueued)
	passed, failed, errors, skipped := p.outcomeListsLocked()
	p.loggerInfo("test progress",
		"status", "completed",
		"test", label,
		"outcome", outcome,
		"completed", completed,
		"total", total,
		"remaining", total-completed,
	)
	p.loggerDebug("plan progress detail",
		"status", "updated",
		"running", running,
		"queued", queued,
	)
	if completed == total {
		p.loggerInfo("plan progress",
			"status", "complete",
			"completed", completed,
			"total", total,
			"remaining", 0,
			"passed", passed,
			"failed", failed,
			"errors", errors,
			"skipped", skipped,
			"queued", queued,
		)
	}
	p.mu.Unlock()
	if p.onStateChange != nil {
		p.onStateChange(key, progressComplete, outcome)
	}
}

// abort records that an execution was entered or scheduled but did not reach a
// result. Aborted executions must remain resumable; they are deliberately not
// represented as completed errors.
func (p *progressTracker) abort(tr core.TestRequirement, outcome core.Outcome) {
	if p == nil {
		return
	}
	key := core.ExecutionKey{TR: tr.ID, Variant: tr.Variant}
	label := core.ExecutionLabel(tr.ID, tr.Variant)
	p.mu.Lock()
	if p.state[key] == progressComplete || p.state[key] == progressAborted {
		p.mu.Unlock()
		return
	}
	p.state[key] = progressAborted
	p.outcomes[key] = outcome
	completed, total := p.countsLocked()
	running, queued := p.listsLocked(progressRunning, progressQueued)
	p.loggerInfo("test progress",
		"status", "aborted",
		"test", label,
		"outcome", outcome,
		"completed", completed,
		"total", total,
		"remaining", total-completed,
	)
	p.loggerDebug("plan progress detail",
		"status", "updated",
		"running", running,
		"queued", queued,
	)
	p.mu.Unlock()
	if p.onStateChange != nil {
		p.onStateChange(key, progressAborted, outcome)
	}
}

func (p *progressTracker) countsLocked() (completed, total int) {
	total = len(p.order)
	for _, key := range p.order {
		if p.state[key] == progressComplete {
			completed++
		}
	}
	return completed, total
}

func (p *progressTracker) listsLocked(wanted ...progressState) (running, queued []string) {
	for _, key := range p.order {
		switch p.state[key] {
		case progressRunning:
			for _, state := range wanted {
				if state == progressRunning {
					running = append(running, core.ExecutionLabel(key.TR, key.Variant))
					break
				}
			}
		case progressQueued:
			for _, state := range wanted {
				if state == progressQueued {
					queued = append(queued, core.ExecutionLabel(key.TR, key.Variant))
					break
				}
			}
		}
	}
	return running, queued
}

func (p *progressTracker) outcomeListsLocked() (passed, failed, errors, skipped []string) {
	for _, key := range p.order {
		switch p.outcomes[key] {
		case core.OutcomePass:
			passed = append(passed, core.ExecutionLabel(key.TR, key.Variant))
		case core.OutcomeFail:
			failed = append(failed, core.ExecutionLabel(key.TR, key.Variant))
		case core.OutcomeError:
			errors = append(errors, core.ExecutionLabel(key.TR, key.Variant))
		case core.OutcomeSkip:
			skipped = append(skipped, core.ExecutionLabel(key.TR, key.Variant))
		}
	}
	return passed, failed, errors, skipped
}

func progressLabels(keys []core.ExecutionKey) []string {
	labels := make([]string, 0, len(keys))
	for _, key := range keys {
		labels = append(labels, core.ExecutionLabel(key.TR, key.Variant))
	}
	return labels
}

func (p *progressTracker) loggerInfo(msg string, args ...any) {
	if p != nil && p.logger != nil {
		p.logger.Info(msg, args...)
	}
}

func (p *progressTracker) loggerDebug(msg string, args ...any) {
	if p != nil && p.logger != nil {
		p.logger.Debug(msg, args...)
	}
}

// summarizeTR collapses the item-level verdicts for one execution into the
// user-facing progress outcome. Errors take precedence over failures, then
// passes; a result containing only skips is shown as skipped.
func summarizeTR(verdicts []core.Verdict, tr core.TestRequirement) core.Outcome {
	best := core.OutcomeSkip
	seen := false
	for _, verdict := range verdicts {
		if verdict.TR != tr.ID || verdict.Variant != tr.Variant {
			continue
		}
		seen = true
		if outcomeRank(verdict.Outcome) > outcomeRank(best) {
			best = verdict.Outcome
		}
	}
	if !seen {
		return core.OutcomeError
	}
	return best
}

func outcomeRank(outcome core.Outcome) int {
	switch outcome {
	case core.OutcomeError:
		return 3
	case core.OutcomeFail:
		return 2
	case core.OutcomePass:
		return 1
	default:
		return 0
	}
}
