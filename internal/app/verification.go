package app

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"changeme/internal/agent"
	"changeme/internal/store"
)

type VerificationReport struct {
	Confidence       float64  `json:"confidence"`
	Verdict          string   `json:"verdict"`
	Summary          string   `json:"summary"`
	Issues           []string `json:"issues,omitempty"`
	HasRetrieval     bool     `json:"hasRetrieval"`
	HasMemory        bool     `json:"hasMemory"`
	HasWorkspace     bool     `json:"hasWorkspace"`
	SourceCount      int      `json:"sourceCount"`
	EvidenceCoverage float64  `json:"evidenceCoverage"`
	EvidencePasses   int      `json:"evidencePasses"`
	CheckedAtUnix    int64    `json:"checkedAtUnix"`
}

func (s *ChatService) verifyAnswer(prompt, response string, chunks []store.ScoredChunk, meta AgentResponseMeta, plan agent.Plan) VerificationReport {
	report := VerificationReport{
		HasRetrieval:     meta.UsedRetrieval,
		HasMemory:        meta.UsedMemory,
		HasWorkspace:     meta.UsedWorkspaceMemory,
		SourceCount:      meta.SourceCount,
		EvidenceCoverage: meta.EvidenceCoverage,
		EvidencePasses:   meta.EvidencePasses,
		CheckedAtUnix:    time.Now().Unix(),
	}

	issues := make([]string, 0, 4)
	score := 0.30

	if meta.UsedRetrieval {
		score += 0.18
	}
	if meta.UsedMemory {
		score += 0.08
	}
	if meta.UsedWorkspaceMemory {
		score += 0.05
	}

	if meta.SourceCount > 0 {
		score += math.Min(0.20, float64(meta.SourceCount)*0.04)
	} else if meta.UsedRetrieval {
		issues = append(issues, "retrieval used but no sources were attached")
		score -= 0.12
	}

	if meta.EvidenceCoverage > 0 {
		score += math.Min(0.15, meta.EvidenceCoverage*0.20)
	}
	if meta.EvidencePasses > 0 {
		score += math.Min(0.10, float64(meta.EvidencePasses-1)*0.03)
	}

	lowerResp := strings.ToLower(strings.TrimSpace(response))
	if lowerResp == "" {
		issues = append(issues, "assistant response is empty")
		score = 0.0
	}
	for _, phrase := range []string{"i don't know", "i do not know", "not sure", "uncertain", "i am not sure"} {
		if strings.Contains(lowerResp, phrase) {
			issues = append(issues, "response contains uncertainty language")
			score -= 0.08
			break
		}
	}

	if !meta.UsedRetrieval && meta.SourceCount == 0 && !meta.UsedMemory && !meta.UsedWorkspaceMemory {
		score -= 0.05
	}

	if meta.UsedRetrieval && meta.SourceCount > 0 && len(chunks) > 0 {
		coverageBoost := math.Min(0.10, float64(len(chunks))*0.015)
		score += coverageBoost
	}

	if plan.EvidenceCoverageTarget > 0 && meta.EvidenceCoverage+0.0001 < plan.EvidenceCoverageTarget {
		issues = append(issues, fmt.Sprintf("coverage below target (%.0f%% < %.0f%%)", meta.EvidenceCoverage*100, plan.EvidenceCoverageTarget*100))
		score -= 0.04
	}

	if len(response) < 40 && len(prompt) > 120 {
		issues = append(issues, "short answer for a long query")
		score -= 0.03
	}

	score = math.Max(0, math.Min(1, score))
	report.Confidence = score
	report.Issues = dedupeIssues(issues)
	report.Verdict = verificationVerdict(score, meta, report.Issues)
	report.Summary = verificationSummary(report)
	return report
}

func verificationVerdict(score float64, meta AgentResponseMeta, issues []string) string {
	switch {
	case score >= 0.82:
		return "strong"
	case score >= 0.65:
		return "moderate"
	case score >= 0.45:
		return "weak"
	default:
		if meta.UsedRetrieval && meta.SourceCount == 0 {
			return "unsupported"
		}
		return "weak"
	}
}

func verificationSummary(r VerificationReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s confidence (%.0f%%)", strings.Title(r.Verdict), r.Confidence*100)
	if r.SourceCount > 0 {
		fmt.Fprintf(&b, " · %d sources", r.SourceCount)
	}
	if r.EvidencePasses > 0 {
		fmt.Fprintf(&b, " · %d passes", r.EvidencePasses)
	}
	if r.EvidenceCoverage > 0 {
		fmt.Fprintf(&b, " · %.0f%% coverage", r.EvidenceCoverage*100)
	}
	if len(r.Issues) > 0 {
		fmt.Fprintf(&b, " · %s", r.Issues[0])
	}
	return strings.TrimSpace(b.String())
}

func dedupeIssues(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, issue := range in {
		issue = strings.TrimSpace(issue)
		if issue == "" {
			continue
		}
		key := strings.ToLower(issue)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, issue)
	}
	sort.Strings(out)
	return out
}
