package ingest

import (
	"strings"
	"unicode"
)

// SummarizeText returns a short extractive summary built from the most informative sentences.
// It is intentionally lightweight and offline-friendly.
func SummarizeText(text string, maxSentences int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if maxSentences <= 0 {
		maxSentences = 3
	}

	sentences := splitSentences(text)
	if len(sentences) == 0 {
		return truncateSmart(text, 240)
	}
	if len(sentences) <= maxSentences {
		return strings.Join(sentences, " ")
	}

	freq := make(map[string]int)
	for _, s := range sentences {
		for _, w := range strings.Fields(strings.ToLower(s)) {
			w = strings.Trim(w, "[](){}<>.,;:!?\"'`“”’")
			if len(w) < 3 || isStopWord(w) {
				continue
			}
			freq[w]++
		}
	}

	type scored struct {
		idx   int
		score float64
	}
	scores := make([]scored, 0, len(sentences))
	for i, s := range sentences {
		words := strings.Fields(strings.ToLower(s))
		if len(words) == 0 {
			continue
		}
		var score float64
		for _, w := range words {
			w = strings.Trim(w, "[](){}<>.,;:!?\"'`“”’")
			if len(w) < 3 || isStopWord(w) {
				continue
			}
			score += float64(freq[w])
		}
		// Prefer sentences that contain section-like cues or stronger text density.
		if strings.HasPrefix(strings.TrimSpace(s), "#") || strings.Contains(s, ":") {
			score *= 1.08
		}
		if len([]rune(s)) > 180 {
			score *= 0.94
		}
		scores = append(scores, scored{idx: i, score: score})
	}

	if len(scores) == 0 {
		return truncateSmart(text, 240)
	}

	// Pick top-scoring sentences but keep the original order for readability.
	top := make(map[int]struct{}, maxSentences)
	for len(top) < maxSentences && len(scores) > 0 {
		best := 0
		for i := 1; i < len(scores); i++ {
			if scores[i].score > scores[best].score || (scores[i].score == scores[best].score && scores[i].idx < scores[best].idx) {
				best = i
			}
		}
		top[scores[best].idx] = struct{}{}
		scores = append(scores[:best], scores[best+1:]...)
	}

	selected := make([]string, 0, len(top))
	for i, s := range sentences {
		if _, ok := top[i]; ok {
			selected = append(selected, s)
		}
	}
	if len(selected) == 0 {
		return truncateSmart(text, 240)
	}
	return strings.Join(selected, " ")
}

func splitSentences(text string) []string {
	var out []string
	var current strings.Builder
	for _, r := range text {
		current.WriteRune(r)
		if r == '.' || r == '!' || r == '?' || r == '\n' {
			s := strings.TrimSpace(current.String())
			if s != "" {
				out = append(out, strings.TrimSpace(s))
			}
			current.Reset()
		}
	}
	if tail := strings.TrimSpace(current.String()); tail != "" {
		out = append(out, tail)
	}
	cleaned := out[:0]
	for _, s := range out {
		s = strings.Join(strings.Fields(s), " ")
		s = strings.TrimFunc(s, func(r rune) bool { return unicode.IsSpace(r) || r == '•' || r == '-' })
		if s != "" {
			cleaned = append(cleaned, s)
		}
	}
	return cleaned
}

func isStopWord(w string) bool {
	switch w {
	case "the", "and", "for", "with", "that", "this", "from", "have", "will", "your", "about", "into", "there", "their", "what", "when", "where", "which", "while", "would", "could", "should", "you", "are", "was", "were", "has", "had", "not", "but", "also", "can", "our", "any", "all", "may", "use", "used", "using", "over", "under", "than", "then", "they", "them", "its", "it's", "we", "us":
		return true
	default:
		return false
	}
}

func truncateSmart(text string, limit int) string {
	text = strings.TrimSpace(strings.Join(strings.Fields(text), " "))
	if limit <= 0 || len(text) <= limit {
		return text
	}
	if limit <= 3 {
		return text[:limit]
	}
	return text[:limit-3] + "..."
}
