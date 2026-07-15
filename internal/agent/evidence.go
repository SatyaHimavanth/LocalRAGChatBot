package agent

import (
	"strings"
	"time"
)

// EvidenceEffort controls how aggressively the agent should gather evidence.
// It refers to retrieval/evidence construction, not hidden model reasoning.
type EvidenceEffort string

// RetrievalScope controls whether the agent should search the active collection
// or the entire library of collections for a turn.
type RetrievalScope string

const (
	RetrievalScopeCurrent RetrievalScope = "current"
	RetrievalScopeAll     RetrievalScope = "all"
)

// NormalizeRetrievalScope converts an arbitrary string into a supported scope.
func NormalizeRetrievalScope(raw string) RetrievalScope {
	switch RetrievalScope(strings.ToLower(strings.TrimSpace(raw))) {
	case RetrievalScopeAll:
		return RetrievalScopeAll
	default:
		return RetrievalScopeCurrent
	}
}

// String returns the stable wire value.
func (s RetrievalScope) String() string {
	switch s {
	case RetrievalScopeAll:
		return "all"
	default:
		return "current"
	}
}

// Label returns a human-friendly label for the UI.
func (s RetrievalScope) Label() string {
	switch s {
	case RetrievalScopeAll:
		return "All collections"
	default:
		return "Current collection"
	}
}

// Guidance returns a short instruction line for the system prompt.
func (s RetrievalScope) Guidance() string {
	switch s {
	case RetrievalScopeAll:
		return "Retrieval scope is set to all collections. Search across the full knowledge base when the question may span multiple collections."
	default:
		return "Retrieval scope is limited to the active collection. Prefer the current collection unless the user clearly requests broader search."
	}
}

const (
	EvidenceEffortLow    EvidenceEffort = "low"
	EvidenceEffortMedium EvidenceEffort = "medium"
	EvidenceEffortHigh   EvidenceEffort = "high"
)

// NormalizeEvidenceEffort converts an arbitrary string into a supported effort level.
func NormalizeEvidenceEffort(raw string) EvidenceEffort {
	switch EvidenceEffort(strings.ToLower(strings.TrimSpace(raw))) {
	case EvidenceEffortLow:
		return EvidenceEffortLow
	case EvidenceEffortHigh:
		return EvidenceEffortHigh
	default:
		return EvidenceEffortMedium
	}
}

// String returns the stable wire value.
func (e EvidenceEffort) String() string {
	switch e {
	case EvidenceEffortLow:
		return "low"
	case EvidenceEffortHigh:
		return "high"
	default:
		return "medium"
	}
}

// Label returns a human-friendly label for the UI.
func (e EvidenceEffort) Label() string {
	switch e {
	case EvidenceEffortLow:
		return "Low"
	case EvidenceEffortHigh:
		return "High"
	default:
		return "Medium"
	}
}

// TopK controls the initial candidate pool size for evidence gathering.
func (e EvidenceEffort) TopK() int {
	switch e {
	case EvidenceEffortLow:
		return 4
	case EvidenceEffortHigh:
		return 12
	default:
		return 8
	}
}

// CandidatePoolSize controls the broader evidence pool before compression.
func (e EvidenceEffort) CandidatePoolSize() int {
	switch e {
	case EvidenceEffortLow:
		return 12
	case EvidenceEffortHigh:
		return 36
	default:
		return 24
	}
}

// NeighborDepth controls how many adjacent chunks to include around each hit.
func (e EvidenceEffort) NeighborDepth() int {
	switch e {
	case EvidenceEffortLow:
		return 0
	case EvidenceEffortHigh:
		return 2
	default:
		return 1
	}
}

// ContextMultiplier slightly expands the prompt budget as evidence effort rises.
func (e EvidenceEffort) ContextMultiplier() float64 {
	switch e {
	case EvidenceEffortLow:
		return 0.85
	case EvidenceEffortHigh:
		return 1.20
	default:
		return 1.00
	}
}

// EvidencePasses returns the maximum number of retrieval refinement passes.
func (e EvidenceEffort) EvidencePasses() int {
	switch e {
	case EvidenceEffortLow:
		return 1
	case EvidenceEffortHigh:
		return 3
	default:
		return 2
	}
}

// EvidenceLimit returns the maximum number of chunks kept after compression.
func (e EvidenceEffort) EvidenceLimit() int {
	switch e {
	case EvidenceEffortLow:
		return 6
	case EvidenceEffortHigh:
		return 16
	default:
		return 10
	}
}

// MaxEvidenceTokens returns the approximate token budget for retrieved evidence.
func (e EvidenceEffort) MaxEvidenceTokens() int {
	switch e {
	case EvidenceEffortLow:
		return 6000
	case EvidenceEffortHigh:
		return 20000
	default:
		return 12000
	}
}

// MaxEvidenceLatency returns the soft time budget for evidence construction.
func (e EvidenceEffort) MaxEvidenceLatency() time.Duration {
	switch e {
	case EvidenceEffortLow:
		return 650 * time.Millisecond
	case EvidenceEffortHigh:
		return 4 * time.Second
	default:
		return 1500 * time.Millisecond
	}
}

// CoverageThreshold is the minimum acceptable coverage before we stop expanding.
func (e EvidenceEffort) CoverageThreshold() float64 {
	switch e {
	case EvidenceEffortLow:
		return 0.55
	case EvidenceEffortHigh:
		return 0.85
	default:
		return 0.72
	}
}

// Guidance returns a short instruction line for the system prompt.
func (e EvidenceEffort) Guidance() string {
	switch e {
	case EvidenceEffortLow:
		return "Evidence effort is low. Prefer a faster, narrower evidence bundle and keep the answer concise."
	case EvidenceEffortHigh:
		return "Evidence effort is high. Gather broader neighboring context, favor accuracy over speed, and verify the evidence before answering."
	default:
		return "Evidence effort is medium. Balance speed and coverage with a moderately broad evidence bundle."
	}
}
